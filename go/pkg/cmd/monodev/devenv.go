package monodev

import (
	"context"
	"os/exec"
	"time"

	"github.com/spf13/cobra"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/go/pkg/parallel"
)

func init() {
	CommandManager.Register(DevEnv())
}

type component interface {
	Command() string
	Run(ctx context.Context)
}

type componentManager struct {
	components []component
}

func newComponentManager() *componentManager {
	return &componentManager{}
}

func (m *componentManager) Run(ctx context.Context) error {
	for _, c := range m.components {
		cmd := c.Command()
		if cmd != "" {
			_, err := exec.LookPath(cmd)
			if err != nil {
				return xerrors.Newf("%s is not found", cmd)
			}
		}
	}

	supervisor := parallel.NewSupervisor(ctx)
	for _, c := range m.components {
		supervisor.Add(c.Run)
	}

	// Wait for signals
	<-ctx.Done()

	logger.Log.Debug("Shutting down")
	ctx, cFunc := context.WithTimeout(context.Background(), 5*time.Second)
	if err := supervisor.Shutdown(ctx); err != nil {
		cFunc()
		return xerrors.WithStack(err)
	}
	cFunc()
	logger.Log.Info("All subprocesses finished")
	return nil
}

func (m *componentManager) AddComponent(c component) {
	m.components = append(m.components, c)
}

func goModuleProxy(cmd *cobra.Command) {
	goModuleProxyCmd := &cobra.Command{
		Use:   "gomodule-proxy",
		Short: "Start memcached and minio",
		RunE: func(cmd *cobra.Command, _ []string) error {
			m := newComponentManager()
			m.AddComponent(&memcachedComponent{})
			m.AddComponent(&minioComponent{})

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
			m.AddComponent(&minioComponent{})
			m.AddComponent(&gitDataServiceComponent{})
			m.AddComponent(&docSearchService{})
			m.AddComponent(&memcachedComponent{})

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
