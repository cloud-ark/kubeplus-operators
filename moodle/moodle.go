package main

import (
	"bytes"
	"fmt"
	operatorv1 "github.com/cloud-ark/kubeplus-operators/moodle/pkg/apis/moodlecontroller/v1"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apiutil "k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/remotecommand"
	"strconv"
	"strings"
	"time"
	"math/rand"
	"errors"
)

var (
	API_VERSION    = "moodlecontroller.kubeplus/v1"
	MOODLE_KIND    = "Moodle"
	CONTAINER_NAME = "moodle"
	PLUGIN_MAP     = map[string]map[string]string{
		"profilecohort": {
			"downloadLink":  "https://moodle.org/plugins/download.php/17929/local_profilecohort_moodle35_2018092800.zip",
			"installFolder": "/var/www/html/local/",
		},
		"wiris": {
			"downloadLink":  "https://moodle.org/plugins/download.php/18916/filter_wiris_moodle36_2019020700.zip",
			"installFolder": "/var/www/html/filter/",
		},
	}
)

func (c *Controller) deployMoodle(foo *operatorv1.Moodle) (string, string, string, []string, []string, error) {
	fmt.Println("Inside deployMoodle")
	var moodlePodName, serviceURIToReturn string
	var supportedPlugins, unsupportedPlugins, erredPlugins []string

	c.createPersistentVolume(foo)
	c.createPersistentVolumeClaim(foo)

	servicePort := c.createService(foo)

	c.createIngress(foo)

	err, moodlePodName, secretName := c.createDeployment(foo)

	if err != nil {
		return serviceURIToReturn, moodlePodName, secretName, unsupportedPlugins, erredPlugins, err
	}

	// Wait couple of seconds more just to give the Pod some more time.
	time.Sleep(time.Second * 2)

	plugins := foo.Spec.Plugins

	supportedPlugins, unsupportedPlugins = c.getSupportedPlugins(plugins)

	if len(supportedPlugins) > 0 {
		namespace := getNamespace(foo)
		erredPlugins = c.installPlugins(supportedPlugins, moodlePodName, namespace)
	}

	serviceURIToReturn = foo.Name + ":" + servicePort
	fmt.Println("Returning from deployMoodle")

	return serviceURIToReturn, moodlePodName, secretName, unsupportedPlugins, erredPlugins, nil
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

func (c *Controller) installPlugins(plugins []string, moodlePodName, namespace string) []string {
	fmt.Println("Inside installPlugins")
	erredPlugins := make([]string, 0)
	for _, pluginName := range plugins {
		fmt.Printf("Installing plugin %s\n", pluginName)
		pluginInstallDetails := PLUGIN_MAP[pluginName]

		downloadLink := pluginInstallDetails["downloadLink"]
		installFolder := pluginInstallDetails["installFolder"]
		fmt.Printf("Download Link:%s\n", downloadLink)
		fmt.Printf("Install Folder:%s\n", installFolder)
		success := c.exec(pluginName, moodlePodName, namespace, downloadLink, installFolder)
		if !success {
			erredPlugins = append(erredPlugins, pluginName)
		}
	}
	fmt.Printf("Erred Plugins:%v\n", erredPlugins)
	fmt.Println("Done installing Plugins")
	return erredPlugins
}

func (c *Controller) exec(pluginName, moodlePodName, namespace, downloadLink, installFolder string) bool {

	fmt.Println("Inside exec")

	_, err := c.kubeclientset.CoreV1().Pods(namespace).Get(moodlePodName, metav1.GetOptions{})

	if err != nil {
		fmt.Errorf("could not get pod info: %v", err)
		panic(err)
	}

	indexOfLastSlash := strings.LastIndex(downloadLink, "/")
	pluginZipFileName := downloadLink[indexOfLastSlash+1:]
	fmt.Printf("Plugin ZipFile Name:%s\n", pluginZipFileName)

	downloadPluginCmd := "wget " + downloadLink + " -O /tmp/" + pluginZipFileName
	fmt.Printf("Download Plugin Cmd:%s\n", downloadPluginCmd)
	c.executeExecCall(moodlePodName, namespace, downloadPluginCmd)

	unzipPluginCmd := "unzip /tmp/" + pluginZipFileName + " -d " + "/tmp/."
	fmt.Printf("Unzip Plugin Cmd:%s\n", unzipPluginCmd)
	c.executeExecCall(moodlePodName, namespace, unzipPluginCmd)

	mvPluginCmd := "mv /tmp/" + pluginName + " " + installFolder + "/."
	fmt.Printf("Move Plugin Cmd:%s\n", mvPluginCmd)
	success := c.executeExecCall(moodlePodName, namespace, mvPluginCmd)

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
func (c *Controller) executeExecCall(moodlePodName, namespace, command string) bool {
	var success = true
	fmt.Println("Inside executeExecCall")
	req := c.kubeclientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(moodlePodName).
		Namespace(namespace).
		SubResource("exec")

	scheme := runtime.NewScheme()
	if err := apiv1.AddToScheme(scheme); err != nil {
		success = false
	}

	parameterCodec := runtime.NewParameterCodec(scheme)
	req.VersionedParams(&apiv1.PodExecOptions{
		Command:   strings.Fields(command),
		Container: CONTAINER_NAME,
		//Stdin:     stdin != nil,
		Stdout: true,
		Stderr: true,
		TTY:    false,
	}, parameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(c.cfg, "POST", req.URL())
	if err != nil {
		success = false
	}

	var (
		execOut bytes.Buffer
		execErr bytes.Buffer
	)

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: &execOut,
		Stderr: &execErr,
		Tty:    false,
	})

	if err != nil {
		success = false
	}

	responseString := execOut.String()
	fmt.Printf("Output:%v\n", responseString)
	return success
}

func (c *Controller) generatePassword(moodlePort int) string {
     seed := moodlePort
     rand.Seed(int64(seed))
     mina := 97
     maxa := 122
     minA := 65
     maxA := 90
     min0 := 48
     max0 := 57
     length := 8

     password := make([]string, length)     
 
     i := 0
     for i < length {
         charSet := rand.Intn(3)
         if charSet == 0 {
            passwordInt := rand.Intn(maxa - mina) + mina
            password[i] = string(passwordInt)
         }
         if charSet == 1 {
            passwordInt := rand.Intn(maxA - minA) + minA
            password[i] = string(passwordInt)
         }
         if charSet == 2 {
            passwordInt := rand.Intn(max0 - min0) + min0
            password[i] = string(passwordInt)
         }       
         i++
     }
     passwordString := strings.Join(password,"")
     fmt.Printf("Generated Password:%s\n", passwordString)

     return passwordString
}

func getNamespace(foo *operatorv1.Moodle) string {
	namespace := apiv1.NamespaceDefault
	if foo.Namespace != "" {
		namespace = foo.Namespace
	}
	return namespace
}

func (c *Controller) createIngress(foo *operatorv1.Moodle) {
     
     moodleName := foo.Name

     moodlePath := "/" + moodleName
     moodleServiceName := moodleName
     moodlePort := MOODLE_PORT

     ingress := &extensionsv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: moodleName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: API_VERSION,
					Kind:       MOODLE_KIND,
					Name:       foo.Name,
					UID:        foo.UID,
				},
			},
		},
		Spec: extensionsv1beta1.IngressSpec{
		      Rules: []extensionsv1beta1.IngressRule{
		      	     {
				IngressRuleValue: extensionsv1beta1.IngressRuleValue{
				    HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
				    	  Paths: []extensionsv1beta1.HTTPIngressPath{
					    {
						Path: moodlePath,
						Backend: extensionsv1beta1.IngressBackend{
						   ServiceName: moodleServiceName,
						   ServicePort: apiutil.FromInt(moodlePort),
						},
					    },
					  },
				    },
				},
			     },
		      },
		},
     }

     namespace := getNamespace(foo)
     ingressesClient := c.kubeclientset.ExtensionsV1beta1().Ingresses(namespace)

     fmt.Println("Creating Ingress...")
     result, err := ingressesClient.Create(ingress)
     if err != nil {
	panic(err)
     }
     fmt.Printf("Created Ingress %q.\n", result.GetObjectMeta().GetName())
}

