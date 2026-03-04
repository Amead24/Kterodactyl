# Phase 14: Go API Integration Tests - Research

**Researched:** 2026-02-18
**Domain:** Go blackbox integration testing with httptest.NewServer for real HTTP round-trips
**Confidence:** HIGH

## Summary

Phase 14 requires a single multi-step integration test that exercises the full API lifecycle as an external consumer: register a user, create a game server, retrieve it, and delete it -- all via real HTTP round-trips against a live `httptest.Server`. The test lives in `test/integration/` as a separate Go package, importing the project's `internal/api` package to construct the server, but otherwise treating the API as a blackbox (no access to unexported symbols).

The project already has all the building blocks needed. The `api.Config` and `api.NewServer()` are exported, and the `Server.HTTPServer()` method returns an `*http.Server` whose `.Handler` field can be passed to `httptest.NewServer()`. Authentication uses JWT tokens returned in JSON responses. The existing unit tests in `internal/api/` demonstrate all the request/response patterns. The integration test reuses the same fake K8s client and auth infrastructure, but wires them up externally and communicates exclusively over HTTP.

The main technical considerations are: (1) rate limiting on the registration endpoint (3 req/min) could interfere with repeated test runs -- a single test function avoids this; (2) the multi-step flow requires careful ordering since each step depends on state from previous steps (invite token -> registration token -> auth token -> server CRUD); (3) the `Makefile` placeholder `test-integration` target needs to be updated to run the test.

**Primary recommendation:** Create a single Go test file `test/integration/api_lifecycle_test.go` with one `TestAPILifecycle` function that runs the full multi-step flow sequentially. Use `httptest.NewServer` with the existing `api.NewServer(cfg).HTTPServer().Handler` pattern. Use `net/http.Client` for all requests. Seed an admin user and invite token via the fake K8s client, then exercise the API purely over HTTP from that point.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| GAPI-04 | Multi-step API flow test validates the full lifecycle: register user -> create server -> get server -> delete server | Single integration test function in `test/integration/api_lifecycle_test.go` exercises this exact flow via real HTTP round-trips against `httptest.NewServer`. Pre-seeds admin + invite via fake client, then all API operations go through HTTP. |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `net/http/httptest` | stdlib | Start real HTTP server on localhost with random port | Standard Go approach for integration-level HTTP testing; provides real TCP connections |
| `net/http` | stdlib | HTTP client for making real requests | Standard Go HTTP client; `http.Client` with `http.NewRequest` |
| `testing` | stdlib | Test framework | Already used throughout the project's API test suite |
| `encoding/json` | stdlib | JSON encoding/decoding of request/response bodies | Standard Go JSON handling |

### Supporting (from project dependencies, already in go.mod)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `sigs.k8s.io/controller-runtime/pkg/client/fake` | v0.23.1 | Fake K8s client for server setup | Wiring up the `api.Config` with a fake backend |
| `github.com/kterodactyl/kterodactyl/internal/api` | local | Server construction (`Config`, `NewServer`, `HTTPServer`) | Building the test server |
| `github.com/kterodactyl/kterodactyl/internal/auth` | local | User/invite/JWT types for pre-seeding admin state | Creating admin user and invite token in fake client before HTTP flow |
| `github.com/kterodactyl/kterodactyl/internal/manifest` | local | Game manifest loader for server creation | Loading minecraft manifest so `CreateGameServer` works |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `httptest.NewServer` (real TCP) | `httptest.NewRecorder` (in-memory) | Recorder skips TCP layer; NewServer validates full HTTP stack including middleware, serialization, headers |
| Standard `testing` | Ginkgo/Gomega | Project's API tests use `testing`; e2e uses Ginkgo but that's for K8s cluster tests. Integration should match API test conventions |
| Single sequential test | Separate subtests per step | Steps are dependent (registration token feeds into auth token feeds into CRUD); separate subtests would need shared state, defeating isolation |
| Build tag `//go:build integration` | Separate `test/integration/` directory | Separate directory achieves isolation and matches success criteria; build tag optional but adds clarity |

## Architecture Patterns

