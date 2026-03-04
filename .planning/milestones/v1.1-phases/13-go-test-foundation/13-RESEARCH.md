# Phase 13: Go Test Foundation - Research

**Researched:** 2026-02-18
**Domain:** Go unit testing for HTTP API handlers with fake Kubernetes clients
**Confidence:** HIGH

## Summary

Phase 13 adds unit tests for three untested API handler groups: mods (`handlers_mods.go`), backups (`handlers_backups.go`), and metrics proxy (`handlers_metrics.go`). The project already has a well-established test pattern in `helpers_test.go` and existing test files (`handlers_admin_test.go`, `handlers_auth_test.go`, `handlers_games_test.go`, `handlers_gameserver_test.go`) that use `httptest`, a fake `controller-runtime` client, JWT-based auth helpers, and chi router integration. The new tests follow this existing pattern.

The main technical challenges are: (1) the mod and backup handlers depend on `execInPod` (SPDY exec) and `createS3Client` (minio) which cannot be faked through the existing `controller-runtime/client/fake` -- these require either interface extraction or testing only the pre-exec/pre-S3 validation paths; (2) the metrics handler depends on `metricsClient` which is a concrete `*metricsv.Clientset` and has a known fake clientset available at `k8s.io/metrics/pkg/client/clientset/versioned/fake`; (3) the envtest `suite_test.go` in the controller package uses `client.New()` instead of the manager's cached client, which is a separate concern from the API handler tests but needs fixing per phase requirements.

**Primary recommendation:** Follow the existing `helpers_test.go` pattern exactly. Register `&gamev1alpha1.Backup{}` in `WithStatusSubresource()`. For handlers that call `execInPod` or S3, test the validation-layer paths (auth, not-found, wrong-state) via HTTP status codes and leave the exec/S3 code paths for integration tests. For the metrics handler, use `k8s.io/metrics/pkg/client/clientset/versioned/fake.NewSimpleClientset()` or test the nil-metricsClient path. Add Makefile targets `test`, `test-integration`, `test-e2e`, and `test-playwright` with proper separation.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- Test both happy paths AND error cases for all three handler groups (mod, backup, metrics proxy)
- Scope is strictly those three handler groups -- no audit of other handlers
- Error case assertions use HTTP status codes only -- do not couple tests to specific error message wording
- For file-handling endpoints (mod upload, backup restore), use mock/pre-built request bodies rather than real multipart form data -- tests exercise handler logic, not HTTP parsing

### Claude's Discretion
- Test output verbosity and filtering approach
- Envtest cached-client fix strategy (minimal fix vs broader cleanup)
- Fake boundary decisions (what to fake for K8s client, S3, filesystem)
- Makefile target naming and test execution workflow
- Test file organization and naming conventions

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| INFRA-03 | Developer can run each test tier independently (make test, make test-integration, make test-e2e, make test-playwright) | Makefile targets section below details the split; existing `make test` already runs unit tests, needs refinement to exclude integration; new targets for each tier |
| INFRA-04 | Each test creates and cleans up its own resources without leaking state to other tests | Existing pattern: each subtest calls `newTestServer(t)` creating a fresh fake client -- zero shared state; backup tests need fresh Backup CRs per subtest |
| GAPI-01 | Mod handler endpoints have httptest-based tests covering upload and list flows | `handlers_mods_test.go` tests for handleListMods and handleUploadMod; upload tested via validation paths (server not found, wrong state, no mod path); list tested via validation paths (exec is external) |
| GAPI-02 | Backup handler endpoints have httptest-based tests covering create, list, and restore flows | `handlers_backups_test.go` tests for handleCreateBackup (happy path creates Backup CR), handleListBackups (returns sorted list), handleRestoreBackup (validation paths); S3/exec paths are integration scope |
| GAPI-03 | Metrics proxy handler has httptest-based tests | `handlers_metrics_test.go` tests for handleGetMetrics with nil metricsClient (503), server not found (404), and optionally with fake metrics clientset for happy path |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `net/http/httptest` | stdlib | HTTP test server and recorder | Standard Go approach, already used in project |
| `sigs.k8s.io/controller-runtime/pkg/client/fake` | v0.23.1 (in go.mod) | Fake K8s client for CRD operations | Already used in existing tests; handles Create/Get/List/Update/Delete/Status |
| `testing` | stdlib | Test framework | Standard Go, already used throughout project |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `k8s.io/metrics/pkg/client/clientset/versioned/fake` | v0.35.1 (in go.mod) | Fake metrics API clientset | Metrics handler tests that need PodMetrics data |
| `k8s.io/apimachinery/pkg/runtime` | v0.35.1 | Scheme registration for fake clients | Required to register Backup type with fake client |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Fake K8s client | envtest (real API server) | envtest is heavyweight, slow startup; fake client is instant, already proven in this project |
| Status code-only assertions | Full response body assertions | User decided status-code-only for error cases; body assertions for happy paths where structure matters |
| Interface extraction for execInPod/S3 | Test only validation paths | Interface extraction is larger refactor; validation-path testing covers the same handler logic without touching exec/S3 plumbing |

