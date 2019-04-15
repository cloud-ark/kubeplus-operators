package main

import (
	"context"
	"flag"
	"sync"
	"time"

	"github.com/golang/glog"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	clientset "github.com/cloud-ark/kubeplus-operators/moodle/pkg/client/clientset/versioned"
	informers "github.com/cloud-ark/kubeplus-operators/moodle/pkg/client/informers/externalversions"
	"github.com/cloud-ark/kubeplus-operators/moodle/pkg/signals"
)

var (
	masterURL  string
	kubeconfig string
)

func main() {
	flag.Parse()
	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)

	ctx, cancel := context.WithCancel(context.Background())
	stopCh := signals.SetupSignalHandler()

	if err != nil {
		glog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient := kubernetes.NewForConfigOrDie(cfg)
	moodleClient := clientset.NewForConfigOrDie(cfg)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	moodleInformerFactory := informers.NewSharedInformerFactory(moodleClient, time.Second*30)
	moodleController := NewMoodleController(cfg, kubeClient, moodleClient, kubeInformerFactory, moodleInformerFactory)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		moodleController.Run(1, ctx.Done())
	}()

	podController := NewPodController(cfg,
		kubeClient,
		moodleInformerFactory.Moodlecontroller().V1().Moodles().Lister(),
		kubeInformerFactory)

	wg.Add(1)
	go func() {
		defer wg.Done()
		podController.Run(1, ctx.Done())
	}()
	// https://github.com/kubernetes/sample-controller/blob/master/main.go
	// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	kubeInformerFactory.Start(ctx.Done())
	moodleInformerFactory.Start(ctx.Done())
	<-stopCh
	cancel()
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