### Recommended Project Structure
```
test/
├── e2e/                        # Existing: Ginkgo e2e tests (Kind cluster)
│   ├── e2e_suite_test.go
│   └── e2e_test.go
├── integration/                # NEW: blackbox integration tests
│   └── api_lifecycle_test.go   # Multi-step API flow test
└── utils/                      # Existing: shared test utilities
    └── utils.go
```

### Pattern 1: httptest.NewServer for Real HTTP Round-Trips
**What:** Start a real HTTP server using `httptest.NewServer`, get its URL, and make requests with `http.Client`.
**When to use:** Integration tests that need to verify the full HTTP stack (TCP, middleware, serialization).
**Example:**
```go
// Source: https://pkg.go.dev/net/http/httptest
srv := api.NewServer(cfg)
ts := httptest.NewServer(srv.HTTPServer().Handler)
defer ts.Close()

// All requests go to ts.URL
resp, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json", body)
```

### Pattern 2: Sequential Multi-Step Flow in a Single Test
**What:** One test function runs the entire lifecycle sequentially, passing state between steps.
**When to use:** When steps are causally dependent (token from step N is needed in step N+1).
**Example:**
```go
func TestAPILifecycle(t *testing.T) {
    // Setup: create httptest.NewServer with fake K8s backend
    // Pre-seed: admin user + invite token (via fake client, not HTTP)

    // Step 1: Register user (POST /api/v1/auth/register) -> get JWT token
    // Step 2: Create game server (POST /api/v1/gameservers) -> get server name
    // Step 3: Get game server (GET /api/v1/gameservers/{name}) -> verify fields
    // Step 4: Delete game server (DELETE /api/v1/gameservers/{name}) -> verify 204
    // Step 5: Verify deleted (GET /api/v1/gameservers/{name}) -> verify 404
}
```

### Pattern 3: Pre-Seeding via Fake Client (Hybrid Setup)
**What:** Use the fake K8s client directly to create pre-conditions (admin user, invite token, admin ConfigMap) that would otherwise require a complex bootstrap flow. Then exercise the API purely over HTTP.
**When to use:** When the test's purpose is to validate the HTTP API flow, not the admin bootstrap flow. Pre-seeding avoids testing unrelated code paths and keeps the test focused.
**Example:**
```go
// Pre-seed admin user in fake K8s client (not via HTTP)
adminUser := &auth.User{
    Username:     "testadmin",
    Email:        "admin@test.com",
    PasswordHash: hashedPassword,
    Role:         auth.RoleAdmin,
    CreatedAt:    time.Now().UTC().Format(time.RFC3339),
}
userStore.CreateUser(ctx, adminUser)

// Pre-seed invite token in fake K8s client
inviteService.CreateInvite(ctx, "newuser@test.com", "testadmin", 72)

// Now test the HTTP flow: register -> create -> get -> delete
```

### Pattern 4: Response Parsing with Typed Structs
**What:** Define local response structs in the test file for JSON decoding. These mirror the API's exported types but are decoupled.
**When to use:** Blackbox testing where you want to validate the JSON contract, not import internal response types.
**Example:**
```go
// Local to the test -- not importing api.GameServerResponse
type gameServerResponse struct {
    Name       string            `json:"name"`
    GameType   string            `json:"gameType"`
    State      string            `json:"state"`
    Parameters map[string]string `json:"parameters,omitempty"`
    CreatedAt  string            `json:"createdAt"`
}
```

**Note:** Since the test is within the same Go module, importing `api.GameServerResponse` directly is also valid. The choice is whether to test the JSON contract (local structs) or reuse types (import). For a blackbox integration test, local structs are more appropriate because they validate what an external consumer would see.

