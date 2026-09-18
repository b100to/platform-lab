#!/usr/bin/env bash
# Reproduce how nodeTaintsPolicy changes hard topology spread, on the kind lab.
#
#   ./scripts/topology-spread-repro.sh
#
# Runs three checks against Deployments that differ only in nodeTaintsPolicy
# (unset = Ignore, vs Honor), all with a hard hostname constraint:
#
#   A    3 replicas on 2 schedulable nodes. Tainted nodes count as empty
#        domains under Ignore, so the third pod stays Pending.
#   B-1  2 replicas, then one node becomes unschedulable and loses its pods.
#        The taint stands in for what the scheduler sees on a NotReady node.
#        Under Ignore the replacement stays Pending; under Honor it lands on
#        the surviving node.
#   B-2  The node comes back. Nothing moves the Honor pods apart again, which
#        is the gap a descheduler fills.
#
# Everything lives in a throwaway namespace. The taint and the namespace are
# removed on exit, including on failure.
set -uo pipefail

CONTEXT="${CONTEXT:-kind-lab}"
NS="${NS:-tsc-repro}"
NODE="${NODE:-lab-worker3}"
IMAGE="${IMAGE:-registry.k8s.io/pause:3.10}"
TAINT="repro/down"

kc() { kubectl --context "$CONTEXT" "$@"; }

cleanup() {
  kc taint node "$NODE" "${TAINT}-" >/dev/null 2>&1
  kc delete ns "$NS" --wait=false >/dev/null 2>&1
}
trap cleanup EXIT

deployment() { # name replicas [extra constraint line]
  cat <<EOF
apiVersion: apps/v1
kind: Deployment
metadata: {name: $1, namespace: $NS}
spec:
  replicas: $2
  selector: {matchLabels: {app: $1}}
  template:
    metadata: {labels: {app: $1}}
    spec:
      terminationGracePeriodSeconds: 0
      containers:
        - {name: pause, image: $IMAGE, imagePullPolicy: IfNotPresent}
      topologySpreadConstraints:
        - maxSkew: 1
          topologyKey: kubernetes.io/hostname
          whenUnsatisfiable: DoNotSchedule
${3:-}
          labelSelector: {matchLabels: {app: $1}}
          matchLabelKeys: [pod-template-hash]
EOF
}

HONOR="          nodeTaintsPolicy: Honor"

pods() {
  kc -n "$NS" get pods --sort-by=.metadata.name \
    -o custom-columns='POD:.metadata.name,STATUS:.status.phase,NODE:.spec.nodeName'
}

kc create ns "$NS" >/dev/null

echo "=== A. 3 replicas, hard hostname spread"
{ deployment ignore 3; echo ---; deployment honor 3 "$HONOR"; } | kc apply -f - >/dev/null
kc -n "$NS" wait --for=condition=Available deploy/honor --timeout=90s >/dev/null
sleep 5
pods
kc -n "$NS" get events --field-selector reason=FailedScheduling \
  -o custom-columns='MSG:.message' --no-headers | tail -1 | cut -c1-160

echo
echo "=== B-0. 2 replicas, before"
kc -n "$NS" delete deploy ignore honor --wait=true >/dev/null
{ deployment ignore 2; echo ---; deployment honor 2 "$HONOR"; } | kc apply -f - >/dev/null
kc -n "$NS" wait --for=condition=Available deploy/ignore deploy/honor --timeout=90s >/dev/null
pods

echo
echo "=== B-1. $NODE becomes unschedulable and loses its pods"
kc taint node "$NODE" "${TAINT}=true:NoSchedule" >/dev/null
kc -n "$NS" delete pod --field-selector "spec.nodeName=$NODE" --wait=true >/dev/null
sleep 8
pods

echo
echo "=== B-2. $NODE comes back"
kc taint node "$NODE" "${TAINT}-" >/dev/null
sleep 8
pods
