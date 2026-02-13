---
sidebar_position: 1
---

# Contributing Game Definitions

This guide explains how to add support for a new game to Kterodactyl. Each game is defined by a directory containing a manifest file and a Dockerfile. When the operator starts, it loads all game definitions and makes them available through the API and web UI.

## Overview

The game definition system is the primary way Kterodactyl supports new game types. A game definition tells the operator:

- What container image to run
- Which ports to expose
- What resources to allocate
- What parameters users can configure
- How to validate those parameters (via JSON Schema)
- Where mods and backup data are stored

The frontend automatically generates configuration forms from the game's JSON Schema, so users get a tailored experience for each game without any frontend code changes.

## Directory Structure

Each game lives in its own directory under `games/`:

```
games/
  minecraft/
    manifest.yaml     # Game definition (required)
    Dockerfile        # Container image build (required)
  valheim/
    manifest.yaml
    Dockerfile
```

The directory name must match the `name` field in the manifest. Use lowercase with no spaces or special characters.

## Manifest Format

The `manifest.yaml` file defines all properties of a game type.

### Required Fields

```yaml
name: minecraft                         # Unique identifier (lowercase, no spaces)
displayName: "Minecraft Java Edition"   # Human-readable name for the UI
image: itzg/minecraft-server:latest     # Container image to run
ports:                                  # Network ports exposed by the server
  - name: game
    containerPort: 25565
    protocol: TCP
```

### Optional Fields

```yaml
modPath: /mods                          # Container path for mod file uploads
backupPath: /data                       # Container path to back up
resources:                              # CPU/memory requests and limits
  requests:
    cpu: "500m"
    memory: "1Gi"
  limits:
    cpu: "2"
    memory: "4Gi"
parameters:                             # Default parameter key-value pairs
  EULA: "TRUE"
  TYPE: "VANILLA"
parameterSchema:                        # JSON Schema for validation and forms
  type: object
  properties: ...
```

### Complete Field Reference

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `name` | Yes | string | Unique identifier for the game type. Must be lowercase, no spaces. Must match the directory name. |
| `displayName` | Yes | string | Human-readable name displayed in the UI game browser. |
| `image` | Yes | string | Container image to run for this game type (e.g., `itzg/minecraft-server:latest`). |
| `ports` | Yes | list | Network ports exposed by the game server. Each entry has `name`, `containerPort`, and `protocol` (TCP or UDP). |
| `modPath` | No | string | Container path where mod files are uploaded (e.g., `/mods`). Enables the mod upload feature for this game. |
| `backupPath` | No | string | Container path to include in backups (e.g., `/data`). Defaults to `/data` if not specified. |
| `parameters` | No | map | Default key-value pairs passed as environment variables to the game container. |
| `parameterSchema` | No | object | JSON Schema (Draft 2020-12) defining parameter constraints and UI metadata. |
| `resources` | No | object | Kubernetes resource requests and limits for the game container (`cpu`, `memory`). |

## Parameter Schema

The `parameterSchema` field is a JSON Schema object embedded in YAML. It serves two purposes:

