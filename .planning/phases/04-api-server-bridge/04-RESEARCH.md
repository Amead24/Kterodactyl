# Phase 4: API Server Bridge - Research

**Researched:** 2026-02-10
**Domain:** Go REST API server as authenticated gateway to Kubernetes API
**Confidence:** HIGH

## Summary

Phase 4 builds a Go REST API server that acts as an authenticated gateway between users and the Kubernetes API. The server must validate JWT tokens (built in Phase 3), map users to namespaces, and proxy CRUD operations on GameServer custom resources. It must never expose the Kubernetes API directly. The API server will load game definitions from a local `games/` directory (YAML manifests) and enforce rate limiting.

The existing codebase already has a comprehensive auth layer (JWT, middleware, user store) in `internal/auth/` and CRD types in `api/v1alpha1/`. The API server needs to integrate with the existing controller-runtime manager via its `manager.Add()` mechanism, running alongside the operator controllers in the same process. This avoids a separate deployment and gives the API server direct access to the controller-runtime client for Kubernetes operations.

**Primary recommendation:** Use chi v5 router with httprate rate limiting, integrate as a `manager.Server` Runnable in the existing controller-runtime manager, use the existing `sigs.k8s.io/controller-runtime/pkg/client` for all Kubernetes operations (no raw client-go needed), and keep JSON encoding simple with `encoding/json` (avoid chi/render to minimize dependencies since the API is straightforward).

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/go-chi/chi/v5` | v5.2.5 | HTTP router | Lightweight, 100% net/http compatible, supports route groups/middleware, no external deps, Go 1.20+ support policy |
| `github.com/go-chi/httprate` | v0.15.0+ | Rate limiting middleware | Same ecosystem as chi, sliding window counter (CloudFlare pattern), per-IP/per-endpoint/global support |
| `github.com/go-chi/cors` | latest | CORS middleware | Same ecosystem as chi, handles preflight correctly, required for browser-based UI in later phases |
| `sigs.k8s.io/controller-runtime` | v0.23.1 | K8s client + manager lifecycle | Already in go.mod, provides typed client for CRD CRUD, manager.Server for HTTP server lifecycle |
| `encoding/json` | stdlib | JSON encoding/decoding | Sufficient for this API, avoids extra dependency; chi/render adds complexity without proportional benefit |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `gopkg.in/yaml.v3` | v3.0.1 | Game manifest loading | Already in go.mod (indirect), needed to parse YAML game definitions from `games/` directory |
| `github.com/go-chi/chi/v5/middleware` | (included with chi) | Standard middleware | RequestID, Logger, Recoverer, Timeout, RealIP |
| `net/http/httptest` | stdlib | API handler testing | Table-driven tests for all handlers |
| `github.com/go-logr/logr` | v1.4.3 | Structured logging | Already in go.mod, use controller-runtime's logger for consistency |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| chi v5 | net/http (Go 1.22+ routing) | Go 1.22+ has improved routing but chi provides better middleware ecosystem, route groups, and URL param extraction |
| chi v5 | gorilla/mux | Gorilla was archived/unarchived; chi has cleaner middleware composition |
| httprate | golang.org/x/time/rate | x/time/rate is lower-level (no HTTP middleware), httprate integrates with chi directly |
| encoding/json | chi/render | render adds Binder/Renderer interfaces; overkill for simple JSON CRUD API |
| yaml.v3 | encoding/json for manifests | YAML is more human-friendly for game definitions that admins will edit by hand |

**Installation:**
```bash
go get github.com/go-chi/chi/v5@v5.2.5
go get github.com/go-chi/httprate@latest
go get github.com/go-chi/cors@latest
```

Note: `sigs.k8s.io/controller-runtime`, `gopkg.in/yaml.v3`, and `github.com/go-logr/logr` are already in go.mod.

## Architecture Patterns

### Recommended Project Structure
```
internal/
  api/                        # NEW - REST API server package
    server.go                 # Server struct, router setup, manager.Server integration
    routes.go                 # Route definitions (wires handlers + middleware)
    middleware.go             # API-specific middleware (namespace scoping, ownership)
    handlers_auth.go          # POST /auth/login, POST /auth/register, POST /auth/refresh
    handlers_gameserver.go    # CRUD handlers for GameServer resources
    handlers_games.go         # GET /games (list available game manifests)
    handlers_admin.go         # Admin-only endpoints (invite, user management)
    handlers_health.go        # GET /healthz, GET /readyz for the API server
    response.go               # JSON response helpers (success, error, list)
    request.go                # JSON request binding helpers, validation
    server_test.go            # Server integration tests
    handlers_auth_test.go     # Auth handler unit tests
    handlers_gameserver_test.go # GameServer handler unit tests
  auth/                       # EXISTING - Authentication (from Phase 3)
  controller/                 # EXISTING - Reconcilers (from Phase 1-2)
  manifest/                   # NEW - Game manifest loading
    manifest.go               # GameManifest type, LoadManifests(), GetManifest()
    manifest_test.go          # Manifest loading tests
  util/                       # EXISTING - Labels, networking helpers
