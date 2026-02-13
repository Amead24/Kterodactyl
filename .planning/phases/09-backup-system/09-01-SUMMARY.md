---
phase: 09-backup-system
plan: 01
subsystem: controller
tags: [backup, s3, minio-go, crd, tar, gzip, cron, kubernetes-operator]

# Dependency graph
requires:
  - phase: 01-operator-foundation
    provides: CRD type patterns, controller reconciliation loop, AdminConfig pattern
  - phase: 08-mod-support
    provides: tar-over-exec pattern for file transfer to/from pods
provides:
  - Backup CRD with 4-state machine (Pending, InProgress, Completed, Failed)
  - BackupReconciler with S3 backup/restore pipelines
  - AdminConfig S3 configuration fields
  - BackupPath manifest field and annotation
  - Scheduled backup logic via cron annotations on GameServer
  - Backup retention enforcement
affects: [09-02, 09-03, backup-api-endpoints, backup-frontend-ui]

# Tech tracking
tech-stack:
  added: [minio-go/v7, robfig/cron/v3]
  patterns: [tar-gzip-s3-streaming-pipeline, lazy-s3-client-from-secret, schedule-via-annotation-and-requeue, synthetic-reconcile-request-for-gameserver-watch]

key-files:
  created:
    - api/v1alpha1/backup_types.go
    - api/v1alpha1/backup_lifecycle.go
    - internal/controller/backup_controller.go
    - config/crd/bases/game.kterodactyl.io_backups.yaml
  modified:
    - internal/util/labels.go
    - internal/manifest/manifest.go
    - internal/controller/gameserver_controller.go
    - games/minecraft/manifest.yaml
    - cmd/main.go
    - go.mod
    - go.sum
    - config/rbac/role.yaml

key-decisions:
  - "Operator-driven backup over CronJob: BackupReconciler performs tar-from-pod->gzip->S3 upload directly, avoiding cross-namespace credential distribution and separate backup container images"
  - "S3 credentials in Secret, S3 config in AdminConfig: kterodactyl-s3-credentials Secret for accessKeyID/secretAccessKey, AdminConfig ConfigMap for endpoint/bucket/region/SSL settings"
  - "Scheduled backups via GameServer annotation watch with synthetic reconcile requests: schedule-<gsname> naming pattern to distinguish schedule triggers from Backup CR reconciliation"
  - "Lazy S3 client initialization: client created on first backup, cached on reconciler struct, avoiding startup failures when S3 not configured"
  - "Auto-create S3 bucket on first backup via BucketExists + MakeBucket for improved setup experience"

patterns-established:
  - "Backup streaming pipeline: io.Pipe connects exec tar stdout -> gzip.NewWriter -> minio.PutObject for zero-memory-buffering backup"
  - "Restore streaming pipeline: minio.GetObject -> gzip.NewReader -> exec tar stdin for zero-memory-buffering restore"
  - "Schedule annotation pattern: cron expression on GameServer annotation, parsed via robfig/cron/v3, Backup CR created when due"
  - "Synthetic reconcile request: Watches on secondary resource enqueues schedule-<name> requests handled by name prefix detection in Reconcile"

# Metrics
duration: 6min
completed: 2026-02-13
---

# Phase 9 Plan 1: Backup System Backend Summary

**Backup CRD with 4-state machine, BackupReconciler performing tar-from-pod->gzip->S3 backup and S3->gunzip->tar-into-pod restore via minio-go/v7, with scheduled backups via cron annotations**

## Performance

- **Duration:** 6 min
- **Started:** 2026-02-13T02:22:32Z
- **Completed:** 2026-02-13T02:29:03Z
- **Tasks:** 2
- **Files modified:** 13

## Accomplishments
- Backup CRD registered with Kubernetes API with 4-state lifecycle (Pending, InProgress, Completed, Failed)
- BackupReconciler streams tar data from game server pods through gzip to S3 via io.Pipe (zero memory buffering)
- Restore pipeline downloads from S3, decompresses gzip, and extracts tar into pod
- Scheduled backup support via cron annotation on GameServer with automatic Backup CR creation
- Retention enforcement deletes oldest backups beyond configured limit (both CR and S3 object)
- S3 configuration via AdminConfig ConfigMap, credentials via Kubernetes Secret
- BackupPath field on game manifests (Minecraft defaults to /data)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create Backup CRD types, backupPath manifest field, labels, and AdminConfig S3 fields** - `b4291ac` (feat)
2. **Task 2: Create BackupReconciler with S3 backup/restore and wire into manager** - `8b52d34` (feat)

## Files Created/Modified
- `api/v1alpha1/backup_types.go` - Backup CRD type definitions with spec (gameServerName, backupPaths) and status (state, S3 metadata, timing)
- `api/v1alpha1/backup_lifecycle.go` - Backup state constants, valid transitions, terminal state helper, condition types
- `internal/controller/backup_controller.go` - BackupReconciler with S3 backup/restore pipelines, scheduling, retention
- `internal/util/labels.go` - Backup annotations (backup-path, schedule, retention, last-backup-time) and LabelBackupGameServer
- `internal/manifest/manifest.go` - BackupPath field on GameManifest and rawGameManifest
- `games/minecraft/manifest.yaml` - Added backupPath: /data
- `internal/controller/gameserver_controller.go` - AdminConfig S3 fields (endpoint, bucket, region, SSL, retention count)
- `cmd/main.go` - BackupReconciler wired into operator manager
- `go.mod` / `go.sum` - minio-go/v7 and robfig/cron/v3 dependencies
- `config/crd/bases/game.kterodactyl.io_backups.yaml` - Generated Backup CRD manifest
- `config/rbac/role.yaml` - Updated RBAC with backup resource permissions
- `api/v1alpha1/zz_generated.deepcopy.go` - Regenerated deepcopy methods

## Decisions Made
- Operator-driven backup over CronJob approach for simplicity in homelab context
- S3 credentials stored in Secret (kterodactyl-s3-credentials), S3 config in AdminConfig ConfigMap
- Synthetic reconcile request pattern (schedule-<gsname>) for GameServer watch-triggered schedule reconciliation
- Lazy S3 client initialization to avoid startup failures when S3 not yet configured
- Auto-create S3 bucket on first backup for improved admin setup experience
- 30-minute context timeout for backup operations to handle large game data
- 64MB multipart upload part size for efficient streaming

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Moved BackupReconciler registration after clientset creation in main.go**
- **Found during:** Task 2 (wiring BackupReconciler into manager)
- **Issue:** Plan placed BackupReconciler registration alongside other controllers, but clientset/restConfig are defined later in main.go
- **Fix:** Moved registration to after clientset creation, before API server setup
- **Files modified:** cmd/main.go
- **Verification:** `go build ./...` compiles cleanly
- **Committed in:** 8b52d34 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Necessary ordering fix for compilation. No scope creep.

## Issues Encountered
- Go binary not in default PATH (found at /home/tony/sdk/go1.24/bin/go with version 1.25.3) -- resolved by setting PATH explicitly for all build commands

## User Setup Required
None - no external service configuration required for this plan (S3 configuration is documented for admin setup but not required at build time).

## Next Phase Readiness
- Backup CRD and controller are ready for API endpoint integration (Plan 02)
- Frontend backup UI can be built against the Backup CRD types (Plan 03)
- S3 credentials Secret and AdminConfig S3 fields need to be configured by admin before backups will function

## Self-Check: PASSED

All 13 claimed files verified present on disk. Both task commits (b4291ac, 8b52d34) verified in git history.

---
*Phase: 09-backup-system*
*Completed: 2026-02-13*