## Architecture Patterns

### Recommended Test File Organization
```
internal/api/
  handlers_mods_test.go      # NEW: mod upload, list, delete tests
  handlers_backups_test.go    # NEW: backup create, list, delete, restore, schedule tests
  handlers_metrics_test.go    # NEW: metrics proxy tests
  helpers_test.go             # EXISTING: shared test helpers (minor additions)
  handlers_gameserver_test.go # EXISTING: reference pattern
  handlers_admin_test.go      # EXISTING: reference pattern
  handlers_games_test.go      # EXISTING: reference pattern
  handlers_auth_test.go       # EXISTING: reference pattern
```

### Pattern 1: Per-Subtest Fresh Server (Existing Pattern)
**What:** Each `t.Run()` subtest calls `newTestServer(t)` to get a completely isolated fake K8s client and router.
**When to use:** Every test -- this is the established pattern.
**Example:**
```go
// Source: /home/tony/kterodactyl/internal/api/handlers_gameserver_test.go
func TestHandleCreateBackup(t *testing.T) {
    t.Run("creates backup for ready server", func(t *testing.T) {
        ts := newTestServer(t)
        token := ts.generateToken(t, "alice", auth.RoleUser)
        // ... create GameServer, make request, assert
    })
    t.Run("returns 404 for non-existent server", func(t *testing.T) {
        ts := newTestServer(t) // fresh state, zero leakage
        // ...
    })
}
```

### Pattern 2: GameServer Pre-Creation with State
**What:** Use `createTestGameServerWithState()` to set up a GameServer in a specific lifecycle state before testing handlers that require Running/Ready state.
**When to use:** Mod, backup, and metrics handlers all verify the GameServer state before proceeding.
**Example:**
```go
// Source: /home/tony/kterodactyl/internal/api/handlers_gameserver_test.go
createTestGameServerWithState(t, ts.client, "mc-server", "user-alice", "alice", "minecraft", gamev1alpha1.GameServerStateReady)
```

### Pattern 3: Backup CRD Test Helper
**What:** A new helper `createTestBackup()` to pre-populate Backup CRs with specific states for list/restore/delete tests.
**When to use:** Backup list, restore, and delete tests need pre-existing Backup objects.
**Example:**
```go
func createTestBackup(t *testing.T, k8sClient client.Client, name, namespace, gsName string, state gamev1alpha1.BackupState) {
    t.Helper()
    backup := &gamev1alpha1.Backup{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name,
            Namespace: namespace,
            Labels: map[string]string{
                util.LabelBackupGameServer: gsName,
                util.LabelManagedBy:        util.ManagedByValue,
            },
        },
        Spec: gamev1alpha1.BackupSpec{GameServerName: gsName},
    }
    if err := k8sClient.Create(t.Context(), backup); err != nil {
        t.Fatalf("failed to create test backup: %v", err)
    }
    if state != "" {
        backup.Status.State = state
        if err := k8sClient.Status().Update(t.Context(), backup); err != nil {
            t.Fatalf("failed to set backup state: %v", err)
        }
    }
}
```

