package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/k8s/kind"
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
	var name, kindPath, k8sVersion, kubeConfig, manifest string
	workerNum := 1
	cmd := &cobra.Command{
		Use: "create",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cluster, err := kind.NewCluster(kindPath, name, getKubeConfig(kubeConfig))
			if err != nil {
				return err
			}
			exists, err := cluster.IsExist(cmd.Context(), name)
			if err != nil {
				return err
			}

			if !exists {
				if err := cluster.Create(cmd.Context(), k8sVersion, workerNum); err != nil {
					return err
				}
			} else {
				fmt.Println("Cluster is already exist. Only apply the manifest.")
			}

			ctx, cancelFunc := context.WithTimeout(cmd.Context(), 3*time.Minute)
			defer cancelFunc()
			if err := cluster.WaitReady(ctx); err != nil {
				return err
			}

			if manifest != "" {
				if err := cluster.Apply(manifest, "kindcluster"); err != nil {
					return err
				}
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "kindcluster", "Name of cluster")
	cmd.Flags().StringVar(&kindPath, "kind", "", "The path of kind")
	cmd.Flags().StringVar(&k8sVersion, "k8s-version", "", "Cluster version")
	cmd.Flags().StringVar(&kubeConfig, "kubeconfig", "", "A path to the kubeconfig file. If not specified, will be used default file of kubectl")
	cmd.Flags().IntVar(&workerNum, "worker-num", workerNum, "The number of worker")
	cmd.Flags().StringVar(&manifest, "manifest", "", "The path of default manifest")

	rootCmd.AddCommand(cmd)
}

func deleteCmd(rootCmd *cobra.Command) {
	var name, kindPath, kubeConfig string
	cmd := &cobra.Command{
		Use: "delete",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cluster, err := kind.NewCluster(kindPath, name, getKubeConfig(kubeConfig))
			if err != nil {
				return xerrors.WithStack(err)
			}
			if exists, err := cluster.IsExist(cmd.Context(), name); err != nil {
				return err
			} else if !exists {
				return nil
			}

			if err := cluster.Delete(cmd.Context()); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "kindcluster", "Name of cluster")
	cmd.Flags().StringVar(&kindPath, "kind", "", "The path of kind")
	cmd.Flags().StringVar(&kubeConfig, "kubeconfig", "", "A path to the kubeconfig file. If not specified, will be used default file of kubectl")

	rootCmd.AddCommand(cmd)
}

func applyCmd(rootCmd *cobra.Command) {
	var name, kindPath, manifest string
	cmd := &cobra.Command{
		Use: "apply",
		RunE: func(_ *cobra.Command, _ []string) error {
			cluster, err := kind.NewCluster(kindPath, name, "")
			if err != nil {
				return err
			}
			if err := cluster.Apply(manifest, "kindcluster"); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "kindcluster", "")
	cmd.Flags().StringVar(&kindPath, "kind", "", "")
	cmd.Flags().StringVar(&manifest, "manifest", "", "")

	rootCmd.AddCommand(cmd)
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