func (c *Controller) createPersistentVolume(foo *operatorv1.Moodle) {
	fmt.Println("Inside createPersistentVolume")

	deploymentName := foo.Name
	persistentVolume := &apiv1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploymentName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: API_VERSION,
					Kind:       MOODLE_KIND,
					Name:       foo.Name,
					UID:        foo.UID,
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

	deploymentName := foo.Name
	persistentVolumeClaim := &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploymentName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: API_VERSION,
					Kind:       MOODLE_KIND,
					Name:       foo.Name,
					UID:        foo.UID,
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
				Requests: apiv1.ResourceList{
					"storage": resource.MustParse("1Gi"),
					//							map[string]resource.Quantity{
					//							"storage": resource.MustParse("1Gi"),
					//						},
				},
			},
		},
	}

	namespace := getNamespace(foo)
	persistentVolumeClaimClient := c.kubeclientset.CoreV1().PersistentVolumeClaims(namespace)

	fmt.Println("Creating persistentVolumeClaim...")
	result, err := persistentVolumeClaimClient.Create(persistentVolumeClaim)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created persistentVolumeClaim %q.\n", result.GetObjectMeta().GetName())
}

func (c *Controller) createDeployment(foo *operatorv1.Moodle) (error, string, string) {

	fmt.Println("Inside createDeployment")

	namespace := getNamespace(foo)
	deploymentsClient := c.kubeclientset.AppsV1().Deployments(namespace)

	deploymentName := foo.Name
	moodlePort := MOODLE_PORT

	image := "lmecld/nginxformoodle8:latest"
	//image := "lmecld/nginxformoodle6:latest"
	volumeName := "moodle-data"

	adminPassword := c.generatePassword(MOODLE_PORT)

	secretName := c.createSecret(foo, adminPassword)

	//MySQL Service IP and Port
	mysqlServiceName := foo.Spec.MySQLServiceName
	fmt.Printf("MySQL Service name:%v\n", mysqlServiceName)

	mysqlServiceClient := c.kubeclientset.CoreV1().Services(namespace)
	mysqlServiceResult, err := mysqlServiceClient.Get(mysqlServiceName, metav1.GetOptions{})

	if err != nil {
		fmt.Printf("Error getting MySQL Service details: %v\n", err)
		return err, "", secretName
	}

	mysqlHostIP := mysqlServiceResult.Spec.ClusterIP
	mysqlServicePortInt := mysqlServiceResult.Spec.Ports[0].Port
	fmt.Println("MySQL Service Port int:%d\n", mysqlServicePortInt)
	mysqlServicePort := fmt.Sprint(mysqlServicePortInt)
	fmt.Println("MySQL Service Port:%d\n", mysqlServicePort)
	fmt.Println("MySQL Host IP:%s\n", mysqlHostIP)

	CONTAINER_PORT := MOODLE_PORT

	HOST_NAME := deploymentName + ":" + strconv.Itoa(MOODLE_PORT)
	fmt.Println("HOST_NAME:%s\n", HOST_NAME)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploymentName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: API_VERSION,
					Kind:       MOODLE_KIND,
					Name:       foo.Name,
					UID:        foo.UID,
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
										Command: []string{"/bin/sh", "-c", "/usr/local/scripts/moodleinstall.sh; /usr/sbin/nginx -s reload"},
									},
								},
							},
							Ports: []apiv1.ContainerPort{
								{
									ContainerPort: int32(CONTAINER_PORT),
								},
							},
							/*
								ReadinessProbe: &apiv1.Probe{
									Handler: apiv1.Handler{
										TCPSocket: &apiv1.TCPSocketAction{
											Port: apiutil.FromInt(80),
										},
									},
									InitialDelaySeconds: 5,
									TimeoutSeconds:      60,
									PeriodSeconds:       2,
								},*/
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
									Name:  "MOODLE_PORT",
									Value: strconv.Itoa(moodlePort),
								},
								{
									Name: "HOST_NAME",
									Value: HOST_NAME,
									/*ValueFrom: &apiv1.EnvVarSource{
									  FieldRef: &apiv1.ObjectFieldSelector{
									      FieldPath: "status.hostIP",
									  },
									},*/
								},
							},
							VolumeMounts: []apiv1.VolumeMount{
								{
									Name:      volumeName,
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
		return err, "", ""
	}
	fmt.Printf("Created deployment %q.\n", result.GetObjectMeta().GetName())

	moodlePodName, podReady := c.waitForPod(foo)

	if podReady {
		return nil, moodlePodName, secretName
	} else {
		err1 := errors.New("Moodle Pod Timeout")
		return err1, moodlePodName, secretName
	}
}

