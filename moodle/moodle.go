package main

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	operatorv1 "github.com/cloud-ark/kubeplus-operators/moodle/pkg/apis/moodlecontroller/v1"
	"github.com/cloud-ark/kubeplus-operators/moodle/pkg/utils/constants"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiutil "k8s.io/apimachinery/pkg/util/intstr"
)

func (c *MoodleController) deployMoodle(foo *operatorv1.Moodle) (string, string, string, []string, []string, error) {
	fmt.Println("MoodleController.go  : Inside deployMoodle")
	var moodlePodName, serviceURIToReturn string
	var supportedPlugins, unsupportedPlugins, erredPlugins []string

	c.createPersistentVolume(foo)

	if foo.Spec.PVCVolumeName == "" {
		c.createPersistentVolumeClaim(foo)
	}
	servicePort := c.createService(foo)

	c.createIngress(foo)

	err, moodlePodName, secretName := c.createDeployment(foo)

	if err != nil {
		return serviceURIToReturn, moodlePodName, secretName, unsupportedPlugins, erredPlugins, err
	}

	// Wait couple of seconds more just to give the Pod some more time.
	time.Sleep(time.Second * 2)

	plugins := foo.Spec.Plugins

	supportedPlugins, unsupportedPlugins = c.util.GetSupportedPlugins(plugins)
	if len(supportedPlugins) > 0 {
		namespace := getNamespace(foo)
		erredPlugins = c.util.EnsurePluginsInstalled(foo, supportedPlugins, moodlePodName, namespace, constants.PLUGIN_MAP)
	}
	if len(erredPlugins) > 0 {
		err = errors.New("Error Installing Supported Plugin")
	}

	if foo.Spec.DomainName == "" {
		serviceURIToReturn = foo.Name + ":" + servicePort
	} else {
		serviceURIToReturn = foo.Spec.DomainName + ":" + servicePort
	}

	fmt.Println("MoodleController.go  : MoodleController.go: Returning from deployMoodle")

	return serviceURIToReturn, moodlePodName, secretName, unsupportedPlugins, erredPlugins, err
}

func (c *MoodleController) generatePassword(moodlePort int) string {
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
			passwordInt := rand.Intn(maxa-mina) + mina
			password[i] = string(passwordInt)
		}
		if charSet == 1 {
			passwordInt := rand.Intn(maxA-minA) + minA
			password[i] = string(passwordInt)
		}
		if charSet == 2 {
			passwordInt := rand.Intn(max0-min0) + min0
			password[i] = string(passwordInt)
		}
		i++
	}
	passwordString := strings.Join(password, "")
	fmt.Printf("MoodleController.go  : MoodleController.go  : Generated Password:%s\n", passwordString)

	return passwordString
}

func getNamespace(foo *operatorv1.Moodle) string {
	namespace := apiv1.NamespaceDefault
	if foo.Namespace != "" {
		namespace = foo.Namespace
	}
	return namespace
}

func (c *MoodleController) createIngress(foo *operatorv1.Moodle) {

	moodleName := foo.Name

	moodleTLSCertSecretName := ""
	tls := foo.Spec.Tls

	fmt.Printf("MoodleController.go: TLS: %s\n", tls)
	if len(tls) > 0  {
		moodleTLSCertSecretName = moodleName + "-domain-cert"
	}

	moodlePath := "/"

	moodleDomainName := getDomainName(foo)
	if moodleDomainName == "" {
		moodlePath = moodlePath + moodleName
	}

	moodleServiceName := moodleName
	moodlePort := MOODLE_PORT

	specObj := getIngressSpec(moodlePort, moodleDomainName, moodlePath, 
		moodleTLSCertSecretName, moodleServiceName, tls)

	ingress := getIngress(foo, specObj, moodleName, tls)

	namespace := getNamespace(foo)
	ingressesClient := c.kubeclientset.ExtensionsV1beta1().Ingresses(namespace)

	fmt.Println("MoodleController.go  : Creating Ingress...")
	result, err := ingressesClient.Create(ingress)
	if err != nil {
		panic(err)
	}
	fmt.Printf("MoodleController.go  : Created Ingress %q.\n", result.GetObjectMeta().GetName())
}

