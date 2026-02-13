---
sidebar_position: 3
---

# Backups and Restore

Kterodactyl supports on-demand and scheduled backups of game server data to S3-compatible storage. This guide covers creating backups, understanding backup states, and restoring from backups.

## Prerequisites

Before using backups, your cluster administrator must configure S3-compatible storage. See the [Backup Configuration](/docs/configuration/backups) guide for setup instructions.

## On-Demand Backups

### Creating a Backup

1. Navigate to the server detail page for your game server
2. Click the **Backups** tab
3. Click **Create Backup** (this button only appears when the server is in an active state -- the Pod must be running)

The operator creates a `Backup` custom resource and begins the backup process:

1. Connects to the running game server Pod
2. Creates a tar archive of the backup paths (defined by the game manifest, typically `/data`)
3. Compresses the archive with gzip
4. Uploads the compressed archive to the configured S3 bucket

:::info Server Must Be Running
Backups require an active Pod because the operator reads data directly from the container filesystem. You cannot create a backup of a stopped server.
:::

### Backup States

Each backup progresses through lifecycle states:

| State | Description |
|-------|-------------|
| **Pending** | The Backup resource has been created and is queued for processing. |
| **InProgress** | The operator is actively archiving and uploading data to S3. |
| **Completed** | The backup finished successfully. Size and S3 location are recorded in the status. |
| **Failed** | The backup encountered an error. Check the status message for details. |

### Viewing Backups

The Backups tab lists all backups for the server with their state, size, and creation time. The tab is always visible regardless of server state, so you can browse backup history even when the server is stopped.

## Scheduled Backups

Administrators can configure automatic backup schedules for game servers.

### Setting a Schedule

Scheduled backups are configured per-server by an administrator using a cron expression. The operator watches for schedule annotations on GameServer resources and creates Backup resources at the specified intervals.

Common cron schedules:

| Schedule | Cron Expression |
|----------|----------------|
| Every hour | `0 * * * *` |
| Every 6 hours | `0 */6 * * *` |
| Daily at midnight | `0 0 * * *` |
| Weekly on Sunday | `0 0 * * 0` |

:::tip
Scheduled backups only run when the server Pod is active. If the server is stopped at the scheduled time, the backup is skipped.
:::

## Restore (Admin Only)

Restoring from a backup replaces the current server data with the contents of a previous backup. This action is restricted to administrators.

### Restore Process

1. Navigate to the Backups tab on the server detail page
2. Find the backup you want to restore from
3. Click **Restore** and confirm in the dialog

The restore process:

1. Downloads the backup archive from S3
2. Decompresses and extracts the archive
3. Writes the contents back to the game server Pod's filesystem

:::warning Destructive Action
Restoring a backup **replaces** the current server data with the backup contents. Any changes made since the backup was created will be lost. Consider creating a fresh backup before restoring an older one.
:::

### Restore Requirements

- The server must be running (the Pod must exist to write data into)
- Only administrators can perform restores
- The backup must be in the **Completed** state

## Backup Storage

Backups are stored in the configured S3-compatible storage (AWS S3, MinIO, or any S3-compatible provider). Each backup is stored as a gzip-compressed tar archive with a unique key derived from the server name and backup timestamp.

The backup status records:

- **S3 Bucket**: The bucket where the backup is stored
- **S3 Key**: The object key (path) within the bucket
- **Size**: The compressed backup size in bytes
- **Started At**: When the backup process began
- **Completed At**: When the backup finished uploading

## Managing Backups

### Deleting Backups (Admin Only)

Administrators can delete individual backups to free storage space. Deleting a backup removes it from the Kubernetes cluster but does **not** delete the S3 object. Manual S3 cleanup or lifecycle policies should be configured for storage management.