games/                        # NEW - Game manifest YAML files (project root)
  minecraft.yaml
  valheim.yaml
cmd/
  main.go                     # MODIFIED - add API server to manager
```

### Pattern 1: Controller-Runtime Manager HTTP Server Integration
**What:** Run the REST API server as a `manager.Server` Runnable alongside the operator controllers
**When to use:** When the API server needs access to the same controller-runtime client and lives in the same binary
**Example:**
```go
// Source: pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/manager
import "sigs.k8s.io/controller-runtime/pkg/manager"

// In cmd/main.go, after setting up controllers:
apiServer := api.NewServer(api.Config{
    Client:            mgr.GetClient(),
    Scheme:            mgr.GetScheme(),
    JWTService:        jwtService,
    UserStore:         userStore,
    InviteService:     inviteService,
    ManifestLoader:    manifestLoader,
    OperatorNamespace: operatorNamespace,
    AdminConfig:       adminCfg, // or load per-request
})

if err := mgr.Add(&manager.Server{
    Name:   "api-server",
    Server: apiServer.HTTPServer(), // returns *http.Server
}); err != nil {
    setupLog.Error(err, "unable to add API server")
    os.Exit(1)
}
```

### Pattern 2: Namespace-Scoped GameServer CRUD via controller-runtime Client
**What:** Use the existing `sigs.k8s.io/controller-runtime/pkg/client` to perform GameServer CRUD in user namespaces
**When to use:** All GameServer operations in handlers
**Example:**
```go
// Source: existing codebase patterns in internal/controller/gameserver_controller.go
import (
    "sigs.k8s.io/controller-runtime/pkg/client"
    gamev1alpha1 "github.com/kterodactyl/kterodactyl/api/v1alpha1"
)

// List user's GameServers
func (s *Server) listGameServers(ctx context.Context, namespace string) ([]gamev1alpha1.GameServer, error) {
    list := &gamev1alpha1.GameServerList{}
    if err := s.client.List(ctx, list, client.InNamespace(namespace)); err != nil {
        return nil, err
    }
    return list.Items, nil
}

