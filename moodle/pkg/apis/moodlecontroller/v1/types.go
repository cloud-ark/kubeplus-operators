package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Moodle is a specification for a Moodle resource
// +k8s:openapi-gen=true
type Moodle struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MoodleSpec   `json:"spec"`
	Status MoodleStatus `json:"status"`
}

// MoodleSpec is the spec for a MoodleSpec resource
// +k8s:openapi-gen=true
type MoodleSpec struct {
	//Comma separated list of plugin names from: https://moodle.org/plugins/
	Plugins []string `json:"plugins"`
	//MySQL Service name
	MySQLServiceName string `json:"mySQLServiceName"`
	//MySQL Username
	MySQLUserName string `json:"mySQLUserName"`
	//MySQL Password
	MySQLUserPassword string `json:"mySQLUserPassword"`
	//Moodle Admin Email
	MoodleAdminEmail string `json:"moodleAdminEmail"`
	//PVC Volume Name
	PVCVolumeName string `json:"PVCVolumeName"`
	//Domain Name
	DomainName string `json:"domainName"`
	//TLS Flag
	Tls string `json:"tls"`
}

// MoodleStatus is the status for a Moodle resource
// +k8s:openapi-gen=true
type MoodleStatus struct {
	PodName            string   `json:"podName"`
	SecretName         string   `json:"secretName"`
	Status             string   `json:"status"`
	Url                string   `json:"url"`
	InstalledPlugins   []string `json:"installedPlugins"`
	UnsupportedPlugins []string `json:"unsupportedPlugins"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// MoodleList is a list of Moodle resources
type MoodleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Moodle `json:"items"`
}
