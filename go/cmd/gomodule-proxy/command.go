package main

import (
	"context"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.f110.dev/go-memcached/client"

	"github.com/spf13/pflag"
	"golang.org/x/xerrors"

	"go.f110.dev/mono/go/pkg/gomodule"
	"go.f110.dev/mono/go/pkg/logger"
)

type goModuleProxyCommand struct {
	ConfigPath  string
	ModuleDir   string
	Addr        string
	UpstreamURL string
	GitHubToken string

	GitHubAppId          int64
	GitHubInstallationId int64
	PrivateKeyFile       string

	StorageEndpoint        string
	StorageRegion          string
	StorageAccessKey       string
	StorageSecretAccessKey string
	StorageBucket          string

	MemcachedServers []string

	upstream *url.URL
	config   gomodule.Config
	cache    *gomodule.ModuleCache
}

func newGoModuleProxyCommand() *goModuleProxyCommand {
	return &goModuleProxyCommand{
		Addr:        ":7589",
		UpstreamURL: "https://proxy.golang.org",
	}
}

func (c *goModuleProxyCommand) Flags(fs *pflag.FlagSet) {
	fs.StringVarP(&c.ConfigPath, "config", "c", c.ConfigPath, "Configuration file path")
	fs.StringVar(&c.ModuleDir, "mod-dir", c.ModuleDir, "Module directory")
	fs.StringVar(&c.Addr, "addr", c.Addr, "Listen addr")
	fs.StringVar(&c.UpstreamURL, "upstream", c.UpstreamURL, "Upstream module proxy URL")
	fs.StringVar(&c.GitHubToken, "github-token", c.GitHubToken, "GitHub API token")
	fs.Int64Var(&c.GitHubAppId, "github-app-id", c.GitHubAppId, "GitHub App ID")
	fs.Int64Var(&c.GitHubInstallationId, "github-installation-id", c.GitHubInstallationId, "GitHub App Installation ID")
	fs.StringVar(&c.PrivateKeyFile, "github-app-private-key-file", c.PrivateKeyFile, "PEM-encoded private key file for GitHub App")
	fs.StringVar(&c.StorageEndpoint, "storage-endpoint", c.StorageEndpoint, "The endpoint of object storage")
	fs.StringVar(&c.StorageRegion, "storage-region", c.StorageRegion, "The name of region of object storage")
	fs.StringVar(&c.StorageBucket, "storage-bucket", c.StorageBucket, "The name of bucket for an archive file")
	fs.StringVar(&c.StorageAccessKey, "storage-access-key", c.StorageAccessKey, "Access key")
	fs.StringVar(&c.StorageSecretAccessKey, "storage-secret-access-key", c.StorageSecretAccessKey, "Secret access key")
	fs.StringSliceVar(&c.MemcachedServers, "memcached-servers", nil, "Memcached server name and address for the metadata cache")
}

func (c *goModuleProxyCommand) RequiredFlags() []string {
	return []string{"config"}
}

func (c *goModuleProxyCommand) Init() error {
	conf, err := gomodule.ReadConfig(c.ConfigPath)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	c.config = conf

	uu, err := url.Parse(c.UpstreamURL)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	c.upstream = uu

	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	if os.Getenv("GITHUB_TOKEN") != "" {
		c.GitHubToken = os.Getenv("GITHUB_TOKEN")
	}

	if c.PrivateKeyFile != "" {
		if _, err := os.Stat(c.PrivateKeyFile); os.IsNotExist(err) {
			return xerrors.Errorf(": %w", err)
		}
	}

	if c.StorageEndpoint != "" && c.StorageRegion != "" &&
		c.StorageBucket != "" && c.StorageAccessKey != "" && c.StorageSecretAccessKey != "" && len(c.MemcachedServers) > 0 {
		var servers []client.Server
		for _, v := range c.MemcachedServers {
			s := strings.SplitN(v, "=", 2)
			server, err := client.NewServerWithMetaProtocol(context.Background(), s[0], "tcp", s[1])
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}
			servers = append(servers, server)
		}
		cachePool, err := client.NewSinglePool(servers...)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		c.cache = gomodule.NewModuleCache(cachePool, c.StorageEndpoint, c.StorageRegion, c.StorageBucket, c.StorageAccessKey, c.StorageSecretAccessKey)
	}

	return nil
}

func (c *goModuleProxyCommand) Run() error {
	stopErrCh := make(chan error, 1)
	startErrCh := make(chan error, 1)

	proxy := gomodule.NewModuleProxy(c.config, c.ModuleDir, c.cache, c.GitHubAppId, c.GitHubInstallationId, c.PrivateKeyFile)
	server := gomodule.NewProxyServer(c.Addr, c.upstream, proxy)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	go func() {
		defer cancel()

		select {
		case <-ctx.Done():
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			logger.Log.Info("Shutting down the server")
			if err := server.Stop(ctx); err != nil {
				stopErrCh <- xerrors.Errorf(": %w", err)
			}
			cancel()
			logger.Log.Info("Server shutdown successfully")
			close(stopErrCh)
		case <-stopErrCh:
			return
		}
	}()

	go func() {
		if err := server.Start(); err != nil {
			startErrCh <- xerrors.Errorf(": %w", err)
		}
	}()

	// Wait for stopping a server
	select {
	case err, ok := <-startErrCh:
		if ok {
			return xerrors.Errorf(": %w", err)
		}
	case err, ok := <-stopErrCh:
		if ok {
			return xerrors.Errorf(": %w", err)
		}
	}

	return nil
}
