// Copyright 2019 Yusuke Kuoka. All Rights Reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
//
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package reconciler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/mumoshu/crossover/pkg/kubeclient"
	"github.com/mumoshu/crossover/pkg/log"
	"github.com/mumoshu/crossover/pkg/types"
	"gopkg.in/yaml.v3"
)

type TrafficSplitReconciler struct {
	TrafficSplits kubeclient.ReadOnlyClient
	ConfigMaps    kubeclient.Client
	Namespace     string
	TsToConfigs   map[string]string

	log.Logger
}

func (r *TrafficSplitReconciler) Reconcile(name string) error {
	ts := TrafficSplit{}
	err := r.TrafficSplits.Get(r.Namespace, name, &ts)
	if err != nil {
		if err == types.ErrNotExist {
			r.Infof("Trafficsplit %s/%s not found. Skipping reconcilation. This will be retried soon", r.Namespace, name)
			return nil
		}
		r.Errorf("Unexpected error while getting Trafficsplit %s/%s: %v", r.Namespace, name, err)
		return err
	}

	specYaml := bytes.Buffer{}
	enc := yaml.NewEncoder(&specYaml)
	enc.SetIndent(2)
	if err := enc.Encode(ts.Spec); err != nil {
		return err
	}
	r.Infof("Reconciling trafficsplit %s/%s:\n%s", r.Namespace, name, specYaml.String())

	svc := ts.Spec.Service

	svcToTsBackend := map[string]TrafficSplitBackend{}
	for _, b := range ts.Spec.Backends {
		svcToTsBackend[b.Service] = b
	}

	tplCm := ConfigMap{}
	tplCmName, ok := r.TsToConfigs[name]
	if !ok {
		panic(fmt.Sprintf("detected misconfiguration: no configmap name defined for trafficsplit named %q", name))
	}
	cmName := fmt.Sprintf("%s-gen", tplCmName)
	// TODO specific this via command-line flag(1. same with the trafficsplit object 2. same with the controller 3. the one specified via annotation 4. the one specified via flag)
	xdsNs := r.Namespace

	err = r.ConfigMaps.Get(xdsNs, tplCmName, &tplCm)
	if err != nil {
		if err == types.ErrNotExist {
			r.Errorf("Could not find template ConfigMap %q. Please create it: %v", tplCmName, err)
			return nil
		} else {
			return err
		}
	}

	data := map[string]string{}
DATA:
	for file, conf := range tplCm.Data {
		obj := map[string]interface{}{}

		if err := yaml.Unmarshal([]byte(conf), &obj); err != nil {
			return err
		}

		found := find(obj, []string{"resources", "*", "virtual_hosts", "name=" + svc, "routes", "*", "route", "weighted_clusters", "clusters"}, func(clusters interface{}) {
			for name, backend := range svcToTsBackend {
				find(clusters, []string{"name=" + name}, func(cluster interface{}) {
					set(cluster, "weight", backend.Weight)
				})
			}
		})
		if !found {
			continue DATA
		}

		buf := bytes.Buffer{}
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		err := enc.Encode(obj)
		if err != nil {
			r.Infof("Skipping SMI merge for %s/%s: %v", xdsNs, cmName, err)
			return nil
		}

		data[file] = buf.String()
	}

	tplCm.Data = data

	cm := ConfigMap{}

	if err = r.ConfigMaps.Get(xdsNs, cmName, &cm); err != nil {
		if err == types.ErrNotExist {
			cm := tplCm
			delete(cm.ObjectMeta.Annotations, "kubectl.kubernetes.io/last-applied-configuration")
			cm.ObjectMeta.Name = cmName
			cm.ObjectMeta.ResourceVersion = ""
			if err := r.ConfigMaps.Create(xdsNs, cm); err != nil {
				return err
			}
			return nil
		}
		return err
	}

	cm.Data = data
	return r.ConfigMaps.Replace(xdsNs, cmName, &cm)
}

type TrafficSplit struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired behavior of the traffic split.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#spec-and-status
	Spec TrafficSplitSpec `json:"spec,omitempty"`
}

// TrafficSplitSpec is the specification for a TrafficSplit
type TrafficSplitSpec struct {
	Service  string                `json:"service,omitempty"`
	Backends []TrafficSplitBackend `json:"backends,omitempty"`
}

// TrafficSplitBackend defines a backend
type TrafficSplitBackend struct {
	Service string `json:"service,omitempty"`
	Weight  int    `json:"weight,omitempty"`
}

type TrafficSplitBackendV1Alpha2 struct {
	Service string `json:"service,omitempty"`
	Weight  int    `json:"weight,omitempty"`
}

type TrafficSplitBackendV1Alpha1 struct {
	Service string `json:"service,omitempty"`
	Weight  string `json:"weight,omitempty"`
}

func (e *TrafficSplitBackend) UnmarshalJSON(data []byte) (err error) {
	var v1alpha2 TrafficSplitBackendV1Alpha2
	if err = json.Unmarshal(data, &v1alpha2); err != nil {
		var v1alpha1 TrafficSplitBackendV1Alpha1
		if err = json.Unmarshal(data, &v1alpha1); err != nil {
			return err
		}
		v1alpha2.Service = v1alpha1.Service
		weight, err := strconv.Atoi(v1alpha1.Weight)
		if err != nil {
			return err
		}
		v1alpha2.Weight = weight
	}
	e.Service = v1alpha2.Service
	e.Weight = v1alpha2.Weight
	return nil
}
