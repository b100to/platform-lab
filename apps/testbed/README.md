# testbed

Fixtures for exercising `operators/idle-reaper`. Each one exists to make a
specific behaviour observable — or to prove a behaviour does *not* happen.

| Workload | Replicas | Selected | Proves |
|---|---|---|---|
| `lab-dev/api` | 3 | yes | scale down, and restore to **3** |
| `lab-dev/worker` | 2 | yes | restore to **2**, not to some shared default |
| `lab-dev/cache` | 1 | yes | restore to **1** |
| `lab-dev/autoscaled` | 2 | yes | skipped because an HPA owns its replicas (D4) |
| `lab-dev/untagged` | 1 | no | the selector is applied — the namespace is not swept |
| `lab-prod/api` | 2 | no | namespace boundary holds despite identical labels |

The three different replica counts are the point. A controller that restores
every workload to the same number passes a single-workload test and fails here.

## Expected reclaim

When `lab-dev` is asleep, the requests released by the three scalable
workloads are:

```
cpu    3×200m + 2×100m + 1×50m  =  850m
memory 3×256Mi + 2×128Mi + 1×64Mi = 1088Mi
```

`autoscaled` and `untagged` are excluded, so their requests must **not** appear
in `status.reclaimed`.

## Use

```bash
kubectl apply -k apps/testbed
kubectl get deploy -n lab-dev
```

The HPA has no metrics-server behind it in kind. That is fine: the controller
only checks whether an HPA targets the Deployment, not what it reports.
