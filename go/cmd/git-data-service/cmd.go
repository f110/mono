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

	"go.f110.dev/mono/go/pkg/fsm"
	"go.f110.dev/mono/go/pkg/git"
	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/go/pkg/storage"
)

type gitDataServiceCommand struct {
	*fsm.FSM
	s        *grpc.Server
	gitRepos []*goGit.Repository

	Listen                string
	RepositoryInitTimeout time.Duration

	MinIOEndpoint        string
	MinIORegion          string
	MinIOAccessKey       string
	MinIOSecretAccessKey string

	Bucket string

	Repositories    []string
	RefreshInterval time.Duration
	RefreshTimeout  time.Duration
	RefreshWorkers  int

	repositories []Repository
}

type Repository struct {
	Name   string
	URL    string
	Prefix string
}

const (
	stateInit fsm.State = iota
	stateStartUpdater
	stateStartServer
	stateShuttingDown
)

func newGitDataServiceCommand() *gitDataServiceCommand {
	c := &gitDataServiceCommand{}
	c.FSM = fsm.NewFSM(
		map[fsm.State]fsm.StateFunc{
			stateInit:         c.init,
			stateStartUpdater: c.startUpdater,
			stateStartServer:  c.startServer,
			stateShuttingDown: c.shuttingDown,
		},
		stateInit,
		stateShuttingDown,
	)

	return c
}

func (c *gitDataServiceCommand) init() (fsm.State, error) {
	opt := storage.NewMinIOOptionsViaEndpoint(c.MinIOEndpoint, c.MinIORegion, c.MinIOAccessKey, c.MinIOSecretAccessKey)
	storageClient := storage.NewMinIOStorage(c.Bucket, opt)

	repo := make(map[string]*goGit.Repository)
	var repos []*goGit.Repository
	for _, r := range c.repositories {
		storer := git.NewObjectStorageStorer(storageClient, r.Prefix)

		if !storer.Exist() {
			ctx, cancel := context.WithTimeout(c.Context(), c.RepositoryInitTimeout)
			logger.Log.Info("Init repository", zap.String("name", r.Name), zap.String("url", r.URL), zap.String("prefix", r.Prefix))
			if _, err := git.InitObjectStorageRepository(ctx, storageClient, r.URL, r.Prefix); err != nil {
				cancel()
				return fsm.Error(err)
			}
			cancel()
		}

		gitRepo, err := goGit.Open(storer, nil)
		if err != nil {
			return fsm.Error(xerrors.WithStack(err))
		}

		repo[r.Name] = gitRepo
		repos = append(repos, gitRepo)
	}
	c.gitRepos = repos

	s := grpc.NewServer()
	service, err := git.NewDataService(repo)
	if err != nil {
		return fsm.Error(err)
	}
	git.RegisterGitDataServer(s, service)
	c.s = s

	return fsm.Next(stateStartUpdater)
}

func (c *gitDataServiceCommand) startUpdater() (fsm.State, error) {
	if c.RefreshInterval == 0 {
		return fsm.Next(stateStartServer)
	}

	updater, err := newRepositoryUpdater(c.gitRepos, c.RefreshInterval, c.RefreshTimeout, c.RefreshWorkers)
	if err != nil {
		return fsm.Error(err)
	}
	logger.Log.Info("Start updater", zap.Duration("refresh_interval", c.RefreshInterval), zap.Int("workers", c.RefreshWorkers))
	go updater.Run(c.Context())

	return fsm.Next(stateStartServer)
}

func (c *gitDataServiceCommand) startServer() (fsm.State, error) {
	lis, err := net.Listen("tcp", c.Listen)
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}

	logger.Log.Info("Start listen", zap.String("addr", c.Listen))
	go func() {
		if err := c.s.Serve(lis); err != nil {
			logger.Log.Error("gRPC server returns error", logger.Error(err))
		}
	}()

	return fsm.Wait()
}

func (c *gitDataServiceCommand) shuttingDown() (fsm.State, error) {
	logger.Log.Debug("Graceful stopping")
	c.s.GracefulStop()
	logger.Log.Info("Stop server")

	return fsm.Finish()
}

func (c *gitDataServiceCommand) Flags(fs *pflag.FlagSet) {
	fs.StringVar(&c.Listen, "listen", ":8056", "Listen addr")
	fs.StringVar(&c.MinIOEndpoint, "minio-endpoint", c.MinIOEndpoint, "The endpoint of MinIO")
	fs.StringVar(&c.MinIORegion, "minio-region", c.MinIORegion, "The region name")
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

	fs.DurationVar(&c.RepositoryInitTimeout, "repository-init-timeout", 5*time.Minute, "The duration for timeout to initializing repository")
}

func (c *gitDataServiceCommand) ValidateFlagValue() error {
	if len(c.Repositories) == 0 {
		return errors.New("--repository is mandatory")
	}
	var repositories []Repository
	for _, v := range c.Repositories {
		if strings.Index(v, "|") == -1 {
			return xerrors.Newf("--repository=%s is invalid", v)
		}
		s := strings.Split(v, "|")
		repositories = append(repositories, Repository{Name: s[0], URL: s[1], Prefix: s[2]})
	}
	c.repositories = repositories

	return nil
}
