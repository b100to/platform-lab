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
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	finopsv1alpha1 "github.com/b100to/platform-lab/operators/idle-reaper/api/v1alpha1"
)

// IdleWindowReconciler reconciles an IdleWindow object.
type IdleWindowReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	// Now is injectable so tests can place the clock inside or outside a
	// window without waiting for wall time.
	Now func() time.Time
}

// +kubebuilder:rbac:groups=finops.b100to.dev,resources=idlewindows,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=finops.b100to.dev,resources=idlewindows/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=finops.b100to.dev,resources=idlewindows/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// tally accumulates what one pass over the selected workloads did.
type tally struct {
	affected  int32
	skipped   int32
	reclaimed corev1.ResourceList

	// blockers names what kept a workload from shrinking, so the reason can
	// reach conditions and Events rather than vanishing into a counter.
	blockers []string
}

// Reconcile drives the selected workloads toward the state the schedule
// implies for the current moment.
//
// Nothing about why this was called is used: not the event, not what changed,
// not what happened last time. The current clock and the current cluster state
// are read fresh every pass, which is what makes a missed event, a restarted
// controller, and a duplicate call all behave identically.
func (r *IdleWindowReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var window finopsv1alpha1.IdleWindow
	if err := r.Get(ctx, req.NamespacedName, &window); err != nil {
		// Deleted between the event and this read. Workloads keep their
		// annotations, so their original size is still recoverable by hand.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if ptrBool(window.Spec.Suspend) {
		window.Status.Phase = ""
		r.setReady(&window, metav1.ConditionFalse, "Suspended", "spec.suspend is set; no workloads are managed")
		return ctrl.Result{}, r.writeStatus(ctx, &window)
	}

	state, err := evaluateWindow(window.Spec, r.now())
	if err != nil {
		// A bad schedule is not retryable: requeueing hammers the API server
		// until a human edits the spec. Report it and wait for that edit.
		r.setReady(&window, metav1.ConditionFalse, "InvalidSchedule", err.Error())
		r.event(&window, corev1.EventTypeWarning, "InvalidSchedule", err.Error())
		return ctrl.Result{}, r.writeStatus(ctx, &window)
	}

	targets, err := r.selectDeployments(ctx, &window)
	if err != nil {
		r.setReady(&window, metav1.ConditionFalse, "SelectorError", err.Error())
		_ = r.writeStatus(ctx, &window)
		return ctrl.Result{}, err
	}

	hpaOwned, err := r.deploymentsOwnedByHPA(ctx, window.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	var t tally
	t.reclaimed = corev1.ResourceList{}
	for i := range targets {
		if err := r.reconcileDeployment(ctx, &window, &targets[i], state, hpaOwned, &t); err != nil {
			// One workload failing must not strand the rest: a partial pass is
			// better than an all-or-nothing rollback, and the next reconcile
			// retries whatever is still out of sync.
			log.Error(err, "workload not reconciled", "deployment", targets[i].Name)
			r.event(&window, corev1.EventTypeWarning, "ScaleFailed",
				"deployment "+targets[i].Name+": "+err.Error())
		}
	}

	phase := finopsv1alpha1.PhaseAwake
	if state.asleep {
		phase = finopsv1alpha1.PhaseAsleep
	}
	if window.Status.Phase != phase {
		now := metav1.NewTime(r.now())
		window.Status.LastTransitionTime = &now
		r.event(&window, corev1.EventTypeNormal, "PhaseChanged", "now "+phase)
	}

	census, err := r.countDrainableNodes(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	blockingPDBs, pdbNames, err := r.pdbsBlockingDrain(ctx, window.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	next := metav1.NewTime(state.next)
	window.Status.Phase = phase
	window.Status.AffectedWorkloads = t.affected
	window.Status.SkippedWorkloads = t.skipped
	window.Status.Reclaimed = t.reclaimed
	window.Status.DrainableNodes = census.drainable
	window.Status.WorkerNodes = census.workers
	window.Status.BlockingPDBs = blockingPDBs
	window.Status.NextTransitionTime = &next
	window.Status.ObservedGeneration = window.Generation
	r.setReady(&window, metav1.ConditionTrue, "Reconciled",
		"selected "+strconv.Itoa(len(targets))+" deployments")
	r.setUnblocked(&window, t.blockers, pdbNames)

	if err := r.writeStatus(ctx, &window); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: requeueAfter(r.now(), state.next)}, nil
}

// selectDeployments returns the Deployments this window applies to.
//
// No selector means the whole namespace. The object is already namespace
// scoped, and the statement being made — "this namespace is idle at these
// hours" — is usually about all of it. A selector is how you carve out less,
// not how you opt in.
func (r *IdleWindowReconciler) selectDeployments(ctx context.Context, w *finopsv1alpha1.IdleWindow) ([]appsv1.Deployment, error) {
	opts := []client.ListOption{client.InNamespace(w.Namespace)}

	if w.Spec.Selector != nil {
		sel, err := metav1.LabelSelectorAsSelector(w.Spec.Selector)
		if err != nil {
			return nil, err
		}
		opts = append(opts, client.MatchingLabelsSelector{Selector: sel})
	}

	var list appsv1.DeploymentList
	if err := r.List(ctx, &list, opts...); err != nil {
		return nil, err
	}
	return list.Items, nil
}

// deploymentsOwnedByHPA names the Deployments in a namespace that an HPA
// already drives, so this controller can stay out of their replica field.
func (r *IdleWindowReconciler) deploymentsOwnedByHPA(ctx context.Context, ns string) (map[string]bool, error) {
	var list autoscalingv2.HorizontalPodAutoscalerList
	if err := r.List(ctx, &list, client.InNamespace(ns)); err != nil {
		return nil, err
	}
	owned := make(map[string]bool, len(list.Items))
	for _, h := range list.Items {
		if h.Spec.ScaleTargetRef.Kind == "Deployment" {
			owned[h.Spec.ScaleTargetRef.Name] = true
		}
	}
	return owned, nil
}

// reconcileDeployment brings one workload in line with the window state.
func (r *IdleWindowReconciler) reconcileDeployment(
	ctx context.Context,
	w *finopsv1alpha1.IdleWindow,
	dep *appsv1.Deployment,
	state windowState,
	hpaOwned map[string]bool,
	t *tally,
) error {
	if hpaOwned[dep.Name] && w.Spec.HPAPolicy != finopsv1alpha1.HPAPolicyScale {
		t.skipped++
		if w.Spec.HPAPolicy == finopsv1alpha1.HPAPolicyWarn {
			t.blockers = append(t.blockers, dep.Name+" (HPA)")
			r.event(w, corev1.EventTypeWarning, "NotReclaimed",
				"deployment "+dep.Name+" is scaled by a HorizontalPodAutoscaler and was left alone")
		}
		return nil
	}

	current := int32(1)
	if dep.Spec.Replicas != nil {
		current = *dep.Spec.Replicas
	}
	saved, hasSaved := annotationInt(dep, finopsv1alpha1.AnnotationSavedReplicas)
	applied, hasApplied := annotationInt(dep, finopsv1alpha1.AnnotationAppliedReplicas)

	// Someone changed the replica count since this controller last wrote it.
	// Treat that as a person acting deliberately and stand down until the next
	// boundary, rather than overwriting them.
	manuallyChanged := hasApplied && current != applied
	if manuallyChanged && ptrBool(w.Spec.RespectManualScale) {
		t.skipped++
		if !state.asleep {
			// The window is over and the value is theirs now. Drop the
			// bookkeeping so the next sleep starts from what they chose.
			return r.clearAnnotations(ctx, dep)
		}
		return nil
	}

	if state.asleep {
		min := int32(0)
		if w.Spec.MinReplicas != nil {
			min = *w.Spec.MinReplicas
		}
		original := current
		if hasSaved {
			original = saved
		}

		t.affected++
		addReclaimed(t.reclaimed, podRequests(dep), original-min)

		if current == min && hasSaved {
			return nil // already where it should be
		}
		return r.scaleTo(ctx, w, dep, min, original)
	}

	if !hasSaved {
		return nil // never slept under this controller; not ours to restore
	}
	t.affected++
	if current == saved {
		return r.clearAnnotations(ctx, dep)
	}
	return r.restoreTo(ctx, w, dep, saved)
}

// scaleTo shrinks a workload and records where it came from.
//
// The original count is written onto the Deployment, not into IdleWindow
// status: losing the IdleWindow, or the controller, must never leave a
// workload at zero with no record of its original size.
func (r *IdleWindowReconciler) scaleTo(ctx context.Context, w *finopsv1alpha1.IdleWindow, dep *appsv1.Deployment, to, original int32) error {
	patch := client.MergeFrom(dep.DeepCopy())
	if dep.Annotations == nil {
		dep.Annotations = map[string]string{}
	}
	dep.Annotations[finopsv1alpha1.AnnotationSavedReplicas] = strconv.Itoa(int(original))
	dep.Annotations[finopsv1alpha1.AnnotationAppliedReplicas] = strconv.Itoa(int(to))
	dep.Annotations[finopsv1alpha1.AnnotationOwnedBy] = w.Name
	dep.Spec.Replicas = &to

	if err := r.Patch(ctx, dep, patch); err != nil {
		return err
	}
	r.event(w, corev1.EventTypeNormal, "ScaledDown",
		dep.Name+": "+strconv.Itoa(int(original))+" -> "+strconv.Itoa(int(to)))
	return nil
}

// restoreTo returns a workload to its recorded size and removes the bookkeeping.
func (r *IdleWindowReconciler) restoreTo(ctx context.Context, w *finopsv1alpha1.IdleWindow, dep *appsv1.Deployment, to int32) error {
	patch := client.MergeFrom(dep.DeepCopy())
	delete(dep.Annotations, finopsv1alpha1.AnnotationSavedReplicas)
	delete(dep.Annotations, finopsv1alpha1.AnnotationAppliedReplicas)
	delete(dep.Annotations, finopsv1alpha1.AnnotationOwnedBy)
	dep.Spec.Replicas = &to

	if err := r.Patch(ctx, dep, patch); err != nil {
		return err
	}
	r.event(w, corev1.EventTypeNormal, "ScaledUp", dep.Name+": restored to "+strconv.Itoa(int(to)))
	return nil
}

func (r *IdleWindowReconciler) clearAnnotations(ctx context.Context, dep *appsv1.Deployment) error {
	if _, ok := dep.Annotations[finopsv1alpha1.AnnotationSavedReplicas]; !ok {
		return nil
	}
	patch := client.MergeFrom(dep.DeepCopy())
	delete(dep.Annotations, finopsv1alpha1.AnnotationSavedReplicas)
	delete(dep.Annotations, finopsv1alpha1.AnnotationAppliedReplicas)
	delete(dep.Annotations, finopsv1alpha1.AnnotationOwnedBy)
	return r.Patch(ctx, dep, patch)
}

// podRequests totals the resource requests of one pod of this Deployment.
func podRequests(dep *appsv1.Deployment) corev1.ResourceList {
	total := corev1.ResourceList{}
	for _, c := range dep.Spec.Template.Spec.Containers {
		for name, q := range c.Resources.Requests {
			if existing, ok := total[name]; ok {
				existing.Add(q)
				total[name] = existing
			} else {
				total[name] = q.DeepCopy()
			}
		}
	}
	return total
}

// addReclaimed adds perPod × count into total.
func addReclaimed(total, perPod corev1.ResourceList, count int32) {
	if count <= 0 {
		return
	}
	for name, q := range perPod {
		scaled := q.DeepCopy()
		scaled.Set(q.Value() * int64(count))
		if name == corev1.ResourceCPU {
			scaled = *resource.NewMilliQuantity(q.MilliValue()*int64(count), q.Format)
		}
		if existing, ok := total[name]; ok {
			existing.Add(scaled)
			total[name] = existing
		} else {
			total[name] = scaled
		}
	}
}

func (r *IdleWindowReconciler) writeStatus(ctx context.Context, w *finopsv1alpha1.IdleWindow) error {
	err := r.Status().Update(ctx, w)
	// A conflict means someone else wrote first; the next reconcile reads the
	// fresh object and recomputes. Nothing is lost by dropping this write.
	return client.IgnoreNotFound(ignoreConflict(err))
}

// pdbsBlockingDrain finds PodDisruptionBudgets that currently permit no
// disruption at all.
//
// A PDB never stops this controller from writing a replica count — it stops an
// eviction. That matters anyway: a node cannot be removed while a pod on it
// refuses to be evicted, and node removal is the part that saves money. The
// common shape is one replica with minAvailable one, which allows zero
// disruptions forever.
//
// status.disruptionsAllowed is read rather than inferred from the spec because
// it is the number the eviction API actually enforces.
func (r *IdleWindowReconciler) pdbsBlockingDrain(ctx context.Context, ns string) (int32, []string, error) {
	var list policyv1.PodDisruptionBudgetList
	if err := r.List(ctx, &list, client.InNamespace(ns)); err != nil {
		return 0, nil, err
	}

	var count int32
	var names []string
	for i := range list.Items {
		pdb := &list.Items[i]
		// A budget that currently guards no pods blocks nothing, however
		// strict it looks. Once the workload is already at zero, the same
		// disruptionsAllowed of zero means "nothing to disrupt" rather than
		// "refuses disruption", and reporting it would be a false alarm
		// appearing exactly when the reclaim succeeded.
		if pdb.Status.DisruptionsAllowed == 0 && pdb.Status.CurrentHealthy > 0 {
			count++
			names = append(names, pdb.Name+" (PDB)")
		}
	}
	return count, names, nil
}

// setUnblocked states whether anything is standing between this namespace and
// a fully reclaimed one.
func (r *IdleWindowReconciler) setUnblocked(w *finopsv1alpha1.IdleWindow, workloadBlockers, pdbBlockers []string) {
	all := append(append([]string{}, workloadBlockers...), pdbBlockers...)
	if len(all) == 0 {
		meta.SetStatusCondition(&w.Status.Conditions, metav1.Condition{
			Type:               finopsv1alpha1.ConditionUnblocked,
			Status:             metav1.ConditionTrue,
			Reason:             "NothingBlocking",
			Message:            "no workload or PodDisruptionBudget prevents reclaiming this namespace",
			ObservedGeneration: w.Generation,
		})
		return
	}
	meta.SetStatusCondition(&w.Status.Conditions, metav1.Condition{
		Type:               finopsv1alpha1.ConditionUnblocked,
		Status:             metav1.ConditionFalse,
		Reason:             "ReclaimBlocked",
		Message:            "not fully reclaimable: " + strings.Join(all, ", "),
		ObservedGeneration: w.Generation,
	})
}

func (r *IdleWindowReconciler) setReady(w *finopsv1alpha1.IdleWindow, status metav1.ConditionStatus, reason, msg string) {
	meta.SetStatusCondition(&w.Status.Conditions, metav1.Condition{
		Type:               finopsv1alpha1.ConditionReady,
		Status:             status,
		Reason:             reason,
		Message:            msg,
		ObservedGeneration: w.Generation,
	})
}

func (r *IdleWindowReconciler) event(w *finopsv1alpha1.IdleWindow, kind, reason, msg string) {
	if r.Recorder != nil {
		r.Recorder.Event(w, kind, reason, msg)
	}
}

func (r *IdleWindowReconciler) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func annotationInt(dep *appsv1.Deployment, key string) (int32, bool) {
	raw, ok := dep.Annotations[key]
	if !ok {
		return 0, false
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return int32(v), true
}

func ptrBool(b *bool) bool { return b != nil && *b }

func ignoreConflict(err error) error {
	if errors.IsConflict(err) {
		return nil
	}
	return err
}

// windowsForDeployment maps a changed Deployment back to the IdleWindows that
// select it.
//
// Without this the controller only wakes on its own timer, so a workload
// scaled by hand stays undetected until the next scheduled pass — up to
// requeueAfter's cap. Correcting drift is the reason to run a controller at
// all, and drift noticed ten minutes late is a poor version of it.
func (r *IdleWindowReconciler) windowsForDeployment(ctx context.Context, obj client.Object) []reconcile.Request {
	dep, ok := obj.(*appsv1.Deployment)
	if !ok {
		return nil
	}

	var windows finopsv1alpha1.IdleWindowList
	if err := r.List(ctx, &windows, client.InNamespace(dep.Namespace)); err != nil {
		return nil
	}

	var reqs []reconcile.Request
	for i := range windows.Items {
		w := &windows.Items[i]
		if w.Spec.Selector != nil {
			sel, err := metav1.LabelSelectorAsSelector(w.Spec.Selector)
			if err != nil || !sel.Matches(labels.Set(dep.Labels)) {
				continue
			}
		}
		reqs = append(reqs, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(w),
		})
	}
	return reqs
}

// SetupWithManager registers the controller with the manager.
func (r *IdleWindowReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor("idle-reaper")
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&finopsv1alpha1.IdleWindow{}).
		Watches(&appsv1.Deployment{}, handler.EnqueueRequestsFromMapFunc(r.windowsForDeployment)).
		Named("idlewindow").
		Complete(r)
}
