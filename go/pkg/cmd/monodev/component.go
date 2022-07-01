package monodev

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"go.uber.org/zap"

	"go.f110.dev/mono/go/pkg/logger"
)

type memcachedComponent struct{}

func (c *memcachedComponent) Command() string {
	return "memcached"
}

func (c *memcachedComponent) Run(ctx context.Context) {
	memcached := exec.CommandContext(ctx, "memcached", "-p", "11212")
	w := logger.NewNamedWriter(os.Stdout, "memcached")
	memcached.Stdout = w
	memcached.Stderr = w
	logger.Log.Info("Start memcached", zap.Int("port", 11212))
	if err := memcached.Run(); err != nil {
		logger.Log.Info("Some error was occurred", zap.Error(err))
	}
	logger.Log.Info("Shutdown memcached")
}

type minioComponent struct{}

func (c *minioComponent) Command() string {
	return "minio"
}

func (c *minioComponent) Run(ctx context.Context) {
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
}
