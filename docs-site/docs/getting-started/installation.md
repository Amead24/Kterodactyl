---
sidebar_position: 3
title: Installation
---

# Installation

This guide walks through installing Kterodactyl using Helm and performing initial setup.

## Install with Helm

### Minimal Installation

Install with default settings:

```bash
helm install kterodactyl ./chart -n kterodactyl-system --create-namespace
```

### Installation with Custom Values

For a more complete setup, create a `values.yaml` with your configuration:

```yaml
adminConfig:
  networking:
    baseDomain: "game.example.com"
    gateway:
      name: "my-gateway"
      namespace: "gateway-system"
  auth:
    panelURL: "https://panel.example.com"
  backup:
    enabled: true
    s3:
      endpoint: "minio.minio-system.svc.cluster.local:9000"
      bucket: "kterodactyl-backups"
      region: "us-east-1"
      useSSL: "false"
    retentionCount: "10"
```

Then install:

```bash
helm install kterodactyl ./chart -n kterodactyl-system --create-namespace -f values.yaml
```

## Post-Install Steps

### 1. Access the API Server

**Using port-forward (default ClusterIP service):**

```bash
kubectl port-forward svc/kterodactyl-api -n kterodactyl-system 8080:8080
```

Then visit `http://localhost:8080` to access the web UI.

**Using LoadBalancer:**

If you set `apiService.type: LoadBalancer` in your values, wait for the external IP:

```bash
kubectl get svc kterodactyl-api -n kterodactyl-system -w
```

Once assigned, the API and UI are available at `http://<EXTERNAL-IP>:8080`.

### 2. JWT Signing Key

A JWT signing key is **auto-generated on first start** and stored in a Kubernetes Secret named `kterodactyl-jwt-signing-key` in the release namespace. No manual action is required.

The key persists across restarts. If you need to rotate the key, delete the Secret and the operator will generate a new one:

```bash
kubectl delete secret kterodactyl-jwt-signing-key -n kterodactyl-system
```

:::warning
Rotating the JWT key invalidates all existing user sessions.
:::

### 3. Create Secrets for Optional Features

Kterodactyl stores sensitive credentials in Kubernetes Secrets that are **not managed by the Helm chart**. You must create them manually if using these features.

**S3 Backup Credentials** (if `adminConfig.backup.enabled: true`):

```bash
kubectl create secret generic kterodactyl-s3-credentials \
  -n kterodactyl-system \
  --from-literal=access-key=YOUR_ACCESS_KEY \
  --from-literal=secret-key=YOUR_SECRET_KEY
```

**SMTP Credentials** (if `adminConfig.smtp.host` is set):

```bash
kubectl create secret generic kterodactyl-smtp-credentials \
  -n kterodactyl-system \
  --from-literal=password=YOUR_SMTP_PASSWORD
```

### 4. Bootstrap Your First Admin User

Use the bootstrap endpoint to create the initial admin account:

```bash
curl -X POST http://localhost:8080/api/v1/auth/bootstrap \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"changeme","email":"admin@example.com"}'
```

:::tip
The bootstrap endpoint only works when no users exist in the system. After creating the first admin, use the invite flow to add more users.
:::

After bootstrapping, log in through the web UI at `http://localhost:8080` or via the API:

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"changeme"}'
```

### 5. CRD Upgrades

:::warning CRD Upgrades
Helm does not upgrade CRDs automatically. After chart upgrades, apply CRDs manually if they have changed:

```bash
kubectl apply -f chart/crds/
```
:::

## Verify Installation

Check that the operator is running:

```bash
kubectl get pods -n kterodactyl-system
```

You should see the operator pod in `Running` state. Check the logs for any errors:

```bash
kubectl logs -n kterodactyl-system -l app.kubernetes.io/name=kterodactyl
```

Verify the CRDs are installed:

```bash
kubectl get crd gameservers.game.kterodactyl.io
kubectl get crd backups.game.kterodactyl.io
```

## Next Steps

- [Configure Helm values](/docs/configuration/helm-values) for your environment
- [Set up AdminConfig](/docs/configuration/admin-config) for resource limits and quotas
- [Configure networking](/docs/configuration/networking) for DNS routing
- [Enable backups](/docs/configuration/backups) with S3-compatible storage
