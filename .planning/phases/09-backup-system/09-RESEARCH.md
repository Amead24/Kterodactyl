# Phase 9: Backup System - Research

**Researched:** 2026-02-12
**Domain:** S3-compatible object storage, Kubernetes CRD design, tar-based backup/restore, scheduled CronJobs, operator reconciliation
**Confidence:** HIGH

## Summary

Phase 9 adds a backup system to Kterodactyl, enabling users to create on-demand backups of game server data, admins to configure scheduled backups via cron schedules, and admins to restore game servers from backups. Backups are stored in S3-compatible object storage (MinIO, AWS S3, GCS) and tracked via a new Backup CRD.

The architecture introduces a new `Backup` CRD in the same `game.kterodactyl.io` API group and a `BackupReconciler` controller. The backup process uses the existing tar-over-exec pattern (proven in Phase 8 mod upload) to extract game server data from running pods, then streams the tar archive to S3 via the `minio-go/v7` client. Scheduled backups are handled by the operator itself using a timer-based approach within the reconciliation loop (not Kubernetes CronJobs), keeping the architecture simpler and avoiding cross-namespace CronJob management complexity. The Backup CRD tracks state (Pending/InProgress/Completed/Failed), size, S3 location, and timing metadata. Restore creates a new backup Job that downloads from S3 and extracts into the game server pod.

The key architectural decision is **how the operator performs backups**: the operator pod itself runs the backup logic (tar from pod via exec, stream to S3) rather than spawning separate Jobs/CronJobs. This is simpler for a homelab context, avoids needing S3 credentials distributed to Job pods, and reuses the exec infrastructure already in the codebase. For scheduled backups, the controller stores a `backupSchedule` annotation on GameServer CRDs and uses a periodic reconciliation requeue to check if a backup is due.

**Primary recommendation:** Add a Backup CRD with BackupReconciler that uses tar-over-exec to capture game data and minio-go/v7 to upload to S3. Store S3 configuration in AdminConfig ConfigMap and S3 credentials in a Secret. Implement scheduling via annotations on GameServer + periodic requeue in the BackupReconciler.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/minio/minio-go/v7` | v7.0.98 | S3-compatible object storage client | De facto Go SDK for S3-compatible storage; works with MinIO, AWS S3, GCS; 2800+ importers |
| `archive/tar` | stdlib | Create/extract tar archives for backup data | Already used in mod upload (Phase 8); proven pattern in this codebase |
| `compress/gzip` | stdlib | Compress tar archives before S3 upload | Standard Go compression; reduces storage costs and upload time |
| `k8s.io/client-go/tools/remotecommand` | v0.35.1 | SPDY exec for streaming data from pods | Already used for console and mod upload; proven in this codebase |
| `k8s.io/api/batch/v1` | v0.35.1 | CronJob/Job types (available but not primary approach) | Already in k8s.io/api dependency; used if CronJob approach is chosen |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `io` | stdlib | Pipe-based streaming (io.Pipe) | Connect tar reader to S3 upload without buffering entire archive |
| `path/filepath` | stdlib | Safe path construction for backup file naming | Backup naming with timestamps |
| `time` | stdlib | Cron schedule parsing, timestamp tracking | Next backup time calculation |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| minio-go/v7 | aws-sdk-go-v2 | AWS SDK is larger, more complex; minio-go is simpler, works with all S3-compatible storage including MinIO |
| Operator-driven backup | Kubernetes CronJob | CronJob requires distributing S3 credentials to Job pods, cross-namespace management, and separate backup container image |
| tar-over-exec from operator | Sidecar container approach | Sidecar uses resources permanently; exec-based is on-demand |
| gzip compression | zstd | zstd is faster but requires external dependency; gzip is stdlib |

### Installation
```bash
go get github.com/minio/minio-go/v7
```

No new frontend dependencies needed; backup UI uses existing React patterns with shadcn/ui components.

## Architecture Patterns

### Recommended Changes to Existing Files

```
api/v1alpha1/
  backup_types.go              # (new) Backup CRD type definitions
  backup_lifecycle.go          # (new) Backup state machine constants
  groupversion_info.go         # (modify) register Backup type with scheme
  zz_generated.deepcopy.go    # (regenerated) via make generate
