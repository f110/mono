package logger

import (
	"fmt"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

func KubernetesObject(key string, obj runtime.Object) zap.Field {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return zap.Skip()
	}
	return zap.String(key, fmt.Sprintf("%s/%s", accessor.GetNamespace(), accessor.GetName()))
}
