---
sidebar_position: 5
title: Authentication
---

# Authentication

Kterodactyl uses JWT (JSON Web Token) authentication with an invite-based user registration system. All user data is stored as Kubernetes Secrets.

## JWT Tokens

### How It Works

Kterodactyl uses HMAC-SHA256 (HS256) for JWT signing. A single signing key is used to both sign and verify tokens, which is appropriate since the same service handles both operations.

On first startup, the operator automatically generates a signing key and stores it in a Kubernetes Secret named `kterodactyl-jwt-signing-key`. This key persists across restarts.

### Token Lifecycle

1. User logs in via `POST /api/v1/auth/login` with username and password
2. Server returns a JWT token in the response
3. Client includes the token in the `Authorization: Bearer <token>` header for authenticated requests
4. When a token is within 2 hours of expiry, the middleware issues a fresh token in the `X-Refresh-Token` response header
5. Client can also explicitly refresh via `POST /api/v1/auth/refresh`

### Configuration

| Setting | Description | Default |
|---|---|---|
| `adminConfig.auth.jwtExpirationHours` | How long tokens remain valid | `24` hours |

Longer expiration times are more convenient for users but increase the window of exposure if a token is compromised. The default of 24 hours balances convenience with security.

### Key Rotation

To rotate the JWT signing key, delete the Secret and the operator will generate a new one on next startup:

```bash
kubectl delete secret kterodactyl-jwt-signing-key -n kterodactyl-system
```

:::warning
Rotating the key invalidates **all** existing user sessions. Users must log in again.
:::

## User Model

Users are stored as Kubernetes Secrets with the naming convention `user-<username>`. Each Secret contains:

- Username (also stored in labels for efficient queries)
- Email address
- Password hash (Argon2id with OWASP-recommended parameters)
- Role (`admin` or `user`)
- Created timestamp

The username label (`kterodactyl.io/username`) enables efficient list operations via Kubernetes label selectors without needing to read every Secret's data field.

## Invite-Based Registration

Kterodactyl uses an invite flow for user registration:

1. **Admin creates an invite** via `POST /api/v1/admin/invites` with the new user's email address
2. The system generates a unique invite token stored as a Kubernetes Secret (named `invite-<first-12-chars>`)
3. If SMTP is configured, an email is sent with the registration link
4. The invite link is also returned in the API response (for manual sharing)
5. **User registers** via `POST /api/v1/auth/register` with the invite token, username, and password
6. The invite Secret is deleted after successful registration

### Configuration

| Setting | Description | Default |
|---|---|---|
| `adminConfig.auth.inviteExpirationHours` | How long invite tokens remain valid | `72` hours |
| `adminConfig.auth.registrationEnabled` | Whether new users can register | `true` |
| `adminConfig.auth.panelURL` | Public URL used in invite email links | `""` |

Setting `registrationEnabled` to `false` prevents new registrations even with a valid invite token.

## SMTP Setup

Email sending is optional. Without SMTP, invite links are returned directly in the API response and admins can share them manually.

To enable email invitations:

### 1. Configure SMTP Settings

```yaml
adminConfig:
  smtp:
    host: "smtp.example.com"
    port: "587"
    username: "kterodactyl@example.com"
    from: "Kterodactyl <kterodactyl@example.com>"
  auth:
    panelURL: "https://panel.example.com"
```

### 2. Create SMTP Password Secret

```bash
kubectl create secret generic kterodactyl-smtp-credentials \
  -n kterodactyl-system \
  --from-literal=password=YOUR_SMTP_PASSWORD
```

:::info TLS Configuration
Kterodactyl uses opportunistic TLS for SMTP connections. It attempts TLS if the server supports it but falls back to plain text if not. SMTP authentication is auto-discovered based on server capabilities.
:::

### 3. Verify

Create a test invite via the admin API. If SMTP is configured correctly, the invited user will receive an email. If SMTP fails, the invite is still created and the link is returned in the response -- the failure is logged but does not block invite creation.

## Bootstrap

When the system is first installed with no users, use the bootstrap endpoint to create the initial admin:

```bash
curl -X POST http://localhost:8080/api/v1/auth/bootstrap \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"your-secure-password","email":"admin@example.com"}'
```

The bootstrap endpoint only works when no users exist. After the first admin is created, all subsequent users must be added through the invite flow.

## Roles

Kterodactyl has two roles:

| Role | Capabilities |
|---|---|
| `admin` | Full access: manage users, create invites, delete any server, manage backup schedules, restore backups |
| `user` | Self-service: create/manage own servers, create/list own backups, upload mods |

The first user created via bootstrap is always an admin. Subsequent users registered via invite are assigned the `user` role by default.

:::warning
Admin self-deletion is prevented to avoid leaving the system without an administrator.
:::
