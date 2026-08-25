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

package controller

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	finopsv1alpha1 "github.com/b100to/platform-lab/operators/idle-reaper/api/v1alpha1"
)

// wakeState summarises the WakeRequests pointing at one IdleWindow.
type wakeState struct {
	// active is how many unexpired requests are holding the namespace awake.
	active int32
	// earliestExpiry is when the first of them stops holding, or zero when
	// none are active. It is a reconcile deadline: the namespace has to go
	// back to sleep the moment the last request lapses, not at the next
	// scheduled boundary.
	earliestExpiry time.Time
}

// evaluateWakeRequests decides which requests still hold, and records the
// outcome on each one.
//
// The expiry is derived from the object's own creation time rather than taken
// from the spec. A request that could set its own end time could also extend
// itself, and the point of the object is that nobody has to remember to
// cancel it.
func (r *IdleWindowReconciler) evaluateWakeRequests(
	ctx context.Context,
	window *finopsv1alpha1.IdleWindow,
	now time.Time,
) (wakeState, error) {
	var list finopsv1alpha1.WakeRequestList
	if err := r.List(ctx, &list, client.InNamespace(window.Namespace)); err != nil {
		return wakeState{}, err
	}

	maxWake, err := time.ParseDuration(window.Spec.MaxWakeDuration)
	if err != nil {
		// An unparsable cap must not become an unlimited one.
		maxWake = 0
	}

	var state wakeState
	for i := range list.Items {
		req := &list.Items[i]
		phase, expiresAt := classifyWakeRequest(req, maxWake, now)

		if phase == finopsv1alpha1.WakePhaseActive {
			state.active++
			if state.earliestExpiry.IsZero() || expiresAt.Before(state.earliestExpiry) {
				state.earliestExpiry = expiresAt
			}
		}

		if err := r.recordWakeRequest(ctx, window, req, phase, expiresAt); err != nil {
			return state, err
		}
	}
	return state, nil
}

// classifyWakeRequest decides a single request's phase without touching the API.
func classifyWakeRequest(
	req *finopsv1alpha1.WakeRequest,
	maxWake time.Duration,
	now time.Time,
) (string, time.Time) {
	requested, err := time.ParseDuration(req.Spec.Duration)
	if err != nil {
		return finopsv1alpha1.WakePhaseRejected, time.Time{}
	}
	if maxWake <= 0 || requested > maxWake {
		return finopsv1alpha1.WakePhaseRejected, time.Time{}
	}

	expiresAt := req.CreationTimestamp.Time.Add(requested)
	if now.Before(expiresAt) {
		return finopsv1alpha1.WakePhaseActive, expiresAt
	}
	return finopsv1alpha1.WakePhaseExpired, expiresAt
}

// recordWakeRequest writes a request's outcome back, and announces the
// transitions worth announcing.
func (r *IdleWindowReconciler) recordWakeRequest(
	ctx context.Context,
	window *finopsv1alpha1.IdleWindow,
	req *finopsv1alpha1.WakeRequest,
	phase string,
	expiresAt time.Time,
) error {
	previous := req.Status.Phase
	if previous == phase && req.Status.Window == window.Name {
		return nil
	}

	req.Status.Phase = phase
	req.Status.Window = window.Name
	if !expiresAt.IsZero() {
		t := metav1.NewTime(expiresAt)
		req.Status.ExpiresAt = &t
	}

	switch phase {
	case finopsv1alpha1.WakePhaseActive:
		r.event(window, corev1EventNormal, "WakeGranted",
			req.Name+": awake until "+expiresAt.Format(time.RFC3339)+" -- "+req.Spec.Reason)
	case finopsv1alpha1.WakePhaseExpired:
		r.event(window, corev1EventNormal, "WakeExpired",
			req.Name+": expired, the schedule applies again")
	case finopsv1alpha1.WakePhaseRejected:
		r.event(window, corev1EventWarning, "WakeRejected",
			req.Name+": duration "+req.Spec.Duration+" exceeds maxWakeDuration "+window.Spec.MaxWakeDuration)
	}

	return ignoreConflict(r.Status().Update(ctx, req))
}
