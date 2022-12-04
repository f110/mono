package monodev

import (
	"github.com/spf13/cobra"
)

func init() {
	CommandManager.Register(DevEnv())
}

func goModuleProxy(cmd *cobra.Command) {
	goModuleProxyCmd := &cobra.Command{
		Use:   "gomodule-proxy",
		Short: "Start memcached and minio",
		RunE: func(cmd *cobra.Command, _ []string) error {
			m := newComponentManager()
			m.AddComponent(memcached)
			m.AddComponent(minio)

			return m.Run(cmd.Context())
		},
	}

	cmd.AddCommand(goModuleProxyCmd)
}

func repoDoc(cmd *cobra.Command) {
	repoDocCmd := &cobra.Command{
		Use:   "repo-doc",
		Short: "Start minio and git-data-service",
		RunE: func(cmd *cobra.Command, _ []string) error {
			m := newComponentManager()
			m.AddComponent(docSearchService)

			return m.Run(cmd.Context())
		},
	}

	cmd.AddCommand(repoDocCmd)
}

func build(cmd *cobra.Command) {
	buildCmd := &cobra.Command{
		Use:   "build",
		Short: "Start MySQL",
		RunE: func(cmd *cobra.Command, _ []string) error {
			m := newComponentManager()
			m.AddComponent(buildDatabase)
			m.AddComponent(minio)
			m.AddComponent(etcd)

			return m.Run(cmd.Context())
		},
	}
	etcd.Flags(buildCmd.Flags())

	cmd.AddCommand(buildCmd)
}

func DevEnv() *cobra.Command {
	devEnvCmd := &cobra.Command{
		Use:   "dev-env",
		Short: "Start some middlewares for development",
	}

	for _, v := range []func(*cobra.Command){
		goModuleProxy,
		repoDoc,
		build,
	} {
		v(devEnvCmd)
	}

	return devEnvCmd
}
