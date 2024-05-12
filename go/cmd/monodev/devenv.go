package main

import (
	"context"

	"go.f110.dev/mono/go/cli"
)

func init() {
	CommandManager.Register(DevEnv())
}

func goModuleProxy(cmd *cli.Command) {
	goModuleProxyCmd := &cli.Command{
		Use:   "gomodule-proxy",
		Short: "Start memcached and minio",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			m := newComponentManager()
			m.AddComponent(memcached)
			m.AddComponent(minio)

			return m.Run(ctx)
		},
	}

	cmd.AddCommand(goModuleProxyCmd)
}

func repoDoc(cmd *cli.Command) {
	repoDocCmd := &cli.Command{
		Use:   "repo-doc",
		Short: "Start minio and git-data-service",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			m := newComponentManager()
			m.AddComponent(docSearchService)

			return m.Run(ctx)
		},
	}

	cmd.AddCommand(repoDocCmd)
}

func build(cmd *cli.Command) {
	buildCmd := &cli.Command{
		Use:   "build",
		Short: "Start MySQL",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			m := newComponentManager()
			m.AddComponent(buildDatabase)
			m.AddComponent(buildMySQLUSER)
			m.AddComponent(minio)
			m.AddComponent(buildBucket)

			return m.Run(ctx)
		},
	}
	etcd.Flags(buildCmd.Flags())
	minio.Flags(buildCmd.Flags())

	cmd.AddCommand(buildCmd)
}

func DevEnv() *cli.Command {
	devEnvCmd := &cli.Command{
		Use:   "env",
		Short: "Start some middlewares for development",
	}

	for _, v := range []func(*cli.Command){
		goModuleProxy,
		repoDoc,
		build,
	} {
		v(devEnvCmd)
	}

	return devEnvCmd
}
