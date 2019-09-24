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

type TemplateProcessor struct {
	configMaps map[string]*ConfigMap
	namespace  string
	templates  map[string]*ConfigMap
	noop       bool
	token      string
	insecure   bool
}

func NewTemplateProcessor(namespace, token string, insecure bool) *TemplateProcessor {
	return &TemplateProcessor{
		namespace:  namespace,
		configMaps: make(map[string]*ConfigMap),
		templates:  make(map[string]*ConfigMap),
		token:      token,
		insecure:   insecure,
	}
}

func (tp *TemplateProcessor) setNoop(noop bool) {
	tp.noop = noop
}

func process(namespace, token string, configmaps []string, noop, insecure bool) {
	tp := NewTemplateProcessor(namespace, token, insecure)
	tp.setNoop(noop)
	tp.sync(configmaps)
}

func (tp *TemplateProcessor) sync(configmaps []string) {
	var cms []*ConfigMap

	for _, c := range configmaps {
		cm, err := tp.getConfigMap(tp.namespace, c, tp.token)
		if err != nil {
			log.Println(err)
			continue
		}
		cms = append(cms, cm)
	}

	if err := newWriter("").write(cms); err != nil {
		panic(err)
	}

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
