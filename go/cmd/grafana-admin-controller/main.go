package main

import (
	"context"
	"flag"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog"

	clientset "go.f110.dev/mono/go/pkg/k8s/client/versioned"
	"go.f110.dev/mono/go/pkg/k8s/controllers/grafana"
	informers "go.f110.dev/mono/go/pkg/k8s/informers/externalversions"
	"go.f110.dev/mono/lib/logger"
	"go.f110.dev/mono/lib/signals"
)

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

	ctx, cancelFunc := context.WithCancel(context.Background())
	signals.SetupSignalHandler(cancelFunc)

	kubeconfigPath := ""
	if dev {
		h, err := os.UserHomeDir()
		if err != nil {
			klog.Error(err)
			os.Exit(1)
		}
		kubeconfigPath = filepath.Join(h, ".kube", "config")
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}

	coreClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
	client, err := clientset.NewForConfig(cfg)
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}

	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      leaseLockName,
			Namespace: leaseLockNamespace,
		},
		Client: coreClient.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: id,
		},
	}
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   30 * time.Second,
		RenewDeadline:   15 * time.Second,
		RetryPeriod:     5 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				coreSharedInformerFactory := kubeinformers.NewSharedInformerFactory(coreClient, 30*time.Second)
				sharedInformerFactory := informers.NewSharedInformerFactory(client, 30*time.Second)

				c, err := grafana.NewAdminController(coreSharedInformerFactory, sharedInformerFactory, client)
				if err != nil {
					logger.Log.Error("Failed start admin controller", zap.Error(err))
					return
				}

				coreSharedInformerFactory.Start(ctx.Done())
				sharedInformerFactory.Start(ctx.Done())

				var wg sync.WaitGroup

				wg.Add(1)
				go func() {
					defer wg.Done()

					c.Run(ctx, workers)
				}()

				wg.Wait()
			},
			OnStoppedLeading: func() {
				logger.Log.Debug("Leader lost", zap.String("id", id))
				os.Exit(0)
			},
			OnNewLeader: func(identity string) {
				if identity == id {
					return
				}
				logger.Log.Debug("New leader elected", zap.String("id", identity))
			},
		},
	})
}
