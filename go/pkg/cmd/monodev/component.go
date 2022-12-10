package monodev

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	goGit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	gitTransport "github.com/go-git/go-git/v5/plumbing/transport"
	gitHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/pflag"
	"go.f110.dev/go-memcached/client"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.f110.dev/mono/go/pkg/docutil"
	"go.f110.dev/mono/go/pkg/git"
	"go.f110.dev/mono/go/pkg/grpcutil"
	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/go/pkg/storage"
)

var memcached = &simpleCommandComponent{
	Name:          "memcached",
	Args:          []string{"-p", "11212"},
	VerboseOutput: true,
	Ports:         ports{{Name: "memcached", Number: 11212}},
}

var minio = &simpleCommandComponent{
	Name: "minio",
	Args: []string{"server",
		"--address", "127.0.0.1:9000",
		"--console-address", "127.0.0.1:50000",
		filepath.Join(os.Getenv("BUILD_WORKING_DIRECTORY"), ".minio_data"),
	},
	EnvVar: []string{
		"MINIO_ROOT_USER=minioadmin",
		"MINIO_ROOT_PASSWORD=minioadmin",
	},
	Ports:            ports{{Name: "minio", Number: 9000}, {Name: "console", Number: 50000}},
	VerboseOutput:    true,
	WithoutLogPrefix: true,
}

var etcd = &simpleCommandComponent{
	Name:          "etcd",
	Args:          []string{"--data-dir", filepath.Join(os.Getenv("BUILD_WORKING_DIRECTORY"), ".etcd_data")},
	Ports:         ports{{Name: "", Number: 2379}},
	VerboseOutput: true,
}

var gitDataService = &grpcServerComponent{
	Name:   "git-data-service",
	Listen: 9010,
	Deps:   []component{minio, memcached, gitDataServiceBucket, kepData},
	Register: func(ctx context.Context, s *grpc.Server) {
		repositories := []*bucketData{kepData}

		opt := storage.NewS3OptionToExternal(
			fmt.Sprintf("http://127.0.0.1:%d", minio.Ports.GetNumber("minio")),
			"US",
			"minioadmin",
			"minioadmin",
		)
		opt.PathStyle = true
		storageClient := storage.NewS3("git-data-service", opt)

		memcachedServer, err := client.NewServerWithMetaProtocol(
			ctx,
			"local",
			"tcp",
			fmt.Sprintf("127.0.0.1:%d", memcached.Ports.GetNumber("memcached")),
		)
		if err != nil {
			logger.Log.Error("Failed to create Server", logger.Error(err))
			return
		}
		cachePool, err := client.NewSinglePool(memcachedServer)
		if err != nil {
			logger.Log.Error("Failed to create cache pool", logger.Error(err))
			return
		}
		repo := make(map[string]*goGit.Repository)
		for _, v := range repositories {
			storer := git.NewObjectStorageStorer(storageClient, v.Prefix, cachePool)
			r, err := goGit.Open(storer, nil)
			if err != nil {
				logger.Log.Error("Failed to open the repository", logger.Error(err))
				return
			}
			repo[v.Name] = r
		}
		service, err := git.NewDataService(repo)
		if err != nil {
			return
		}
		git.RegisterGitDataServer(s, service)
	},
}

var docSearchService = &grpcServerComponent{
	Name:   "doc-search-service",
	Listen: 9011,
	Deps:   []component{minio, gitDataService},
	Register: func(ctx context.Context, s *grpc.Server) {
		opt := storage.NewS3OptionToExternal(
			fmt.Sprintf("http://127.0.0.1:%d", minio.Ports.GetNumber("minio")),
			"US",
			"minioadmin",
			"minioadmin",
		)
		opt.PathStyle = true
		storageClient := storage.NewS3("git-data-service", opt)

		grpcConn, err := grpc.Dial(fmt.Sprintf("127.0.0.1:%d", gitDataService.Listen),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpcutil.WithLogging(),
		)
		if err != nil {
			logger.Log.Error("Failed to dial", logger.Error(err))
			return
		}
		gitDataClient := git.NewGitDataClient(grpcConn)

		service := docutil.NewDocSearchService(gitDataClient, storageClient)
		if err := service.Initialize(ctx, 8, 1); err != nil {
			logger.Log.Error("Failed to initialize doc-search-service", logger.Error(err))
			return
		}
		docutil.RegisterDocSearchServer(s, service)
	},
}

