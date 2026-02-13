---
phase: 08-mod-support
plan: 02
subsystem: api
tags: [rest-api, mod-upload, tar-over-exec, spdy, multipart, file-management]

# Dependency graph
requires:
  - phase: 08-mod-support
    plan: 01
    provides: "ModPath field on GameManifest, AnnotationModPath constant, PVC mod storage, pod volume mount"
  - phase: 07-console-realtime
    provides: "SPDY remotecommand exec pattern in handlers_console.go"
  - phase: 04-api-server-bridge
    provides: "API server, chi router, response helpers, auth middleware"
provides:
  - "POST /api/v1/gameservers/{name}/mods endpoint for uploading mod files via tar-over-exec"
  - "GET /api/v1/gameservers/{name}/mods endpoint for listing installed mods"
  - "DELETE /api/v1/gameservers/{name}/mods/{filename} endpoint for removing mods"
  - "execInPod shared helper for executing commands in game server pods"
  - "ModPath annotation set during GameServer creation from manifest"
  - "Ready->Creating and Allocated->Creating state transitions for restart-after-upload"
affects: [08-mod-support, frontend-mods]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Tar-over-exec: io.Pipe connects tar.Writer goroutine to SPDY exec stdin for file streaming into pods"
    - "execInPod reusable helper wrapping SPDY remotecommand pattern for arbitrary pod command execution"
    - "ls -la parsing for file listing from pod exec output"

key-files:
  created:
    - "internal/api/handlers_mods.go"
  modified:
    - "internal/api/handlers_gameserver.go"
    - "internal/api/routes.go"
    - "api/v1alpha1/gameserver_lifecycle.go"

key-decisions:
  - "Upload triggers server restart by setting state to Creating -- ensures mods are loaded on fresh server start"
  - "100MB upload limit via MaxBytesReader -- sufficient for v1 homelab use, can increase later"
  - "30s timeout retained for mod routes in v1 -- local cluster with 100MB limit should suffice; comment notes future extraction if needed"

patterns-established:
  - "execInPod helper: reusable pod exec wrapper returning stdout/stderr/error for any command"
  - "Tar streaming pattern: io.Pipe + goroutine tar writer + SPDY exec stdin for zero-disk file transfer"

# Metrics
duration: 4min
completed: 2026-02-13
---

# Phase 8 Plan 2: Mod API Handlers Summary

**Mod upload/list/delete REST API handlers using tar-over-exec streaming with reusable execInPod helper and automatic server restart after upload**

## Performance

- **Duration:** 4 min
- **Started:** 2026-02-13T01:13:31Z
- **Completed:** 2026-02-13T01:17:30Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Three mod management endpoints: upload (POST with multipart), list (GET), and delete (DELETE)
- Upload streams files into running pods via tar-over-exec without intermediate disk storage
- Shared execInPod helper wraps SPDY remotecommand pattern for reuse across all three handlers
- GameServer creation sets modPath annotation from manifest so controller can mount correct directory
- Server automatically restarts after mod upload to load new mods

## Task Commits

Each task was committed atomically:

1. **Task 1: Set modPath annotation during GameServer creation and create mod handlers** - `ad0b987` (feat)
2. **Task 2: Register mod routes in the router** - `d104fdb` (feat)

## Files Created/Modified
- `internal/api/handlers_mods.go` - Upload, list, delete handlers plus execInPod helper and parseLsOutput utility
- `internal/api/handlers_gameserver.go` - Sets AnnotationModPath annotation during GameServer creation
- `internal/api/routes.go` - Registers /mods routes under /{name} route group
- `api/v1alpha1/gameserver_lifecycle.go` - Added Ready->Creating and Allocated->Creating state transitions

## Decisions Made
- Upload triggers server restart (state -> Creating) to ensure mods are loaded on fresh start; follows same pattern as handleRestartGameServer
- 100MB upload limit via http.MaxBytesReader is sufficient for homelab v1; can be increased later
- 30s timeout retained for mod upload routes; comment added noting potential need for extraction to outside timeout group for large files in v2
- parseLsOutput skips "." and ".." entries and the "total" line from ls -la output

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Added Ready->Creating and Allocated->Creating state transitions**
- **Found during:** Task 1 (mod handler implementation)
- **Issue:** The existing handleRestartGameServer handler already sets state to Creating from Ready/Allocated, but ValidTransitions did not include these transitions, making the state machine inconsistent
- **Fix:** Added GameServerStateCreating to the valid targets for Ready and Allocated states
- **Files modified:** api/v1alpha1/gameserver_lifecycle.go
- **Verification:** go build passes, transitions now consistent with existing restart handler behavior
- **Committed in:** ad0b987 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug fix)
**Impact on plan:** Essential for correctness -- restart-after-upload requires valid state transitions. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Mod API handlers are complete; Plan 03 (frontend mod upload UI) can proceed
- All three endpoints compile and are registered in the router
- execInPod helper is available for any future pod command execution needs

## Self-Check: PASSED

All 4 modified/created files exist. Both task commits (ad0b987, d104fdb) verified in git log.

---
*Phase: 08-mod-support*
*Completed: 2026-02-13*