internal/
  controller/
    backup_controller.go       # (new) BackupReconciler with S3 operations
    backup_controller_test.go  # (new) controller tests
    gameserver_controller.go   # (modify) add backup-related AdminConfig fields
  api/
    handlers_backups.go        # (new) backup CRUD + trigger + restore endpoints
    routes.go                  # (modify) add backup routes
    server.go                  # (modify) no change needed - uses existing client
  util/
    labels.go                  # (modify) add backup-related labels/annotations
config/
  crd/                         # (regenerated) via make manifests
  rbac/                        # (regenerated) via make manifests
  samples/
    game_v1alpha1_backup.yaml  # (new) sample Backup CR
web/src/
  api/backups.ts               # (new) backup API client functions
  types/api.ts                 # (modify) add BackupResponse, BackupRequest types
  components/backups/
    backup-list.tsx            # (new) list backups with status, size, date
    backup-trigger.tsx         # (new) trigger on-demand backup button
    backup-schedule.tsx        # (new) schedule configuration (admin only)
    restore-dialog.tsx         # (new) restore confirmation dialog (admin only)
  pages/server-detail.tsx      # (modify) add Backups tab
```

### Pattern 1: Backup CRD Type Design
**What:** A new Kubernetes CRD in the same API group (`game.kterodactyl.io/v1alpha1`) that tracks backup state, references a GameServer, and stores S3 metadata.
**When to use:** Every backup operation creates a Backup CR.
**Example:**
```go
// Source: Kubebuilder CRD pattern + existing GameServer types in this codebase
type BackupSpec struct {
    // GameServerName references the GameServer to back up.
    // +kubebuilder:validation:MinLength=1
    GameServerName string `json:"gameServerName"`

    // BackupPaths lists container paths to include in the backup.
    // If empty, defaults to the game's data directory.
    // +optional
    BackupPaths []string `json:"backupPaths,omitempty"`
}

