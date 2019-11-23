package kubeclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/mumoshu/crossover/pkg/types"
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

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode == 404 {
		log.Printf("Get %s/%s: %s", namespace, name, data)
		return types.ErrNotExist
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("non 200 response code: %v: %v", resp.StatusCode, req)
	}

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

			scanner := bufio.NewScanner(resp.Body)

			// Read chunks until error or stop
			for names != nil  && scanner.Scan() {
				log.Printf("Watch reading next chunk...")
				evt := map[string]interface{}{}
				body := scanner.Bytes()
				if err := json.Unmarshal(body, &evt); err != nil {
					log.Printf("Watch failed: %s: parsing %s: %v", u, body, err)
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

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode == 404 {
		log.Printf("Create %s/%s: %s", namespace, tp.Resource, body)
		return types.ErrNotExist
	}

	if resp.StatusCode != 201 {
		return fmt.Errorf("non 201 response code: %v: %v", resp.StatusCode, req)
	}

	return nil
}

func (tp *KubeClient) Replace(namespace, name string, obj interface{}) error {
	u := fmt.Sprintf("%s/%s/namespaces/%s/%s/%s", tp.Server, tp.GroupVersion, namespace, tp.Resource, name)
	client := tp.HttpClient
	req, err := http.NewRequest("PUT", u, nil)
	if err != nil {
		return fmt.Errorf("http put request creation: %v", err)
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
		return fmt.Errorf("http put: %v", err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode == 404 {
		log.Printf("Replace %s/%s: %s", namespace, tp.Resource, body)
		return types.ErrNotExist
	}

	// 409 CONFLICT here mean that another crossover sidecar has successfully updated the resource i.e. the configmap
	// is already up-to-date, that we don't need to retry it now.
	if resp.StatusCode == 409 {
		log.Printf("Replace %s/%s: %s", namespace, tp.Resource, body)
		return nil
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("non 200 response code: %v: %v", resp.StatusCode, req)
	}

	return nil
}
