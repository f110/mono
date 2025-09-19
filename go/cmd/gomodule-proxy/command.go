package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"go.f110.dev/go-memcached/client"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/ctxutil"
	"go.f110.dev/mono/go/fsm"
	"go.f110.dev/mono/go/githubutil"
	"go.f110.dev/mono/go/gomodule"
	"go.f110.dev/mono/go/logger"
)

type goModuleProxyCommand struct {
	*fsm.FSM

	ConfigPath      string
	ModuleDir       string
	Addr            string
	UpstreamURL     string
	CABundleFile    string
	SigningKeyFile  string
	RemoveBazelFile bool

	StorageEndpoint        string
	StorageRegion          string
	StorageAccessKey       string
	StorageSecretAccessKey string
	StorageBucket          string
	StorageCACertFile      string

	MemcachedServers []string

	upstream   *url.URL
	config     gomodule.Config
	cache      *gomodule.ModuleCache
	caBundle   []byte
	signingKey *ecdsa.PrivateKey
	server     *gomodule.ProxyServer

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
		return ctxutil.WithTimeout(context.Background(), 5*time.Second)
	}
	return c
}

func (c *goModuleProxyCommand) Flags(fs *cli.FlagSet) {
	fs.String("config", "Configuration file path").Var(&c.ConfigPath).Shorthand("c").Default(c.ConfigPath).Required()
	fs.String("mod-dir", "Module directory").Var(&c.ModuleDir).Default(c.ModuleDir)
	fs.String("addr", "Listen addr").Var(&c.Addr).Default(c.Addr)
	fs.String("upstream", "Upstream module proxy URL").Var(&c.UpstreamURL).Default(c.UpstreamURL)
	fs.String("ca-bundle-file", "A file path that contains ca certificate to clone a repository").Var(&c.CABundleFile).Default(c.CABundleFile)
	fs.String("storage-endpoint", "The endpoint of object storage").Var(&c.StorageEndpoint).Default(c.StorageEndpoint)
	fs.String("storage-region", "The name of region of object storage").Var(&c.StorageRegion).Default(c.StorageRegion)
	fs.String("storage-bucket", "The name of bucket for an archive file").Var(&c.StorageBucket).Default(c.StorageBucket)
	fs.String("storage-access-key", "Access key").Var(&c.StorageAccessKey).Default(c.StorageAccessKey)
	fs.String("storage-secret-access-key", "Secret access key").Var(&c.StorageSecretAccessKey).Default(c.StorageSecretAccessKey)
	fs.String("storage-ca-file", "File path that contains the certificate of CA").Var(&c.StorageCACertFile).Default(c.StorageCACertFile)
	fs.StringArray("memcached-servers", "Memcached server name and address for the metadata cache").Var(&c.MemcachedServers)
	fs.String("signing-key-file", "A file path that contains signing key").Var(&c.SigningKeyFile)
	fs.Bool("remove-bazel-file", "Remove bazel related files. CAUTION: This flag may cause a checksum mismatch").Var(&c.RemoveBazelFile)

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

	var proxyOpts []gomodule.ProxyServerOption
	if c.SigningKeyFile != "" {
		buf, err := os.ReadFile(c.SigningKeyFile)
		if err != nil {
			return fsm.Error(err)
		}
		block, _ := pem.Decode(buf)
		signingKey, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return fsm.Error(err)
		}
		c.signingKey = signingKey
		proxyOpts = append(proxyOpts, gomodule.WithGitHubUserAuthentication(gomodule.NewUserAuthentication(c.signingKey, nil)))
	}

	var moduleProxyOpts []gomodule.ModuleProxyOption
	if c.RemoveBazelFile {
		moduleProxyOpts = append(moduleProxyOpts, gomodule.RemoveBazelFiles(true))
	}

	proxy := gomodule.NewModuleProxy(c.config, c.ModuleDir, c.cache, c.githubClientFactory.REST, c.githubClientFactory.TokenProvider, c.caBundle, moduleProxyOpts...)
	c.server = gomodule.NewProxyServer(c.Addr, c.upstream, proxy, c.githubClientFactory, proxyOpts...)

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
