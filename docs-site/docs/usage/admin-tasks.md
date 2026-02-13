---
sidebar_position: 4
---

# Admin Tasks

This guide covers administrative workflows: user management, invitations, backup scheduling, and monitoring.

## Admin Role

Kterodactyl has two roles: **user** and **admin**. Admins have access to all user functionality plus additional management capabilities:

- Creating and managing user invitations
- Viewing and deleting user accounts
- Setting backup schedules for any server
- Deleting backups and restoring from backups

### Bootstrapping the First Admin

After a fresh installation, there are no users in the system. The first admin must be created via the API:

```bash
# Port-forward the API service
kubectl port-forward -n kterodactyl-system svc/kterodactyl-api 8080:8080

# Register the first user (automatically becomes admin when no users exist)
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "your-secure-password", "email": "admin@example.com"}'
```

:::tip
See the [Installation guide](/docs/getting-started/installation) for the complete post-install bootstrap process, including the commands printed by `helm install`.
:::

## User Invitations

New users can only register with a valid invitation token. This prevents unauthorized account creation.

### Creating an Invite

1. Log in as an admin
2. Navigate to **Admin** > **Users** in the sidebar
3. Click **Create Invite**
4. Enter the email address for the new user
5. Copy the invitation link or token from the response

The invitation link contains a one-time token that expires after the configured duration (default: 72 hours, configurable via `adminConfig.auth.inviteExpirationHours` in the Helm values).

:::info SMTP Optional
If SMTP is configured, the invitation email is sent automatically. If SMTP is not configured, the invite link is returned in the API response -- the admin must share it manually.
:::

### Invite Workflow

1. Admin creates invite for a specific email address
2. Admin shares the invite link with the user (or SMTP delivers it)
3. User visits the registration page with the invite token
4. User creates their username and password
5. The invite token is consumed and cannot be reused

## User Management

### Viewing Users

Navigate to **Admin** > **Users** to see all registered users. The list shows each user's username, email, role, and creation date.

### Deleting Users

Click the delete icon next to a user to remove their account. A confirmation dialog prevents accidental deletions.

:::warning
Deleting a user removes their account but does not automatically delete their game servers. Server resources in the user's namespace remain until manually cleaned up.
:::

**Self-deletion is prevented.** An admin cannot delete their own account to avoid creating an admin-less cluster.

## Backup Scheduling

Admins can set automatic backup schedules for any game server in the cluster.

### Setting a Schedule

Use the backup schedule endpoint to configure recurring backups:

```bash
curl -X PUT http://localhost:8080/api/v1/gameservers/<name>/backup-schedule \
  -H "Authorization: Bearer <admin-token>" \
  -H "Content-Type: application/json" \
  -d '{"schedule": "0 */6 * * *"}'
```

The schedule uses standard cron syntax. See the [Backups and Restore](/docs/usage/backups-restore) guide for common schedule examples.

### Managing Backup Storage

Admins should monitor S3 storage usage and configure bucket lifecycle policies to automatically expire old backups. Kterodactyl does not currently implement automatic retention policies -- this is a planned feature.

## Monitoring

Kterodactyl exposes Prometheus metrics for monitoring operator and API health.

### Available Metrics

The operator and API server expose 5 Prometheus metrics on the manager's metrics endpoint (port 8443):

- **Server counts** by state and game type
- **Reconciliation duration** for operator controllers
- **HTTP request rates**, latency, and in-flight counts for the API

See the [Metrics Reference](/docs/reference/metrics) for the complete list with labels and example PromQL queries.

### Setting Up Monitoring

If you have Prometheus installed in your cluster, create a `ServiceMonitor` to scrape Kterodactyl metrics:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: kterodactyl
  namespace: kterodactyl-system
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: kterodactyl
  endpoints:
    - port: metrics
      path: /metrics
      scheme: https
      tlsConfig:
        insecureSkipVerify: true
```

:::info
The Helm chart includes a `ServiceMonitor` resource that is enabled when `serviceMonitor.enabled` is set to `true` in the values. See the [Helm Values Reference](/docs/configuration/helm-values) for details.
:::

### Key Dashboards

Consider creating Grafana dashboards for:

- **Server Overview**: Total servers by state, creation rate, error rate
- **API Health**: Request rate, p99 latency, error percentage, in-flight requests
- **Operator Health**: Reconciliation duration, reconciliation rate, error rate by controller
