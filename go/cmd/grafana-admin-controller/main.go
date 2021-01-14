package main

import (
	"context"
	"flag"
	"fmt"
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
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog"

	"go.f110.dev/mono/go/pkg/fsm"
	clientset "go.f110.dev/mono/go/pkg/k8s/client/versioned"
	"go.f110.dev/mono/go/pkg/k8s/controllers/grafana"
	informers "go.f110.dev/mono/go/pkg/k8s/informers/externalversions"
	"go.f110.dev/mono/go/pkg/logger"
)

const (
	stateInit fsm.State = iota
	stateLeaderElection
	stateStartWorker
	stateShutdown
)

type process struct {
	FSM    *fsm.FSM
	ctx    context.Context
	cancel context.CancelFunc

	userController *grafana.UserController

	id                 string
	dev                bool
	workers            int
	leaseLockName      string
	leaseLockNamespace string

	coreClient kubernetes.Interface
	client     clientset.Interface
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
	return stateStartWorker, nil
}

func (p *process) startWorker() (fsm.State, error) {
	coreSharedInformerFactory := kubeinformers.NewSharedInformerFactory(p.coreClient, 30*time.Second)
	sharedInformerFactory := informers.NewSharedInformerFactory(p.client, 30*time.Second)

	c, err := grafana.NewUserController(coreSharedInformerFactory, sharedInformerFactory, p.client)
	if err != nil {
		return fsm.Error(xerrors.Errorf(": %w", err))
	}
	p.userController = c

	coreSharedInformerFactory.Start(p.ctx.Done())
	sharedInformerFactory.Start(p.ctx.Done())

	p.userController.Run(p.ctx, p.workers)
	return fsm.WaitState, nil
}

func (p *process) shutdown() (fsm.State, error) {
	p.cancel()
	p.userController.Shutdown()
	return fsm.CloseState, nil
}

func main() {
	id := ""
	metricsAddr := ""
	enableLeaderElection := false
	leaseLockName := ""
	leaseLockNamespace := ""
	clusterDomain := ""
	workers := 1
	dev := false
	fs := pflag.NewFlagSet("grafana-admin-controller", pflag.ExitOnError)
	fs.StringVar(&id, "id", uuid.New().String(), "the holder identity name")
	fs.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	fs.BoolVar(&enableLeaderElection, "enable-leader-election", enableLeaderElection,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	fs.StringVar(&leaseLockName, "lease-lock-name", "", "the lease lock resource name")
	fs.StringVar(&leaseLockNamespace, "lease-lock-namespace", "", "the lease lock resource namespace")
	fs.StringVar(&clusterDomain, "cluster-domain", clusterDomain, "Cluster domain")
	fs.IntVar(&workers, "workers", workers, "The number of workers on each controller")
	fs.BoolVar(&dev, "dev", dev, "development mode")
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

	ctx, cancel := context.WithCancel(context.Background())
	p := &process{
		ctx:                ctx,
		cancel:             cancel,
		id:                 id,
		workers:            workers,
		dev:                dev,
		leaseLockName:      leaseLockName,
		leaseLockNamespace: leaseLockNamespace,
	}
	p.FSM = fsm.NewFSM(
		map[fsm.State]fsm.StateFunc{
			stateInit:           p.init,
			stateLeaderElection: p.leaderElection,
			stateStartWorker:    p.startWorker,
			stateShutdown:       p.shutdown,
		},
		stateInit,
		stateShutdown,
	)

	p.FSM.SignalHandling(os.Interrupt, syscall.SIGTERM)
	if err := p.FSM.Loop(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
