package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/pflag"
	"golang.org/x/xerrors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/component-base/metrics/legacyregistry"
	"k8s.io/klog"

	"go.f110.dev/mono/go/pkg/fsm"
	clientset "go.f110.dev/mono/go/pkg/k8s/client/versioned"
	"go.f110.dev/mono/go/pkg/k8s/controllers/grafana"
	"go.f110.dev/mono/go/pkg/k8s/controllers/harbor"
	"go.f110.dev/mono/go/pkg/k8s/controllers/minio"
	informers "go.f110.dev/mono/go/pkg/k8s/informers/externalversions"
	"go.f110.dev/mono/go/pkg/k8s/probe"
	"go.f110.dev/mono/go/pkg/logger"
)

const (
	stateInit fsm.State = iota
	stateStartMetricsServer
	stateLeaderElection
	stateStartWorkers
	stateShutdown
)

type controller interface {
	StartWorkers(ctx context.Context, workers int)
	Shutdown()
}

type process struct {
	FSM    *fsm.FSM
	ctx    context.Context
	cancel context.CancelFunc
	probe  *probe.Probe

	controllers []controller

	id                   string
	metricsAddr          string
	enableLeaderElection bool
	dev                  bool
	workers              int
	leaseLockName        string
	leaseLockNamespace   string
	clusterDomain        string
	harborNamespace      string
	harborServiceName    string
	adminSecretName      string
	coreConfigMapName    string

	config     *rest.Config
	coreClient *kubernetes.Clientset
	client     *clientset.Clientset
}

func newProcess() *process {
	ctx, cancel := context.WithCancel(context.Background())
	p := &process{
		ctx:         ctx,
		cancel:      cancel,
		controllers: make([]controller, 0),
		workers:     1,
	}
	p.FSM = fsm.NewFSM(
		map[fsm.State]fsm.StateFunc{
			stateInit:               p.init,
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

func (p *process) init() (fsm.State, error) {
	kubeconfigPath := ""
	if p.dev {
		h, err := os.UserHomeDir()
		if err != nil {
			return fsm.Error(xerrors.Errorf(": %w", err))
		}
		kubeconfigPath = filepath.Join(h, ".kube", "config")
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return fsm.Error(xerrors.Errorf(": %w", err))
	}
	p.config = cfg

	coreClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return fsm.Error(xerrors.Errorf(": %w", err))
	}
	p.coreClient = coreClient
	client, err := clientset.NewForConfig(cfg)
	if err != nil {
		return fsm.Error(xerrors.Errorf(": %w", err))
	}
	p.client = client

	p.probe = probe.NewProbe(p.metricsAddr)

	return stateStartMetricsServer, nil
}

func (p *process) startMetricsServer() (fsm.State, error) {
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

func (p *process) leaderElection() (fsm.State, error) {
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
		return fsm.Error(xerrors.Errorf(": %w", err))
	}
	go e.Run(p.ctx)

	select {
	case <-elected:
	case <-p.ctx.Done():
		return fsm.UnknownState, nil
	}
	return stateStartWorkers, nil
}

func (p *process) startWorkers() (fsm.State, error) {
	coreSharedInformerFactory := kubeinformers.NewSharedInformerFactory(p.coreClient, 30*time.Second)
	sharedInformerFactory := informers.NewSharedInformerFactory(p.client, 30*time.Second)

	gu, err := grafana.NewUserController(coreSharedInformerFactory, sharedInformerFactory, p.coreClient, p.client)
	if err != nil {
		return fsm.Error(xerrors.Errorf(": %w", err))
	}
	p.controllers = append(p.controllers, gu)

	hpc, err := harbor.NewProjectController(
		p.ctx,
		p.coreClient,
		p.client,
		p.config,
		sharedInformerFactory,
		p.harborNamespace,
		p.harborServiceName,
		p.adminSecretName,
		p.coreConfigMapName,
		p.dev,
	)
	if err != nil {
		return fsm.Error(xerrors.Errorf(": %w", err))
	}
	p.controllers = append(p.controllers, hpc)

	hrac, err := harbor.NewRobotAccountController(
		p.ctx,
		p.coreClient,
		p.client,
		p.config,
		sharedInformerFactory,
		p.harborNamespace,
		p.harborServiceName,
		p.adminSecretName,
		p.dev,
	)
	if err != nil {
		return fsm.Error(xerrors.Errorf(": %w", err))
	}
	p.controllers = append(p.controllers, hrac)

	mbc, err := minio.NewBucketController(p.coreClient, p.client, p.config, sharedInformerFactory, p.dev)
	if err != nil {
		return fsm.Error(xerrors.Errorf(": %w", err))
	}
	p.controllers = append(p.controllers, mbc)

	coreSharedInformerFactory.Start(p.ctx.Done())
	sharedInformerFactory.Start(p.ctx.Done())

	for _, v := range p.controllers {
		v.StartWorkers(p.ctx, p.workers)
	}
	return fsm.WaitState, nil
}

func (p *process) shutdown() (fsm.State, error) {
	p.cancel()

	for _, v := range p.controllers {
		v.Shutdown()
	}

	return fsm.CloseState, nil
}

func main() {
	p := newProcess()
	fs := pflag.NewFlagSet("controller-manager", pflag.ExitOnError)
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
	logger.Flags(fs)

	goFlagSet := flag.NewFlagSet("", flag.ContinueOnError)
	klog.InitFlags(goFlagSet)
	fs.AddGoFlagSet(goFlagSet)

	if err := fs.Parse(os.Args[1:]); err != nil {
		panic(err)
	}

	if err := logger.OverrideKlog(); err != nil {
		panic(err)
	}

	if err := p.FSM.Loop(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
