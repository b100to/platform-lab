# platform-lab

A reproducible Kubernetes lab for practising the parts of platform work that are
hard to rehearse on a production cluster: node failure, recovery, deployment
safety, and runtime policy.

Everything runs locally on a 4-node [kind](https://kind.sigs.k8s.io) cluster
(1 control-plane, 3 workers labelled as separate zones) so that failure
scenarios can be triggered on purpose and measured.

## Why

Cost-optimised clusters run close to the edge: fewer nodes, tighter requests,
less headroom. That trade-off is only defensible if recovery behaviour is known
rather than assumed. This lab exists to turn assumptions into measured numbers.

## Quickstart

```bash
make up        # create the cluster
make status    # nodes + pods
make nodes     # containers backing each node

make kill-node NODE=lab-worker2    # simulate node failure
make revive-node NODE=lab-worker2  # bring it back

make down      # tear down
```

## Layout

| Path | What |
|------|------|
| `clusters/kind/` | Cluster topology |
| `platform/` | Platform components (GitOps, policy, security) |
| `apps/` | Sample workloads used as failure targets |
| `runbooks/` | Recovery procedures — written before the drill, corrected after |
| `gamedays/` | Drill records: scenario, timeline, measured recovery, findings |

The runbooks and gameday records are the point. The manifests just make them
reproducible.

## Roadmap

- [ ] **Failure drills** — node loss, zone loss, pod eviction. Measure recovery, write runbooks
- [ ] **Deployment safety** — progressive delivery with automated rollback gates
- [ ] **Policy guardrails** — admission policies enforcing what should never reach a cluster
- [ ] **Runtime security** — container-runtime detection alongside API-layer auditing

## Conventions

- No secrets in this repo. `gitleaks` runs as a pre-commit hook:
  ```bash
  git config core.hooksPath .githooks
  make scan
  ```
- Nothing here is derived from any employer's environment.
