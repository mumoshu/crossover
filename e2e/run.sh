#!/usr/bin/env bash

set -xe
helm repo add stable https://kubernetes-charts.storage.googleapis.com
helm upgrade --install envoy stable/envoy -f example/values.e2e.yaml \
  --set services.eerie-octopus-podinfo.weight=0 \
  --set services.bold-olm-podinfo.weight=100
helm repo add flagger https://flagger.app
helm upgrade --install bold-olm flagger/podinfo --set canary.enabled=false
helm upgrade --install eerie-octopus flagger/podinfo --set canary.enabled=false
kubectl run -it --rm --image alpine:3.9 tester -- sh -c '
  apk add --update curl
  watch curl http://envoy:10000/headers
' &
pid=$!
sleep 5
# 100% bold-olm
helm upgrade --install envoy stable/envoy -f example/values.e2e.yaml \
  --set services.eerie-octopus-podinfo.weight=25 \
  --set services.bold-olm-podinfo.weight=75
sleep 5
helm upgrade --install envoy stable/envoy -f example/values.e2e.yaml \
  --set services.eerie-octopus-podinfo.weight=50 \
  --set services.bold-olm-podinfo.weight=50
sleep 5
helm upgrade --install envoy stable/envoy -f example/values.e2e.yaml \
  --set services.eerie-octopus-podinfo.weight=75 \
  --set services.bold-olm-podinfo.weight=25
sleep 5
# 100% eerie-octopus
helm upgrade --install envoy stable/envoy -f example/values.e2e.yaml \
  --set services.eerie-octopus-podinfo.weight=100 \
  --set services.bold-olm-podinfo.weight=0
sleep 5
kill $pid