func getIngress(foo *operatorv1.Moodle, specObj extensionsv1beta1.IngressSpec, moodleName, tls string) *extensionsv1beta1.Ingress {

	var ingress *extensionsv1beta1.Ingress 

	if len(tls) > 0 {
			ingress = &extensionsv1beta1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name: moodleName,
				Annotations: map[string]string{
					"kubernetes.io/ingress.class": "nginx",
					"nginx.ingress.kubernetes.io/rewrite-target": "/",
					"certmanager.k8s.io/issuer": moodleName,
					"certmanager.k8s.io/acme-challenge-type": "http01",
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: constants.API_VERSION,
						Kind:       constants.MOODLE_KIND,
						Name:       foo.Name,
						UID:        foo.UID,
					},
				},
			},
			Spec: specObj,
		}
	} else {
			ingress = &extensionsv1beta1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name: moodleName,
				Annotations: map[string]string{
					"kubernetes.io/ingress.class": "nginx",
					"nginx.ingress.kubernetes.io/ssl-redirect": "false",
					"nginx.ingress.kubernetes.io/rewrite-target": "/",
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: constants.API_VERSION,
						Kind:       constants.MOODLE_KIND,
						Name:       foo.Name,
						UID:        foo.UID,
					},
				},
			},
			Spec: specObj,
			/*
			Spec: extensionsv1beta1.IngressSpec{
				TLS: []extensionsv1beta1.IngressTLS{
					{
						Hosts: []string{moodleDomainName},
						SecretName: moodleTLSCertSecretName,
					},
				},
				Rules: []extensionsv1beta1.IngressRule{
					{
						Host: moodleDomainName,
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
		*/
		}
	}
	return ingress
}

func getIngressSpec(moodlePort int, moodleDomainName, moodlePath, moodleTLSCertSecretName, 
	moodleServiceName, tls string) extensionsv1beta1.IngressSpec {

	var specObj extensionsv1beta1.IngressSpec

	if len(tls) > 0 {
		specObj = extensionsv1beta1.IngressSpec{
			TLS: []extensionsv1beta1.IngressTLS{
				{
					Hosts: []string{moodleDomainName},
					SecretName: moodleTLSCertSecretName,
				},
			},
			Rules: []extensionsv1beta1.IngressRule{
				{
					Host: moodleDomainName,
					IngressRuleValue: extensionsv1beta1.IngressRuleValue{
						HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
							Paths: []extensionsv1beta1.HTTPIngressPath{
								{
									Path: moodlePath,
									Backend: extensionsv1beta1.IngressBackend{
										ServiceName: moodleServiceName,
										ServicePort: apiutil.FromInt(80),//apiutil.FromInt(moodlePort),
									},
								},
							},
						},
					},
				},
			},
		}
	} else {
		specObj = extensionsv1beta1.IngressSpec{
			Rules: []extensionsv1beta1.IngressRule{
				{
					Host: moodleDomainName,
					IngressRuleValue: extensionsv1beta1.IngressRuleValue{
						HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
							Paths: []extensionsv1beta1.HTTPIngressPath{
								{
									Path: moodlePath,
									Backend: extensionsv1beta1.IngressBackend{
										ServiceName: moodleServiceName,
										ServicePort: apiutil.FromInt(80),
									},
								},
							},
						},
					},
				},
			},
		}
	}

	return specObj
}

func (c *MoodleController) createPersistentVolume(foo *operatorv1.Moodle) {
	fmt.Println("MoodleController.go  : Inside createPersistentVolume")

	deploymentName := foo.Name
	persistentVolume := &apiv1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploymentName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: constants.API_VERSION,
					Kind:       constants.MOODLE_KIND,
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

	fmt.Println("MoodleController.go  : Creating persistentVolume...")
	result, err := persistentVolumeClient.Create(persistentVolume)
	if err != nil {
		panic(err)
	}
	fmt.Printf("MoodleController.go  : Created persistentVolume %q.\n", result.GetObjectMeta().GetName())
}

func (c *MoodleController) createPersistentVolumeClaim(foo *operatorv1.Moodle) {
	fmt.Println("MoodleController.go  : Inside createPersistentVolumeClaim")

	storageClassName := "standard"
	deploymentName := foo.Name
	persistentVolumeClaim := &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploymentName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: constants.API_VERSION,
					Kind:       constants.MOODLE_KIND,
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
			StorageClassName: &storageClassName,
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

	fmt.Println("MoodleController.go  : Creating persistentVolumeClaim...")
	result, err := persistentVolumeClaimClient.Create(persistentVolumeClaim)
	if err != nil {
		panic(err)
	}
	fmt.Printf("MoodleController.go  : Created persistentVolumeClaim %q.\n", result.GetObjectMeta().GetName())
}

