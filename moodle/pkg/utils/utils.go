package utils

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"time"

	operatorv1 "github.com/cloud-ark/kubeplus-operators/moodle/pkg/apis/moodlecontroller/v1"
	"github.com/cloud-ark/kubeplus-operators/moodle/pkg/utils/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

var mutex = &sync.Mutex{}

type Utils struct {
	cfg           *restclient.Config
	kubeclientset kubernetes.Interface
}

func NewUtils(
	_cfg *restclient.Config,
	_kubeclientset kubernetes.Interface) Utils {
	utils := Utils{cfg: _cfg, kubeclientset: _kubeclientset}
	return utils
}
func (u *Utils) Exec(pluginName, moodlePodName, namespace, downloadLink, installFolder string) bool {

	fmt.Println("Inside exec")

	_, err := u.kubeclientset.CoreV1().Pods(namespace).Get(moodlePodName, metav1.GetOptions{})

	if err != nil {
		fmt.Printf("Could not get the pod: %s\n", moodlePodName)
		panic(err)
	}

	indexOfLastSlash := strings.LastIndex(downloadLink, "/")
	pluginZipFileName := downloadLink[indexOfLastSlash+1:]
	fmt.Printf("Plugin ZipFile Name:%s\n", pluginZipFileName)

	downloadPluginCmd := "wget " + downloadLink + " -O /tmp/" + pluginZipFileName
	fmt.Printf("Download Plugin Cmd:%s\n", downloadPluginCmd)
	u.ExecuteExecCall(moodlePodName, namespace, downloadPluginCmd)

	unzipPluginCmd := "unzip /tmp/" + pluginZipFileName + " -d " + "/tmp/."
	fmt.Printf("Unzip Plugin Cmd:%s\n", unzipPluginCmd)
	u.ExecuteExecCall(moodlePodName, namespace, unzipPluginCmd)

	mvPluginCmd := "mv /tmp/" + pluginName + " " + installFolder + "."
	fmt.Printf("Move Plugin Cmd:%s\n", mvPluginCmd)
	success := u.ExecuteExecCall(moodlePodName, namespace, mvPluginCmd)

	if success {
		fmt.Printf("Done installing plugin %s\n", pluginName)
	} else {
		fmt.Printf("Encountered error in installing plugin %s\n", pluginName)
	}
	return success
}

/*
  Reference for kubectl exec:
  - https://github.com/a4abhishek/Client-Go-Examples/blob/master/exec_to_pod/exec_to_pod.go
  - https://stackoverflow.com/questions/43314689/example-of-exec-in-k8ss-pod-by-using-go-client/43315545#43315545
  - https://github.com/kubernetes/client-go/issues/204
*/
func (u *Utils) ExecuteExecCall(moodlePodName, namespace, command string) bool {
	fmt.Println("Inside ExecuteExecCall")
	req := u.kubeclientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(moodlePodName).
		Namespace(namespace).
		SubResource("exec")

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		fmt.Printf("Error found trying to Exec command on pod: %s \n", err.Error())
		return false
	}

	parameterCodec := runtime.NewParameterCodec(scheme)
	req.VersionedParams(&corev1.PodExecOptions{
		Command:   strings.Fields(command),
		Container: constants.CONTAINER_NAME,
		//Stdin:     stdin != nil,
		Stdout: true,
		Stderr: true,
		TTY:    false,
	}, parameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(u.cfg, "POST", req.URL())
	if err != nil {
		fmt.Printf("Error found trying to Exec command on pod: %s \n", err.Error())
		return false
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
		fmt.Printf("StdOutput: %s\n", execOut.String())
		fmt.Printf("StdErr: %s\n", execErr.String())
		fmt.Printf("The command %s returned False : %s \n", command, err.Error())
		return false
	}

	responseString := execOut.String()
	fmt.Printf("Output! :%v\n", responseString)
	return true
}

func (u *Utils) EnsurePluginsInstalled(moodle *operatorv1.Moodle,
	supportedPlugins []string,
	name,
	namespace string,
	pluginData map[string]map[string]string) []string {

	mutex.Lock()
	defer mutex.Unlock()
	fmt.Println("Inside EnsurePluginsInstalled")

	erredPlugins := make([]string, 0)

	for _, pluginName := range supportedPlugins {
		fmt.Printf("  Installing plugin %s\n", pluginName)
		pluginInstallDetails := pluginData[pluginName]

		downloadLink := pluginInstallDetails["downloadLink"]
		installFolder := pluginInstallDetails["installFolder"]
		command := fmt.Sprintf("stat %s%s", installFolder, pluginName)
		fmt.Printf("    Download Link:%s\n", downloadLink)
		fmt.Printf("    Install Folder:%s\n", installFolder)
		output := u.ExecuteExecCall(name, namespace, command)
		if !output { // if stat installFolder/pluginName failed -> install it.
			fmt.Printf("    Plugin does not exist yet. Need to download and install it... :%s\n", installFolder)
			success := u.Exec(pluginName, name, namespace, downloadLink, installFolder)
			if !success {
				fmt.Printf("    Plugin %s failed to install...\n", pluginName)
				erredPlugins = append(erredPlugins, pluginName)
			}
		} else {
			fmt.Printf("  The plugin is already installed. Stat returned True... :%s\n", command)
		}
	}
	fmt.Printf("Erred Plugins:%v\n", erredPlugins)
	fmt.Println("Done installing Plugins")
	return erredPlugins
}

func (u *Utils) WaitForPod(timeout int, podname, namespace string) (string, bool) {
	elapsed := 0
	for {
		pod, _ := u.kubeclientset.CoreV1().Pods(namespace).Get(podname, metav1.GetOptions{})
		if pod.Status.Phase == corev1.PodRunning {
			fmt.Printf("The pod %s is now running.\n", podname)
			return podname, true
		} else if pod.Status.Phase == corev1.PodFailed {
			fmt.Printf("The pod %s has failed.\n", podname)
			return podname, false
		}
		fmt.Println("Waiting for Moodle Pod to get ready.")
		if elapsed >= timeout {
			return podname, false
		}
		time.Sleep(time.Second * 4)
		elapsed += 4
	}
}

func (u *Utils) GetPodFullName(timeout int, podname, namespace string) (string, bool) {
	elapsed := 0
	for {
		pods, err := u.kubeclientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
		if err != nil {
			fmt.Println("Error could not list pods" + err.Error())
		}
		for _, pod := range pods.Items {
			fmt.Println(pod.Name)
			splitted := strings.Split(pod.Name, "-")
			name := splitted[:len(splitted)-2]
			if len(name) > 1 { // if the name has a '-' in it.
				if strings.Join(name, "-") == podname {
					return pod.Name, true
				}
			} else if len(name) == 1 {
				if name[0] == podname {
					return pod.Name, true
				}
			}
		}
		if elapsed >= timeout {
			return "", false
		}
		time.Sleep(time.Second * 4)
		elapsed += 4
	}
}
func (u *Utils) GetSupportedPlugins(plugins []string) ([]string, []string) {

	var supportedPlugins, unsupportedPlugins []string

	for _, p := range plugins {
		if _, ok := constants.PLUGIN_MAP[p]; ok {
			supportedPlugins = append(supportedPlugins, p)
		} else {
			unsupportedPlugins = append(unsupportedPlugins, p)
		}
	}
	return supportedPlugins, unsupportedPlugins
}
