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

if [ ! -z "${USE_FLAGGER}" ]; then
  PRIMARY_SVC=podinfo-primary
  CANARY_SVC=podinfo-canary
  ENVOY_EXTRA_ARGS="-f example/values.flagger.yaml"
else
  PRIMARY_SVC=bold-olm-podinfo
  CANARY_SVC=eerie-octopus-podinfo
  ENVOY_EXTRA_ARGS="-f example/values.services.yaml"
fi

PODINFO_EXTRA_FLAGS=""
VEGETA_EXTRA_FLAGS=""
if [ ! -z "${USE_H2C}" ]; then
  ENVOY_EXTRA_ARGS="${ENVOY_EXTRA_ARGS} --set services.podinfo.backends.${CANARY_SVC}.http2.enabled=true --set services.podinfo.backends.${PRIMARY_SVC}.http2.enabled=true"
  PODINFO_EXTRA_FLAGS="--set h2c.enabled=true"
  VEGETA_EXTRA_FLAGS="-http2=true -h2c"
else
  VEGETA_EXTRA_FLAGS="-http2=false"
fi

if [ ! -z "${USE_SMI}" ]; then
  ENVOY_EXTRA_ARGS="${ENVOY_EXTRA_ARGS} --set services.podinfo.smi.enabled=true"
fi

# Clean up resources left by the previous e2e run
kubectl delete -f example/smi/trafficsplits.crd.yaml || true
kubectl delete -f example/smi/trafficsplits-v1alpha1.crd.yaml || true
kubectl delete canary podinfo && sleep 10 || :
kubectl delete trafficsplit podinfo || :

if [ ! -z "${USE_FLAGGER}" ]; then
    ENVOY_EXTRA_ARGS="${ENVOY_EXTRA_ARGS} --set smi.apiVersions.trafficSplits=v1alpha1"

  $HELM repo add flagger https://flagger.app
  $HELM upgrade --install flagger flagger/flagger \
    --set image.repository=mumoshu/flagger \
    --set meshProvider=smi:envoy-crossover \
    --set image.tag=k8s-svc-v1 \
    --set crd.create=true \
    --wait

  # Given this CRD, Flagger is able to automatically create trafficsplit on demand
  kubectl apply -f example/smi/trafficsplits-v1alpha1.crd.yaml

  RELEASE=eerie-octopus
  kubectl apply -f - <<EOS
apiVersion: v1
kind: Service
metadata:
  labels:
    app: podinfo
  name: podinfo
  namespace: default
spec:
  ports:
  - name: http
    port: 9898
    protocol: TCP
    targetPort: http
  - name: grpc
    port: 9999
    protocol: TCP
    targetPort: grpc
  selector:
    app: podinfo
    release: ${RELEASE}
  type: ClusterIP
EOS

  kubectl apply -f - << EOS
apiVersion: flagger.app/v1alpha3
kind: Canary
metadata:
  name: podinfo
  namespace: default
spec:
  provider: smi:crossover-envoy
  # deployment reference
  targetRef:
    apiVersion: core/v1
    kind: Service
    name: podinfo
  service:
    port: 9898
  canaryAnalysis:
    # schedule interval
    interval: 5s
    # canary increment step in percentage
    stepWeight: 20
    # We don't need this for canary release. Setting iterations enables Blue/Green deployment
    #iterations: 5
    threshold: 2
    metrics: []
EOS
else
  kubectl apply -f example/smi/trafficsplits.crd.yaml
fi

PODINFO_FLAGS="--set image.tag=3.1.4 --set canary.enabled=false ${PODINFO_EXTRA_FLAGS}"

echo Setting up podinfo backends.

$HELM repo add sp https://stefanprodan.github.io/podinfo
$HELM upgrade --install bold-olm "${PODINFO_CHART}" --wait ${PODINFO_FLAGS} ${HELM_EXTRA_ARGS}
$HELM upgrade --install eerie-octopus "${PODINFO_CHART}" --wait ${PODINFO_FLAGS} ${HELM_EXTRA_ARGS}

sleep 5

echo Setting up Envoy front proxy.

if [ ! -z "${USE_FLAGGER}" ]; then
  :
elif [ ! -z "${USE_SMI}" ]; then
  kubectl apply -f example/smi/podinfo-v0.trafficsplit.yaml
fi

