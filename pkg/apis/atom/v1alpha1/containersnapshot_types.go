package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ContainerSnapshotSpec defines the desired state of ContainerSnapshot
type ContainerSnapshotSpec struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// PodName+ContainerName is the name of the running container going to have a snapshot
	PodName       string `json:"podName"`
	ContainerName string `json:"containerName"`

	// Image is the snapshot image, registry host and tag are optional
	Image string `json:"image"`

	// ImagePushSecret is a reference to a docker-registry secret in the same namespace to use for pushing checkout image,
	// same as an ImagePullSecret.
	// More info: https://kubernetes.io/docs/concepts/containers/images#specifying-imagepullsecrets-on-a-pod
	ImagePushSecret v1.LocalObjectReference `json:"imagePushSecret"`
}

// ContainerSnapshotStatus defines the observed state of ContainerSnapshot
type ContainerSnapshotStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// JobRef is a reference to the internal snapshot job which does the real commit/push works
	JobRef v1.LocalObjectReference `json:"jobRef"`

	// NodeName is the name of the node the container running on, the snapshot job must run on this node
	NodeName string `json:"nodeName"`

	// container snapshot worker state
	// +kubebuilder:validation:Enum=Created;Running;Complete;Failed
	WorkerState WorkerState `json:"state"`

	// The latest available observations of the snapshot
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []SnapshotCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

type WorkerState string

const (
	WorkerCreated  WorkerState = "Created"
	WorkerRunning              = "Running"
	WorkerComplete             = "Complete"
	WorkerFailed               = "Failed"
)

type SnapshotCondition struct {
	// Type of job condition, Complete or Failed.
	// +kubebuilder:validation:Enum=SourceContainerNotFound;SourcePodNotReady;DockerCommitFailed;DockerPushFailed
	Type SnapshotConditionType `json:"type" protobuf:"bytes,1,opt,name=type,casttype=SnapshotConditionType"`
	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status" protobuf:"bytes,2,opt,name=status,casttype=k8s.io/api/core/v1.ConditionStatus"`
	// Last time the condition was checked.
	// +optional
	LastProbeTime metav1.Time `json:"lastProbeTime,omitempty" protobuf:"bytes,3,opt,name=lastProbeTime"`
	// Last time the condition transit from one status to another.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty" protobuf:"bytes,4,opt,name=lastTransitionTime"`
	// (brief) reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty" protobuf:"bytes,5,opt,name=reason"`
	// Human readable message indicating details about last transition.
	// +optional
	Message string `json:"message,omitempty" protobuf:"bytes,6,opt,name=message"`
}

type SnapshotConditionType string

const (
	SourceContainerNotFound SnapshotConditionType = "SourceContainerNotFound"
	SourcePodNotReady                             = "SourcePodNotReady"
	DockerCommitFailed                            = "DockerCommitFailed"
	DockerPushFailed                              = "DockerPushFailed"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ContainerSnapshot is the Schema for the containersnapshots API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=containersnapshots,scope=Namespaced
// +kubebuilder:printcolumn:name="Name",type="string",JSONPath=".metadata.name"
// +kubebuilder:printcolumn:name="Pod",type="string",JSONPath=".spec.podName",description="pod name of snapshot source"
// +kubebuilder:printcolumn:name="Container",type="string",JSONPath=".spec.containerName",description="container name of snapshot source"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.state",description="container snapshot worker state"
type ContainerSnapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ContainerSnapshotSpec   `json:"spec,omitempty"`
	Status ContainerSnapshotStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ContainerSnapshotList contains a list of ContainerSnapshot
type ContainerSnapshotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ContainerSnapshot `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ContainerSnapshot{}, &ContainerSnapshotList{})
}
