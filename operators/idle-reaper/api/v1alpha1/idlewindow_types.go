/*
Copyright 2026.

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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Annotations written onto the workloads this controller scales.
//
// The original replica count lives on the workload rather than in IdleWindow
// status on purpose: deleting the IdleWindow, or losing the controller, must
// not strand a workload at zero with no record of where it came from.
// See DESIGN.md, decision D1.
const (
	// AnnotationSavedReplicas holds the replica count observed before the
	// first scale-down. It is the value restored on wake.
	AnnotationSavedReplicas = "finops.b100to.dev/saved-replicas"

	// AnnotationAppliedReplicas holds the replica count this controller last
	// wrote. A workload whose current replicas differ from this was changed by
	// someone else, which is how manual intervention is detected (D3).
	AnnotationAppliedReplicas = "finops.b100to.dev/applied-replicas"

	// AnnotationOwnedBy names the IdleWindow responsible for a workload, so two
	// overlapping selectors do not silently fight over the same Deployment.
	AnnotationOwnedBy = "finops.b100to.dev/owned-by"
)

// Phase values reported in IdleWindowStatus.
const (
	// PhaseAwake means the current time is outside the idle window.
	PhaseAwake = "Awake"
	// PhaseAsleep means the current time is inside the idle window.
	PhaseAsleep = "Asleep"
)

// Condition types reported in IdleWindowStatus.
const (
	// ConditionReady is false when the schedule cannot be parsed or the
	// controller cannot act on the selected workloads.
	ConditionReady = "Ready"
)

// IdleWindowSpec declares when a set of workloads is considered idle.
//
// It describes a recurring window, not an action. The controller compares the
// current time against this declaration on every reconcile, so a missed event
// or a controller restart cannot leave workloads in the wrong state.
type IdleWindowSpec struct {
	// selector narrows which Deployments in this namespace the window applies
	// to. Omit it to cover the whole namespace, which is the common case: an
	// IdleWindow already scopes itself to one namespace, and declaring "this
	// namespace is idle at these hours" is the thing most people mean.
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// sleepAt is a five-field cron expression marking the start of the idle
	// window, evaluated in timezone.
	// +required
	// +kubebuilder:validation:MinLength=9
	SleepAt string `json:"sleepAt"`

	// wakeAt is a five-field cron expression marking the end of the idle
	// window, evaluated in timezone.
	// +required
	// +kubebuilder:validation:MinLength=9
	WakeAt string `json:"wakeAt"`

	// timezone is an IANA location name used to evaluate sleepAt and wakeAt.
	// Schedules are written by people in a specific place; pinning to UTC
	// silently shifts the window twice a year for locations with DST.
	// +optional
	// +kubebuilder:default="Asia/Seoul"
	Timezone string `json:"timezone,omitempty"`

	// minReplicas is the replica count applied during the idle window.
	// Zero reclaims the most, but some workloads must stay reachable.
	// +optional
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// skipIfHPA leaves workloads with a HorizontalPodAutoscaler untouched.
	// Two controllers writing the same replica field oscillate, so v1alpha1
	// avoids the conflict instead of resolving it (D4).
	// +optional
	// +kubebuilder:default=true
	SkipIfHPA *bool `json:"skipIfHPA,omitempty"`

	// respectManualScale leaves a workload alone for the remainder of the
	// current window if someone changed its replica count by hand. Automation
	// overriding a person who scaled up in a hurry is an incident, not a
	// correction (D3).
	// +optional
	// +kubebuilder:default=true
	RespectManualScale *bool `json:"respectManualScale,omitempty"`

	// suspend stops the controller from acting without deleting the object,
	// so a window can be disabled during an incident and restored afterwards.
	// +optional
	// +kubebuilder:default=false
	Suspend *bool `json:"suspend,omitempty"`
}

// IdleWindowStatus reports what the controller observed and did.
type IdleWindowStatus struct {
	// phase is Awake or Asleep, derived from the current time.
	// +optional
	Phase string `json:"phase,omitempty"`

	// reclaimed is the sum of resource requests currently not scheduled
	// because of this window. This is a measurement, not a cost estimate —
	// converting it to money requires pricing the controller does not have.
	// +optional
	Reclaimed corev1.ResourceList `json:"reclaimed,omitempty"`

	// affectedWorkloads counts the Deployments this window currently holds
	// scaled down.
	// +optional
	AffectedWorkloads int32 `json:"affectedWorkloads,omitempty"`

	// skippedWorkloads counts selected Deployments deliberately left alone —
	// an attached HPA, or a manual scale being respected. Without this the
	// controller looks like it silently did nothing.
	// +optional
	SkippedWorkloads int32 `json:"skippedWorkloads,omitempty"`

	// nextTransitionTime is when the phase is expected to change. It doubles
	// as a check that the schedule was parsed the way the author intended.
	// +optional
	NextTransitionTime *metav1.Time `json:"nextTransitionTime,omitempty"`

	// lastTransitionTime is when the phase last changed.
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`

	// observedGeneration is the spec generation this status reflects.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// conditions represent the current state of the IdleWindow resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=iw
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Scaled",type=integer,JSONPath=`.status.affectedWorkloads`
// +kubebuilder:printcolumn:name="Skipped",type=integer,JSONPath=`.status.skippedWorkloads`
// +kubebuilder:printcolumn:name="CPU",type=string,JSONPath=`.status.reclaimed.cpu`
// +kubebuilder:printcolumn:name="Next",type=string,JSONPath=`.status.nextTransitionTime`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// IdleWindow is the Schema for the idlewindows API
type IdleWindow struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of IdleWindow
	// +required
	Spec IdleWindowSpec `json:"spec"`

	// status defines the observed state of IdleWindow
	// +optional
	Status IdleWindowStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// IdleWindowList contains a list of IdleWindow
type IdleWindowList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []IdleWindow `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(scheme *runtime.Scheme) error {
		scheme.AddKnownTypes(SchemeGroupVersion, &IdleWindow{}, &IdleWindowList{})
		return nil
	})
}
