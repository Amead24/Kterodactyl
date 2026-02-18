# Architecture Patterns

**Domain:** CI/CD test infrastructure for Kubernetes game server operator
**Researched:** 2026-02-17

## Recommended Architecture

The testing architecture adds three layers to the existing codebase without modifying production code structure. Each layer targets a different integration boundary: Go API handlers (httptest), full-stack browser flows (Playwright against kind cluster), and CI orchestration (GitHub Actions).

```
+------------------------------------------------------------------+
|                     GitHub Actions CI Pipeline                     |
|  +------------+  +---------------+  +---------------------------+ |
|  | Lint + Unit |  | Go API Integ. |  | E2E: kind + Helm + PW    | |
|  | (envtest)   |  | (httptest)    |  |                           | |
|  +------+------+  +------+--------+  | +-------+ +------------+ | |
|         |                |            | | kind  | | Playwright | | |
|         v                v            | |cluster+--> browser   | | |
|     go test ./...    go test          | +---+---+ +------+-----+ | |
|     (existing)     ./test/            |     |            |       | |
|                    integration/       |     v            v       | |
|                                       | Helm deploy  localhost   | |
|                                       | + port-fwd   :8080      | |
|                                       +---------------------------+ |
+------------------------------------------------------------------+
```

### Three Test Layers

| Layer | What It Tests | Runtime | K8s Required |
|-------|---------------|---------|--------------|
| Unit + Controller (existing) | Reconciler logic, handler logic | envtest (fake API server) | No (envtest) |
| API Integration (new) | HTTP API contract, auth flows, CRUD sequences | httptest + fake client | No |
| E2E (new, replaces scaffold) | Full user journey: deploy, login, create server, view UI | kind + Helm + Playwright | Yes |

### Component Boundaries

| Component | Responsibility | Communicates With |
|-----------|---------------|-------------------|
| `test/integration/` | Go httptest-based API integration tests | `internal/api` Server via `httptest.NewServer` |
| `test/e2e/` | Ginkgo-based kind cluster lifecycle (replaces scaffold) | kind CLI, kubectl, Helm, Playwright |
| `e2e/` | Playwright browser tests (TypeScript) | Running app via `localhost:8080` |
| `hack/kind-config.yaml` | kind cluster configuration with port mappings | kind CLI |
| `hack/ci-values.yaml` | Helm values override for CI test environment | Helm chart |
| `hack/wait-for-ready.sh` | Script to wait for deployment + port-forward | kubectl, curl |
| `.github/workflows/ci.yml` | Unified CI pipeline (replaces 3 separate workflows) | All test layers |

## Existing Code: What Stays, What Changes

### Unchanged (production code)

| File/Directory | Why Unchanged |
|----------------|---------------|
| `cmd/main.go` | Production entrypoint; tests use different bootstrapping |
| `internal/api/` | Test subjects, not modified for testability (already well-structured) |
| `internal/controller/` | Already has envtest suite, continues working as-is |
| `internal/api/helpers_test.go` | Existing unit test helpers stay; new integration tests are separate |
| `chart/` | Helm chart used as-is in E2E; CI values override only |
| `web/` | Frontend code unchanged; Playwright tests interact via browser |

### Modified (existing files that need updates)

| File | Change | Rationale |
|------|--------|-----------|
| `Makefile` | Add `test-integration`, `test-e2e-playwright` targets; update `test-e2e` | New test layers need Make targets for local dev and CI |
| `.github/workflows/` | Consolidate into single `ci.yml` or add `ci.yml` alongside existing | Unified pipeline with proper job dependencies |
| `web/package.json` | No change needed | Playwright has its own package.json in `e2e/` |
| `.gitignore` | Add `e2e/test-results/`, `e2e/playwright-report/`, `e2e/.auth/` | Playwright generates artifacts that should not be committed |
| `.dockerignore` | Add `e2e/`, `test/`, `playwright-report/` | Exclude test artifacts from Docker builds |
| `chart/templates/service.yaml` | Add conditional `nodePort` field | kind E2E needs NodePort with extraPortMappings |