var kepRepository = &gitDataDirectory{
	Name:   "kep",
	URL:    "https://github.com/kubernetes/enhancements.git",
	Dir:    ".example_git_data",
	Branch: "master",
}

var gitDataServiceBucket = &minioBucket{
	Name:     "git-data-service",
	Bucket:   "git-data-service",
	Instance: minio,
}

var kepData = &bucketData{
	Name:     "kep",
	Prefix:   "kep",
	Instance: minio,
	Data:     kepRepository,
}

var mysql = &mysqlComponent{
	Port: port{Name: "mysql", Number: 13306},
}

var buildDatabase = &mysqlDatabase{
	Name:  "build",
	MySQL: mysql,
}

var buildMySQLUSER = &mysqlUser{
	Name:     "build",
	Password: "build",
	MySQL:    mysql,
}

type simpleCommandComponent struct {
	Name               string
	Type               componentType
	ExecutableFilePath string
	Args               []string
	EnvVar             []string
	Ports              ports
	VerboseOutput      bool
	WithoutLogPrefix   bool
}

var _ component = &simpleCommandComponent{}

type port struct {
	Name   string
	Number int
}

type ports []port

func (p ports) GetNumber(name string) int {
	for _, v := range p {
		if v.Name == name {
			return v.Number
		}
	}

	return -1
}

func (c *simpleCommandComponent) GetName() string {
	return c.Name
}

func (c *simpleCommandComponent) GetType() componentType {
	return c.Type
}

func (c *simpleCommandComponent) GetDeps() []component {
	return nil
}

func (c *simpleCommandComponent) Run(ctx context.Context) {
	cmd := exec.CommandContext(context.Background(), c.ExecutableFilePath, c.Args...)
	if c.VerboseOutput {
		var out, err io.Writer
		if c.WithoutLogPrefix {
			out = os.Stdout
			err = os.Stderr
		} else {
			w := logger.NewNamedWriter(os.Stdout, c.Name)
			out = w
			err = w
		}
		cmd.Stdout = out
		cmd.Stderr = err
	}
	cmd.Env = append(os.Environ(), c.EnvVar...)

	go func() {
		<-ctx.Done()
		if cmd.Process != nil && cmd.Process.Pid > 0 {
			cmd.Process.Signal(syscall.SIGTERM)
		}
	}()

	defer func() {
		if cmd.Process != nil && !cmd.ProcessState.Exited() {
			logger.Log.Info("Kill " + c.Name + " by SIGTERM")
			cmd.Process.Signal(syscall.SIGTERM)
		}
	}()
	logger.Log.Info("Start " + c.ExecutableFilePath)
	if err := cmd.Run(); err != nil {
		logger.Log.Info("Some error was occurred", zap.Error(err))
	}
	logger.Log.Info("Shutdown " + c.Name)
}

func (c *simpleCommandComponent) Ready() bool {
	for _, v := range c.Ports {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf(":%d", v.Number), 100*time.Millisecond)
		if err != nil {
			return false
		}

		conn.Close()
	}

	return true
}

func (c *simpleCommandComponent) Flags(fs *pflag.FlagSet) {
	fs.StringVar(&c.ExecutableFilePath, c.Name, c.Name, "Executable file path")
}

type grpcServerComponent struct {
	Name     string
	Deps     []component
	Listen   int
	Register func(context.Context, *grpc.Server)
}

