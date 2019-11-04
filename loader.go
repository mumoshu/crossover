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

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

type Loader struct {
	configMaps map[string]*ConfigMap
	namespace  string
	templates  map[string]*ConfigMap
	noop       bool
	token      string
	insecure   bool
	server     string
}

type Opts struct {
	Insecure bool
	Noop     bool
	Server   string
}

func NewLoader(namespace, token string, opts Opts) *Loader {
	return &Loader{
		namespace:  namespace,
		configMaps: make(map[string]*ConfigMap),
		templates:  make(map[string]*ConfigMap),
		token:      token,
		insecure:   opts.Insecure,
		server:     opts.Server,
		noop:       opts.Noop,
	}
}

func process(namespace, token string, configmaps []string, opts Opts) {
	tp := NewLoader(namespace, token, opts)
	tp.doLoad(configmaps)
}

func (tp *Loader) doLoad(configmaps []string) error {
	var cms []*ConfigMap

	for _, c := range configmaps {
		cm, err := tp.getConfigMap(tp.namespace, c)
		if err != nil {
			log.Printf("get configmap %s/%s: %v", tp.namespace, c, err)
			continue
		}
		cms = append(cms, cm)
	}

	if err := newWriter("").write(cms); err != nil {
		return fmt.Errorf("failed writing %v: %v", cms, err)
	}

	return nil
}

func (tp *Loader) doWatch(configmaps []string, done chan struct{}) error {
	updated := make(chan *ConfigMap)

	for _, c := range configmaps {
		if err := tp.startWatchingConfigMap(done, tp.namespace, c, updated); err != nil {
			return fmt.Errorf("failed to watch %s: %v", c, err)
		}
	}

LOOP:
	for {
		select {
		case cm, ok := <-updated:
			if !ok {
				break LOOP
			}
			log.Printf("watch: %v resourceVersion=%s", cm.ObjectMeta.Name, cm.ObjectMeta.ResourceVersion)
			if err := newWriter("").write([]*ConfigMap{cm}); err != nil {
				return err
			}
		case <-done:
			return nil
		}
	}

	return nil
}

func printObject(v interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	err := encoder.Encode(&v)
	if err != nil {
		return fmt.Errorf("error encoding object: %v", err)
	}
	return nil
}