type BackupStatus struct {
    // State is the current backup lifecycle state.
    // +kubebuilder:validation:Enum=Pending;InProgress;Completed;Failed
    State BackupState `json:"state,omitempty"`

    // S3Key is the object key in the S3 bucket where the backup is stored.
    S3Key string `json:"s3Key,omitempty"`

    // S3Bucket is the S3 bucket name.
    S3Bucket string `json:"s3Bucket,omitempty"`

    // Size is the backup size in bytes.
    Size int64 `json:"size,omitempty"`

    // StartedAt is when the backup started.
    StartedAt *metav1.Time `json:"startedAt,omitempty"`

    // CompletedAt is when the backup completed.
    CompletedAt *metav1.Time `json:"completedAt,omitempty"`

    // Message provides human-readable status details.
    Message string `json:"message,omitempty"`

    // Conditions represent the latest observations.
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}
```

### Pattern 2: Backup Process (tar-from-pod + S3 upload)
**What:** Extract game data from a running pod via exec, pipe through gzip, stream to S3.
**When to use:** When a Backup CR transitions from Pending to InProgress.
**Key details:**
- Use exec to run `tar -cf - -C /data .` in the game server pod (capture all game data)
- Pipe stdout through `gzip.NewWriter` for compression
- Use `io.Pipe` to connect gzip output to `minio.PutObject` input
- S3 object key format: `backups/<namespace>/<gameserver-name>/<timestamp>.tar.gz`
- Track upload size via a counting writer wrapper
- Update Backup status with S3 key, size, completion time on success

### Pattern 3: S3 Client Configuration via AdminConfig + Secret
**What:** S3 endpoint/bucket in AdminConfig ConfigMap, credentials in a separate Secret.
**When to use:** BackupReconciler initialization and per-reconciliation.
**Key details:**
- AdminConfig fields: `backupS3Endpoint`, `backupS3Bucket`, `backupS3Region`, `backupS3UseSSL`
- Secret name: `kterodactyl-s3-credentials` in operator namespace
- Secret keys: `accessKeyID`, `secretAccessKey`
- Create minio.Client lazily on first backup, cache on reconciler struct
- If S3 not configured, backups fail gracefully with clear error message

### Pattern 4: Scheduled Backups via GameServer Annotation
**What:** Admin sets a cron schedule annotation on a GameServer; BackupReconciler periodically creates Backup CRs on schedule.
**When to use:** Admin configures recurring backups for a game server.
**Key details:**
- Annotation: `kterodactyl.io/backup-schedule` with cron expression (e.g., `0 3 * * *`)
- Annotation: `kterodactyl.io/backup-retention` with max backup count (e.g., `5`)
- BackupReconciler watches GameServers too (via Watches), checking schedules
- On each reconcile of a GameServer with a schedule, check if it is time for a new backup
- If due, create a new Backup CR referencing that GameServer
- Clean up old backups beyond retention count (delete Backup CR + S3 object)
- Use `RequeueAfter` to wake up near the next scheduled time

### Pattern 5: Restore Process (S3 download + tar-into-pod)
**What:** Download backup from S3, extract into a game server pod to restore game state.
**When to use:** Admin triggers restore of a specific backup.
**Key details:**
- Server must be in Ready or Shutdown state for restore
- If Shutdown, start the server first (transition to Creating), wait for Ready
- Download from S3 via `minio.GetObject`
- Pipe through `gzip.NewReader` then `tar -xf - -C /data` via exec
- After restore, restart the game server (transition to Creating) so it picks up restored data
- This is an admin-only operation due to destructive nature

### Anti-Patterns to Avoid
- **Buffering entire backup in memory:** Use `io.Pipe` streaming. Game server data can be gigabytes. Never read it all into memory.
- **Storing S3 credentials in AdminConfig ConfigMap:** ConfigMaps are not for secrets. Use a proper Kubernetes Secret.
- **Creating CronJobs in user namespaces:** Requires distributing S3 credentials across namespaces, complex RBAC, and separate backup container images. Keep backup logic in the operator.
- **Backup without server state check:** Backup during server startup or mid-save can produce inconsistent data. Only backup from Ready or Allocated state.
- **Ignoring backup retention:** Unbounded backups consume S3 storage forever. Always enforce retention limits.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| S3 multipart upload | Custom HTTP S3 client | `minio-go/v7` PutObject | Multipart upload, retry logic, signature handling are complex |
| Tar archive creation/extraction | Custom binary format | `archive/tar` stdlib | Tar format handles permissions, directories, symlinks correctly |
| Gzip compression | Custom compression | `compress/gzip` stdlib | Battle-tested, efficient, universally understood format |
| Cron schedule parsing | Custom parser | `github.com/robfig/cron/v3` or manual next-time calculation | Cron format is deceptively complex (day-of-week, ranges, steps) |
| S3 credential management | Custom encryption | Kubernetes Secret | K8s Secret encryption at rest, RBAC-controlled access |

**Key insight:** The backup pipeline is essentially three pipes connected: exec stdout -> gzip -> S3 upload. Each pipe has a well-tested library. The value is in wiring them together correctly, not reimplementing any piece.

## Common Pitfalls

### Pitfall 1: S3 PutObject Requires Content-Length or Streaming
**What goes wrong:** minio-go PutObject with -1 size (unknown) triggers multipart upload, which may fail with some S3-compatible providers if parts are too small.
**Why it happens:** Backup size is unknown before starting the tar stream.
**How to avoid:** Use -1 for size (minio-go handles multipart automatically) but set a reasonable `PartSize` in PutObjectOptions (e.g., 64MB). Alternatively, buffer to a temp file first if backups are expected to be small (< 5GB).
**Warning signs:** "EntityTooSmall" errors from S3 provider.

### Pitfall 2: Backup of Non-Running Server
**What goes wrong:** exec into pod fails because pod does not exist in Shutdown/Error state.
**Why it happens:** User triggers backup when server is stopped.
**How to avoid:** Check GameServer state before backup. Only allow backups when state is Ready or Allocated (pod is running). Return clear error message otherwise. For scheduled backups, skip if server is not running.
**Warning signs:** "pod not found" or "container not running" errors during backup exec.

### Pitfall 3: Concurrent Backups of Same Server
**What goes wrong:** Two backup operations run simultaneously, competing for exec connections and producing potentially corrupt archives.
**Why it happens:** User triggers manual backup while scheduled backup is running.
**How to avoid:** Check for existing InProgress backups for the same GameServer before starting a new one. BackupReconciler should skip if another backup is already InProgress.
**Warning signs:** Multiple "InProgress" Backup CRs for the same GameServer.

### Pitfall 4: Missing RBAC for Backup CRD
**What goes wrong:** Controller gets "forbidden" errors when creating/updating Backup CRs.
**Why it happens:** Kubebuilder RBAC markers not added for the new Backup resource.
**How to avoid:** Add RBAC markers to backup_controller.go: `// +kubebuilder:rbac:groups=game.kterodactyl.io,resources=backups,verbs=get;list;watch;create;update;patch;delete` and `// +kubebuilder:rbac:groups=game.kterodactyl.io,resources=backups/status,verbs=get;update;patch`. Then run `make manifests`.
**Warning signs:** "backups.game.kterodactyl.io is forbidden" in controller logs.

