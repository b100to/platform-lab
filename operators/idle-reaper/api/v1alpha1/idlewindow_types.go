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
	// PhaseWakeRequested means the schedule says asleep but an unexpired
	// WakeRequest is holding the namespace up.
	PhaseWakeRequested = "WakeRequested"
)

// Condition types reported in IdleWindowStatus.
const (
	// ConditionReady is false when the schedule cannot be parsed or the
	// controller cannot act on the selected workloads.
	ConditionReady = "Ready"

	// ConditionUnblocked is false when something stops the namespace from
	// being fully reclaimed: a workload this controller will not scale, or a
	// PodDisruptionBudget that will not let a node be drained. Reclaiming
	// nothing and reclaiming everything both look like success in a bare
	// counter; this is where the difference is stated.
	ConditionUnblocked = "Unblocked"
)

// HPA policy values.
const (
	HPAPolicySkip  = "Skip"
	HPAPolicyWarn  = "Warn"
	HPAPolicyScale = "Scale"
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

	// hpaPolicy decides what to do with a workload whose replica count is
	// already owned by a HorizontalPodAutoscaler.
	//
	//	Skip  - leave it alone, quietly
	//	Warn  - leave it alone and say so, in an Event and in conditions
	//	Scale - treat it like any other workload
	//
	// Warn is the default because silence is the wrong answer here: a
	// workload that never shrinks is exactly what someone reading this object
	// needs to know about, and a dev-environment HPA that pins replicas is
	// usually a mistake worth surfacing rather than absorbing.
	//
	// Scale exists for the case where the HPA is known to be dormant, and is
	// deliberately not the default: two controllers writing the same field
	// will fight.
	// +optional
	// +kubebuilder:default=Warn
	// +kubebuilder:validation:Enum=Skip;Warn;Scale
	HPAPolicy string `json:"hpaPolicy,omitempty"`

	// respectManualScale leaves a workload alone for the remainder of the
	// current window if someone changed its replica count by hand. Automation
	// overriding a person who scaled up in a hurry is an incident, not a
	// correction (D3).
	// +optional
	// +kubebuilder:default=true
	RespectManualScale *bool `json:"respectManualScale,omitempty"`

	// maxWakeDuration caps how long a single WakeRequest may hold this
	// namespace awake. Requests asking for more are rejected.
	//
	// The cap lives here, on the policy object, rather than in whatever front
	// end raises requests. A Slack command or a portal is a client of this
	// API, and a client is the wrong place to enforce a limit: the API server
	// has to be the thing that says no, or the limit only holds while the
	// client behaves.
	// +optional
	// +kubebuilder:default="8h"
	// +kubebuilder:validation:Pattern=`^([0-9]+(\.[0-9]+)?(s|m|h))+$`
	MaxWakeDuration string `json:"maxWakeDuration,omitempty"`

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
	AffectedWorkloads int32 `json:"affectedWorkloads"`

	// skippedWorkloads counts selected Deployments deliberately left alone —
	// an attached HPA, or a manual scale being respected. Without this the
	// controller looks like it silently did nothing.
	// +optional
	SkippedWorkloads int32 `json:"skippedWorkloads"`

	// blockingPDBs counts PodDisruptionBudgets in this namespace that
	// currently allow no disruption at all. Those do not stop this controller
	// from changing replica counts, but they do stop a node from being
	// drained, which is where the saving actually comes from.
	// +optional
	BlockingPDBs int32 `json:"blockingPDBs"`

	// drainableNodes counts worker nodes whose only remaining pods are managed
	// by DaemonSets. Those nodes are the ones a node autoscaler can actually
	// remove, and removing them is where the money is — scaling pods to zero
	// on a node that stays running saves nothing.
	//
	// This is a cluster-wide observation reported by a namespaced object, so
	// two IdleWindows in different namespaces will report the same figure.
	// +optional
	DrainableNodes int32 `json:"drainableNodes"`

	// workerNodes is the number of non-control-plane nodes considered, so
	// drainableNodes can be read as a fraction rather than a bare count.
	// +optional
	WorkerNodes int32 `json:"workerNodes"`

	// activeWakeRequests counts the requests currently holding this namespace
	// awake, so a window that is awake off-schedule says why.
	// +optional
	ActiveWakeRequests int32 `json:"activeWakeRequests"`

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
// +kubebuilder:printcolumn:name="Drainable",type=string,JSONPath=`.status.drainableNodes`
// +kubebuilder:printcolumn:name="Nodes",type=string,JSONPath=`.status.workerNodes`
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
