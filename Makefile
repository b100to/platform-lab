CLUSTER ?= lab
CONFIG  ?= clusters/kind/kind-config.yaml

.PHONY: help up down status nodes kill-node revive-node scan

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
