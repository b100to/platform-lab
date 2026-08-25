# platform-lab

Local Kubernetes workshop — a 4-node [kind](https://kind.sigs.k8s.io) cluster
plus whatever I am currently building or testing against it.

Not a curated portfolio. Things land here when I need somewhere to run them.

## Cluster

1 control-plane + 3 workers. The workers carry `topology.kubernetes.io/zone`
labels (`zone-a/b/c`) so placement and spread behaviour can be exercised
without a cloud account.

```bash
make up                            # create
make status                        # nodes + pods
make nodes                         # containers backing each node
make kill-node NODE=lab-worker2    # stop a node
make revive-node NODE=lab-worker2  # start it again
make down                          # tear down
```

Host ports **18080 / 18443** map to NodePort 30080 / 30443 on the
control-plane node (8080/8443 were already taken locally).

## What's here

| Path | What |
|------|------|
| `clusters/kind/` | Cluster topology |
| [`operators/idle-reaper/`](operators/idle-reaper) | An operator that scales an idle namespace down and reports what stops the cluster from shrinking |
| `apps/demo/` | Sample workload to test against |
| `scripts/probe.sh` | Availability probe — reports error rate and longest outage |
| `platform/` | Install-time values for the components above |

## idle-reaper

The one thing here that is more than an experiment. It declares when a
namespace is idle, scales its workloads down while it is, and reports which
workload or PodDisruptionBudget is keeping a node from being removed.

```sh
make lab-install     # CRD and test workloads
make lab-deploy      # build the image, load it, install the chart
make lab-sleep       # place the current moment inside an idle window
make lab-status
make lab-wake
```

See [operators/idle-reaper](operators/idle-reaper) for what it does and
[its design notes](operators/idle-reaper/DESIGN.md) for why.

## Conventions

- No secrets in this repo. `gitleaks` runs as a pre-commit hook:
  ```bash
  git config core.hooksPath .githooks
  make scan
  ```
- Nothing here is derived from any employer's environment.
