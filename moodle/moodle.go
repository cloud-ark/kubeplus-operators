	package main

import (
        "fmt"
	"strings"
	"bytes"
	"time"
	"os"
	operatorv1 "github.com/cloud-ark/kubeplus-operators/moodle/pkg/apis/moodlecontroller/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiutil "k8s.io/apimachinery/pkg/util/intstr"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/tools/remotecommand"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	API_VERSION = "moodlecontroller.kubeplus/v1"
	MOODLE_KIND = "Moodle"
	CONTAINER_NAME = "moodle"
	PLUGIN_MAP = map[string]map[string]string{
		"profilecohort":{
			"downloadLink": "https://moodle.org/plugins/download.php/17929/local_profilecohort_moodle35_2018092800.zip",
	 		"installFolder": "/var/www/html/local/",
		 },
		"wiris":{
			"downloadLink": "https://moodle.org/plugins/download.php/18185/filter_wiris_moodle35_2018110900.zip",
	 		"installFolder": "/var/www/html/filter/",
		 },
	}
)

func (c *Controller) deployMoodle(foo *operatorv1.Moodle) (string, string, []string, []string, error) {
	fmt.Println("Inside deployMoodle")
	var serviceIP, servicePort, moodlePodName, serviceIPToReturn string
	var supportedPlugins, unsupportedPlugins, erredPlugins []string

	c.createPersistentVolume(foo)
	c.createPersistentVolumeClaim(foo)
	err := c.createDeployment(foo)

	if err != nil {
	   return serviceIPToReturn, moodlePodName, unsupportedPlugins, erredPlugins, err
	}

	serviceIP, servicePort, moodlePodName = c.createService(foo)

	// Wait couple of seconds more just to give the Pod some more time.
	time.Sleep(time.Second * 2)

	plugins := foo.Spec.Plugins

	supportedPlugins, unsupportedPlugins = c.getSupportedPlugins(plugins)

	if (len(supportedPlugins) > 0 ) {
	   erredPlugins = c.installPlugins(supportedPlugins, moodlePodName)
	}

	serviceIPToReturn = serviceIP + ":" + servicePort
	fmt.Println("Returning from deployMoodle")

	return serviceIPToReturn, moodlePodName, unsupportedPlugins, erredPlugins, nil
}

func (c *Controller) getSupportedPlugins(plugins []string) ([]string, []string) {

     var supportedPlugins, unsupportedPlugins []string

     for _, p := range plugins {
     	 if _, ok := PLUGIN_MAP[p]; ok {
	    supportedPlugins = append(supportedPlugins, p)    
	 } else {
	    unsupportedPlugins = append(unsupportedPlugins, p)
	 }
     }
     fmt.Println("Supported Plugins:%v", supportedPlugins)
     fmt.Println("Unsupported Plugins:%v", unsupportedPlugins)
     return supportedPlugins, unsupportedPlugins
}


func (c *Controller) installPlugins(plugins []string, moodlePodName string) []string {
	fmt.Println("Inside installPlugins")
	erredPlugins := make([]string, 0)
	for _, pluginName := range plugins {
	    fmt.Printf("Installing plugin %s\n", pluginName)
	    pluginInstallDetails := PLUGIN_MAP[pluginName]
	    
	    downloadLink := pluginInstallDetails["downloadLink"]
	    installFolder := pluginInstallDetails["installFolder"]
	    fmt.Printf("Download Link:%s\n", downloadLink)
	    fmt.Printf("Install Folder:%s\n", installFolder)
	    success := c.exec(pluginName, moodlePodName, downloadLink, installFolder)
	    if !success {
	       erredPlugins = append(erredPlugins, pluginName)
	    }
	}
	fmt.Printf("Erred Plugins:%v\n", erredPlugins)
	fmt.Println("Done installing Plugins")
	return erredPlugins
}