// Create a GameServer in user's namespace
func (s *Server) createGameServer(ctx context.Context, namespace, owner string, req *CreateGameServerRequest) (*gamev1alpha1.GameServer, error) {
    gs := &gamev1alpha1.GameServer{
        ObjectMeta: metav1.ObjectMeta{
            Name:      req.Name,
            Namespace: namespace,
            Labels:    util.GameServerLabels(owner, req.GameType),
        },
        Spec: gamev1alpha1.GameServerSpec{
            GameType: req.GameType,
            Image:    manifest.Image, // from loaded manifest
            Ports:    manifest.Ports,
            // ...
        },
    }
    if err := s.client.Create(ctx, gs); err != nil {
        return nil, err
    }
    return gs, nil
}
```

### Pattern 3: User-to-Namespace Mapping via JWT Claims
**What:** Extract the user's namespace from JWT claims (already set by Phase 3's JWT service) and scope all operations to that namespace
**When to use:** Every authenticated request
**Example:**
```go
// The existing auth middleware puts KterodactylClaims in context.
// Claims already include Namespace: "user-<username>"
func namespaceFromContext(r *http.Request) string {
    claims := auth.GetUserFromContext(r.Context())
    if claims == nil {
        return "" // middleware should prevent this
    }
    return claims.Namespace
}

// Handler uses this to scope operations:
func (s *Server) handleListGameServers(w http.ResponseWriter, r *http.Request) {
    ns := namespaceFromContext(r)
    servers, err := s.listGameServers(r.Context(), ns)
    // ...
}
```

### Pattern 4: Game Manifest Loading from Directory
**What:** Load YAML game definitions from a `games/` directory at startup, making them available as templates for GameServer creation
**When to use:** Server initialization and game listing endpoint
**Example:**
```go
// games/minecraft.yaml
// ---
// name: minecraft
// displayName: "Minecraft Java Edition"
// image: itzg/minecraft-server:latest
// ports:
//   - name: game
//     containerPort: 25565
//     protocol: TCP
// parameters:
//   EULA: "TRUE"
//   TYPE: "VANILLA"
// resources:
//   requests:
//     cpu: "500m"
//     memory: "1Gi"
//   limits:
//     cpu: "2"
//     memory: "4Gi"

type GameManifest struct {
    Name        string                         `yaml:"name"`
    DisplayName string                         `yaml:"displayName"`
    Image       string                         `yaml:"image"`
    Ports       []gamev1alpha1.GameServerPort   `yaml:"ports"`
    Parameters  map[string]string              `yaml:"parameters"`
    Resources   corev1.ResourceRequirements    `yaml:"resources"`
}

func LoadManifests(dir string) (map[string]*GameManifest, error) {
    // Read all .yaml files from dir
    // Parse each into GameManifest
    // Return map keyed by manifest.Name
}
```

### Pattern 5: Chi Router with Middleware Stacks
**What:** Organize routes with chi's route groups and per-group middleware
**When to use:** Route definitions
**Example:**
```go
func (s *Server) routes() chi.Router {
    r := chi.NewRouter()

    // Global middleware
    r.Use(middleware.RequestID)
    r.Use(middleware.RealIP)
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)
    r.Use(middleware.Timeout(30 * time.Second))
    r.Use(cors.Handler(cors.Options{
        AllowedOrigins:   []string{"*"}, // tighten in production
        AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowedHeaders:   []string{"Authorization", "Content-Type"},
        ExposedHeaders:   []string{"X-Refresh-Token"},
        AllowCredentials: true,
        MaxAge:           300,
    }))
    r.Use(httprate.LimitByIP(100, time.Minute))

    // Health endpoints (unauthenticated)
    r.Get("/healthz", s.handleHealthz)
    r.Get("/readyz", s.handleReadyz)

    // Public auth endpoints
    r.Post("/api/v1/auth/login", s.handleLogin)
    r.Post("/api/v1/auth/register", s.handleRegister)

    // Authenticated routes
    r.Route("/api/v1", func(r chi.Router) {
        r.Use(s.authMiddleware.Authenticate)

        // Auth management
        r.Post("/auth/refresh", s.handleRefresh)

        // GameServer CRUD
        r.Route("/gameservers", func(r chi.Router) {
            r.Get("/", s.handleListGameServers)
            r.Post("/", s.handleCreateGameServer)
            r.Route("/{name}", func(r chi.Router) {
                r.Get("/", s.handleGetGameServer)
                r.Delete("/", s.handleDeleteGameServer)
                // PUT for update (e.g., change parameters)
                r.Put("/", s.handleUpdateGameServer)
            })
        })

        // Game manifests (available games)
        r.Get("/games", s.handleListGames)
        r.Get("/games/{gameType}", s.handleGetGame)

        // Admin routes
        r.Route("/admin", func(r chi.Router) {
            r.Use(auth.RequireAdmin)
            r.Post("/invites", s.handleCreateInvite)
            r.Get("/users", s.handleListUsers)
            r.Delete("/users/{username}", s.handleDeleteUser)
        })
    })

    return r
}
```

### Pattern 6: JSON Response Helpers
**What:** Consistent JSON response format across all endpoints
**When to use:** Every handler
**Example:**
```go
// Consistent error response format (matches Phase 3's JSON error format)
type ErrorResponse struct {
    Error   string `json:"error"`
    Details string `json:"details,omitempty"`
}

