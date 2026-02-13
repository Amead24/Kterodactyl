---
phase: 08-mod-support
plan: 01
subsystem: infra
tags: [kubernetes, pvc, persistent-storage, mods, game-manifest]

# Dependency graph
requires:
  - phase: 05-game-definition-framework
    provides: "GameManifest struct and manifest loader in internal/manifest/manifest.go"
  - phase: 01-operator-foundation
    provides: "GameServerReconciler, reconcilePod, reconcileCreating, AdminConfig pattern"
provides:
  - "ModPath field on GameManifest for declaring game mod directories"
  - "AnnotationModPath constant for passing modPath from API to controller"
  - "reconcileModPVC method creating PVC per GameServer with OwnerReference"
  - "Pod volume mount at modPath when annotation present"
  - "AdminConfig ModStorageSize (1Gi default) and ModStorageClass fields"
  - "RBAC marker for PersistentVolumeClaim operations"
  - "Minecraft manifest declares modPath: /mods"
affects: [08-mod-support, api-handlers, frontend-mods]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "PVC per GameServer with OwnerReference for automatic garbage collection"
    - "Annotation-based communication between API layer and controller for manifest-derived data"
    - "Immutable PVC spec pattern: only set spec fields when CreationTimestamp.IsZero()"

key-files:
  created: []
  modified:
    - "internal/manifest/manifest.go"
    - "internal/controller/gameserver_controller.go"
    - "internal/util/labels.go"
    - "games/minecraft/manifest.yaml"

key-decisions:
  - "ModPath stored as annotation on GameServer (AnnotationModPath) because controller has no access to manifest loader"
  - "PVC spec only set on creation (CreationTimestamp.IsZero check) since K8s PVC specs are immutable after creation"
  - "ModStorageSize defaults to 1Gi; ModStorageClass empty means cluster default StorageClass"

patterns-established:
  - "Annotation-based mod path communication: API sets kterodactyl.io/mod-path, controller reads it"
  - "PVC immutability guard: check CreationTimestamp.IsZero() before setting spec in CreateOrUpdate"

# Metrics
duration: 3min
completed: 2026-02-13
---

# Phase 8 Plan 1: Mod Storage Infrastructure Summary

**PVC-per-GameServer mod storage with manifest modPath field, controller PVC creation via OwnerReference, and pod volume mounting at game-specific mod directory**

## Performance

- **Duration:** 3 min
- **Started:** 2026-02-13T01:08:20Z
- **Completed:** 2026-02-13T01:11:00Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- GameManifest now declares modPath field for game-specific mod directories
- Controller creates a PVC per GameServer (owned by CR for automatic cleanup) when modPath annotation is set
- Pod spec includes volume mount from mods PVC at the manifest-defined mod directory
- AdminConfig supports configurable mod storage size and storage class
- Minecraft manifest declares modPath: /mods for itzg/minecraft-server mod directory

## Task Commits

Each task was committed atomically:

1. **Task 1: Add ModPath to game manifest and Minecraft definition** - `c6eb7b6` (feat)
2. **Task 2: Add PVC creation and volume mounting to controller** - `237d562` (feat)

## Files Created/Modified
- `internal/manifest/manifest.go` - Added ModPath field to GameManifest and rawGameManifest, populated during loading
- `internal/controller/gameserver_controller.go` - Added RBAC, AdminConfig fields, reconcileModPVC, PVC call in reconcileCreating, volume mount in reconcilePod
- `internal/util/labels.go` - Added AnnotationModPath constant for mod path annotation
- `games/minecraft/manifest.yaml` - Added modPath: /mods for Minecraft mod directory

## Decisions Made
- ModPath communicated from API to controller via annotation (kterodactyl.io/mod-path) because the controller does not have access to the manifest loader -- the manifest is only loaded by the API server
- PVC spec fields only set on creation (CreationTimestamp.IsZero check) because Kubernetes PVC specs are immutable after creation; CreateOrUpdate would fail on update otherwise
- ModStorageSize defaults to 1Gi and ModStorageClass defaults to empty (cluster default) for simplicity in homelab environments

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Mod storage infrastructure is in place; Plan 02 (mod upload/list/delete API handlers) can proceed
- Plan 02 will set AnnotationModPath during GameServer creation in handlers_gameserver.go and implement the tar-over-exec upload endpoints
- Plan 03 will add the frontend mod upload UI

## Self-Check: PASSED

All 4 modified files exist. Both task commits (c6eb7b6, 237d562) verified in git log.

---
*Phase: 08-mod-support*
*Completed: 2026-02-13*
