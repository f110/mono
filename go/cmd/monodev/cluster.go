package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/ctxutil"
	"go.f110.dev/mono/go/k8s/kind"
	"go.f110.dev/mono/go/logger"
)

const (
	defaultClusterName = "mono"
)

func init() {
	CommandManager.Register(Cluster())
}

func getKubeConfig(kubeConfig string) string {
	if kubeConfig == "" {
		if v := os.Getenv("BUILD_WORKSPACE_DIRECTORY"); v != "" {
			// Running on bazel
			kubeConfig = filepath.Join(v, ".kubeconfig")
		} else {
			cwd, err := os.Getwd()
			if err != nil {
				return ""
			}
			kubeConfig = filepath.Join(cwd, ".kubeconfig")
		}
	}

	return kubeConfig
}

func setupCluster(kindPath, name, k8sVersion string, workerNum int, kubeConfig, manifestFile string) error {
	kubeConfig = getKubeConfig(kubeConfig)
	kindCluster, err := kind.NewCluster(kindPath, name, kubeConfig)
	if err != nil {
		return xerrors.WithStack(err)
	}
	exists, err := kindCluster.IsExist(context.Background(), name)
	if err != nil {
		return xerrors.WithStack(err)
	}

	if !exists {
		if err := kindCluster.Create(context.Background(), k8sVersion, workerNum); err != nil {
			return xerrors.WithStack(err)
		}
	} else {
		logger.Log.Info("Cluster is already exist. Only apply the manifest.")
	}
	ctx, cancelFunc := ctxutil.WithTimeout(context.Background(), 3*time.Minute)
	cancelFunc()
	if err := kindCluster.WaitReady(ctx); err != nil {
		return xerrors.WithStack(err)
	}
	if err := kindCluster.Apply(manifestFile, "monodev"); err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func deleteCluster(kindPath, name, kubeConfig string) error {
	kubeConfig = getKubeConfig(kubeConfig)
	kindCluster, err := kind.NewCluster(kindPath, name, kubeConfig)
	if err != nil {
		return xerrors.WithStack(err)
	}
	if exists, err := kindCluster.IsExist(context.Background(), name); err != nil {
		return xerrors.WithStack(err)
	} else if !exists {
		return nil
	}

	if err := kindCluster.Delete(context.Background()); err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func runContainer(kindPath, name, manifestFile, namespace string, images []string) error {
	kindCluster, err := kind.NewCluster(kindPath, name, "")
	if err != nil {
		return xerrors.WithStack(err)
	}
	if exist, err := kindCluster.IsExist(context.Background(), name); err != nil {
		return xerrors.WithStack(err)
	} else if !exist {
		return xerrors.Define("Cluster does not exist. You create the cluster first.").WithStack()
	}

	for _, imageTagAndFile := range images {
		if !strings.Contains(imageTagAndFile, "=") {
			continue
		}
		s := strings.SplitN(imageTagAndFile, "=", 2)
		if len(s) != 2 {
			continue
		}
		imageName, imageFile := s[0], s[1]
		s = strings.SplitN(imageName, ":", 2)
		if len(s) != 2 {
			continue
		}
		imageRepo, imageTag := s[0], s[1]

		if err := kindCluster.LoadImageFiles(context.Background(), &kind.ContainerImageFile{
			File:       imageFile,
			Repository: imageRepo,
			Tag:        imageTag,
		}); err != nil {
			return xerrors.WithStack(err)
		}
	}

	if err := kindCluster.Apply(manifestFile, "monodev"); err != nil {
		return xerrors.WithStack(err)
	}

	client, err := kindCluster.Clientset()
	if err != nil {
		return xerrors.WithStack(err)
	}

	f, err := os.Open(manifestFile)
	if err != nil {
		return xerrors.WithStack(err)
	}
	d := yaml.NewYAMLOrJSONDecoder(f, 4096)
	restartPods := make([]*corev1.Pod, 0)
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

		obj, _, err := unstructured.UnstructuredJSONScheme.Decode(ext.Raw, nil, nil)
		if err != nil {
			return xerrors.WithStack(err)
		}
		v, ok := obj.(metav1.Type)
		if !ok {
			continue
		}
		switch v.GetKind() {
		case "Deployment":
			deploy := &appsv1.Deployment{}
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(*unstructured.Unstructured).Object, deploy)
			if err != nil {
				return xerrors.WithStack(err)
			}

			ns := deploy.Namespace
			if ns == "" {
				ns = metav1.NamespaceDefault
			}
			deploy, err = client.AppsV1().Deployments(ns).Get(context.Background(), deploy.Name, metav1.GetOptions{})
			if err != nil {
				return xerrors.WithStack(err)
			}

			childPods, err := childPodsOfDeployment(context.Background(), client, deploy)
			if err != nil {
				return xerrors.WithStack(err)
			}
			restartPods = append(restartPods, childPods...)
		case "StatefulSet":
			statefulSet := &appsv1.StatefulSet{}
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(*unstructured.Unstructured).Object, statefulSet)
			if err != nil {
				return xerrors.WithStack(err)
			}

			ns := statefulSet.Namespace
			if ns == "" {
				ns = metav1.NamespaceDefault
			}
			statefulSet, err = client.AppsV1().StatefulSets(ns).Get(context.Background(), statefulSet.Name, metav1.GetOptions{})
			if err != nil {
				return xerrors.WithStack(err)
			}

			childPods, err := childPodsOfStatefulSet(context.Background(), client, statefulSet)
			if err != nil {
				return xerrors.WithStack(err)
			}
			restartPods = append(restartPods, childPods...)
		}
	}

	for _, v := range restartPods {
		logger.Log.Info("Delete Pod", zap.String("name", v.Name))
		err := client.CoreV1().Pods(namespace).Delete(context.Background(), v.Name, metav1.DeleteOptions{})
		if err != nil {
			return xerrors.WithStack(err)
		}
	}

	return nil
}

