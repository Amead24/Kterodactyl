# Phase 15: Kind Cluster Environment - Research

**Researched:** 2026-02-19
**Domain:** Kubernetes local development environment (kind + Helm + Makefile orchestration)
**Confidence:** HIGH

## Summary

Phase 15 delivers two Makefile targets (`test-e2e-setup` and `test-e2e-teardown`) that give developers a one-command workflow to spin up and tear down a complete Kterodactyl environment in kind. The environment must be accessible at `localhost:8080` after setup completes.

The project already has significant scaffolding for this: a `chart/` directory with a complete Helm chart (CRDs, RBAC, Deployment, Service, ConfigMap), an existing `setup-test-e2e` Makefile target that creates a bare kind cluster, and a `docker-build` target that builds the operator image. What is missing is: (1) a kind cluster config with `extraPortMappings` for NodePort access, (2) Helm values overrides for the CI/test environment, (3) the Service template's `nodePort` field support, (4) a readiness wait mechanism, and (5) Makefile targets that orchestrate the full lifecycle (build image, create cluster, load image, helm install, wait for readiness).

**Primary recommendation:** Create `hack/kind-config.yaml` with `extraPortMappings` (containerPort 30080 -> hostPort 8080), add `hack/ci-values.yaml` with NodePort + image overrides, update the Helm service template to support `nodePort`, write a `hack/wait-for-ready.sh` script, and wire everything into `test-e2e-setup` / `test-e2e-teardown` Makefile targets.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| INFRA-01 | Developer can create a kind cluster with Helm-deployed Kterodactyl via a single Makefile target | `make test-e2e-setup` target chains: kind create cluster (with config), docker build, kind load docker-image, helm install (with CI values), wait-for-ready. All pieces exist individually; this phase wires them together. |
| INFRA-02 | Developer can tear down the test environment via a single Makefile target | `make test-e2e-teardown` target runs `kind delete cluster --name $KIND_CLUSTER`. kind delete is idempotent (exits 0 even if cluster doesn't exist). Helm release is automatically destroyed when the cluster is deleted. |
</phase_requirements>

## Standard Stack

### Core

| Tool | Version | Purpose | Why Standard |
|------|---------|---------|--------------|
| kind | v0.31.0 | Local K8s cluster in Docker containers | Already referenced in Makefile as `KIND` variable. Standard for Kubebuilder projects. Idempotent create/delete. Docker-native, CI-friendly. |
| kindest/node | v1.32.11 | K8s node image matching production K8s v1.32.3 | Production Talos cluster runs K8s v1.32.3. Testing against same minor version ensures API compatibility. Pin to SHA256 digest for reproducibility. |
| Helm | v3.x (latest) | Chart-based deployment | The project has a complete Helm chart in `chart/`. Helm install automatically applies CRDs from `chart/crds/`, then renders and applies templates. |
| Docker | (system) | Container image building | Already used via `$(CONTAINER_TOOL)` in Makefile. Required by kind (kind nodes are Docker containers). |

### Supporting

| Tool | Version | Purpose | When to Use |
|------|---------|---------|-------------|
| kubectl | (system) | Cluster interaction, readiness checks | Used by `wait-for-ready.sh` to check deployment/pod status. Already defined as `KUBECTL` in Makefile. |
| curl | (system) | HTTP health check | Final readiness gate: verify `localhost:8080/healthz` returns 200 before declaring setup complete. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| NodePort + extraPortMappings | kubectl port-forward | port-forward is fragile in CI (process can die, race conditions). NodePort is deterministic and survives the test suite duration. Prior decision: NodePort chosen. |
| kind | k3d/minikube | kind is already scaffolded by Kubebuilder, integrated into Makefile, and standard for CI. Switching provides no benefit. |
| Helm install | kustomize deploy (existing `make deploy`) | Helm matches production deployment. Tests should exercise the same installation path users follow. The existing `make deploy` uses kustomize, which diverges from the Helm chart. |
| Shell script orchestration | Go test BeforeSuite | Shell script in Makefile is simpler, more debuggable, and decoupled from test framework. The existing Ginkgo BeforeSuite uses `make deploy` but Phase 15 targets are test-framework-agnostic. |

**Installation (if kind is not already installed):**
```bash
# Download binary (CI approach)
curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.31.0/kind-linux-amd64
chmod +x ./kind
sudo mv ./kind /usr/local/bin/kind

# Or via go install (developer approach)
go install sigs.k8s.io/kind@v0.31.0
```

## Architecture Patterns

### Recommended File Structure

```
hack/
  kind-config.yaml       # kind cluster configuration with extraPortMappings
  ci-values.yaml         # Helm values overrides for test/CI environment
  wait-for-ready.sh      # Readiness gate script (kubectl wait + curl health check)
chart/
  templates/
    service.yaml         # MODIFIED: add conditional nodePort field
  values.yaml            # MODIFIED: add apiService.nodePort field
Makefile                 # MODIFIED: add test-e2e-setup and test-e2e-teardown targets
```

### Pattern 1: Kind Cluster Config with extraPortMappings

**What:** A YAML config file that tells kind to map a NodePort (30080) inside the cluster to a host port (8080) on the developer's machine.

**When to use:** Any time you need deterministic port access to services running inside a kind cluster without relying on `kubectl port-forward`.

**Critical constraint:** The kind node `containerPort` MUST equal the Kubernetes service `nodePort`. The `hostPort` is what the developer accesses (localhost:8080).

**Example:**
```yaml
# hack/kind-config.yaml
# Source: https://kind.sigs.k8s.io/docs/user/configuration/
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  image: kindest/node:v1.32.11@sha256:5fc52d52a7b9574015299724bd68f183702956aa4a2116ae75a63cb574b35af8
  extraPortMappings:
  - containerPort: 30080
    hostPort: 8080
    listenAddress: "0.0.0.0"
    protocol: TCP
```

**Confidence: HIGH** -- Verified against official kind docs. The `listenAddress: "0.0.0.0"` is important for WSL2 compatibility (127.0.0.1 does not work from within WSL2 when Docker Desktop manages networking).

### Pattern 2: Helm CI Values Override

**What:** A separate values file that overrides production defaults for the test environment.

**When to use:** When deploying via Helm into kind for testing. Never modify the base `values.yaml` for test-only settings.

**Example:**
```yaml
# hack/ci-values.yaml
image:
  repository: kterodactyl
  tag: test
  pullPolicy: Never    # Image is loaded via kind load, not pulled from registry

apiService:
  type: NodePort
  port: 8080
  nodePort: 30080      # Must match kind-config.yaml containerPort

manager:
  resources:
    limits:
      cpu: "1"
      memory: 256Mi
    requests:
      cpu: 100m
      memory: 128Mi
```

**Confidence: HIGH** -- Standard Helm override pattern. `pullPolicy: Never` is mandatory because kind-loaded images are not in any registry.

### Pattern 3: Readiness Wait Script

**What:** A shell script that blocks until the deployed application is actually serving requests.

**When to use:** After `helm install`, before declaring setup complete or running tests.

**Two-stage readiness check:**
1. `kubectl wait` for the deployment to have `Available` condition (K8s level)
2. `curl` the health endpoint through the NodePort (network level, confirms extraPortMappings work)

**Example:**
```bash
#!/usr/bin/env bash
# hack/wait-for-ready.sh
set -euo pipefail

NAMESPACE="${1:-kterodactyl-system}"
TIMEOUT="${2:-180s}"

echo "Waiting for deployment to be available..."
kubectl wait deployment -l app.kubernetes.io/name=kterodactyl \
  -n "$NAMESPACE" \
  --for=condition=Available \
  --timeout="$TIMEOUT"

echo "Waiting for API health endpoint..."
for i in $(seq 1 30); do
  if curl -sf http://localhost:8080/healthz > /dev/null 2>&1; then
    echo "Kterodactyl is ready at http://localhost:8080"
    exit 0
  fi
  echo "  Attempt $i/30: not ready yet..."
  sleep 2
done

echo "ERROR: Kterodactyl did not become ready within timeout"
exit 1
```

**Confidence: HIGH** -- Two-stage check catches both Kubernetes-level issues (pod crash loops) and network-level issues (port mapping misconfiguration).

### Pattern 4: Makefile Target Orchestration

**What:** Makefile targets that chain the full lifecycle.

**Example:**
```makefile
KIND_CLUSTER ?= kterodactyl-test-e2e
E2E_IMG ?= kterodactyl:test

.PHONY: test-e2e-setup
test-e2e-setup: ## Create kind cluster, build/load image, Helm install, wait for readiness.
	@$(KIND) create cluster --name $(KIND_CLUSTER) --config hack/kind-config.yaml
	$(MAKE) docker-build IMG=$(E2E_IMG)
	@$(KIND) load docker-image $(E2E_IMG) --name $(KIND_CLUSTER)
	helm install kterodactyl chart/ \
		-f hack/ci-values.yaml \
		-n kterodactyl-system --create-namespace --wait --timeout 3m
	bash hack/wait-for-ready.sh kterodactyl-system 180s

.PHONY: test-e2e-teardown
test-e2e-teardown: ## Delete the kind cluster and all resources.
	@$(KIND) delete cluster --name $(KIND_CLUSTER)
```

**Confidence: HIGH** -- Straightforward Make orchestration. `kind delete cluster` is idempotent (exits 0 if cluster doesn't exist).

### Anti-Patterns to Avoid

- **Modifying base `values.yaml` for test settings:** Use a separate `-f hack/ci-values.yaml` override file. Base values should reflect production defaults.
- **Using `kubectl port-forward` instead of NodePort:** Port-forward is a running process that can die, creating flaky CI. NodePort with extraPortMappings is stateless and deterministic.
- **Using `:latest` image tag with kind:** Kind with `:latest` tag triggers `imagePullPolicy: Always`, which tries to pull from a registry. Use a specific tag like `:test` with `pullPolicy: Never`.
- **Hardcoding the kind cluster name without a variable:** The Makefile already uses `KIND_CLUSTER` variable. Keep it. CI may override it with run-ID-prefixed names for isolation.
- **Skipping the health check after Helm install:** `helm install --wait` waits for the deployment to be ready at the K8s level, but does not verify the application is actually serving requests through the NodePort mapping. The curl health check is essential.
- **Using `listenAddress: "127.0.0.1"` in kind config:** On WSL2 with Docker Desktop, `127.0.0.1` may not be accessible from the WSL2 shell. Use `0.0.0.0` for cross-platform compatibility.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| K8s cluster lifecycle | Custom scripts to manage Docker containers as K8s nodes | `kind create cluster` / `kind delete cluster` | kind handles kubelet, etcd, API server, networking, and cleanup. Idempotent delete. |
| Image loading into cluster | Docker save/import pipelines | `kind load docker-image` | kind handles the Docker-to-containerd transfer. Supports named clusters. |
| CRD installation | Manual `kubectl apply -f chart/crds/` | Helm's `chart/crds/` directory convention | Helm installs CRDs from `crds/` automatically on `helm install`. No separate step needed. |
| Deployment readiness | Custom polling loops in Go test code | `kubectl wait --for=condition=Available` + curl | Standard K8s tooling. The shell script is reusable across test frameworks. |
| Helm values templating | Envsubst or sed replacements in values files | Helm `--set` flags or `-f override.yaml` | Helm's value merging is battle-tested. Override files are declarative and auditable. |

**Key insight:** Every building block for this phase already exists as a standalone tool (kind, Docker, Helm, kubectl, curl). Phase 15 is purely orchestration -- wiring existing tools into a deterministic sequence via Makefile targets.

## Common Pitfalls

### Pitfall 1: Image Tag `:latest` With Kind

**What goes wrong:** The operator pod enters `ImagePullBackOff` because Kubernetes tries to pull from a registry.
**Why it happens:** When the image tag is `:latest` (or omitted), Kubernetes defaults to `imagePullPolicy: Always`. Kind-loaded images exist only in the node's containerd store, not in any registry.
**How to avoid:** Use a specific tag (e.g., `kterodactyl:test`) and set `imagePullPolicy: Never` in the Helm values override.
**Warning signs:** Pod status shows `ImagePullBackOff` or `ErrImagePull`. `kubectl describe pod` shows "Failed to pull image."

### Pitfall 2: NodePort Mismatch

**What goes wrong:** `curl localhost:8080` gets "connection refused" even though the pod is running.
**Why it happens:** The kind config's `containerPort`, the Helm service's `nodePort`, and the application's container port are not aligned correctly.
**How to avoid:** Ensure the chain: kind `containerPort: 30080` == service `nodePort: 30080`, service `targetPort: 8080` == container port 8080, kind `hostPort: 8080` == what the developer/test accesses. The three different port values serve three different purposes.
**Warning signs:** Pod is Running, service exists, but `curl localhost:8080` fails. Check `kubectl get svc -n kterodactyl-system` to verify nodePort assignment.

### Pitfall 3: WSL2 listenAddress

**What goes wrong:** `curl localhost:8080` fails on WSL2 even though Docker shows the port mapping.
**Why it happens:** When `listenAddress` is `127.0.0.1`, Docker Desktop binds the port on the Windows host's loopback. The WSL2 VM may not route to Windows loopback correctly depending on Docker Desktop version and WSL2 networking mode.
**How to avoid:** Use `listenAddress: "0.0.0.0"` in `hack/kind-config.yaml`. This binds on all interfaces and works on both native Linux and WSL2.
**Warning signs:** Works on CI (native Linux) but fails on developer's WSL2 machine.

### Pitfall 4: Stale Cluster State on Re-creation

**What goes wrong:** After teardown + setup, the new cluster has stale Docker networks or port conflicts.
**Why it happens:** Docker may not fully clean up kind's Docker network on `kind delete cluster`. Rarely, the host port stays bound.
**How to avoid:** `test-e2e-setup` should call `kind delete cluster` before `kind create cluster` to ensure a clean slate. Since `kind delete` is idempotent, this is safe.
**Warning signs:** `kind create cluster` fails with "port already in use" or "network already exists."

### Pitfall 5: Helm CRD Upgrade Limitation

**What goes wrong:** After modifying a CRD and re-running setup, the CRD in the cluster is stale.
**Why it happens:** Helm only installs CRDs from the `crds/` directory on the FIRST `helm install`. It does NOT update CRDs on `helm upgrade`. Since `test-e2e-teardown` destroys the entire cluster, this is actually not a problem for the teardown-recreate workflow.
**How to avoid:** The teardown-recreate pattern naturally avoids this because each `test-e2e-setup` starts with a fresh cluster. This is one reason the idempotent teardown-create cycle is the right pattern for development.
**Warning signs:** Only relevant if someone tries to `helm upgrade` in an existing cluster without teardown.

### Pitfall 6: Docker Build Time in CI

**What goes wrong:** The `docker build` step takes 5+ minutes on cold cache, making the total setup time unacceptable.
**Why it happens:** The multi-stage Dockerfile downloads Go modules, npm dependencies, builds the React SPA, and compiles the Go binary. On a fresh CI runner, nothing is cached.
**How to avoid:** For Phase 15 (local dev focus), this is acceptable. For Phase 17 (CI pipeline), use Docker layer caching (`docker/build-push-action` with GHA cache) and consider pre-building the image in an earlier job.
**Warning signs:** Total `test-e2e-setup` takes >5 minutes locally.

## Code Examples

Verified patterns from official sources and codebase analysis:

### Kind Cluster Config

```yaml
# hack/kind-config.yaml
# Source: https://kind.sigs.k8s.io/docs/user/configuration/
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  image: kindest/node:v1.32.11@sha256:5fc52d52a7b9574015299724bd68f183702956aa4a2116ae75a63cb574b35af8
  extraPortMappings:
  - containerPort: 30080
    hostPort: 8080
    listenAddress: "0.0.0.0"
    protocol: TCP
```

### Helm Service Template Update

The current `chart/templates/service.yaml` does NOT include a `nodePort` field. It must be updated:

```yaml
# chart/templates/service.yaml (updated)
apiVersion: v1
kind: Service
metadata:
  name: {{ include "kterodactyl.fullname" . }}-api
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "kterodactyl.labels" . | nindent 4 }}
spec:
  type: {{ .Values.apiService.type }}
  ports:
  - name: http
    port: {{ .Values.apiService.port }}
    targetPort: 8080
    protocol: TCP
    {{- if and (eq .Values.apiService.type "NodePort") .Values.apiService.nodePort }}
    nodePort: {{ .Values.apiService.nodePort }}
    {{- end }}
  selector:
    {{- include "kterodactyl.selectorLabels" . | nindent 4 }}
```

### Helm Values Addition

Add `nodePort` field to `chart/values.yaml`:

```yaml
apiService:
  type: ClusterIP
  port: 8080
  # -- NodePort value (only used when type is NodePort)
  nodePort: ""
```

### CI Values Override

```yaml
# hack/ci-values.yaml
image:
  repository: kterodactyl
  tag: test
  pullPolicy: Never

apiService:
  type: NodePort
  port: 8080
  nodePort: 30080

# Relaxed resources for kind (single-node, limited resources)
manager:
  resources:
    limits:
      cpu: "1"
      memory: 256Mi
    requests:
      cpu: 100m
      memory: 128Mi
```

### Makefile Targets

```makefile
# New variables
E2E_IMG ?= kterodactyl:test

.PHONY: test-e2e-setup
test-e2e-setup: ## Create kind cluster with Helm-deployed Kterodactyl (accessible at localhost:8080).
	@$(KIND) delete cluster --name $(KIND_CLUSTER) 2>/dev/null || true
	$(KIND) create cluster --name $(KIND_CLUSTER) --config hack/kind-config.yaml
	$(MAKE) docker-build IMG=$(E2E_IMG)
	$(KIND) load docker-image $(E2E_IMG) --name $(KIND_CLUSTER)
	helm install kterodactyl chart/ \
		-f hack/ci-values.yaml \
		-n kterodactyl-system --create-namespace --wait --timeout 3m
	bash hack/wait-for-ready.sh kterodactyl-system 180s
	@echo "Kterodactyl is ready at http://localhost:8080"

.PHONY: test-e2e-teardown
test-e2e-teardown: ## Delete the kind cluster and all associated resources.
	$(KIND) delete cluster --name $(KIND_CLUSTER)
```

### Readiness Wait Script

```bash
#!/usr/bin/env bash
# hack/wait-for-ready.sh
set -euo pipefail

NAMESPACE="${1:-kterodactyl-system}"
TIMEOUT="${2:-180s}"

echo "Waiting for deployment to be available in namespace $NAMESPACE..."
kubectl wait deployment -l app.kubernetes.io/name=kterodactyl \
  -n "$NAMESPACE" \
  --for=condition=Available \
  --timeout="$TIMEOUT"

echo "Verifying API is accessible at localhost:8080..."
for i in $(seq 1 30); do
  if curl -sf http://localhost:8080/healthz > /dev/null 2>&1; then
    echo "Kterodactyl is ready at http://localhost:8080"
    exit 0
  fi
  sleep 2
done

echo "ERROR: API did not become accessible at localhost:8080 within 60s"
echo "Debug: kubectl get pods -n $NAMESPACE"
kubectl get pods -n "$NAMESPACE"
echo "Debug: kubectl logs -l app.kubernetes.io/name=kterodactyl -n $NAMESPACE --tail=50"
kubectl logs -l app.kubernetes.io/name=kterodactyl -n "$NAMESPACE" --tail=50 2>/dev/null || true
exit 1
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| kustomize deploy (`make deploy`) for e2e | Helm install with values override | Phase 15 (now) | Helm matches production deployment path; tests exercise the same install mechanism users follow |
| `kubectl port-forward` in CI | NodePort + kind extraPortMappings | kind v0.5.0+ (2019) | Deterministic, stateless port access. No background process management. |
| `kind get clusters` + conditional create | Delete-then-create (idempotent) | Best practice | Avoids stale state. `kind delete` exits 0 if cluster doesn't exist. |
| `imagePullPolicy: IfNotPresent` with kind | `imagePullPolicy: Never` + explicit `kind load` | Kubernetes convention | `Never` guarantees the cluster uses the locally built image, not a stale registry image. |

**Deprecated/outdated:**
- The existing `setup-test-e2e` target creates a bare kind cluster without config, port mappings, or Helm deployment. It will be superseded by `test-e2e-setup`.
- The existing `test-e2e` target uses `make deploy` (kustomize) instead of Helm. Phase 15 creates `test-e2e-setup` as the new Helm-based equivalent.
- The existing `cleanup-test-e2e` target will be superseded by `test-e2e-teardown` (same functionality, consistent naming).

## Codebase-Specific Findings

### Helm Chart Service Template Gap

The current `chart/templates/service.yaml` does not support `nodePort`. It unconditionally renders without a `nodePort` field. This is the **only production code change** required: add a conditional `nodePort` field when `apiService.type` is `NodePort`.

### Existing Makefile Targets (Superseded, Not Deleted)

The Makefile already has `setup-test-e2e`, `test-e2e`, and `cleanup-test-e2e` targets (lines 78-99). These use kustomize-based deployment (not Helm). The new `test-e2e-setup` and `test-e2e-teardown` targets should be added alongside them. The old targets can be left for backward compatibility or removed -- this is a planner decision.

### Image Name Convention

The Makefile uses `IMG ?= controller:latest` as the default image name. For kind loading, a distinct tag is needed. The `E2E_IMG ?= kterodactyl:test` variable avoids collision with the default and makes the purpose clear.

### CRD Handling

The Helm chart stores CRDs in `chart/crds/` (two files: `game.kterodactyl.io_backups.yaml` and `game.kterodactyl.io_gameservers.yaml`). Helm automatically installs these on `helm install`. No separate `make install` (kustomize CRD apply) is needed.

### Deployment Label Selector

The `kubectl wait` command in the readiness script needs to match the deployment. The Helm chart's deployment uses the label `app.kubernetes.io/name: kterodactyl` (from `_helpers.tpl` selectorLabels). This is the correct selector for `kubectl wait`.

### Container Security Context

The deployment template enforces `readOnlyRootFilesystem: true` and `runAsNonRoot: true`. These work fine in kind. No special security context overrides are needed for the test environment.

### Health Endpoints

The deployment template configures liveness probe at `:8081/healthz` and readiness probe at `:8081/readyz`. The API server listens on `:8080`. The curl health check in `wait-for-ready.sh` should target `localhost:8080/healthz` (the API server's health endpoint exposed through NodePort), not port 8081 (which is internal to the pod).

Verifying the health endpoint exists on the API port: the `--api-bind-address=:8080` flag is set in the deployment args. The API server mounts health endpoints. The health check through NodePort validates the entire chain: host -> Docker -> kind node -> NodePort -> Service -> Pod -> container port 8080.

## Open Questions

1. **Should the old Makefile targets (`setup-test-e2e`, `test-e2e`, `cleanup-test-e2e`) be removed or kept?**
   - What we know: The new targets (`test-e2e-setup`, `test-e2e-teardown`) supersede them functionally. The old `test-e2e` target also runs Ginkgo tests, which the new targets do not.
   - What's unclear: Whether Phase 16 (Playwright) or Phase 17 (CI) will need to compose these targets differently.
   - Recommendation: Keep the old targets but update `test-e2e` to use the new Helm-based setup. Or rename to clearly delineate. Let the planner decide.

2. **Should `helm` be added as a Makefile dependency (like `kustomize`, `controller-gen`)?**
   - What we know: `kustomize` and other tools are auto-installed via `go-install-tool` in the Makefile. Helm is more commonly installed system-wide.
   - What's unclear: Whether developers will have Helm pre-installed. The CI workflow in `.github/workflows/test-e2e.yml` does not currently install Helm (it would need to).
   - Recommendation: Add a `HELM ?= helm` variable and a download target similar to the existing tool installation pattern, OR document Helm as a prerequisite. The planner should decide based on ergonomics.

3. **Health endpoint on API port -- RESOLVED**
   - Confirmed: `internal/api/routes.go:58-59` registers `/healthz` and `/readyz` on the chi router (port 8080). The readiness script can safely `curl localhost:8080/healthz`. This is separate from the controller-runtime health probes on port 8081 (used by kubelet liveness/readiness probes).

## Sources

### Primary (HIGH confidence)
- [kind official configuration docs](https://kind.sigs.k8s.io/docs/user/configuration/) -- extraPortMappings, cluster schema, node configuration
- [kind quick start](https://kind.sigs.k8s.io/docs/user/quick-start/) -- `kind load docker-image`, create/delete commands, v0.31.0
- [kind WSL2 docs](https://kind.sigs.k8s.io/docs/user/using-wsl2/) -- listenAddress considerations, Docker Desktop integration
- [Helm CRD best practices](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/) -- CRDs in `crds/` directory auto-installed on `helm install`
- Codebase analysis: `Makefile`, `chart/`, `test/e2e/`, `config/` -- existing targets, Helm templates, e2e scaffolding

### Secondary (MEDIUM confidence)
- [Exposing NodePort in kind cluster](https://scriptcrunch.com/expose-nodeport-kind-cluster/) -- NodePort + extraPortMappings alignment requirement (cross-verified with kind official docs)
- [Helm --wait behavior](https://github.com/helm/helm/issues/3173) -- `--wait` waits for pod readiness but may not catch all networking issues
- [kind issue #2889](https://github.com/kubernetes-sigs/kind/issues/2889) -- WSL2 port mapping specifics (ingress-focused but informative)
- [kind issue #2182](https://github.com/kubernetes-sigs/kind/issues/2182) -- `kind delete cluster` idempotency behavior confirmed

### Tertiary (LOW confidence)
- Prior milestone research in `.planning/research/ARCHITECTURE.md` -- proposes `listenAddress: "127.0.0.1"` which may not work on WSL2; this research corrects to `"0.0.0.0"`

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- kind, Helm, Docker, kubectl are all well-established. Versions are pinned.
- Architecture: HIGH -- The pattern (kind config + Helm values override + readiness script + Makefile targets) is well-documented and widely used. All building blocks already exist in the codebase.
- Pitfalls: HIGH -- Each pitfall is verified against official documentation or known issues. The WSL2 listenAddress issue is the most subtle but documented.

**What might I have missed:**
- Confirmed: `/healthz` and `/readyz` are registered on the API router (port 8080) at `internal/api/routes.go:58-59`. No ambiguity.
- If the operator requires Gateway API CRDs to start (beyond its own CRDs), those would need to be installed separately. The Helm chart only installs its own CRDs from `chart/crds/`. However, the operator likely handles missing Gateway API CRDs gracefully (it watches for them but doesn't crash without them).
- Docker Desktop on WSL2 has occasional networking issues beyond listenAddress. If developers hit problems, `kubectl port-forward` remains a fallback (but is out of scope for this phase's Makefile targets).

**Research date:** 2026-02-19
**Valid until:** 2026-03-19 (stable domain, tools are mature)
