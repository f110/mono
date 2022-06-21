package repoindexer

import (
	"os"
	"path/filepath"

	"go.f110.dev/xerrors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func newK8sClient(dev bool) (kubernetes.Interface, *rest.Config, error) {
	var k8sConf *rest.Config
	if dev {
		h, err := os.UserHomeDir()
		if err != nil {
			return nil, nil, xerrors.WithStack(err)
		}
		kubeconfigPath := filepath.Join(h, ".kube/config")
		cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, nil, xerrors.WithStack(err)
		}
		k8sConf = cfg
	} else {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return nil, nil, xerrors.WithStack(err)
		}
		k8sConf = cfg
	}
	k8sClient, err := kubernetes.NewForConfig(k8sConf)
	if err != nil {
		return nil, nil, xerrors.WithStack(err)
	}

	return k8sClient, k8sConf, nil
}