$HELM repo add stable https://kubernetes-charts.storage.googleapis.com
$HELM upgrade --install envoy stable/envoy -f example/values.yaml \
  --set services.podinfo.backends.${CANARY_SVC}.weight=0 \
  --set services.podinfo.backends.${PRIMARY_SVC}.weight=100 --wait ${ENVOY_EXTRA_ARGS} ${HELM_EXTRA_ARGS}

echo Starting port-forward.

kubectl port-forward svc/envoy 10000 > e2e.pf.log &

sleep 5

echo Starting Vegeta.

DURATION=${DURATION:-30s}

if [ ! -z "${USE_FLAGGER}" ]; then
  DURATION=100s
fi

VEGETA_EXTRA_FLAGS=$VEGETA_EXTRA_FLAGS RATE=30 TARGET_ADDR=http://localhost:10000/headers DURATION="${DURATION}" $(dirname $0)/tools.sh encode | \
  tee e2e.encode.log | \
  $(dirname $0)/tools.sh parse | \
  tee e2e.parse.log | \
  $(dirname $0)/tools.sh aggregate \
  > e2e.aggregate.log &
vegeta_pid=$!

sleep 5

# 25% eerie-octopus
if [ ! -z "${USE_FLAGGER}" ]; then
  RELEASE=bold-olm
  kubectl apply -f - <<EOS
apiVersion: v1
kind: Service
metadata:
  labels:
    app: podinfo
  name: podinfo
  namespace: default
spec:
  ports:
  - name: http
    port: 9898
    protocol: TCP
    targetPort: http
  - name: grpc
    port: 9999
    protocol: TCP
    targetPort: grpc
  selector:
    app: podinfo
    release: ${RELEASE}
  type: ClusterIP
EOS
elif [ ! -z "${USE_SMI}" ]; then
  kubectl apply -f example/smi/podinfo-v1.trafficsplit.yaml
else
  $HELM upgrade --install envoy stable/envoy -f example/values.yaml \
  --set services.podinfo.backends.${CANARY_SVC}.weight=25 \
  --set services.podinfo.backends.${PRIMARY_SVC}.weight=75 ${ENVOY_EXTRA_ARGS} ${HELM_EXTRA_ARGS}
fi

sleep 5

# 50% eerie-octopus
if [ ! -z "${USE_FLAGGER}" ]; then
  :
elif [ ! -z "${USE_SMI}" ]; then
  kubectl apply -f example/smi/podinfo-v2.trafficsplit.yaml
else
  $HELM upgrade --install envoy stable/envoy -f example/values.yaml \
  --set services.podinfo.backends.${CANARY_SVC}.weight=50 \
  --set services.podinfo.backends.${PRIMARY_SVC}.weight=50 ${ENVOY_EXTRA_ARGS} ${HELM_EXTRA_ARGS}
fi

sleep 5

# 75% eerie-octopus
if [ ! -z "${USE_FLAGGER}" ]; then
  :
elif [ ! -z "${USE_SMI}" ]; then
  kubectl apply -f example/smi/podinfo-v3.trafficsplit.yaml
else
  $HELM upgrade --install envoy stable/envoy -f example/values.yaml \
  --set services.podinfo.backends.${CANARY_SVC}.weight=75 \
  --set services.podinfo.backends.${PRIMARY_SVC}.weight=25 ${ENVOY_EXTRA_ARGS} ${HELM_EXTRA_ARGS}
fi

sleep 5

# 100% eerie-octopus
if [ ! -z "${USE_FLAGGER}" ]; then
  :
elif [ ! -z "${USE_SMI}" ]; then
  kubectl apply -f example/smi/podinfo-v4.trafficsplit.yaml
else
  $HELM upgrade --install envoy stable/envoy -f example/values.yaml \
  --set services.podinfo.backends.${CANARY_SVC}.weight=100 \
  --set services.podinfo.backends.${PRIMARY_SVC}.weight=0 ${ENVOY_EXTRA_ARGS} ${HELM_EXTRA_ARGS}
fi

sleep 5

echo Waiting for Vegeta to stop...

# Stop before the exit trap, so that we don't get errors due to race like:
#   Stopping all sub-processes
#   {"attack":"","seq":946,"code":0,"timestamp":"2019-11-04T16:46:16.885395434+09:00","latency":1089318,"bytes_out":0,"bytes_in":0,"error":"Get http://localhost:10000/headers: dial tcp 0.0.0.0:0-\u003e[::1]:10000: connect: connection refused","body":null}
wait $vegeta_pid

sleep 1