### New Files/Directories

```
kterodactyl/
  e2e/                              # Playwright test suite (TypeScript)
    playwright.config.ts             # Playwright configuration
    package.json                     # Separate from web/ to avoid polluting prod deps
    tsconfig.json                    # TypeScript config for tests
    tests/
      auth.spec.ts                   # Login, register, token refresh flows
      gameserver-crud.spec.ts        # Create, list, start, stop, delete game servers
      admin.spec.ts                  # Admin panel, user management, invites
      health.spec.ts                 # Smoke test: health endpoints + SPA loads
    fixtures/
      auth.fixture.ts                # Authenticated page/context fixture
    helpers/
      api-client.ts                  # Direct API calls for test setup (skip UI)

  test/
    integration/                     # Go API integration tests
      api_test.go                    # Full HTTP lifecycle tests
      auth_flow_test.go              # Multi-step auth scenarios
      gameserver_lifecycle_test.go   # Create -> Start -> Stop -> Delete sequences
      helpers_test.go                # Shared test utilities
    e2e/                             # Existing Ginkgo E2E (updated, not replaced)
      e2e_suite_test.go              # Updated: use Helm instead of kustomize
      e2e_test.go                    # Updated: add app-specific test cases

  hack/
    kind-config.yaml                 # kind cluster config with port mappings
    ci-values.yaml                   # Helm values for CI environment
    wait-for-ready.sh                # Script to wait for deployment readiness
```

## Data Flow: Test Execution

### Layer 1: Go API Integration Tests (httptest)

**Confidence: HIGH** -- This pattern is already established in the codebase via `internal/api/*_test.go`.

```
go test ./test/integration/
  |
  +--> testServer := newTestServer(t)     # Same pattern as helpers_test.go
  |     +-- fake K8s client (controller-runtime/pkg/client/fake)
  |     +-- real chi router
  |     +-- real JWT service
  |     +-- real manifest loader
  |
  +--> httptest.NewServer(srv.router)      # Full HTTP stack, real TCP
  |
  +--> HTTP requests to localhost:PORT     # Real HTTP client calls
  |
  +--> Assert responses                    # Status codes, JSON bodies, headers
```

This layer differs from the existing `internal/api/*_test.go` unit tests in scope:

| Existing Unit Tests (`internal/api/`) | New Integration Tests (`test/integration/`) |
|---------------------------------------|---------------------------------------------|
| Single handler, single request | Multi-step sequences across handlers |
| `httptest.NewRecorder` (no TCP) | `httptest.NewServer` (real TCP) |
| Test in `package api` (whitebox) | Test in `package integration` (blackbox) |
| Verify handler behavior | Verify API contract and state transitions |

The integration tests reuse the `testServer` construction pattern already proven in `internal/api/helpers_test.go` but run against a real `httptest.NewServer` for full HTTP round-trips. They test multi-step flows: register user, login, create game server, list servers, verify server appears, start server, verify state change.

