---
sidebar_position: 4
title: Backups
---

# Backups

Kterodactyl includes a built-in backup system that stores game server data in S3-compatible object storage. Backups can be created on demand or on a schedule.

## How Backups Work

When a backup is triggered, the operator:

1. Reads the game data from the running pod (using the game manifest's `backupPath`, defaulting to `/data`)
2. Creates a tar archive of the directory contents
3. Compresses the archive with gzip
4. Uploads the compressed archive to S3
5. Records the backup metadata (size, S3 key, timestamps) on a Backup custom resource

Backups are created as Kubernetes custom resources (`Backup`) that track the state of each backup operation through four states: `Pending`, `InProgress`, `Completed`, and `Failed`.

## Setup

### 1. Enable Backups

Set `adminConfig.backup.enabled` to `true` in your Helm values:

```yaml
adminConfig:
  backup:
    enabled: true
    s3:
      endpoint: "minio.minio-system.svc.cluster.local:9000"
      bucket: "kterodactyl-backups"
      region: "us-east-1"
      useSSL: "false"
    retentionCount: "5"
```

### 2. Create S3 Credentials Secret

The S3 access key and secret key are stored in a Kubernetes Secret:

```bash
kubectl create secret generic kterodactyl-s3-credentials \
  -n kterodactyl-system \
  --from-literal=access-key=YOUR_ACCESS_KEY \
  --from-literal=secret-key=YOUR_SECRET_KEY
```

### 3. Configure Endpoint and Bucket

| Setting | Description |
|---|---|
| `s3.endpoint` | The S3-compatible storage endpoint (hostname and port) |
| `s3.bucket` | Bucket name for storing backups (auto-created on first backup) |
| `s3.region` | S3 region identifier |
| `s3.useSSL` | Whether to use HTTPS for S3 connections |

:::tip Bucket Auto-Creation
The backup bucket is automatically created on the first backup if it does not already exist. You do not need to create it manually.
:::

## S3-Compatible Storage Options

### MinIO (Homelab)

[MinIO](https://min.io/) is a lightweight, self-hosted S3-compatible storage server. It runs as a single pod in your cluster:

```yaml
adminConfig:
  backup:
    enabled: true
    s3:
      endpoint: "minio.minio-system.svc.cluster.local:9000"
      bucket: "kterodactyl-backups"
      region: "us-east-1"
      useSSL: "false"
```

### AWS S3

```yaml
adminConfig:
  backup:
    enabled: true
    s3:
      endpoint: "s3.amazonaws.com"
      bucket: "my-kterodactyl-backups"
      region: "us-west-2"
      useSSL: "true"
```

### Google Cloud Storage

GCS provides an S3-compatible endpoint via the interoperability API:

```yaml
adminConfig:
  backup:
    enabled: true
    s3:
      endpoint: "storage.googleapis.com"
      bucket: "my-kterodactyl-backups"
      region: "auto"
      useSSL: "true"
```

## Retention

The `retentionCount` setting controls how many backups are kept per game server. When a new backup completes and the count exceeds the retention limit, the oldest backup is automatically deleted from both S3 and the Kubernetes API.

```yaml
adminConfig:
  backup:
    retentionCount: "5"
```

Set a higher value if you want more backup history, or a lower value to save storage space.

## Scheduled Backups

Admins can set a cron schedule for automatic backups on a per-server basis via the API. The schedule is stored as an annotation on the GameServer resource and evaluated by the Backup controller.

```bash
curl -X PUT http://localhost:8080/api/v1/gameservers/my-minecraft/backup-schedule \
  -H "Authorization: Bearer <admin-token>" \
  -H "Content-Type: application/json" \
  -d '{"schedule":"0 */6 * * *"}'
```

This example creates a backup every 6 hours. The schedule uses standard cron syntax.

## Restore

Admins can restore a backup to its original game server. The restore process downloads the backup from S3, decompresses it, and extracts the files into the running pod:

```bash
curl -X POST http://localhost:8080/api/v1/gameservers/my-minecraft/backups/backup-123/restore \
  -H "Authorization: Bearer <admin-token>"
```

:::warning
Restoring a backup overwrites the current game data in the running server. Consider creating a backup before restoring to preserve the current state.
:::
