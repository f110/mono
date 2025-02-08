package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/component-base/metrics/legacyregistry"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/ctxutil"
	"go.f110.dev/mono/go/fsm"
	"go.f110.dev/mono/go/k8s/client"
	"go.f110.dev/mono/go/k8s/controllers"
	"go.f110.dev/mono/go/k8s/probe"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/vault"
)

const (
	ControllerGrafanaUser        = "grafana-user"
	ControllerHarborProject      = "harbor-project"
	ControllerHarborRobotAccount = "harbor-robot-account"
	ControllerMinIOCluster       = "minio-cluster"
	ControllerMinIOBucket        = "minio-bucket"
	ControllerMinIOUser          = "minio-user"
	ControllerConsulBackup       = "consul-backup"
)

type ChildController struct {
	Name   string
	New    func(context.Context, *Controllers, kubeinformers.SharedInformerFactory, *client.InformerFactory) (controller, error)
	Enable bool
}

type ChildControllers []*ChildController

var AllChildControllers = ChildControllers{
	{
		Name: ControllerGrafanaUser,
		New: func(_ context.Context, p *Controllers, core kubeinformers.SharedInformerFactory, factory *client.InformerFactory) (controller, error) {
			gu, err := controllers.NewGrafanaController(core, factory, p.coreClient, p.client)
			if err != nil {
				return nil, xerrors.WithStack(err)
			}
			return gu, nil
		},
		Enable: true,
	},
	{
		Name: ControllerHarborProject,
		New: func(ctx context.Context, p *Controllers, _ kubeinformers.SharedInformerFactory, factory *client.InformerFactory) (controller, error) {
			hpc, err := controllers.NewHarborProjectController(
				ctx,
				p.coreClient,
				p.client,
				p.config,
				factory,
				p.harborNamespace,
				p.harborServiceName,
				p.adminSecretName,
				p.coreConfigMapName,
				p.dev,
			)
			if err != nil {
				return nil, xerrors.WithStack(err)
			}
			return hpc, nil
		},
		Enable: true,
	},
	{
		Name: ControllerHarborRobotAccount,
		New: func(ctx context.Context, p *Controllers, _ kubeinformers.SharedInformerFactory, factory *client.InformerFactory) (controller, error) {
			hrac, err := controllers.NewHarborRobotAccountController(
				ctx,
				p.coreClient,
				p.client,
				p.config,
				factory,
				p.harborNamespace,
				p.harborServiceName,
				p.adminSecretName,
				p.dev,
			)
			if err != nil {
				return nil, xerrors.WithStack(err)
			}
			return hrac, nil
		},
		Enable: true,
	},
	{
		Name: ControllerMinIOCluster,
		New: func(_ context.Context, p *Controllers, core kubeinformers.SharedInformerFactory, factory *client.InformerFactory) (controller, error) {
			mcc := controllers.NewMinIOClusterController(p.coreClient, p.client, p.config, core, factory, p.dev)
			return mcc, nil
		},
		Enable: true,
	},
	{
		Name: ControllerMinIOBucket,
		New: func(_ context.Context, p *Controllers, core kubeinformers.SharedInformerFactory, factory *client.InformerFactory) (controller, error) {
			mbc, err := controllers.NewMinIOBucketController(p.coreClient, p.client, p.config, core, factory, p.dev)
			if err != nil {
				return nil, xerrors.WithStack(err)
			}
			return mbc, nil
		},
		Enable: true,
	},
	{
		Name: ControllerMinIOUser,
		New: func(_ context.Context, p *Controllers, core kubeinformers.SharedInformerFactory, factory *client.InformerFactory) (controller, error) {
			muc, err := controllers.NewMinIOUserController(
				p.coreClient,
				p.client,
				p.config,
				core,
				factory,
				p.vaultClient,
				p.dev,
			)
			if err != nil {
				return nil, xerrors.WithStack(err)
			}
			return muc, nil
		},
		Enable: true,
	},
	{
		Name: ControllerConsulBackup,
		New: func(_ context.Context, p *Controllers, core kubeinformers.SharedInformerFactory, factory *client.InformerFactory) (controller, error) {
			b, err := controllers.NewConsulBackupController(core, factory, p.coreClient, p.client, p.config, p.dev)
			if err != nil {
				return nil, xerrors.WithStack(err)
			}
			return b, nil
		},
		Enable: true,
	},
}

const (
	stateInit fsm.State = iota
	stateCheckResources
	stateStartMetricsServer
	stateLeaderElection
	stateStartWorkers
	stateShutdown
)