**Key architectural detail:** The integration tests import `internal/api` to construct the server. This is possible because the tests live in `test/integration/` within the same Go module. The `testServer` helper from `helpers_test.go` is not directly reusable (it is in the `api` package's test files, not exported), so the integration test package will have its own setup function that mirrors the same pattern:

```go
// test/integration/helpers_test.go
package integration

import (
    "github.com/kterodactyl/kterodactyl/internal/api"
    "github.com/kterodactyl/kterodactyl/internal/auth"
    "github.com/kterodactyl/kterodactyl/internal/manifest"
    // ... same imports as internal/api/helpers_test.go
)

func newIntegrationServer(t *testing.T) (*httptest.Server, *auth.JWTService) {
    // Same setup as internal/api/helpers_test.go newTestServer()
    // but returns httptest.NewServer instead of testServer wrapper
    srv := api.NewServer(api.Config{
        Client:            fakeClient,
        JWTService:        jwtSvc,
        UserStore:         userStore,
        // ...
        BindAddress:       ":0",
    })
    ts := httptest.NewServer(srv.HTTPServer().Handler)
    t.Cleanup(ts.Close)
    return ts, jwtSvc
}
```

**Important note on HTTPServer().Handler:** The existing `Server.HTTPServer()` method returns `*http.Server` whose `Handler` field is the chi router. This is the integration point -- `httptest.NewServer` wraps that handler. No production code changes needed.

### Layer 2: E2E Tests (kind + Helm + Playwright)

**Confidence: MEDIUM** -- kind and Helm patterns are well-established; Playwright integration with Go projects is less documented but straightforward.

```
make test-e2e
  |
  +--> kind create cluster --config hack/kind-config.yaml
  |     +-- extraPortMappings: containerPort 30080 -> hostPort 8080
  |
  +--> docker build -t kterodactyl:test .
  +--> kind load docker-image kterodactyl:test
  |
  +--> helm install kterodactyl chart/ -f hack/ci-values.yaml
  |     +-- apiService.type: NodePort
  |     +-- apiService.nodePort: 30080
  |     +-- image.repository: kterodactyl
  |     +-- image.tag: test
  |     +-- image.pullPolicy: Never
  |
  +--> hack/wait-for-ready.sh              # Wait for deployment ready
  |     +-- kubectl wait deployment ...
  |     +-- curl healthz endpoint
  |
  +--> npx playwright test                 # Browser tests against localhost:8080
  |     +-- Tests interact with real app running in kind
  |     +-- Auth flows use real JWT
  |     +-- GameServer CRUD hits real K8s API
  |
  +--> kind delete cluster
```

### kind Cluster Configuration

```yaml
# hack/kind-config.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 30080    # NodePort for API server
    hostPort: 8080           # Accessible as localhost:8080
    listenAddress: "127.0.0.1"
    protocol: TCP
```

**Why NodePort + extraPortMappings over port-forward:**
- `kubectl port-forward` is fragile in CI (can die, race conditions with process management)
- NodePort with kind extraPortMappings is deterministic and survives test suite duration
- Playwright's `baseURL` points to `http://localhost:8080` -- no process management needed

**Why NOT the existing Ginkgo E2E pattern for browser tests:**
The existing `test/e2e/` scaffold uses Ginkgo + `kubectl` commands + Go's `os/exec`. This is appropriate for operator-level verification (pod running, CRDs installed, metrics endpoint). But browser testing requires a real browser engine (Chromium), DOM interaction, and visual assertions. Playwright is purpose-built for this. The two test types complement each other: Ginkgo verifies the operator, Playwright verifies the user experience.

### Helm CI Values Override

```yaml
# hack/ci-values.yaml
image:
  repository: kterodactyl
  tag: test
  pullPolicy: Never          # Image loaded into kind via `kind load docker-image`

apiService:
  type: NodePort
  nodePort: 30080             # Must match kind extraPortMappings containerPort

manager:
  resources:
    limits:
      cpu: 1
      memory: 512Mi
    requests:
      cpu: 100m
      memory: 128Mi

adminConfig:
  auth:
    registrationEnabled: "true"
    jwtExpirationHours: "24"
  networking:
    baseDomain: "test.local"
    gateway:
      name: ""                # Disable Gateway API in CI (no controller available)
```

**Required chart modification:** The Helm chart's `service.yaml` needs a conditional `nodePort` field:

```yaml
# chart/templates/service.yaml (modified)
spec:
  type: {{ .Values.apiService.type }}
  ports:
  - name: http
    port: {{ .Values.apiService.port }}
    targetPort: 8080
    {{- if and (eq .Values.apiService.type "NodePort") .Values.apiService.nodePort }}
    nodePort: {{ .Values.apiService.nodePort }}
    {{- end }}
    protocol: TCP
```

This is backward-compatible: when `nodePort` is not set in values, the field is omitted and Kubernetes assigns one automatically (existing behavior).

### Playwright Configuration

```typescript
// e2e/playwright.config.ts
import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  timeout: 30_000,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: process.env.CI
    ? [['html', { open: 'never' }], ['github']]
    : [['html']],
  use: {
    baseURL: process.env.BASE_URL || 'http://localhost:8080',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  // No webServer config: the app runs in kind, not spawned by Playwright
});
```

**Key decision: No `webServer` in Playwright config.** Unlike typical Playwright setups where the config launches a dev server, here the app is deployed into kind via Helm before Playwright runs. Playwright just needs a `baseURL` pointing to the already-running app. This keeps concerns separated: Make/scripts manage the cluster lifecycle, Playwright focuses on browser testing.

## Patterns to Follow

### Pattern 1: Test Fixture for Authenticated Playwright Tests

**What:** Reusable authentication state for Playwright tests that need a logged-in user.
**When:** Any test that interacts with authenticated API endpoints or UI pages.

```typescript
// e2e/fixtures/auth.fixture.ts
import { test as base, type Page } from '@playwright/test';

type AuthFixtures = {
  authenticatedPage: Page;
};

export const test = base.extend<AuthFixtures>({
  authenticatedPage: async ({ page }, use) => {
    // Register + login via API (faster than UI flow)
    const response = await page.request.post('/api/v1/auth/register', {
      data: {
        username: `testuser-${Date.now()}`,
        email: `test-${Date.now()}@test.com`,
        password: 'TestPassword123!',
      },
    });
    const { token } = await response.json();

    // Set token in localStorage (matches the React app's zustand auth store)
    await page.goto('/');
    await page.evaluate((t) => {
      localStorage.setItem('auth-storage', JSON.stringify({
        state: { token: t, user: null },
        version: 0,
      }));
    }, token);
    await page.goto('/');

    await use(page);
  },
});
```

**Why this pattern:** The React app uses zustand with persist middleware storing the JWT in `localStorage` under `auth-storage`. Setting this directly via `page.evaluate` is faster and more reliable than navigating to the login page and typing credentials for every test.

### Pattern 2: API Client for Test Setup (Playwright)

**What:** Direct HTTP client for creating test data without going through the UI.
**When:** Tests that need preconditions (game servers already exist, users already registered).

```typescript
// e2e/helpers/api-client.ts
export class TestApiClient {
  private token?: string;

  constructor(private baseURL: string) {}

  async register(username: string, password: string): Promise<string> {
    const res = await fetch(`${this.baseURL}/api/v1/auth/register`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        username,
        email: `${username}@test.com`,
        password,
      }),
    });
    const data = await res.json();
    this.token = data.token;
    return data.token;
  }

  async createGameServer(name: string, gameType: string): Promise<void> {
    await fetch(`${this.baseURL}/api/v1/gameservers`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${this.token}`,
      },
      body: JSON.stringify({
        name,
        gameType,
        parameters: { EULA: 'TRUE', TYPE: 'VANILLA' },
      }),
    });
  }
}
```

### Pattern 3: Go Integration Test with Multi-Step Flows

**What:** Testing full API contract sequences using real HTTP.
**When:** Validating state transitions and cross-handler consistency.

```go
// test/integration/gameserver_lifecycle_test.go
package integration

