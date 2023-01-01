package kind

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"go.f110.dev/xerrors"
	goyaml "gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	configv1alpha4 "sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
)

var NodeImageHash = map[string]string{
	"v1.19.3":  "e1ac015e061da4b931cc4f693e22d7bc1110f031faf7b2af4c4fefac9e65565d",
	"v1.19.1":  "98cf5288864662e37115e362b23e4369c8c4a408f99cbc06e58ac30ddc721600",
	"v1.19.0":  "3b0289b2d1bab2cb9108645a006939d2f447a10ad2bb21919c332d06b548bbc6",
	"v1.18.8":  "f4bcc97a0ad6e7abaf3f643d890add7efe6ee4ab90baeb374b4f41a4c95567eb",
	"v1.17.11": "5240a7a2c34bf241afb54ac05669f8a46661912eab05705d660971eeb12f6555",
}

type Cluster struct {
	kind          string
	name          string
	kubeConfig    string
	tmpKubeConfig bool

	clientset kubernetes.Interface
}

func NewCluster(kind, name, kubeConfig string) (*Cluster, error) {
	_, err := exec.LookPath(kind)
	if err != nil {
		return nil, err
	}

	return &Cluster{kind: kind, name: name, kubeConfig: kubeConfig}, nil
}

func (c *Cluster) IsExist(ctx context.Context, name string) (bool, error) {
	cmd := exec.CommandContext(ctx, c.kind, "get", "clusters")
	buf, err := cmd.CombinedOutput()
	if err != nil {
		return false, xerrors.WithStack(err)
	}
	s := bufio.NewScanner(bytes.NewReader(buf))
	for s.Scan() {
		line := s.Text()
		if line == name {
			return true, nil
		}
	}

	return false, nil
}