func (c *MoodleController) createDeployment(foo *operatorv1.Moodle) (error, string, string) {

	fmt.Println("MoodleController.go  : Inside createDeployment")

	namespace := getNamespace(foo)
	deploymentsClient := c.kubeclientset.AppsV1().Deployments(namespace)

	deploymentName := foo.Name

	moodlePort := MOODLE_PORT

	image := "lmecld/nginxformoodle8:latest"
	//image := "lmecld/nginxformoodle:9.0"
	//image := "lmecld/nginxformoodle6:latest"
	//	image = "lmecld/nginxformoodle10:latest"

	volumeName := "moodle-data"

	claimName := foo.Spec.PVCVolumeName
	if claimName == "" {
		claimName = foo.Name
	}

	secretName := ""
	adminPassword := ""
	secretName, adminPassword = c.getSecret(foo)
	if adminPassword == "" {
		adminPassword = c.generatePassword(MOODLE_PORT)
		secretName = c.createSecret(foo, adminPassword)
	}

	//MySQL Service IP and Port
	mysqlServiceName := foo.Spec.MySQLServiceName
	fmt.Printf("MoodleController.go  : MySQL Service name:%v\n", mysqlServiceName)

	mysqlUserName := foo.Spec.MySQLUserName
	fmt.Printf("MoodleController.go  : MySQL Username:%v\n", mysqlUserName)

	passwordLocation := foo.Spec.MySQLUserPassword
	secretPasswordSplitted := strings.Split(passwordLocation, ".")
	mysqlSecretName := secretPasswordSplitted[0]
	mysqlPasswordField := secretPasswordSplitted[1]

	secretsClient := c.kubeclientset.CoreV1().Secrets(namespace)
	secret, err := secretsClient.Get(mysqlSecretName, metav1.GetOptions{})

	if err != nil {
		fmt.Printf("MoodleController.go  : Error, secret %s, not found from in namespace %s: %v\n", mysqlSecretName, namespace, err)
	}
	if _, ok := secret.Data[mysqlPasswordField]; !ok {
		fmt.Printf("MoodleController.go  : Error, secret  %s, does not have %s field.\n", mysqlSecretName, mysqlPasswordField)
	}
	mysqlUserPassword := string(secret.Data[mysqlPasswordField])

	fmt.Printf("MoodleController.go  : MySQL Password:%s\n", mysqlUserPassword)

	moodleAdminEmail := foo.Spec.MoodleAdminEmail
	fmt.Printf("MoodleController.go  : Moodle Admin Email:%v\n", moodleAdminEmail)

	mysqlServiceClient := c.kubeclientset.CoreV1().Services(namespace)
	mysqlServiceResult, err := mysqlServiceClient.Get(mysqlServiceName, metav1.GetOptions{})

	if err != nil {
		fmt.Printf("MoodleController.go  : Error getting MySQL Service details: %v\n", err)
		return err, "", secretName
	}

	mysqlHostIP := mysqlServiceName
	mysqlServicePortInt := mysqlServiceResult.Spec.Ports[0].Port
	fmt.Printf("MoodleController.go  : MySQL Service Port int:%d\n", mysqlServicePortInt)
	mysqlServicePort := fmt.Sprint(mysqlServicePortInt)
	fmt.Printf("MoodleController.go  : MySQL Service Port:%s\n", mysqlServicePort)
	fmt.Printf("MoodleController.go  : MySQL Host IP:%s\n", mysqlHostIP)

	CONTAINER_PORT := MOODLE_PORT

	HOST_NAME := ""
	if foo.Spec.DomainName == "" {
		HOST_NAME = deploymentName + ":" + strconv.Itoa(MOODLE_PORT)
	} else {
		HOST_NAME = foo.Spec.DomainName
	}

	fmt.Printf("MoodleController.go  : HOST_NAME:%s\n", HOST_NAME)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploymentName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: constants.API_VERSION,
					Kind:       constants.MOODLE_KIND,
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
							Name:  constants.CONTAINER_NAME,
							Image: image,
							Lifecycle: &apiv1.Lifecycle{
								PostStart: &apiv1.Handler{
									Exec: &apiv1.ExecAction{
										Command: []string{"/bin/sh", "-c", "/usr/local/scripts/moodleinstall.sh; sleep 5; if [ ! -f /var/run/nginx.pid ]; then nohup /usr/sbin/nginx >& /dev/null; else /usr/sbin/nginx -s reload; fi"},
										// Command: []string{"/bin/sh", "-c", "/usr/local/scripts/moodleinstall.sh; sleep 5; /usr/sbin/nginx -s reload"},
										//Command: []string{"/bin/sh", "-c", "/usr/local/scripts/moodleinstall.sh"},
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
									Value: mysqlUserName,
								},
								{
									Name:  "MYSQL_PASSWORD",
									Value: mysqlUserPassword,
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
									Value: moodleAdminEmail,
								},
								{
									Name:  "MOODLE_PORT",
									Value: strconv.Itoa(moodlePort),
								},
								{
									Name:  "HOST_NAME",
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
									ClaimName: claimName,
								},
							},
						},
					},
				},
			},
		},
	}

	// Create Deployment
	fmt.Println("MoodleController.go  : Creating deployment...")
	result, err := deploymentsClient.Create(deployment)
	if err != nil {
		panic(err)
		return err, "", ""
	}
	fmt.Printf("MoodleController.go  : Created deployment %q.\n", result.GetObjectMeta().GetName())

	/*
	podname, _ := c.util.GetPodFullName(constants.TIMEOUT, foo.Name, foo.Namespace)
	moodlePodName, podReady := c.util.WaitForPod(constants.TIMEOUT, podname, foo.Namespace)
	*/

	moodlePodName, podReady := c.waitForPod(foo)

	if podReady {
		return nil, moodlePodName, secretName
	} else {
		err1 := errors.New("Moodle Pod Timeout")
		return err1, moodlePodName, secretName
	}
}

