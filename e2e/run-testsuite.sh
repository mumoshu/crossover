#!/usr/bin/env bash

# Exit on Ctrl-C
trap "exit" INT

# Stop all the sub-processes on exit
trap 'if [ ! -z "$(jobs -p)" ]; then echo Stopping all sub-processes 1>&2 ; kill $(jobs -p); fi' EXIT

set -xe

HELM=${HELM:-helm}

HELM_EXTRA_ARGS=${HELM_EXTRA_ARGS:-}

PODINFO_CHART=${PODINFO_CHART:-sp/podinfo}
USE_H2C=${USE_H2C:-}

echo Using $PODINFO_CHART
echo USE_H2C=${USE_H2C}

ENVOY_EXTRA_ARGS=""
PODINFO_EXTRA_FLAGS=""
VEGETA_EXTRA_FLAGS=""
if [ ! -z "${USE_H2C}" ]; then
  ENVOY_EXTRA_ARGS="--set services.podinfo.backends.eerie-octopus-podinfo.http2.enabled=true --set services.podinfo.backends.bold-olm-podinfo.http2.enabled=true"
  PODINFO_EXTRA_FLAGS="--set h2c.enabled=true"
  VEGETA_EXTRA_FLAGS="-http2=true -h2c"
else
  VEGETA_EXTRA_FLAGS="-http2=false"
fi

if [ ! -z "${USE_SMI}" ]; then
  ENVOY_EXTRA_ARGS="${ENVOY_EXTRA_ARGS} --set services.podinfo.smi.enabled=true"
fi

PODINFO_FLAGS="--set image.tag=3.1.4 --set canary.enabled=false ${PODINFO_EXTRA_FLAGS}"

echo Setting up podinfo backends.

$HELM repo add sp https://stefanprodan.github.io/podinfo
$HELM upgrade --install bold-olm "${PODINFO_CHART}" --wait ${PODINFO_FLAGS} ${HELM_EXTRA_ARGS}
$HELM upgrade --install eerie-octopus "${PODINFO_CHART}" --wait ${PODINFO_FLAGS} ${HELM_EXTRA_ARGS}

sleep 5

echo Setting up Envoy front proxy.

kubectl apply -f example/smi/trafficsplits.crd.yaml
kubectl apply -f example/smi/podinfo-v0.trafficsplit.yaml

$HELM repo add stable https://kubernetes-charts.storage.googleapis.com
$HELM upgrade --install envoy stable/envoy -f example/values.yaml \
  --set services.podinfo.backends.eerie-octopus-podinfo.weight=0 \
  --set services.podinfo.backends.bold-olm-podinfo.weight=100 --wait ${ENVOY_EXTRA_ARGS} ${HELM_EXTRA_ARGS}

echo Starting port-forward.

kubectl port-forward svc/envoy 10000 > e2e.pf.log &

sleep 5

echo Starting Vegeta.

DURATION=${DURATION:-30s}

VEGETA_EXTRA_FLAGS=$VEGETA_EXTRA_FLAGS RATE=30 TARGET_ADDR=http://localhost:10000/headers DURATION="${DURATION}" $(dirname $0)/tools.sh encode | \
  tee e2e.encode.log | \
  $(dirname $0)/tools.sh parse | \
  tee e2e.parse.log | \
  $(dirname $0)/tools.sh aggregate \
  > e2e.aggregate.log &
vegeta_pid=$!

sleep 5

# 25% bold-olm
if [ ! -z "${USE_SMI}" ]; then
  kubectl apply -f example/smi/podinfo-v1.trafficsplit.yaml
else
  $HELM upgrade --install envoy stable/envoy -f example/values.yaml \
  --set services.podinfo.backends.eerie-octopus-podinfo.weight=25 \
  --set services.podinfo.backends.bold-olm-podinfo.weight=75 ${ENVOY_EXTRA_ARGS} ${HELM_EXTRA_ARGS}
fi

sleep 5

# 50% bold-olm
if [ ! -z "${USE_SMI}" ]; then
  kubectl apply -f example/smi/podinfo-v2.trafficsplit.yaml
else
  $HELM upgrade --install envoy stable/envoy -f example/values.yaml \
  --set services.podinfo.backends.eerie-octopus-podinfo.weight=50 \
  --set services.podinfo.backends.bold-olm-podinfo.weight=50 ${ENVOY_EXTRA_ARGS} ${HELM_EXTRA_ARGS}
fi

sleep 5

# 75% bold-olm
if [ ! -z "${USE_SMI}" ]; then
  kubectl apply -f example/smi/podinfo-v3.trafficsplit.yaml
else
  $HELM upgrade --install envoy stable/envoy -f example/values.yaml \
  --set services.podinfo.backends.eerie-octopus-podinfo.weight=75 \
  --set services.podinfo.backends.bold-olm-podinfo.weight=25 ${ENVOY_EXTRA_ARGS} ${HELM_EXTRA_ARGS}
fi

sleep 5

# 100% eerie-octopus

# 100% bold-olm
if [ ! -z "${USE_SMI}" ]; then
  kubectl apply -f example/smi/podinfo-v4.trafficsplit.yaml
else
  $HELM upgrade --install envoy stable/envoy -f example/values.yaml \
  --set services.podinfo.backends.eerie-octopus-podinfo.weight=100 \
  --set services.podinfo.backends.bold-olm-podinfo.weight=0 ${ENVOY_EXTRA_ARGS} ${HELM_EXTRA_ARGS}
fi

sleep 5

echo Waiting for Vegeta to stop...

# Stop before the exit trap, so that we don't get errors due to race like:
#   Stopping all sub-processes
#   {"attack":"","seq":946,"code":0,"timestamp":"2019-11-04T16:46:16.885395434+09:00","latency":1089318,"bytes_out":0,"bytes_in":0,"error":"Get http://localhost:10000/headers: dial tcp 0.0.0.0:0-\u003e[::1]:10000: connect: connection refused","body":null}
wait $vegeta_pid

sleep 1
