package monodev

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	goGit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"go.f110.dev/mono/go/pkg/git"
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
			break
		}
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		conn.Close()
		break
	}

	opt := storage.NewS3OptionToExternal("http://127.0.0.1:9000", "US", "minioadmin", "minioadmin")
	opt.PathStyle = true
	client := storage.NewS3("git-data-service", opt)
	if !client.ExistBucket(ctx, "git-data-service") {
		logger.Log.Info("git-data-service bucket is not found. make the bucket")
		if err := client.MakeBucket(ctx, "git-data-service"); err != nil {
			logger.Log.Error("Failed to make bucket", logger.Error(err))
			return
		}
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

		logger.Log.Debug("Walk directory", zap.String("path", filepath.Join(workDir, ".example_git_data/.git")))
		err = filepath.Walk(filepath.Join(workDir, ".example_git_data/.git"), func(p string, info fs.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}

			name := strings.TrimPrefix(p, filepath.Join(workDir, ".example_git_data/.git")+"/")
			file, err := os.Open(p)
			if err != nil {
				return err
			}
			logger.Log.Debug("Put object", zap.String("name", repositoryName+"/"+name))
			err = client.PutReader(context.Background(), repositoryName+"/"+name, file)
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

	storer := git.NewObjectStorageStorer(client, repositoryName)
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

	logger.Log.Info("Start gRPC server", zap.String("addr", ":9010"))
	if err := s.Serve(lis); err != nil {
		logger.Log.Warn("Serve gRPC", logger.Error(err))
		return
	}
}