func TestGameServerLifecycle(t *testing.T) {
    ts, jwtSvc := newIntegrationServer(t)

    // Step 1: Register a user
    regResp, err := http.Post(ts.URL+"/api/v1/auth/register",
        "application/json",
        strings.NewReader(`{"username":"lifecycle","email":"lc@test.com","password":"Pass123!"}`))
    require.NoError(t, err)
    require.Equal(t, http.StatusOK, regResp.StatusCode)

    var authBody map[string]string
    json.NewDecoder(regResp.Body).Decode(&authBody)
    token := authBody["token"]

    // Step 2: Create a game server
    req, _ := http.NewRequest("POST", ts.URL+"/api/v1/gameservers",
        strings.NewReader(`{"name":"mc-test","gameType":"minecraft","parameters":{"EULA":"TRUE","TYPE":"VANILLA"}}`))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+token)
    createResp, err := http.DefaultClient.Do(req)
    require.NoError(t, err)
    require.Equal(t, http.StatusCreated, createResp.StatusCode)

    // Step 3: List servers -- verify it appears
    req, _ = http.NewRequest("GET", ts.URL+"/api/v1/gameservers", nil)
    req.Header.Set("Authorization", "Bearer "+token)
    listResp, err := http.DefaultClient.Do(req)
    require.NoError(t, err)
    require.Equal(t, http.StatusOK, listResp.StatusCode)

    var servers []map[string]interface{}
    json.NewDecoder(listResp.Body).Decode(&servers)
    require.Len(t, servers, 1)
    require.Equal(t, "mc-test", servers[0]["name"])
}
```

### Pattern 4: Wait-for-Ready Script

**What:** Shell script that blocks until the app is healthy in kind.
**When:** Between Helm install and Playwright test execution.

```bash
#!/usr/bin/env bash
# hack/wait-for-ready.sh
set -euo pipefail