### Pattern 4: GameServer with Annotations
**What:** A helper to create GameServers with mod/backup path annotations set, since mod and backup handlers check these annotations.
**When to use:** Mod and backup restore handlers check `AnnotationModPath` and `AnnotationBackupPath`.
**Example:**
```go
func createTestGameServerWithAnnotations(t *testing.T, k8sClient client.Client, name, namespace, owner, gameType string, state gamev1alpha1.GameServerState, annotations map[string]string) {
    t.Helper()
    createTestGameServerWithState(t, k8sClient, name, namespace, owner, gameType, state)
    gs := &gamev1alpha1.GameServer{}
    if err := k8sClient.Get(t.Context(), client.ObjectKey{Name: name, Namespace: namespace}, gs); err != nil {
        t.Fatalf("failed to get gs: %v", err)
    }
    gs.Annotations = annotations
    if err := k8sClient.Update(t.Context(), gs); err != nil {
        t.Fatalf("failed to update annotations: %v", err)
    }
}
```

### Anti-Patterns to Avoid
- **Shared test state across subtests:** Never reuse a `testServer` between `t.Run()` calls. Each subtest gets its own via `newTestServer(t)`.
- **Asserting on error message text:** User decision: error assertions use HTTP status codes only. Do not write `if resp.Error != "specific text"` for error cases.
- **Testing exec/S3 plumbing in unit tests:** The `execInPod()` and `createS3Client()` functions require real K8s/S3 backends. Unit tests cover the handler validation logic up to the point where exec/S3 is called.
- **Using `t.Parallel()` with shared fake clients:** The fake client is not safe for concurrent writes from multiple goroutines.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Fake K8s client | Custom mock struct | `controller-runtime/pkg/client/fake` | Already used in project; handles scheme, status subresource, list filtering |
| Fake metrics API client | Custom mock for PodMetricses | `k8s.io/metrics/pkg/client/clientset/versioned/fake` | Official fake; supports Get/List; known resource name issue has workaround |
| JWT test tokens | Manual token construction | `testServer.generateToken()` | Already exists in helpers_test.go |
| HTTP request/response testing | Manual net.Conn manipulation | `httptest.NewRequest()` + `httptest.NewRecorder()` | Standard Go pattern, already established |

**Key insight:** The project has an excellent test infrastructure in `helpers_test.go`. The primary work is writing new test files that follow the established patterns, not building new infrastructure.

## Common Pitfalls

### Pitfall 1: Missing WithStatusSubresource for Backup Type
**What goes wrong:** `client.Status().Update()` silently does nothing if the type is not registered with `WithStatusSubresource()`.
**Why it happens:** The existing `newTestServer()` only registers `&gamev1alpha1.GameServer{}` with `WithStatusSubresource()`. Backup handler tests need `&gamev1alpha1.Backup{}` registered too.
**How to avoid:** Update the fake client builder in `newTestServer()` to include `WithStatusSubresource(&gamev1alpha1.GameServer{}, &gamev1alpha1.Backup{})`.
**Warning signs:** Tests pass but backup status state is always empty after `Status().Update()`.

### Pitfall 2: Metrics Fake Clientset Resource Name Mismatch
**What goes wrong:** `fake.NewSimpleClientset()` for metrics registers PodMetrics under resource name "podmetricses" but the typed client queries resource "pods".
**Why it happens:** Known k8s.io/metrics issue -- the resource name in the scheme doesn't match what the fake tracker expects.
**How to avoid:** Two options: (a) Use `PrependReactor` on the fake clientset to intercept Get calls and return the desired PodMetrics object, or (b) test only the nil-metricsClient path (503 response) and the GameServer-not-found path (404) which don't need the metrics fake. Option (b) is simpler and covers the handler logic; option (a) covers the happy path but requires more plumbing.
**Warning signs:** Metrics Get() returns NotFound even though you added objects to the fake.

### Pitfall 3: Envtest Cached vs Direct Client
**What goes wrong:** Controller suite_test.go uses `client.New()` (direct, non-cached client) for `k8sClient`, while the reconciler uses `mgr.GetClient()` (cached client). The test client reads bypass the cache, causing inconsistencies.
**Why it happens:** Original Kubebuilder scaffold uses `client.New()`. The manager's cached client has eventual consistency -- writes via the direct client may not be visible to the cached client immediately.
**How to avoid:** Minimal fix: change `k8sClient` to use `mgr.GetClient()` in `suite_test.go`. This ensures the test client and the reconciler see the same cache. This is a one-line change.
**Warning signs:** Flaky controller tests where status updates are not visible, or Eventually blocks timeout intermittently.

