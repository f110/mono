package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	_ "embed"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"go.f110.dev/xerrors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"go.f110.dev/mono/go/logger/slogger"
)

// kwok serves pod logs only for pods that have a matching Logs object, and the
// Logs CRD must exist in the apiserver first: `--enable-crds=Logs` makes kwok
// watch the resource but does not install the CRD definition.
//
//go:embed kwok_logs_crd.yaml
var kwokLogsCRDYAML []byte

var crdGVR = schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}

// kwok emulates a kube-apiserver for local development of the build system.
// A hand-run kube-apiserver (reusing the existing etcd component) accepts the
// Job/Pod/Node objects the builder creates, and kwok stands in for the kubelet
// so that pod logs can be served from fixture files. See docs/build.md.
const (
	kwokAPIServerPort = 6443
	kwokNodePort      = 10250
	kwokNodeName      = "kwok-node"
	kwokNodeIP        = "127.0.0.1"
	// kwokAuthToken is a static bearer token wired into both the apiserver
	// token file and the generated kubeconfig. It only grants access to the
	// throw-away local cluster.
	kwokAuthToken = "monodev-dev-token"
)

func kwokDir() string {
	return filepath.Join(os.Getenv("BUILD_WORKING_DIRECTORY"), ".kwok")
}

func kwokFile(name string) string {
	return filepath.Join(kwokDir(), name)
}

// kwokPKIComponent generates the certificate, service account key, token file
// and kubeconfig that the kube-apiserver and kwok need. All artifacts are
// written under BUILD_WORKING_DIRECTORY/.kwok and reused if already present.
type kwokPKIComponent struct{}

var kwokPKI = &kwokPKIComponent{}

var _ component = &kwokPKIComponent{}

func (c *kwokPKIComponent) GetName() string        { return "kwok-pki" }
func (c *kwokPKIComponent) GetType() componentType { return componentTypeOneshot }
func (c *kwokPKIComponent) GetDeps() []component   { return nil }

func (c *kwokPKIComponent) Run(_ context.Context) {
	if err := os.MkdirAll(filepath.Join(kwokDir(), "apiserver"), 0755); err != nil {
		slogger.Log.Error("Failed to make .kwok directory", slogger.E(err))
		return
	}
	if err := c.generateServingCert(); err != nil {
		slogger.Log.Error("Failed to generate serving certificate", slogger.E(err))
		return
	}
	if err := c.generateServiceAccountKey(); err != nil {
		slogger.Log.Error("Failed to generate service account key", slogger.E(err))
		return
	}
	if err := c.writeTokenFile(); err != nil {
		slogger.Log.Error("Failed to write token file", slogger.E(err))
		return
	}
	if err := c.writeKubeconfig(); err != nil {
		slogger.Log.Error("Failed to write kubeconfig", slogger.E(err))
		return
	}
	slogger.Log.Info("Prepared kwok PKI and kubeconfig", slog.String("dir", kwokDir()))
}

