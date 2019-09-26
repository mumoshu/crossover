# envoy-xds-configmap-loader

The minimal and sufficient init/sidecar container to serve xDS files from Kubernetes configmaps in near real-time.

Features:

- **Minimal dependencies**: No dependencies other than Go standard library
- **Gradual migration path**: Start with vanilla Envoy with static config. Then turn on dynamic config with envoy-xds-configmap-loader.
- **Easy to maintain**: No gRPC/REST server to maintain. Distribute xDS data via Envoy's `local file` config-source.
- **Completeness**: Access to every feature Envoy provides. `envoy-xds-configmap-loader` makes no leaky abstraction on top.

## Features

### Minimal dependencies

No dependencies other than Go standard library. No need for kubectl or client-go as we rely on the stable v1 configmaps only.

### Gradual migration path / Easy to start

Edit your static envoy configuration to load xDS from local files.
Update local files via configmaps by adding `envoy-xds-configmap-loader` as an init container and a sidecar container of your Envoy pod.
That's all you need really!

### Easy to maintain / Simple to understand

No gRPC, REST server or serious K8s controller to maintain and debug!

It's just a simple golang program to HTTP get configmaps, write their contents as local files, and swapping symlinks.

### Feature Complete

Access to every feature Envoy provides. `envoy-xds-configmap-loader` makes no leaky abstraction on top of Envoy.

## Use-cases

- Ingress Gateway
- Canary Releases
- In-Cluster Router/Load-Balancer

### Ingress Gateway

Turn [stable/envoy](https://github.com/helm/charts/tree/master/stable/envoy) chart into a dynamically configurable API Gateway, Ingress Gateway or Front Proxy

### Canary Releases

Do weighted load-balancing and canary deployments with zero Envoy restart, redeployment and CRD. [Just distributed configmap contents as RDS files!](https://www.envoyproxy.io/learn/incremental-deploys#weighted-load-balancing).

### In-Cluster Router/Load-Balancer

Wanna add retries, circuit-breakers, tracing, metrics to your traffic? Deploy Envoy paired with `envoy-xds-configmap-loader` in front of your apps. No need for service meshes from day 1.

## What's this?

`envoy-xds-configmap-loader` is an init-container AND a sidecar for your Envoy proxy to use K8s ConfigMaps as xDS backend.

This works by loading kvs defined within specified configmap(s) and writing files assuming the key is the filename and the value is the content.

You then point your Envoy to read xDS from the directory `/srv/runtime/*.yaml`.

`envoy-xds-configmap-loader` writes files read from configmap(s) into the directory, triggers [symlink swap](https://www.envoyproxy.io/docs/envoy/latest/configuration/operations/runtime#updating-runtime-values-via-symbolic-link-swap)
 so that Envoy finally detects and applies changes. 
 
 ## Why not use configmap volumes?
 
You may [already know that K8s supports mounting configmaps as container volumes out of the box](https://kubernetes.io/docs/tasks/configure-pod-container/configure-pod-configmap/#add-configmap-data-to-a-volume).

The downside of using that feature to feed Envoy xDS files is that it takes 1 minute(default, configurable via kubelet `--sync-interval`) a change is reflected to the volume.

And more importantly, Envoy is unable to detect changes made in configmap volumes due to that it relies on `inotify` `MOVE` events to occur, where configmap volume changes only trigger the below events:

```
root@envoy-675dc8d98b-tvw9b:/# inotifywait -m /xds/rds.yaml
Setting up watches.
Watches established.
/xds/rds.yaml OPEN
/xds/rds.yaml ACCESS
/xds/rds.yaml CLOSE_NOWRITE,CLOSE
/xds/rds.yaml ATTRIB
/xds/rds.yaml DELETE_SELF
```

So in nutshell, `envoy-xds-configmap-loader` is the minimal and sufficient companion to actually distribute xDS via configmaps, without using more advanced CRD-based solutions like Istio and VMWare Contour.

## Getting Started

Try weighted load-balancing using `envoy-xds-configmap-loader`!

Firstly run the loader along with Envoy using the [stable/envoy]() chart:

```
helm upgrade --install envoy stable/envoy -f example/values.yaml
```

Then install backends - we use @stefanprodan's awesome [podinfo](https://github.com/stefanprodan/podinfo):

```
helm repo add flagger https://flagger.app
helm upgrade --install bold-olm flagger/podinfo --set canary.enabled=false
helm upgrade --install eerie-octopus flagger/podinfo --set canary.enabled=false
```

In another terminal, run the tester pod to watch traffic shifts:

```
kubectl run -it --rm --image alpine:3.9 tester sh

apk add --update curl
watch curl http://envoy:10000
```

Finally, try changing load-balancing weights instantly and without restarting Envoy at all:

```
# 100% bold-olm
helm upgrade --install envoy ~/charts/stable/envoy -f example/values.yaml \
  --set services.eerie-octopus-podinfo.weight=0 \
  --set services.bold-olm-podinfo.weight=100

# 100% eerie-octopus
helm upgrade --install envoy ~/charts/stable/envoy -f example/values.yaml \
  --set services.eerie-octopus-podinfo.weight=100 \
  --set services.bold-olm-podinfo.weight=0
```

See [example/values.yaml]() for more details on the configuration.

## Developing

Bring your own K8s cluster, move to the project root, and run the following commands to give it a ride:

```
sudo mkdir /srv/runtime
sudo chmod -R 777 /srv/runtime
k get secret -o json $(k get secret | grep default-token | awk '{print $1 }') | jq -r .data.token | base64 -D > mytoken
export APISERVER=$(k config view --minify -o json | jq -r .clusters[0].cluster.server)
make build && ./envoy-xds-configmap-loader --namespace default --token-file ./mytoken --configmap incendiary-shark-envoy-xds --onetime --insecure --apiserver "http://127.0.0.1:8001"
```

## References

- [File Based Dynamic Configuration of Routes in Envoy Proxy](https://medium.com/grensesnittet/file-based-dynamic-configuration-of-routes-in-envoy-proxy-6234dae968d2)
- ["How does one atomically change a symlink to a directory in busybox?"](https://unix.stackexchange.com/questions/5093/how-does-one-atomically-change-a-symlink-to-a-directory-in-busybox)
- [Runtime configuration — envoy](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/operations/runtime.html)