NAMESPACE="${1:-kterodactyl-system}"
TIMEOUT="${2:-120s}"
URL="${3:-http://localhost:8080/healthz}"

echo "Waiting for deployment to be ready..."
kubectl wait deployment -n "$NAMESPACE" \
  -l app.kubernetes.io/name=kterodactyl \
  --for=condition=Available --timeout="$TIMEOUT"

echo "Waiting for health endpoint at $URL ..."
for i in $(seq 1 60); do
  if curl -sf "$URL" > /dev/null 2>&1; then
    echo "App is healthy!"
    exit 0
  fi
  sleep 2
done

echo "ERROR: App did not become healthy within timeout"
kubectl logs -n "$NAMESPACE" -l app.kubernetes.io/name=kterodactyl --tail=50
exit 1
```

## Anti-Patterns to Avoid

### Anti-Pattern 1: Playwright Spawning the Go Binary Directly

**What:** Using Playwright's `webServer` config to `go run cmd/main.go` or run the compiled binary.
**Why bad:** The Go binary requires a Kubernetes cluster (`ctrl.GetConfigOrDie()` in `cmd/main.go` line 170). Running it outside K8s crashes immediately. The binary also needs CRDs installed, the `kterodactyl-admin-config` ConfigMap, and a JWT signing key Secret bootstrapped.
**Instead:** Deploy to kind via Helm (which handles all prerequisites), then point Playwright at the running service.

### Anti-Pattern 2: Mixing Ginkgo and Standard Go Tests in Integration Suite

**What:** Using Ginkgo (BDD framework) for the new API integration tests.
**Why bad:** The existing API unit tests in `internal/api/` use standard `testing.T` with `httptest`. The new integration tests extend this pattern. Mixing Ginkgo and standard tests creates confusion about which assertion library to use, and Ginkgo's global state (`BeforeSuite`/`AfterSuite`) is unnecessary for tests that only need a `testServer`.
**Instead:** Use standard `testing.T` for integration tests. Reserve Ginkgo for controller tests (where envtest requires suite lifecycle) and the kind-based E2E tests (where cluster lifecycle requires `BeforeSuite`/`AfterSuite`).

### Anti-Pattern 3: Running Playwright Tests Inside the kind Cluster

**What:** Using Testkube or similar to execute Playwright inside K8s pods.
**Why bad:** Massively increases complexity (need browser images in cluster, volume mounts for test code, artifact extraction from pods). Appropriate for production monitoring, not CI testing.
**Instead:** Run Playwright on the host/CI runner against `localhost:8080` exposed via kind port mapping.

### Anti-Pattern 4: Separate CI Workflows per Test Layer

**What:** Keeping `test.yml`, `test-e2e.yml`, and `lint.yml` as separate workflows and adding more.
**Why bad:** No dependency ordering between workflows. Expensive E2E tests run even when lint fails. No shared caching. Harder to reason about overall CI status.
**Instead:** Single `ci.yml` with dependent jobs: `lint` -> `unit-test` -> `integration-test` -> `e2e-test`. Each job runs only if its predecessor passes. Keep old workflow files temporarily for backward compatibility, then remove.

### Anti-Pattern 5: Building the Docker Image in Every Test Job

**What:** Running `docker build` in both the Go E2E job and again for Playwright if separated.
**Why bad:** Docker builds are slow (Go compilation + npm ci + Vite build). Double-building wastes CI minutes.
**Instead:** Build once in the E2E job. The image is only needed for kind, so build it only there.

### Anti-Pattern 6: Putting Playwright Dependencies in web/package.json

**What:** Adding `@playwright/test` to the existing `web/package.json`.
**Why bad:** Playwright pulls ~100MB of browser binaries. These would inflate the Docker build context (web/ is COPYed in Dockerfile), increase `npm ci` time for the production frontend, and create dependency conflicts between React's test tooling and Playwright.
**Instead:** Separate `e2e/package.json` with only Playwright dependencies.

## Build Order and Dependency Graph

The build order matters because of cascading dependencies:

```
1. Lint (parallel, no deps)
   +-- Go lint (golangci-lint)
   +-- Frontend lint (eslint)