type SuccessResponse struct {
    Data interface{} `json:"data,omitempty"`
}

type ListResponse struct {
    Data  interface{} `json:"data"`
    Count int         `json:"count"`
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
    respondJSON(w, status, ErrorResponse{Error: message})
}
```

### Anti-Patterns to Avoid
- **Exposing Kubernetes API directly:** Never forward raw K8s API responses to the user. Always transform into a clean REST response.
- **Global client without scoping:** Always scope K8s client operations to the user's namespace from JWT claims. Never allow namespace parameter from request body.
- **Blocking the controller-runtime manager:** The API server must run as a non-blocking Runnable. Use `manager.Server` which handles lifecycle correctly.
- **Loading AdminConfig at startup only:** AdminConfig should be loaded per-request (or cached with short TTL) since it can change without restart.
- **Sharing auth middleware state:** The existing `AuthMiddleware` from Phase 3 is stateless (uses JWTService); safe to share across goroutines.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| HTTP routing | Custom mux/path matching | chi v5 | URL params, middleware chains, route groups |
| Rate limiting | Token bucket from scratch | httprate | Sliding window, per-IP/endpoint, HTTP 429 headers |
| CORS handling | Manual OPTIONS handler | go-chi/cors | Preflight complexity, header management |
| Request ID tracking | Manual header management | chi/middleware.RequestID | Thread-safe, standard X-Request-Id header |
| Panic recovery | Manual defer/recover | chi/middleware.Recoverer | Logs stack trace, returns 500 |
| K8s CRUD operations | Raw client-go REST calls | controller-runtime client | Already typed for CRDs, handles serialization |
| JSON encoding | Custom serialization | encoding/json | Standard, well-tested, sufficient for this API |
| HTTP server lifecycle | Manual goroutine + signal handling | manager.Server | Integrates with controller-runtime graceful shutdown |

**Key insight:** The controller-runtime client already knows how to do typed CRUD on GameServer resources because the CRD types are registered in the scheme. No need for dynamic/unstructured clients or raw REST calls.

## Common Pitfalls

### Pitfall 1: Namespace Injection Attack
**What goes wrong:** User supplies a namespace in the request body/URL, allowing them to access other users' GameServers.
**Why it happens:** Developer adds namespace as a request parameter for convenience.
**How to avoid:** ALWAYS derive namespace from JWT claims (`claims.Namespace`). Never accept namespace from user input. The only namespace a user can access is `user-<username>`.
**Warning signs:** Any handler that reads namespace from request body or URL path.

### Pitfall 2: Race Condition Between API and Controller
**What goes wrong:** API creates a GameServer, but the controller hasn't reconciled yet. User immediately reads back the resource and sees no status.
**Why it happens:** Kubernetes is eventually consistent; status subresource updates happen asynchronously via reconciliation.
**How to avoid:** Document that status may take a moment to populate after creation. Return the created spec immediately (not status). Consider returning 202 Accepted for create operations.
**Warning signs:** Tests that assert status fields immediately after creation.

### Pitfall 3: AdminConfig Staleness
**What goes wrong:** API server loads AdminConfig at startup and never refreshes. Admin changes ConfigMap but API still uses old limits.
**Why it happens:** Developer caches config for performance without invalidation.
**How to avoid:** Load AdminConfig per-request using `LoadAdminConfig()` (already implemented in controller). The ConfigMap read is fast (cached by controller-runtime's informer cache). Alternatively, add a short TTL cache (30s-60s).
**Warning signs:** AdminConfig stored as a field on the server struct without refresh mechanism.

### Pitfall 4: CORS Middleware Placement
**What goes wrong:** CORS middleware placed inside a route group instead of at the top level. Preflight OPTIONS requests get 404 because no route matches.
**Why it happens:** Developer organizes CORS with other middleware in an authenticated group.
**How to avoid:** CORS middleware MUST be a top-level middleware on the chi router (`r.Use(cors.Handler(...))`), NOT inside `r.Group()` or `r.With()`.
**Warning signs:** Browser preflight requests returning 404 or 405.

### Pitfall 5: GameServer Name Collisions
**What goes wrong:** User creates two GameServers with the same name in their namespace.
**Why it happens:** The Kubernetes API will return `AlreadyExists`, but the API server doesn't handle this gracefully.
**How to avoid:** Check for `errors.IsAlreadyExists(err)` and return HTTP 409 Conflict with a clear error message.
**Warning signs:** Generic 500 errors when creating duplicate-named GameServers.

### Pitfall 6: Missing Request Body Validation
**What goes wrong:** Empty or malformed JSON bodies cause panics or cryptic errors.
**Why it happens:** Go's json.Decoder silently produces zero values for missing fields.
**How to avoid:** Validate all required fields after decoding. Return HTTP 400 with specific field-level error messages. Use a validation function on each request type.
**Warning signs:** Handlers that decode JSON but don't check required fields.

### Pitfall 7: Forgetting Owner Labels on Created GameServers
**What goes wrong:** GameServer is created without the `kterodactyl.io/owner` label. The reconciler transitions it to Error state ("Missing owner label").
**Why it happens:** The API handler forgets to apply labels that the controller expects.
**How to avoid:** Use `util.GameServerLabels(owner, gameType)` for every GameServer creation. Add a test that verifies labels are present.
**Warning signs:** GameServers created via API immediately going to Error state.

## Code Examples

Verified patterns from official sources and the existing codebase:

### Server Struct and Constructor
```go
// Source: project patterns + pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/manager
package api