func (c *Controller) exec(pluginName, moodlePodName, downloadLink, installFolder string) bool {

	fmt.Println("Inside exec")

	_, err := c.kubeclientset.CoreV1().Pods("default").Get(moodlePodName, metav1.GetOptions{})

	if err != nil {
		fmt.Errorf("could not get pod info: %v", err)
		panic(err)
	}

	indexOfLastSlash := strings.LastIndex(downloadLink, "/")
	pluginZipFileName := downloadLink[indexOfLastSlash+1:]
	fmt.Printf("Plugin ZipFile Name:%s\n", pluginZipFileName)

	downloadPluginCmd := "wget " + downloadLink + " -O /tmp/" + pluginZipFileName
	fmt.Printf("Download Plugin Cmd:%s\n", downloadPluginCmd)
	c.executeExecCall(moodlePodName, downloadPluginCmd)

	unzipPluginCmd := "unzip /tmp/" + pluginZipFileName + " -d " + "/tmp/."
	fmt.Printf("Unzip Plugin Cmd:%s\n", unzipPluginCmd)
	c.executeExecCall(moodlePodName, unzipPluginCmd)

	mvPluginCmd := "mv /tmp/" + pluginName + " " + installFolder + "/."
	fmt.Printf("Move Plugin Cmd:%s\n", mvPluginCmd)
	success := c.executeExecCall(moodlePodName, mvPluginCmd)

	if success {
		fmt.Printf("Done installing plugin %s\n", pluginName)
	} else {
	     fmt.Printf("Encountered error in installing plugin\n")
	}
	return success
}

/*
  Reference for kubectl exec:
  - https://github.com/a4abhishek/Client-Go-Examples/blob/master/exec_to_pod/exec_to_pod.go
  - https://stackoverflow.com/questions/43314689/example-of-exec-in-k8ss-pod-by-using-go-client/43315545#43315545
  - https://github.com/kubernetes/client-go/issues/204
*/
func (c *Controller) executeExecCall(moodlePodName, command string) bool {
     	var success = true
	fmt.Println("Inside executeExecCall")
	req := c.kubeclientset.CoreV1().RESTClient().Post().
			Resource("pods").
			Name(moodlePodName).
			Namespace("default").
			SubResource("exec")
			
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		//panic(err)
		success = false
	}

	parameterCodec := runtime.NewParameterCodec(scheme)
	req.VersionedParams(&corev1.PodExecOptions{
		Command:   strings.Fields(command),
		Container: CONTAINER_NAME,
		//Stdin:     stdin != nil,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, parameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(c.cfg, "POST", req.URL())
	if err != nil {
		//return "", fmt.Errorf("failed to init executor: %v", err)
		//panic(err)
		success = false
	}

	
	var (
		execOut bytes.Buffer
		execErr bytes.Buffer
	)

     //fmt.Println("8")
	err = exec.Stream(remotecommand.StreamOptions{
	        Stdin: nil,
		Stdout: &execOut,
		Stderr: &execErr,
		Tty:    false,
	})

     //fmt.Println("9")
	if err != nil {
		//return "", fmt.Errorf("could not execute: %v", err)
		//panic(err)
		success = false
	}

     //fmt.Println("10")

     responseString := execOut.String()
     //fmt.Println("11")

     fmt.Printf("Output:%v\n", responseString)

     return success
}


func (c *Controller) createPersistentVolume(foo *operatorv1.Moodle) {
	fmt.Println("Inside createPersistentVolume")

	deploymentName := foo.Spec.Name
	persistentVolume := &apiv1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: deploymentName,
				OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: API_VERSION,
					Kind: MOODLE_KIND,
					Name: foo.Name,
					UID: foo.UID,
				},
			},
		},
		Spec: apiv1.PersistentVolumeSpec{
				StorageClassName: "manual",
				Capacity: apiv1.ResourceList{
//					map[string]resource.Quantity{
						"storage": resource.MustParse("1Gi"),
//					},
				},
				AccessModes: []apiv1.PersistentVolumeAccessMode{
//					{
						"ReadWriteOnce",
//					},
				},
				PersistentVolumeSource: apiv1.PersistentVolumeSource{
					HostPath: &apiv1.HostPathVolumeSource{
						Path: "/mnt/moodle-data",
					},
				},
			},
	}

	persistentVolumeClient := c.kubeclientset.CoreV1().PersistentVolumes()

	fmt.Println("Creating persistentVolume...")
	result, err := persistentVolumeClient.Create(persistentVolume)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created persistentVolume %q.\n", result.GetObjectMeta().GetName())

}

