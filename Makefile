.PHONY: build
build:
	go build -o envoy-xds-configmap-loader .

.PHONY: image
image: image/build image/push

.PHONY: image/build
image/build:
	docker build -t mumoshu/envoy-xds-configmap-loader:canary-$(shell git rev-parse --short HEAD) .

.PHONY: image/push
image/push:
	docker push mumoshu/envoy-xds-configmap-loader:canary-$(shell git rev-parse --short HEAD)

.PHONY: run
run: build
	./envoy-xds-configmap-loader --namespace default --token-file ./mytoken --configmap incendiary-shark-envoy-xds --onetime --insecure --apiserver "https://kubernetes.docker.internal:6443"