### Pitfall 5: S3 Credentials Not Configured
**What goes wrong:** Backup immediately fails with cryptic minio-go error.
**Why it happens:** Admin hasn't created the S3 credentials Secret or configured S3 endpoint in AdminConfig.
**How to avoid:** On BackupReconciler startup or first backup, validate S3 configuration. Set Backup state to Failed with clear message: "S3 not configured: missing backupS3Endpoint in admin config" or "S3 credentials secret not found".
**Warning signs:** All backups immediately fail with no S3-related fields populated.

### Pitfall 6: Backup Data Path Varies by Game
**What goes wrong:** Backing up wrong directory; restore overwrites wrong files.
**Why it happens:** Different games store data in different directories (Minecraft: `/data`, Valheim: `/config/worlds`).
**How to avoid:** Add a `backupPath` field to game manifests (like `modPath`). Store as annotation on GameServer CR. BackupReconciler reads this annotation to determine what to tar.
**Warning signs:** Backup is empty or missing game save data; restore doesn't affect game state.

### Pitfall 7: Deepcopy Generation After Adding New Types
**What goes wrong:** Build fails with missing DeepCopyObject method.
**Why it happens:** New Backup CRD types need generated deepcopy methods.
**How to avoid:** After adding `backup_types.go`, run `make generate` before `make manifests`. The `controller-gen` tool generates `zz_generated.deepcopy.go`.
**Warning signs:** Compile error: "Backup does not implement runtime.Object" or "missing method DeepCopyObject".

### Pitfall 8: Kubebuilder Scaffold Updates PROJECT File
**What goes wrong:** Running `kubebuilder create api` modifies PROJECT file and creates unexpected boilerplate.
**Why it happens:** Kubebuilder scaffolding is designed for fresh resources.
**How to avoid:** Either use `kubebuilder create api --group game --version v1alpha1 --kind Backup` and accept the scaffolding, or manually create the type files following the existing GameServer pattern. If using kubebuilder, review generated files and clean up unnecessary boilerplate.
**Warning signs:** Unexpected file changes in PROJECT, config/crd, config/rbac.

### Pitfall 9: PVC Spec Immutability (Backup Storage PVC)
**What goes wrong:** Attempting to resize or modify PVC spec fails.
**Why it happens:** Kubernetes PVC specs are immutable after creation.
**How to avoid:** This is already handled in the codebase pattern (CreationTimestamp.IsZero check). If backup storage PVCs are needed, use the same pattern. However, for this design, backups go directly to S3, so no PVC is needed for backup storage.
**Warning signs:** "spec is immutable after creation" error from PVC update.

## Code Examples

Verified patterns from official sources and codebase analysis:

