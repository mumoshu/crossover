package controller

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/mumoshu/crossover/pkg/kubeclient"
	"github.com/mumoshu/crossover/pkg/reconciler"
)

type Manager struct {
	Namespace     string
	Noop          bool
	Token         string
	Insecure      bool
	Server        string
	SMIEnabled    bool
	Watch         bool
	SyncInterval  time.Duration
	OutputDir     string
	Onetime       bool
	ConfigMaps    StringSlice
	TrafficSplits StringSlice
}

func (m *Manager) Run(ctx context.Context) error {
	controllers := []*Controller{}

	cmclient := &kubeclient.KubeClient{
		Resource:     "configmaps",
		GroupVersion: "api/v1",
		Server:       m.Server,
		Token:        m.Token,
		HttpClient:   createHttpClient(m.Insecure),
	}

	var genConfigs []string
	if m.SMIEnabled {
		for _, c := range m.ConfigMaps {
			genConfigs = append(genConfigs, c + "-gen")
		}
	} else {
		genConfigs = m.ConfigMaps
	}
	configmaps := &Controller{
		updated:   make(chan string),
		namespace: m.Namespace,
		client:    cmclient,
		reconciler: &reconciler.ConfigmapReconciler{
			Client:    cmclient,
			Namespace: m.Namespace,
			OutputDir: m.OutputDir,
		},
		resourceNames: genConfigs,
	}

	if m.SMIEnabled {
		if len(m.ConfigMaps) != len(m.TrafficSplits) {
			return fmt.Errorf("mismatching number of configmaps and trafficsplits")
		}
		tsToConfigs := map[string]string{}
		for i := range m.ConfigMaps {
			tsToConfigs[m.TrafficSplits[i]] = m.ConfigMaps[i]
		}
		tsclient := &kubeclient.KubeClient{
			Resource:     "trafficsplits",
			GroupVersion: "apis/split.smi-spec.io/v1alpha2",
			Server:       m.Server,
			Token:        m.Token,
			HttpClient:   createHttpClient(m.Insecure),
		}
		trafficsplits := &Controller{
			updated:   make(chan string),
			namespace: m.Namespace,
			client:    tsclient,
			reconciler: &reconciler.TrafficSplitReconciler{
				TrafficSplits: tsclient,
				ConfigMaps:    cmclient,
				TsToConfigs: tsToConfigs,
				Namespace:     m.Namespace,
			},
			resourceNames: m.TrafficSplits,
		}

		// trafficsplits controller needs to be before configmaps controller
		// so that the former can create <configmap-name>-gen from <confgimap-name> that is rendered to the local fs
		controllers = append(controllers, trafficsplits)
	}
	controllers = append(controllers, configmaps)

	if m.Onetime {
		for i := range controllers {
			c := controllers[i]
			if err := c.Once(); err != nil {
				return err
			}
		}
		return nil
	}

	log.Println("Starting crossover...")

	var wg sync.WaitGroup

	for i := range controllers {
		c := controllers[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.Poll(ctx, m.ConfigMaps, m.SyncInterval); err != nil {
				log.Fatalf("%v", err)
			}
		}()
	}

	if m.Watch {
		for i := range controllers {
			c := controllers[i]
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := c.Watch(ctx); err != nil {
					log.Fatalf("Watch stopped due to error: %v", err)
				}
				log.Printf("Watch stopped normally.")
			}()
		}
	}

	for i := range controllers {
		c := controllers[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.Run(ctx); err != nil {
				log.Fatalf("Run loop stopped due to error: %v", err)
			}
			log.Printf("Run loop stopped normally.")
		}()
	}

	wg.Wait()

	return nil
}

func createHttpClient(insecure bool) *http.Client {
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
			InsecureSkipVerify: insecure,
		},
	}
	client := &http.Client{
		Transport: transport,
	}
	return client
}
