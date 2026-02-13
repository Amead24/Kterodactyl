---
sidebar_position: 1
title: Helm Values Reference
---

# Helm Values Reference

Complete reference for all configurable values in the Kterodactyl Helm chart. Values are organized to match the structure of `chart/values.yaml`.

## Core Settings

| Parameter | Description | Default |
|---|---|---|
| `replicaCount` | Number of operator replicas (should be 1 with leader election) | `1` |
| `image.repository` | Container image repository | `ghcr.io/kterodactyl/kterodactyl` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `image.tag` | Image tag (defaults to chart appVersion) | `""` |
| `imagePullSecrets` | Image pull secrets for private registries | `[]` |
| `nameOverride` | Override the chart name | `""` |
| `fullnameOverride` | Override the full release name | `""` |

## ServiceAccount

| Parameter | Description | Default |
|---|---|---|
| `serviceAccount.create` | Create a ServiceAccount for the operator | `true` |
| `serviceAccount.annotations` | Annotations for the ServiceAccount | `{}` |
| `serviceAccount.name` | ServiceAccount name override (generated from fullname if empty) | `""` |

## Manager

Configuration for the operator and API server binary.

| Parameter | Description | Default |
|---|---|---|
| `manager.extraArgs` | Extra command-line arguments for the manager binary | `[]` |
| `manager.resources.limits.cpu` | CPU limit for the manager container | `500m` |
| `manager.resources.limits.memory` | Memory limit for the manager container | `128Mi` |
| `manager.resources.requests.cpu` | CPU request for the manager container | `10m` |
| `manager.resources.requests.memory` | Memory request for the manager container | `64Mi` |
| `manager.podSecurityContext.runAsNonRoot` | Run the pod as non-root | `true` |
| `manager.podSecurityContext.seccompProfile.type` | Seccomp profile type | `RuntimeDefault` |
| `manager.securityContext.readOnlyRootFilesystem` | Mount root filesystem as read-only | `true` |
| `manager.securityContext.allowPrivilegeEscalation` | Allow privilege escalation | `false` |
| `manager.securityContext.capabilities.drop` | Linux capabilities to drop | `["ALL"]` |

## Scheduling

| Parameter | Description | Default |
|---|---|---|
| `nodeSelector` | Node labels for pod assignment | `{}` |
| `tolerations` | Tolerations for pod scheduling | `[]` |
| `affinity` | Affinity rules for pod scheduling | `{}` |

## API Service

Configuration for the Kubernetes Service that exposes the API server and embedded UI.

| Parameter | Description | Default |
|---|---|---|
| `apiService.type` | Service type (`ClusterIP` for internal, `LoadBalancer` for direct access) | `ClusterIP` |
| `apiService.port` | API server port | `8080` |

## Metrics

| Parameter | Description | Default |
|---|---|---|
| `metrics.enabled` | Enable the metrics endpoint | `true` |
| `metrics.service.port` | Metrics service port | `8443` |

## ServiceMonitor

Prometheus ServiceMonitor for automatic metric scraping.

| Parameter | Description | Default |
|---|---|---|
| `serviceMonitor.enabled` | Create a ServiceMonitor resource | `false` |
| `serviceMonitor.labels` | Additional labels for the ServiceMonitor | `{}` |
| `serviceMonitor.interval` | Scrape interval (empty uses Prometheus default) | `""` |
| `serviceMonitor.scrapeTimeout` | Scrape timeout (empty uses Prometheus default) | `""` |

## NetworkPolicy

| Parameter | Description | Default |
|---|---|---|
| `networkPolicy.enabled` | Create a NetworkPolicy for metrics traffic | `false` |

## AdminConfig

These values populate the `kterodactyl-admin-config` ConfigMap. The operator reads this ConfigMap on every reconciliation, so changes take effect without restarting the operator.

### Limits

| Parameter | Description | Default |
|---|---|---|
| `adminConfig.limits.maxServersGlobal` | Maximum GameServers cluster-wide | `"100"` |
| `adminConfig.limits.maxServersPerUser` | Maximum GameServers per user | `"5"` |

### Quota

Resource quota applied to user namespaces.

