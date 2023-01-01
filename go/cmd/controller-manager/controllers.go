package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/vault/api"
	"github.com/spf13/pflag"
	"go.f110.dev/xerrors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/component-base/metrics/legacyregistry"

	"go.f110.dev/mono/go/fsm"
	"go.f110.dev/mono/go/k8s/probe"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/pkg/k8s/client"
	"go.f110.dev/mono/go/pkg/k8s/controllers/consul"
	"go.f110.dev/mono/go/pkg/k8s/controllers/grafana"
	"go.f110.dev/mono/go/pkg/k8s/controllers/harbor"
	"go.f110.dev/mono/go/pkg/k8s/controllers/minio"
)

const (
	ControllerGrafanaUser        = "grafana-user"
	ControllerHarborProject      = "harbor-project"
	ControllerHarborRobotAccount = "harbor-robot-account"
	ControllerMinIOBucket        = "minio-bucket"
	ControllerMinIOUser          = "minio-user"
	ControllerConsulBackup       = "consul-backup"
)

type ChildController struct {
	Name string
	New  func(*Controllers, kubeinformers.SharedInformerFactory, *client.InformerFactory) (controller, error)
}

type ChildControllers []ChildController

var AllChildControllers = ChildControllers{
	{
		Name: ControllerGrafanaUser,
		New: func(p *Controllers, core kubeinformers.SharedInformerFactory, factory *client.InformerFactory) (controller, error) {
			gu, err := grafana.NewUserController(core, factory, p.coreClient, p.client)
			if err != nil {
				return nil, xerrors.WithStack(err)
			}
			return gu, nil
		},
	},
	{
		Name: ControllerHarborProject,
		New: func(p *Controllers, _ kubeinformers.SharedInformerFactory, factory *client.InformerFactory) (controller, error) {
			hpc, err := harbor.NewProjectController(
				p.ctx,
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
	},
	{
		Name: ControllerHarborRobotAccount,
		New: func(p *Controllers, _ kubeinformers.SharedInformerFactory, factory *client.InformerFactory) (controller, error) {
			hrac, err := harbor.NewRobotAccountController(
				p.ctx,
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
	},
	{
		Name: ControllerMinIOBucket,
		New: func(p *Controllers, core kubeinformers.SharedInformerFactory, factory *client.InformerFactory) (controller, error) {
			mbc, err := minio.NewBucketController(p.coreClient, p.client, p.config, core, factory, p.dev)
			if err != nil {
				return nil, xerrors.WithStack(err)
			}
			return mbc, nil
		},
	},
	{
		Name: ControllerMinIOUser,
		New: func(p *Controllers, core kubeinformers.SharedInformerFactory, factory *client.InformerFactory) (controller, error) {
			muc, err := minio.NewUserController(
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
	},
	{
		Name: ControllerConsulBackup,
		New: func(p *Controllers, core kubeinformers.SharedInformerFactory, factory *client.InformerFactory) (controller, error) {
			b, err := consul.NewBackupController(core, factory, p.coreClient, p.client, p.config, p.dev)
			if err != nil {
				return nil, xerrors.WithStack(err)
			}
			return b, nil
		},
	},
}

func NewChildControllers(args []string) (ChildControllers, error) {
	if len(args) == 0 {
		return AllChildControllers, nil
	}

	c := make(map[string]ChildController)
	for _, v := range AllChildControllers {
		c[v.Name] = v
	}

	childControllers := make(ChildControllers, 0)
	for _, v := range args {
		cont, ok := c[v]
		if !ok {
			return nil, xerrors.Newf("%s is unknown controller", v)
		}

		childControllers = append(childControllers, cont)
	}

	return childControllers, nil
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
	args   []string
	ctx    context.Context
	cancel context.CancelFunc
	probe  *probe.Probe

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
	vaultClient *api.Client
}

func New(args []string) *Controllers {
	ctx, cancel := context.WithCancel(context.Background())
	p := &Controllers{
		args:        args,
		ctx:         ctx,
		cancel:      cancel,
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
	p.FSM.SignalHandling(os.Interrupt, syscall.SIGTERM)

	return p
}

func (p *Controllers) Flags(fs *pflag.FlagSet) {
	fs.StringVar(&p.id, "id", uuid.New().String(), "the holder identity name")
	fs.StringVar(&p.metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	fs.BoolVar(&p.enableLeaderElection, "enable-leader-election", p.enableLeaderElection,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	fs.StringVar(&p.leaseLockName, "lease-lock-name", "", "the lease lock resource name")
	fs.StringVar(&p.leaseLockNamespace, "lease-lock-namespace", "", "the lease lock resource namespace")
	fs.StringVar(&p.clusterDomain, "cluster-domain", p.clusterDomain, "Cluster domain")
	fs.IntVar(&p.workers, "workers", p.workers, "The number of workers on each controller")
	fs.StringVar(&p.harborNamespace, "harbor-namespace", "", "the namespace name to which harbor service belongs")
	fs.StringVar(&p.harborServiceName, "harbor-service-name", "", "the service name of harbor")
	fs.StringVar(&p.adminSecretName, "admin-secret-name", "", "the secret name that including admin password")
	fs.StringVar(&p.coreConfigMapName, "core-configmap-name", "", "the configmap name that used harbor core")
	fs.BoolVar(&p.dev, "dev", p.dev, "development mode")
	fs.StringVar(&p.serviceAccountTokenFile, "service-account-token-file", "/var/run/secrets/kubernetes.io/serviceaccount/token", "a file path that contains JWT token")
	fs.StringVar(&p.vaultAddr, "vault-addr", "http://127.0.0.1:8200", "the address to vault")
	fs.StringVar(&p.vaultToken, "vault-token", "", "the token for vault")
	fs.StringVar(&p.vaultK8sAuthPath, "vault-k8s-auth-path", "auth/kubernetes", "The mount path of kubernetes auth method")
	fs.StringVar(&p.vaultK8sAuthRole, "vault-k8s-auth-role", "", "Role name for k8s auth method")
	logger.Flags(fs)
}

func (p *Controllers) init() (fsm.State, error) {
	fs := pflag.NewFlagSet("controller-manager", pflag.ExitOnError)
	p.Flags(fs)
	goFlagSet := flag.NewFlagSet("", flag.ContinueOnError)
	fs.AddGoFlagSet(goFlagSet)
	if err := fs.Parse(p.args); err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	if err := logger.OverrideKlog(); err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	p.args = fs.Args()[1:]

	kubeconfigPath := ""
	if p.dev {
		h, err := os.UserHomeDir()
		if err != nil {
			return fsm.Error(xerrors.WithStack(err))
		}
		kubeconfigPath = filepath.Join(h, ".kube", "config")
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
		vaultClient, err := api.NewClient(&api.Config{Address: p.vaultAddr})
		if err != nil {
			return fsm.Error(xerrors.WithStack(err))
		}
		if p.vaultToken != "" {
			vaultClient.SetToken(p.vaultToken)
		} else if _, err := os.Stat(p.serviceAccountTokenFile); err == nil {
			buf, err := os.ReadFile(p.serviceAccountTokenFile)
			if err != nil {
				return fsm.Error(xerrors.WithStack(err))
			}
			saToken := strings.TrimSpace(string(buf))
			data, err := vaultClient.Logical().Write(
				fmt.Sprintf("%s/login", p.vaultK8sAuthPath),
				map[string]interface{}{
					"jwt":  saToken,
					"role": p.vaultK8sAuthRole,
				},
			)
			if err != nil {
				return fsm.Error(xerrors.WithStack(err))
			}
			vaultClient.SetToken(data.Auth.ClientToken)
		}
		p.vaultClient = vaultClient
	}

	return stateCheckResources, nil
}

func (p *Controllers) checkResources() (fsm.State, error) {
	_, apiList, err := p.coreClient.Discovery().ServerGroupsAndResources()
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
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
		return fsm.Error(xerrors.New("minio-operator is not installed"))
	}

	return stateStartMetricsServer, nil
}

func (p *Controllers) startMetricsServer() (fsm.State, error) {
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

	return stateLeaderElection, nil
}

func (p *Controllers) leaderElection() (fsm.State, error) {
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
	go e.Run(p.ctx)

	select {
	case <-elected:
	case <-p.ctx.Done():
		return fsm.UnknownState, nil
	}
	return stateStartWorkers, nil
}

func (p *Controllers) startWorkers() (fsm.State, error) {
	coreSharedInformerFactory := kubeinformers.NewSharedInformerFactory(p.coreClient, 30*time.Second)
	factory := client.NewInformerFactory(p.client, client.NewInformerCache(), metav1.NamespaceAll, 30*time.Second)

	child, err := NewChildControllers(p.args)
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	for _, v := range child {
		cont, err := v.New(p, coreSharedInformerFactory, factory)
		if err != nil {
			return fsm.Error(xerrors.WithMessagef(err, "Failed create %s controller", v.Name))
		}
		p.controllers = append(p.controllers, cont)
	}

	coreSharedInformerFactory.Start(p.ctx.Done())
	factory.Run(p.ctx)

	for _, v := range p.controllers {
		v.StartWorkers(p.ctx, p.workers)
	}
	return fsm.WaitState, nil
}

func (p *Controllers) shutdown() (fsm.State, error) {
	p.cancel()

	for _, v := range p.controllers {
		v.Shutdown()
	}

	return fsm.CloseState, nil
}