func (c *Controller) createSecret(foo *operatorv1.Moodle, adminPassword string) string {

        fmt.Println("Inside createSecret")
        secretName := foo.Name

	fmt.Printf("Secret Name:%s\n", secretName)
	fmt.Printf("Admin Password:%s\n", adminPassword)
	
        secret := &apiv1.Secret{
                ObjectMeta: metav1.ObjectMeta{
                        Name: secretName,
                        OwnerReferences: []metav1.OwnerReference{
                                {
                                        APIVersion: API_VERSION,
                                        Kind:       MOODLE_KIND,
                                        Name:       foo.Name,
                                        UID:        foo.UID,
                                },
                        },
                        Labels: map[string]string{
                                "secret": secretName,
                        },
                },
                Data: map[string][]byte {
		      "adminPassword": []byte(adminPassword),
		},
        }

        namespace := getNamespace(foo)
        secretsClient := c.kubeclientset.CoreV1().Secrets(namespace)

        fmt.Println("Creating secrets..")
	result, err := secretsClient.Create(secret)
        if err != nil {
	   panic(err)
	}
        fmt.Printf("Created Secret %q.\n", result.GetObjectMeta().GetName())
	return secretName
}

func (c *Controller) createService(foo *operatorv1.Moodle) (string) {

	fmt.Println("Inside createService")
	deploymentName := foo.Name
	moodlePort := MOODLE_PORT

	namespace := getNamespace(foo)
	serviceClient := c.kubeclientset.CoreV1().Services(namespace)
	service := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploymentName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: API_VERSION,
					Kind:       MOODLE_KIND,
					Name:       foo.Name,
					UID:        foo.UID,
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
					Port:       int32(moodlePort),
					TargetPort: apiutil.FromInt(moodlePort),
					NodePort:   int32(MOODLE_PORT),
					Protocol:   apiv1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"app": deploymentName,
			},
			Type: apiv1.ServiceTypeNodePort,
			//Type: apiv1.ServiceTypeClusterIP,
		},
	}

	result1, err1 := serviceClient.Create(service)
	if err1 != nil {
		panic(err1)
	}
	fmt.Printf("Created service %q.\n", result1.GetObjectMeta().GetName())

	//nodePort1 := result1.Spec.Ports[0].NodePort
	//nodePort := fmt.Sprint(nodePort1)
	servicePort := fmt.Sprint(moodlePort)

	// Parse ServiceIP and Port
	serviceIP := result1.Spec.ClusterIP
	fmt.Println("Moodle Service IP:%s", serviceIP)

	//servicePortInt := result1.Spec.Ports[0].Port
	//servicePort := fmt.Sprint(servicePortInt)

	serviceURI := serviceIP + ":" + servicePort

	fmt.Printf("Service URI%s\n", serviceURI)

	return servicePort
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
		namespace := getNamespace(foo)
		c.installPlugins(supportedPlugins, podName, namespace)
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