### Anti-Patterns to Avoid
- **Using httptest.NewRecorder instead of httptest.NewServer:** The success criteria specifies "real HTTP round-trips." `NewRecorder` skips TCP entirely. Use `NewServer` for actual network communication.
- **Separate subtests for dependent steps:** `t.Run("register", ...)` then `t.Run("create", ...)` are independent in Go testing. Using subtests for causally dependent steps requires shared state, which is fragile. Use a single function with `t.Helper()` helpers.
- **Pre-seeding everything via fake client:** The register step should go through HTTP to validate the registration endpoint. Only seed what the API cannot create for itself (admin user, invite token, admin ConfigMap).
- **Testing admin flows in this phase:** The requirement is register -> create -> get -> delete. Keep the test focused on this lifecycle. Admin operations (create invite, list users, etc.) are already covered by Phase 13 unit tests.
- **Using `t.Parallel()`:** The test depends on sequential ordering; parallel execution would cause races.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| HTTP test server | Custom TCP listener + goroutine | `httptest.NewServer()` | Handles port selection, TLS optional, cleanup via `defer ts.Close()` |
| JSON request builder | Manual `bytes.Buffer` + `json.Marshal` | Helper function `jsonPost(url, body)` | Reduces boilerplate, ensures Content-Type header |
| JWT token extraction | Manual string parsing | `json.Unmarshal` into `map[string]string` | Token is a JSON field in the response body |
| Fake K8s client | Custom mock implementation | `controller-runtime/pkg/client/fake` | Already proven in Phase 13 tests |
| Invite token creation | Manual Secret construction | `auth.InviteService.CreateInvite()` | Handles token generation, Secret creation, expiration |

**Key insight:** The integration test's value is in exercising the HTTP API contract (routes, middleware, auth, JSON serialization). The backing K8s client can be fake because we're not testing Kubernetes -- we're testing the HTTP API layer.

## Common Pitfalls

### Pitfall 1: Rate Limiting Blocks Test Requests
**What goes wrong:** The registration endpoint (`POST /api/v1/auth/register`) has a rate limit of 3 requests per minute per IP. With `httptest.NewServer`, all requests come from `127.0.0.1`. Re-running the test within a minute or having multiple registration calls fails with 429.
**Why it happens:** `httprate.LimitByIP` is applied in `routes.go` and is active for the test server's router.
**How to avoid:** The test only needs one registration call. A single `TestAPILifecycle` function with one register call stays within limits. If running tests repeatedly, the rate limiter resets per server instance (each `httptest.NewServer` gets a fresh router, so the rate limiter state is fresh too).
**Warning signs:** HTTP 429 responses during test runs.

### Pitfall 2: Missing AdminConfig ConfigMap for Registration
**What goes wrong:** Registration checks `AdminConfig.RegistrationEnabled`. If the ConfigMap is not found, `LoadAdminConfig` returns defaults where `RegistrationEnabled` is `true`. So this is actually fine -- but if someone adds a ConfigMap with `registrationEnabled: false` to the fake client, registration breaks.
**Why it happens:** `LoadAdminConfig` falls back to defaults when ConfigMap is missing, and defaults have `RegistrationEnabled: true`.
**How to avoid:** Either don't create the admin ConfigMap (rely on defaults) or explicitly create one with `registrationEnabled: true`. The simpler approach is to not create it.
**Warning signs:** Registration returns 403 "registration is disabled".

### Pitfall 3: Accessing Unexported Router Field
**What goes wrong:** The `Server.router` field is unexported. You cannot do `server.router` from the `test/integration` package.
**Why it happens:** Go's visibility rules -- lowercase fields are package-private.
**How to avoid:** Use `srv.HTTPServer().Handler` which returns the router via the exported `*http.Server`. This is the designed API surface.
**Warning signs:** Compile error "server.router undefined (cannot refer to unexported field)".

### Pitfall 4: Timeout Middleware Interfering with Tests
**What goes wrong:** The REST API routes have a 30-second timeout middleware. This is fine for normal operations but could theoretically interfere with debugging slow tests.
**Why it happens:** `middleware.Timeout(30 * time.Second)` is applied to all authenticated REST routes.
**How to avoid:** Not a real problem for integration tests -- requests complete in milliseconds with a fake K8s client. Just be aware it exists.
**Warning signs:** Context deadline exceeded errors during debugging.

### Pitfall 5: SPA Catch-All Masking 404s
**What goes wrong:** The router has `r.NotFound(serveSPA().ServeHTTP)` which serves the embedded frontend for unmatched routes. API routes return proper 404 JSON, but if you hit a wrong URL pattern (e.g., `/api/v2/...`), you get an HTML response instead of a JSON 404.
**Why it happens:** The SPA catch-all serves `index.html` for any route not matched by API handlers.
**How to avoid:** Ensure test URLs match the exact API route patterns (`/api/v1/...`). Validate that responses have `Content-Type: application/json`.
**Warning signs:** Getting 200 with HTML content when expecting a JSON error response.