func (c *Cluster) Create(ctx context.Context, clusterVersion string, workerNum int) error {
	kindConfFile, err := ioutil.TempFile("", "kind.config.yaml")
	if err != nil {
		return xerrors.WithStack(err)
	}
	defer os.Remove(kindConfFile.Name())

	imageHash, ok := NodeImageHash[clusterVersion]
	if !ok {
		return xerrors.Newf("not supported k8s version: %s", clusterVersion)
	}
	image := fmt.Sprintf("kindest/node:%s@sha256:%s", clusterVersion, imageHash)

	clusterConf := &configv1alpha4.Cluster{
		TypeMeta: configv1alpha4.TypeMeta{
			APIVersion: "kind.x-k8s.io/v1alpha4",
			Kind:       "Cluster",
		},
		Nodes: []configv1alpha4.Node{
			{Role: configv1alpha4.ControlPlaneRole, Image: image},
		},
	}
	for i := 0; i < workerNum; i++ {
		clusterConf.Nodes = append(clusterConf.Nodes,
			configv1alpha4.Node{Role: configv1alpha4.WorkerRole, Image: image})
	}
	if buf, err := goyaml.Marshal(clusterConf); err != nil {
		return xerrors.WithStack(err)
	} else {
		if _, err := kindConfFile.Write(buf); err != nil {
			return xerrors.WithStack(err)
		}
	}

	if c.kubeConfig == "" {
		f, err := ioutil.TempFile("", "config")
		if err != nil {
			return err
		}
		c.kubeConfig = f.Name()
		c.tmpKubeConfig = true
	}
	cmd := exec.CommandContext(
		ctx,
		c.kind, "create", "cluster",
		"--name", c.name,
		"--kubeconfig", c.kubeConfig,
		"--config", kindConfFile.Name(),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func (c *Cluster) KubeConfig() string {
	return c.kubeConfig
}

func (c *Cluster) Delete(ctx context.Context) error {
	found, err := c.IsExist(ctx, c.name)
	if err != nil {
		return xerrors.WithStack(err)
	}
	if !found {
		return nil
	}

	if c.tmpKubeConfig {
		defer os.Remove(c.kubeConfig)
	}
	cmd := exec.CommandContext(ctx, c.kind, "delete", "cluster", "--name", c.name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type ContainerImageFile struct {
	File       string
	Repository string
	Tag        string

	repoTags string
}

type manifest struct {
	RepoTags []string `json:"RepoTags"`
}

func (c *Cluster) LoadImageFiles(ctx context.Context, images ...*ContainerImageFile) error {
	for _, v := range images {
		if err := readImageManifest(v); err != nil {
			return err
		}

		log.Printf("Load image file: %s", v.repoTags)
		cmd := exec.CommandContext(ctx, c.kind, "load", "image-archive", "--name", c.name, v.File)
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	cmd := exec.CommandContext(ctx, c.kind, "get", "nodes", "--name", c.name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	nodes := make([]string, 0)
	s := bufio.NewScanner(bytes.NewReader(out))
	for s.Scan() {
		nodes = append(nodes, s.Text())
	}

	for _, node := range nodes {
		for _, image := range images {
			log.Printf("Set an image tag %s:%s on %s", image.Repository, image.Tag, node)
			cmd = exec.CommandContext(
				ctx,
				"docker", "exec", node,
				"ctr", "-n", "k8s.io",
				"images", "tag",
				"--force",
				"docker.io/"+image.repoTags,
				fmt.Sprintf("%s:%s", image.Repository, image.Tag),
			)
			if err := cmd.Run(); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *Cluster) RESTConfig() (*rest.Config, error) {
	if exist, err := c.IsExist(context.Background(), c.name); err != nil {
		return nil, xerrors.WithStack(err)
	} else if !exist {
		return nil, xerrors.New("The cluster is not created yet")
	}
	if c.kubeConfig == "" {
		kubeConf, err := ioutil.TempFile("", "kubeconfig")
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		cmd := exec.CommandContext(
			context.Background(),
			c.kind, "export", "kubeconfig",
			"--kubeconfig", kubeConf.Name(),
			"--name", c.name,
		)
		if err := cmd.Run(); err != nil {
			return nil, xerrors.WithStack(err)
		}
		c.kubeConfig = kubeConf.Name()
		defer func() {
			os.Remove(kubeConf.Name())
			c.kubeConfig = ""
		}()
	}

	cfg, err := clientcmd.LoadFromFile(c.kubeConfig)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	clientConfig := clientcmd.NewDefaultClientConfig(*cfg, &clientcmd.ConfigOverrides{})
	restCfg, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	return restCfg, nil
}

func (c *Cluster) Clientset() (kubernetes.Interface, error) {
	if c.clientset != nil {
		return c.clientset, nil
	}

	restCfg, err := c.RESTConfig()
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	cs, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, err
	}
	c.clientset = cs

	return cs, nil
}

func (c *Cluster) WaitReady(ctx context.Context) error {
	client, err := c.Clientset()
	if err != nil {
		return xerrors.WithStack(err)
	}

	return wait.PollImmediate(1*time.Second, 3*time.Minute, func() (done bool, err error) {
		nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, err
		}

		notReadyNodes := make(map[string]struct{})
	Nodes:
		for _, v := range nodes.Items {
			for _, c := range v.Status.Conditions {
				if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
					continue Nodes
				}
			}
			notReadyNodes[v.Name] = struct{}{}
		}
		if len(notReadyNodes) == 0 {
			return true, nil
		}

		return false, nil
	})
}

func (c *Cluster) Apply(f, fieldManager string) error {
	buf, err := ioutil.ReadFile(f)
	if err != nil {
		return xerrors.WithStack(err)
	}
	cfg, err := c.RESTConfig()
	if err != nil {
		return xerrors.WithStack(err)
	}

	if err := applyManifestFromString(cfg, string(buf), fieldManager); err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func readImageManifest(image *ContainerImageFile) error {
	f, err := os.Open(image.File)
	if err != nil {
		return err
	}
	r := tar.NewReader(f)
	for {
		hdr, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Name != "manifest.json" {
			// Skip reading if the file name is not manifest.json.
			if _, err := io.Copy(ioutil.Discard, r); err != nil {
				return err
			}
			continue
		}

		manifests := make([]manifest, 0)
		if err := json.NewDecoder(r).Decode(&manifests); err != nil {
			return err
		}
		if len(manifests) == 0 {
			return xerrors.New("manifest.json is empty")
		}
		image.repoTags = manifests[0].RepoTags[0]
	}

	return nil
}

func portForward(ctx context.Context, cfg *rest.Config, client kubernetes.Interface, svc *corev1.Service, port int) (*portforward.PortForwarder, error) {
	selector := labels.SelectorFromSet(svc.Spec.Selector)
	podList, err := client.CoreV1().Pods(svc.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	var pod *corev1.Pod
	for i, v := range podList.Items {
		if v.Status.Phase == corev1.PodRunning {
			pod = &podList.Items[i]
			break
		}
	}
	if pod == nil {
		return nil, xerrors.New("all pods are not running yet")
	}

	req := client.CoreV1().RESTClient().Post().Resource("pods").Namespace(svc.Namespace).Name(pod.Name).SubResource("portforward")
	transport, upgrader, err := spdy.RoundTripperFor(cfg)
	if err != nil {
		return nil, err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, req.URL())

	readyCh := make(chan struct{})
	pf, err := portforward.New(dialer, []string{fmt.Sprintf(":%d", port)}, context.Background().Done(), readyCh, nil, nil)
	if err != nil {
		return nil, err
	}

	go func() {
		err := pf.ForwardPorts()
		if err != nil {
			log.Print(err)
		}
	}()

	select {
	case <-readyCh:
	case <-time.After(5 * time.Second):
		return nil, xerrors.New("timed out")
	}

	return pf, nil
}

type object struct {
	Object           runtime.Object
	GroupVersionKind *schema.GroupVersionKind
	Raw              []byte
}

func applyManifestFromString(cfg *rest.Config, manifest, fieldManager string) error {
	objs := make([]object, 0)
	d := yaml.NewYAMLOrJSONDecoder(strings.NewReader(manifest), 4096)
	for {
		ext := runtime.RawExtension{}
		if err := d.Decode(&ext); err != nil {
			if err == io.EOF {
				break
			}
			return xerrors.WithStack(err)
		}
		if len(ext.Raw) == 0 {
			continue
		}

		obj, gvk, err := unstructured.UnstructuredJSONScheme.Decode(ext.Raw, nil, nil)
		if err != nil {
			return xerrors.WithStack(err)
		}
		objs = append(objs, object{Object: obj, GroupVersionKind: gvk, Raw: ext.Raw})
	}

	disClient, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return xerrors.WithStack(err)
	}
	_, apiResourcesList, err := disClient.ServerGroupsAndResources()
	if err != nil {
		return xerrors.WithStack(err)
	}

	for _, obj := range objs {
		gv := obj.GroupVersionKind.GroupVersion()

		conf := *cfg
		conf.GroupVersion = &gv
		if gv.Group == "" {
			conf.APIPath = "/api"
		} else {
			conf.APIPath = "/apis"
		}
		conf.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
		client, err := rest.RESTClientFor(&conf)
		if err != nil {
			return xerrors.WithStack(err)
		}

		var apiResource *metav1.APIResource
		for _, v := range apiResourcesList {
			if v.GroupVersion == gv.String() {
				for _, v := range v.APIResources {
					if v.Kind == obj.GroupVersionKind.Kind && !strings.HasSuffix(v.Name, "/status") {
						apiResource = &v
						break
					}
				}
			}
		}
		if apiResource == nil {
			continue
		}

		unstructuredObj := obj.Object.(*unstructured.Unstructured)
		method := http.MethodPatch
		err = wait.PollImmediate(5*time.Second, 30*time.Second, func() (bool, error) {
			var req *rest.Request
			switch method {
			case http.MethodPatch:
				req = client.Patch(types.ApplyPatchType)
			default:
				req = client.Post()
			}
			data, err := runtime.Encode(unstructured.UnstructuredJSONScheme, obj.Object)
			if err != nil {
				log.Print(err)
				return true, nil
			}
			force := true
			namespace := unstructuredObj.GetNamespace()
			if apiResource.Namespaced && namespace == "" {
				namespace = metav1.NamespaceDefault
			}
			res := req.
				NamespaceIfScoped(namespace, apiResource.Namespaced).
				Resource(apiResource.Name).
				Name(unstructuredObj.GetName()).
				VersionedParams(&metav1.PatchOptions{FieldManager: fieldManager, Force: &force}, metav1.ParameterCodec).
				Body(data).
				Do(context.TODO())
			if err := res.Error(); err != nil {
				switch {
				case apierrors.IsAlreadyExists(err):
					method = http.MethodPatch
					return false, nil
				case apierrors.IsInternalError(err):
					return false, nil
				}

				log.Printf("%s.%s: %v", unstructuredObj.GetKind(), unstructuredObj.GetName(), err)
				log.Print(string(obj.Raw))
				return true, nil
			}
			log.Printf("%s.%s was created", unstructuredObj.GetKind(), unstructuredObj.GetName())
			return true, nil
		})
		if err != nil {
			return xerrors.WithStack(err)
		}
	}

	return nil
}
