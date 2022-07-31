package monodev

import (
	"bytes"
	"context"
	"fmt"
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
	workDir := os.Getenv("BUILD_WORKING_DIRECTORY")

	minio := exec.CommandContext(ctx,
		"minio",
		"server",
		"--address", "127.0.0.1:9000",
		"--console-address", "127.0.0.1:50000",
		filepath.Join(workDir, ".minio_data"),
	)
	minio.Env = append(os.Environ(), []string{
		fmt.Sprintf("MINIO_ROOT_USER=minioadmin"),
		fmt.Sprintf("MINIO_ROOT_PASSWORD=minioadmin"),
	}...)
	//w := logger.NewNamedWriter(os.Stdout, "minio")
	minio.Stdout = os.Stdout
	minio.Stderr = os.Stdout

	defer func() {
		if minio.Process != nil && !minio.ProcessState.Exited() {
			logger.Log.Info("Kill minio by SIGTERM")
			minio.Process.Signal(syscall.SIGTERM)
		}
	}()

	logger.Log.Info("Start minio", zap.Int("port", 9000), zap.String("user", "minioadmin"), zap.String("password", "minioadmin"))
	if err := minio.Run(); err != nil && err.Error() != "signal: killed" {
		logger.Log.Info("Some error was occurred", zap.Error(err))
	}
	logger.Log.Info("Shutdown minio")
}

type gitDataServiceComponent struct{}

func (c *gitDataServiceComponent) Command() string {
	return ""
}

func (c *gitDataServiceComponent) Run(ctx context.Context) {
	const (
		repositoryURL  = "https://github.com/kubernetes/enhancements.git"
		repositoryName = "kep"
	)

	workDir := os.Getenv("BUILD_WORKING_DIRECTORY")

	deadline := time.Now().Add(10 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", ":9000", 100*time.Millisecond)
		if time.Now().After(deadline) {
			logger.Log.Info("Deadline exceeded")
			return
		}
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		conn.Close()
		break
	}
	for {
		conn, err := net.DialTimeout("tcp", ":11212", 100*time.Millisecond)
		if time.Now().After(deadline) {
			logger.Log.Info("Deadline exceeded")
			return
		}
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		conn.Close()
		break
	}

	if _, err := os.Stat(filepath.Join(workDir, ".example_git_data")); os.IsNotExist(err) {
		logger.Log.Info("example data is not found. clone the repository")
		_, err := goGit.PlainClone(filepath.Join(workDir, ".example_git_data"), false, &goGit.CloneOptions{
			URL:           repositoryURL,
			Depth:         1,
			ReferenceName: plumbing.NewBranchReferenceName("master"),
			SingleBranch:  true,
			NoCheckout:    true,
		})
		if err != nil {
			return
		}
	}

	opt := storage.NewS3OptionToExternal("http://127.0.0.1:9000", "US", "minioadmin", "minioadmin")
	opt.PathStyle = true
	storageClient := storage.NewS3("git-data-service", opt)

	if !storageClient.ExistBucket(ctx, "git-data-service") {
		logger.Log.Info("git-data-service bucket is not found. make the bucket")
		if err := storageClient.MakeBucket(ctx, "git-data-service"); err != nil {
			logger.Log.Error("Failed to make bucket", logger.Error(err))
			return
		}

		logger.Log.Debug("Walk directory", zap.String("path", filepath.Join(workDir, ".example_git_data/.git")))
		err := filepath.Walk(filepath.Join(workDir, ".example_git_data/.git"), func(p string, info fs.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}

			name := strings.TrimPrefix(p, filepath.Join(workDir, ".example_git_data/.git")+"/")
			file, err := os.Open(p)
			if err != nil {
				return err
			}
			logger.Log.Debug("Put object", zap.String("name", repositoryName+"/"+name))
			err = storageClient.PutReader(context.Background(), repositoryName+"/"+name, file)
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

	memcachedServer, err := client.NewServerWithMetaProtocol(context.Background(), "local", "tcp", "127.0.0.1:11212")
	if err != nil {
		logger.Log.Error("Failed to create Server", logger.Error(err))
		return
	}
	cachePool, err := client.NewSinglePool(memcachedServer)
	if err != nil {
		logger.Log.Error("Failed to create cache pool", logger.Error(err))
		return
	}
	_ = cachePool
	storer := git.NewObjectStorageStorer(storageClient, repositoryName, cachePool)
	repo, err := goGit.Open(storer, nil)
	if err != nil {
		logger.Log.Error("Failed to open the repository", logger.Error(err))
		return
	}
	service, err := git.NewDataService(map[string]*goGit.Repository{repositoryName: repo})
	if err != nil {
		return
	}
	s := grpc.NewServer()
	git.RegisterGitDataServer(s, service)
	lis, err := net.Listen("tcp", ":9010")
	if err != nil {
		logger.Log.Error("Failed to listen", logger.Error(err))
		return
	}

	go func() {
		<-ctx.Done()

		logger.Log.Debug("Graceful stop gRPC server")
		s.GracefulStop()
	}()

	logger.Log.Info("Start gRPC server", zap.String("addr", ":9010"))
	if err := s.Serve(lis); err != nil {
		logger.Log.Warn("Serve gRPC", logger.Error(err))
		return
	}
	logger.Log.Info("Stop gRPC server")
}

type mysqlComponent struct{}

func (c *mysqlComponent) Command() string {
	return "mysqld"
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
		"--port=3306",
		"--skip-networking=0",
	)
	mysql.Stdout = os.Stdout
	mysql.Stderr = os.Stderr

	defer func() {
		logger.Log.Info("Shutdown MySQL")

		hostname, err := os.Hostname()
		if err != nil {
			logger.Log.Error("Failed to get hostname", logger.Error(err))
			return
		}
		pidFile := filepath.Join(dataDir, hostname+".pid")
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
	}()

	logger.Log.Info("Start MySQL")
	if err := mysql.Start(); err != nil {
		logger.Log.Info("Some error was occurred", logger.Error(err))
		return
	}
	<-ctx.Done()
}

type docSearchService struct{}

func (c *docSearchService) Command() string {
	return ""
}

func (c *docSearchService) Run(ctx context.Context) {
	deadline := time.Now().Add(10 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", ":9010", 100*time.Millisecond)
		if time.Now().After(deadline) {
			logger.Log.Info("Deadline exceeded")
			break
		}
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		conn.Close()
		break
	}

	grpcConn, err := grpc.Dial("127.0.0.1:9010",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpcutil.WithLogging(),
	)
	if err != nil {
		logger.Log.Error("Failed to dial", logger.Error(err))
		return
	}
	gitDataClient := git.NewGitDataClient(grpcConn)

	service := docutil.NewDocSearchService(gitDataClient)
	initCtx, stop := context.WithTimeout(ctx, 1*time.Minute)
	if err := service.Initialize(initCtx, 1); err != nil {
		stop()
		logger.Log.Error("Failed to initialize doc-search-service", logger.Error(err))
		return
	}
	stop()
	s := grpc.NewServer()
	docutil.RegisterDocSearchServer(s, service)
	lis, err := net.Listen("tcp", ":9011")
	if err != nil {
		logger.Log.Error("Failed to listen", logger.Error(err))
		return
	}

	go func() {
		<-ctx.Done()

		logger.Log.Debug("Graceful stop doc-search-service")
		s.GracefulStop()
	}()

	logger.Log.Info("Start doc-search-service", zap.String("addr", ":9011"))
	if err := s.Serve(lis); err != nil {
		logger.Log.Warn("Serve gRPC", logger.Error(err))
		return
	}
	logger.Log.Info("Stop doc-search-service")
}
