---
phase: 04-api-server-bridge
plan: 01
subsystem: api
tags: [chi, httprate, cors, yaml, manifest, rest-api, middleware, health-check]

# Dependency graph
requires:
  - phase: 03-authentication
    provides: "JWT service, auth middleware, user store, invite service, error types"
  - phase: 01-operator-foundation
    provides: "GameServer CRD types, AdminConfig, LoadAdminConfig, controller-runtime client patterns"
provides:
  - "GameManifest type and Loader with LoadFromDirectory/Get/List"
  - "Minecraft reference manifest in games/"
  - "API Server struct with Config, NewServer, HTTPServer"
  - "Chi router with 16 endpoints, 4 middleware groups, rate limiting"
  - "JSON response helpers: respondJSON, respondError, respondList"
  - "Request types with Validate(): Login, Register, CreateGameServer, UpdateGameServer, CreateInvite"
  - "Per-request AdminConfig loading via loadAdminConfig"
  - "Health endpoints: GET /healthz, GET /readyz"
affects: [04-02, 04-03, 04-04, 05-manager-integration]

# Tech tracking
tech-stack:
  added: [chi/v5 v5.2.5, go-chi/httprate v0.15.0, go-chi/cors v1.2.2]
  patterns: [chi-router-middleware-stacks, yaml-manifest-loading, raw-intermediate-types-for-yaml-k8s-types]

key-files:
  created:
    - internal/manifest/manifest.go
    - internal/manifest/manifest_test.go
    - games/minecraft.yaml
    - internal/api/server.go
    - internal/api/routes.go
    - internal/api/response.go
    - internal/api/request.go
    - internal/api/middleware.go
    - internal/api/handlers_health.go
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "Raw intermediate types for YAML deserialization of K8s types (resource.Quantity, GameServerPort have only JSON tags)"
  - "CORS at top-level router (not in route group) for proper preflight handling"
  - "Per-request AdminConfig loading via controller.LoadAdminConfig to avoid staleness"
  - "13 placeholder stubs returning 501 for handlers implemented in Plans 02/03"

patterns-established:
  - "rawGameManifest/rawPort pattern: intermediate YAML types bridging yaml.v3 tags to K8s JSON-only types"
  - "respondJSON/respondError/respondList: consistent JSON API response format"
  - "namespaceFromContext/usernameFromContext: JWT claims extraction from request context"
  - "Request type Validate() pattern: each request struct has its own validation method"

# Metrics
duration: 5min
completed: 2026-02-11
---

# Phase 4 Plan 01: API Server Foundation Summary

**Chi v5 REST API scaffold with manifest loader, 16-endpoint router, rate limiting, CORS, and health probes**

## Performance

- **Duration:** 5 min
- **Started:** 2026-02-11T03:28:05Z
- **Completed:** 2026-02-11T03:33:34Z
- **Tasks:** 2
- **Files modified:** 11

## Accomplishments
- Game manifest loader reads YAML files from directory, validates required fields, and provides Get/List access with alphabetical sorting
- Minecraft reference manifest in games/ with ports, parameters, and resource requirements
- API server struct wires all Phase 3 auth dependencies plus manifest loader and controller-runtime client
- Chi router defines all 16 endpoints with 4 middleware groups: global (RequestID, RealIP, Logger, Recoverer, Timeout, CORS, rate limit), public auth (tighter rate limits), authenticated user routes, and admin-only routes
- JSON response helpers produce consistent error and success formats across all endpoints

## Task Commits

Each task was committed atomically:

1. **Task 1: Game manifest loader and Minecraft reference** - `3f238e9` (feat)
2. **Task 2: API server scaffold with chi router, middleware, response helpers, and health endpoints** - `cc1f92c` (feat)

