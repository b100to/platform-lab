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
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	finopsv1alpha1 "github.com/b100to/platform-lab/operators/idle-reaper/api/v1alpha1"
)

// Metrics exist so alerting can ask questions Events cannot answer.
//
// Events expire — an hour by default — so "has this been blocked all
// afternoon?" has no answer in the event stream. Conditions hold the current
// state but only for whoever reads that specific object. A gauge is the thing
// an alert rule can evaluate over time, which is why the useful question here
// — "it is the middle of the idle window and still nothing is reclaimable" —
// is expressible only as a metric.
//
// Alert routing deliberately stops at exposing these. Putting a webhook inside
// the operator would bury notification policy in a component that has no
// business holding credentials for it.
var (
	windowLabels = []string{"namespace", "window"}

	metricAsleep = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "idlereaper_window_asleep",
		Help: "1 when the current time is inside the idle window, 0 otherwise.",
	}, windowLabels)

	metricScaled = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "idlereaper_scaled_workloads",
		Help: "Workloads this window currently holds scaled down.",
	}, windowLabels)

	metricBlocked = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "idlereaper_blocked_workloads",
		Help: "Selected workloads deliberately left alone, by an HPA or a manual scale.",
	}, windowLabels)

	metricBlockingPDBs = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "idlereaper_blocking_pdbs",
		Help: "PodDisruptionBudgets in this namespace that currently allow no disruption.",
	}, windowLabels)

	metricReclaimedCPU = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "idlereaper_reclaimed_cpu_millicores",
		Help: "CPU requests released by this window, in millicores.",
	}, windowLabels)

	metricReclaimedMemory = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "idlereaper_reclaimed_memory_bytes",
		Help: "Memory requests released by this window, in bytes.",
	}, windowLabels)

	// Node counts are a property of the cluster, not of any one window. Two
	// IdleWindows in different namespaces observe the same nodes, so labelling
	// these per window would publish the same number several times and invite
	// double counting in a sum().
	metricDrainableNodes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "idlereaper_drainable_nodes",
		Help: "Worker nodes whose only remaining pods are managed by DaemonSets.",
	})

	metricWorkerNodes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "idlereaper_worker_nodes",
		Help: "Nodes considered for reclaim: neither control-plane nor Fargate.",
	})
)

func init() {
	metrics.Registry.MustRegister(
		metricAsleep,
		metricScaled,
		metricBlocked,
		metricBlockingPDBs,
		metricReclaimedCPU,
		metricReclaimedMemory,
		metricDrainableNodes,
		metricWorkerNodes,
	)
}

// publishMetrics mirrors the status just written.
func publishMetrics(w *finopsv1alpha1.IdleWindow, census nodeCensus) {
	labels := prometheus.Labels{"namespace": w.Namespace, "window": w.Name}

	asleep := 0.0
	if w.Status.Phase == finopsv1alpha1.PhaseAsleep {
		asleep = 1
	}
	metricAsleep.With(labels).Set(asleep)
	metricScaled.With(labels).Set(float64(w.Status.AffectedWorkloads))
	metricBlocked.With(labels).Set(float64(w.Status.SkippedWorkloads))
	metricBlockingPDBs.With(labels).Set(float64(w.Status.BlockingPDBs))

	metricReclaimedCPU.With(labels).Set(float64(w.Status.Reclaimed.Cpu().MilliValue()))
	metricReclaimedMemory.With(labels).Set(float64(w.Status.Reclaimed.Memory().Value()))

	metricDrainableNodes.Set(float64(census.drainable))
	metricWorkerNodes.Set(float64(census.workers))
}

// forgetMetrics drops the series for a window that no longer exists.
//
// A GaugeVec keeps every label combination it has ever seen. Without this, a
// deleted IdleWindow keeps reporting its last value forever, and any alert
// watching for "asleep but nothing reclaimed" would fire on an object nobody
// can find.
func forgetMetrics(namespace, name string) {
	labels := prometheus.Labels{"namespace": namespace, "window": name}
	for _, m := range []*prometheus.GaugeVec{
		metricAsleep, metricScaled, metricBlocked,
		metricBlockingPDBs, metricReclaimedCPU, metricReclaimedMemory,
	} {
		m.Delete(labels)
	}
}
