package k8smanifest

import (
	"bytes"
	"io"

	"go.f110.dev/xerrors"
	"k8s.io/apimachinery/pkg/runtime"
	k8sserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"

	k8sclient "go.f110.dev/mono/go/k8s/client"
)

var sc = runtime.NewScheme()

func init() {
	if err := scheme.AddToScheme(sc); err != nil {
		panic(err)
	}
	if err := k8sclient.AddToScheme(sc); err != nil {
		panic(err)
	}
}

type Encoder struct {
	w io.Writer
	s *k8sserializer.Serializer
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		w: w,
		s: k8sserializer.NewSerializerWithOptions(k8sserializer.DefaultMetaFactory, sc, sc, k8sserializer.SerializerOptions{Yaml: true}),
	}
}

func (e *Encoder) Encode(obj runtime.Object) error {
	if obj.GetObjectKind().GroupVersionKind().Kind == "" {
		gvks, unversioned, err := sc.ObjectKinds(obj)
		if err == nil && !unversioned && len(gvks) > 0 {
			obj.GetObjectKind().SetGroupVersionKind(gvks[0])
		}
	}
	if err := e.s.Encode(obj, e.w); err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func Marshal(obj runtime.Object) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := NewEncoder(buf).Encode(obj); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
