package bff

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"

	"go.f110.dev/xerrors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.f110.dev/mono/go/build/api"
	"go.f110.dev/mono/go/build/bff/bffconnect"
	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/storage"
)

func bffCmd(ctx context.Context, opts options) error {
	if opts.SecretAccessKeyFile != "" {
		b, err := os.ReadFile(opts.SecretAccessKeyFile)
		if err != nil {
			return xerrors.WithStack(err)
		}
		opts.SecretAccessKey = strings.TrimSpace(string(b))
	}

	conn, err := grpc.NewClient(opts.APIHost, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return xerrors.WithStack(err)
	}
	apiClient := api.NewAPIClient(conn)
	s3Opt := storage.NewS3OptionToExternal(opts.StorageEndpoint, opts.StorageRegion, opts.AccessKey, opts.SecretAccessKey)
	b := bffconnect.NewBFF(opts.Addr, apiClient, opts.Bucket, s3Opt)
	go func() {
		<-ctx.Done()
		b.Shutdown(context.Background())
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		logger.Log.Info("Listen", logger.String("addr", opts.Addr))
		if err := b.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Error("Error", logger.Error(err))
			return
		}
	}()

	wg.Wait()
	return nil
}

type options struct {
	Addr                string
	APIHost             string
	Bucket              string
	StorageEndpoint     string
	StorageRegion       string
	AccessKey           string
	SecretAccessKey     string
	SecretAccessKeyFile string
}

func AddCommand(rootCmd *cli.Command) {
	opt := options{}
	cmd := &cli.Command{
		Use: "bff",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			return bffCmd(ctx, opt)
		},
	}

	fs := cmd.Flags()
	fs.String("addr", "Listen address").Var(&opt.Addr)
	fs.String("api", "API Host which user's browser can access").Var(&opt.APIHost)
	fs.String("storage-endpoint", "The endpoint of MinIO. If this value is empty, then we find the endpoint from kube-apiserver using incluster config.").Var(&opt.StorageEndpoint)
	fs.String("storage-region", "The region name of MinIO.").Var(&opt.StorageRegion)
	fs.String("bucket", "The bucket name that will be used a log storage").Var(&opt.Bucket).Default("logs")
	fs.String("access-key", "The access key").Var(&opt.AccessKey)
	fs.String("secret-access-key", "The secret access key").Var(&opt.SecretAccessKey)
	fs.String("secret-access-key-file", "The file path that contains secret access key").Var(&opt.SecretAccessKeyFile)

	rootCmd.AddCommand(cmd)
}
