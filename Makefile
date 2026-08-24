CLUSTER ?= lab
CONFIG  ?= clusters/kind/kind-config.yaml

# idle-reaper demo settings
NS       ?= lab-dev
WINDOW   ?= dev-nights
OPERATOR ?= operators/idle-reaper
METRICS  ?= 18090
HEALTH   ?= 18091
IMG      ?= idle-reaper:dev
OPERATOR_NS ?= idle-reaper-system

.PHONY: help up down status nodes kill-node revive-node scan \
        lab-install lab-run lab-sleep lab-wake lab-status lab-metrics lab-reset \
        lab-image lab-deploy lab-undeploy

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

up: ## Create the kind cluster
	kind create cluster --name $(CLUSTER) --config $(CONFIG)
	kubectl cluster-info --context kind-$(CLUSTER)

down: ## Delete the kind cluster
	kind delete cluster --name $(CLUSTER)

status: ## Show nodes and pods
	kubectl get nodes -o wide
	kubectl get pods -A

nodes: ## List the containers backing each node
	docker ps --filter "name=$(CLUSTER)-" --format "table {{.Names}}\t{{.Status}}"

kill-node: ## Simulate node failure. usage: make kill-node NODE=lab-worker2
	@test -n "$(NODE)" || (echo "NODE is required, e.g. make kill-node NODE=$(CLUSTER)-worker2"; exit 1)
	docker stop $(NODE)
	@echo "stopped $(NODE) -- watch: kubectl get nodes -w"

revive-node: ## Bring a stopped node back. usage: make revive-node NODE=lab-worker2
	@test -n "$(NODE)" || (echo "NODE is required, e.g. make revive-node NODE=$(CLUSTER)-worker2"; exit 1)
	docker start $(NODE)

scan: ## Scan the repo for leaked secrets
	gitleaks detect --no-banner --redact

# --- idle-reaper -------------------------------------------------------------
# Waiting for 20:00 to see a night window work is not a demo. These targets
# build the schedule around the current hour instead: sleeping means "the last
# boundary was a sleep", so putting sleepAt an hour behind and wakeAt an hour
# ahead lands the clock inside the window right now. Waking is the same two
# values swapped.

lab-install: ## Install the CRD and the test workloads
	$(MAKE) -C $(OPERATOR) install
	kubectl apply -k apps/testbed

lab-run: ## Run the controller against the cluster (foreground)
	@# Ports are moved off the defaults so a stale controller from an earlier
	@# run fails loudly on start instead of quietly holding 8081.
	cd $(OPERATOR) && go run ./cmd/main.go \
		--metrics-bind-address=:$(METRICS) --metrics-secure=false \
		--health-probe-bind-address=:$(HEALTH) \
		--reclaimable-node-selector='platform-lab.dev/role=app'

lab-sleep: ## Put $(NS) inside an idle window starting now
	@H=$$(date +%H); \
	printf '%s\n' \
	  'apiVersion: finops.b100to.dev/v1alpha1' \
	  'kind: IdleWindow' \
	  'metadata:' \
	  '  name: $(WINDOW)' \
	  '  namespace: $(NS)' \
	  'spec:' \
	  "  sleepAt: \"0 $$(( (10#$$H + 23) % 24 )) * * *\"" \
	  "  wakeAt: \"0 $$(( (10#$$H + 1) % 24 )) * * *\"" \
	  | kubectl apply -f -
	@echo "asleep -- watch: kubectl get iw -n $(NS) -w"

lab-wake: ## Move $(NS) outside the idle window
	@H=$$(date +%H); \
	kubectl patch idlewindow $(WINDOW) -n $(NS) --type=merge -p \
		"{\"spec\":{\"sleepAt\":\"0 $$(( (10#$$H + 1) % 24 )) * * *\",\"wakeAt\":\"0 $$(( (10#$$H + 23) % 24 )) * * *\"}}"
	@echo "awake -- workloads should return to their original replica counts"

lab-status: ## Show the window, the workloads, and what holds each node
	@kubectl get idlewindow -n $(NS) 2>/dev/null || echo "no IdleWindow in $(NS)"
	@echo
	@kubectl get deploy -n $(NS) --no-headers | awk '{printf "  %-12s %s\n", $$1, $$2}'
	@echo
	@kubectl get idlewindow $(WINDOW) -n $(NS) \
		-o jsonpath='{range .status.conditions[*]}  {.type}={.status}  {.message}{"\n"}{end}' 2>/dev/null || true
	@echo
	@for n in $$(kubectl get nodes -l platform-lab.dev/role -o name | cut -d/ -f2); do \
		role=$$(kubectl get node $$n -o jsonpath='{.metadata.labels.platform-lab\.dev/role}'); \
		pods=$$(kubectl get pods -A --field-selector spec.nodeName=$$n --no-headers 2>/dev/null \
			| awk '{print $$2}' | sed 's/-[a-z0-9]*$$//' | sort -u | tr '\n' ' '); \
		printf "  %-14s [%-5s] %s\n" "$$n" "$$role" "$$pods"; \
	done

lab-metrics: ## Scrape the controller's own metrics
	@curl -s --max-time 5 http://localhost:$(METRICS)/metrics | grep -E '^idlereaper' | sort \
		|| echo "no metrics -- is 'make lab-run' running?"

lab-image: ## Build the controller image and load it into the kind nodes
	cd $(OPERATOR) && docker build -t $(IMG) .
	kind load docker-image $(IMG) --name $(CLUSTER)

lab-deploy: lab-image ## Install the operator into the cluster with Helm
	helm upgrade --install idle-reaper $(OPERATOR)/dist/chart \
		-n $(OPERATOR_NS) --create-namespace \
		-f platform/idle-reaper/values-lab.yaml
	kubectl rollout status -n $(OPERATOR_NS) deploy/idle-reaper-controller-manager --timeout=120s

lab-undeploy: ## Remove the operator (the CRD is kept)
	-helm uninstall idle-reaper -n $(OPERATOR_NS)

lab-reset: ## Delete the window and put every workload back
	-kubectl delete idlewindow --all -n $(NS) --ignore-not-found
	@sleep 2
	@for d in api:3 worker:2 cache:1 autoscaled:2 untagged:1; do \
		n=$${d%%:*}; r=$${d##*:}; \
		kubectl annotate deploy $$n -n $(NS) \
			finops.b100to.dev/saved-replicas- \
			finops.b100to.dev/applied-replicas- \
			finops.b100to.dev/owned-by- >/dev/null 2>&1 || true; \
		kubectl scale deploy $$n -n $(NS) --replicas=$$r >/dev/null; \
	done
	@echo "reset -- annotations cleared, replicas restored"