// generateServingCert writes a self-signed certificate used both as kwok's
// kubelet serving certificate and as the CA the apiserver trusts when proxying
// pod log requests to kwok (--kubelet-certificate-authority).
func (c *kwokPKIComponent) generateServingCert() error {
	certPath, keyPath := kwokFile("kwok.crt"), kwokFile("kwok.key")
	if fileExists(certPath) && fileExists(keyPath) {
		return nil
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return xerrors.WithStack(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "kwok"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		IPAddresses:           []net.IP{net.ParseIP(kwokNodeIP)},
		DNSNames:              []string{kwokNodeName, "localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		return xerrors.WithStack(err)
	}
	if err := writePEM(certPath, "CERTIFICATE", der); err != nil {
		return err
	}
	return writePEM(keyPath, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(priv))
}

// generateServiceAccountKey writes an RSA key pair used by the apiserver for
// signing service account tokens (--service-account-signing-key-file /
// --service-account-key-file).
func (c *kwokPKIComponent) generateServiceAccountKey() error {
	keyPath, pubPath := kwokFile("sa.key"), kwokFile("sa.pub")
	if fileExists(keyPath) && fileExists(pubPath) {
		return nil
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return xerrors.WithStack(err)
	}
	if err := writePEM(keyPath, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(priv)); err != nil {
		return err
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return xerrors.WithStack(err)
	}
	return writePEM(pubPath, "PUBLIC KEY", pubDER)
}

func (c *kwokPKIComponent) writeTokenFile() error {
	// Format: token,user,uid,"group1,group2"
	content := fmt.Sprintf("%s,dev,dev,\"system:masters\"\n", kwokAuthToken)
	return os.WriteFile(kwokFile("token.csv"), []byte(content), 0600)
}

func (c *kwokPKIComponent) writeKubeconfig() error {
	cfg := clientcmdapi.NewConfig()
	cfg.Clusters["kwok"] = &clientcmdapi.Cluster{
		Server:                fmt.Sprintf("https://127.0.0.1:%d", kwokAPIServerPort),
		InsecureSkipTLSVerify: true,
	}
	cfg.AuthInfos["dev"] = &clientcmdapi.AuthInfo{Token: kwokAuthToken}
	cfg.Contexts["kwok"] = &clientcmdapi.Context{Cluster: "kwok", AuthInfo: "dev", Namespace: "default"}
	cfg.CurrentContext = "kwok"
	return xerrors.WithStack(clientcmd.WriteToFile(*cfg, kwokFile("kubeconfig")))
}

// kubeAPIServer runs a bare kube-apiserver backed by the existing etcd
// component. The kubeconfig written by kwokPKI points here.
var kubeAPIServer = &simpleCommandComponent{
	Name: "kube-apiserver",
	Args: []string{
		"--etcd-servers=http://127.0.0.1:2379",
		fmt.Sprintf("--secure-port=%d", kwokAPIServerPort),
		"--cert-dir=" + kwokFile("apiserver"),
		"--service-account-issuer=https://kubernetes.default.svc",
		"--service-account-key-file=" + kwokFile("sa.pub"),
		"--service-account-signing-key-file=" + kwokFile("sa.key"),
		"--api-audiences=https://kubernetes.default.svc",
		"--token-auth-file=" + kwokFile("token.csv"),
		"--authorization-mode=AlwaysAllow",
		"--kubelet-certificate-authority=" + kwokFile("kwok.crt"),
		"--kubelet-preferred-address-types=InternalIP",
		"--service-cluster-ip-range=10.0.0.0/24",
		"--disable-admission-plugins=ServiceAccount",
	},
	Ports:         ports{{Number: kwokAPIServerPort}},
	VerboseOutput: true,
	Deps:          []component{etcd, kwokPKI},
}

// kwokLogsCRDComponent installs the kwok Logs CRD (required before kwok can
// serve pod logs from Logs objects).
type kwokLogsCRDComponent struct{}

var kwokLogsCRD = &kwokLogsCRDComponent{}

var _ component = &kwokLogsCRDComponent{}

func (c *kwokLogsCRDComponent) GetName() string        { return "kwok-logs-crd" }
func (c *kwokLogsCRDComponent) GetType() componentType { return componentTypeOneshot }
func (c *kwokLogsCRDComponent) GetDeps() []component    { return []component{kubeAPIServer} }

func (c *kwokLogsCRDComponent) Run(ctx context.Context) {
	restCfg, err := clientcmd.BuildConfigFromFlags("", kwokFile("kubeconfig"))
	if err != nil {
		slogger.Log.Error("Failed to build config from kubeconfig", slogger.E(err))
		return
	}
	dynClient, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		slogger.Log.Error("Failed to create dynamic client", slogger.E(err))
		return
	}
	obj := &unstructured.Unstructured{}
	if err := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(kwokLogsCRDYAML), 4096).Decode(obj); err != nil {
		slogger.Log.Error("Failed to decode kwok Logs CRD", slogger.E(err))
		return
	}

	deadline := time.Now().Add(30 * time.Second)
	for {
		_, err = dynClient.Resource(crdGVR).Create(ctx, obj, metav1.CreateOptions{})
		if err == nil || apierrors.IsAlreadyExists(err) {
			slogger.Log.Info("kwok Logs CRD is applied")
			return
		}
		if time.Now().After(deadline) {
			slogger.Log.Error("Failed to apply kwok Logs CRD", slogger.E(err))
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// kwok simulates the kubelet: it manages the fake node and serves pod logs
// (via the Logs CRD) so that the builder's postProcess can fetch fixture logs.
var kwok = &simpleCommandComponent{
	Name: "kwok",
	Args: []string{
		"--kubeconfig=" + kwokFile("kubeconfig"),
		"--manage-all-nodes=true",
		"--node-ip=" + kwokNodeIP,
		fmt.Sprintf("--node-port=%d", kwokNodePort),
		"--tls-cert-file=" + kwokFile("kwok.crt"),
		"--tls-private-key-file=" + kwokFile("kwok.key"),
		"--enable-crds=Logs",
	},
	Ports:         ports{{Number: kwokNodePort}},
	VerboseOutput: true,
	Deps:          []component{kubeAPIServer, kwokLogsCRD},
}

// kwokNodeComponent registers the single fake node once kwok is up.
type kwokNodeComponent struct{}

var kwokNode = &kwokNodeComponent{}

var _ component = &kwokNodeComponent{}

func (c *kwokNodeComponent) GetName() string        { return "kwok-node" }
func (c *kwokNodeComponent) GetType() componentType { return componentTypeOneshot }
func (c *kwokNodeComponent) GetDeps() []component   { return []component{kwok} }

func (c *kwokNodeComponent) Run(ctx context.Context) {
	restCfg, err := clientcmd.BuildConfigFromFlags("", kwokFile("kubeconfig"))
	if err != nil {
		slogger.Log.Error("Failed to build config from kubeconfig", slogger.E(err))
		return
	}
	client, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		slogger.Log.Error("Failed to create clientset", slogger.E(err))
		return
	}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: kwokNodeName,
			Labels: map[string]string{
				"type":                   "kwok",
				"kubernetes.io/hostname": kwokNodeName,
				"kubernetes.io/os":       "linux",
				"kubernetes.io/arch":     "amd64",
			},
			Annotations: map[string]string{
				"kwok.x-k8s.io/node":           "fake",
				"node.alpha.kubernetes.io/ttl": "0",
			},
		},
	}

	deadline := time.Now().Add(30 * time.Second)
	for {
		_, err = client.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})
		if err == nil || apierrors.IsAlreadyExists(err) {
			slogger.Log.Info("kwok node is ready", slog.String("name", kwokNodeName))
			return
		}
		if time.Now().After(deadline) {
			slogger.Log.Error("Failed to create kwok node", slogger.E(err))
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func writePEM(path, typ string, der []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return xerrors.WithStack(err)
	}
	defer f.Close()
	if err := pem.Encode(f, &pem.Block{Type: typ, Bytes: der}); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}
