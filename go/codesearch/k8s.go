package codesearch

import (
	"os"
	"path/filepath"

	"go.f110.dev/kubeproto/go/k8sclient"
	"go.f110.dev/xerrors"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func newK8sClient(dev bool) (*k8sclient.Set, error) {
	var k8sConf *rest.Config
	if dev {
		h, err := os.UserHomeDir()
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		kubeconfigPath := filepath.Join(h, ".kube/config")
		cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		k8sConf = cfg
	} else {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		k8sConf = cfg
	}
	k8sClient, err := k8sclient.NewSet(k8sConf)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	return k8sClient, nil
}
