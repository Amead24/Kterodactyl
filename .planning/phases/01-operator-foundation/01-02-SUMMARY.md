---
phase: 01-operator-foundation
plan: 02
subsystem: infra
tags: [kubernetes, controller-runtime, reconciler, state-machine, finalizer, createorupdate, rbac, pod-management]

# Dependency graph
requires:
  - phase: 01-01
    provides: "GameServer CRD types, state machine lifecycle, shared label constants"
provides:
  - "Full GameServer reconciliation loop with 6-state machine (internal/controller/gameserver_controller.go)"
  - "Pod management via controllerutil.CreateOrUpdate with owner references"
  - "Finalizer-based cleanup preventing orphaned resources"
  - "Event recording for state transitions"
  - "RBAC for pods, namespaces, resourcequotas, networkpolicies, events"
  - "Controller wired into Manager with EventRecorder and leader election"
affects: [01-03, 01-04, 02-networking, 03-namespace-isolation]

# Tech tracking
tech-stack:
  added: []
  patterns: [reconcile-loop, state-machine-dispatch, createorupdate-idempotent, finalizer-cleanup, re-fetch-before-status-update, predicate-filtering]

key-files:
  created: []
  modified:
    - internal/controller/gameserver_controller.go
    - cmd/main.go
    - api/v1alpha1/gameserver_lifecycle.go
    - config/rbac/role.yaml

key-decisions:
  - "Added Ready->Error, Allocated->Error, Starting->Creating transitions to lifecycle map for controller to handle Pod disappearance scenarios"
  - "Used AnnotationChangedPredicate alongside GenerationChangedPredicate in event filter to detect allocation annotation changes"
  - "Set LeaderElectionID to kterodactyl-operator.kterodactyl.io for uniqueness"
  - "Pod RestartPolicy=Never since operator manages lifecycle, not kubelet"

patterns-established:
  - "Reconcile dispatch pattern: switch on Status.State to call state-specific handler"
  - "Re-fetch before status update: always Get fresh copy before Status().Update() to avoid conflict errors"
  - "Finalizer pattern: add on create, remove on delete after cleanup, prevents orphaned resources"
  - "CreateOrUpdate for idempotent Pod management with owner reference via SetControllerReference"
  - "State transition helper: validates transition, re-fetches, updates conditions, records event"

# Metrics
duration: 4min
completed: 2026-02-10
---

# Phase 1 Plan 2: GameServer Controller Reconciliation Loop Summary

**Full reconciler with 6-state machine dispatch, idempotent Pod management via CreateOrUpdate, finalizer cleanup, and event recording wired into Manager**

## Performance

- **Duration:** 4 min
- **Started:** 2026-02-10T13:59:22Z
- **Completed:** 2026-02-10T14:03:39Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Implemented complete GameServer reconciliation loop with state dispatch across all 6 lifecycle states
- Pod creation/update via controllerutil.CreateOrUpdate with owner references and labels from util package
- Finalizer-based cleanup ensures Pods are deleted before GameServer CR removal
- Controller wired into Manager with EventRecorder and kterodactyl-specific leader election ID
- RBAC generated from markers covers pods, namespaces, resourcequotas, networkpolicies, events

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement GameServer reconciler with Pod management and state machine** - `09c6b1a` (feat)
2. **Task 2: Wire controller into Manager and verify end-to-end compilation** - `e47f80a` (feat)

## Files Created/Modified
- `internal/controller/gameserver_controller.go` - Full reconciler: 643 lines with 6 state handlers, Pod management, finalizer, event recording
- `cmd/main.go` - Added EventRecorder wiring, changed LeaderElectionID to kterodactyl-operator
- `api/v1alpha1/gameserver_lifecycle.go` - Added Ready->Error, Allocated->Error, Starting->Creating transitions
- `config/rbac/role.yaml` - Generated RBAC with pods, namespaces, resourcequotas, networkpolicies, events

## Decisions Made
- **Extended state transition map:** Added Ready->Error, Allocated->Error, and Starting->Creating transitions. The original lifecycle map from 01-01 did not account for Pod disappearance scenarios that the controller needs to handle (e.g., Pod deleted externally while GameServer is Ready).
- **Event filter predicates:** Used `predicate.Or(GenerationChangedPredicate, AnnotationChangedPredicate)` to ensure the controller reconciles both spec changes (generation) and allocation annotation changes.
- **Leader election ID:** Changed from Kubebuilder's random hash default to `kterodactyl-operator.kterodactyl.io` for clarity and uniqueness.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Extended ValidTransitions map for controller state handling**
- **Found during:** Task 1 (implementing reconcileReady and reconcileStarting)
- **Issue:** The plan specifies Ready->Error transitions (Pod disappeared) and Starting->Creating transitions (Pod deleted externally), but the ValidTransitions map from 01-01 did not include these. The controller's transitionState helper validates transitions and would reject them.
- **Fix:** Added Starting->Creating, Ready->Error, and Allocated->Error transitions to the ValidTransitions map in gameserver_lifecycle.go
- **Files modified:** api/v1alpha1/gameserver_lifecycle.go
- **Verification:** `go build ./...` succeeds, transitions used correctly in controller
- **Committed in:** 09c6b1a (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking issue)
**Impact on plan:** Necessary for controller correctness. The original transition map was designed for the abstract state machine; the concrete controller implementation revealed additional transitions needed for error recovery. No scope creep.

## Issues Encountered
None beyond the auto-fixed deviation above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Controller reconciliation loop complete and compiling
- Ready for unit/integration tests (Plan 01-03)
- State machine transitions cover all error recovery scenarios
- RBAC ready for cluster deployment
- Pod management pattern established for networking layer (Phase 2)

## Self-Check: PASSED

All 4 key files verified present. Both task commits (09c6b1a, e47f80a) verified in git history.

---
*Phase: 01-operator-foundation*
*Completed: 2026-02-10*
