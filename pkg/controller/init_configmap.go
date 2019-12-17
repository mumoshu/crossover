package controller

import (
	"github.com/mumoshu/crossover/pkg/kubeclient"
	"github.com/mumoshu/crossover/pkg/reconciler"
	"github.com/mumoshu/crossover/pkg/types"
)

func (m *Manager) InitConfigMap(ns, src, dst string, cmclient *kubeclient.KubeClient) error {
	srcCm := reconciler.ConfigMap{}
	dstCm := reconciler.ConfigMap{}

	if err := cmclient.Get(ns, src, &srcCm); err != nil {
		return err
	}

	if err := cmclient.Get(ns, dst, &dstCm); err != nil {
		if err == types.ErrNotExist {
			cm := srcCm
			delete(cm.ObjectMeta.Annotations, "kubectl.kubernetes.io/last-applied-configuration")
			cm.ObjectMeta.Name = dst
			cm.ObjectMeta.ResourceVersion = ""
			if err := cmclient.Create(ns, cm); err != nil {
				return err
			}
			return nil
		}
		return err
	}
	return nil
}