## Files Created/Modified
- `internal/manifest/manifest.go` - GameManifest type, Loader with LoadFromDirectory/Get/List, raw intermediate types for YAML-K8s bridge
- `internal/manifest/manifest_test.go` - 7 tests: load directory, empty dir, invalid YAML, missing name, missing image, get not found, nonexistent dir
- `games/minecraft.yaml` - Minecraft Java Edition reference manifest with ports, parameters, and resource requirements
- `internal/api/server.go` - Server struct, Config, NewServer constructor, HTTPServer factory
- `internal/api/routes.go` - Chi router with 16 endpoints, middleware stacks, rate limiting, 13 placeholder stubs
- `internal/api/response.go` - respondJSON, respondError, respondList helpers and ErrorResponse type
- `internal/api/request.go` - Request types with Validate() for Login, Register, CreateGameServer, UpdateGameServer, CreateInvite
- `internal/api/middleware.go` - namespaceFromContext, usernameFromContext, per-request loadAdminConfig
- `internal/api/handlers_health.go` - GET /healthz and GET /readyz returning {"status":"ok"}
- `go.mod` - Added chi/v5, httprate, cors direct dependencies
- `go.sum` - Updated with new dependency hashes

## Decisions Made
- Used raw intermediate types (rawGameManifest, rawPort, rawResources) for YAML deserialization because K8s types (resource.Quantity, GameServerPort) only have JSON struct tags, not yaml.v3 tags. This is a clean bridge pattern that converts string quantities to resource.Quantity via resource.ParseQuantity.
- CORS middleware placed at top-level router (not inside route groups) per Pitfall 4 from research -- preflight OPTIONS requests need to match before any route-specific middleware.
- AdminConfig loaded per-request via controller.LoadAdminConfig rather than cached on server struct, avoiding Pitfall 3 (staleness when ConfigMap changes).
- 13 placeholder handler stubs return HTTP 501 Not Implemented -- allows router to compile and health endpoints to be verified while Plans 02/03 implement the actual handlers.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed resource.Quantity YAML deserialization**
- **Found during:** Task 1 (manifest loader)
- **Issue:** corev1.ResourceRequirements contains resource.Quantity fields that only implement JSON Unmarshaler, not yaml.v3 Unmarshaler. Direct YAML unmarshaling failed with "cannot unmarshal !!str into resource.Quantity".
- **Fix:** Created rawResources intermediate type with string values, parse via resource.ParseQuantity after YAML unmarshaling
- **Files modified:** internal/manifest/manifest.go
- **Verification:** All manifest tests pass including resource quantity verification
- **Committed in:** 3f238e9 (Task 1 commit)

**2. [Rule 1 - Bug] Fixed GameServerPort YAML deserialization**
- **Found during:** Task 1 (manifest loader)
- **Issue:** GameServerPort struct uses JSON tags only (json:"containerPort"), so yaml.v3 defaults to lowercased field names and containerPort maps to 0.
- **Fix:** Created rawPort intermediate type with yaml tags, convert to GameServerPort after YAML unmarshaling
- **Files modified:** internal/manifest/manifest.go
- **Verification:** TestLoadFromDirectory verifies ContainerPort=25565 for minecraft manifest
- **Committed in:** 3f238e9 (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (2 bugs)
**Impact on plan:** Both auto-fixes necessary for correctness. The yaml.v3 vs JSON tag incompatibility is a known K8s ecosystem issue. Clean intermediate type pattern established for future manifest handling. No scope creep.

## Issues Encountered
- Go 1.18 system binary cannot parse go.mod with `go 1.25.3` directive. Found correct Go 1.25.3 binary at `/home/tony/sdk/go1.24/bin/go` (Go SDK manager naming convention). Used this for all build/test operations.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- API server scaffold complete with all route definitions and middleware stacks
- Plans 02 (auth + games handlers) and 03 (gameserver CRUD + admin handlers) will replace the 13 placeholder stubs with real implementations
- Plan 04 will integrate the API server with the controller-runtime manager in cmd/main.go
- All dependencies (chi, httprate, cors) installed and verified

## Self-Check: PASSED

All 9 created files verified present. Both task commits (3f238e9, cc1f92c) verified in git log.

---
*Phase: 04-api-server-bridge*
*Completed: 2026-02-11*