func (c *MoodleController) getSecret(foo *operatorv1.Moodle) (string, string) {
	fmt.Println("MoodleController.go  : Inside getSecret")
	secretName := foo.Name

	namespace := getNamespace(foo)
	secretsClient := c.kubeclientset.CoreV1().Secrets(namespace)

	fmt.Println("MoodleController.go  : Getting secrets..")
	result, err := secretsClient.Get(secretName, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("MoodleController.go : %v", err)
		//panic(err)
	}
	if result != nil {
		fmt.Printf("MoodleController.go  : Getting Secret %q.\n", result.GetObjectMeta().GetName())

		adminPasswordByteArray := result.Data["adminPassword"]
		adminPassword := string(adminPasswordByteArray)

		fmt.Printf("MoodleController.go  : Admin Password %q.\n", adminPassword)

		return secretName, adminPassword

	} else {
		return "", ""
	}
}

func (c *MoodleController) createSecret(foo *operatorv1.Moodle, adminPassword string) string {

	fmt.Println("MoodleController.go  : Inside createSecret")
	secretName := foo.Name

	fmt.Printf("MoodleController.go  : Secret Name:%s\n", secretName)
	fmt.Printf("MoodleController.go  : Admin Password:%s\n", adminPassword)

	secret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: constants.API_VERSION,
					Kind:       constants.MOODLE_KIND,
					Name:       foo.Name,
					UID:        foo.UID,
				},
			},
			Labels: map[string]string{
				"secret": secretName,
			},
		},
		Data: map[string][]byte{
			"adminPassword": []byte(adminPassword),
		},
	}

	namespace := getNamespace(foo)
	secretsClient := c.kubeclientset.CoreV1().Secrets(namespace)

	fmt.Println("MoodleController.go  : Creating secrets..")
	result, err := secretsClient.Create(secret)
	if err != nil {
		panic(err)
	}
	fmt.Printf("MoodleController.go  : Created Secret %q.\n", result.GetObjectMeta().GetName())
	return secretName
}

func (c *MoodleController) createService(foo *operatorv1.Moodle) string {

	fmt.Println("MoodleController.go  : Inside createService")
	deploymentName := foo.Name
	moodlePort := MOODLE_PORT

	namespace := getNamespace(foo)
	serviceClient := c.kubeclientset.CoreV1().Services(namespace)

	serviceObj, servicePort := getServiceSpec(moodlePort, deploymentName, foo.Spec.DomainName)
	service := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploymentName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: constants.API_VERSION,
					Kind:       constants.MOODLE_KIND,
					Name:       foo.Name,
					UID:        foo.UID,
				},
			},
			Labels: map[string]string{
				"app": deploymentName,
			},
		},
		Spec: serviceObj, 

		/*
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{
					Name:       "my-port",
					Port:       int32(moodlePort),
					TargetPort: apiutil.FromInt(moodlePort),
					//NodePort:   int32(MOODLE_PORT),
					Protocol:   apiv1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"app": deploymentName,
			},
			//Type: apiv1.ServiceTypeNodePort,
			Type: apiv1.ServiceTypeClusterIP,
			//Type: apiv1.ServiceTypeLoadBalancer,
		},*/
	}

	result1, err1 := serviceClient.Create(service)
	if err1 != nil {
		panic(err1)
	}
	fmt.Printf("MoodleController.go  : Created service %q.\n", result1.GetObjectMeta().GetName())

	//nodePort1 := result1.Spec.Ports[0].NodePort
	//nodePort := fmt.Sprint(nodePort1)
	//servicePort := fmt.Sprint(moodlePort)

	// Parse ServiceIP and Port
	serviceIP := result1.Spec.ClusterIP
	fmt.Printf("MoodleController.go  : Moodle Service IP:%s", serviceIP)

	//servicePortInt := result1.Spec.Ports[0].Port
	//servicePort := fmt.Sprint(servicePortInt)

	serviceURI := serviceIP + ":" + servicePort

	fmt.Printf("MoodleController.go  : Service URI%s\n", serviceURI)

	return servicePort
}

