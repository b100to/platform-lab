# testbed

Fixtures for exercising `operators/idle-reaper`. Each one exists to make a
specific behaviour observable — or to prove a behaviour does *not* happen.

| Workload | Replicas | Selected | Proves |
|---|---|---|---|
| `lab-dev/api` | 3 | yes | scale down, and restore to **3** |
| `lab-dev/worker` | 2 | yes | restore to **2**, not to some shared default |
| `lab-dev/cache` | 1 | yes | restore to **1** |
| `lab-dev/autoscaled` | 2 | yes | carries an HPA — skipped while `skipIfHPA` is on (D4) |
| `lab-dev/untagged` | 1 | yes | different labels, so a selector can exclude it

The three different replica counts are the point. A controller that restores
every workload to the same number passes a single-workload test and fails here.

## Expected reclaim

With no selector and `skipIfHPA: false`, the whole namespace goes to zero:

```
cpu    3×200m + 2×100m + 1×50m + 2×50m + 1×50m  =  1000m
memory 3×256Mi + 2×128Mi + 1×64Mi + 2×64Mi + 1×64Mi = 1280Mi
```

Narrowing with `selector: {app: demo}` and `skipIfHPA: true` leaves only
api/worker/cache, which reclaims 850m / 1088Mi instead. Both numbers are worth
checking against `status.reclaimed`.

## Use

```bash
kubectl apply -k apps/testbed
kubectl get deploy -n lab-dev
```

The HPA has no metrics-server behind it in kind. That is fine: the controller
only checks whether an HPA targets the Deployment, not what it reports.
