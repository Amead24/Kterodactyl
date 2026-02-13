---
sidebar_position: 2
title: AdminConfig
---

# AdminConfig

The AdminConfig is a Kubernetes ConfigMap named `kterodactyl-admin-config` that controls the operational behavior of the Kterodactyl operator. It is the primary runtime configuration mechanism.

## How It Works

The operator loads the AdminConfig ConfigMap **on every reconciliation loop**. This means you can update the ConfigMap at any time and changes take effect immediately -- no operator restart is required.

The Helm chart generates this ConfigMap from the `adminConfig` section of your `values.yaml`. You can also edit the ConfigMap directly with `kubectl`:

```bash
kubectl edit configmap kterodactyl-admin-config -n kterodactyl-system
```

:::info Sensible Defaults
The operator works without the AdminConfig ConfigMap by using built-in defaults. This means a minimal Helm install with no custom values produces a functional system.
:::

## Configuration Sections

### Limits

Controls the maximum number of game servers that can be created.

| Key | Description | Default |
|---|---|---|
| `maxServersGlobal` | Maximum GameServer resources cluster-wide | `100` |
| `maxServersPerUser` | Maximum GameServer resources per user | `5` |

These limits are enforced at the API level when users create new game servers.

### Quota

Resource quota settings applied to user game server pods. These prevent any single user from consuming excessive cluster resources.

| Key | Description | Default |
|---|---|---|
| `cpuRequests` | Total CPU requests across all user pods | `4` |
| `cpuLimits` | Total CPU limits across all user pods | `8` |
| `memoryRequests` | Total memory requests across all user pods | `8Gi` |
| `memoryLimits` | Total memory limits across all user pods | `16Gi` |
| `pods` | Maximum number of pods | `5` |
| `pvcs` | Maximum number of PersistentVolumeClaims | `5` |
| `storage` | Total PVC storage | `50Gi` |

### Container Defaults

Default resource allocation for game server containers. Game manifests can specify their own resource requirements, but these defaults are used as boundaries.

| Key | Description | Default |
|---|---|---|
| `cpu` | Default CPU limit per container | `2` |
| `memory` | Default memory limit per container | `4Gi` |
| `requestCPU` | Default CPU request per container | `500m` |
| `requestMemory` | Default memory request per container | `1Gi` |
| `maxCPU` | Maximum CPU a user can request | `4` |
| `maxMemory` | Maximum memory a user can request | `8Gi` |
| `minCPU` | Minimum CPU a user can request | `100m` |
| `minMemory` | Minimum memory a user can request | `128Mi` |

The `min*` and `max*` values define the range within which users can customize their server resource allocation. The `cpu`, `memory`, `requestCPU`, and `requestMemory` values are the defaults applied when a user does not specify custom resources.

### Networking

Controls DNS routing for game servers.

| Key | Description | Default |
|---|---|---|
| `baseDomain` | Base domain for game server DNS entries | `""` (disabled) |
| `gateway.name` | Name of the Gateway resource | `kterodactyl-gateway` |
| `gateway.namespace` | Namespace of the Gateway resource | Release namespace |
| `gateway.controllerNamespace` | Namespace of the Gateway controller | `envoy-gateway-system` |

When `baseDomain` is set, each game server gets a DNS entry following the pattern `game.username.baseDomain`. For example, a Minecraft server owned by user `alice` with base domain `game.example.com` would be accessible at `minecraft.alice.game.example.com`.

See [Networking](/docs/configuration/networking) for detailed setup instructions.

### Auth

Authentication and user management settings.

| Key | Description | Default |
|---|---|---|
| `jwtExpirationHours` | How long JWT tokens remain valid | `24` |
| `inviteExpirationHours` | How long invite tokens remain valid | `72` |
| `registrationEnabled` | Whether new users can register via invite | `true` |
| `panelURL` | Public URL of the panel (used in invite emails) | `""` |

See [Authentication](/docs/configuration/auth) for detailed auth configuration.

### SMTP

Email sending configuration for invite notifications. All SMTP fields are optional -- without SMTP, invite links are returned directly in the API response.

| Key | Description | Default |
|---|---|---|
| `host` | SMTP server hostname | `""` (disabled) |
| `port` | SMTP server port | `587` |
| `username` | SMTP authentication username | `""` |
| `from` | Sender email address for invite emails | `""` |

:::info SMTP Password
The SMTP password is stored separately in a Kubernetes Secret named `kterodactyl-smtp-credentials` to avoid exposing credentials in the ConfigMap.
:::

### Storage

Mod storage configuration.

| Key | Description | Default |
|---|---|---|
| `modStorageClass` | StorageClass for mod PVCs (empty uses cluster default) | `""` |
| `modStorageSize` | Storage size for each mod PVC | `1Gi` |

Each game server that supports mods gets a dedicated PersistentVolumeClaim for storing uploaded mod files. The PVC is created when a mod is first uploaded.

### Backup

S3-compatible backup configuration.

| Key | Description | Default |
|---|---|---|
| `enabled` | Enable the backup system | `false` |
| `s3.endpoint` | S3-compatible storage endpoint | `""` |
| `s3.bucket` | S3 bucket name | `kterodactyl-backups` |
| `s3.region` | S3 region | `us-east-1` |
| `s3.useSSL` | Use SSL for S3 connections | `false` |
| `retentionCount` | Number of backups to retain per server | `5` |

The backup bucket is auto-created on first backup if it does not exist. S3 credentials are stored in the `kterodactyl-s3-credentials` Secret.

See [Backups](/docs/configuration/backups) for detailed setup instructions.