func (c *Controller) createPersistentVolumeClaim(foo *operatorv1.Moodle) {
	fmt.Println("Inside createPersistentVolumeClaim")

	deploymentName := foo.Spec.Name
	persistentVolumeClaim := &apiv1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: deploymentName,
				OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: API_VERSION,
					Kind: MOODLE_KIND,
					Name: foo.Name,
					UID: foo.UID,
				},
			},
		},
		Spec: apiv1.PersistentVolumeClaimSpec{
				AccessModes: []apiv1.PersistentVolumeAccessMode{
//					{
						"ReadWriteOnce",
//					},
				},
				Resources: apiv1.ResourceRequirements{
									Requests: apiv1.ResourceList {
							"storage": resource.MustParse("1Gi"),
//							map[string]resource.Quantity{
//							"storage": resource.MustParse("1Gi"),
//						},
					},
				},
		},
	}

	persistentVolumeClaimClient := c.kubeclientset.CoreV1().PersistentVolumeClaims(apiv1.NamespaceDefault)

	fmt.Println("Creating persistentVolumeClaim...")
	result, err := persistentVolumeClaimClient.Create(persistentVolumeClaim)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created persistentVolumeClaim %q.\n", result.GetObjectMeta().GetName())
}


func (c *Controller) createDeployment(foo *operatorv1.Moodle) error {

	fmt.Println("Inside createDeployment")

	deploymentsClient := c.kubeclientset.AppsV1().Deployments(apiv1.NamespaceDefault)

	deploymentName := foo.Spec.Name
	image := "lmecld/nginxformoodle6:latest"
	volumeName := "moodle-data"
	adminPassword := foo.Spec.AdminPassword

	//MySQL Service IP and Port
	mysqlServiceName := deploymentName + "-mysql"
	mysqlServiceClient := c.kubeclientset.CoreV1().Services(apiv1.NamespaceDefault)
	mysqlServiceResult, err := mysqlServiceClient.Get(mysqlServiceName, metav1.GetOptions{})

	if err != nil {
	   fmt.Printf("Error getting MySQL Service details: %v\n", err)
	   return err
	}

	mysqlHostIP := mysqlServiceResult.Spec.ClusterIP
	mysqlServicePortInt := mysqlServiceResult.Spec.Ports[0].Port
	fmt.Println("MySQL Service Port int:%d\n", mysqlServicePortInt)
	mysqlServicePort := fmt.Sprint(mysqlServicePortInt)
	fmt.Println("MySQL Service Port:%d\n", mysqlServicePort)
	fmt.Println("MySQL Host IP:%s\n", mysqlHostIP)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploymentName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: API_VERSION,
					Kind: MOODLE_KIND,
					Name: foo.Name,
					UID: foo.UID,
				},
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": deploymentName,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": deploymentName,
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:  CONTAINER_NAME,
							Image: image,
							Lifecycle: &apiv1.Lifecycle{
								PostStart: &apiv1.Handler{
								    Exec: &apiv1.ExecAction{
								       Command: []string{"/bin/sh", "-c", "/usr/local/scripts/moodleinstall.sh"},
								    },
								},
							}, 
							Ports: []apiv1.ContainerPort{
								{
									ContainerPort: 80,
								},
							},
							ReadinessProbe: &apiv1.Probe{
								Handler: apiv1.Handler{
									TCPSocket: &apiv1.TCPSocketAction{
										Port: apiutil.FromInt(80),
									},
								},
								InitialDelaySeconds: 5,
								TimeoutSeconds:      60,
								PeriodSeconds:       2,
							},
							Env: []apiv1.EnvVar{
								{
									Name:  "APPLICATION_NAME",
									Value: deploymentName,
								},
								{
									Name:  "MYSQL_DATABASE",
									Value: "moodle",
								},
								{
									Name:  "MYSQL_USER",
									Value: "user1",
								},
								{
									Name:  "MYSQL_PASSWORD",
									Value: "password1",
								},
								{
									Name:  "MYSQL_HOST",
									Value: mysqlHostIP,
									/*ValueFrom: &apiv1.EnvVarSource{
									  FieldRef: &apiv1.ObjectFieldSelector{
									      FieldPath: "status.hostIP",
									  },									
									},*/
								},
								{
									Name:  "MYSQL_PORT",
									Value: mysqlServicePort,
								},
								{
									Name:  "MYSQL_TABLE_PREFIX",
									Value: "mdl_",
								},
								{
									Name:  "MOODLE_ADMIN_PASSWORD",
									Value: adminPassword,
								},
								{
									Name:  "MOODLE_ADMIN_EMAIL",
									Value: "abc@abc.com",
								},
								{
									Name:  "HOST_NAME",
									Value: HOST_IP,
									/*ValueFrom: &apiv1.EnvVarSource{
									  FieldRef: &apiv1.ObjectFieldSelector{
									      FieldPath: "status.hostIP",
									  },									
									},*/
								},
							},
							VolumeMounts: []apiv1.VolumeMount{
								{
									Name: volumeName,
									MountPath: "/opt/moodledata",

								},
							},	
						},
					},
					Volumes: []apiv1.Volume{
						{
							Name: volumeName,
							VolumeSource: apiv1.VolumeSource{
								PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
										ClaimName: deploymentName,
								},
							},
						},
					},
				},
			},
		},
	}

	// Create Deployment
	fmt.Println("Creating deployment...")
	result, err := deploymentsClient.Create(deployment)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created deployment %q.\n", result.GetObjectMeta().GetName())
	return nil
}


