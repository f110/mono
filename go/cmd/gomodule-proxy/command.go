package main

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"go.f110.dev/go-memcached/client"

	"go.f110.dev/mono/go/fsm"
	"go.f110.dev/mono/go/githubutil"
	"go.f110.dev/mono/go/gomodule"
	"go.f110.dev/mono/go/logger"
)

type goModuleProxyCommand struct {
	*fsm.FSM

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
	server   *gomodule.ProxyServer

	githubClientFactory *githubutil.GitHubClientFactory
}

const (
	stateInit fsm.State = iota
	stateStartServer
	stateShuttingDown
)

func newGoModuleProxyCommand() *goModuleProxyCommand {
	c := &goModuleProxyCommand{
		Addr:                ":7589",
		UpstreamURL:         "https://proxy.golang.org",
		githubClientFactory: githubutil.NewGitHubClientFactory("gomodule-proxy", false),
	}
	c.FSM = fsm.NewFSM(
		map[fsm.State]fsm.StateFunc{
			stateInit:         c.init,
			stateStartServer:  c.startServer,
			stateShuttingDown: c.shuttingDown,
		},
		stateInit,
		stateShuttingDown,
	)
	c.FSM.CloseContext = func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.Background(), 5*time.Second)
	}
	return c
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

func (c *goModuleProxyCommand) init(_ context.Context) (fsm.State, error) {
	if err := c.githubClientFactory.Init(); err != nil {
		return fsm.Error(err)
	}

	conf, err := gomodule.ReadConfig(c.ConfigPath)
	if err != nil {
		return fsm.Error(err)
	}
	c.config = conf

	uu, err := url.Parse(c.UpstreamURL)
	if err != nil {
		return fsm.Error(err)
	}
	c.upstream = uu

	if c.CABundleFile != "" {
		b, err := os.ReadFile(c.CABundleFile)
		if err != nil {
			return fsm.Error(err)
		}
		c.caBundle = b
	}

	if c.StorageEndpoint != "" && c.StorageRegion != "" &&
		c.StorageBucket != "" && c.StorageAccessKey != "" && c.StorageSecretAccessKey != "" && len(c.MemcachedServers) > 0 {
		var servers []client.Server
		for _, v := range c.MemcachedServers {
			s := strings.SplitN(v, "=", 2)
			server, err := client.NewServerWithMetaProtocol(context.Background(), s[0], "tcp", s[1], client.EnableHeartbeat, client.EnableAutoReconnect)
			if err != nil {
				return fsm.Error(err)
			}
			servers = append(servers, server)
		}
		cachePool, err := client.NewSinglePool(servers...)
		if err != nil {
			return fsm.Error(err)
		}
		c.cache = gomodule.NewModuleCache(cachePool, c.StorageEndpoint, c.StorageRegion, c.StorageBucket, c.StorageAccessKey, c.StorageSecretAccessKey, c.StorageCACertFile)
	} else {
		logger.Log.Debug("Disable cache")
	}

	proxy := gomodule.NewModuleProxy(c.config, c.ModuleDir, c.cache, c.githubClientFactory.REST, c.githubClientFactory.TokenProvider, c.caBundle)
	c.server = gomodule.NewProxyServer(c.Addr, c.upstream, proxy)

	return fsm.Next(stateStartServer)
}

func (c *goModuleProxyCommand) startServer(_ context.Context) (fsm.State, error) {
	go func() {
		if err := c.server.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Warn("Server returns error", logger.Error(err))
		}
	}()

	return fsm.Wait()
}

func (c *goModuleProxyCommand) shuttingDown(ctx context.Context) (fsm.State, error) {
	if c.server != nil {
		logger.Log.Info("Shutting down proxy")
		if err := c.server.Stop(ctx); err != nil {
			return fsm.Error(err)
		}
	}

	return fsm.Finish()
}
