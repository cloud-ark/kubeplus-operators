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
	Name string `json:"name"`
	AdminPassword string `json:"adminPassword"`
	Plugins []string `json:"plugins"`
}

// MoodleStatus is the status for a Moodle resource
// +k8s:openapi-gen=true
type MoodleStatus struct {
	Status string `json:"status"`
	Url string `json:"url"`
	InstalledPlugins []string `json:"installedPlugins"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// MoodleList is a list of Moodle resources
type MoodleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Moodle `json:"items"`
}