type controller interface {
	StartWorkers(ctx context.Context, workers int)
	Shutdown()
}

type Controllers struct {
	*fsm.FSM
	args  []string
	probe *probe.Probe

	controllers []controller

	id                      string
	metricsAddr             string
	enableLeaderElection    bool
	dev                     bool
	workers                 int
	leaseLockName           string
	leaseLockNamespace      string
	clusterDomain           string
	harborNamespace         string
	harborServiceName       string
	adminSecretName         string
	coreConfigMapName       string
	serviceAccountTokenFile string
	vaultAddr               string
	vaultToken              string
	vaultK8sAuthPath        string
	vaultK8sAuthRole        string

	config      *rest.Config
	coreClient  *kubernetes.Clientset
	client      *client.Set
	vaultClient *vault.Client
}

func New(args []string) *Controllers {
	p := &Controllers{
		args:        args,
		controllers: make([]controller, 0),
		workers:     1,
	}
	p.FSM = fsm.NewFSM(
		map[fsm.State]fsm.StateFunc{
			stateInit:               p.init,
			stateCheckResources:     p.checkResources,
			stateStartMetricsServer: p.startMetricsServer,
			stateLeaderElection:     p.leaderElection,
			stateStartWorkers:       p.startWorkers,
			stateShutdown:           p.shutdown,
		},
		stateInit,
		stateShutdown,
	)

	return p
}