### Pitfall 6: Manifest Loader Must Be Initialized
**What goes wrong:** `handleCreateGameServer` calls `s.manifestLoader.Get(req.GameType)`. If the manifest loader is nil or has no manifests, creation fails with "unknown game type".
**Why it happens:** The manifest loader needs to be initialized with at least one game manifest (e.g., minecraft) for server creation to work.
**How to avoid:** The existing `defaultTestManifestLoader(t)` from `helpers_test.go` creates a minecraft manifest. Since that's in `package api` (unexported helper), the integration test must replicate this setup. Create a temp directory with a `minecraft/manifest.yaml` and call `manifest.LoadFromDirectory()`.
**Warning signs:** 400 "unknown game type: minecraft" when creating a server.

## Code Examples

### Example 1: Test Server Setup with httptest.NewServer
```go
// Source: project code analysis + https://pkg.go.dev/net/http/httptest
func setupTestServer(t *testing.T) (*httptest.Server, *auth.InviteService) {
    t.Helper()

    signingKey := []byte("test-signing-key-32-bytes-long!!")
    jwtSvc := auth.NewJWTService(signingKey, 24*time.Hour)

    scheme := runtime.NewScheme()
    _ = corev1.AddToScheme(scheme)
    _ = gamev1alpha1.AddToScheme(scheme)
    fakeClient := fake.NewClientBuilder().
        WithScheme(scheme).
        WithStatusSubresource(&gamev1alpha1.GameServer{}).
        Build()

    inviteSvc := auth.NewInviteService(fakeClient, "kterodactyl-system", nil, "https://panel.test")
    userStore := auth.NewUserStore(fakeClient, "kterodactyl-system")
    loader := createTestManifestLoader(t)

    srv := api.NewServer(api.Config{
        Client:            fakeClient,
        JWTService:        jwtSvc,
        UserStore:         userStore,
        InviteService:     inviteSvc,
        ManifestLoader:    loader,
        OperatorNamespace: "kterodactyl-system",
        BindAddress:       ":0",
    })

    ts := httptest.NewServer(srv.HTTPServer().Handler)
    t.Cleanup(ts.Close)

    return ts, inviteSvc
}
```

### Example 2: Multi-Step Lifecycle Test Flow
```go
func TestAPILifecycle(t *testing.T) {
    ts, inviteSvc := setupTestServer(t)
    client := ts.Client()

    // Pre-seed: create invite token for registration
    ctx := context.Background()
    invite, err := inviteSvc.CreateInvite(ctx, "alice@test.com", "bootstrap", 72)
    if err != nil {
        t.Fatalf("failed to create invite: %v", err)
    }

    // Step 1: Register user
    regBody := map[string]string{
        "username":    "alice",
        "email":       "alice@test.com",
        "password":    "securepassword123",
        "inviteToken": invite.Token,
    }
    resp := jsonPost(t, client, ts.URL+"/api/v1/auth/register", regBody)
    assertStatus(t, resp, http.StatusCreated)
    regResult := decodeJSON(t, resp)
    token := regResult["token"].(string)
    if token == "" {
        t.Fatal("expected JWT token from registration")
    }

    // Step 2: Create game server
    createBody := map[string]string{
        "name":     "my-mc-server",
        "gameType": "minecraft",
    }
    resp = jsonPostAuth(t, client, ts.URL+"/api/v1/gameservers", createBody, token)
    assertStatus(t, resp, http.StatusCreated)
    createResult := decodeJSON(t, resp)
    if createResult["name"] != "my-mc-server" {
        t.Errorf("expected name 'my-mc-server', got %v", createResult["name"])
    }

    // Step 3: Get game server
    resp = jsonGetAuth(t, client, ts.URL+"/api/v1/gameservers/my-mc-server", token)
    assertStatus(t, resp, http.StatusOK)
    getResult := decodeJSON(t, resp)
    if getResult["gameType"] != "minecraft" {
        t.Errorf("expected gameType 'minecraft', got %v", getResult["gameType"])
    }

    // Step 4: Delete game server
    resp = jsonDeleteAuth(t, client, ts.URL+"/api/v1/gameservers/my-mc-server", token)
    assertStatus(t, resp, http.StatusNoContent)

    // Step 5: Verify deleted
    resp = jsonGetAuth(t, client, ts.URL+"/api/v1/gameservers/my-mc-server", token)
    assertStatus(t, resp, http.StatusNotFound)
}
```