### Pitfall 4: Testing Upload/Restore via Handler's Full Code Path
**What goes wrong:** `handleUploadMod` calls `execInPod()` which requires a real `clientset` (SPDY exec), not a fake `controller-runtime` client. Similarly, `handleRestoreBackup` calls `createS3Client()` and `execInPod()`.
**Why it happens:** These are infrastructure-level operations that talk to real Kubernetes API servers and S3 endpoints.
**How to avoid:** Test the validation paths (server not found, wrong state, no mod path annotation, backup not completed, etc.) which exercise the handler logic before it reaches exec/S3. These paths return well-defined HTTP status codes. The exec/S3 paths are integration test scope.
**Warning signs:** Tests panic or fail with nil pointer dereference on `s.clientset` or `s.restConfig`.

### Pitfall 5: Admin-Only Routes Without Admin Token
**What goes wrong:** Backup delete, backup restore, and backup schedule routes have `auth.RequireAdmin` middleware. Tests using regular user tokens get 403.
**Why it happens:** Route definition in `routes.go` applies `RequireAdmin` to these endpoints.
**How to avoid:** Use `ts.generateToken(t, "admin", auth.RoleAdmin)` for admin-only endpoint tests. Include a "non-admin gets 403" error case test.
**Warning signs:** Tests for delete/restore/schedule endpoints always get 403.

## Code Examples

### Example 1: Backup Create Happy Path Test
```go
// Pattern based on: /home/tony/kterodactyl/internal/api/handlers_gameserver_test.go
func TestHandleCreateBackup(t *testing.T) {
    t.Run("creates backup for ready server", func(t *testing.T) {
        ts := newTestServer(t)
        token := ts.generateToken(t, "alice", auth.RoleUser)

        createTestGameServerWithState(t, ts.client, "mc-srv", "user-alice", "alice", "minecraft",
            gamev1alpha1.GameServerStateReady)

        req := httptest.NewRequest(http.MethodPost, "/api/v1/gameservers/mc-srv/backups", nil)
        addAuthHeader(req, token)
        rec := ts.doRequest(req)

        if rec.Code != http.StatusCreated {
            t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body.String())
        }

        var resp BackupResponse
        if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
            t.Fatalf("decode: %v", err)
        }
        if resp.GameServerName != "mc-srv" {
            t.Errorf("gameServerName = %q, want %q", resp.GameServerName, "mc-srv")
        }
    })

    t.Run("server not found returns 404", func(t *testing.T) {
        ts := newTestServer(t)
        token := ts.generateToken(t, "alice", auth.RoleUser)

        req := httptest.NewRequest(http.MethodPost, "/api/v1/gameservers/nonexistent/backups", nil)
        addAuthHeader(req, token)
        rec := ts.doRequest(req)

        if rec.Code != http.StatusNotFound {
            t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
        }
    })
}
```

### Example 2: Metrics Handler Test (Nil Client Path)
```go
func TestHandleGetMetrics(t *testing.T) {
    t.Run("metrics unavailable returns 503", func(t *testing.T) {
        ts := newTestServer(t) // metricsClient is nil by default
        token := ts.generateToken(t, "alice", auth.RoleUser)

        createTestGameServerWithState(t, ts.client, "mc-srv", "user-alice", "alice", "minecraft",
            gamev1alpha1.GameServerStateReady)

        req := httptest.NewRequest(http.MethodGet, "/api/v1/gameservers/mc-srv/metrics", nil)
        addAuthHeader(req, token)
        rec := ts.doRequest(req)

        if rec.Code != http.StatusServiceUnavailable {
            t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
        }
    })

    t.Run("server not found returns 404", func(t *testing.T) {
        ts := newTestServer(t)
        token := ts.generateToken(t, "alice", auth.RoleUser)

        req := httptest.NewRequest(http.MethodGet, "/api/v1/gameservers/nonexistent/metrics", nil)
        addAuthHeader(req, token)
        rec := ts.doRequest(req)

        if rec.Code != http.StatusNotFound {
            t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
        }
    })
}
```