### Go: minio-go Client Creation
```go
// Source: github.com/minio/minio-go/v7 - pkg.go.dev v7.0.98
import (
    "github.com/minio/minio-go/v7"
    "github.com/minio/minio-go/v7/pkg/credentials"
)

func newS3Client(endpoint, accessKey, secretKey string, useSSL bool) (*minio.Client, error) {
    return minio.New(endpoint, &minio.Options{
        Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
        Secure: useSSL,
    })
}
```

### Go: Backup Pipeline (tar-from-exec + gzip + S3 upload)
```go
// Source: Codebase pattern from handlers_mods.go execInPod + minio-go PutObject
func (r *BackupReconciler) performBackup(ctx context.Context, gs *gamev1alpha1.GameServer, backup *gamev1alpha1.Backup) error {
    // 1. Set up pipe: exec stdout -> gzip -> S3 upload
    pr, pw := io.Pipe()

    // 2. Gzip wrapper on the pipe writer
    gzWriter := gzip.NewWriter(pw)

    // 3. Background goroutine: exec tar in pod, pipe through gzip
    errCh := make(chan error, 1)
    go func() {
        defer pw.Close()
        defer gzWriter.Close()

        // exec: tar -cf - -C <backupPath> .
        // stdout goes to gzWriter which goes to pw
        err := r.execTarFromPod(ctx, gs.Namespace, gs.Name, backupPath, gzWriter)
        errCh <- err
    }()

    // 4. Main goroutine: upload pipe reader to S3
    s3Key := fmt.Sprintf("backups/%s/%s/%s.tar.gz",
        gs.Namespace, gs.Name, time.Now().UTC().Format("20060102-150405"))

    info, err := r.s3Client.PutObject(ctx, r.s3Bucket, s3Key, pr, -1,
        minio.PutObjectOptions{
            ContentType: "application/gzip",
            PartSize:    64 * 1024 * 1024, // 64MB parts
        })
    if err != nil {
        return fmt.Errorf("S3 upload failed: %w", err)
    }

    // 5. Check exec goroutine error
    if execErr := <-errCh; execErr != nil {
        return fmt.Errorf("tar exec failed: %w", execErr)
    }

    // 6. Update backup status
    backup.Status.S3Key = s3Key
    backup.Status.S3Bucket = r.s3Bucket
    backup.Status.Size = info.Size
    return nil
}
```

### Go: Restore Pipeline (S3 download + gunzip + tar-into-pod)
```go
// Source: Inverse of backup pipeline; minio-go GetObject + existing execInPod pattern
func (r *BackupReconciler) performRestore(ctx context.Context, gs *gamev1alpha1.GameServer, backup *gamev1alpha1.Backup) error {
    // 1. Download from S3
    obj, err := r.s3Client.GetObject(ctx, backup.Status.S3Bucket, backup.Status.S3Key, minio.GetObjectOptions{})
    if err != nil {
        return fmt.Errorf("S3 download failed: %w", err)
    }
    defer obj.Close()

    // 2. Decompress gzip
    gzReader, err := gzip.NewReader(obj)
    if err != nil {
        return fmt.Errorf("gzip decompression failed: %w", err)
    }
    defer gzReader.Close()

    // 3. Exec tar extraction in pod: tar -xf - -C <backupPath>
    _, stderr, err := r.execInPod(ctx, gs.Namespace, gs.Name,
        []string{"tar", "-xf", "-", "-C", backupPath}, gzReader)
    if err != nil {
        return fmt.Errorf("tar restore failed: %s: %w", stderr, err)
    }

    return nil
}
```

### Go: Backup CRD Registration (same API group)
```go
// Source: Existing groupversion_info.go pattern in this codebase
// In api/v1alpha1/groupversion_info.go, Backup is already registered via init()
// in backup_types.go:
func init() {
    SchemeBuilder.Register(&Backup{}, &BackupList{})
}
```

