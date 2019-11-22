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

gen-mytoken:
	kubectl get secret default-token-lggrz -o jsonpath={.data.token} | base64 -D > mytoken

gen-configmap:
	@helm fetch stable/envoy
	@helm template envoy*.tgz --name incendiary-shark -x templates/xds.configmap.yaml -f example/values.yaml \
      --set services.podinfo.backends.eerie-octopus-podinfo.weight=100 \
      --set services.podinfo.backends.bold-olm-podinfo.weight=0

.PHONY: run/onetime
run/onetime: build
	./envoy-xds-configmap-loader \
	  --namespace default \
	  --token-file ./mytoken \
	  --configmap incendiary-shark-envoy-xds \
	  --onetime \
	  --insecure \
	  --apiserver "https://kubernetes.docker.internal:6443" \
	  --output-dir "./test/out"

.PHONY: run/watch
run/watch: build
	./envoy-xds-configmap-loader \
	  --namespace default \
	  --token-file ./mytoken \
	  --configmap incendiary-shark-envoy-xds \
	  --insecure \
	  --apiserver "https://kubernetes.docker.internal:6443" \
	  --watch \
	  --output-dir "./test/out"

.PHONY: run/watch-smi
run/watch-smi: build
	./envoy-xds-configmap-loader \
	  --namespace default \
	  --token-file ./mytoken \
	  --configmap envoy-xds \
	  --trafficsplit podinfo \
	  --insecure \
	  --apiserver "https://kubernetes.docker.internal:6443" \
	  --watch \
	  --smi \
	  --output-dir "./test/out"

.PHONY: watch
watch:
	@curl -s -k -H "Authorization: Bearer $(shell cat ./mytoken)" https://kubernetes.docker.internal:6443/api/v1/watch/namespaces/default/configmaps/incendiary-shark-envoy-xds

.PHONY: e2e/h2c
e2e/h2c:
	USE_H2C=1 ./e2e/run-testsuite.sh

.PHONY: e2e/h1
e2e/h1:
	./e2e/run-testsuite.sh

.PHONY: e2e/h2c-smi
e2e/h2c-smi:
	USE_H2C=1 USE_SMI=1 ./e2e/run-testsuite.sh

.PHONY: e2e/h1-smi
e2e/h1-smi:
	USE_SMI=1 ./e2e/run-testsuite.sh

.PHONY: e2e/jplot
e2e/jplot:
	./e2e/run.sh "tail -f e2e.aggregate.log | ./e2e/tools.sh jplot"
