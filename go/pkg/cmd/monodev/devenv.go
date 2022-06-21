package monodev

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/go/pkg/parallel"
)

func init() {
	CommandManager.Register(DevEnv())
}

func goModuleProxy(cmd *cobra.Command) {
	goModuleProxyCmd := &cobra.Command{
		Use:   "gomodule-proxy",
		Short: "Start memcached and minio",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancelFunc := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer cancelFunc()

			_, err := exec.LookPath("memcached")
			if err != nil {
				return xerrors.New("memcached is not found")
			}
			_, err = exec.LookPath("minio")
			if err != nil {
				return xerrors.New("minio is not found")
			}

			supervisor := parallel.NewSupervisor(ctx)
			supervisor.Add(func(ctx context.Context) {
				memcached := exec.CommandContext(ctx, "memcached", "-p", "11212")
				w := logger.NewNamedWriter(os.Stdout, "memcached")
				memcached.Stdout = w
				memcached.Stderr = w
				logger.Log.Info("Start memcached", zap.Int("port", 11212))
				if err := memcached.Run(); err != nil {
					logger.Log.Info("Some error was occurred", zap.Error(err))
				}
				logger.Log.Info("Shutdown memcached")
			})
			supervisor.Add(func(ctx context.Context) {
				minio := exec.CommandContext(ctx,
					"minio",
					"server",
					"--address", "127.0.0.1:9000",
					"--console-address", "127.0.0.1:50000",
					".minio_data",
				)
				minio.Env = append(os.Environ(), []string{
					fmt.Sprintf("MINIO_ROOT_USER=minioadmin"),
					fmt.Sprintf("MINIO_ROOT_PASSWORD=minioadmin"),
				}...)
				//w := logger.NewNamedWriter(os.Stdout, "minio")
				minio.Stdout = os.Stdout
				minio.Stderr = os.Stdout
				logger.Log.Info("Start minio", zap.Int("port", 9000), zap.String("user", "minioadmin"), zap.String("password", "minioadmin"))
				if err := minio.Run(); err != nil {
					logger.Log.Info("Some error was occurred", zap.Error(err))
				}
				logger.Log.Info("Shutdown minio")
			})

			// Wait for signals
			<-ctx.Done()

			ctx, cFunc := context.WithTimeout(cmd.Context(), 5*time.Second)
			if err := supervisor.Shutdown(ctx); err != nil {
				cFunc()
				return xerrors.WithStack(err)
			}
			cFunc()
			logger.Log.Info("All subprocesses finished")
			return nil
		},
	}

	cmd.AddCommand(goModuleProxyCmd)
}

func DevEnv() *cobra.Command {
	devEnvCmd := &cobra.Command{
		Use:   "dev-env",
		Short: "Start some middlewares for development",
	}

	goModuleProxy(devEnvCmd)

	return devEnvCmd
}
