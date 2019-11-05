#!/usr/bin/env bash

# Exit on Ctrl-C
trap "exit" INT

# Stop all the sub-processes on exit
trap 'if [ ! -z "$(jobs -p)" ]; then echo Stopping all sub-processes 1>&2 ; kill $(jobs -p); fi' EXIT

set -xe

HELM_EXTRA_ARGS=${HELM_EXTRA_ARGS:-}

PODINFO_CHART=${PODINFO_CHART:-sp/podinfo}
USE_H2C=${USE_H2C:-}

echo Using $PODINFO_CHART
echo USE_H2C=${USE_H2C}

ENVOY_EXTRA_FLAGS=""
PODINFO_EXTRA_FLAGS=""
VEGETA_EXTRA_FLAGS=""
if [ ! -z "${USE_H2C}" ]; then
  ENVOY_EXTRA_FLAGS="--set services.eerie-octopus-podinfo.http2.enabled=true --set services.bold-olm-podinfo.http2.enabled=true"
  PODINFO_EXTRA_FLAGS="--set h2c.enabled=true"
  VEGETA_EXTRA_FLAGS="-http2=true -h2c"
else
  VEGETA_EXTRA_FLAGS="-http2=false"
fi

PODINFO_FLAGS="--set image.tag=3.1.4 --set canary.enabled=false ${PODINFO_EXTRA_FLAGS}"

echo Setting up Envoy front proxy.

helm repo add stable https://kubernetes-charts.storage.googleapis.com
helm upgrade --install envoy stable/envoy -f example/values.yaml \
  --set services.eerie-octopus-podinfo.weight=0 \
  --set services.bold-olm-podinfo.weight=100 --wait ${ENVOY_EXTRA_ARGS} ${HELM_EXTRA_ARGS}

echo Starting port-forward.

kubectl port-forward svc/envoy 10000 > e2e.pf.log &

sleep 5

echo Setting up podinfo backends.

helm repo add sp https://stefanprodan.github.io/podinfo
helm upgrade --install bold-olm "${PODINFO_CHART}" --wait ${PODINFO_FLAGS} ${HELM_EXTRA_ARGS}
helm upgrade --install eerie-octopus "${PODINFO_CHART}" --wait ${PODINFO_FLAGS} ${HELM_EXTRA_ARGS}

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

# 100% bold-olm
helm upgrade --install envoy stable/envoy -f example/values.yaml \
  --set services.eerie-octopus-podinfo.weight=25 \
  --set services.bold-olm-podinfo.weight=75

sleep 5

helm upgrade --install envoy stable/envoy -f example/values.yaml \
  --set services.eerie-octopus-podinfo.weight=50 \
  --set services.bold-olm-podinfo.weight=50

sleep 5

helm upgrade --install envoy stable/envoy -f example/values.yaml \
  --set services.eerie-octopus-podinfo.weight=75 \
  --set services.bold-olm-podinfo.weight=25

sleep 5

# 100% eerie-octopus

helm upgrade --install envoy stable/envoy -f example/values.yaml \
  --set services.eerie-octopus-podinfo.weight=100 \
  --set services.bold-olm-podinfo.weight=0

sleep 5

echo Waiting for Vegeta to stop...

# Stop before the exit trap, so that we don't get errors due to race like:
#   Stopping all sub-processes
#   {"attack":"","seq":946,"code":0,"timestamp":"2019-11-04T16:46:16.885395434+09:00","latency":1089318,"bytes_out":0,"bytes_in":0,"error":"Get http://localhost:10000/headers: dial tcp 0.0.0.0:0-\u003e[::1]:10000: connect: connection refused","body":null}
wait $vegeta_pid

sleep 1
