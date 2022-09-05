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

func DevEnv() *cobra.Command {
	devEnvCmd := &cobra.Command{
		Use:   "dev-env",
		Short: "Start some middlewares for development",
	}

	goModuleProxy(devEnvCmd)
	repoDoc(devEnvCmd)

	return devEnvCmd
}