func (p *Controllers) Flags(fs *cli.FlagSet) {
	fs.String("id", "the holder identity name").Var(&p.id).Default(uuid.New().String())
	fs.String("metrics-addr", "The address the metric endpoint binds to.").Var(&p.metricsAddr).Default(":8081")
	fs.Bool("enable-leader-election",
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.").Var(&p.enableLeaderElection).Default(p.enableLeaderElection)
	fs.String("lease-lock-name", "the lease lock resource name").Var(&p.leaseLockName).Default("mono")
	fs.String("lease-lock-namespace", "the lease lock resource namespace").Var(&p.leaseLockNamespace).Default("default")
	fs.String("cluster-domain", "Cluster domain").Var(&p.clusterDomain).Default(p.clusterDomain)
	fs.Int("workers", "The number of workers on each controller").Var(&p.workers).Default(p.workers)
	fs.String("harbor-namespace", "the namespace name to which harbor service belongs").Var(&p.harborNamespace)
	fs.String("harbor-service-name", "the service name of harbor").Var(&p.harborServiceName)
	fs.String("admin-secret-name", "the secret name that including admin password").Var(&p.adminSecretName)
	fs.String("core-configmap-name", "the configmap name that used harbor core").Var(&p.coreConfigMapName)
	fs.Bool("dev", "development mode").Var(&p.dev).Default(p.dev)
	fs.String("service-account-token-file", "a file path that contains JWT token").Var(&p.serviceAccountTokenFile).Default("/var/run/secrets/kubernetes.io/serviceaccount/token")
	fs.String("vault-addr", "the address to vault").Var(&p.vaultAddr).Default("http://127.0.0.1:8200")
	fs.String("vault-token", "the token for vault").Var(&p.vaultToken)
	fs.String("vault-k8s-auth-path", "The mount path of kubernetes auth method").Var(&p.vaultK8sAuthPath).Default("auth/kubernetes")
	fs.String("vault-k8s-auth-role", "Role name for k8s auth method").Var(&p.vaultK8sAuthRole)
}

func (p *Controllers) init(ctx context.Context) (fsm.State, error) {
	if err := logger.OverrideKlog(); err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}

	kubeconfigPath := ""
	if p.dev {
		if v := os.Getenv("BUILD_WORKSPACE_DIRECTORY"); v != "" {
			kubeconfigPath = filepath.Join(v, ".kubeconfig")
			logger.Log.Info("Use local kubeconfig", zap.String("path", kubeconfigPath))
		} else {
			h, err := os.UserHomeDir()
			if err != nil {
				return fsm.Error(xerrors.WithStack(err))
			}
			kubeconfigPath = filepath.Join(h, ".kube", "config")
		}
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	p.config = cfg

	coreClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	p.coreClient = coreClient
	apiClientset, err := client.NewSet(cfg)
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	p.client = apiClientset

	p.probe = probe.NewProbe(p.metricsAddr)

	if p.vaultAddr != "" {
		if p.vaultToken != "" {
			vc, err := vault.NewClient(p.vaultAddr, p.vaultToken)
			if err != nil {
				return fsm.Error(err)
			}
			p.vaultClient = vc
		} else if _, err := os.Stat(p.serviceAccountTokenFile); err == nil {
			buf, err := os.ReadFile(p.serviceAccountTokenFile)
			if err != nil {
				return fsm.Error(xerrors.WithStack(err))
			}
			saToken := strings.TrimSpace(string(buf))
			logger.Log.Info("Using a service account for Vault authentication")
			ctx, cancel := ctxutil.WithTimeout(ctx, 5*time.Second)
			vc, err := vault.NewClientAsK8SServiceAccount(ctx, p.vaultAddr, p.vaultK8sAuthPath, p.vaultK8sAuthRole, saToken)
			if err != nil {
				cancel()
				return fsm.Error(err)
			}
			cancel()
			p.vaultClient = vc
		}
	}

	if len(p.args) != 0 {
		c := make(map[string]*ChildController)
		for _, v := range AllChildControllers {
			c[v.Name] = v
			v.Enable = false
		}

		for _, v := range p.args {
			cont, ok := c[v]
			if !ok {
				continue
			}

			logger.Log.Debug("Enable controller", zap.String("name", cont.Name))
			cont.Enable = true
		}
	}
	return fsm.Next(stateCheckResources)
}

func (p *Controllers) checkResources(_ context.Context) (fsm.State, error) {
	logger.Log.Info("Check custom resource definitions")
	_, apiList, err := p.coreClient.Discovery().ServerGroupsAndResources()
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	enabledBucketController := false
	for _, v := range AllChildControllers {
		if v.Name == ControllerMinIOBucket && v.Enable {
			enabledBucketController = true
			break
		}
	}
	if enabledBucketController {
		found := false
		for _, v := range apiList {
			if v.GroupVersion == "miniocontroller.min.io/v1beta1" {
				for _, v := range v.APIResources {
					if v.Kind == "MinIOInstance" {
						found = true
						break
					}
				}
			}
		}
		if !found {
			return fsm.Error(xerrors.Define("minio-operator is not installed").WithStack())
		}
	}

	return fsm.Next(stateStartMetricsServer)
}

func (p *Controllers) startMetricsServer(_ context.Context) (fsm.State, error) {
	logger.Log.Info("Start metrics server")
	http.Handle("/metrics", legacyregistry.HandlerWithReset())
	go http.ListenAndServe(":9300", nil)

	go func() {
		for {
			select {
			case c := <-p.probe.Readiness():
				close(c)
			}
		}
	}()

	return fsm.Next(stateLeaderElection)
}

func (p *Controllers) leaderElection(ctx context.Context) (fsm.State, error) {
	if !p.enableLeaderElection || p.dev {
		return fsm.Next(stateStartWorkers)
	}

	logger.Log.Info("Start leader election")
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      p.leaseLockName,
			Namespace: p.leaseLockNamespace,
		},
		Client: p.coreClient.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: p.id,
		},
	}

	elected := make(chan struct{})
	e, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   30 * time.Second,
		RenewDeadline:   15 * time.Second,
		RetryPeriod:     5 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(_ context.Context) {
				close(elected)
			},
			OnStoppedLeading: func() {
				p.FSM.Shutdown()
			},
			OnNewLeader: func(_ string) {},
		},
	})
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	go e.Run(ctx)

	select {
	case <-elected:
	case <-ctx.Done():
		return fsm.UnknownState, nil
	}
	return fsm.Next(stateStartWorkers)
}

func (p *Controllers) startWorkers(ctx context.Context) (fsm.State, error) {
	logger.Log.Info("Start workers")
	coreSharedInformerFactory := kubeinformers.NewSharedInformerFactory(p.coreClient, 30*time.Second)
	factory := client.NewInformerFactory(p.client, client.NewInformerCache(), metav1.NamespaceAll, 30*time.Second)

	for _, v := range AllChildControllers {
		if !v.Enable {
			continue
		}

		cont, err := v.New(ctx, p, coreSharedInformerFactory, factory)
		if err != nil {
			return fsm.Error(xerrors.WithMessagef(err, "Failed create %s controller", v.Name))
		}
		p.controllers = append(p.controllers, cont)
	}

	coreSharedInformerFactory.Start(ctx.Done())
	factory.Run(ctx)

	for _, v := range p.controllers {
		v.StartWorkers(ctx, p.workers)
	}
	return fsm.Wait()
}

func (p *Controllers) shutdown(_ context.Context) (fsm.State, error) {
	for _, v := range p.controllers {
		v.Shutdown()
	}

	return fsm.Finish()
}
