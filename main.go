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
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	configmaps   stringSlice
	namespace    string
	noop         bool
	onetime      bool
	syncInterval time.Duration
	tokenfile    string
	insecure     bool
	server       string
	watch        bool
)

func main() {
	defaultNs := os.Getenv("NS")
	if defaultNs == "" {
		defaultNs = os.Getenv("POD_NAMESPACE")
	}
	flag.StringVar(&namespace, "namespace", defaultNs, "the namespace to process.")
	flag.StringVar(&tokenfile, "token-file", "/var/run/secrets/kubernetes.io/serviceaccount/token", "path to serviceaccount token file")
	flag.StringVar(&server, "apiserver", "https://kubernetes", "K8s api endpoint")
	flag.Var(&configmaps, "configmap", "the configmap to process.")
	flag.BoolVar(&noop, "dry-run", false, "print processed configmaps and secrets and do not submit them to the cluster.")
	flag.BoolVar(&onetime, "onetime", false, "run one time and exit.")
	flag.BoolVar(&insecure, "insecure", false, "disable tls server verification")
	flag.BoolVar(&watch, "watch", false, "use watch api to detect changes near realtime")
	flag.DurationVar(&syncInterval, "sync-interval", (60 * time.Second), "the time duration between template processing.")
	flag.Parse()

	tokenBytes, err := ioutil.ReadFile(tokenfile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "reading token: %v\n", err)
	}

	token := strings.TrimSpace(string(tokenBytes))

	opts := Opts{
		Noop:     noop,
		Insecure: insecure,
		Server:   server,
	}

	loader := NewLoader(namespace, token, opts)

	if onetime {
		if err := loader.doLoad(configmaps); err != nil {
			log.Fatalf("%v", err)
		}
		os.Exit(0)
	}

	log.Println("Starting envoy-xds-configmap-loader...")

	var wg sync.WaitGroup
	done := make(chan struct{})

	go func() {
		wg.Add(1)
		for {
			if err := loader.doLoad(configmaps); err != nil {
				log.Fatalf("%v", err)
			}
			log.Printf("Syncing templates complete. Next sync in %v seconds.", syncInterval.Seconds())
			select {
			case <-time.After(syncInterval):
			case <-done:
				wg.Done()
				return
			}
		}
	}()

	if watch {
		go func() {
			wg.Add(1)
			if err := loader.doWatch(configmaps, done); err != nil {
				log.Fatalf("watch: %v", err)
			}
			wg.Done()
		}()
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	<-signalChan
	log.Printf("Shutdown signal received, exiting...")
	close(done)
	wg.Wait()
	os.Exit(0)
}
