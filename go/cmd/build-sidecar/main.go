package main

import (
	"context"
	"fmt"
	"os"

	"go.f110.dev/mono/go/build/cmd/sidecar"
	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/logger"
)

func buildSidecar(args []string) error {
	logger.SetLogLevel("debug")
	root := &cli.Command{
		Use: "build-sidecar",
	}

	clone := sidecar.NewCloneCommand()
	cloneCmd := &cli.Command{
		Use: clone.Name(),
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			return clone.Run(ctx)
		},
	}
	clone.SetFlags(cloneCmd.Flags())
	root.AddCommand(cloneCmd)

	report := sidecar.NewTestReportCommand()
	reportCmd := &cli.Command{
		Use: report.Name(),
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			return report.Run(ctx)
		},
	}
	report.SetFlags(reportCmd.Flags())
	root.AddCommand(reportCmd)

	credential := sidecar.NewCredentialCommand()
	credentialCmd := &cli.Command{
		Use: "credential",
	}
	credential.SetGlobalFlags(credentialCmd.Flags())
	root.AddCommand(credentialCmd)
	containerRegistryCmd := &cli.Command{
		Use: "container-registry",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			return credential.ContainerRegistry(ctx)
		},
	}
	credential.SetContainerRegistryFlags(containerRegistryCmd.Flags())
	credentialCmd.AddCommand(containerRegistryCmd)

	return root.Execute(args)
}

func main() {
	if err := buildSidecar(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