import (
    "net/http"
    "time"
    "github.com/go-chi/chi/v5"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "github.com/kterodactyl/kterodactyl/internal/auth"
    "github.com/kterodactyl/kterodactyl/internal/manifest"
)

type Config struct {
    Client            client.Client
    JWTService        *auth.JWTService
    UserStore         auth.UserService
    InviteService     *auth.InviteService
    ManifestLoader    *manifest.Loader
    OperatorNamespace string
    BindAddress       string // e.g., ":8080"
}

type Server struct {
    client            client.Client
    jwtService        *auth.JWTService
    userStore         auth.UserService
    inviteService     *auth.InviteService
    authMiddleware    *auth.AuthMiddleware
    manifestLoader    *manifest.Loader
    operatorNamespace string
    router            chi.Router
    bindAddress       string
}

func NewServer(cfg Config) *Server {
    s := &Server{
        client:            cfg.Client,
        jwtService:        cfg.JWTService,
        userStore:         cfg.UserStore,
        inviteService:     cfg.InviteService,
        authMiddleware:    auth.NewAuthMiddleware(cfg.JWTService),
        manifestLoader:    cfg.ManifestLoader,
        operatorNamespace: cfg.OperatorNamespace,
        bindAddress:       cfg.BindAddress,
    }
    s.router = s.routes()
    return s
}

