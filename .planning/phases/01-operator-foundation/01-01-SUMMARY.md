---
phase: 01-operator-foundation
plan: 01
subsystem: infra
tags: [kubebuilder, kubernetes, crd, go, controller-runtime, state-machine]

# Dependency graph
requires: []
provides:
  - "GameServer CRD type definitions (api/v1alpha1/gameserver_types.go)"
  - "State machine lifecycle with 6 states and transition validation (api/v1alpha1/gameserver_lifecycle.go)"
  - "Shared label/annotation constants and helpers (internal/util/labels.go)"
  - "Generated CRD manifest (config/crd/bases/game.kterodactyl.io_gameservers.yaml)"
  - "Kubebuilder v4 project scaffold with Makefile, Dockerfile, RBAC"
  - "Sample GameServer CR for testing (config/samples/game_v1alpha1_gameserver.yaml)"
affects: [01-02, 01-03, 01-04, 02-networking, 03-namespace-isolation, 04-api-server]

# Tech tracking
tech-stack:
  added: [kubebuilder-v4.11.1, controller-runtime-v0.23.1, go-1.24, controller-gen-v0.20.0]
  patterns: [kubebuilder-v4-layout, crd-markers, state-machine, namespace-per-user-labels]

key-files:
  created:
    - api/v1alpha1/gameserver_types.go
    - api/v1alpha1/gameserver_lifecycle.go
    - api/v1alpha1/groupversion_info.go
    - api/v1alpha1/zz_generated.deepcopy.go
    - internal/util/labels.go
    - internal/controller/gameserver_controller.go
    - internal/controller/gameserver_controller_test.go
    - config/crd/bases/game.kterodactyl.io_gameservers.yaml
    - config/rbac/role.yaml
    - config/samples/game_v1alpha1_gameserver.yaml
    - cmd/main.go
    - Makefile
    - Dockerfile
    - go.mod
    - PROJECT
    - .gitignore
  modified: []

key-decisions:
  - "GameServerState type defined in gameserver_types.go for proximity to Status struct that uses it; constants and transition logic live in gameserver_lifecycle.go"
  - "Used Kubebuilder v4.11.1 scaffolding conventions (api/, internal/, cmd/) not custom pkg/ layout"
  - "Marked v1alpha1 as storageversion for future CRD versioning safety"

patterns-established:
  - "Kubebuilder v4 project layout: api/v1alpha1/ for types, internal/controller/ for reconcilers, internal/util/ for shared utilities"
  - "CRD markers pattern: validation, printcolumn, subresource, storageversion on type definitions"
  - "State machine pattern: ValidTransitions map with IsValidTransition/IsTerminal helpers"
  - "Label conventions: app.kubernetes.io/managed-by=kterodactyl, kterodactyl.io/owner, kterodactyl.io/game"

# Metrics
duration: 5min
completed: 2026-02-10
---

# Phase 1 Plan 1: Project Scaffold and CRD Types Summary

**Kubebuilder v4.11.1 project with GameServer CRD (5 spec fields, 4 status fields, 4 print columns), 6-state lifecycle machine, and shared label conventions**

## Performance

- **Duration:** 5 min
- **Started:** 2026-02-10T13:51:33Z
- **Completed:** 2026-02-10T13:56:29Z
- **Tasks:** 2
- **Files modified:** 56

## Accomplishments
- Scaffolded complete Kubebuilder v4 project with Go 1.24, controller-runtime v0.23.1
- Defined GameServer CRD with full spec (gameType, image, resources, ports, parameters) and status (state, address, ports, conditions)
- Implemented state machine with 6 states (Creating, Starting, Ready, Allocated, Shutdown, Error) and validated transitions
- Established shared label conventions and helper functions for namespace-per-user model

## Task Commits

Each task was committed atomically:

1. **Task 1: Scaffold Kubebuilder v4 project and define GameServer CRD types** - `a4dc36a` (feat)
2. **Task 2: Implement state machine lifecycle and shared label constants** - `1551f18` (feat)

