---
phase: 07-console-realtime
plan: 01
subsystem: api
tags: [websocket, gorilla-websocket, k8s-metrics, pod-logs, remotecommand, spdy, console]

# Dependency graph
requires:
  - phase: 04-api-server-bridge
    provides: "Chi v5 API server with JWT auth middleware and gameserver CRUD handlers"
  - phase: 03-authentication
    provides: "JWTService with ValidateToken for WebSocket query param auth"
provides:
  - "WebSocket console endpoint at /api/v1/gameservers/{name}/console"
  - "REST metrics endpoint at GET /api/v1/gameservers/{name}/metrics"
  - "kubernetes.Clientset and rest.Config wired through api.Server for pod operations"
  - "Metrics API client (metricsv.Clientset) on api.Server"
  - "RBAC for pods/log (get) and pods/exec (create)"
affects: [07-02, 08-console-realtime-frontend]

# Tech tracking
tech-stack:
  added: [gorilla/websocket v1.5.4-pre, k8s.io/metrics v0.35.1]
  patterns: [write-channel-pattern, query-param-jwt-auth, timeout-scoped-middleware]

key-files:
  created:
    - internal/api/handlers_console.go
    - internal/api/handlers_metrics.go
  modified:
    - cmd/main.go
    - internal/api/server.go
    - internal/api/routes.go
    - internal/controller/gameserver_controller.go
    - config/rbac/role.yaml
    - go.mod
    - go.sum

key-decisions:
  - "gorilla/websocket v1.5.4-pre used instead of v1.5.3 -- k8s.io/client-go@v0.35 transitively requires this version"
  - "WebSocket console route registered outside timeout middleware group to prevent 30s connection kills"
  - "Write channel pattern (chan []byte, 256 buffer) prevents concurrent WebSocket write panics"
  - "JWT auth via query param for WebSocket (upgrade handshake cannot carry Authorization headers)"
  - "Metrics API errors return 503 (not 500) for graceful degradation when metrics-server unavailable"

patterns-established:
  - "Write channel pattern: single writer goroutine owns WebSocket writes, all senders use buffered channel"
  - "Scoped timeout middleware: Timeout(30s) applied to REST route group only, not global"
  - "Query param JWT auth: WebSocket handlers validate token from ?token= instead of Authorization header"

# Metrics
duration: 7min
completed: 2026-02-12
---

# Phase 7 Plan 1: Console & Metrics Backend Summary

**WebSocket console proxy with pod log streaming and exec, REST metrics endpoint from Kubernetes Metrics API, and route restructure for WebSocket timeout isolation**

## Performance

- **Duration:** 7 min
- **Started:** 2026-02-12T12:55:31Z
- **Completed:** 2026-02-12T13:03:00Z
- **Tasks:** 3
- **Files modified:** 9

## Accomplishments
- WebSocket console handler streams pod logs in real-time via Follow=true with 100-line tail history
- Command execution via SPDY remotecommand exec into the gameserver container
- Ping/pong keepalive at 30-second intervals for Cloudflare Tunnel proxy compatibility
- REST metrics endpoint returns CPU/memory from Kubernetes Metrics API with spec limits for context
- Routes restructured: timeout middleware scoped to REST group only, WebSocket routes are long-lived

## Task Commits

Each task was committed atomically:

1. **Task 1: Add dependencies and wire kubernetes.Clientset, rest.Config, and metrics client** - `f8b133b` (feat)
2. **Task 2: Implement WebSocket console handler with log streaming and command exec** - `8bcf5dd` (feat)
3. **Task 3: Implement metrics handler and restructure routes for WebSocket support** - `63bbc60` (feat)

## Files Created/Modified
- `internal/api/handlers_console.go` - WebSocket console handler with log streaming, command exec, write channel pattern, and ping/pong keepalive (323 lines)
- `internal/api/handlers_metrics.go` - REST metrics handler returning CPU/memory from Kubernetes Metrics API (109 lines)
- `internal/api/routes.go` - Restructured: timeout middleware scoped to REST group, WebSocket route outside
- `internal/api/server.go` - Added Clientset, RestConfig, MetricsClient to Config and Server structs
- `cmd/main.go` - Creates kubernetes.Clientset and metricsv.Clientset, extracted restConfig variable
- `internal/controller/gameserver_controller.go` - Added RBAC markers for pods/log and pods/exec
- `config/rbac/role.yaml` - Regenerated from RBAC markers
- `go.mod` / `go.sum` - Added gorilla/websocket and k8s.io/metrics dependencies

## Decisions Made
- gorilla/websocket v1.5.4-pre used instead of plan's v1.5.3 because k8s.io/client-go@v0.35 transitively requires this version (same API, compatible)
- WebSocket console route registered at top-level router (before /api/v1 group) to avoid timeout middleware
- Write channel pattern with 256-buffer channel for concurrency-safe WebSocket writes
- JWT auth via query param (not header) for WebSocket connections -- browser WebSocket API cannot set custom headers during upgrade
- Metrics API errors return 503 Service Unavailable for graceful degradation when metrics-server not installed

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] gorilla/websocket version conflict**
- **Found during:** Task 1 (dependency installation)
- **Issue:** Pinning gorilla/websocket@v1.5.3 caused k8s.io/client-go@v0.35 and related deps to downgrade to v0.33.0-beta.0
- **Fix:** Used gorilla/websocket@v1.5.4-0.20250319132907-e064f32e3674 (required by k8s.io/client-go@v0.35.0)
- **Files modified:** go.mod, go.sum
- **Verification:** go build ./... passes, all APIs identical
- **Committed in:** f8b133b (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking dependency)
**Impact on plan:** Version pinning adjusted to satisfy transitive dependency. No API differences. No scope creep.

## Issues Encountered
- go mod tidy needed after adding new imports that pulled in additional transitive dependencies (spdystream, flowrate) -- standard Go workflow

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Backend WebSocket and metrics endpoints are ready for Plan 02 (frontend xterm.js console and metrics UI)
- metrics-server must be installed in the cluster for the metrics endpoint to return data (returns 503 gracefully without it)
- Console endpoint requires a running game server pod in Ready or Allocated state

## Self-Check: PASSED

All 8 key files exist. All 3 task commits verified in git log.

---
*Phase: 07-console-realtime*
*Completed: 2026-02-12*