func (c *Controller) createService(foo *operatorv1.Moodle) (string, string, string) {

	fmt.Println("Inside createService")
	deploymentName := foo.Spec.Name

	serviceClient := c.kubeclientset.CoreV1().Services(apiv1.NamespaceDefault)
	service := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploymentName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: API_VERSION,
					Kind: MOODLE_KIND,
					Name: foo.Name,
					UID: foo.UID,
				},
			},
			Labels: map[string]string{
				"app": deploymentName,
			},
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{
					Name:       "my-port",
					Port:       80,
					TargetPort: apiutil.FromInt(80),
					NodePort: 80,
					Protocol:   apiv1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"app": deploymentName,
			},
			Type: apiv1.ServiceTypeNodePort,
		},
	}

	result1, err1 := serviceClient.Create(service)
	if err1 != nil {
		panic(err1)
	}
	fmt.Printf("Created service %q.\n", result1.GetObjectMeta().GetName())

	nodePort1 := result1.Spec.Ports[0].NodePort
	nodePort := fmt.Sprint(nodePort1)
	servicePort := nodePort

	moodlePodName := c.waitForPod(foo)

	// Parse ServiceIP and Port
	serviceIP := os.Getenv("HOST_IP")
	fmt.Println("HOST IP:%s", serviceIP)

	serviceIPToReturn := serviceIP + ":" + servicePort

	fmt.Printf("Service IP to Return:%s\n", serviceIPToReturn)

	return serviceIP, servicePort, moodlePodName
}

