package main

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	goGit "github.com/go-git/go-git/v5"
	"github.com/spf13/pflag"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"go.f110.dev/mono/go/pkg/git"
	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/go/pkg/storage"
)

type gitDataServiceCommand struct {
	Listen string

	MinIOEndpoint        string
	MinIORegion          string
	MinIOName            string
	MinIONamespace       string
	MinIOPort            int
	MinIOAccessKey       string
	MinIOSecretAccessKey string

	Bucket string

	Repositories    []string
	RefreshInterval time.Duration
	RefreshTimeout  time.Duration
	RefreshWorkers  int

	repositories []repository
}

func (c *gitDataServiceCommand) Run(ctx context.Context) error {
	opt := storage.NewMinIOOptionsViaEndpoint(c.MinIOEndpoint, c.MinIORegion, c.MinIOAccessKey, c.MinIOSecretAccessKey)
	storageClient := storage.NewMinIOStorage(c.Bucket, opt)

	repo := make(map[string]*goGit.Repository)
	var repos []*goGit.Repository
	for _, r := range c.repositories {
		storer := git.NewObjectStorageStorer(storageClient, r.Prefix)
		gitRepo, err := goGit.Open(storer, nil)
		if err != nil {
			return xerrors.WithStack(err)
		}

		repo[r.Name] = gitRepo
		repos = append(repos, gitRepo)
	}

	if c.RefreshInterval > 0 {
		updater, err := newRepositoryUpdater(repos, c.RefreshInterval, c.RefreshTimeout, c.RefreshWorkers)
		if err != nil {
			return err
		}
		go updater.Run(ctx)
	}

	s := grpc.NewServer()
	service, err := newService(repo)
	if err != nil {
		return err
	}
	git.RegisterGitDataServer(s, service)
	lis, err := net.Listen("tcp", c.Listen)
	if err != nil {
		return xerrors.WithStack(err)
	}

	logger.Log.Info("Start listen", zap.String("addr", c.Listen))
	go func() {
		if err := s.Serve(lis); err != nil {
			logger.Log.Error("gRPC server returns error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	logger.Log.Debug("Graceful stopping")
	s.GracefulStop()
	logger.Log.Info("Stop server")
	return nil
}

func (c *gitDataServiceCommand) Flags(fs *pflag.FlagSet) {
	fs.StringVar(&c.Listen, "listen", ":8056", "Listen addr")
	fs.StringVar(&c.MinIOEndpoint, "minio-endpoint", c.MinIOEndpoint, "The endpoint of MinIO")
	fs.StringVar(&c.MinIORegion, "minio-region", c.MinIORegion, "The region name")
	fs.StringVar(&c.MinIOName, "minio-name", c.MinIOName, "The name of MinIO")
	fs.StringVar(&c.MinIONamespace, "minio-namespace", c.MinIONamespace, "The namespace of MinIO")
	fs.IntVar(&c.MinIOPort, "minio-port", c.MinIOPort, "Port number of MinIO")
	fs.StringVar(&c.Bucket, "minio-bucket", c.Bucket, "Deprecated. Use --bucket instead. The bucket name that will be used")
	fs.StringVar(&c.MinIOAccessKey, "minio-access-key", c.MinIOAccessKey, "The access key for MinIO API")
	fs.StringVar(&c.MinIOSecretAccessKey, "minio-secret-access-key", c.MinIOSecretAccessKey, "The secret access key for MinIO API")

	fs.StringSliceVar(&c.Repositories, "repository", nil, "The repository name that will be served."+
		"The value consists three elements separated by a vertical bar. The first element is the repository name. "+
		"The second element is a url for the repository. "+
		"The third element is a prefix in an object storage. (e.g. go|https://github.com/golang/go.git|golang/go)")
	fs.DurationVar(&c.RefreshInterval, "refresh-interval", 0, "The interval time for updating the repository"+
		"If set zero, interval updating is disabled.")
	fs.DurationVar(&c.RefreshTimeout, "refresh-timeout", 1*time.Minute, "The duration for timeout to updating repository")
	fs.IntVar(&c.RefreshWorkers, "refresh-workers", 1, "The number of workers for updating repository")
}

func (c *gitDataServiceCommand) ValidateFlagValue() error {
	if len(c.Repositories) == 0 {
		return errors.New("--repository is mandatory")
	}
	var repositories []repository
	for _, v := range c.Repositories {
		if strings.Index(v, "|") == -1 {
			return xerrors.Newf("--repository=%s is invalid", v)
		}
		s := strings.Split(v, "|")
		repositories = append(repositories, repository{Name: s[0], URL: s[1], Prefix: s[2]})
	}
	c.repositories = repositories

	return nil
}
