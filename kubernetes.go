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
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"time"
)

var ErrNotExist = errors.New("object does not exist")

type ConfigMap struct {
	ApiVersion string            `json:"apiVersion"`
	Data       map[string]string `json:"data"`
	Kind       string            `json:"kind"`
	ObjectMeta Metadata          `json:"metadata"`
}

type Metadata struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

var server string

func init() {
	server = os.Getenv("APISERVER")

	if server == "" {
		server = "http://127.0.0.1:8001"
	}
}

func (tp *TemplateProcessor) getConfigMap(namespace, name, token string) (*ConfigMap, error) {
	u := fmt.Sprintf("%s/api/v1/namespaces/%s/configmaps/%s", server, namespace, name)
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: tp.insecure,
		},
	}
	client := &http.Client{
		Transport: transport,
	}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 404 {
		return nil, ErrNotExist
	}

	if resp.StatusCode != 200 {
		return nil, errors.New("non 200 response code")
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	var cm ConfigMap
	if err := json.Unmarshal(data, &cm); err != nil {
		return nil, err
	}

	return &cm, nil
}

func newConfigMap(namespace, name, key, value string) *ConfigMap {
	c := &ConfigMap{
		ApiVersion: "v1",
		Data:       make(map[string]string),
		Kind:       "ConfigMap",
		ObjectMeta: Metadata{
			Name:      name,
			Namespace: namespace,
		},
	}
	c.Data[key] = value
	return c
}

func createConfigMap(c *ConfigMap) error {
	body, err := json.MarshalIndent(&c, "", "  ")
	if err != nil {
		return fmt.Errorf("error encoding configmap %s: %v", c.ObjectMeta.Name, err)
	}

	u := fmt.Sprintf("http://127.0.0.1:8001/api/v1/namespaces/%s/configmaps", c.ObjectMeta.Namespace)
	resp, err := http.Post(u, "", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("error creating configmap %s: %v", c.ObjectMeta.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		return fmt.Errorf("error creating configmap %s; got HTTP %v status code", c.ObjectMeta.Name, resp.StatusCode)
	}

	return nil
}