| Parameter | Description | Default |
|---|---|---|
| `adminConfig.quota.cpuRequests` | Total CPU requests across all user pods | `"4"` |
| `adminConfig.quota.cpuLimits` | Total CPU limits across all user pods | `"8"` |
| `adminConfig.quota.memoryRequests` | Total memory requests across all user pods | `"8Gi"` |
| `adminConfig.quota.memoryLimits` | Total memory limits across all user pods | `"16Gi"` |
| `adminConfig.quota.pods` | Maximum number of pods | `"5"` |
| `adminConfig.quota.pvcs` | Maximum number of PersistentVolumeClaims | `"5"` |
| `adminConfig.quota.storage` | Total PVC storage | `"50Gi"` |

### Container Defaults

Default resource allocation for game server containers. These are used when a game manifest does not specify resource requirements.

| Parameter | Description | Default |
|---|---|---|
| `adminConfig.containerDefaults.cpu` | Default CPU limit | `"2"` |
| `adminConfig.containerDefaults.memory` | Default memory limit | `"4Gi"` |
| `adminConfig.containerDefaults.requestCPU` | Default CPU request | `"500m"` |
| `adminConfig.containerDefaults.requestMemory` | Default memory request | `"1Gi"` |
| `adminConfig.containerDefaults.maxCPU` | Maximum CPU a user can request | `"4"` |
| `adminConfig.containerDefaults.maxMemory` | Maximum memory a user can request | `"8Gi"` |
| `adminConfig.containerDefaults.minCPU` | Minimum CPU a user can request | `"100m"` |
| `adminConfig.containerDefaults.minMemory` | Minimum memory a user can request | `"128Mi"` |

### Networking

| Parameter | Description | Default |
|---|---|---|
| `adminConfig.networking.baseDomain` | Base domain for game server DNS (empty disables DNS routing) | `""` |
| `adminConfig.networking.gateway.name` | Gateway resource name for HTTPRoute parent references | `"kterodactyl-gateway"` |
| `adminConfig.networking.gateway.namespace` | Gateway namespace (defaults to release namespace) | `""` |
| `adminConfig.networking.gateway.controllerNamespace` | Gateway controller namespace | `"envoy-gateway-system"` |

### Auth

| Parameter | Description | Default |
|---|---|---|
| `adminConfig.auth.jwtExpirationHours` | JWT token expiration time in hours | `"24"` |
| `adminConfig.auth.inviteExpirationHours` | Invite token expiration time in hours | `"72"` |
| `adminConfig.auth.registrationEnabled` | Whether invite-based registration is enabled | `"true"` |
| `adminConfig.auth.panelURL` | Public URL of the panel (used in invite emails) | `""` |

### SMTP

| Parameter | Description | Default |
|---|---|---|
| `adminConfig.smtp.host` | SMTP server hostname (empty disables email sending) | `""` |
| `adminConfig.smtp.port` | SMTP server port | `"587"` |
| `adminConfig.smtp.username` | SMTP authentication username | `""` |
| `adminConfig.smtp.from` | Sender email address | `""` |

:::info SMTP Password
The SMTP password is stored in a separate Kubernetes Secret (`kterodactyl-smtp-credentials`), not in the ConfigMap, to prevent credential exposure.
:::

### Storage

| Parameter | Description | Default |
|---|---|---|
| `adminConfig.storage.modStorageClass` | StorageClass for mod PVCs (empty uses cluster default) | `""` |
| `adminConfig.storage.modStorageSize` | Storage size for mod PVCs | `"1Gi"` |

### Backup

| Parameter | Description | Default |
|---|---|---|
| `adminConfig.backup.enabled` | Enable the backup system | `false` |
| `adminConfig.backup.s3.endpoint` | S3-compatible storage endpoint | `""` |
| `adminConfig.backup.s3.bucket` | S3 bucket name (auto-created on first backup) | `"kterodactyl-backups"` |
| `adminConfig.backup.s3.region` | S3 region | `"us-east-1"` |
| `adminConfig.backup.s3.useSSL` | Use SSL for S3 connections | `"false"` |
| `adminConfig.backup.retentionCount` | Number of backups to retain per server | `"5"` |

:::tip S3 Credentials
S3 access and secret keys are stored in a separate Kubernetes Secret (`kterodactyl-s3-credentials`), not in the ConfigMap. See the [installation guide](/docs/getting-started/installation#3-create-secrets-for-optional-features) for the creation command.
:::