## Files Created/Modified
- `api/v1alpha1/gameserver_types.go` - GameServer CRD spec/status type definitions with kubebuilder validation markers
- `api/v1alpha1/gameserver_lifecycle.go` - State machine constants, transition map, IsValidTransition/IsTerminal functions
- `api/v1alpha1/groupversion_info.go` - GVK registration for game.kterodactyl.io/v1alpha1
- `api/v1alpha1/zz_generated.deepcopy.go` - Auto-generated DeepCopy methods
- `internal/util/labels.go` - Shared label/annotation constants, UserNamespace and GameServerLabels helpers
- `internal/controller/gameserver_controller.go` - Scaffolded reconciler stub
- `internal/controller/gameserver_controller_test.go` - Scaffolded test stub
- `config/crd/bases/game.kterodactyl.io_gameservers.yaml` - Generated CRD manifest
- `config/rbac/role.yaml` - Generated RBAC ClusterRole
- `config/samples/game_v1alpha1_gameserver.yaml` - Realistic Minecraft example CR
- `cmd/main.go` - Operator entrypoint with manager setup
- `Makefile` - Build/test/deploy targets
- `Dockerfile` - Multi-stage container build
- `go.mod` / `go.sum` - Go module with all dependencies
- `PROJECT` - Kubebuilder project metadata
- `.gitignore` - Go/Kubebuilder ignore patterns

## Decisions Made
- **GameServerState type placement:** Defined in `gameserver_types.go` since it is used directly in the Status struct's json-tagged field. The lifecycle file contains only constants and behavior functions. This keeps the type close to its usage for controller-gen marker processing.
- **Kubebuilder v4 conventions:** Followed the scaffolded layout exactly (api/, internal/, cmd/) rather than custom pkg/ layout from earlier architecture docs. This ensures make targets and code generation work correctly.
- **Storage version marker:** Applied `+kubebuilder:storageversion` to v1alpha1 from the start to prevent CRD versioning debt when v1beta1/v1 are introduced later.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Installed Go 1.24.0 to user-local directory**
- **Found during:** Task 1 (pre-scaffold)
- **Issue:** System Go was 1.18; Kubebuilder v4.11.1 requires Go 1.24+. No sudo access for system-wide install.
- **Fix:** Installed Go 1.24.0 to /home/tony/.local/go and Kubebuilder to the same bin directory
- **Files modified:** None (toolchain only)
- **Verification:** `go version` reports 1.24.0, `kubebuilder version` reports v4.11.1
- **Committed in:** N/A (not a code change)

**2. [Rule 3 - Blocking] Added GameServerState type to types file for compilation**
- **Found during:** Task 1 (make manifests failed)
- **Issue:** GameServerStatus.State field references GameServerState type, but the type was only planned for Task 2's lifecycle file. controller-gen cannot resolve forward references across files that don't compile.
- **Fix:** Added the `GameServerState` type definition (string alias with validation marker) to `gameserver_types.go` where it is used. Task 2's lifecycle file contains the constants and behavior, not the type declaration.
- **Files modified:** api/v1alpha1/gameserver_types.go
- **Verification:** `make manifests generate` succeeds, `go build ./...` succeeds
- **Committed in:** a4dc36a (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (2 blocking issues)
**Impact on plan:** Both fixes were necessary for compilation and tooling. GameServerState type placement is architecturally sound -- the type lives with its usage, constants live with behavior. No scope creep.

## Issues Encountered
None beyond the auto-fixed deviations above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Project scaffold complete and compiling with Go 1.24 + Kubebuilder v4.11.1
- CRD types ready for controller reconciliation logic (Plan 01-02)
- State machine ready for status management in reconciler
- Label conventions established for namespace and Pod management
- Generated CRD manifest ready for cluster installation with `make install`

## Self-Check: PASSED

All 14 key files verified present. Both task commits (a4dc36a, 1551f18) verified in git history.

---
*Phase: 01-operator-foundation*
*Completed: 2026-02-10*