func getDomainName(foo *operatorv1.Moodle) string {
	return foo.Spec.DomainName

/*	if len(foo.Spec.DomainName) > 0 {
		return foo.Spec.DomainName
	} else {
		return foo.Name
	}
*/
}

func getServiceSpec(moodlePort int, deploymentName, domainName string) (apiv1.ServiceSpec, string) {

	var serviceObj apiv1.ServiceSpec

	var servicePort string

	if domainName == "" {
		serviceObj = apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{
					Name:       "my-port",
					Port:       int32(moodlePort),
					TargetPort: apiutil.FromInt(moodlePort),
					NodePort:   int32(moodlePort),
					Protocol:   apiv1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"app": deploymentName,
			},
			Type: apiv1.ServiceTypeNodePort,
			//Type: apiv1.ServiceTypeClusterIP,
			//Type: apiv1.ServiceTypeLoadBalancer,
		}
		servicePort = strconv.Itoa(moodlePort)
	} else {
		serviceObj = apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{
					Name:       "my-port",
					//Port:       int32(moodlePort),
					Port: 80,
					TargetPort: apiutil.FromInt(moodlePort),
					//NodePort:   int32(MOODLE_PORT),
					Protocol:   apiv1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"app": deploymentName,
			},
			//Type: apiv1.ServiceTypeNodePort,
			Type: apiv1.ServiceTypeClusterIP,
			//Type: apiv1.ServiceTypeLoadBalancer,
		}
		servicePort = strconv.Itoa(80)
	}
	return serviceObj, servicePort
}

func (c *MoodleController) handlePluginDeployment(moodle *operatorv1.Moodle) (string, []string, []string, []string) {

	installedPlugins := moodle.Status.InstalledPlugins
	specPlugins := moodle.Spec.Plugins
	unsupportedPlugins := moodle.Status.UnsupportedPlugins

	fmt.Printf("MoodleController.go  : Spec Plugins:%v\n", specPlugins)
	fmt.Printf("MoodleController.go  : Installed Plugins:%v\n", installedPlugins)
	var addList []string
	var removeList []string

	// addList = specList - installedList - unsupportedPlugins
	addList = c.getDiff(specPlugins, installedPlugins)
	fmt.Printf("MoodleController.go  : Plugins to install:%v\n", addList)

	if unsupportedPlugins != nil {
		addList = c.getDiff(addList, unsupportedPlugins)
	}

	// removeList = installedList - specList
	removeList = c.getDiff(installedPlugins, specPlugins)
	fmt.Printf("MoodleController.go  : Plugins to remove:%v\n", removeList)

	var podName string
	var supportedPlugins, unsupportedPlugins1 []string
	supportedPlugins, unsupportedPlugins1 = c.util.GetSupportedPlugins(addList)

	var erredPlugins []string
	if len(supportedPlugins) > 0 {
		podName = moodle.Status.PodName
		namespace := getNamespace(moodle)
		//podname, _ := c.util.GetPodFullName(constants.TIMEOUT, podName, namespace)
		erredPlugins = c.util.EnsurePluginsInstalled(moodle, supportedPlugins, podName, namespace, constants.PLUGIN_MAP)
	}
	if len(removeList) > 0 {
		fmt.Println("MoodleController.go  : ============= Plugin removal not implemented yet ===============")
	}

	/*
	   if len(supportedPlugins) > 0 || len(removeList) > 0 {
	   	return podName, supportedPlugins, unsupportedPlugins
	   } else {
	      return podName, supportedPlugins, unsupportedPlugins
	   }*/

	return podName, supportedPlugins, unsupportedPlugins1, erredPlugins
}

func (c *MoodleController) getDiff(leftHandSide, rightHandSide []string) []string {
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

func (c *MoodleController) isInitialDeployment(foo *operatorv1.Moodle) bool {
	if foo.Status.Url == "" {
		return true
	} else {
		return false
	}
}
func (c *MoodleController) waitForPod(foo *operatorv1.Moodle) (string, bool) {
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

func (c *MoodleController) getPods(namespace, deploymentName string) *apiv1.PodList {
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
