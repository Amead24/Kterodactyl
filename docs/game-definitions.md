# Contributing Game Definitions

## Overview

Each game in Kterodactyl is defined by a directory under `games/` containing a `manifest.yaml` file and a `Dockerfile`. The manifest describes the game type, its container image, exposed ports, default parameters, resource requirements, and an optional JSON Schema for parameter validation and frontend form generation.

When the operator starts, it loads all valid game definitions from the `games/` directory. Each game becomes available through the API at `GET /api/v1/games` and can be used to create game servers via `POST /api/v1/servers`.

## Directory Structure

```
games/
  minecraft/
    manifest.yaml     # Game definition (required)
    Dockerfile        # Container image reference (required)
  valheim/
    manifest.yaml
    Dockerfile
```

Each game directory name should match the `name` field in the manifest. Use lowercase with no spaces.

## Manifest Format

The `manifest.yaml` file defines all properties of a game type. Below is the complete field reference.

```yaml
# Required fields
name: minecraft                         # Unique identifier (lowercase, no spaces)
displayName: Minecraft Java Edition     # Human-readable name for the UI
image: itzg/minecraft-server:latest     # Container image to run

# Required: network ports exposed by the game server
ports:
  - name: game
    containerPort: 25565
    protocol: TCP

# Optional: default parameter key-value pairs (environment variables)
parameters:
  EULA: "TRUE"
  TYPE: VANILLA

# Optional: JSON Schema for parameter validation and UI form generation
parameterSchema:
  type: object
  properties:
    TYPE:
      type: string
      enum: ["VANILLA", "PAPER", "SPIGOT"]

# Optional: CPU/memory requests and limits
resources:
  requests:
    memory: "1Gi"
    cpu: "500m"
  limits:
    memory: "2Gi"
    cpu: "1000m"
```

### Field Reference

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Unique identifier for the game type. Must be lowercase with no spaces. Must match the directory name. |
| `displayName` | Yes | Human-readable name displayed in the UI. |
| `image` | Yes | Container image to run for this game type. |
| `ports` | Yes | List of network ports with `name`, `containerPort`, and `protocol`. |
| `parameters` | No | Default key-value pairs passed as environment variables to the game container. |
| `parameterSchema` | No | JSON Schema (Draft 2020-12) defining parameter constraints and UI metadata. |
| `resources` | No | Kubernetes resource requests and limits (`cpu`, `memory`). |

## Parameter Schema

The `parameterSchema` field is a JSON Schema object embedded in YAML. It defines constraints on the `parameters` map and drives automatic form generation in the frontend. Because all parameters are environment variables, every property must use `type: string`.

### Supported Constraints

| Keyword | Purpose | Example |
|---------|---------|---------|
| `enum` | Dropdown selection from a fixed list | `enum: ["VANILLA", "PAPER"]` |
| `pattern` | Regex validation for formatted strings | `pattern: "^[1-9][0-9]*$"` |
| `const` | Fixed value that cannot be changed | `const: "TRUE"` |
| `maxLength` | Maximum character length for text fields | `maxLength: 50` |
| `title` | UI label for the parameter | `title: "Server Type"` |
| `description` | Help text shown in the UI | `description: "Choose server implementation"` |
| `default` | Documents the default value | `default: "VANILLA"` |
| `required` | Array of mandatory parameter names | `required: ["EULA", "TYPE"]` |

### Example Schema

```yaml
parameterSchema:
  type: object
  properties:
    EULA:
      type: string
      title: "EULA Agreement"
      description: "Must be TRUE to accept the Minecraft EULA"
      const: "TRUE"
    TYPE:
      type: string
      title: "Server Type"
      description: "Minecraft server implementation"
      enum: ["VANILLA", "PAPER", "SPIGOT", "FORGE"]
      default: "VANILLA"
    MAX_PLAYERS:
      type: string
      title: "Max Players"
      description: "Maximum number of concurrent players"
      pattern: "^[1-9][0-9]*$"
      default: "20"
  required:
    - EULA
    - TYPE
```

### Important Notes

- All parameter types must be `string` (not `integer`, `boolean`, etc.) because environment variables are always strings.
- Use `pattern` with regex for numeric constraints (e.g., `"^[1-9][0-9]*$"` for positive integers).
- The schema is validated at manifest load time. Invalid schemas will prevent the operator from starting.
- Schemas are compiled once at startup for efficient per-request validation.

## Dockerfile

Each game directory must include a `Dockerfile`. For most games, this will be a thin wrapper over an existing community image.

### Using Existing Community Images

```dockerfile
# Minecraft Java Edition
# Image: https://github.com/itzg/docker-minecraft-server
FROM itzg/minecraft-server:latest
```

### Custom Images (SteamCMD Pattern)

For games that require SteamCMD to install, use a multi-stage build:

```dockerfile
# Valheim dedicated server via SteamCMD
# Docs: https://developer.valvesoftware.com/wiki/SteamCMD
FROM steamcmd/steamcmd:latest AS installer
RUN steamcmd +login anonymous +app_update 896660 validate +quit

FROM debian:bookworm-slim
COPY --from=installer /root/Steam/steamapps/common/ValheimDedicatedServer /valheim
WORKDIR /valheim
ENTRYPOINT ["./valheim_server.x86_64"]
```

### Conventions

- Include a comment at the top with the image name and link to documentation.
- Use existing community images when available rather than building from scratch.
- Ensure the image is publicly accessible (no private registry auth required).

## Testing Your Game Definition

After creating your game directory, verify the manifest loads correctly:

```bash
go test ./internal/manifest/... -v
```

The manifest loader validates all required fields and compiles the JSON Schema at load time. Any errors in your manifest will surface immediately as test failures.

To run the full test suite (including API handler tests that use game manifests):

```bash
go test ./... -count=1
```

## PR Checklist

Before submitting your pull request, verify all items:

- [ ] Directory created under `games/<gamename>/`
- [ ] `manifest.yaml` has all required fields (`name`, `displayName`, `image`, `ports`)
- [ ] `name` field matches the directory name (lowercase, no spaces)
- [ ] `Dockerfile` exists with documentation comments
- [ ] Container image is publicly accessible
- [ ] `parameterSchema` validates correctly (if defined)
- [ ] All parameter schema properties use `type: string` (not integer, boolean, etc.)
- [ ] `go test ./internal/manifest/...` passes
- [ ] `go test ./...` passes (full suite)

## Reference

See `games/minecraft/` for a complete reference implementation with 10 configurable parameters and a full JSON Schema definition.
