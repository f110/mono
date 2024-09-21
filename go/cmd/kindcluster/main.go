package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/ctxutil"
	"go.f110.dev/mono/go/k8s/kind"
)

func main() {
	rootCmd := &cli.Command{
		Use: "kindcluster",
	}
	createCmd(rootCmd)
	deleteCmd(rootCmd)
	applyCmd(rootCmd)

	if err := rootCmd.Execute(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}

func createCmd(rootCmd *cli.Command) {
	var name, kindPath, k8sVersion, kubeConfig, manifest string
	workerNum := 1
	cmd := &cli.Command{
		Use: "create",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			cluster, err := kind.NewCluster(kindPath, name, getKubeConfig(kubeConfig))
			if err != nil {
				return err
			}
			exists, err := cluster.IsExist(ctx, name)
			if err != nil {
				return err
			}

			if !exists {
				if err := cluster.Create(ctx, k8sVersion, workerNum); err != nil {
					return err
				}
			} else {
				fmt.Println("Cluster is already exist. Only apply the manifest.")
			}

			tCtx, cancelFunc := ctxutil.WithTimeout(ctx, 3*time.Minute)
			defer cancelFunc()
			if err := cluster.WaitReady(tCtx); err != nil {
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
	cmd.Flags().String("name", "Name of cluster").Var(&name).Default("kindcluster")
	cmd.Flags().String("kind", "The path of kind").Var(&kindPath)
	cmd.Flags().String("k8s-version", "Cluster version").Var(&k8sVersion)
	cmd.Flags().String("kubeconfig", "A path to the kubeconfig file. If not specified, will be used default file of kubectl").Var(&kubeConfig)
	cmd.Flags().Int("worker-num", "The number of worker").Var(&workerNum).Default(workerNum)
	cmd.Flags().String("manifest", "The path of default manifest").Var(&manifest)

	rootCmd.AddCommand(cmd)
}

func deleteCmd(rootCmd *cli.Command) {
	var name, kindPath, kubeConfig string
	cmd := &cli.Command{
		Use: "delete",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			cluster, err := kind.NewCluster(kindPath, name, getKubeConfig(kubeConfig))
			if err != nil {
				return xerrors.WithStack(err)
			}
			if exists, err := cluster.IsExist(ctx, name); err != nil {
				return err
			} else if !exists {
				return nil
			}

			if err := cluster.Delete(ctx); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().String("name", "Name of cluster").Var(&name).Default("kindcluster")
	cmd.Flags().String("kind", "The path of kind").Var(&kindPath)
	cmd.Flags().String("kubeconfig", "A path to the kubeconfig file. If not specified, will be used default file of kubectl").Var(&kubeConfig)

	rootCmd.AddCommand(cmd)
}

func applyCmd(rootCmd *cli.Command) {
	var name, kindPath, manifest string
	cmd := &cli.Command{
		Use: "apply",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
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
	cmd.Flags().String("name", "").Var(&name).Default("kindcluster")
	cmd.Flags().String("kind", "").Var(&kindPath)
	cmd.Flags().String("manifest", "").Var(&manifest)

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