var _ component = &grpcServerComponent{}

func (c *grpcServerComponent) GetName() string {
	return c.Name
}

func (c *grpcServerComponent) GetType() componentType {
	return componentTypeService
}

func (c *grpcServerComponent) Run(ctx context.Context) {
	s := grpc.NewServer()
	c.Register(ctx, s)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", c.Listen))
	if err != nil {
		logger.Log.Error("Failed to listen", logger.Error(err))
		return
	}

	go func() {
		<-ctx.Done()

		logger.Log.Debug(fmt.Sprintf("Graceful stop %s server", c.Name))
		s.GracefulStop()
	}()

	logger.Log.Info(fmt.Sprintf("Start %s server", c.Name), zap.Int("listen", c.Listen))
	if err := s.Serve(lis); err != nil {
		logger.Log.Warn("Serve gRPC", logger.Error(err))
		return
	}
	logger.Log.Info("Stop gRPC server")
}

func (c *grpcServerComponent) GetDeps() []component {
	return c.Deps
}

func (c *grpcServerComponent) Ready() bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf(":%d", c.Listen), 100*time.Millisecond)
	if err != nil {
		return false
	}

	conn.Close()
	return true
}

type gitDataDirectory struct {
	Name   string
	Dir    string
	URL    string
	Branch string
}

var _ component = &gitDataDirectory{}

func (c *gitDataDirectory) GetName() string {
	return c.Name
}

func (c *gitDataDirectory) GetType() componentType {
	return componentTypeOneshot
}

func (c *gitDataDirectory) Run(ctx context.Context) {
	dir := filepath.Join(os.Getenv("BUILD_WORKING_DIRECTORY"), c.Dir)
	var auth gitTransport.AuthMethod
	if v := os.Getenv("GITHUB_TOKEN"); v != "" {
		auth = &gitHttp.BasicAuth{Username: "octocat", Password: v}
	}
	logger.Log.Info("git directory is not found. clone the repository", zap.String("dir", dir))
	_, err := goGit.PlainCloneContext(ctx, dir, false, &goGit.CloneOptions{
		URL:           c.URL,
		Depth:         1,
		ReferenceName: plumbing.NewBranchReferenceName(c.Branch),
		SingleBranch:  true,
		NoCheckout:    true,
		Auth:          auth,
	})
	if err != nil && !errors.Is(err, goGit.ErrRepositoryAlreadyExists) {
		logger.Log.Info("failed to clone the repository", zap.Error(err))
		return
	}
	logger.Log.Info("Finished cloning the repository")
}

func (c *gitDataDirectory) GetDeps() []component {
	return nil
}

func (c *gitDataDirectory) OutputDir() string {
	return filepath.Join(os.Getenv("BUILD_WORKING_DIRECTORY"), c.Dir, ".git")
}

type minioBucket struct {
	Name     string
	Bucket   string
	Instance component
}

var _ component = &minioBucket{}

func (c *minioBucket) GetName() string {
	return c.Name
}

func (c *minioBucket) GetType() componentType {
	return componentTypeOneshot
}

func (c *minioBucket) Run(ctx context.Context) {
	if c.Instance.GetName() != "minio" {
		logger.Log.Error("instance is not MinIO")
		return
	}

	port := c.Instance.(*simpleCommandComponent).Ports.GetNumber("minio")
	opt := storage.NewS3OptionToExternal(
		fmt.Sprintf("http://127.0.0.1:%d", port),
		"US",
		"minioadmin",
		"minioadmin",
	)
	opt.PathStyle = true
	storageClient := storage.NewS3(c.Bucket, opt)

	if storageClient.ExistBucket(ctx, c.Bucket) {
		logger.Log.Info("the bucket is found")
	} else {
		logger.Log.Info("the bucket is not found. make the bucket")
		if err := storageClient.MakeBucket(ctx, c.Bucket); err != nil {
			logger.Log.Error("Failed to make bucket", logger.Error(err))
			return
		}
	}
}

