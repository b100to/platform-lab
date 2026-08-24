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
| `operators/` | Controllers built here |
| `apps/demo/` | Sample workload to test against |
| `scripts/probe.sh` | Availability probe — reports error rate and longest outage |
| `platform/` | Platform components |

## Conventions

- No secrets in this repo. `gitleaks` runs as a pre-commit hook:
  ```bash
  git config core.hooksPath .githooks
  make scan
  ```
- Nothing here is derived from any employer's environment.
