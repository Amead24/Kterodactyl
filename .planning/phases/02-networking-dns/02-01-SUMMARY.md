---
phase: 02-networking-dns
plan: 01
subsystem: networking
tags: [gateway-api, dns, httproute, kubernetes-operator]

# Dependency graph
requires:
  - phase: 01-operator-foundation
    provides: "GameServer CRD, controller, AdminConfig struct, labels.go utility package"
provides:
  - "Gateway API v1.4.1 dependency and scheme registration"
  - "GameServerDNSName() utility for DNS name construction"
  - "Networking constants: AnnotationDNSName, AnnotationExternalDNSTTL, LabelHTTPRouteOwner"
  - "AdminConfig networking fields: BaseDomain, GatewayName, GatewayNamespace, GatewayControllerNamespace"
affects: [02-networking-dns, 03-port-management]

# Tech tracking
tech-stack:
  added: [sigs.k8s.io/gateway-api v1.4.1]
  patterns: [dns-name-construction, configmap-driven-networking-config]

key-files:
  created:
    - internal/util/networking.go
  modified:
    - go.mod
    - go.sum
    - cmd/main.go
    - config/manager/admin-config.yaml
    - internal/controller/gameserver_controller.go

key-decisions:
  - "DNS name pattern: game.username.baseDomain (e.g., minecraft.alice.example.com)"
  - "BaseDomain empty string means DNS routing is disabled (opt-in)"
  - "Gateway API scheme registered in init() alongside existing CRD scheme"
  - "Networking constants in separate networking.go file, not in labels.go"

patterns-established:
  - "Networking utilities in internal/util/networking.go"
  - "ConfigMap-driven networking config with sensible defaults"

# Metrics
duration: 3min
completed: 2026-02-10
---

# Phase 2 Plan 1: Gateway API + Networking Utilities Summary

**Gateway API v1.4.1 dependency with GameServerDNSName utility, scheme registration, and admin ConfigMap networking fields**

## Performance

- **Duration:** 3 min
- **Started:** 2026-02-10T21:10:41Z
- **Completed:** 2026-02-10T21:14:33Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- Added Gateway API v1.4.1 as a direct Go module dependency
- Created `internal/util/networking.go` with DNS name construction utility and networking constants
- Registered Gateway API types (HTTPRoute, Gateway) in the operator scheme via `gatewayv1.Install(scheme)`
- Extended admin ConfigMap and AdminConfig struct with baseDomain, gatewayName, gatewayNamespace, gatewayControllerNamespace

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Gateway API dependency and create networking utilities** - `486a731` (feat)
2. **Task 2: Register Gateway API scheme and extend admin ConfigMap** - `aa31f43` (feat)

## Files Created/Modified
- `internal/util/networking.go` - DNS name construction utility and networking constants (AnnotationDNSName, LabelHTTPRouteOwner)
- `go.mod` / `go.sum` - Added sigs.k8s.io/gateway-api v1.4.1 dependency
- `cmd/main.go` - Gateway API scheme registration in init()
- `config/manager/admin-config.yaml` - Added baseDomain, gatewayName, gatewayNamespace, gatewayControllerNamespace fields
- `internal/controller/gameserver_controller.go` - AdminConfig struct and LoadAdminConfig extended with networking fields

## Decisions Made
- DNS name pattern: `game.username.baseDomain` (e.g., `minecraft.alice.example.com`)
- BaseDomain empty string means DNS routing is disabled (opt-in behavior)
- Gateway API scheme registered in `init()` alongside existing CRD scheme registration
- Networking constants separated into `networking.go` rather than mixed into `labels.go`
- Default gateway name "kterodactyl-gateway" in namespace "kterodactyl-system" with controller data plane in "envoy-gateway-system"

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Installed Go 1.26.0 for module compatibility**
- **Found during:** Task 1
- **Issue:** System Go 1.18 could not parse go.mod with `go 1.25.3` directive
- **Fix:** Installed Go 1.26.0 to ~/go-install/go/ and used it for all build commands
- **Files modified:** None (runtime environment only)
- **Verification:** `go build ./...` and `go vet ./...` pass
- **Committed in:** N/A (environment fix, not code change)

**2. [Rule 3 - Blocking] Ran go mod tidy after adding direct Gateway API import**
- **Found during:** Task 2
- **Issue:** Gateway API was marked as `// indirect` after `go get`; needed to be direct after importing in cmd/main.go
- **Fix:** Ran `go mod tidy` to correct dependency markers
- **Files modified:** go.mod, go.sum
- **Verification:** `grep "gateway-api" go.mod` shows no `// indirect` marker
- **Committed in:** aa31f43 (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (2 blocking)
**Impact on plan:** Both fixes necessary for correct build toolchain and dependency management. No scope creep.

## Issues Encountered
None beyond the auto-fixed deviations above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Gateway API types importable and scheme-registered for HTTPRoute/Gateway serialization
- `GameServerDNSName()` utility ready for DNS controller in Plan 02
- AdminConfig networking fields ready for DNS controller to read baseDomain/gateway config
- `go build ./...` and `go vet ./...` pass cleanly

## Self-Check: PASSED

All files verified present. All commits verified in git log.

---
*Phase: 02-networking-dns*
*Completed: 2026-02-10*
