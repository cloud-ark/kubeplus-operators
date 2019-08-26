package main

import (
	"fmt"
	"time"

	_ "github.com/lib/pq"
	restclient "k8s.io/client-go/rest"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	operatorv1 "github.com/cloud-ark/kubeplus-operators/moodle/pkg/apis/moodlecontroller/v1"
	clientset "github.com/cloud-ark/kubeplus-operators/moodle/pkg/client/clientset/versioned"
	operatorscheme "github.com/cloud-ark/kubeplus-operators/moodle/pkg/client/clientset/versioned/scheme"
	informers "github.com/cloud-ark/kubeplus-operators/moodle/pkg/client/informers/externalversions"
	listers "github.com/cloud-ark/kubeplus-operators/moodle/pkg/client/listers/moodlecontroller/v1"
	"github.com/cloud-ark/kubeplus-operators/moodle/pkg/utils"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	appslisters "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

const controllerAgentName = "moodle-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a Foo is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Foo fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Foo"
	// MessageResourceSynced is the message used for an Event fired when a Foo
	// is synced successfully
	MessageResourceSynced = "Foo synced successfully"
)

var (
	MOODLE_PORT_BASE = 32000
	MOODLE_PORT      int
)

func init() {
}

// Controller is the controller implementation for Foo resources
type MoodleController struct {
	cfg *restclient.Config
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// sampleclientset is a clientset for our own API group
	sampleclientset clientset.Interface

	deploymentsLister appslisters.DeploymentLister
	deploymentsSynced cache.InformerSynced
	moodleLister      listers.MoodleLister
	foosSynced        cache.InformerSynced
	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
	util     utils.Utils
}

// NewController returns a new sample controller
func NewMoodleController(
	cfg *restclient.Config,
	kubeclientset kubernetes.Interface,
	sampleclientset clientset.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	moodleInformerFactory informers.SharedInformerFactory) *MoodleController {

	// obtain references to shared index informers for the Deployment and Foo
	// types.
	deploymentInformer := kubeInformerFactory.Apps().V1().Deployments()
	moodleInformer := moodleInformerFactory.Moodlecontroller().V1().Moodles()

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

	controller := &MoodleController{
		cfg:               cfg,
		kubeclientset:     kubeclientset,
		sampleclientset:   sampleclientset,
		deploymentsLister: deploymentInformer.Lister(),
		deploymentsSynced: deploymentInformer.Informer().HasSynced,
		moodleLister:      moodleInformer.Lister(),
		foosSynced:        moodleInformer.Informer().HasSynced,
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Moodles"),
		recorder:          recorder,
		util:              utils,
	}

	glog.Info("Setting up event handlers")
	// Set up an event handler for when Foo resources change
	moodleInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueFoo,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*operatorv1.Moodle)
			oldDepl := old.(*operatorv1.Moodle)
			//fmt.Println("MoodleController.go  : New Version:%s", newDepl.ResourceVersion)
			//fmt.Println("MoodleController.go  : Old Version:%s", oldDepl.ResourceVersion)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			} else {
				controller.enqueueFoo(new)
			}
		},
	})
	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *MoodleController) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	glog.Info("Starting Moodle controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced, c.foosSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	glog.Info("Starting workers")
	// Launch two workers to process Pod resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	glog.Info("Started workers")
	<-stopCh
	glog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *MoodleController) runWorker() {
	for c.processNextWorkItem() {
	}
}

// enqueueFoo takes a Foo resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Foo.
func (c *MoodleController) enqueueFoo(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Foo resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Foo resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *MoodleController) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		glog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	glog.V(4).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a Foo, we should not do anything more
		// with it.
		if ownerRef.Kind != "Foo" {
			return
		}

		foo, err := c.moodleLister.Moodles(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			glog.V(4).Infof("ignoring orphaned object '%s' of foo '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueFoo(foo)
		return
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *MoodleController) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}
	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Foo resource to be synced.
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		//fmt.Println("MoodleController.go  : processNextItem before forgetting")
		c.workqueue.Forget(obj)
		//fmt.Println("MoodleController.go  : processNextItem after forgetting")
		glog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)
	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Foo resource