### Example 3: Mod List Test (Validation Path)
```go
func TestHandleListMods(t *testing.T) {
    t.Run("server not found returns 404", func(t *testing.T) {
        ts := newTestServer(t)
        token := ts.generateToken(t, "alice", auth.RoleUser)

        req := httptest.NewRequest(http.MethodGet, "/api/v1/gameservers/nonexistent/mods", nil)
        addAuthHeader(req, token)
        rec := ts.doRequest(req)

        if rec.Code != http.StatusNotFound {
            t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
        }
    })

    t.Run("server not running returns 409", func(t *testing.T) {
        ts := newTestServer(t)
        token := ts.generateToken(t, "alice", auth.RoleUser)

        createTestGameServerWithState(t, ts.client, "mc-srv", "user-alice", "alice", "minecraft",
            gamev1alpha1.GameServerStateShutdown)

        req := httptest.NewRequest(http.MethodGet, "/api/v1/gameservers/mc-srv/mods", nil)
        addAuthHeader(req, token)
        rec := ts.doRequest(req)

        if rec.Code != http.StatusConflict {
            t.Errorf("status = %d, want %d", rec.Code, http.StatusConflict)
        }
    })

    t.Run("no mod path returns 400", func(t *testing.T) {
        ts := newTestServer(t)
        token := ts.generateToken(t, "alice", auth.RoleUser)

        // GameServer is Ready but has no mod path annotation
        createTestGameServerWithState(t, ts.client, "mc-srv", "user-alice", "alice", "minecraft",
            gamev1alpha1.GameServerStateReady)

        req := httptest.NewRequest(http.MethodGet, "/api/v1/gameservers/mc-srv/mods", nil)
        addAuthHeader(req, token)
        rec := ts.doRequest(req)

        if rec.Code != http.StatusBadRequest {
            t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
        }
    })
}
```

### Example 4: Envtest Cached-Client Fix
```go
// In internal/controller/suite_test.go, change:
//   k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
// To:
//   k8sClient = mgr.GetClient()
// This ensures the test client uses the same cache as the reconciler.
```

## Makefile Targets

### Recommended Target Structure

The existing `make test` already runs unit tests (excluding e2e). The changes needed:

```makefile
# Existing target -- no change needed for basic unit tests:
.PHONY: test
test: manifests generate fmt vet setup-envtest ## Run unit + controller tests.
	KUBEBUILDER_ASSETS="..." go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

# New target -- integration tests (placeholder, no integration tests yet):
.PHONY: test-integration
test-integration: ## Run integration tests.
	@echo "No integration tests yet"

# Existing target -- already present:
.PHONY: test-e2e
test-e2e: setup-test-e2e manifests generate fmt vet ## Run e2e tests with Kind.
	...

# New target -- Playwright tests (placeholder):
.PHONY: test-playwright
test-playwright: ## Run Playwright browser tests.
	@echo "No Playwright tests yet"
```

**Key observation:** The existing `make test` command already satisfies INFRA-03 for unit tests. The main additions are `test-integration` and `test-playwright` placeholder targets so that `make test-integration` and `make test-playwright` exist and succeed (even if empty). The `-count=1` flag should be added for cache-busting during development.

### Test Filtering Recommendation
For development workflows, recommend `-v -run TestHandle` patterns:
```bash
# Run only backup handler tests
go test ./internal/api/... -v -run TestHandleCreateBackup
# Run all handler tests with verbose output
go test ./internal/api/... -v -count=1
```

## Fake Boundary Decisions (Discretion Area)

### What to Fake

| Dependency | Fake Strategy | Rationale |
|------------|---------------|-----------|
| K8s CRD client (`s.client`) | `controller-runtime/pkg/client/fake` | Already established; handles GameServer + Backup CRUD |
| Metrics API (`s.metricsClient`) | `nil` (test 503 path) + optionally `k8s.io/metrics/fake` with reactor | Simplest approach covers handler logic; reactor approach covers happy path |
| Pod exec (`s.execInPod` via `s.clientset`) | Not faked -- test validation paths only | Requires SPDY executor; too deep for unit tests |
| S3 client (`createS3Client`) | Not faked -- test validation paths only | Requires real or mocked S3 endpoint; integration scope |
| JWT/Auth | Existing `generateToken()` + `addAuthHeader()` | Already established |
| AdminConfig (for restore) | ConfigMap in fake client | `loadAdminConfig` reads from ConfigMap; fake client can hold it |

