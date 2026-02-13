---
sidebar_position: 2
---

# CRD Reference

Kterodactyl defines two Custom Resource Definitions (CRDs): **GameServer** and **Backup**. Both are in the `game.kterodactyl.io` API group at version `v1alpha1`.

## GameServer

A GameServer represents a single game server instance managed by the operator.

- **API Group:** `game.kterodactyl.io`
- **Version:** `v1alpha1`
- **Kind:** `GameServer`
- **Short Name:** `gs`
- **Scope:** Namespaced

### Spec Fields

| Field | Type | Required | Validation | Description |
|-------|------|----------|------------|-------------|
| `gameType` | string | Yes | MinLength=1, MaxLength=63, Pattern=`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$` | References the game definition (e.g., `minecraft`, `valheim`). Must be a valid DNS label. |
| `image` | string | Yes | MinLength=1 | Container image to run for the game server. |
| `resources` | ResourceRequirements | No | -- | CPU and memory requests and limits for the game container. |
| `ports` | GameServerPort[] | No | -- | Ports exposed by the game server. |
| `parameters` | map[string]string | No | -- | Game-specific configuration passed as environment variables. |

### GameServerPort

| Field | Type | Required | Validation | Description |
|-------|------|----------|------------|-------------|
| `name` | string | Yes | MinLength=1 | Descriptive identifier for the port (e.g., `game`). |
| `containerPort` | int32 | Yes | Min=1, Max=65535 | Port number on the container. |
| `protocol` | string | No | Enum: TCP, UDP. Default: TCP | Network protocol. |

### Status Fields

| Field | Type | Description |
|-------|------|-------------|
| `state` | string | Current lifecycle state: `Creating`, `Starting`, `Ready`, `Allocated`, `Shutdown`, or `Error`. |
| `address` | string | Connection address for the game server (DNS name when base domain is configured). |
| `ports` | GameServerStatusPort[] | Allocated external ports for the game server. |
| `conditions` | Condition[] | Standard Kubernetes conditions: `Ready`, `Progressing`, `Degraded`. |

### GameServerStatusPort

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Descriptive identifier matching the spec port name. |
| `port` | int32 | Allocated external port number. |
| `protocol` | string | Network protocol (TCP or UDP). |

### Print Columns

When using `kubectl get gameservers`:

```
NAME         GAME        STATE    ADDRESS                              AGE
my-server    minecraft   Ready    minecraft.alice.tonymead.org         5m
```

| Column | Source |
|--------|--------|
| Game | `.spec.gameType` |
| State | `.status.state` |
| Address | `.status.address` |
| Age | `.metadata.creationTimestamp` |

### Example

```yaml
apiVersion: game.kterodactyl.io/v1alpha1
kind: GameServer
metadata:
  name: my-minecraft
  namespace: user-alice
spec:
  gameType: minecraft
  image: itzg/minecraft-server:latest
  resources:
    requests:
      cpu: "500m"
      memory: "1Gi"
    limits:
      cpu: "2"
      memory: "4Gi"
  ports:
    - name: game
      containerPort: 25565
      protocol: TCP
  parameters:
    EULA: "TRUE"
    TYPE: "VANILLA"
    DIFFICULTY: "normal"
    MAX_PLAYERS: "20"
```

### Status Subresource

The GameServer uses a [status subresource](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#status-subresource), meaning:

- `spec` and `status` are updated independently via separate API calls
- The operator updates `status` via `Status().Update()` after reconciliation
- Users can update `spec.parameters` without affecting the status

---

## Backup

A Backup represents a point-in-time snapshot of a game server's data stored in S3-compatible storage.

- **API Group:** `game.kterodactyl.io`
- **Version:** `v1alpha1`
- **Kind:** `Backup`
- **Short Name:** `bk`
- **Scope:** Namespaced

### Spec Fields

| Field | Type | Required | Validation | Description |
|-------|------|----------|------------|-------------|
| `gameServerName` | string | Yes | MinLength=1 | References the GameServer to back up. |
| `backupPaths` | string[] | No | -- | Container paths to include in the backup. If empty, uses the `backupPath` annotation from the GameServer. |

### Status Fields

| Field | Type | Description |
|-------|------|-------------|
| `state` | string | Current backup state: `Pending`, `InProgress`, `Completed`, or `Failed`. |
| `s3Key` | string | Object key in the S3 bucket. |
| `s3Bucket` | string | S3 bucket name where the backup is stored. |
| `size` | int64 | Backup size in bytes. |
| `startedAt` | Time | Timestamp when the backup process began. |
| `completedAt` | Time | Timestamp when the backup finished. |
| `message` | string | Human-readable status details (e.g., error messages on failure). |
| `conditions` | Condition[] | Standard Kubernetes conditions. |

### Print Columns

When using `kubectl get backups`:

```
NAME                    GAMESERVER     STATE       SIZE       AGE
my-minecraft-backup-1   my-minecraft   Completed   15728640   2h
```

| Column | Source |
|--------|--------|
| GameServer | `.spec.gameServerName` |
| State | `.status.state` |
| Size | `.status.size` |
| Age | `.metadata.creationTimestamp` |

### Example

```yaml
apiVersion: game.kterodactyl.io/v1alpha1
kind: Backup
metadata:
  name: my-minecraft-backup-1
  namespace: user-alice
spec:
  gameServerName: my-minecraft
  backupPaths:
    - /data
```

### Status Subresource

Like GameServer, Backup uses a status subresource. The Backup Controller updates the status as the backup progresses through its lifecycle states.

---

## Common Patterns

### Owner References

GameServer resources set owner references on all resources they create (Pod, Service, HTTPRoute, PVC). When a GameServer is deleted, Kubernetes garbage collection automatically cleans up all owned resources.

### Finalizers

GameServers use the finalizer `game.kterodactyl.io/finalizer` to ensure cleanup logic runs before the resource is deleted. The operator removes the finalizer after completing cleanup.

### Labels and Annotations

The operator uses labels and annotations for resource management:

| Label/Annotation | Purpose |
|-----------------|---------|
| `kterodactyl.io/gameserver` | Links child resources back to the owning GameServer |
| `kterodactyl.io/game-type` | Game type identifier for filtering |
| `kterodactyl.io/mod-path` | (annotation) Container path for mod uploads |
| `kterodactyl.io/backup-path` | (annotation) Container path for backups |
| `kterodactyl.io/backup-schedule` | (annotation) Cron schedule for automatic backups |
