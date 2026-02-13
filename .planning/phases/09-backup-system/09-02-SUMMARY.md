---
phase: 09-backup-system
plan: 02
subsystem: api
tags: [backup, rest-api, s3, minio-go, cron, restore, chi-router]

# Dependency graph
requires:
  - phase: 09-backup-system
    provides: Backup CRD types, BackupReconciler, S3 configuration, label/annotation constants
  - phase: 04-api-server-bridge
    provides: API server patterns (Server struct, chi router, response helpers, execInPod)
  - phase: 08-mod-support
    provides: execInPod helper for tar-over-exec, annotation pattern for manifest fields
provides:
  - 5 backup REST API endpoints (create, list, delete, restore, schedule)
  - BackupResponse type mapping CRD to API response
  - S3 client creation on API server side for restore operations
  - BackupPath annotation set during GameServer creation
affects: [09-03, backup-frontend-ui]

# Tech tracking
tech-stack:
  added: []
  patterns: [api-side-s3-client-for-restore, cron-validation-via-robfig-parser, backup-cr-creation-from-api]

key-files:
  created:
    - internal/api/handlers_backups.go
  modified:
    - internal/api/handlers_gameserver.go
    - internal/api/routes.go

key-decisions:
  - "Direct restore in API handler via S3 download -> gunzip -> tar-into-pod, same pattern as mod upload; simpler than annotation-based restore via reconciler for v1"
  - "S3 client created per-request in API handler (not cached like BackupReconciler) since restore is infrequent; avoids stale credentials"
  - "Backup create/list available to authenticated users; delete/restore/schedule require admin role"
  - "S3 object cleanup deferred to BackupReconciler finalizer or future enhancement; orphan S3 objects acceptable for homelab v1"

patterns-established:
  - "API-side S3 client pattern: createS3Client reads credentials Secret and AdminConfig per-request for backup restore"
  - "Backup route nesting: /backups under /{name} with RequireAdmin on specific sub-routes only"

# Metrics
duration: 3min
completed: 2026-02-13
---

# Phase 9 Plan 2: Backup API Endpoints Summary

**5 backup REST API endpoints with create/list for users, admin-only delete/restore/schedule, S3-backed restore via gunzip-tar-into-pod pipeline**

## Performance

- **Duration:** 3 min
- **Started:** 2026-02-13T02:32:15Z
- **Completed:** 2026-02-13T02:35:21Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Five backup API handlers: create on-demand backup, list backups, delete backup, restore from backup, set/remove backup schedule
- Restore pipeline downloads from S3, decompresses gzip, and extracts tar into pod via execInPod (same pattern as mod upload)
- BackupResponse type maps all Backup CRD status fields (state, S3 metadata, timing) to clean API types
- GameServer creation now sets backupPath annotation from game manifest
- Cron expression validation via robfig/cron/v3 parser for schedule endpoint

## Task Commits

Each task was committed atomically:

1. **Task 1: Create backup API handlers** - `e503973` (feat)
2. **Task 2: Register backup routes in chi router** - `13f59d4` (feat)

## Files Created/Modified
- `internal/api/handlers_backups.go` - Backup CRUD handlers, restore pipeline, schedule management, BackupResponse type, S3 client helper
- `internal/api/handlers_gameserver.go` - Added backupPath annotation from manifest during GameServer creation
- `internal/api/routes.go` - Registered 5 backup routes with appropriate auth middleware (create/list for users, delete/restore/schedule admin-only)

## Decisions Made
- Direct restore in API handler (S3 -> gunzip -> tar-into-pod) rather than annotation-based reconciler approach, matching the mod upload pattern
- Per-request S3 client in API handler since restore is infrequent; avoids caching stale credentials
- Create and list endpoints available to all authenticated users; delete, restore, and schedule management require admin
- S3 object cleanup on backup deletion deferred to future enhancement; CR deletion only for v1

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Missing `context` import in handlers_backups.go on first build attempt -- added and resolved immediately

## User Setup Required
None - no external service configuration required for this plan.

## Next Phase Readiness
- All 5 backup API endpoints ready for frontend integration (Plan 03)
- S3 credentials Secret and AdminConfig S3 fields must be configured by admin before backup/restore will function
- Backup schedule cron validation ensures only valid expressions are accepted

## Self-Check: PASSED

All 3 claimed files verified present on disk. Both task commits (e503973, 13f59d4) verified in git history.

---
*Phase: 09-backup-system*
*Completed: 2026-02-13*