### Recommendation
Use the nil-metricsClient approach for the metrics handler as the primary strategy. This tests the most important handler logic paths (auth, GameServer lookup, nil-client guard). If the planner wants the happy path too, add a reactor-based approach as an optional enhancement.

## Envtest Fix Strategy (Discretion Area)

### Recommendation: Minimal Fix

The issue: `internal/controller/suite_test.go` line 107 creates `k8sClient` via `client.New()` (direct, non-cached). The reconciler uses `mgr.GetClient()` (cached). They see different views of the data.

**Fix:** Replace `client.New()` with `mgr.GetClient()`. This is a one-line change that must happen AFTER the manager is created (move `k8sClient` assignment below `mgr` creation). The fix looks like:

```go
// Before (line 107):
k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})

// After (move below mgr creation, ~line 127):
k8sClient = mgr.GetClient()
```

This is the minimal fix. A broader cleanup (removing unused `cfg` variable, etc.) is not needed for this phase.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Manual mock structs for K8s client | `controller-runtime/pkg/client/fake` with `WithStatusSubresource` | controller-runtime v0.15.0 | Must explicitly register status subresource types |
| `envtest` for all tests | Fake client for unit tests, envtest for integration | Community best practice | Faster test execution, better isolation |

**Deprecated/outdated:**
- `k8s.io/metrics/pkg/client/clientset/versioned/fake.NewSimpleClientset()`: Deprecated in v0.35.1 in favor of `NewClientset()`. However, since the project uses v0.35.1, `NewSimpleClientset` still works and is the simpler option.

## Open Questions

1. **Metrics happy-path testing depth**
   - What we know: The fake metrics clientset has a known resource name mismatch bug. A reactor workaround exists.
   - What's unclear: Whether the reactor approach is worth the complexity for a single happy-path test.
   - Recommendation: Start with nil-metricsClient tests (covers auth + GameServer lookup + nil guard). Add reactor-based happy path only if team wants full coverage. LOW priority.

2. **parseLsOutput unit test**
   - What we know: `parseLsOutput()` in `handlers_mods.go` is a pure function that parses `ls -la` output.
   - What's unclear: Whether testing this pure function is in-scope (it's not a handler endpoint).
   - Recommendation: Include it -- it's a free win for coverage and is in the mod handler file. Pure function tests are trivially isolated.

## Sources

### Primary (HIGH confidence)
- Project source code: `/home/tony/kterodactyl/internal/api/helpers_test.go` -- existing test infrastructure pattern
- Project source code: `/home/tony/kterodactyl/internal/api/handlers_gameserver_test.go` -- established test patterns
- Project source code: `/home/tony/kterodactyl/internal/api/handlers_mods.go`, `handlers_backups.go`, `handlers_metrics.go` -- target handler code
- Project source code: `/home/tony/kterodactyl/internal/controller/suite_test.go` -- envtest cached-client issue
- Project source code: `/home/tony/kterodactyl/Makefile` -- existing test targets

### Secondary (MEDIUM confidence)
- [controller-runtime fake client package](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/client/fake) -- WithStatusSubresource behavior
- [k8s.io/metrics fake clientset package](https://pkg.go.dev/k8s.io/metrics/pkg/client/clientset/versioned/fake) -- NewSimpleClientset API
- [k8s.io/metrics issue #37](https://github.com/kubernetes/metrics/issues/37) -- fake clientset resource name mismatch

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- project already has the exact pattern; no new libraries needed
- Architecture: HIGH -- following existing test file organization and patterns; additions are straightforward
- Pitfalls: HIGH -- identified from direct code inspection (missing WithStatusSubresource, nil clientset dereference, admin-only routes)
- Makefile targets: HIGH -- simple additions to existing Makefile with clear separation

**Research date:** 2026-02-18
**Valid until:** 2026-03-18 (stable domain, no fast-moving dependencies)