### Example 3: HTTP Helper Functions for Clean Test Code
```go
func jsonPost(t *testing.T, client *http.Client, url string, body map[string]string) *http.Response {
    t.Helper()
    b, err := json.Marshal(body)
    if err != nil {
        t.Fatalf("failed to marshal body: %v", err)
    }
    resp, err := client.Post(url, "application/json", bytes.NewReader(b))
    if err != nil {
        t.Fatalf("POST %s failed: %v", url, err)
    }
    return resp
}

func jsonPostAuth(t *testing.T, client *http.Client, url string, body map[string]string, token string) *http.Response {
    t.Helper()
    b, err := json.Marshal(body)
    if err != nil {
        t.Fatalf("failed to marshal body: %v", err)
    }
    req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
    if err != nil {
        t.Fatalf("failed to create request: %v", err)
    }
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+token)
    resp, err := client.Do(req)
    if err != nil {
        t.Fatalf("POST %s failed: %v", url, err)
    }
    return resp
}

func assertStatus(t *testing.T, resp *http.Response, expected int) {
    t.Helper()
    if resp.StatusCode != expected {
        body, _ := io.ReadAll(resp.Body)
        t.Fatalf("expected status %d, got %d: %s", expected, resp.StatusCode, string(body))
    }
}

func decodeJSON(t *testing.T, resp *http.Response) map[string]interface{} {
    t.Helper()
    defer resp.Body.Close()
    var result map[string]interface{}
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        t.Fatalf("failed to decode JSON response: %v", err)
    }
    return result
}
```

### Example 4: Manifest Loader Setup for Integration Test
```go
func createTestManifestLoader(t *testing.T) *manifest.Loader {
    t.Helper()
    dir := t.TempDir()
    mcDir := filepath.Join(dir, "minecraft")
    if err := os.MkdirAll(mcDir, 0755); err != nil {
        t.Fatal(err)
    }
    minecraft := `name: minecraft
displayName: Minecraft Java Edition
image: itzg/minecraft-server:latest
ports:
  - name: game
    containerPort: 25565
    protocol: TCP
parameters:
  EULA: "TRUE"
  TYPE: VANILLA
resources:
  requests:
    memory: "1Gi"
    cpu: "500m"
  limits:
    memory: "2Gi"
    cpu: "1000m"
parameterSchema:
  type: object
  properties:
    EULA:
      type: string
      const: "TRUE"
    TYPE:
      type: string
      enum: ["VANILLA", "PAPER", "SPIGOT"]
      default: "VANILLA"
  required:
    - EULA
    - TYPE
`
    if err := os.WriteFile(filepath.Join(mcDir, "manifest.yaml"), []byte(minecraft), 0644); err != nil {
        t.Fatal(err)
    }
    loader, err := manifest.LoadFromDirectory(dir)
    if err != nil {
        t.Fatalf("failed to load manifests: %v", err)
    }
    return loader
}
```

### Example 5: Makefile Target Update
```makefile
.PHONY: test-integration
test-integration: ## Run integration tests.
	go test ./test/integration/... -v -count=1
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `httptest.NewRecorder` for all HTTP tests | `httptest.NewRecorder` for unit tests, `httptest.NewServer` for integration | Go community best practice | Integration tests catch middleware, serialization, and routing issues that recorder tests miss |
| Build tags for test separation | Separate directories + build tags | Go module conventions | `test/integration/` as a package provides clear separation without requiring flags |

**Deprecated/outdated:**
- None relevant to this phase. Standard library `httptest` has been stable for years.

## Makefile Integration

The existing `make test-integration` placeholder:
```makefile
.PHONY: test-integration
test-integration: ## Run integration tests.
	@echo "No integration tests yet -- see Phase 14"
