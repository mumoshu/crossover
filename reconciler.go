/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"fmt"

	"github.com/deislabs/smi-sdk-go/pkg/apis/split/v1alpha2"
	v1 "k8s.io/api/core/v1"
)

// reconciler reconciles ReplicaSets
type reconciler struct {
	cache map[string]map[string]v1alpha2.TrafficSplitBackend
}

func New() *reconciler {
	return &reconciler{
		cache: map[string]map[string]v1alpha2.TrafficSplitBackend{},
	}
}

func (r *reconciler) get(ctx context.Context, namespace, name string, res interface{}) error {
	return nil
}

func (r *reconciler) info(msg string, kvs ...interface{}) {

}

func (r *reconciler) err(e error, msg string, kvs ...interface{}) {

}

func (r *reconciler) ReconcileTrafficSplit(namespace, name string) error {
	ts := &v1alpha2.TrafficSplit{}
	err := r.get(context.TODO(), namespace, name, ts)
	if err != nil {
		return err
	}

	// Print the ReplicaSet
	r.info("Reconciling TrafficSplit", "service", ts.Spec.Service)

	svc := ts.Spec.Service

	_, ok := r.cache[svc]
	if !ok {
		r.cache[svc] = map[string]v1alpha2.TrafficSplitBackend{}
	}

	for _, b := range ts.Spec.Backends {
		backendSvc := b.Service
		r.cache[svc][backendSvc] = b
	}

	cm := &v1.ConfigMap{}
	cmName := fmt.Sprintf("%s-xds", svc)
	// TODO specific this via command-line flag(1. same with the trafficsplit object 2. same with the controller 3. the one specified via annotation 4. the one specified via flag)
	xdsNs := "kube-system"
	err = r.get(context.TODO(), xdsNs, cmName, cm)
	if err != nil {
		r.err(err, "Could not find ConfigMap %q", cmName)
	}

	xds, err := buildXDSConfig(cm, ts)
	if err != nil {
		r.err(err, "Could not build xDS config")
		return err
	}

	err = r.write(xds)
	if err != nil {
		r.err(err, "Could not write ConfigMap")
		return err
	}

	return nil
}

func (r *reconciler) write(xds map[string]interface{}) error {
	return nil
}

func buildXDSConfig(cm *v1.ConfigMap, rs *v1alpha2.TrafficSplit) (map[string]interface{}, error) {
	return nil, nil
}
