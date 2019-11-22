package kubeclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/mumoshu/envoy-configmap-loader/pkg/types"
)

type ReadOnlyClient interface {
	Get(namespace, name string, obj interface{}) error
	RetryWatch(ctx context.Context, namespace, name string, updated chan string) error
}

type Client interface {
	ReadOnlyClient

	Create(namespace string, obj interface{}) error
	Replace(namespace, name string, obj interface{}) error
}

type KubeClient struct {
	Resource      string
	Token, Server string
	// api/v1 for configmaps, apis/split.smi-spec.io/v1alpha2 for trafficsplits
	GroupVersion string
	HttpClient   *http.Client
}

var _ ReadOnlyClient = &KubeClient{}
var _ Client = &KubeClient{}

func (tp *KubeClient) Get(namespace, name string, obj interface{}) error {
	u := fmt.Sprintf("%s/%s/namespaces/%s/%s/%s", tp.Server, tp.GroupVersion, namespace, tp.Resource, name)
	client := tp.HttpClient
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return fmt.Errorf("http get request creation: %v", err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", tp.Token))

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http get: %v", err)
	}

	if resp.StatusCode == 404 {
		return types.ErrNotExist
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("non 200 response code: %v: %v", resp.StatusCode, req)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()

	if err := json.Unmarshal(data, obj); err != nil {
		return err
	}

	return nil
}

// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#watch-64
func (tp *KubeClient) RetryWatch(ctx context.Context, namespace, name string, updated chan string) error {
	u := fmt.Sprintf("%s/%s/watch/namespaces/%s/%s/%s", tp.Server, tp.GroupVersion, namespace, tp.Resource, name)
	client := tp.HttpClient

	backoff := 5 * time.Second

WATCHES:
	for {
		names := make(chan string)

		go func() {
			defer close(names)

			log.Printf("Watch starting...")

			req, err := http.NewRequest("GET", u, nil)
			if err != nil {
				log.Printf("Watch failed: %v", fmt.Errorf("http get request creation: %v", err))
				return
			}
			req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", tp.Token))

			resp, err := client.Do(req)
			if err != nil {
				log.Printf("Watch failed: %v", fmt.Errorf("http get: %v", err))
				return
			}

			// Read chunks until error or stop
			for names != nil {
				log.Printf("Watch reading next chunk...")
				evt := map[string]interface{}{}
				if err := json.NewDecoder(resp.Body).Decode(&evt); err != nil {
					log.Printf("Watch failed: %v", fmt.Errorf("json decode: %v", err))
					return
				}
				names <- name
			}

			log.Printf("Sent all chunks.")
		}()

	CHUNK_READS:
		for {
			select {
			case <-ctx.Done():
				names = nil
				log.Printf("Watch cancelled.")
				break WATCHES
			case name, ok := <-names:
				if !ok {
					log.Printf("Watch read all chunks.")
					break CHUNK_READS
				}
				log.Printf("Enqueing %s", name)
				updated <- name
			}
		}

		// Prevent busy loop
		log.Printf("Watch stopped. Retrying in %s", backoff)
		time.Sleep(backoff)
	}

	log.Printf("Watch canceled")

	return nil
}

func (tp *KubeClient) Create(namespace string, obj interface{}) error {
	u := fmt.Sprintf("%s/%s/namespaces/%s/%s", tp.Server, tp.GroupVersion, namespace, tp.Resource)
	client := tp.HttpClient
	req, err := http.NewRequest("POST", u, nil)
	if err != nil {
		return fmt.Errorf("http get request creation: %v", err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", tp.Token))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	bs, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	req.Body = ioutil.NopCloser(bytes.NewReader(bs))

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http get: %v", err)
	}

	if resp.StatusCode == 404 {
		return types.ErrNotExist
	}

	if resp.StatusCode != 201 {
		return fmt.Errorf("non 201 response code: %v: %v", resp.StatusCode, req)
	}

	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()

	return nil
}

func (tp *KubeClient) Replace(namespace, name string, obj interface{}) error {
	u := fmt.Sprintf("%s/%s/namespaces/%s/%s/%s", tp.Server, tp.GroupVersion, namespace, tp.Resource, name)
	client := tp.HttpClient
	req, err := http.NewRequest("PUT", u, nil)
	if err != nil {
		return fmt.Errorf("http get request creation: %v", err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", tp.Token))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	bs, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	req.Body = ioutil.NopCloser(bytes.NewReader(bs))

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http get: %v", err)
	}

	if resp.StatusCode == 404 {
		return types.ErrNotExist
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("non 200 response code: %v: %v", resp.StatusCode, req)
	}

	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()

	return nil
}