func (c *minioBucket) GetDeps() []component {
	return []component{c.Instance}
}

type bucketData struct {
	Name     string
	Prefix   string
	Data     component
	Instance component
}

var _ component = &bucketData{}

func (c *bucketData) GetName() string {
	return c.Name
}

func (c *bucketData) GetType() componentType {
	return componentTypeOneshot
}

func (c *bucketData) GetDeps() []component {
	return []component{c.Data, c.Instance}
}

func (c *bucketData) Run(ctx context.Context) {
	if c.Instance.GetName() != "minio" {
		logger.Log.Error("instance is not MinIO")
		return
	}

	port := c.Instance.(*simpleCommandComponent).Ports.GetNumber("minio")
	opt := storage.NewS3OptionToExternal(
		fmt.Sprintf("http://127.0.0.1:%d", port),
		"US",
		"minioadmin",
		"minioadmin",
	)
	opt.PathStyle = true
	storageClient := storage.NewS3(c.Name, opt)

	var dir string
	if x, ok := c.Data.(interface{ OutputDir() string }); ok {
		dir = x.OutputDir()
	}
	if dir == "" {
		logger.Log.Error("the component, specified by Data, is not support OutputDir")
		return
	}

	logger.Log.Debug("Walk directory", zap.String("path", dir))
	err := filepath.Walk(dir, func(p string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		name := strings.TrimPrefix(p, dir+"/")
		file, err := os.Open(p)
		if err != nil {
			return err
		}
		logger.Log.Debug("Put object", zap.String("name", c.Prefix+"/"+name))
		err = storageClient.PutReader(ctx, c.Prefix+"/"+name, file)
		if err != nil {
			return err
		}
		file.Close()
		return nil
	})
	if err != nil {
		logger.Log.Error("Failed to put git data", logger.Error(err))
		return
	}
}

type mysqlComponent struct {
	Port port

	cmd *exec.Cmd
}

var _ component = &mysqlComponent{}

func (c *mysqlComponent) GetName() string {
	return "mysqld"
}

func (c *mysqlComponent) GetType() componentType {
	return componentTypeService
}

func (c *mysqlComponent) GetDeps() []component {
	return nil
}

func (c *mysqlComponent) Run(ctx context.Context) {
	workDir := os.Getenv("BUILD_WORKING_DIRECTORY")
	baseDir := filepath.Join(workDir, ".mysql")
	dataDir := filepath.Join(baseDir, "data")
	secureFileDir := filepath.Join(baseDir, "mysql8-files")

	for _, v := range []string{baseDir, secureFileDir} {
		if _, err := os.Stat(v); os.IsNotExist(err) {
			if err := os.Mkdir(v, 0755); err != nil {
				logger.Log.Error("Failed to make directory", logger.Error(err), zap.String("path", v))
				return
			}
		}
	}

	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		logger.Log.Info("Initialize data directory")
		cmd := exec.CommandContext(ctx,
			"mysqld",
			"--initialize-insecure",
			"--user=mysql",
			"--datadir="+dataDir,
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			logger.Log.Error("Failed to initialize data dir", logger.Error(err))
		}
	}

	mysqldPath, err := exec.LookPath("mysqld")
	if err != nil {
		logger.Log.Error("Failed to get path of mysqld")
		return
	}

	mysql := exec.CommandContext(context.Background(),
		"mysqld_safe",
		"--mysqld="+mysqldPath,
		"--user=mysql",
		"--basedir="+baseDir,
		"--datadir="+dataDir,
		"--socket="+filepath.Join(baseDir, "mysqld.sock"),
		"--secure-file-priv="+secureFileDir,
		"--bind-address=127.0.0.1",
		fmt.Sprintf("--port=%d", c.Port.Number),
		"--skip-networking=0",
		fmt.Sprintf("--lc-messages-dir=%s", filepath.Clean(filepath.Join(filepath.Dir(mysqldPath), "../share/mysql8"))),
	)
	mysql.Stdout = os.Stdout
	mysql.Stderr = os.Stderr
	go func() {
		<-ctx.Done()
		c.shutdown(dataDir)
	}()

	defer c.shutdown(dataDir)

	logger.Log.Info("Start MySQL")
	c.cmd = mysql
	if err := mysql.Run(); err != nil {
		logger.Log.Info("Some error was occurred", logger.Error(err))
		return
	}
}

