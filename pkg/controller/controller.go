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

package controller

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/mumoshu/envoy-configmap-loader/pkg/kubeclient"
	"github.com/mumoshu/envoy-configmap-loader/pkg/reconciler"
	"github.com/mumoshu/envoy-configmap-loader/pkg/types"
)

type Controller struct {
	namespace string
	resourceNames StringSlice

	client     kubeclient.Client
	reconciler reconciler.Reconciler
	updated    chan string
}

type Opts struct {
	Insecure bool
	Noop     bool
	Server   string
}

func NewController(namespace string, client kubeclient.Client, reconciler reconciler.Reconciler) *Controller {
	sync := &Controller{
		namespace:  namespace,
		client:     client,
		reconciler: reconciler,
		updated:    make(chan string),
	}
	return sync
}

func (s *Controller) Poll(ctx context.Context, resourceNames []string, syncInterval time.Duration) error {
	for {
		for _, c := range resourceNames {
			s.updated <- c
		}
		log.Printf("Enqueued %d resources. Next sync in %v seconds.", len(resourceNames), syncInterval.Seconds())
		select {
		case <-time.After(syncInterval):
		case <-ctx.Done():
			return nil
		}
	}
	return nil
}

func (s *Controller) Once() error {
	for _, c := range s.resourceNames {
		if err := s.reconciler.Reconcile(c); err != nil {
			return err
		}
	}
	return nil
}

func (s *Controller) Watch(ctx context.Context) error {
	wg := sync.WaitGroup{}

	for i := range s.resourceNames {
		c := s.resourceNames[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.client.RetryWatch(ctx, s.namespace, c, s.updated); err != nil {
				panic(fmt.Errorf("failed to watch %s: %v", c, err))
			}
		}()
	}

	wg.Wait()

	return nil
}

// Run starts the reconcilation loop and blocks until it finishes
func (s *Controller) Run(ctx context.Context) error {
LOOP:
	for {
		select {
		case name, ok := <-s.updated:
			if !ok {
				// Consume all the pending messages so that blockers like watchers can exit
				s.updated = nil
				break LOOP
			}
			if err := s.reconciler.Reconcile(name); err != nil && err != types.ErrNotExist {
				return err
			}
		case <-ctx.Done():
			break LOOP
			return nil
		}
	}
	return nil
}
