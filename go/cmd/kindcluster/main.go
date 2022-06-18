package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
	goyaml "gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	configv1alpha4 "sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
)

func main() {
	rootCmd := &cobra.Command{
		Use: "kindcluster",
	}
	createCmd(rootCmd)
	deleteCmd(rootCmd)
	applyCmd(rootCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}

func createCmd(rootCmd *cobra.Command) {
	var name, kind, k8sVersion, manifest string
	workerNum := 1
	cmd := &cobra.Command{
		Use: "create",
		RunE: func(_ *cobra.Command, _ []string) error {
			cluster, err := NewCluster(kind, name, "")
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}
			if err := cluster.Create(k8sVersion, workerNum); err != nil {
				return xerrors.Errorf(": %w", err)
			}
			if manifest != "" {
				if err := cluster.Apply(manifest, "kindcluster"); err != nil {
					return xerrors.Errorf(": %w", err)
				}
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "kindcluster", "Name of cluster")
	cmd.Flags().StringVar(&kind, "kind", "", "The path of kind")
	cmd.Flags().StringVar(&k8sVersion, "k8s-version", "v1.20.2", "Cluster version")
	cmd.Flags().IntVar(&workerNum, "worker-num", workerNum, "The number of worker")
	cmd.Flags().StringVar(&manifest, "manifest", "", "The path of default manifest")

	rootCmd.AddCommand(cmd)
}

func deleteCmd(rootCmd *cobra.Command) {
	var name, kind string
	cmd := &cobra.Command{
		Use: "delete",
		RunE: func(_ *cobra.Command, _ []string) error {
			cluster, err := NewCluster(kind, name, "")
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}
			if err := cluster.Delete(); err != nil {
				return xerrors.Errorf(": %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "kindcluster", "Name of cluster")
	cmd.Flags().StringVar(&kind, "kind", "", "The path of kind")

	rootCmd.AddCommand(cmd)
}

func applyCmd(rootCmd *cobra.Command) {
	var name, kind, manifest string
	cmd := &cobra.Command{
		Use: "apply",
		RunE: func(_ *cobra.Command, _ []string) error {
			cluster, err := NewCluster(kind, name, "")
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}
			if err := cluster.Apply(manifest, "kindcluster"); err != nil {
				return xerrors.Errorf(": %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "kindcluster", "")
	cmd.Flags().StringVar(&kind, "kind", "", "")
	cmd.Flags().StringVar(&manifest, "manifest", "", "")

	rootCmd.AddCommand(cmd)
}

var KindNodeImageHash = map[string]string{
	"v1.23.4":  "0e34f0d0fd448aa2f2819cfd74e99fe5793a6e4938b328f657c8e3f81ee0dfb9",
	"v1.23.0":  "49824ab1727c04e56a21a5d8372a402fcd32ea51ac96a2706a12af38934f81ac",
	"v1.22.7":  "1dfd72d193bf7da64765fd2f2898f78663b9ba366c2aa74be1fd7498a1873166",
	"v1.22.0":  "b8bda84bb3a190e6e028b1760d277454a72267a5454b57db34437c34a588d047",
	"v1.21.10": "84709f09756ba4f863769bdcabe5edafc2ada72d3c8c44d6515fc581b66b029c",
	"v1.21.1":  "69860bda5563ac81e3c0057d654b5253219618a22ec3a346306239bba8cfa1a6",
	"v1.20.15": "393bb9096c6c4d723bb17bceb0896407d7db581532d11ea2839c80b28e5d8deb",
	"v1.20.7":  "cbeaf907fc78ac97ce7b625e4bf0de16e3ea725daf6b04f930bd14c67c671ff9",
	"v1.20.2":  "8f7ea6e7642c0da54f04a7ee10431549c0257315b3a634f6ef2fecaaedb19bab",
	"v1.19.16": "81f552397c1e6c1f293f967ecb1344d8857613fb978f963c30e907c32f598467",
	"v1.19.11": "07db187ae84b4b7de440a73886f008cf903fcf5764ba8106a9fd5243d6f32729",
	"v1.19.7":  "a70639454e97a4b733f9d9b67e12c01f6b0297449d5b9cbbef87473458e26dca",
	"v1.19.3":  "e1ac015e061da4b931cc4f693e22d7bc1110f031faf7b2af4c4fefac9e65565d",
	"v1.19.1":  "98cf5288864662e37115e362b23e4369c8c4a408f99cbc06e58ac30ddc721600",
	"v1.19.0":  "3b0289b2d1bab2cb9108645a006939d2f447a10ad2bb21919c332d06b548bbc6",
	"v1.18.20": "e3dca5e16116d11363e31639640042a9b1bd2c90f85717a7fc66be34089a8169",
	"v1.18.19": "7af1492e19b3192a79f606e43c35fb741e520d195f96399284515f077b3b622c",
	"v1.18.15": "5c1b980c4d0e0e8e7eb9f36f7df525d079a96169c8a8f20d8bd108c0d0889cc4",
	"v1.18.8":  "f4bcc97a0ad6e7abaf3f643d890add7efe6ee4ab90baeb374b4f41a4c95567eb",
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

func (c *Cluster) IsExist(name string) (bool, error) {
	cmd := exec.CommandContext(context.TODO(), c.kind, "get", "clusters")
	buf, err := cmd.CombinedOutput()
	if err != nil {
		return false, xerrors.Errorf(": %w", err)
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

func (c *Cluster) Create(clusterVersion string, workerNum int) error {
	kindConfFile, err := os.CreateTemp("", "kind.config.yaml")
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	defer os.Remove(kindConfFile.Name())

	imageHash, ok := KindNodeImageHash[clusterVersion]
	if !ok {
		return xerrors.Errorf("Not supported k8s version: %s", clusterVersion)
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
	// If workerNum equals 1 is intended to create a single node cluster.
	// In that case, We shouldn't add Node.
	if workerNum > 2 {
		for i := 0; i < workerNum; i++ {
			clusterConf.Nodes = append(clusterConf.Nodes,
				configv1alpha4.Node{Role: configv1alpha4.WorkerRole, Image: image})
		}
	}
	if buf, err := goyaml.Marshal(clusterConf); err != nil {
		return xerrors.Errorf(": %w", err)
	} else {
		if _, err := kindConfFile.Write(buf); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	if c.kubeConfig == "" {
		f, err := os.CreateTemp("", "config")
		if err != nil {
			return err
		}
		c.kubeConfig = f.Name()
		c.tmpKubeConfig = true
	}
	cmd := exec.CommandContext(
		context.TODO(),
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

func (c *Cluster) Delete() error {
	found, err := c.IsExist(c.name)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if !found {
		return nil
	}

	if c.tmpKubeConfig {
		defer os.Remove(c.kubeConfig)
	}
	cmd := exec.CommandContext(context.TODO(), c.kind, "delete", "cluster", "--name", c.name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (c *Cluster) RESTConfig() (*rest.Config, error) {
	if exist, err := c.IsExist(c.name); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	} else if !exist {
		return nil, xerrors.New("The cluster is not created yet")
	}
	if c.kubeConfig == "" {
		kubeConf, err := os.CreateTemp("", "kubeconfig")
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		cmd := exec.CommandContext(
			context.TODO(),
			c.kind, "export", "kubeconfig",
			"--kubeconfig", kubeConf.Name(),
			"--name", c.name,
		)
		if err := cmd.Run(); err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		c.kubeConfig = kubeConf.Name()
		defer func() {
			os.Remove(kubeConf.Name())
			c.kubeConfig = ""
		}()
	}

	cfg, err := clientcmd.LoadFromFile(c.kubeConfig)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
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
		return nil, xerrors.Errorf(": %w", err)
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
		return xerrors.Errorf(": %w", err)
	}

	return PollImmediate(ctx, 1*time.Second, 3*time.Minute, func(ctx context.Context) (done bool, err error) {
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
	buf, err := os.ReadFile(f)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	cfg, err := c.RESTConfig()
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	if err := ApplyManifestFromString(cfg, string(buf), fieldManager); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func LoadUnstructuredFromString(manifest string) ([]*unstructured.Unstructured, error) {
	objs := make([]*unstructured.Unstructured, 0)
	d := yaml.NewYAMLOrJSONDecoder(strings.NewReader(manifest), 4096)
	for {
		ext := runtime.RawExtension{}
		if err := d.Decode(&ext); err != nil {
			if err == io.EOF {
				break
			}
			return nil, xerrors.Errorf(": %w", err)
		}
		if len(ext.Raw) == 0 {
			continue
		}

		obj, _, err := unstructured.UnstructuredJSONScheme.Decode(ext.Raw, nil, nil)
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		objs = append(objs, obj.(*unstructured.Unstructured))
	}

	return objs, nil
}

type Objects []*unstructured.Unstructured

func ApplyManifestFromString(cfg *rest.Config, manifest, fieldManager string) error {
	objs, err := LoadUnstructuredFromString(manifest)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	err = Objects(objs).Apply(cfg, fieldManager)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	return nil
}

func (k Objects) Apply(cfg *rest.Config, fieldManager string) error {
	disClient, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	_, apiResourcesList, err := disClient.ServerGroupsAndResources()
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	for _, obj := range k {
		gv := obj.GroupVersionKind().GroupVersion()

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
			return xerrors.Errorf(": %w", err)
		}

		var apiResource *metav1.APIResource
		for _, v := range apiResourcesList {
			if v.GroupVersion == gv.String() {
				for _, v := range v.APIResources {
					if v.Kind == obj.GroupVersionKind().Kind && !strings.HasSuffix(v.Name, "/status") {
						apiResource = &v
						break
					}
				}
			}
		}
		if apiResource == nil {
			continue
		}

		err = PollImmediate(context.TODO(), 5*time.Second, 30*time.Second, func(ctx context.Context) (bool, error) {
			req := client.Patch(types.ApplyPatchType)
			data, err := runtime.Encode(unstructured.UnstructuredJSONScheme, obj)
			if err != nil {
				return true, nil
			}
			force := true
			res := req.
				NamespaceIfScoped(obj.GetNamespace(), apiResource.Namespaced).
				Resource(apiResource.Name).
				Name(obj.GetName()).
				VersionedParams(&metav1.PatchOptions{FieldManager: fieldManager, Force: &force}, metav1.ParameterCodec).
				Body(data).
				Do(ctx)
			if err := res.Error(); err != nil {
				switch {
				case apierrors.IsAlreadyExists(err):
					return false, nil
				case apierrors.IsInternalError(err):
					fmt.Fprintf(os.Stderr, "%s %v\n", obj.GetName(), err)
					return false, nil
				default:
					fmt.Fprintf(os.Stderr, "Applying %s has error. don't retry: %v\n", obj.GetName(), err)
				}

				return true, nil
			}
			return true, nil
		})
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	return nil
}

func Poll(ctx context.Context, interval, timeout time.Duration, fn func(ctx context.Context) (done bool, err error)) error {
	tick := time.NewTicker(interval)
	defer tick.Stop()

	limit := time.After(timeout)
	for {
		select {
		case <-tick.C:
			fnCtx, cancel := context.WithTimeout(ctx, interval)
			done, err := fn(fnCtx)
			cancel()
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}
			if done {
				return nil
			}
		case <-limit:
			return errors.New("timed out")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func PollImmediate(ctx context.Context, interval, timeout time.Duration, fn func(ctx context.Context) (done bool, err error)) error {
	fnCtx, cancel := context.WithTimeout(ctx, interval)
	done, err := fn(fnCtx)
	cancel()
	if done {
		return nil
	}
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return Poll(ctx, interval, timeout, fn)
}
