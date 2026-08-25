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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ConditionAccepted reports whether the cluster took the request, and when it
// did not, why.
const ConditionAccepted = "Accepted"

// WakeRequest phases.
const (
	// WakePhaseActive means the request still holds its namespace awake.
	WakePhaseActive = "Active"
	// WakePhaseExpired means its duration has elapsed.
	WakePhaseExpired = "Expired"
	// WakePhaseRejected means the request asked for longer than the
	// IdleWindow allows, or no window covers this namespace.
	WakePhaseRejected = "Rejected"
)

// WakeRequestSpec asks for an idle window to be lifted for a while.
//
// This is deliberately a separate object from IdleWindow. The window is
// policy and belongs to whoever runs the platform; a request to step around
// it belongs to whoever needs to work tonight. Splitting them lets RBAC say
// exactly that: developers get create on wakerequests and nothing on
// idlewindows, so they can ask for an exception without being able to edit
// the rule.
type WakeRequestSpec struct {
	// duration is how long the namespace should stay awake, as a Go duration
	// ("3h", "90m").
	//
	// A duration rather than an end time on purpose: nobody should be asked
	// to work out a UTC timestamp at two in the morning. It is also what
	// makes the request self-cancelling — the expiry is derived, so there is
	// nothing for a person to remember to undo.
	// +required
	// +kubebuilder:validation:Pattern=`^([0-9]+(\.[0-9]+)?(s|m|h))+$`
	Duration string `json:"duration"`

	// reason is why the window is being lifted. Required, because a namespace
	// waking up at 3am with no explanation is exactly the thing someone will
	// want explained later — and repeated reasons are themselves a cost
	// signal about how a team works.
	// +required
	// +kubebuilder:validation:MinLength=3
	// +kubebuilder:validation:MaxLength=200
	Reason string `json:"reason"`

	// requestedBy names the person this was raised for. A front end that
	// creates requests on someone's behalf should fill it in; it is a label
	// for humans reading the object, not an authorization input, and the
	// audit trail that counts is the API server's own.
	// +optional
	RequestedBy string `json:"requestedBy,omitempty"`
}

// WakeRequestStatus reports whether the request is still holding.
type WakeRequestStatus struct {
	// phase is Active, Expired, or Rejected.
	// +optional
	Phase string `json:"phase,omitempty"`

	// expiresAt is when this request stops holding the namespace awake.
	// Derived from the creation time and duration rather than supplied, so
	// editing it after the fact does not extend the exception.
	// +optional
	ExpiresAt *metav1.Time `json:"expiresAt,omitempty"`

	// window names the IdleWindow this request applies to.
	// +optional
	Window string `json:"window,omitempty"`

	// conditions carry why a request was rejected.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=wr
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="For",type=string,JSONPath=`.spec.duration`
// +kubebuilder:printcolumn:name="Expires",type=string,JSONPath=`.status.expiresAt`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.spec.reason`
// +kubebuilder:printcolumn:name="By",type=string,JSONPath=`.spec.requestedBy`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// WakeRequest is the Schema for the wakerequests API
type WakeRequest struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of WakeRequest
	// +required
	Spec WakeRequestSpec `json:"spec"`

	// status defines the observed state of WakeRequest
	// +optional
	Status WakeRequestStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// WakeRequestList contains a list of WakeRequest
type WakeRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []WakeRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(scheme *runtime.Scheme) error {
		scheme.AddKnownTypes(SchemeGroupVersion, &WakeRequest{}, &WakeRequestList{})
		return nil
	})
}