func (c *mysqlComponent) Ready() bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf(":%d", c.Port.Number), 100*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	logger.Log.Info("Started MySQL", zap.Int("pid", c.cmd.Process.Pid))

	return true
}

func (c *mysqlComponent) shutdown(dataDir string) {
	logger.Log.Info("Shutdown MySQL")

	hostname, err := os.Hostname()
	if err != nil {
		logger.Log.Error("Failed to get hostname", logger.Error(err))
		return
	}
	pidFile := filepath.Join(dataDir, hostname+".pid")
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		logger.Log.Info("Pid file is not found. Probably mysqld already exited.")
		return
	}
	pidBuf, err := os.ReadFile(pidFile)
	if err != nil {
		logger.Log.Error("Failed to read pid file", logger.Error(err))
		return
	}
	pidBuf = bytes.TrimSpace(pidBuf)
	pid, err := strconv.Atoi(string(pidBuf))
	if err != nil {
		return
	}
	logger.Log.Info("Kill MySQL by SIGTERM", zap.Int("pid", pid))
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		logger.Log.Error("Failed to send signal", logger.Error(err))
		return
	}
}

type mysqlDatabase struct {
	Name  string
	MySQL component
}

var _ component = &mysqlComponent{}

func (c *mysqlDatabase) GetName() string {
	return c.Name
}

func (c *mysqlDatabase) GetType() componentType {
	return componentTypeOneshot
}

func (c *mysqlDatabase) GetDeps() []component {
	return []component{c.MySQL}
}

func (c *mysqlDatabase) Run(ctx context.Context) {
	if c.MySQL.GetName() != "mysqld" {
		logger.Log.Error("MySQL is not mysqld")
		return
	}

	port := c.MySQL.(*mysqlComponent).Port.Number
	db, err := sql.Open("mysql", fmt.Sprintf("root@tcp(127.0.0.1:%d)/", port))
	if err != nil {
		logger.Log.Error("Failed to connect to mysql", logger.Error(err))
		return
	}
	_, err = db.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", c.Name))
	if err != nil {
		logger.Log.Error("Failed to create database", logger.Error(err))
		return
	}
	logger.Log.Info("Created database", zap.String("name", c.Name))
}

type mysqlUser struct {
	Name     string
	Password string
	MySQL    component
}

var _ component = &mysqlUser{}

func (c *mysqlUser) GetName() string {
	return c.Name
}

func (c *mysqlUser) GetType() componentType {
	return componentTypeOneshot
}

func (c *mysqlUser) GetDeps() []component {
	return []component{c.MySQL}
}

func (c *mysqlUser) Run(ctx context.Context) {
	if c.MySQL.GetName() != "mysqld" {
		logger.Log.Error("Mysql is not mysqld")
		return
	}

	port := c.MySQL.(*mysqlComponent).Port.Number
	db, err := sql.Open("mysql", fmt.Sprintf("root@tcp(127.0.0.1:%d)/", port))
	if err != nil {
		logger.Log.Error("Failed to connecto mysql", logger.Error(err))
		return
	}
	_, err = db.ExecContext(ctx, fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'*' IDENTIFIED BY '%s'", c.Name, c.Password))
	if err != nil {
		logger.Log.Error("Failed to create user", logger.Error(err))
		return
	}
	logger.Log.Info("Created user", zap.String("name", c.Name))
}