### Go: AdminConfig S3 Fields
```go
// Source: Existing AdminConfig pattern in gameserver_controller.go
// Add to AdminConfig struct:
type AdminConfig struct {
    // ... existing fields ...

    // Backup S3 configuration
    BackupS3Endpoint string // S3-compatible endpoint (e.g., "minio.local:9000")
    BackupS3Bucket   string // S3 bucket name for backups
    BackupS3Region   string // S3 region (default: "us-east-1")
    BackupS3UseSSL   bool   // Use TLS for S3 connections

    // Backup defaults
    BackupRetentionCount int            // Max backups to retain per server (default: 5)
    BackupStorageSize    resource.Quantity // Not used (S3 storage, not PVC) -- reserved for future
}
```

### Go: Kubebuilder Command to Scaffold Backup CRD
```bash
# From project root:
kubebuilder create api --group game --version v1alpha1 --kind Backup --resource --controller
# This creates:
#   api/v1alpha1/backup_types.go
#   internal/controller/backup_controller.go
#   internal/controller/backup_controller_test.go
#   Updates PROJECT file
#   Updates cmd/main.go (controller registration)

# Then regenerate:
make generate  # deepcopy methods
make manifests # CRD YAML + RBAC
```

### Frontend: Backup API Types
```typescript
// Source: Existing pattern from types/api.ts
export interface BackupResponse {
  name: string;
  gameServerName: string;
  state: 'Pending' | 'InProgress' | 'Completed' | 'Failed';
  s3Key: string;
  s3Bucket: string;
  size: number;
  startedAt?: string;
  completedAt?: string;
  message?: string;
  createdAt: string;
}

export interface CreateBackupRequest {
  gameServerName: string;
}

export interface RestoreRequest {
  backupName: string;
}
```

