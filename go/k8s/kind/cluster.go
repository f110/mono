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
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	goyaml "gopkg.in/yaml.v3"
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

	"go.f110.dev/mono/go/logger"
)

var NodeImageHash = map[string]string{
	"v1.34.0":  "7416a61b42b1662ca6ca89f02028ac133a309a2a30ba309614e8ec94d976dc5a",
	"v1.33.4":  "25a6018e48dfcaee478f4a59af81157a437f15e6e140bf103f85a2e7cd0cbbf2",
	"v1.33.1":  "050072256b9a903bd914c0b2866828150cb229cea0efe5892e2b644d5dd3b34f",
	"v1.32.8":  "abd489f042d2b644e2d033f5c2d900bc707798d075e8186cb65e3f1367a9d5a1",
	"v1.32.5":  "e3b2327e3a5ab8c76f5ece68936e4cafaa82edf58486b769727ab0b3b97a5b0d",
	"v1.32.2":  "f226345927d7e348497136874b6d207e0b32cc52154ad8323129352923a3142f",
	"v1.32.0":  "c48c62eac5da28cdadcf560d1d8616cfa6783b58f0d94cf63ad1bf49600cb027",
	"v1.31.12": "0f5cc49c5e73c0c2bb6e2df56e7df189240d83cf94edfa30946482eb08ec57d2",
	"v1.31.9":  "b94a3a6c06198d17f59cca8c6f486236fa05e2fb359cbd75dabbfc348a10b211",
	"v1.31.6":  "28b7cbb993dfe093c76641a0c95807637213c9109b761f1d422c2400e22b8e87",
	"v1.31.4":  "2cb39f7295fe7eafee0842b1052a599a4fb0f8bcf3f83d96c7f4864c357c6c30",
	"v1.31.2":  "18fbefc20a7113353c7b75b5c869d7145a6abd6269154825872dc59c1329912e",
	"v1.31.0":  "53df588e04085fd41ae12de0c3fe4c72f7013bba32a20e7325357a1ac94ba865",
	"v1.30.13": "397209b3d947d154f6641f2d0ce8d473732bd91c87d9575ade99049aa33cd648",
	"v1.30.10": "4de75d0e82481ea846c0ed1de86328d821c1e6a6a91ac37bf804e5313670e507",
	"v1.30.8":  "17cd608b3971338d9180b00776cb766c50d0a0b6b904ab4ff52fd3fc5c6369bf",
	"v1.30.6":  "b6d08db72079ba5ae1f4a88a09025c0a904af3b52387643c285442afb05ab994",
	"v1.30.4":  "976ea815844d5fa93be213437e3ff5754cd599b040946b5cca43ca45c2047114",
	"v1.30.3":  "bf91e1ef2f7d92bb7734b2b896b3dddea98f0496b34d96e37dd5d7df331b7e56",
	"v1.30.0":  "047357ac0cfea04663786a612ba1eaba9702bef25227a794b52890dd8bcd692e",
	"v1.29.14": "8703bd94ee24e51b778d5556ae310c6c0fa67d761fae6379c8e0bb480e6fea29",
	"v1.29.12": "62c0672ba99a4afd7396512848d6fc382906b8f33349ae68fb1dbfe549f70dec",
	"v1.29.10": "3b2d8c31753e6c8069d4fc4517264cd20e86fd36220671fb7d0a5855103aa84b",
	"v1.29.8":  "d46b7aa29567e93b27f7531d258c372e829d7224b25e3fc6ffdefed12476d3aa",
	"v1.29.7":  "f70ab5d833fca132a100c1f95490be25d76188b053f49a3c0047ff8812360baf",
	"v1.29.4":  "3abb816a5b1061fb15c6e9e60856ec40d56b7b52bcea5f5f1350bc6e2320b6f8",
	"v1.29.2":  "51a1434a5397193442f0be2a297b488b6c919ce8a3931be0ce822606ea5ca245",
	"v1.29.1":  "a0cc28af37cf39b019e2b448c54d1a3f789de32536cb5a5db61a49623e527144",
	"v1.28.15": "a7c05c7ae043a0b8c818f5a06188bc2c4098f6cb59ca7d1856df00375d839251",
	"v1.28.13": "45d319897776e11167e4698f6b14938eb4d52eb381d9e3d7a9086c16c69a8110",
	"v1.28.12": "fa0e48b1e83bb8688a5724aa7eebffbd6337abd7909ad089a2700bf08c30c6ea",
	"v1.28.9":  "dca54bc6a6079dd34699d53d7d4ffa2e853e46a20cd12d619a09207e35300bd0",
	"v1.28.7":  "9bc6c451a289cf96ad0bbaf33d416901de6fd632415b076ab05f5fa7e4f65c58",
	"v1.28.6":  "b7e1cf6b2b729f604133c667a6be8aab6f4dde5bb042c1891ae248d9154f665b",
	"v1.27.16": "2d21a61643eafc439905e18705b8186f3296384750a835ad7a005dceb9546d20",
	"v1.27.13": "17439fa5b32290e3ead39ead1250dca1d822d94a10d26f1981756cd51b24b9d8",
	"v1.27.11": "681253009e68069b8e01aad36a1e0fa8cf18bb0ab3e5c4069b2e65cafdd70843",
	"v1.27.10": "3700c811144e24a6c6181065265f69b9bf0b437c45741017182d7c82b908918f",
	"v1.27.3":  "3966ac761ae0136263ffdb6cfd4db23ef8a83cba8a463690e98317add2c9ba72",
	"v1.27.1":  "9915f5629ef4d29f35b478e819249e89cfaffcbfeebda4324e5c01d53d937b09",
	"v1.27.0":  "c6b22e613523b1af67d4bc8a0c38a4c3ea3a2b8fbc5b367ae36345c9cb844518",
	"v1.26.15": "c79602a44b4056d7e48dc20f7504350f1e87530fe953428b792def00bc1076dd",
	"v1.26.14": "5d548739ddef37b9318c70cb977f57bf3e5015e4552be4e27e57280a8cbb8e4f",
	"v1.26.13": "15ae92d507b7d4aec6e8920d358fc63d3b980493db191d7327541fbaaed1f789",
	"v1.26.6":  "6e2d8b28a5b601defe327b98bd1c2d1930b49e5d8c512e1895099e4504007adb",
	"v1.26.3":  "61b92f38dff6ccc29969e7aa154d34e38b89443af1a2c14e6cfbd2df6419c66f",
	"v1.26.0":  "691e24bd2417609db7e589e1a479b902d2e209892a10ce375fab60a8407c7352",
	"v1.25.16": "6110314339b3b44d10da7d27881849a87e092124afab5956f2e10ecdb463b025",
	"v1.25.11": "227fa11ce74ea76a0474eeefb84cb75d8dad1b08638371ecf0e86259b35be0c8",
	"v1.25.8":  "00d3f5314cc35327706776e95b2f8e504198ce59ac545d0200a89e69fce10b7f",
	"v1.25.3":  "f52781bc0d7a19fb6c405c2af83abfeb311f130707a0e219175677e366cc45d1",
	"v1.25.2":  "9be91e9e9cdf116809841fc77ebdb8845443c4c72fe5218f3ae9eb57fdb4bace",
	"v1.24.17": "ea292d57ec5dd0e2f3f5a2d77efa246ac883c051ff80e887109fabefbd3125c7",
	"v1.24.7":  "577c630ce8e509131eab1aea12c022190978dd2f745aac5eb1fe65c0807eb315",
	"v1.24.6":  "97e8d00bc37a7598a0b32d1fabd155a96355c49fa0d4d4790aab0f161bf31be1",
	"v1.24.0":  "0866296e693efe1fed79d5e6c7af8df71fc73ae45e3679af05342239cdc5bc8e",
	"v1.23.17": "fbb92ac580fce498473762419df27fa8664dbaa1c5a361b5957e123b4035bdcf",
	"v1.23.13": "ef453bb7c79f0e3caba88d2067d4196f427794086a7d0df8df4f019d5e336b61",
	"v1.23.12": "9402cf1330bbd3a0d097d2033fa489b2abe40d479cc5ef47d0b6a6960613148a",
	"v1.23.6":  "b1fa224cc6c7ff32455e0b1fd9cbfd3d3bc87ecaa8fcb06961ed1afb3db0f9ae",
	"v1.23.4":  "0e34f0d0fd448aa2f2819cfd74e99fe5793a6e4938b328f657c8e3f81ee0dfb9",
	"v1.23.0":  "49824ab1727c04e56a21a5d8372a402fcd32ea51ac96a2706a12af38934f81ac",
	"v1.22.15": "7d9708c4b0873f0fe2e171e2b1b7f45ae89482617778c1c875f1053d4cef2e41",
	"v1.22.9":  "8135260b959dfe320206eb36b3aeda9cffcb262f4b44cda6b33f7bb73f453105",
	"v1.22.7":  "1dfd72d193bf7da64765fd2f2898f78663b9ba366c2aa74be1fd7498a1873166",
	"v1.22.0":  "b8bda84bb3a190e6e028b1760d277454a72267a5454b57db34437c34a588d047",
	"v1.21.14": "9d9eb5fb26b4fbc0c6d95fa8c790414f9750dd583f5d7cee45d92e8c26670aa1",
	"v1.21.10": "84709f09756ba4f863769bdcabe5edafc2ada72d3c8c44d6515fc581b66b029c",
	"v1.21.1":  "69860bda5563ac81e3c0057d654b5253219618a22ec3a346306239bba8cfa1a6",
	"v1.20.15": "a32bf55309294120616886b5338f95dd98a2f7231519c7dedcec32ba29699394",
	"v1.20.7":  "cbeaf907fc78ac97ce7b625e4bf0de16e3ea725daf6b04f930bd14c67c671ff9",
	"v1.20.2":  "8f7ea6e7642c0da54f04a7ee10431549c0257315b3a634f6ef2fecaaedb19bab",
	"v1.19.16": "476cb3269232888437b61deca013832fee41f9f074f9bed79f57e4280f7c48b7",
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
		return xerrors.Definef("not supported k8s version: %s", clusterVersion).WithStack()
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
		for range workerNum {
			clusterConf.Nodes = append(clusterConf.Nodes,
				configv1alpha4.Node{Role: configv1alpha4.WorkerRole, Image: image})
		}
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

		logger.Log.Info("Load image file", zap.String("tag", v.repoTags))
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
			logger.Log.Info("Set an image tag", zap.String("tag", fmt.Sprintf("%s:%s", image.Repository, image.Tag)), zap.String("node", node))
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
		return nil, err
	} else if !exist {
		return nil, xerrors.New("The cluster is not created yet")
	}
	if c.kubeConfig == "" {
		kubeConf, err := os.CreateTemp("", "kubeconfig")
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

	err = wait.PollImmediate(1*time.Second, 3*time.Minute, func() (done bool, err error) {
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
	if err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func (c *Cluster) Apply(f, fieldManager string) error {
	buf, err := os.ReadFile(f)
	if err != nil {
		return xerrors.WithStack(err)
	}
	cfg, err := c.RESTConfig()
	if err != nil {
		return err
	}

	if err := applyManifestFromString(cfg, string(buf), fieldManager); err != nil {
		return err
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
			fmt.Print(err)
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

type objects []object

func applyManifestFromString(cfg *rest.Config, manifest, fieldManager string) error {
	objs := make(objects, 0)
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

	if err := objs.Apply(cfg, fieldManager); err != nil {
		return err
	}

	return nil
}

func (k objects) Apply(cfg *rest.Config, fieldManager string) error {
	disClient, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return xerrors.WithStack(err)
	}
	_, apiResourcesList, err := disClient.ServerGroupsAndResources()
	if err != nil {
		return xerrors.WithStack(err)
	}

	for _, obj := range k {
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

				return true, nil
			}
			logger.Log.Info("The object was created", zap.String("kind", unstructuredObj.GetKind()), zap.String("name", unstructuredObj.GetName()))
			return true, nil
		})
		if err != nil {
			return xerrors.WithStack(err)
		}
	}
	return nil
}