1. **Validation**: Parameters submitted through the API are validated against the schema before creating or updating a server
2. **Form generation**: The frontend consumes the schema to auto-generate configuration forms using [react-jsonschema-form](https://github.com/rjsf-team/react-jsonschema-form)

### Important Rule: All Types Must Be String

Because all parameters become container environment variables, every property in the schema must use `type: string`. Do not use `type: integer`, `type: boolean`, or other types. Use constraints like `pattern` and `enum` to enforce value formats.

### Supported JSON Schema Keywords

| Keyword | Purpose | Example |
|---------|---------|---------|
| `enum` | Dropdown selection from a fixed list of values | `enum: ["VANILLA", "PAPER", "SPIGOT"]` |
| `pattern` | Regex validation for formatted input | `pattern: "^[1-9][0-9]*$"` |
| `const` | Fixed value that cannot be changed by the user | `const: "TRUE"` |
| `maxLength` | Maximum character length for text input | `maxLength: 59` |
| `title` | Label displayed in the form UI | `title: "Server Type"` |
| `description` | Help text displayed below the form field | `description: "Choose the server implementation"` |
| `default` | Default value pre-filled in the form | `default: "VANILLA"` |
| `required` | Array of mandatory parameter names (at schema root level) | `required: ["EULA", "TYPE"]` |

### Schema Compilation

Schemas are compiled once when the operator starts using the [santhosh-tekuri/jsonschema/v6](https://github.com/santhosh-tekuri/jsonschema) library. Invalid schemas will prevent the operator from starting. This fail-fast behavior ensures all game definitions are valid before serving traffic.

## Minecraft Walkthrough

The Minecraft game definition is the reference implementation. Here is the complete `games/minecraft/manifest.yaml`:

```yaml
name: minecraft
displayName: "Minecraft Java Edition"
image: itzg/minecraft-server:latest
modPath: /mods
backupPath: /data
ports:
  - name: game
    containerPort: 25565
    protocol: TCP
resources:
  requests:
    cpu: "500m"
    memory: "1Gi"
  limits:
    cpu: "2"
    memory: "4Gi"
parameters:
  EULA: "TRUE"
  TYPE: "VANILLA"
  DIFFICULTY: "normal"
  MODE: "survival"
  MAX_PLAYERS: "20"
  MEMORY: "2G"
  MOTD: "A Kterodactyl Minecraft Server"
  PVP: "true"
  SEED: ""
  ONLINE_MODE: "true"
parameterSchema:
  type: object
  properties:
    EULA:
      type: string
      title: "Accept EULA"
      description: "Must be TRUE to accept the Minecraft EULA"
      const: "TRUE"
      default: "TRUE"
    TYPE:
      type: string
      title: "Server Type"
      description: "Minecraft server implementation to use"
      enum: ["VANILLA", "PAPER", "SPIGOT", "FORGE", "FABRIC", "QUILT"]
      default: "VANILLA"
    DIFFICULTY:
      type: string
      title: "Difficulty"
      description: "Game difficulty level"
      enum: ["peaceful", "easy", "normal", "hard"]
      default: "normal"
    MODE:
      type: string
      title: "Game Mode"
      description: "Default game mode for new players"
      enum: ["survival", "creative", "adventure", "spectator"]
      default: "survival"
    MAX_PLAYERS:
      type: string
      title: "Max Players"
      description: "Maximum number of concurrent players (must be a positive integer)"
      pattern: "^[1-9][0-9]*$"
      default: "20"
    MEMORY:
      type: string
      title: "JVM Memory"
      description: "Java heap memory allocation for the server"
      enum: ["1G", "2G", "4G", "8G"]
      default: "2G"
    MOTD:
      type: string
      title: "Server Message"
      description: "Message displayed in the server list (max 59 characters)"
      maxLength: 59
      default: "A Kterodactyl Minecraft Server"
    PVP:
      type: string
      title: "PvP Enabled"
      description: "Allow player-vs-player combat"
      enum: ["true", "false"]
      default: "true"
    SEED:
      type: string
      title: "World Seed"
      description: "World generation seed (leave empty for random)"
      default: ""
    ONLINE_MODE:
      type: string
      title: "Online Mode"
      description: "Require Mojang authentication for connecting players"
      enum: ["true", "false"]
      default: "true"
  required:
    - EULA
    - TYPE
```

### Key Patterns in This Example

- **`const: "TRUE"` on EULA**: The user cannot change this value. The form renders it as a read-only field. This ensures legal compliance.
- **`enum` for dropdowns**: Server Type, Difficulty, Mode, Memory, PvP, and Online Mode all use `enum` to present a fixed set of choices.
- **`pattern` for numeric input**: MAX_PLAYERS uses `pattern: "^[1-9][0-9]*$"` to validate positive integers since the type is `string`.
- **`maxLength` for text limits**: MOTD limits input to 59 characters (Minecraft protocol limit).
- **`required` array**: Only EULA and TYPE are mandatory. All other parameters have defaults and are optional.
- **`default` values**: Every parameter has a default so servers can be created with minimal configuration.

## Dockerfile Conventions

Each game directory must include a `Dockerfile`.

### Using Community Images

For most games, the Dockerfile is a thin wrapper over an existing community-maintained image:

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

- Include a comment at the top with the image name and a link to its documentation
- Use existing community images when available rather than building from scratch
- Ensure the image is publicly accessible (no private registry authentication required)
- The `image` field in the manifest should reference the published image, not a local build

## Testing Your Game Definition

After creating your game directory, verify the manifest loads correctly:

```bash
# Test manifest loading and schema compilation
go test ./internal/manifest/... -v

# Run the full test suite (includes API handler tests using game manifests)
go test ./... -count=1
```

The manifest loader validates all required fields and compiles the JSON Schema at load time. Errors in your manifest surface immediately as test failures.

## PR Checklist

Before submitting your pull request:

- [ ] Directory created under `games/<gamename>/`
- [ ] `manifest.yaml` has all required fields (`name`, `displayName`, `image`, `ports`)
- [ ] `name` field matches the directory name (lowercase, no spaces)
- [ ] `Dockerfile` exists with documentation comments
- [ ] Container image is publicly accessible
- [ ] `parameterSchema` validates correctly (if defined)
- [ ] All parameter schema properties use `type: string`
- [ ] `modPath` set if the game supports mods
- [ ] `backupPath` set to the game's data directory
- [ ] Default `parameters` provide sensible values for quick server creation
- [ ] `go test ./internal/manifest/...` passes
- [ ] `go test ./...` passes (full test suite)

## Reference

See `games/minecraft/` for the complete reference implementation with 10 configurable parameters and a full JSON Schema definition.
