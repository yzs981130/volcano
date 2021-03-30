// +build !ignore_autogenerated

/*
Copyright 2021 The Volcano Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by conversion-gen. DO NOT EDIT.

package v1beta1

import (
	unsafe "unsafe"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	conversion "k8s.io/apimachinery/pkg/conversion"
	runtime "k8s.io/apimachinery/pkg/runtime"
	scheduling "pkg.yezhisheng.me/volcano/pkg/apis/scheduling"
)

func init() {
	localSchemeBuilder.Register(RegisterConversions)
}

// RegisterConversions adds conversion functions to the given scheme.
// Public to allow building arbitrary schemes.
func RegisterConversions(s *runtime.Scheme) error {
	if err := s.AddGeneratedConversionFunc((*PodGroup)(nil), (*scheduling.PodGroup)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1beta1_PodGroup_To_scheduling_PodGroup(a.(*PodGroup), b.(*scheduling.PodGroup), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*scheduling.PodGroup)(nil), (*PodGroup)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_scheduling_PodGroup_To_v1beta1_PodGroup(a.(*scheduling.PodGroup), b.(*PodGroup), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*PodGroupCondition)(nil), (*scheduling.PodGroupCondition)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1beta1_PodGroupCondition_To_scheduling_PodGroupCondition(a.(*PodGroupCondition), b.(*scheduling.PodGroupCondition), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*scheduling.PodGroupCondition)(nil), (*PodGroupCondition)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_scheduling_PodGroupCondition_To_v1beta1_PodGroupCondition(a.(*scheduling.PodGroupCondition), b.(*PodGroupCondition), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*PodGroupList)(nil), (*scheduling.PodGroupList)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1beta1_PodGroupList_To_scheduling_PodGroupList(a.(*PodGroupList), b.(*scheduling.PodGroupList), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*scheduling.PodGroupList)(nil), (*PodGroupList)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_scheduling_PodGroupList_To_v1beta1_PodGroupList(a.(*scheduling.PodGroupList), b.(*PodGroupList), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*PodGroupSpec)(nil), (*scheduling.PodGroupSpec)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1beta1_PodGroupSpec_To_scheduling_PodGroupSpec(a.(*PodGroupSpec), b.(*scheduling.PodGroupSpec), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*scheduling.PodGroupSpec)(nil), (*PodGroupSpec)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_scheduling_PodGroupSpec_To_v1beta1_PodGroupSpec(a.(*scheduling.PodGroupSpec), b.(*PodGroupSpec), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*PodGroupStatus)(nil), (*scheduling.PodGroupStatus)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1beta1_PodGroupStatus_To_scheduling_PodGroupStatus(a.(*PodGroupStatus), b.(*scheduling.PodGroupStatus), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*scheduling.PodGroupStatus)(nil), (*PodGroupStatus)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_scheduling_PodGroupStatus_To_v1beta1_PodGroupStatus(a.(*scheduling.PodGroupStatus), b.(*PodGroupStatus), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*Queue)(nil), (*scheduling.Queue)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1beta1_Queue_To_scheduling_Queue(a.(*Queue), b.(*scheduling.Queue), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*scheduling.Queue)(nil), (*Queue)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_scheduling_Queue_To_v1beta1_Queue(a.(*scheduling.Queue), b.(*Queue), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*QueueList)(nil), (*scheduling.QueueList)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1beta1_QueueList_To_scheduling_QueueList(a.(*QueueList), b.(*scheduling.QueueList), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*scheduling.QueueList)(nil), (*QueueList)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_scheduling_QueueList_To_v1beta1_QueueList(a.(*scheduling.QueueList), b.(*QueueList), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*QueueSpec)(nil), (*scheduling.QueueSpec)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1beta1_QueueSpec_To_scheduling_QueueSpec(a.(*QueueSpec), b.(*scheduling.QueueSpec), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*QueueStatus)(nil), (*scheduling.QueueStatus)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1beta1_QueueStatus_To_scheduling_QueueStatus(a.(*QueueStatus), b.(*scheduling.QueueStatus), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*scheduling.QueueStatus)(nil), (*QueueStatus)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_scheduling_QueueStatus_To_v1beta1_QueueStatus(a.(*scheduling.QueueStatus), b.(*QueueStatus), scope)
	}); err != nil {
		return err
	}
	if err := s.AddConversionFunc((*scheduling.QueueSpec)(nil), (*QueueSpec)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_scheduling_QueueSpec_To_v1beta1_QueueSpec(a.(*scheduling.QueueSpec), b.(*QueueSpec), scope)
	}); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1beta1_PodGroup_To_scheduling_PodGroup(in *PodGroup, out *scheduling.PodGroup, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	if err := Convert_v1beta1_PodGroupSpec_To_scheduling_PodGroupSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_v1beta1_PodGroupStatus_To_scheduling_PodGroupStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

// Convert_v1beta1_PodGroup_To_scheduling_PodGroup is an autogenerated conversion function.
func Convert_v1beta1_PodGroup_To_scheduling_PodGroup(in *PodGroup, out *scheduling.PodGroup, s conversion.Scope) error {
	return autoConvert_v1beta1_PodGroup_To_scheduling_PodGroup(in, out, s)
}

func autoConvert_scheduling_PodGroup_To_v1beta1_PodGroup(in *scheduling.PodGroup, out *PodGroup, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	if err := Convert_scheduling_PodGroupSpec_To_v1beta1_PodGroupSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_scheduling_PodGroupStatus_To_v1beta1_PodGroupStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

// Convert_scheduling_PodGroup_To_v1beta1_PodGroup is an autogenerated conversion function.
func Convert_scheduling_PodGroup_To_v1beta1_PodGroup(in *scheduling.PodGroup, out *PodGroup, s conversion.Scope) error {
	return autoConvert_scheduling_PodGroup_To_v1beta1_PodGroup(in, out, s)
}

func autoConvert_v1beta1_PodGroupCondition_To_scheduling_PodGroupCondition(in *PodGroupCondition, out *scheduling.PodGroupCondition, s conversion.Scope) error {
	out.Type = scheduling.PodGroupConditionType(in.Type)
	out.Status = v1.ConditionStatus(in.Status)
	out.TransitionID = in.TransitionID
	out.LastTransitionTime = in.LastTransitionTime
	out.Reason = in.Reason
	out.Message = in.Message
	return nil
}

// Convert_v1beta1_PodGroupCondition_To_scheduling_PodGroupCondition is an autogenerated conversion function.
func Convert_v1beta1_PodGroupCondition_To_scheduling_PodGroupCondition(in *PodGroupCondition, out *scheduling.PodGroupCondition, s conversion.Scope) error {
	return autoConvert_v1beta1_PodGroupCondition_To_scheduling_PodGroupCondition(in, out, s)
}

func autoConvert_scheduling_PodGroupCondition_To_v1beta1_PodGroupCondition(in *scheduling.PodGroupCondition, out *PodGroupCondition, s conversion.Scope) error {
	out.Type = PodGroupConditionType(in.Type)
	out.Status = v1.ConditionStatus(in.Status)
	out.TransitionID = in.TransitionID
	out.LastTransitionTime = in.LastTransitionTime
	out.Reason = in.Reason
	out.Message = in.Message
	return nil
}

// Convert_scheduling_PodGroupCondition_To_v1beta1_PodGroupCondition is an autogenerated conversion function.
func Convert_scheduling_PodGroupCondition_To_v1beta1_PodGroupCondition(in *scheduling.PodGroupCondition, out *PodGroupCondition, s conversion.Scope) error {
	return autoConvert_scheduling_PodGroupCondition_To_v1beta1_PodGroupCondition(in, out, s)
}

func autoConvert_v1beta1_PodGroupList_To_scheduling_PodGroupList(in *PodGroupList, out *scheduling.PodGroupList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	out.Items = *(*[]scheduling.PodGroup)(unsafe.Pointer(&in.Items))
	return nil
}

// Convert_v1beta1_PodGroupList_To_scheduling_PodGroupList is an autogenerated conversion function.
func Convert_v1beta1_PodGroupList_To_scheduling_PodGroupList(in *PodGroupList, out *scheduling.PodGroupList, s conversion.Scope) error {
	return autoConvert_v1beta1_PodGroupList_To_scheduling_PodGroupList(in, out, s)
}

func autoConvert_scheduling_PodGroupList_To_v1beta1_PodGroupList(in *scheduling.PodGroupList, out *PodGroupList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	out.Items = *(*[]PodGroup)(unsafe.Pointer(&in.Items))
	return nil
}

// Convert_scheduling_PodGroupList_To_v1beta1_PodGroupList is an autogenerated conversion function.
func Convert_scheduling_PodGroupList_To_v1beta1_PodGroupList(in *scheduling.PodGroupList, out *PodGroupList, s conversion.Scope) error {
	return autoConvert_scheduling_PodGroupList_To_v1beta1_PodGroupList(in, out, s)
}

func autoConvert_v1beta1_PodGroupSpec_To_scheduling_PodGroupSpec(in *PodGroupSpec, out *scheduling.PodGroupSpec, s conversion.Scope) error {
	out.MinMember = in.MinMember
	out.Queue = in.Queue
	out.PriorityClassName = in.PriorityClassName
	out.MinResources = (*v1.ResourceList)(unsafe.Pointer(in.MinResources))
	out.TotalLeaseJobCnt = in.TotalLeaseJobCnt
	out.CurrentLeaseJobCnt = in.CurrentLeaseJobCnt
	out.JobGroupCreationTimeStamp = (*metav1.Time)(unsafe.Pointer(in.JobGroupCreationTimeStamp))
	out.CurrentJobScheduledTimeStamp = (*metav1.Time)(unsafe.Pointer(in.CurrentJobScheduledTimeStamp))
	out.FormerJobDeletionTimeStamp = (*metav1.Time)(unsafe.Pointer(in.FormerJobDeletionTimeStamp))
	out.JobGroupName = in.JobGroupName
	return nil
}

// Convert_v1beta1_PodGroupSpec_To_scheduling_PodGroupSpec is an autogenerated conversion function.
func Convert_v1beta1_PodGroupSpec_To_scheduling_PodGroupSpec(in *PodGroupSpec, out *scheduling.PodGroupSpec, s conversion.Scope) error {
	return autoConvert_v1beta1_PodGroupSpec_To_scheduling_PodGroupSpec(in, out, s)
}

func autoConvert_scheduling_PodGroupSpec_To_v1beta1_PodGroupSpec(in *scheduling.PodGroupSpec, out *PodGroupSpec, s conversion.Scope) error {
	out.MinMember = in.MinMember
	out.Queue = in.Queue
	out.PriorityClassName = in.PriorityClassName
	out.MinResources = (*v1.ResourceList)(unsafe.Pointer(in.MinResources))
	out.TotalLeaseJobCnt = in.TotalLeaseJobCnt
	out.CurrentLeaseJobCnt = in.CurrentLeaseJobCnt
	out.JobGroupCreationTimeStamp = (*metav1.Time)(unsafe.Pointer(in.JobGroupCreationTimeStamp))
	out.CurrentJobScheduledTimeStamp = (*metav1.Time)(unsafe.Pointer(in.CurrentJobScheduledTimeStamp))
	out.FormerJobDeletionTimeStamp = (*metav1.Time)(unsafe.Pointer(in.FormerJobDeletionTimeStamp))
	out.JobGroupName = in.JobGroupName
	return nil
}

// Convert_scheduling_PodGroupSpec_To_v1beta1_PodGroupSpec is an autogenerated conversion function.
func Convert_scheduling_PodGroupSpec_To_v1beta1_PodGroupSpec(in *scheduling.PodGroupSpec, out *PodGroupSpec, s conversion.Scope) error {
	return autoConvert_scheduling_PodGroupSpec_To_v1beta1_PodGroupSpec(in, out, s)
}

func autoConvert_v1beta1_PodGroupStatus_To_scheduling_PodGroupStatus(in *PodGroupStatus, out *scheduling.PodGroupStatus, s conversion.Scope) error {
	out.Phase = scheduling.PodGroupPhase(in.Phase)
	out.Conditions = *(*[]scheduling.PodGroupCondition)(unsafe.Pointer(&in.Conditions))
	out.Running = in.Running
	out.Succeeded = in.Succeeded
	out.Failed = in.Failed
	return nil
}

// Convert_v1beta1_PodGroupStatus_To_scheduling_PodGroupStatus is an autogenerated conversion function.
func Convert_v1beta1_PodGroupStatus_To_scheduling_PodGroupStatus(in *PodGroupStatus, out *scheduling.PodGroupStatus, s conversion.Scope) error {
	return autoConvert_v1beta1_PodGroupStatus_To_scheduling_PodGroupStatus(in, out, s)
}

func autoConvert_scheduling_PodGroupStatus_To_v1beta1_PodGroupStatus(in *scheduling.PodGroupStatus, out *PodGroupStatus, s conversion.Scope) error {
	out.Phase = PodGroupPhase(in.Phase)
	out.Conditions = *(*[]PodGroupCondition)(unsafe.Pointer(&in.Conditions))
	out.Running = in.Running
	out.Succeeded = in.Succeeded
	out.Failed = in.Failed
	return nil
}

// Convert_scheduling_PodGroupStatus_To_v1beta1_PodGroupStatus is an autogenerated conversion function.
func Convert_scheduling_PodGroupStatus_To_v1beta1_PodGroupStatus(in *scheduling.PodGroupStatus, out *PodGroupStatus, s conversion.Scope) error {
	return autoConvert_scheduling_PodGroupStatus_To_v1beta1_PodGroupStatus(in, out, s)
}

func autoConvert_v1beta1_Queue_To_scheduling_Queue(in *Queue, out *scheduling.Queue, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	if err := Convert_v1beta1_QueueSpec_To_scheduling_QueueSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_v1beta1_QueueStatus_To_scheduling_QueueStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

// Convert_v1beta1_Queue_To_scheduling_Queue is an autogenerated conversion function.
func Convert_v1beta1_Queue_To_scheduling_Queue(in *Queue, out *scheduling.Queue, s conversion.Scope) error {
	return autoConvert_v1beta1_Queue_To_scheduling_Queue(in, out, s)
}

func autoConvert_scheduling_Queue_To_v1beta1_Queue(in *scheduling.Queue, out *Queue, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	if err := Convert_scheduling_QueueSpec_To_v1beta1_QueueSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_scheduling_QueueStatus_To_v1beta1_QueueStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

// Convert_scheduling_Queue_To_v1beta1_Queue is an autogenerated conversion function.
func Convert_scheduling_Queue_To_v1beta1_Queue(in *scheduling.Queue, out *Queue, s conversion.Scope) error {
	return autoConvert_scheduling_Queue_To_v1beta1_Queue(in, out, s)
}

func autoConvert_v1beta1_QueueList_To_scheduling_QueueList(in *QueueList, out *scheduling.QueueList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]scheduling.Queue, len(*in))
		for i := range *in {
			if err := Convert_v1beta1_Queue_To_scheduling_Queue(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

// Convert_v1beta1_QueueList_To_scheduling_QueueList is an autogenerated conversion function.
func Convert_v1beta1_QueueList_To_scheduling_QueueList(in *QueueList, out *scheduling.QueueList, s conversion.Scope) error {
	return autoConvert_v1beta1_QueueList_To_scheduling_QueueList(in, out, s)
}

func autoConvert_scheduling_QueueList_To_v1beta1_QueueList(in *scheduling.QueueList, out *QueueList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Queue, len(*in))
		for i := range *in {
			if err := Convert_scheduling_Queue_To_v1beta1_Queue(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

// Convert_scheduling_QueueList_To_v1beta1_QueueList is an autogenerated conversion function.
func Convert_scheduling_QueueList_To_v1beta1_QueueList(in *scheduling.QueueList, out *QueueList, s conversion.Scope) error {
	return autoConvert_scheduling_QueueList_To_v1beta1_QueueList(in, out, s)
}

func autoConvert_v1beta1_QueueSpec_To_scheduling_QueueSpec(in *QueueSpec, out *scheduling.QueueSpec, s conversion.Scope) error {
	out.Weight = in.Weight
	out.Capability = *(*v1.ResourceList)(unsafe.Pointer(&in.Capability))
	out.Reclaimable = (*bool)(unsafe.Pointer(in.Reclaimable))
	out.QuotaRatio = in.QuotaRatio
	return nil
}

// Convert_v1beta1_QueueSpec_To_scheduling_QueueSpec is an autogenerated conversion function.
func Convert_v1beta1_QueueSpec_To_scheduling_QueueSpec(in *QueueSpec, out *scheduling.QueueSpec, s conversion.Scope) error {
	return autoConvert_v1beta1_QueueSpec_To_scheduling_QueueSpec(in, out, s)
}

func autoConvert_scheduling_QueueSpec_To_v1beta1_QueueSpec(in *scheduling.QueueSpec, out *QueueSpec, s conversion.Scope) error {
	out.Weight = in.Weight
	out.Capability = *(*v1.ResourceList)(unsafe.Pointer(&in.Capability))
	// WARNING: in.State requires manual conversion: does not exist in peer-type
	out.Reclaimable = (*bool)(unsafe.Pointer(in.Reclaimable))
	out.QuotaRatio = in.QuotaRatio
	return nil
}

func autoConvert_v1beta1_QueueStatus_To_scheduling_QueueStatus(in *QueueStatus, out *scheduling.QueueStatus, s conversion.Scope) error {
	out.State = scheduling.QueueState(in.State)
	out.Unknown = in.Unknown
	out.Pending = in.Pending
	out.Running = in.Running
	out.Inqueue = in.Inqueue
	return nil
}

// Convert_v1beta1_QueueStatus_To_scheduling_QueueStatus is an autogenerated conversion function.
func Convert_v1beta1_QueueStatus_To_scheduling_QueueStatus(in *QueueStatus, out *scheduling.QueueStatus, s conversion.Scope) error {
	return autoConvert_v1beta1_QueueStatus_To_scheduling_QueueStatus(in, out, s)
}

func autoConvert_scheduling_QueueStatus_To_v1beta1_QueueStatus(in *scheduling.QueueStatus, out *QueueStatus, s conversion.Scope) error {
	out.State = QueueState(in.State)
	out.Unknown = in.Unknown
	out.Pending = in.Pending
	out.Running = in.Running
	out.Inqueue = in.Inqueue
	return nil
}

// Convert_scheduling_QueueStatus_To_v1beta1_QueueStatus is an autogenerated conversion function.
func Convert_scheduling_QueueStatus_To_v1beta1_QueueStatus(in *scheduling.QueueStatus, out *QueueStatus, s conversion.Scope) error {
	return autoConvert_scheduling_QueueStatus_To_v1beta1_QueueStatus(in, out, s)
}
