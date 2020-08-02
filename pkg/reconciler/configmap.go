package reconciler

import (
	"fmt"

	"github.com/mumoshu/crossover/pkg/kubeclient"
	"github.com/mumoshu/crossover/pkg/log"
	"github.com/mumoshu/crossover/pkg/types"
)

type ConfigMap struct {
	ApiVersion string            `json:"apiVersion"`
	Data       map[string]string `json:"data"`
	Kind       string            `json:"kind"`
	ObjectMeta ObjectMeta        `json:"metadata"`
}

type ConfigmapReconciler struct {
	Client    kubeclient.ReadOnlyClient
	Namespace string
	OutputDir string

	log.Logger
}

func (s *ConfigmapReconciler) Reconcile(c string) error {
	s.Infof("Reconciling configmap %s", c)
	cm := ConfigMap{}
	err := s.Client.Get(s.Namespace, c, &cm)
	if err != nil {
		s.Infof("get configmap %s/%s: %v", s.Namespace, c, err)
		return types.ErrNotExist
	}
	if err := newWriter(s.OutputDir, s.Logger).write(cm); err != nil {
		return fmt.Errorf("failed writing %v: %v", cm, err)
	}
	return nil
}