```

Should become:
```makefile
.PHONY: test-integration
test-integration: ## Run integration tests.
	go test ./test/integration/... -v -count=1
```

**Note:** No build prerequisites are needed (`manifests`, `generate`, `fmt`, `vet`, etc.) because integration tests only use fake K8s clients. The existing `make test` command already filters out `e2e` but includes everything else. Since `test/integration/` is a standard Go package, `make test` would also pick it up via `go list ./...`. To keep it separate, either:
- Add `| grep -v /integration` to the existing `make test` command
- Or add a `//go:build integration` build tag to the integration test files

**Recommendation:** Use a `//go:build integration` build tag. This is the cleanest approach: `make test` runs only unit tests (no tag), `make test-integration` runs with `-tags integration`, and there's no need to modify the existing `make test` command.

```makefile
.PHONY: test-integration
test-integration: ## Run integration tests.
	go test -tags integration ./test/integration/... -v -count=1
```

## Open Questions

1. **Build tag vs directory-only separation**
   - What we know: The success criteria says "lives in `test/integration/`". The e2e tests use `//go:build e2e`. Both approaches work.
   - What's unclear: Whether `make test` (which runs `go list ./... | grep -v /e2e`) would unintentionally pick up integration tests.
   - Recommendation: Use `//go:build integration` build tag to match the e2e convention and prevent `make test` from running integration tests. Update `make test-integration` to pass `-tags integration`.

2. **Should the test also exercise login (not just register)?**
   - What we know: GAPI-04 says "register user -> create server -> get server -> delete server". Login is not explicitly required.
   - What's unclear: Whether adding a login step after registration adds value.
   - Recommendation: The registration response already returns a JWT token. Use that token for subsequent steps. Optionally add a login step to verify the registered user can log in, but this is not required by GAPI-04.

3. **Response body validation depth**
   - What we know: Phase 13 used status-code-only for error cases. Happy paths validated response structure.
   - What's unclear: How deeply should the integration test validate response bodies?
   - Recommendation: Validate key fields (name, gameType, token presence) but not exhaustive field-by-field comparison. The integration test proves the flow works end-to-end; unit tests cover field-level details.

## Sources

### Primary (HIGH confidence)
- Project source code: `/home/tony/kterodactyl/internal/api/server.go` -- `Config`, `NewServer`, `HTTPServer` exports
- Project source code: `/home/tony/kterodactyl/internal/api/routes.go` -- full route tree with middleware
- Project source code: `/home/tony/kterodactyl/internal/api/handlers_auth.go` -- register/login response format
- Project source code: `/home/tony/kterodactyl/internal/api/handlers_gameserver.go` -- CRUD handler patterns
- Project source code: `/home/tony/kterodactyl/internal/api/helpers_test.go` -- existing test infrastructure pattern
- Project source code: `/home/tony/kterodactyl/internal/api/request.go` -- exported request types
- Project source code: `/home/tony/kterodactyl/internal/auth/` -- JWT, UserStore, InviteService exports
- Project source code: `/home/tony/kterodactyl/Makefile` -- existing test-integration placeholder
- [Go httptest package](https://pkg.go.dev/net/http/httptest) -- `NewServer` API documentation
- [Go httptest integration testing patterns](https://speedscale.com/blog/testing-golang-with-httptest/) -- best practices

### Secondary (MEDIUM confidence)
- Project source code: `/home/tony/kterodactyl/test/e2e/` -- build tag convention (`//go:build e2e`)
- [Learn Go with tests - HTTP server testing](https://quii.gitbook.io/learn-go-with-tests/build-an-application/http-server) -- community patterns

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- stdlib `httptest.NewServer` + existing project dependencies; no new libraries needed
- Architecture: HIGH -- clear pattern from project's existing test structure (e2e, unit tests) and Go conventions
- Pitfalls: HIGH -- identified from direct code inspection (rate limiting, unexported fields, manifest loader, AdminConfig defaults)
- Makefile targets: HIGH -- simple change to existing placeholder

**Research date:** 2026-02-18
**Valid until:** 2026-03-18 (stable domain, no fast-moving dependencies)