// with the current status of the resource.
func (c *MoodleController) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Foo resource with this namespace/name
	foo, err := c.moodleLister.Moodles(namespace).Get(name)
	if err != nil {
		// The Foo resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("foo '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	fmt.Println("MoodleController.go  : **************************************")

	moodleName := foo.Name
	moodleNamespace := foo.Namespace
	plugins := foo.Spec.Plugins

	fmt.Printf("MoodleController.go  : Moodle Name:%s\n", moodleName)
	fmt.Printf("MoodleController.go  : Moodle Namespace:%s\n", moodleNamespace)
	fmt.Printf("MoodleController.go  : Plugins:%v\n", plugins)

	var status, url string
	var supportedPlugins, unsupportedPlugins []string
	initialDeployment := c.isInitialDeployment(foo)

	if foo.Status.Status != "" && foo.Status.Status == "Moodle Pod Timeout" {
		return nil
	}

	if initialDeployment {

		moodleDomainName := foo.Spec.DomainName

		fmt.Printf("MoodleController.go : Moodle DomainName:%s\n", moodleDomainName)

		MOODLE_PORT = MOODLE_PORT_BASE
		MOODLE_PORT_BASE = MOODLE_PORT_BASE + 1

		fmt.Printf("MoodleController.go : Moodle Port:%s\n", MOODLE_PORT)

		initialDeployment = false

		serviceURL, podName, secretName, unsupportedPlugins, erredPlugins, err := c.deployMoodle(foo)

		var correctlyInstalledPlugins []string
		if err != nil {
			status = err.Error()
		} else {
			status = "Ready"
			url = "http://" + serviceURL
			fmt.Printf("MoodleController.go  : Moodle URL:%s\n", url)
			supportedPlugins, unsupportedPlugins = c.util.GetSupportedPlugins(plugins)
			correctlyInstalledPlugins = c.getDiff(supportedPlugins, erredPlugins)
		}
		c.updateMoodleStatus(foo, podName, secretName, status, url, &correctlyInstalledPlugins, &unsupportedPlugins)
		c.recorder.Event(foo, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	} else {
		podName, installedPlugins, unsupportedPluginsCurrent, erredPlugins := c.handlePluginDeployment(foo)
		if len(installedPlugins) > 0 || len(unsupportedPluginsCurrent) > 0 {
			status = "Ready"
			url = foo.Status.Url
			unsupportedPlugins = foo.Status.UnsupportedPlugins
			unsupportedPlugins = appendList(unsupportedPluginsCurrent, unsupportedPlugins)

			supportedPlugins = foo.Status.InstalledPlugins
			supportedPlugins = append(supportedPlugins, installedPlugins...)
			if len(erredPlugins) > 0 {
				c.updateMoodleStatus(foo, podName, "", "Error in installing some plugins", url, &supportedPlugins, &unsupportedPlugins)
			} else {
				c.updateMoodleStatus(foo, podName, "", status, url, &supportedPlugins, &unsupportedPlugins)
			}
			c.recorder.Event(foo, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
		} else {
			fmt.Printf("MoodleController.go  : Moodle custom resource %s did not change. No plugin installed.\n", moodleName)
		}
	}
	// Returning nil so that the controller does not try to sync the same Moodle instance.
	return nil
}

func appendList(source, destination []string) []string {
	var appendedList []string

	for _, delem := range destination {
		present := false
		for _, selem := range source {
			if delem == selem {
				present = true
				break
			}
		}
		if !present {
			appendedList = append(appendedList, delem)
		}
	}
	return appendedList
}

func (c *MoodleController) updateMoodleStatus(foo *operatorv1.Moodle, podName, secretName, status string,
	url string, plugins *[]string, unsupportedPlugins *[]string) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	fooCopy := foo.DeepCopy()

	fooCopy.Status.PodName = podName
	if secretName != "" {
		fooCopy.Status.SecretName = secretName
	}
	fooCopy.Status.Status = status
	fooCopy.Status.Url = url
	fooCopy.Status.InstalledPlugins = *plugins
	fooCopy.Status.UnsupportedPlugins = *unsupportedPlugins
	// Until #38113 is merged, we must use Update instead of UpdateStatus to
	// update the Status block of the Foo resource. UpdateStatus will not
	// allow changes to the Spec of the resource, which is ideal for ensuring
	// nothing other than resource status has been updated.
	_, err := c.sampleclientset.MoodlecontrollerV1().Moodles(foo.Namespace).Update(fooCopy)
	if err != nil {
		fmt.Printf("MoodleController.go  : ERROR in UpdateFooStatus %e", err)
	}
	return err
}