func (c *Controller) handlePluginDeployment(foo *operatorv1.Moodle) (string, []string, []string) {

     installedPlugins := foo.Status.InstalledPlugins
     specPlugins := foo.Spec.Plugins
     unsupportedPlugins := foo.Status.UnsupportedPlugins

     fmt.Printf("Spec Plugins:%v\n", specPlugins)
     fmt.Printf("Installed Plugins:%v\n", installedPlugins)
     var addList []string
     var removeList []string

     // addList = specList - installedList - unsupportedPlugins
     addList = c.getDiff(specPlugins, installedPlugins)
     fmt.Println("Plugins to install:%v\n", addList)

     if unsupportedPlugins != nil {
         addList = c.getDiff(addList, unsupportedPlugins)
     }

     // removeList = installedList - specList
     removeList = c.getDiff(installedPlugins, specPlugins)
     fmt.Println("Plugins to remove:%v\n", removeList)

     var podName string
     var supportedPlugins, unsupportedPlugins1 []string
     supportedPlugins, unsupportedPlugins1 = c.getSupportedPlugins(addList)
     if len(supportedPlugins) > 0 {
     	podName = foo.Status.PodName
	c.installPlugins(supportedPlugins, podName)
     }
     if len(removeList) > 0 {
     	fmt.Println("============= Plugin removal not implemented yet ===============")
     }

     /*
     if len(supportedPlugins) > 0 || len(removeList) > 0 {
     	return podName, supportedPlugins, unsupportedPlugins
     } else {
        return podName, supportedPlugins, unsupportedPlugins
     }*/

     return podName, supportedPlugins, unsupportedPlugins1
}

func (c *Controller) getDiff(leftHandSide, rightHandSide []string) ([]string) {
     var diffList []string
     for _, inspec := range leftHandSide {
     	 var found = false
     	 for _, installed := range rightHandSide {
	     if inspec == installed {
	     	found = true
		break
	     }
	 }
	 if !found {
	    diffList = append(diffList, inspec)
	 }
     }
     return diffList
}


func (c *Controller) isInitialDeployment(foo *operatorv1.Moodle) (bool) {
     if foo.Status.Url == "" {
     	return true
     } else {
        return false
     }
}

func (c *Controller) waitForPod(foo *operatorv1.Moodle) (string) {
        var podName string
	deploymentName := foo.Spec.Name
	// Check if Postgres Pod is ready or not
	podReady := false
	for {
		pods := c.getPods(deploymentName)
		for _, d := range pods.Items {
		        parts := strings.Split(d.Name, "-")
			parts = parts[:len(parts)-2]
			podDepName := strings.Join(parts, "")
			//fmt.Printf("Pod Deployment name:%s\n", podDepName)
			//if strings.Contains(d.Name, deploymentName) {
			if podDepName == deploymentName {
			   	podName = d.Name
				fmt.Printf("Moodle Pod Name:%s\n", podName)
				podConditions := d.Status.Conditions
				for _, podCond := range podConditions {
					if podCond.Type == corev1.PodReady {
						if podCond.Status == corev1.ConditionTrue {
							fmt.Println("Moodle Pod is running.")
							podReady = true
							break
						}
					}
				}
			}
			if podReady {
				break
			}
		}
		if podReady {
			break
		} else {
			fmt.Println("Waiting for Moodle Pod to get ready.")
			time.Sleep(time.Second * 4)
		}
	}
	fmt.Println("Pod is ready.")
	return podName
}

func (c *Controller) getPods(deploymentName string) *apiv1.PodList {
	// TODO(devkulkarni): This is returning all Pods. We should change this
	// to only return Pods whose Label matches the Deployment Name.
	pods, err := c.kubeclientset.CoreV1().Pods("default").List(metav1.ListOptions{
		//LabelSelector: deploymentName,
		//LabelSelector: metav1.LabelSelector{
		//	MatchLabels: map[string]string{
		//	"app": deploymentName,
		//},
		//},
	})
	//fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))
	if err != nil {
		fmt.Printf("%s", err)
	}
	return pods
}

func int32Ptr(i int32) *int32 { return &i }