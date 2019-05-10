package main

import (
	"fmt"
	"time"

	_ "github.com/lib/pq"
	restclient "k8s.io/client-go/rest"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	operatorscheme "github.com/cloud-ark/kubeplus-operators/moodle/pkg/client/clientset/versioned/scheme"
	listers "github.com/cloud-ark/kubeplus-operators/moodle/pkg/client/listers/moodlecontroller/v1"
	"github.com/cloud-ark/kubeplus-operators/moodle/pkg/utils"
	"github.com/cloud-ark/kubeplus-operators/moodle/pkg/utils/constants"
	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

// PodController handles Moodle pods
type PodController struct {
	cfg *restclient.Config
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	moodleLister  listers.MoodleLister
	podsLister    corelisters.PodLister
	podsSynced    cache.InformerSynced
	podqueue      workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
	util     utils.Utils
}

// NewController returns a new sample controller
func NewPodController(
	cfg *restclient.Config,
	kubeclientset kubernetes.Interface,
	moodleLister_ listers.MoodleLister,
	kubeInformerFactory kubeinformers.SharedInformerFactory) *PodController {

	podInformer := kubeInformerFactory.Core().V1().Pods()

	// Create event broadcaster
	// Add moodle-controller types to the default Kubernetes Scheme so Events can be
	// logged for moodle-controller types.
	operatorscheme.AddToScheme(scheme.Scheme)
	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})
	utils := utils.NewUtils(cfg, kubeclientset)

	controller := &PodController{
		cfg:           cfg,
		kubeclientset: kubeclientset,
		moodleLister:  moodleLister_,
		podsLister:    podInformer.Lister(),
		podsSynced:    podInformer.Informer().HasSynced,
		podqueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Pods"),
		recorder:      recorder,
		util:          utils,
	}

	glog.Info("Setting up event handlers")

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueuePod})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *PodController) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.podqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	glog.Info("Starting Pod controller")
	//
	// // Wait for the caches to be synced before starting workers
	// glog.Info("Waiting for informer caches to sync")
	// if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced, c.foosSynced); !ok {
	// 	return fmt.Errorf("failed to wait for caches to sync")
	// }

	glog.Info("Starting workers")

	// Launch two workers to process Moodle resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second*20, stopCh)
	}

	glog.Info("Started workers")
	<-stopCh
	glog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processPodQueueItem function in order to read and process a message on the
// workqueue.
func (c *PodController) runWorker() {
	for c.processPodQueueItem() {
	}
}
func (c *PodController) processPodQueueItem() bool {
	podObj, shutdown := c.podqueue.Get()
	if shutdown {
		return false
	}
	err := func(podObj interface{}) error {
		defer c.podqueue.Done(podObj)
		var key string
		var ok bool
		if key, ok = podObj.(string); !ok {
			c.podqueue.Forget(podObj)
			runtime.HandleError(fmt.Errorf("PodController.go: expected string in podqueue but got %#v", podObj))
			return nil
		}
		if err := c.handlePod(key); err != nil {
			return fmt.Errorf("PodController.go: error in handlePod key: %s, err: %s", key, err.Error())
		}
		c.podqueue.Forget(podObj)
		return nil
	}(podObj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}
	return true
}

// enqueuePod takes a Pod resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Foo.
func (c *PodController) enqueuePod(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	fmt.Printf("PodController.go     : enqueuePod, adding a pod key %s\n", key)
	c.podqueue.AddRateLimited(key)
}

func (c *PodController) handlePod(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("PodController.go: invalid resource key, can't split : %s", key))
		return fmt.Errorf("PodController.go: invalid resource key, can't split : %s", key)
	}
	pod, err := c.kubeclientset.CoreV1().Pods(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		// fmt.Println("PodController.go     : Couldn't find the replicaset, skipping")
		return fmt.Errorf("Could not find the pod for pod name : %s, namespace %s", name, namespace)
	}
	ownerReferences := pod.GetOwnerReferences()
	if len(ownerReferences) == 0 {
		// fmt.Println("PodController.go     : Pod does not have any owner references.")
		return nil
	}
	ownerReferenceName := ownerReferences[0].Name

	replicaSet, err := c.kubeclientset.AppsV1().ReplicaSets(namespace).Get(ownerReferenceName, metav1.GetOptions{})
	if err != nil {
		// fmt.Println("PodController.go     : Couldn't find the replicaset, skipping")
		return nil
	}
	ownerReferences = replicaSet.GetOwnerReferences()
	if len(ownerReferences) == 0 {
		// fmt.Println("PodController.go     : replicaset does not have a ny owner references.")
		return nil
	}
	ownerReferenceName = ownerReferences[0].Name

	deployment, err := c.kubeclientset.AppsV1().Deployments(namespace).Get(ownerReferenceName, metav1.GetOptions{})
	if err != nil {
		// fmt.Println("PodController.go     : Couldn't find the deployment, skipping")
		return nil
	}
	ownerReferences = deployment.GetOwnerReferences()
	if len(ownerReferences) == 0 {
		// fmt.Println("PodController.go     : deployment does not have any owner references. Cannot be a moodle.")
		return nil
	}

	if ownerReferences[0].Kind != "Moodle" {
		// fmt.Println("PodController.go     : The pod event is not a Moodle instance. Skipping!")
		return nil
	}

	moodle, err := c.moodleLister.Moodles(namespace).Get(ownerReferences[0].Name)
	if err != nil {
		return fmt.Errorf("PodController.go: Could not find the Moodle instance : %s", ownerReferences[0].Name)
	}
	if _, success := c.util.WaitForPod(constants.TIMEOUT, name, namespace); !success {
		return fmt.Errorf("PodController.go: The Moodle Pod failed to start running")
	}
	supported, _ := c.util.GetSupportedPlugins(moodle.Spec.Plugins)
	erredPlugins := c.util.EnsurePluginsInstalled(moodle, supported, name, namespace, constants.PLUGIN_MAP)
	if len(erredPlugins) > 0 {
		return fmt.Errorf("PodController.go: Some plugins failed to install. Cannot ensure state. %v", erredPlugins)
	}
	return nil
}