func (s *Server) HTTPServer() *http.Server {
    return &http.Server{
        Addr:              s.bindAddress,
        Handler:           s.router,
        ReadHeaderTimeout: 10 * time.Second,
        IdleTimeout:       120 * time.Second,
    }
}
```

### Login Handler
```go
// Source: existing auth package patterns
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Username string `json:"username"`
        Password string `json:"password"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, http.StatusBadRequest, "invalid request body")
        return
    }
    if req.Username == "" || req.Password == "" {
        respondError(w, http.StatusBadRequest, "username and password are required")
        return
    }

    user, err := s.userStore.GetUser(r.Context(), req.Username)
    if err != nil {
        respondError(w, http.StatusUnauthorized, "invalid credentials")
        return
    }

    ok, err := auth.VerifyPassword(req.Password, user.PasswordHash)
    if err != nil || !ok {
        respondError(w, http.StatusUnauthorized, "invalid credentials")
        return
    }

    token, err := s.jwtService.GenerateToken(user)
    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to generate token")
        return
    }

    respondJSON(w, http.StatusOK, map[string]string{"token": token})
}
```

### GameServer Create Handler
```go
// Source: existing CRD types + controller patterns
func (s *Server) handleCreateGameServer(w http.ResponseWriter, r *http.Request) {
    claims := auth.GetUserFromContext(r.Context())
    ns := claims.Namespace

    var req CreateGameServerRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, http.StatusBadRequest, "invalid request body")
        return
    }
    if err := req.Validate(); err != nil {
        respondError(w, http.StatusBadRequest, err.Error())
        return
    }

    // Look up game manifest
    manifest, ok := s.manifestLoader.Get(req.GameType)
    if !ok {
        respondError(w, http.StatusBadRequest, "unknown game type: "+req.GameType)
        return
    }

    gs := &gamev1alpha1.GameServer{
        ObjectMeta: metav1.ObjectMeta{
            Name:      req.Name,
            Namespace: ns,
            Labels:    util.GameServerLabels(claims.Username, req.GameType),
        },
        Spec: gamev1alpha1.GameServerSpec{
            GameType:   req.GameType,
            Image:      manifest.Image,
            Ports:      manifest.Ports,
            Parameters: mergeMaps(manifest.Parameters, req.Parameters),
            Resources:  manifest.Resources,
        },
    }

    if err := s.client.Create(r.Context(), gs); err != nil {
        if k8serrors.IsAlreadyExists(err) {
            respondError(w, http.StatusConflict, "game server with this name already exists")
            return
        }
        respondError(w, http.StatusInternalServerError, "failed to create game server")
        return
    }

    respondJSON(w, http.StatusCreated, gameServerToResponse(gs))
}
```

