# idle-reaper

Declares when a namespace is idle, scales its workloads down while it is, and
reports which workload or PodDisruptionBudget is keeping a node from being
removed — because emptying pods on a node that stays running saves nothing.

```yaml
apiVersion: finops.b100to.dev/v1alpha1
kind: IdleWindow
metadata:
  name: dev-nights
  namespace: team-a
spec:
  sleepAt: "0 20 * * 1-5"
  wakeAt: "0 9 * * 1-5"
```

```
$ kubectl get idlewindow -n team-a
NAME         PHASE    SCALED   SKIPPED   CPU    DRAINABLE   NODES
dev-nights   Asleep   4        1         900m   1           2

Unblocked=False   not fully reclaimable: autoscaled (HPA), cache (PDB)
```

## What a window covers

- **Its own namespace, and only that.** `IdleWindow` is namespace-scoped, so
  each team writes its own with its own hours.
- **Deployments.** StatefulSets are left alone — scaling storage-backed
  workloads to zero is a different risk. DaemonSets and CronJobs are untouched.
- **All of them, unless narrowed.** Omitting `selector` covers the whole
  namespace. A selector carves out less; it is not how you opt in.

Workloads whose replicas an HPA already owns, and workloads somebody scaled by
hand during the window, are skipped — and reported rather than silently passed
over.

## Install

```sh
helm install idle-reaper oci://ghcr.io/b100to/charts/idle-reaper \
  --namespace idle-reaper-system --create-namespace
```

The chart installs the CRDs and keeps them on uninstall, so removing the
operator never deletes the windows people wrote.

## Two settings that matter on a real cluster

```yaml
manager:
  args:
    # Only this capacity counts toward drainableNodes. Without it, nodes that
    # have to stay up are included and the fraction can never be filled.
    - --reclaimable-node-selector=node-role/app

  # The operator must not run on a node it is emptying: scaling that capacity
  # to zero would take the controller with it, and nothing would be left to
  # wake anything up. A separate node group, a tainted infra node, or Fargate
  # all solve this.
  nodeSelector: { node-role/infra: "" }
  tolerations:
    - { key: dedicated, value: infra, effect: NoSchedule }
```

## Values

| Key | Default | What |
|---|---|---|
| `manager.replicas` | `1` | Controller replicas |
| `manager.image.repository` | `ghcr.io/b100to/idle-reaper` | Image to run |
| `manager.args` | `["--leader-elect"]` | Extra flags, appended to metrics and health args |
| `manager.nodeSelector` | `{}` | Where the controller runs |
| `manager.tolerations` | `[]` | Taints it may run on |
| `manager.resources` | 10m / 64Mi requests | Requests and limits |
| `rbac.namespaced` | `false` | Role instead of ClusterRole. Node counting needs cluster scope |
| `crd.enabled` | `true` | Install the CRDs |
| `crd.keep` | `true` | Keep CRDs on uninstall |
| `metrics.enabled` | `true` | Serve `/metrics` |
| `metrics.secure` | `true` | HTTPS with authn/authz rather than plain HTTP |
| `prometheus.enabled` | `false` | Create a ServiceMonitor |
| `networkPolicy.enabled` | `false` | Restrict ingress to the controller |

## What it does not do

- **Remove nodes.** Emptying workloads is where this stops; taking the node
  away belongs to a node autoscaler such as Karpenter or Cluster Autoscaler.
  This reports how many became removable so the two do not fight over the
  same node.
- **Touch StatefulSets.** Scaling storage-backed workloads to zero is a
  different risk, and out of scope for `v1alpha1`.
- **Route alerts.** State is exposed as conditions, Events and metrics.
- **Cover several namespaces from one object.** A cluster-scoped kind is the
  obvious next step; what it needs first is a rule for workloads covered by
  both it and their own namespace's window.

## Lifting a window

Someone who has to work during an idle window asks for an exception rather
than being scaled up by hand — and nobody has to remember to undo it:

```yaml
apiVersion: finops.b100to.dev/v1alpha1
kind: WakeRequest
metadata:
  namespace: team-a
spec:
  duration: 3h
  reason: "verifying a payment hotfix"
```

The window reports `WakeRequested` — awake, and why it is awake off-schedule —
and goes back to sleep when the last request lapses. Requests longer than the
window's `maxWakeDuration` (default `8h`) are refused, so a front end raising
requests on someone's behalf cannot widen the policy.

## Metrics

```
idlereaper_window_asleep{namespace,window}
idlereaper_scaled_workloads{namespace,window}
idlereaper_blocked_workloads{namespace,window}
idlereaper_blocking_pdbs{namespace,window}
idlereaper_reclaimed_cpu_millicores{namespace,window}
idlereaper_reclaimed_memory_bytes{namespace,window}
idlereaper_drainable_nodes
idlereaper_worker_nodes
```

Node counts carry no window label: they are a property of the cluster, and
labelling them per window would double-count in a `sum()`.

## Source

[github.com/b100to/platform-lab](https://github.com/b100to/platform-lab/tree/main/operators/idle-reaper)
— including the design notes and the decisions that were wrong the first time.
