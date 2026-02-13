---
sidebar_position: 1
---

# API Endpoints

Complete reference for the Kterodactyl REST API. All endpoints are served by the operator's built-in API server on port 8080.

## Authentication

Most endpoints require a valid JWT token in the `Authorization` header:

```
Authorization: Bearer <token>
```

Tokens are obtained via the login endpoint and refreshed automatically by the middleware when approaching expiry.

## Health

Health check endpoints are unauthenticated and used by Kubernetes probes.

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/healthz` | None | Liveness probe. Returns 200 if the process is running. |
| GET | `/readyz` | None | Readiness probe. Returns 200 if the server is ready to accept traffic. |

## Auth

Authentication and registration endpoints. Login and register are public with tight rate limits.

| Method | Path | Auth | Rate Limit | Description |
|--------|------|------|------------|-------------|
| POST | `/api/v1/auth/login` | None | 5/min per IP | Authenticate with username and password. Returns a JWT token. |
| POST | `/api/v1/auth/register` | None | 3/min per IP | Register a new user account. Requires a valid invitation token. |
| POST | `/api/v1/auth/refresh` | JWT | -- | Refresh an existing JWT token. Returns a new token. |

### Login Request

```json
{
  "username": "alice",
  "password": "secure-password"
}
```

### Register Request

```json
{
  "username": "alice",
  "password": "secure-password",
  "email": "alice@example.com",
  "inviteToken": "abc123..."
}
```

## Games

Game manifest endpoints for browsing available game types.

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/v1/games` | JWT | List all available game types with their display names, images, ports, resources, and parameter schemas. |
| GET | `/api/v1/games/{gameType}` | JWT | Get a specific game type by name (e.g., `minecraft`). |

## GameServers

CRUD and lifecycle management for game servers. All operations are scoped to the authenticated user's namespace.

### CRUD Operations

| Method | Path | Auth | Rate Limit | Description |
|--------|------|------|------------|-------------|
| GET | `/api/v1/gameservers` | JWT | -- | List all game servers owned by the authenticated user. |
| POST | `/api/v1/gameservers` | JWT | 10/min per IP | Create a new game server. Parameters are validated against the game's JSON Schema. |
| GET | `/api/v1/gameservers/{name}` | JWT | -- | Get a specific game server by name. |
| PUT | `/api/v1/gameservers/{name}` | JWT | -- | Update a game server's parameters. Only `spec.parameters` can be changed after creation. |
| DELETE | `/api/v1/gameservers/{name}` | JWT | -- | Delete a game server and all associated resources (Pod, Service, HTTPRoute, PVC). |

### Create Request

```json
{
  "gameType": "minecraft",
  "parameters": {
    "EULA": "TRUE",
    "TYPE": "VANILLA",
    "MAX_PLAYERS": "20"
  }
}
```

### Lifecycle Actions

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/v1/gameservers/{name}/start` | JWT | Start a stopped server (transitions from Shutdown to Creating). |
| POST | `/api/v1/gameservers/{name}/stop` | JWT | Stop a running server (transitions to Shutdown). |
| POST | `/api/v1/gameservers/{name}/restart` | JWT | Restart a running server (transitions to Creating, rebuilds Pod). |

### Metrics

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/v1/gameservers/{name}/metrics` | JWT | Get CPU and memory usage for the server's Pod (requires Kubernetes Metrics Server). Returns 503 if metrics are unavailable. |

## Mods

Mod file management for game servers. Files are stored on a PersistentVolumeClaim mounted at the game's `modPath`.

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/v1/gameservers/{name}/mods` | JWT | Upload a mod file (multipart form data, 100MB limit). Triggers server restart. |
| GET | `/api/v1/gameservers/{name}/mods` | JWT | List all mod files with filenames and sizes. |
| DELETE | `/api/v1/gameservers/{name}/mods/{filename}` | JWT | Delete a specific mod file by filename. |

:::info Upload Limit
Mod uploads are limited to 100MB per file. The 30-second request timeout applies to uploads. For large mod packs, upload files individually.
:::

## Backups

Backup creation and management. Regular users can create and list backups. Delete, restore, and schedule operations require admin role.

### User Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/v1/gameservers/{name}/backups` | JWT | Create an on-demand backup of the game server. Server must be running. |
| GET | `/api/v1/gameservers/{name}/backups` | JWT | List all backups for the game server. |

### Admin Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| DELETE | `/api/v1/gameservers/{name}/backups/{backupName}` | Admin | Delete a backup resource. Does not delete the S3 object. |
| POST | `/api/v1/gameservers/{name}/backups/{backupName}/restore` | Admin | Restore server data from a completed backup. Server must be running. |
| PUT | `/api/v1/gameservers/{name}/backup-schedule` | Admin | Set or update the automatic backup schedule (cron expression). |

### Backup Schedule Request

```json
{
  "schedule": "0 */6 * * *"
}
```

## Admin

Administrative endpoints for user and invitation management. All require admin role.

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/v1/admin/invites` | Admin | Create a new user invitation. Returns the invite token/link. If SMTP is configured, sends an email. |
| GET | `/api/v1/admin/users` | Admin | List all registered users with usernames, emails, roles, and creation dates. |
| DELETE | `/api/v1/admin/users/{username}` | Admin | Delete a user account. Cannot delete yourself. Does not cascade to user's game servers. |

### Invite Request

```json
{
  "email": "newuser@example.com"
}
```

## WebSocket

Real-time console access via WebSocket.

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/v1/gameservers/{name}/console` | JWT (query param) | WebSocket connection for real-time log streaming and command input. |

The console endpoint uses JWT authentication via query parameter because the browser WebSocket API cannot set HTTP headers during the upgrade handshake:

```
ws://host/api/v1/gameservers/my-server/console?token=<jwt>
```

:::info No Timeout
The WebSocket console route is outside the 30-second timeout middleware group. Connections are long-lived and remain open until the client disconnects or the server is stopped.
:::

## Global Rate Limits

In addition to per-endpoint rate limits, a global rate limit of **100 requests per minute per IP** is applied to all routes.

## Error Responses

All error responses use a consistent JSON format:

```json
{
  "error": "description of what went wrong"
}
```

Common HTTP status codes:

| Code | Meaning |
|------|---------|
| 400 | Bad request (validation error, malformed input) |
| 401 | Unauthorized (missing or invalid JWT token) |
| 403 | Forbidden (insufficient role, e.g., non-admin accessing admin endpoints) |
| 404 | Not found (resource does not exist or not in user's namespace) |
| 409 | Conflict (e.g., username already taken) |
| 429 | Too many requests (rate limit exceeded) |
| 500 | Internal server error |
| 501 | Not implemented |
| 503 | Service unavailable (e.g., metrics server unreachable) |