func childPodsOfDeployment(ctx context.Context, client kubernetes.Interface, deploy *appsv1.Deployment) ([]*corev1.Pod, error) {
	replicaSets, err := client.AppsV1().ReplicaSets(deploy.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	pods, err := client.CoreV1().Pods(deploy.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	childPods := make(map[types.UID]*corev1.Pod)
	for _, rs := range replicaSets.Items {
		if !metav1.IsControlledBy(&rs, deploy) {
			continue
		}

		for _, pod := range pods.Items {
			if metav1.IsControlledBy(&pod, &rs) {
				childPods[pod.UID] = &pod
			}
		}
	}

	result := make([]*corev1.Pod, 0, len(childPods))
	for _, v := range childPods {
		result = append(result, v)
	}

	return result, nil
}

func childPodsOfStatefulSet(ctx context.Context, client kubernetes.Interface, stateful *appsv1.StatefulSet) ([]*corev1.Pod, error) {
	pods, err := client.CoreV1().Pods(stateful.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	childPods := make([]*corev1.Pod, 0)
	for _, pod := range pods.Items {
		if metav1.IsControlledBy(&pod, stateful) {
			childPods = append(childPods, &pod)
		}
	}

	return childPods, nil
}

func Cluster() *cli.Command {
	clusterCmd := &cli.Command{
		Use: "cluster",
	}

	clusterName := ""
	kindPath := ""
	k8sVersion := ""
	kubeConfig := ""
	crdFile := ""
	workerNum := 1
	manifestFile := ""

	createCmd := &cli.Command{
		Use:   "create",
		Short: "Create the cluster by kind",
		Run: func(_ context.Context, _ *cli.Command, _ []string) error {
			return setupCluster(kindPath, clusterName, k8sVersion, workerNum, kubeConfig, manifestFile)
		},
	}
	createCmd.Flags().String("kind", "kind command path").Var(&kindPath)
	createCmd.Flags().String("name", "Cluster name").Var(&clusterName).Default(defaultClusterName)
	createCmd.Flags().String("k8s-version", "Kubernetes version").Var(&k8sVersion)
	createCmd.Flags().String("kubeconfig", "Path to the kubeconfig file. If not specified, will be used default file of kubectl").Var(&kubeConfig)
	createCmd.Flags().String("crd", "Applying manifest file after create the cluster").Var(&crdFile)
	createCmd.Flags().Int("worker-num", "The number of k8s workers").Var(&workerNum).Default(3)
	createCmd.Flags().String("manifest", "A manifest file for the container").Var(&manifestFile)
	clusterCmd.AddCommand(createCmd)

	deleteCmd := &cli.Command{
		Use:   "delete",
		Short: "Delete the cluster",
		Run: func(_ context.Context, _ *cli.Command, _ []string) error {
			return deleteCluster(kindPath, clusterName, kubeConfig)
		},
	}
	deleteCmd.Flags().String("kind", "kind command path").Var(&kindPath)
	deleteCmd.Flags().String("name", "Cluster name").Var(&clusterName).Default(defaultClusterName)
	deleteCmd.Flags().String("kubeconfig", "Path to the kubeconfig file. If not specified, will be used default file of kubectl").Var(&kubeConfig)
	clusterCmd.AddCommand(deleteCmd)

	namespace := ""
	var images []string
	runCmd := &cli.Command{
		Use:   "run",
		Short: "Run the container by manifest",
		Run: func(_ context.Context, _ *cli.Command, _ []string) error {
			return runContainer(kindPath, clusterName, manifestFile, namespace, images)
		},
	}
	runCmd.Flags().String("kind", "kind command path").Var(&kindPath)
	runCmd.Flags().String("name", "Cluster name").Var(&clusterName).Default(defaultClusterName)
	runCmd.Flags().String("manifest", "A manifest file for the container").Var(&manifestFile)
	runCmd.Flags().String("namespace", "Namespace").Var(&namespace).Default(metav1.NamespaceDefault)
	runCmd.Flags().StringArray("images", "Load image and tagging (e.g. --images=quay.io/f110/example:latest=./image.tar)").Var(&images)
	clusterCmd.AddCommand(runCmd)

	return clusterCmd
}
