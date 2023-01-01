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
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/githubutil"
	"go.f110.dev/mono/go/gomodule"
	"go.f110.dev/mono/go/pkg/logger"
)

type goModuleProxyCommand struct {
	ConfigPath   string
	ModuleDir    string
	Addr         string
	UpstreamURL  string
	CABundleFile string

	StorageEndpoint        string
	StorageRegion          string
	StorageAccessKey       string
	StorageSecretAccessKey string
	StorageBucket          string
	StorageCACertFile      string

	MemcachedServers []string

	upstream *url.URL
	config   gomodule.Config
	cache    *gomodule.ModuleCache
	caBundle []byte

	githubClientFactory *githubutil.GitHubClientFactory
}

func newGoModuleProxyCommand() *goModuleProxyCommand {
	return &goModuleProxyCommand{
		Addr:                ":7589",
		UpstreamURL:         "https://proxy.golang.org",
		githubClientFactory: githubutil.NewGitHubClientFactory("gomodule-proxy", false),
	}
}

func (c *goModuleProxyCommand) Flags(fs *pflag.FlagSet) {
	fs.StringVarP(&c.ConfigPath, "config", "c", c.ConfigPath, "Configuration file path")
	fs.StringVar(&c.ModuleDir, "mod-dir", c.ModuleDir, "Module directory")
	fs.StringVar(&c.Addr, "addr", c.Addr, "Listen addr")
	fs.StringVar(&c.UpstreamURL, "upstream", c.UpstreamURL, "Upstream module proxy URL")
	fs.StringVar(&c.CABundleFile, "ca-bundle-file", c.CABundleFile, "A file path that contains ca certificate to clone a repository")
	fs.StringVar(&c.StorageEndpoint, "storage-endpoint", c.StorageEndpoint, "The endpoint of object storage")
	fs.StringVar(&c.StorageRegion, "storage-region", c.StorageRegion, "The name of region of object storage")
	fs.StringVar(&c.StorageBucket, "storage-bucket", c.StorageBucket, "The name of bucket for an archive file")
	fs.StringVar(&c.StorageAccessKey, "storage-access-key", c.StorageAccessKey, "Access key")
	fs.StringVar(&c.StorageSecretAccessKey, "storage-secret-access-key", c.StorageSecretAccessKey, "Secret access key")
	fs.StringVar(&c.StorageCACertFile, "storage-ca-file", c.StorageCACertFile, "File path that contains the certificate of CA")
	fs.StringSliceVar(&c.MemcachedServers, "memcached-servers", nil, "Memcached server name and address for the metadata cache")

	c.githubClientFactory.Flags(fs)
}

func (c *goModuleProxyCommand) RequiredFlags() []string {
	return []string{"config"}
}

func (c *goModuleProxyCommand) Init() error {
	if err := c.githubClientFactory.Init(); err != nil {
		return xerrors.WithStack(err)
	}

	conf, err := gomodule.ReadConfig(c.ConfigPath)
	if err != nil {
		return xerrors.WithStack(err)
	}
	c.config = conf

	uu, err := url.Parse(c.UpstreamURL)
	if err != nil {
		return xerrors.WithStack(err)
	}
	c.upstream = uu

	if c.CABundleFile != "" {
		b, err := os.ReadFile(c.CABundleFile)
		if err != nil {
			return xerrors.WithStack(err)
		}
		c.caBundle = b
	}

	if c.StorageEndpoint != "" && c.StorageRegion != "" &&
		c.StorageBucket != "" && c.StorageAccessKey != "" && c.StorageSecretAccessKey != "" && len(c.MemcachedServers) > 0 {
		var servers []client.Server
		for _, v := range c.MemcachedServers {
			s := strings.SplitN(v, "=", 2)
			server, err := client.NewServerWithMetaProtocol(context.Background(), s[0], "tcp", s[1])
			if err != nil {
				return xerrors.WithStack(err)
			}
			servers = append(servers, server)
		}
		cachePool, err := client.NewSinglePool(servers...)
		if err != nil {
			return xerrors.WithStack(err)
		}
		c.cache = gomodule.NewModuleCache(cachePool, c.StorageEndpoint, c.StorageRegion, c.StorageBucket, c.StorageAccessKey, c.StorageSecretAccessKey, c.StorageCACertFile)
	} else {
		logger.Log.Debug("Disable cache")
	}

	return nil
}

func (c *goModuleProxyCommand) Run() error {
	stopErrCh := make(chan error, 1)
	startErrCh := make(chan error, 1)

	proxy := gomodule.NewModuleProxy(c.config, c.ModuleDir, c.cache, c.githubClientFactory.REST, c.githubClientFactory.TokenProvider, c.caBundle)
	server := gomodule.NewProxyServer(c.Addr, c.upstream, proxy)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	go func() {
		defer cancel()

		select {
		case <-ctx.Done():
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			logger.Log.Info("Shutting down the server")
			if err := server.Stop(ctx); err != nil {
				stopErrCh <- xerrors.WithStack(err)
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
			startErrCh <- xerrors.WithStack(err)
		}
	}()

	// Wait for stopping a server
	select {
	case err, ok := <-startErrCh:
		if ok {
			return xerrors.WithStack(err)
		}
	case err, ok := <-stopErrCh:
		if ok {
			return xerrors.WithStack(err)
		}
	}

	return nil
}