func (c *Controller) getDiff(leftHandSide, rightHandSide []string) []string {
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

func (c *Controller) isInitialDeployment(foo *operatorv1.Moodle) bool {
	if foo.Status.Url == "" {
		return true
	} else {
		return false
	}
}

func (c *Controller) waitForPod(foo *operatorv1.Moodle) (string, bool) {
	var podName string
	deploymentName := foo.Name
	namespace := getNamespace(foo)
	// Check if Postgres Pod is ready or not
	podReady := false
	podTimeoutCount := 0
	TIMEOUT_COUNT := 150 // 10 minutes; this should be made configurable
	for {
		pods := c.getPods(namespace, deploymentName)
		for _, d := range pods.Items {
			parts := strings.Split(d.Name, "-")
			parts = parts[:len(parts)-2]
			podDepName := strings.Join(parts, "")
			//fmt.Printf("Pod Deployment name:%s\n", podDepName)
			if podDepName == deploymentName {
				podName = d.Name
				fmt.Printf("Moodle Pod Name:%s\n", podName)
				podConditions := d.Status.Conditions
				for _, podCond := range podConditions {
					if podCond.Type == apiv1.PodReady {
						if podCond.Status == apiv1.ConditionTrue {
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
			podTimeoutCount = podTimeoutCount + 1
			if podTimeoutCount >= TIMEOUT_COUNT {
				podReady = false
				break
			}
		}
	}
	if podReady {
		fmt.Println("Pod is ready.")
	} else {
		fmt.Println("Pod timeout")
	}
	return podName, podReady
}

func (c *Controller) getPods(namespace, deploymentName string) *apiv1.PodList {
	// TODO(devkulkarni): This is returning all Pods. We should change this
	// to only return Pods whose Label matches the Deployment Name.
	pods, err := c.kubeclientset.CoreV1().Pods(namespace).List(metav1.ListOptions{
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
