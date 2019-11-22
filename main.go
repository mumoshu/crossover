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
	"context"
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

	"github.com/mumoshu/envoy-configmap-loader/pkg/controller"
)

func main() {
	var tokenfile string

	manager := &controller.Manager{}

	defaultNs := os.Getenv("NS")
	if defaultNs == "" {
		defaultNs = os.Getenv("POD_NAMESPACE")
	}

	flag.StringVar(&manager.Namespace, "namespace", defaultNs, "the namespace to process.")
	flag.StringVar(&tokenfile, "token-file", "/var/run/secrets/kubernetes.io/serviceaccount/token", "path to serviceaccount token file")
	flag.StringVar(&manager.Server, "apiserver", "https://kubernetes", "K8s api endpoint")
	flag.StringVar(&manager.OutputDir, "output-dir", "", "Directory to putput xDS configs so that Envoy can read")
	flag.Var(&manager.ConfigMaps, "configmap", "the configmap to process.")
	flag.BoolVar(&manager.Noop, "dry-run", false, "print processed configmaps and secrets and do not submit them to the cluster.")
	flag.BoolVar(&manager.Onetime, "onetime", false, "run one time and exit.")
	flag.BoolVar(&manager.Insecure, "insecure", false, "disable tls server verification")
	flag.BoolVar(&manager.Watch, "watch", false, "use watch api to detect changes near realtime")
	flag.BoolVar(&manager.SMIEnabled, "smi", false, "Enable SMI integration")
	flag.Var(&manager.TrafficSplits, "trafficsplit", "the trafficsplit to be watched and merged into the configmap")
	flag.DurationVar(&manager.SyncInterval, "sync-interval", (60 * time.Second), "the time duration between template processing.")
	flag.Parse()

	if len(manager.TrafficSplits) > 0 {
		manager.SMIEnabled = true
	}

	tokenBytes, err := ioutil.ReadFile(tokenfile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "reading token: %v\n", err)
	}

	manager.Token = strings.TrimSpace(string(tokenBytes))

	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(context.Background())

	wg.Add(1)
	go func() {
		if err := manager.Run(ctx); err != nil {
			log.Printf("Error: %v", err)
			os.Exit(1)
		}
		wg.Done()
		cancel()
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-signalChan:
		log.Printf("Shutdown signal received. Exiting...")
		cancel()
		wg.Wait()
	case <-ctx.Done():
		log.Printf("Done writing Envoy configs. Existing...")
	}
	os.Exit(0)
}