### Table-Driven Handler Test
```go
// Source: existing test patterns in internal/auth/auth_test.go
func TestHandleLogin(t *testing.T) {
    tests := []struct {
        name       string
        body       string
        wantStatus int
        wantError  string
    }{
        {
            name:       "valid credentials",
            body:       `{"username":"alice","password":"correctpassword"}`,
            wantStatus: http.StatusOK,
        },
        {
            name:       "missing username",
            body:       `{"password":"test"}`,
            wantStatus: http.StatusBadRequest,
            wantError:  "username and password are required",
        },
        {
            name:       "wrong password",
            body:       `{"username":"alice","password":"wrongpassword"}`,
            wantStatus: http.StatusUnauthorized,
            wantError:  "invalid credentials",
        },
        {
            name:       "invalid json",
            body:       `{invalid`,
            wantStatus: http.StatusBadRequest,
            wantError:  "invalid request body",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Setup: create server with mock/fake user store
            s := newTestServer(t)
            req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
                strings.NewReader(tt.body))
            req.Header.Set("Content-Type", "application/json")
            rec := httptest.NewRecorder()

            s.router.ServeHTTP(rec, req)

            if rec.Code != tt.wantStatus {
                t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
            }
            if tt.wantError != "" {
                body := rec.Body.String()
                if !strings.Contains(body, tt.wantError) {
                    t.Errorf("body = %q, want to contain %q", body, tt.wantError)
                }
            }
        })
    }
}
```

### Manager Integration in main.go
```go
// Source: pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/manager
// In cmd/main.go, after existing controller setup:

// Initialize auth services
signingKey, err := auth.EnsureSigningKey(context.Background(), mgr.GetClient(), operatorNamespace)
if err != nil {
    setupLog.Error(err, "failed to ensure JWT signing key")
    os.Exit(1)
}

adminCfg, err := controller.LoadAdminConfig(context.Background(), mgr.GetClient(), operatorNamespace)
if err != nil {
    setupLog.Error(err, "failed to load admin config")
    os.Exit(1)
}

jwtService := auth.NewJWTService(signingKey, time.Duration(adminCfg.JWTExpirationHours)*time.Hour)
userStore := auth.NewUserStore(mgr.GetClient(), operatorNamespace)
inviteService := auth.NewInviteService(mgr.GetClient(), operatorNamespace, nil, adminCfg.PanelURL)

// Load game manifests
manifestLoader, err := manifest.LoadFromDirectory("games/")
if err != nil {
    setupLog.Error(err, "failed to load game manifests")
    os.Exit(1)
}

// Create and register API server
apiServer := api.NewServer(api.Config{
    Client:            mgr.GetClient(),
    JWTService:        jwtService,
    UserStore:         userStore,
    InviteService:     inviteService,
    ManifestLoader:    manifestLoader,
    OperatorNamespace: operatorNamespace,
    BindAddress:       ":8080",
})

if err := mgr.Add(&manager.Server{
    Name:   "api-server",
    Server: apiServer.HTTPServer(),
}); err != nil {
    setupLog.Error(err, "unable to add API server to manager")
    os.Exit(1)
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| gorilla/mux | chi v5 | 2023+ (gorilla archived/unarchived) | chi is more actively maintained, lighter, better middleware |
| Hand-rolled rate limiting | httprate sliding window | 2024+ | More accurate, CloudFlare-inspired algorithm |
| Separate API binary | manager.Server Runnable | controller-runtime v0.15+ | Single binary, shared client, graceful lifecycle |
| client-go typed clientsets | controller-runtime client | Kubebuilder v3+ | Simpler, scheme-aware, works with CRDs out of box |
| go 1.22 mux routing | chi v5 (still preferred) | Go 1.22 (2024) | Go's stdlib improved but chi still offers superior middleware ecosystem |

**Deprecated/outdated:**
- gorilla/mux: Was archived, now community-maintained but chi is preferred for new projects
- Hand-rolled JSON response helpers: encoding/json is sufficient; chi/render is optional

## REST API Design

### Endpoint Summary

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/healthz` | No | Liveness probe |
| GET | `/readyz` | No | Readiness probe |
| POST | `/api/v1/auth/login` | No | Login, returns JWT |
| POST | `/api/v1/auth/register` | No | Register with invite token |
| POST | `/api/v1/auth/refresh` | Yes | Refresh JWT token |
| GET | `/api/v1/games` | Yes | List available game types |
| GET | `/api/v1/games/{gameType}` | Yes | Get game manifest details |
| GET | `/api/v1/gameservers` | Yes | List user's game servers |
| POST | `/api/v1/gameservers` | Yes | Create a game server |
| GET | `/api/v1/gameservers/{name}` | Yes | Get game server details |
| PUT | `/api/v1/gameservers/{name}` | Yes | Update game server |
| DELETE | `/api/v1/gameservers/{name}` | Yes | Delete game server |
| POST | `/api/v1/admin/invites` | Admin | Create invite |
| GET | `/api/v1/admin/users` | Admin | List all users |
| DELETE | `/api/v1/admin/users/{username}` | Admin | Delete user |

### Rate Limiting Strategy

| Scope | Limit | Window | Purpose |
|-------|-------|--------|---------|
| Global (per IP) | 100 requests | 1 minute | Prevent DDoS |
| Login endpoint | 5 requests | 1 minute | Prevent brute force |
| Register endpoint | 3 requests | 1 minute | Prevent abuse |
| Create GameServer | 10 requests | 1 minute | Prevent resource exhaustion |

## Open Questions

1. **API server port and binding**
   - What we know: The controller-runtime manager already uses :8081 for health probes and :8443 for metrics.
   - What's unclear: Should the API server bind to :8080? Or a configurable port?
   - Recommendation: Use :8080 as default, configurable via flag (e.g., `--api-bind-address`). Add to cmd/main.go flags.

2. **GameServer update scope**
   - What we know: GameServerSpec includes GameType, Image, Resources, Ports, Parameters.
   - What's unclear: Which fields should be updatable via PUT? Changing GameType seems like a "delete and recreate" operation.
   - Recommendation: Allow updating only Parameters (game config) and Resources. GameType and Image come from the manifest and shouldn't change.

3. **Game manifest hot-reload**
   - What we know: Manifests are loaded from `games/` directory at startup.
   - What's unclear: Should manifests be reloadable without restart? Via ConfigMap instead?
   - Recommendation: Start with startup-only loading from filesystem. File-based is simpler for a homelab. Future enhancement could watch the directory or use ConfigMaps.

4. **API versioning strategy**
   - What we know: v1alpha1 is the CRD version.
   - What's unclear: Should the REST API also be versioned (e.g., `/api/v1/`)?
   - Recommendation: Use `/api/v1/` prefix. This decouples REST API versioning from CRD versioning and follows standard practice.

5. **SMTP password for InviteService**
   - What we know: SMTP config is in AdminConfig ConfigMap but password should be in a Secret.
   - What's unclear: How should the API server load the SMTP password?
   - Recommendation: Load SMTP password from a separate Kubernetes Secret (e.g., `kterodactyl-smtp-credentials`). The ConfigMap stores non-sensitive SMTP config.

## Sources

### Primary (HIGH confidence)
- [pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/manager](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/manager) - Manager interface, Runnable, Server type
- [pkg.go.dev/github.com/go-chi/chi/v5](https://pkg.go.dev/github.com/go-chi/chi/v5) - Chi v5.2.5 API, routing patterns, middleware
- [pkg.go.dev/github.com/go-chi/httprate](https://pkg.go.dev/github.com/go-chi/httprate) - Rate limiting middleware v0.15.0
- Existing codebase: `internal/auth/` (middleware, JWT, user store, errors), `api/v1alpha1/` (CRD types), `internal/controller/` (AdminConfig, reconciler patterns)

### Secondary (MEDIUM confidence)
- [github.com/go-chi/cors](https://github.com/go-chi/cors) - CORS middleware configuration
- [github.com/go-chi/render](https://pkg.go.dev/github.com/go-chi/render) - Render package (decided against, but verified API)
- [go-chi/chi releases](https://github.com/go-chi/chi/releases) - Version history, Go compatibility policy

### Tertiary (LOW confidence)
- Community patterns for game manifest loading from YAML directories (no single authoritative source; based on general Go YAML loading patterns)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - chi v5, httprate, cors are well-documented with official Go package docs verified
- Architecture: HIGH - controller-runtime manager.Server pattern is documented in official pkg.go.dev; CRD CRUD via controller-runtime client is proven in existing codebase
- Pitfalls: HIGH - Based on direct analysis of existing codebase patterns (owner labels, namespace scoping, AdminConfig loading)
- Game manifests: MEDIUM - YAML loading pattern is standard Go, but specific manifest schema is project-specific design
- Rate limiting strategy: MEDIUM - Limits are reasonable defaults but may need tuning in practice

**Research date:** 2026-02-10
**Valid until:** 2026-03-12 (30 days - stable domain, well-established patterns)
