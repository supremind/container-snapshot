package v1alpha1

import (
	"github.com/operator-framework/operator-sdk/pkg/status"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ContainerSnapshotSpec defines the desired state of ContainerSnapshot
type ContainerSnapshotSpec struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// PodName+ContainerName is the name of the running container going to have a snapshot
	PodName       string `json:"podName"`
	ContainerName string `json:"containerName"`

	// Image is the snapshot image, registry host and tag are optional
	Image string `json:"image"`

	// ImagePushSecrets are references to docker-registry secret in the same namespace to use for pushing checkout image,
	// same as an ImagePullSecrets.
	// More info: https://kubernetes.io/docs/concepts/containers/images#specifying-imagepullsecrets-on-a-pod
	ImagePushSecrets []v1.LocalObjectReference `json:"imagePushSecrets"`
}

// ContainerSnapshotStatus defines the observed state of ContainerSnapshot
type ContainerSnapshotStatus struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// JobRef is a reference to the internal snapshot job which does the real commit/push works
	JobRef v1.LocalObjectReference `json:"jobRef"`

	// NodeName is the name of the node the container running on, the snapshot job must run on this node
	NodeName string `json:"nodeName"`

	// ContainerID is the docker id of the source container
	ContainerID string `json:"containerID"`

	// container snapshot worker state
	// +kubebuilder:validation:Enum=Created;Running;Complete;Failed;Unknown
	WorkerState WorkerState `json:"workerState"`

	// The latest available observations of the snapshot
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions status.Conditions `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// WorkerState indicates underlaying snapshot worker state
type WorkerState string

const (
	WorkerCreated  WorkerState = "Created"
	WorkerRunning  WorkerState = "Running"
	WorkerComplete WorkerState = "Complete"
	WorkerFailed   WorkerState = "Failed"
	WorkerUnknown  WorkerState = "Unknown"
)

// Conditions indicate errors occurred when creating or running the snapshot worker pod
const (
	SourcePodNotFound       status.ConditionType = "SourcePodNotFound"
	SourceContainerNotFound status.ConditionType = "SourceContainerNotFound"
	SourcePodNotReady       status.ConditionType = "SourcePodNotReady"
	DockerCommitFailed      status.ConditionType = "DockerCommitFailed"
	DockerPushFailed        status.ConditionType = "DockerPushFailed"
	InvalidImage            status.ConditionType = "InvalidImage"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ContainerSnapshot is the Schema for the containersnapshots API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=containersnapshots,scope=Namespaced
// +kubebuilder:printcolumn:name="Name",type="string",JSONPath=".metadata.name"
// +kubebuilder:printcolumn:name="Pod",type="string",JSONPath=".spec.podName",description="pod name of snapshot source"
// +kubebuilder:printcolumn:name="Container",type="string",JSONPath=".spec.containerName",description="container name of snapshot source"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.workerState",description="container snapshot worker state"
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