2. Unit Tests (after lint passes)
   +-- Go unit tests (make test) -- includes envtest controller tests
   +-- Existing internal/api/*_test.go handler tests

3. Integration Tests (after unit tests pass)
   +-- Go API integration tests (go test ./test/integration/)
   +-- No K8s cluster needed, uses controller-runtime fake client

4. E2E Tests (after integration tests pass)
   +-- Build Docker image (multi-stage: frontend + Go binary)
   +-- Create kind cluster with port mappings
   +-- Load image into kind
   +-- Helm install with CI values
   +-- Wait for deployment ready
   +-- Install Playwright browsers (Chromium only)
   +-- Run Playwright tests
   +-- Collect artifacts (traces, screenshots, report)
   +-- Destroy kind cluster
```

### GitHub Actions Pipeline Structure

```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [main]
  pull_request:

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version-file: go.mod }
      - uses: actions/setup-node@v4
        with: { node-version: 22 }
      - run: npm ci
        working-directory: web
      - uses: golangci/golangci-lint-action@v8
        with: { version: v2.7.2 }
      - run: npm run lint
        working-directory: web

  test-unit:
    needs: lint
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version-file: go.mod }
      - run: make test

  test-integration:
    needs: test-unit
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version-file: go.mod }
      - run: go test -v -count=1 ./test/integration/

  test-e2e:
    needs: test-integration
    runs-on: ubuntu-latest
    timeout-minutes: 20
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version-file: go.mod }
      - uses: actions/setup-node@v4
        with: { node-version: 22 }

      # Install kind
      - name: Install kind
        run: |
          curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.31.0/kind-linux-amd64
          chmod +x ./kind && sudo mv ./kind /usr/local/bin/kind

      # Create cluster with port mappings
      - name: Create kind cluster
        run: kind create cluster --config hack/kind-config.yaml --name kterodactyl-e2e

      # Build and load image
      - name: Build Docker image
        run: docker build -t kterodactyl:test .
      - name: Load image into kind
        run: kind load docker-image kterodactyl:test --name kterodactyl-e2e

      # Deploy with Helm
      - name: Helm install
        run: |
          helm install kterodactyl chart/ \
            -f hack/ci-values.yaml \
            -n kterodactyl-system --create-namespace

      # Wait for app
      - name: Wait for readiness
        run: bash hack/wait-for-ready.sh kterodactyl-system 180s

      # Install Playwright
      - name: Install Playwright deps
        run: npm ci
        working-directory: e2e
      - name: Install browsers
        run: npx playwright install --with-deps chromium
        working-directory: e2e

      # Run Playwright
      - name: Run Playwright tests
        run: npx playwright test
        working-directory: e2e
        env:
          BASE_URL: http://localhost:8080

      # Upload artifacts on failure
      - uses: actions/upload-artifact@v4
        if: failure()
        with:
          name: playwright-report
          path: e2e/playwright-report/
          retention-days: 7

      # Cleanup
      - name: Delete kind cluster
        run: kind delete cluster --name kterodactyl-e2e
        if: always()
```

## Integration Points with Existing Code

### How Integration Tests Access the API Server

The key integration point is `api.NewServer()` and `Server.HTTPServer()`:

```
test/integration/helpers_test.go
    |
    +--> imports github.com/kterodactyl/kterodactyl/internal/api
    +--> imports github.com/kterodactyl/kterodactyl/internal/auth
    +--> imports github.com/kterodactyl/kterodactyl/internal/manifest
    |
    +--> api.NewServer(api.Config{...})     # Constructs Server with fake deps
    +--> srv.HTTPServer()                   # Gets *http.Server
    +--> httptest.NewServer(httpSrv.Handler) # Wraps the chi router
```

The `api.Config` struct (defined in `internal/api/server.go`) accepts interfaces and clients that can be faked:
- `Client client.Client` -- use `sigs.k8s.io/controller-runtime/pkg/client/fake`
- `JWTService *auth.JWTService` -- construct with test signing key
- `UserStore auth.UserService` -- `auth.NewUserStore` works with fake client
- `InviteService *auth.InviteService` -- works with fake client
- `ManifestLoader *manifest.Loader` -- `manifest.LoadFromDirectory` with temp dir
- `Clientset` and `RestConfig` -- nil (not needed for API-only tests; WebSocket/exec tests skipped)
- `MetricsClient` -- nil (not needed for API tests)

### How Playwright Tests Access the App

```
e2e/tests/*.spec.ts
    |
    +--> baseURL: http://localhost:8080     # From playwright.config.ts
    |
    +--> kind node extraPortMappings
    |    containerPort: 30080 -> hostPort: 8080
    |
    +--> K8s NodePort Service
    |    port: 8080, nodePort: 30080
    |
    +--> Deployment pod port 8080
    |    (Go binary: --api-bind-address=:8080)
    |
    +--> chi router serves:
         /api/v1/* -> API handlers
         /* -> embedded SPA (go:embed frontend)
```

### How the Ginkgo E2E Evolves

The existing `test/e2e/` uses kustomize-based deployment (`make deploy`). For v1.1:
- Switch from `make deploy` (kustomize) to `helm install` (chart/)
- Remove CertManager dependency (not needed for API server testing)
- Add Playwright execution as a step after operator verification
- Keep the operator verification tests (pod running, metrics endpoint)

```go
// Updated test/e2e/e2e_test.go flow:
// 1. Helm install (replaces make deploy)
// 2. Verify operator pod is running (existing test)
// 3. Verify metrics endpoint (existing test)
// 4. Run Playwright tests (new step)
// 5. Helm uninstall + kind delete (cleanup)
```

## Makefile Targets

New targets to add to the existing Makefile:

```makefile
##@ Testing

.PHONY: test-integration
test-integration: ## Run API integration tests (no K8s cluster needed).
	go test -v -count=1 ./test/integration/

.PHONY: test-e2e-full
test-e2e-full: setup-test-e2e docker-build ## Run full E2E: kind + Helm + Playwright.
	@kind load docker-image $(IMG) --name $(KIND_CLUSTER)
	@helm install kterodactyl chart/ -f hack/ci-values.yaml \
		--set image.repository=$$(echo $(IMG) | cut -d: -f1) \
		--set image.tag=$$(echo $(IMG) | cut -d: -f2) \
		-n kterodactyl-system --create-namespace
	@bash hack/wait-for-ready.sh kterodactyl-system 180s
	@cd e2e && npm ci && npx playwright install --with-deps chromium
	@cd e2e && npx playwright test
	$(MAKE) cleanup-test-e2e
```

## Key Architectural Decision: Separate `e2e/` vs `web/` for Playwright

Playwright tests live in a top-level `e2e/` directory with their own `package.json`, **not** inside `web/`. Reasons:

1. **Dependency isolation**: Playwright's `@playwright/test` and browser binaries (~100MB) should not inflate the production `web/node_modules`. The Vite build for the embedded SPA must not carry test dependencies.
2. **Build context**: The Dockerfile copies `web/` for the frontend build stage. Test dependencies in `web/` would bloat the Docker build context and increase build time.
3. **CI caching**: `web/node_modules` and `e2e/node_modules` can be cached independently. Playwright browser installs are cached separately from npm deps.
4. **Conceptual boundary**: `web/` is the React SPA source. `e2e/` tests the full deployed application (Go + React + K8s). They have different lifecycles and different audiences.

## Scalability Considerations

| Concern | At v1.1 (now) | At v2.0 (more games/tests) | At v3.0 (multi-cluster) |
|---------|---------------|----------------------------|-------------------------|
| E2E test duration | ~5 min (3-5 test files) | Shard Playwright across workers | Parallel kind clusters |
| Docker build time | ~2 min | Layer caching via `docker/build-push-action` | Pre-built base images |
| Kind cluster startup | ~30s | Same (single node sufficient) | Multi-node kind for HA |
| Test data isolation | Unique usernames per test | Test fixtures with cleanup | Namespace-per-test |
| CI cost | Single `ubuntu-latest` runner | Matrix strategy for browsers | Self-hosted runners |

## Sources

- [Playwright webServer configuration](https://playwright.dev/docs/test-webserver) -- HIGH confidence
- [Playwright CI documentation](https://playwright.dev/docs/ci) -- HIGH confidence
- [Playwright test configuration](https://playwright.dev/docs/test-configuration) -- HIGH confidence
- [kind cluster configuration reference](https://kind.sigs.k8s.io/docs/user/configuration/) -- HIGH confidence
- [kind quick start guide](https://kind.sigs.k8s.io/docs/user/quick-start/) -- HIGH confidence
- [helm/kind-action GitHub Action](https://github.com/helm/kind-action) -- HIGH confidence
- [Kubebuilder envtest reference](https://book.kubebuilder.io/reference/envtest) -- HIGH confidence
- [Kubebuilder writing tests guide](https://book.kubebuilder.io/cronjob-tutorial/writing-tests) -- HIGH confidence
- [Testing Kubernetes operators with GitHub Actions and Kind](https://medium.com/codex/testing-kubernetes-operators-using-github-actions-and-kind-c4086d37dd30) -- MEDIUM confidence
- [Best practices for testing Kubernetes operators](https://wafatech.sa/blog/devops/kubernetes/best-practices-for-testing-kubernetes-operators/) -- MEDIUM confidence
- [Exposing NodePort in kind cluster](https://scriptcrunch.com/expose-nodeport-kind-cluster/) -- MEDIUM confidence
- [Chi router testing patterns and best practices](https://github.com/go-chi/chi/issues/478) -- MEDIUM confidence
- [Testing and deployment patterns for Kubebuilder](https://deepwiki.com/kubernetes-sigs/kubebuilder/6-testing-and-deployment) -- MEDIUM confidence
