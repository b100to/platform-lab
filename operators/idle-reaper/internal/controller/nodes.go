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

	corev1 "k8s.io/api/core/v1"
)

const (
	// labelControlPlane marks nodes that run the control plane. Their static
	// pods cannot be evicted and the node is never a candidate for removal.
	labelControlPlane = "node-role.kubernetes.io/control-plane"

	// labelComputeType is set to "fargate" on the synthetic node EKS creates
	// per Fargate pod. Those are not capacity anyone provisioned or can
	// reclaim: the node exists because the pod does, and disappears with it.
	// Counting them makes drainableNodes a fraction that can never be filled,
	// because the pod holding the node is the reason the node is there.
	//
	// This matters more than it sounds: running the controller itself on
	// Fargate is a common way to avoid depending on the EC2 capacity it is
	// trying to reclaim.
	labelComputeType = "eks.amazonaws.com/compute-type"
)

// nodeCensus is what one pass over the cluster's nodes found.
type nodeCensus struct {
	workers   int32
	drainable int32
}

// countDrainableNodes reports how many worker nodes hold nothing but
// DaemonSet pods.
//
// Emptying workloads is only half of a cost saving: the money is in the node
// going away, and a node only goes away once nothing is left that has to be
// evicted first. DaemonSet pods do not count against that — they disappear
// with the node rather than needing somewhere else to go — which is the same
// rule node autoscalers apply when deciding whether a node is empty.
//
// Removing the node is deliberately left to whatever manages nodes. This
// controller only reports how many became removable, so the two concerns do
// not fight over the same resource.
func (r *IdleWindowReconciler) countDrainableNodes(ctx context.Context) (nodeCensus, error) {
	var nodes corev1.NodeList
	if err := r.List(ctx, &nodes); err != nil {
		return nodeCensus{}, err
	}

	// One list of every pod, bucketed in memory. A field-indexed query per node
	// would need an index registered on the manager's cache and would still be
	// one round trip per node.
	var pods corev1.PodList
	if err := r.List(ctx, &pods); err != nil {
		return nodeCensus{}, err
	}
	holders := map[string]int{}
	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Spec.NodeName == "" || !podHoldsNode(pod) {
			continue
		}
		holders[pod.Spec.NodeName]++
	}

	var census nodeCensus
	for i := range nodes.Items {
		node := &nodes.Items[i]
		if _, isControlPlane := node.Labels[labelControlPlane]; isControlPlane {
			continue
		}
		if node.Labels[labelComputeType] == "fargate" {
			continue
		}
		census.workers++
		if holders[node.Name] == 0 {
			census.drainable++
		}
	}
	return census, nil
}

// podHoldsNode reports whether a pod would have to be rescheduled elsewhere
// before its node could be removed.
func podHoldsNode(pod *corev1.Pod) bool {
	// Terminal pods release their node once garbage collected.
	if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		return false
	}
	// DaemonSet pods disappear with the node instead of needing a new home,
	// which is why node autoscalers discount them too.
	return !ownedByDaemonSet(pod)
}

func ownedByDaemonSet(pod *corev1.Pod) bool {
	for _, ref := range pod.OwnerReferences {
		if ref.Kind == "DaemonSet" {
			return true
		}
	}
	return false
}
