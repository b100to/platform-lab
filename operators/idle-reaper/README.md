# idle-reaper

A Kubernetes operator that declares when a namespace is idle, scales its
workloads down while it is, and reports what stopped the cluster from
shrinking any further.

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

Three lines. Everything else — timezone, minimum replicas, how to treat an
HPA, whether to respect a manual scale — has a default that errs toward doing
nothing surprising.

## What it looks like running

```
$ kubectl get idlewindow -n team-a
NAME         PHASE    SCALED   SKIPPED   CPU    DRAINABLE   NODES   NEXT
dev-nights   Asleep   4        1         900m   1           2       2026-08-24T11:00:00Z

$ kubectl describe idlewindow dev-nights -n team-a
Status:
  Conditions:
    Type:     Unblocked
    Status:   False
    Reason:   ReclaimBlocked
    Message:  not fully reclaimable: autoscaled (HPA), cache (PDB)
```

The last line is the point. Four workloads shrank, one did not, and one node
still cannot be removed — and the object says which and why, rather than
leaving a counter to be interpreted.

## Why a controller and not a scheduled job

Scaling workloads on a timer is a cron job's worth of work. The reasons to
spend a controller on it are specific:

**It reads state, not events.** Every pass compares the clock against the
declaration and the cluster against both. A missed event, a restarted
controller and a duplicate call all produce the same result, because none of
them are inputs. A job that fired at 20:00 and failed has simply not run.

**The original size lives with the workload.** Replica counts are recorded as
annotations on the Deployments themselves, so deleting the IdleWindow — or
losing the operator entirely — never strands a workload at zero with no record
of where it came from.

**It can tell the difference between quiet and broken.** A cron job reports
that it ran. This reports that four workloads shrank, one was left alone
because an HPA owns its replicas, and one PodDisruptionBudget will refuse to
let a node drain. Those are the facts that decide whether the saving is real.

**Someone scaling up at midnight wins.** If a replica count no longer matches
what the operator last wrote, that is a person acting deliberately, and the
window stands down until the next boundary rather than overriding them.

## Working during a window

Someone who needs the namespace back before the window ends does not have to
be scaled up by hand, and nobody has to remember to put it away again:

```yaml
kind: WakeRequest
spec:
  duration: 3h
  reason: "verifying a payment hotfix"
```

The window then reports `WakeRequested` rather than `Awake` — awake, and the
reason why it is awake off-schedule — and goes back to sleep when the last
request lapses.

The split is deliberate. `IdleWindow` is policy and belongs to whoever runs the
platform; a request to step around it belongs to whoever needs to work
tonight. RBAC can then say exactly that: `create` on `wakerequests`, nothing on
`idlewindows`. The cap lives on the policy object, so a request over
`maxWakeDuration` is refused no matter what raised it.

[`tools/slack-waker`](../../tools/slack-waker) is one such front end.

## What it will not do

- **Remove nodes.** Emptying workloads is where this stops; taking the node
  away belongs to a node autoscaler. Two controllers deciding the fate of the
  same node is a bad trade for a number this one can simply report.
- **Touch StatefulSets.** Scaling something with attached storage to zero is a
  different risk, and out of scope for `v1alpha1`.
- **Route alerts.** State is exposed as conditions, Events and metrics.
  Deciding who hears about it is the monitoring stack's job, and an operator
  holding webhook credentials is one that cannot be handed to anyone else.
- **Protect production by name.** Namespace names are a weak guard. RBAC and
  deployment policy are the right place for that.

## Measured, and modelled

`status.reclaimed` is measured: the resource requests currently not scheduled
because of this window.

Money is not. Converting requests into a bill needs pricing the cluster does
not have, and a node autoscaler to actually remove the emptied nodes.
[`DESIGN.md`](DESIGN.md) carries a worked model — a 20-service dev cluster on
six `m5.xlarge` nodes, reclaiming 67% of the week — and it stays labelled as a
model, including the sensitivity of its most load-bearing assumption.

## Install

```sh
helm install idle-reaper oci://ghcr.io/b100to/charts/idle-reaper \
  --version 0.1.1 \
  --namespace idle-reaper-system --create-namespace
```

The chart carries the CRD and keeps it on uninstall, so removing the operator
never deletes the windows people wrote.

Two settings matter on a real cluster:

```yaml
manager:
  args:
    # Only this capacity is reported as reclaimable.
    - --reclaimable-node-selector=node-role/app
  # The operator must not run on a node it is emptying.
  nodeSelector: { node-role/infra: "" }
  tolerations:
    - { key: dedicated, value: infra, effect: NoSchedule }
```

An operator scheduled onto the capacity it reclaims will scale that capacity to
zero and take itself with it. Fargate, a small managed node group, or a tainted
infra node all solve this; doing nothing does not.

## Try it locally

From the repository root, against the 4-node kind cluster in
[`clusters/kind`](../../clusters/kind):

```sh
make lab-install     # CRD and a set of test workloads
make lab-deploy      # build the image, load it, install the chart
make lab-sleep       # place the current moment inside an idle window
make lab-status      # workloads, conditions, and what holds each node
make lab-wake
```

Waiting until 20:00 to watch a night window work is not a demo, so `lab-sleep`
builds the schedule around the current hour instead.

## Known limitations

- A workload with an HPA is skipped by default. `hpaPolicy: Scale` overrides
  this, at the cost of two controllers writing the same field.
- Manual-scale detection cannot distinguish a person from another controller.
  A GitOps sync during a window reads as manual intervention.
- Node counts are a cluster-wide observation reported by a namespaced object.
  Two IdleWindows report the same figure.

## Design

[`DESIGN.md`](DESIGN.md) records the decisions, what each one cost, and the
ones that were wrong the first time.
