// +build !ignore_autogenerated

// Code generated by operator-sdk. DO NOT EDIT.

package v1alpha1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ContainerSnapshot) DeepCopyInto(out *ContainerSnapshot) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ContainerSnapshot.
func (in *ContainerSnapshot) DeepCopy() *ContainerSnapshot {
	if in == nil {
		return nil
	}
	out := new(ContainerSnapshot)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ContainerSnapshot) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ContainerSnapshotList) DeepCopyInto(out *ContainerSnapshotList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ContainerSnapshot, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ContainerSnapshotList.
func (in *ContainerSnapshotList) DeepCopy() *ContainerSnapshotList {
	if in == nil {
		return nil
	}
	out := new(ContainerSnapshotList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ContainerSnapshotList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ContainerSnapshotSpec) DeepCopyInto(out *ContainerSnapshotSpec) {
	*out = *in
	out.ImagePushSecret = in.ImagePushSecret
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ContainerSnapshotSpec.
func (in *ContainerSnapshotSpec) DeepCopy() *ContainerSnapshotSpec {
	if in == nil {
		return nil
	}
	out := new(ContainerSnapshotSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ContainerSnapshotStatus) DeepCopyInto(out *ContainerSnapshotStatus) {
	*out = *in
	out.JobRef = in.JobRef
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]SnapshotCondition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ContainerSnapshotStatus.
func (in *ContainerSnapshotStatus) DeepCopy() *ContainerSnapshotStatus {
	if in == nil {
		return nil
	}
	out := new(ContainerSnapshotStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SnapshotCondition) DeepCopyInto(out *SnapshotCondition) {
	*out = *in
	in.LastProbeTime.DeepCopyInto(&out.LastProbeTime)
	in.LastTransitionTime.DeepCopyInto(&out.LastTransitionTime)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SnapshotCondition.
func (in *SnapshotCondition) DeepCopy() *SnapshotCondition {
	if in == nil {
		return nil
	}
	out := new(SnapshotCondition)
	in.DeepCopyInto(out)
	return out
}