### Frontend: Backup API Client
```typescript
// Source: Existing pattern from api/servers.ts
export function createBackup(serverName: string): Promise<BackupResponse> {
  return apiFetch<BackupResponse>(`/gameservers/${serverName}/backups`, {
    method: 'POST',
  });
}

export function listBackups(serverName: string): Promise<ListResponse<BackupResponse>> {
  return apiFetch<ListResponse<BackupResponse>>(`/gameservers/${serverName}/backups`);
}

export function restoreBackup(serverName: string, backupName: string): Promise<void> {
  return apiFetch<void>(`/gameservers/${serverName}/backups/${backupName}/restore`, {
    method: 'POST',
  });
}

export function deleteBackup(serverName: string, backupName: string): Promise<void> {
  return apiFetch<void>(`/gameservers/${serverName}/backups/${backupName}`, {
    method: 'DELETE',
  });
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Velero full-cluster backup | Per-resource targeted backup | 2022+ | Fine-grained control, faster, less storage |
| kubectl cp for file extraction | tar-over-exec (same mechanism) | Stable since K8s 1.5+ | Standard, unchanged pattern |
| AWS SDK v1 | minio-go v7 / AWS SDK v2 | 2023+ | minio-go simpler for S3-compatible; AWS SDK v2 for AWS-only |
| PVC snapshots for backup | S3 streaming backup | Depends on use case | S3 is more portable, PVC snapshots faster but provider-dependent |

**Deprecated/outdated:**
- `aws-sdk-go` v1: Deprecated in favor of aws-sdk-go-v2, but minio-go is still the better choice for S3-compatible storage
- `minio-go` v6: Superseded by v7 with context support and simplified API

## Open Questions

1. **Backup data path per game**
   - What we know: Minecraft uses `/data`, but other games vary. The `modPath` pattern already solves a similar problem.
   - What's unclear: Should we add `backupPath` to game manifests, or use a convention?
   - Recommendation: Add `backupPath` field to game manifests (same pattern as `modPath`). Default to `/data` if not specified. Store as annotation `kterodactyl.io/backup-path` on GameServer CR.

2. **Scheduled backup approach: operator timer vs CronJob**
   - What we know: CronJobs require separate pods, credential distribution, and more complexity. Operator-based scheduling is simpler.
   - What's unclear: Whether operator-based scheduling scales well with many servers.
   - Recommendation: Use operator-based scheduling with annotations + requeue. For a homelab with < 100 servers, this is sufficient. Can migrate to CronJobs later if needed.

3. **Backup during active gameplay**
   - What we know: Some games have inconsistent state when files are read during active gameplay (saves mid-write).
   - What's unclear: Whether game servers support "safe save" commands.
   - Recommendation: For v1, document that backups are best taken during low-activity periods. Future enhancement could send a save command via exec before backup (e.g., `rcon save-all` for Minecraft). Do not block on this for Phase 9.

4. **Maximum backup size**
   - What we know: Game server data can range from MBs to tens of GBs (heavily modded Minecraft worlds).
   - What's unclear: S3 upload timeout behavior with very large backups.
   - Recommendation: No hard limit. minio-go handles multipart uploads automatically. Set a generous context timeout (30 minutes). Log progress periodically.

5. **S3 bucket creation**
   - What we know: minio-go can create buckets via `MakeBucket`.
   - What's unclear: Whether the operator should auto-create the bucket or require admin pre-creation.
   - Recommendation: Auto-create bucket on first backup if it doesn't exist (BucketExists check + MakeBucket). This improves setup experience.

## Sources

### Primary (HIGH confidence)
- Codebase analysis: `internal/api/handlers_mods.go` -- existing tar-over-exec pattern, execInPod helper, SPDY exec setup
- Codebase analysis: `internal/controller/gameserver_controller.go` -- existing reconciliation loop, AdminConfig loading, state machine, CreateOrUpdate pattern, PVC management
- Codebase analysis: `api/v1alpha1/gameserver_types.go` -- CRD type definition patterns, kubebuilder markers, status subresource
- Codebase analysis: `api/v1alpha1/gameserver_lifecycle.go` -- state machine pattern with ValidTransitions, condition types
- Codebase analysis: `internal/manifest/manifest.go` -- GameManifest with modPath field pattern
- Codebase analysis: `cmd/main.go` -- controller registration pattern, scheme setup
- Codebase analysis: `PROJECT` -- Kubebuilder project configuration for scaffolding
- [minio-go/v7 pkg.go.dev](https://pkg.go.dev/github.com/minio/minio-go/v7) -- v7.0.98 API: PutObject, GetObject, MakeBucket, BucketExists function signatures
- [Kubernetes CronJob docs](https://kubernetes.io/docs/concepts/workloads/controllers/cron-jobs/) -- CronJob spec structure, schedule format, concurrency policy
- [k8s.io/api/batch/v1](https://pkg.go.dev/k8s.io/api/batch/v1) -- CronJob/Job Go type definitions

### Secondary (MEDIUM confidence)
- [Kubebuilder: Adding a new API](https://book.kubebuilder.io/cronjob-tutorial/new-api) -- `kubebuilder create api` for same-group new kinds
- [minio/minio-go GitHub](https://github.com/minio/minio-go) -- SDK examples, client initialization patterns
- [IBM Operator Sample: Database Backup](https://ibm.github.io/operator-sample-go-documentation/demos-database-backup/) -- operator-driven CronJob backup pattern reference

### Tertiary (LOW confidence)
- Backup-during-gameplay safety: Game-specific behavior (Minecraft save commands, etc.) needs per-game validation. The general recommendation to backup during low activity is sound but not verified per game engine.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- minio-go/v7 is the de facto Go S3 client (verified via pkg.go.dev); tar/gzip are stdlib; exec is already proven in this codebase
- Architecture: HIGH -- Backup CRD follows exact same pattern as existing GameServer CRD; tar-over-exec is proven in Phase 8 mod upload; operator-driven backup avoids complexity of CronJob approach
- Pitfalls: HIGH -- Identified from codebase analysis (RBAC markers, PVC immutability, exec state checks) and S3 client documentation (content-length, multipart)
- Code examples: HIGH -- Based on existing codebase patterns (execInPod, AdminConfig, CRD types) combined with verified minio-go API signatures

**Research date:** 2026-02-12
**Valid until:** 2026-03-14 (stable patterns; minio-go v7 is mature, Kubernetes CRD patterns are stable)
