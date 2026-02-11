# Phase 5: Game Definition Framework - Research

**Researched:** 2026-02-10
**Domain:** Declarative game manifest schema with JSON Schema parameter definitions for dynamic UI form generation
**Confidence:** HIGH

## Summary

Phase 5 transforms the existing simple game manifest format (flat `map[string]string` parameters) into a rich, schema-driven definition framework. The current `games/minecraft.yaml` manifest and `internal/manifest/` loader from Phase 4 provide a solid foundation, but the requirements demand JSON Schema-based parameter definitions that enable the frontend (Phase 6) to automatically generate configuration forms. The core technical challenge is embedding JSON Schema within YAML manifests and validating user-supplied parameters against those schemas on the backend.

The existing codebase already has a working `manifest.Loader` that reads YAML files from `games/`, a `GameManifest` struct with `Parameters map[string]string`, API handlers that serve game definitions via `/api/v1/games`, and a `handleCreateGameServer` that merges manifest defaults with user-supplied parameters. Phase 5 must evolve the manifest format to include a `parameterSchema` section (JSON Schema), add server-side validation of parameters against that schema, update the API to expose the schema to the frontend, enhance the Minecraft reference game with a complete schema, and add a Dockerfile per game as required by GAME-01.

**Primary recommendation:** Use `santhosh-tekuri/jsonschema/v6` for JSON Schema validation on the backend, embed schemas as inline YAML objects in manifests (they serialize to valid JSON Schema), expose the raw schema object via the `/api/v1/games/{gameType}` endpoint for frontend consumption, and structure games as `games/<game>/manifest.yaml` + `games/<game>/Dockerfile` directories rather than flat YAML files.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/santhosh-tekuri/jsonschema/v6` | v6.0.2 | JSON Schema compilation and validation | Most mature Go JSON Schema library; supports Draft 2020-12, Draft-07; passes JSON Schema Test Suite; Apache-2.0 license; well-maintained |
| `gopkg.in/yaml.v3` | v3.0.1 | YAML manifest parsing | Already in go.mod; handles YAML-to-Go type mapping for inline schemas |
| `encoding/json` | stdlib | JSON Schema serialization to API | Already used; JSON Schema embedded in YAML unmarshals to `map[string]interface{}` which serializes to JSON natively |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `sigs.k8s.io/controller-runtime/pkg/client` | v0.23.1 | K8s client (already in use) | No new K8s-level changes needed; game definitions are file-based, not CRD-based |
| `net/http/httptest` | stdlib | API handler testing | Testing updated game manifest endpoints with schema responses |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `santhosh-tekuri/jsonschema/v6` | `google/jsonschema-go` (v0.4.2) | Google's package is brand new (Dec 2025), pre-v1, MIT licensed, simpler API. However, it only supports Draft-07 and 2020-12, ignores `format` keyword entirely, and uses Go regexp (not ECMA 262). santhosh-tekuri is battle-tested, supports more drafts, and has format assertion support. |
| `santhosh-tekuri/jsonschema/v6` | `kaptinlin/jsonschema` | Newer library aligned with Draft 2020-12, supports i18n error messages. Less proven than santhosh-tekuri, fewer GitHub stars, less community adoption. |
| `santhosh-tekuri/jsonschema/v6` | `xeipuuv/gojsonschema` | Established but less actively maintained; only supports up to Draft-07; no Draft 2020-12 support. |
| JSON Schema for parameters | Laravel-style validation rules (like Pterodactyl) | JSON Schema is a standard with frontend library support (react-jsonschema-form); custom validation rules require custom frontend rendering logic. JSON Schema is the right choice for a system that needs automatic UI generation. |

**Installation:**
```bash
go get github.com/santhosh-tekuri/jsonschema/v6@v6.0.2
```

Note: `gopkg.in/yaml.v3` and `encoding/json` are already in go.mod.

## Architecture Patterns

### Recommended Project Structure
```
games/                              # Game definitions directory (project root)
  minecraft/                        # One directory per game
    manifest.yaml                   # Game metadata + parameter schema
    Dockerfile                      # Game-specific container build (optional, can reference existing image)
  valheim/                          # Example second game
    manifest.yaml
    Dockerfile
internal/
  manifest/                         # EXISTING - Game manifest loading (evolve)
    manifest.go                     # GameManifest struct + Loader (add ParameterSchema)
    manifest_test.go                # Existing + new schema tests
    validate.go                     # NEW - JSON Schema validation of parameters
    validate_test.go                # NEW - Validation tests
  api/                              # EXISTING - REST API handlers (evolve)
    handlers_games.go               # Update GameResponse to include schema
    handlers_gameserver.go          # Add schema validation on create/update
    response.go                     # Update GameResponse type
docs/                               # NEW - Contribution guide
  game-definitions.md               # How to contribute a new game via PR
```

### Pattern 1: Directory-Per-Game Structure
**What:** Each game is a subdirectory under `games/` containing a `manifest.yaml` and optionally a `Dockerfile`, instead of a single flat YAML file.
**When to use:** Always -- GAME-01 requires "Dockerfile + manifest.yaml per game in games/ directory."
**Why:** Allows co-locating the Dockerfile with the manifest. Some games need custom Docker images (e.g., combining SteamCMD with specific startup scripts); others reference existing community images directly.
**Migration:** The loader must change from reading flat `.yaml` files to scanning subdirectories for `manifest.yaml`.

```
games/
  minecraft/
    manifest.yaml          # Game definition
    Dockerfile             # FROM itzg/minecraft-server:latest (thin wrapper or passthrough)
  valheim/
    manifest.yaml
    Dockerfile             # FROM lloesche/valheim-server:latest
```

### Pattern 2: JSON Schema Embedded in YAML Manifest
**What:** The `parameterSchema` field in the manifest contains a JSON Schema definition written in YAML syntax. YAML is a superset of JSON, so a YAML-encoded schema is equivalent to a JSON Schema.
**When to use:** Defining configurable parameters for a game.
**Example:**
```yaml
# games/minecraft/manifest.yaml
name: minecraft
displayName: "Minecraft Java Edition"
image: itzg/minecraft-server:latest
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
# Default parameter values (used when creating a server)
parameters:
  EULA: "TRUE"
  TYPE: "VANILLA"
  DIFFICULTY: "normal"
  MODE: "survival"
  MAX_PLAYERS: "20"
  MEMORY: "2G"
# JSON Schema defining each parameter's type, constraints, and UI hints
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
      description: "Minecraft server implementation"
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
      description: "Maximum number of concurrent players"
      pattern: "^[1-9][0-9]*$"
      default: "20"
    MEMORY:
      type: string
      title: "JVM Memory"
      description: "Java heap memory allocation"
      enum: ["1G", "2G", "4G", "8G"]
      default: "2G"
    MOTD:
      type: string
      title: "Server Message"
      description: "Message displayed in the server list"
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
      description: "Require Mojang authentication"
      enum: ["true", "false"]
      default: "true"
  required:
    - EULA
    - TYPE
```

### Pattern 3: Schema Validation on Server-Side
**What:** When a user creates or updates a GameServer, validate the submitted parameters against the game's `parameterSchema` before creating the K8s resource.
**When to use:** `handleCreateGameServer` and `handleUpdateGameServer` handlers.
**Example:**
```go
// internal/manifest/validate.go
package manifest

import (
    "encoding/json"
    "fmt"

    "github.com/santhosh-tekuri/jsonschema/v6"
)

// ValidateParameters validates a map of string parameters against the
// game manifest's JSON Schema. Returns nil if validation passes or if
// the manifest has no schema defined.
func (m *GameManifest) ValidateParameters(params map[string]string) error {
    if m.ParameterSchema == nil {
        return nil // no schema = no validation
    }

    // Convert the schema (map[string]interface{}) to JSON bytes
    schemaJSON, err := json.Marshal(m.ParameterSchema)
    if err != nil {
        return fmt.Errorf("failed to marshal parameter schema: %w", err)
    }

    // Compile the schema
    c := jsonschema.NewCompiler()
    if err := c.AddResource("schema.json", jsonschema.UnmarshalJSON(bytes.NewReader(schemaJSON))); err != nil {
        return fmt.Errorf("failed to add schema resource: %w", err)
    }
    sch, err := c.Compile("schema.json")
    if err != nil {
        return fmt.Errorf("failed to compile parameter schema: %w", err)
    }

    // Convert params to interface{} map for validation
    instance := make(map[string]interface{}, len(params))
    for k, v := range params {
        instance[k] = v
    }

    // Validate
    if err := sch.Validate(instance); err != nil {
        return fmt.Errorf("parameter validation failed: %w", err)
    }
    return nil
}
```

### Pattern 4: API Response Exposes Schema for Frontend
**What:** The `/api/v1/games/{gameType}` endpoint returns the parameter schema as a raw JSON object so the frontend can use it with `react-jsonschema-form` or similar.
**When to use:** GameResponse needs a `ParameterSchema` field.
**Example:**
```go
// Updated GameResponse in handlers_games.go
type GameResponse struct {
    Name            string                 `json:"name"`
    DisplayName     string                 `json:"displayName"`
    Image           string                 `json:"image"`
    Ports           []PortInfo             `json:"ports"`
    Parameters      map[string]string      `json:"parameters"`
    ParameterSchema map[string]interface{} `json:"parameterSchema,omitempty"`
}
```

The frontend (Phase 6) will consume `parameterSchema` directly as a JSON Schema and render it with `react-jsonschema-form` (RJSF). This is exactly the pattern used by Helm chart forms in OpenShift Console and Rancher.

### Pattern 5: Dockerfile as Thin Wrapper
**What:** The Dockerfile for a game that uses an existing community image can be a single `FROM` line, serving primarily as documentation and as a hook for future customization.
**When to use:** Games that have well-maintained community Docker images (Minecraft via itzg, Valheim via lloesche).
**Example:**
```dockerfile
# games/minecraft/Dockerfile
# Minecraft Java Edition game server
# Based on the community-maintained itzg/minecraft-server image
# https://github.com/itzg/docker-minecraft-server
FROM itzg/minecraft-server:latest
```

For games that need custom setup (SteamCMD-based games, custom startup scripts), the Dockerfile becomes more substantial.

### Anti-Patterns to Avoid
- **Custom validation rule syntax:** Do NOT invent a Pterodactyl-style `required|string|between:1,10` rule syntax. JSON Schema is a standard with frontend library support. Custom rules require custom frontend rendering.
- **Separate schema files:** Do NOT put the schema in a separate `schema.json` file alongside the manifest. Inline is simpler, keeps everything in one file, and YAML can express JSON Schema cleanly.
- **Typed parameter values in manifests:** Parameters are `map[string]string` because they become container environment variables. Do NOT change this to typed values -- env vars are always strings. The JSON Schema defines the type for validation and UI purposes, but the stored value is always a string.
- **Compiling schemas at request time:** Compile schemas once at loader startup, not per-request. Schema compilation is expensive; validation is cheap.
- **Breaking the existing API contract:** The current `GET /api/v1/games` response format must remain backward-compatible. Add `parameterSchema` as an optional field, do not remove existing fields.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON Schema validation | Custom validation logic per field type | `santhosh-tekuri/jsonschema/v6` | Enum, pattern, min/max, required, const -- all handled by the library. Custom validation misses edge cases. |
| Frontend form generation from schema | Custom form renderer per field | `react-jsonschema-form` (Phase 6) | Standard library that reads JSON Schema and renders forms. Pterodactyl-style custom rendering is a maintenance burden. |
| YAML Schema equivalence | Converting YAML to JSON Schema manually | `yaml.Unmarshal` into `map[string]interface{}` | YAML is a superset of JSON; YAML maps unmarshal to the same types as JSON objects. No conversion needed. |
| Schema documentation | Writing separate docs for each parameter | JSON Schema `title` + `description` fields | These fields are consumed by both the UI (for labels/tooltips) and the contribution guide. |
| Parameter default merging | Custom default-setting logic | JSON Schema `default` field + existing `mergeMaps()` | The manifest `parameters` map holds defaults; the schema `default` values document them. The existing `mergeMaps` in `handlers_gameserver.go` already handles merging. |

**Key insight:** JSON Schema is both a validation tool AND a UI specification. By embedding it in the manifest, you get server-side validation, frontend form generation, and parameter documentation from a single source of truth.

## Common Pitfalls

### Pitfall 1: All Parameters Are Strings
**What goes wrong:** Developer changes `Parameters map[string]string` to `map[string]interface{}` to support typed values from the schema.
**Why it happens:** The JSON Schema defines types like `integer`, `boolean`, etc., which suggests the parameters should be typed.
**How to avoid:** Parameters are environment variables. Environment variables are always strings. Keep `Parameters map[string]string`. The JSON Schema type is for UI and validation purposes -- the UI can render a number input but stores the value as `"20"`, not `20`. The schema validation must accept string values where the schema says `type: string` (which all parameters should use).
**Warning signs:** Compilation errors around type mismatches when validating parameters.

### Pitfall 2: Schema Compilation Per Request
**What goes wrong:** Every `handleCreateGameServer` call compiles the JSON Schema from scratch, causing latency spikes.
**Why it happens:** Developer puts schema compilation inside the validation function.
**How to avoid:** Compile schemas once during `LoadFromDirectory()`. Store the compiled `*jsonschema.Schema` alongside the `GameManifest`. The `ValidateParameters()` method uses the pre-compiled schema.
**Warning signs:** High latency on game server creation endpoints.

### Pitfall 3: Breaking the Existing Loader Contract
**What goes wrong:** Changing from flat files to directory structure breaks existing tests and the `cmd/main.go` invocation.
**Why it happens:** The `LoadFromDirectory()` function currently reads `.yaml` files directly from the directory; the new structure expects subdirectories.
**How to avoid:** Update `LoadFromDirectory()` to scan for subdirectories containing `manifest.yaml`. Update tests to create the new directory structure. Update `.dockerignore` if needed so the `games/` directory is included in the Docker build context.
**Warning signs:** `manifest.LoadFromDirectory("games/")` failing after restructure.

### Pitfall 4: Dockerfile Not Included in Container Image
**What goes wrong:** The operator container image does not include the `games/` directory, so `LoadFromDirectory` fails at runtime.
**Why it happens:** The current `Dockerfile` copies `COPY . .` but the `.dockerignore` might exclude the `games/` directory, or the Dockerfile only copies the binary.
**How to avoid:** The current Dockerfile copies `COPY . .` then builds, so `games/` is available at build time but NOT in the final distroless stage. The `games/` directory must be explicitly copied into the final stage: `COPY --from=builder /workspace/games /games`.
**Warning signs:** Container starts but crashes with "failed to load game manifests."

### Pitfall 5: YAML Anchors/Aliases in Schema
**What goes wrong:** YAML anchors (`&`, `*`) in the schema confuse the JSON Schema compiler because they are YAML features, not JSON features.
**Why it happens:** Someone writes YAML-specific syntax in the schema section.
**How to avoid:** Document that the `parameterSchema` section must use only JSON-compatible YAML (no anchors, no YAML-specific types). `yaml.v3` resolves anchors before Go sees the data, so technically it works, but it makes schemas harder to read and port to JSON.
**Warning signs:** Schema works in YAML but fails when serialized to JSON.

### Pitfall 6: Missing Validation on Update Path
**What goes wrong:** `handleCreateGameServer` validates parameters against the schema, but `handleUpdateGameServer` does not.
**Why it happens:** Developer adds validation to create but forgets update.
**How to avoid:** The merged parameters (existing + update) must be validated against the schema. Add validation after `mergeMaps()` in the update handler.
**Warning signs:** Users can bypass schema constraints by creating with valid params then updating with invalid ones.

## Code Examples

Verified patterns from official sources and the existing codebase:

### Updated GameManifest Struct
```go
// internal/manifest/manifest.go
// Source: existing codebase + JSON Schema integration

// GameManifest defines a game type template loaded from a YAML manifest.
type GameManifest struct {
    Name            string                          `yaml:"name"`
    DisplayName     string                          `yaml:"displayName"`
    Image           string                          `yaml:"image"`
    Ports           []gamev1alpha1.GameServerPort    `yaml:"ports"`
    Parameters      map[string]string               `yaml:"parameters"`
    Resources       corev1.ResourceRequirements      `yaml:"-"`

    // ParameterSchema is the raw JSON Schema object defining parameter
    // types, constraints, and UI metadata. Stored as a generic map
    // so it can be serialized to JSON and consumed by the frontend.
    ParameterSchema map[string]interface{}           `yaml:"-"`

    // compiledSchema is the pre-compiled JSON Schema for efficient
    // parameter validation. Set during LoadFromDirectory().
    compiledSchema *jsonschema.Schema
}
```

### Updated rawGameManifest with Schema
```go
// rawGameManifest intermediate type for YAML unmarshaling
type rawGameManifest struct {
    Name            string                 `yaml:"name"`
    DisplayName     string                 `yaml:"displayName"`
    Image           string                 `yaml:"image"`
    Ports           []rawPort              `yaml:"ports"`
    Parameters      map[string]string      `yaml:"parameters"`
    Resources       rawResources           `yaml:"resources"`
    ParameterSchema map[string]interface{} `yaml:"parameterSchema"`
}
```

### Schema Compilation During Loading
```go
// In LoadFromDirectory, after parsing the manifest:
import (
    "bytes"
    "encoding/json"
    "github.com/santhosh-tekuri/jsonschema/v6"
)

// Compile parameter schema if present
var compiledSchema *jsonschema.Schema
if raw.ParameterSchema != nil {
    schemaJSON, err := json.Marshal(raw.ParameterSchema)
    if err != nil {
        return nil, fmt.Errorf("manifest %s: failed to marshal parameter schema: %w", filePath, err)
    }

    c := jsonschema.NewCompiler()
    schemaURL := fmt.Sprintf("games/%s/manifest.yaml#/parameterSchema", raw.Name)
    if err := c.AddResource(schemaURL, jsonschema.UnmarshalJSON(bytes.NewReader(schemaJSON))); err != nil {
        return nil, fmt.Errorf("manifest %s: invalid parameter schema: %w", filePath, err)
    }
    compiledSchema, err = c.Compile(schemaURL)
    if err != nil {
        return nil, fmt.Errorf("manifest %s: failed to compile parameter schema: %w", filePath, err)
    }
}

m := &GameManifest{
    // ... existing fields ...
    ParameterSchema: raw.ParameterSchema,
    compiledSchema:  compiledSchema,
}
```

### Parameter Validation Method
```go
// ValidateParameters validates user-supplied parameters against the
// manifest's JSON Schema. Returns nil if no schema is defined.
func (m *GameManifest) ValidateParameters(params map[string]string) error {
    if m.compiledSchema == nil {
        return nil
    }

    // Convert string params to interface{} for schema validation
    instance := make(map[string]interface{}, len(params))
    for k, v := range params {
        instance[k] = v
    }

    if err := m.compiledSchema.Validate(instance); err != nil {
        return fmt.Errorf("parameter validation failed: %w", err)
    }
    return nil
}
```

### Updated Directory Scanner
```go
// LoadFromDirectory reads game manifests from subdirectories of dir.
// Each subdirectory must contain a manifest.yaml file.
func LoadFromDirectory(dir string) (*Loader, error) {
    entries, err := os.ReadDir(dir)
    if err != nil {
        return nil, fmt.Errorf("failed to read games directory %s: %w", dir, err)
    }

    manifests := make(map[string]*GameManifest)

    for _, entry := range entries {
        if !entry.IsDir() {
            // Also support flat .yaml files for backward compatibility
            // during migration (optional)
            continue
        }

        manifestPath := filepath.Join(dir, entry.Name(), "manifest.yaml")
        data, err := os.ReadFile(manifestPath)
        if err != nil {
            // Try manifest.yml as alternative
            manifestPath = filepath.Join(dir, entry.Name(), "manifest.yml")
            data, err = os.ReadFile(manifestPath)
            if err != nil {
                continue // skip directories without manifests
            }
        }

        // Parse and process manifest...
    }

    if len(manifests) == 0 {
        return nil, fmt.Errorf("no valid game manifests found in %s", dir)
    }

    return &Loader{manifests: manifests}, nil
}
```

### Updated API Response
```go
// handlers_games.go - Updated response with schema
type GameResponse struct {
    Name            string                 `json:"name"`
    DisplayName     string                 `json:"displayName"`
    Image           string                 `json:"image"`
    Ports           []PortInfo             `json:"ports"`
    Parameters      map[string]string      `json:"parameters"`
    ParameterSchema map[string]interface{} `json:"parameterSchema,omitempty"`
}

func gameManifestToResponse(m *manifest.GameManifest) *GameResponse {
    ports := make([]PortInfo, len(m.Ports))
    for i, p := range m.Ports {
        ports[i] = PortInfo{
            Name:          p.Name,
            ContainerPort: p.ContainerPort,
            Protocol:      string(p.Protocol),
        }
    }
    params := m.Parameters
    if params == nil {
        params = map[string]string{}
    }
    return &GameResponse{
        Name:            m.Name,
        DisplayName:     m.DisplayName,
        Image:           m.Image,
        Ports:           ports,
        Parameters:      params,
        ParameterSchema: m.ParameterSchema,
    }
}
```

### Validation in Create Handler
```go
// In handleCreateGameServer, after looking up the manifest:
m, ok := s.manifestLoader.Get(req.GameType)
if !ok {
    respondError(w, http.StatusBadRequest, "unknown game type: "+req.GameType)
    return
}

// Merge defaults with user overrides
parameters := mergeMaps(m.Parameters, req.Parameters)

// Validate merged parameters against schema
if err := m.ValidateParameters(parameters); err != nil {
    respondError(w, http.StatusBadRequest, err.Error())
    return
}
```

### Minecraft Reference Dockerfile
```dockerfile
# games/minecraft/Dockerfile
#
# Minecraft Java Edition game server
# Uses the community-maintained itzg/minecraft-server image which handles
# automatic version detection, mod loader installation, and server.properties
# management via environment variables.
#
# Documentation: https://docker-minecraft-server.readthedocs.io/
# Source: https://github.com/itzg/docker-minecraft-server
#
# This Dockerfile serves as a reference implementation for game definitions.
# For games with existing community images, it can be a thin wrapper.
# For games requiring custom setup, add SteamCMD, startup scripts, etc.

FROM itzg/minecraft-server:latest
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Pterodactyl Egg JSON with Laravel validation rules | JSON Schema for parameter definitions | JSON Schema adopted across Helm/OpenShift/Rancher 2020+ | Standard schema language with frontend library support; eliminates custom validation code |
| Flat YAML file per game | Directory per game (Dockerfile + manifest) | Kubernetes Operator patterns (Helm charts as directories) | Co-locates all game-related artifacts; enables future build pipeline |
| `map[string]string` parameters with no validation | Schema-validated parameters | This phase | Prevents invalid configurations; enables dynamic UI forms |
| Hand-written UI forms per game | Auto-generated forms from JSON Schema | react-jsonschema-form (RJSF) mature since 2020+ | New games automatically get configuration UI with no frontend code changes |

**Deprecated/outdated:**
- Pterodactyl-style egg JSON: Complex custom format requiring game-specific UI logic; replaced by JSON Schema which is a standard
- Single flat YAML manifest: Phase 4's approach works but does not satisfy GAME-01 (Dockerfile + manifest per game)

## JSON Schema Design Considerations

### Why `type: string` for All Parameters

Game server parameters become container environment variables, which are always strings. The JSON Schema should use `type: string` for all properties with additional constraints:

- Use `enum` for fixed choices (difficulty, game mode, server type)
- Use `pattern` for numeric strings (`"^[1-9][0-9]*$"` for MAX_PLAYERS)
- Use `const` for fixed values (EULA must be "TRUE")
- Use `maxLength` for free-text fields (MOTD)
- Use `default` to document the default value

This approach means the schema validation is straightforward (all values are strings being validated against string constraints) and the frontend renders appropriate widgets based on the constraint type:
- `enum` -> dropdown/select
- `const` -> disabled/hidden field
- `pattern` with numeric regex -> number input
- `maxLength` -> text input with character limit
- No constraints -> plain text input

### UI Metadata in Schema

JSON Schema supports `title` and `description` on each property. These are consumed by `react-jsonschema-form` to render labels and help text. Additional UI hints can go in a separate `uiSchema` section if needed (Phase 6 concern), but `title`/`description`/`enum`/`default` cover 90% of cases.

## Minecraft Reference Game - Environment Variables

Based on the `itzg/minecraft-server` Docker image documentation, these are the key configurable parameters for the reference Minecraft game definition:

| Env Var | Purpose | Values | Default |
|---------|---------|--------|---------|
| `EULA` | Accept Minecraft EULA | `TRUE` (required) | `TRUE` |
| `TYPE` | Server implementation | `VANILLA`, `PAPER`, `SPIGOT`, `FORGE`, `FABRIC`, `QUILT` | `VANILLA` |
| `DIFFICULTY` | Game difficulty | `peaceful`, `easy`, `normal`, `hard` | `normal` |
| `MODE` | Default game mode | `survival`, `creative`, `adventure`, `spectator` | `survival` |
| `MAX_PLAYERS` | Max concurrent players | Integer string | `20` |
| `MEMORY` | JVM heap allocation | `1G`, `2G`, `4G`, `8G` | `2G` |
| `MOTD` | Server list message | String (max 59 chars) | "" |
| `PVP` | Player combat | `true`, `false` | `true` |
| `SEED` | World generation seed | String | "" |
| `ONLINE_MODE` | Mojang authentication | `true`, `false` | `true` |
| `VERSION` | Minecraft version | Version string or `LATEST` | `LATEST` |
| `LEVEL` | World save name | String | `world` |

## Contribution Guide Outline

GAME-05 requires documentation for contributing new game definitions via PR. The guide should cover:

1. **Directory structure**: Create `games/<gamename>/` with `manifest.yaml` and `Dockerfile`
2. **Manifest format**: Required fields (`name`, `displayName`, `image`, `ports`), optional fields (`parameters`, `parameterSchema`, `resources`)
3. **Parameter schema**: How to define JSON Schema for configurable parameters, using the Minecraft manifest as a reference
4. **Dockerfile conventions**: Use existing community images when available; include documentation comments
5. **Testing**: How to validate the manifest loads correctly (`go test ./internal/manifest/...`)
6. **PR checklist**: Manifest validates, image is publicly accessible, schema generates correct forms, documentation comments present

## Open Questions

1. **Schema caching strategy**
   - What we know: Schemas should be compiled once at startup, not per-request
   - What's unclear: Should we support schema hot-reload (watching the games/ directory)?
   - Recommendation: Compile once at startup, matching Phase 4's approach. Hot-reload is a future enhancement. The operator restarts when game definitions change (new container image with updated games/).

2. **Dockerfile build pipeline**
   - What we know: GAME-01 requires Dockerfiles per game, but the current system uses pre-existing images (`itzg/minecraft-server:latest`)
   - What's unclear: Should the operator build game images from Dockerfiles? Or are Dockerfiles just documentation?
   - Recommendation: Dockerfiles are documentation and a future build hook. The `image` field in the manifest specifies the actual container image to use. The Dockerfile documents how that image is built or what base image it uses. A CI/CD pipeline can build custom images from these Dockerfiles in the future.

3. **Schema draft version**
   - What we know: `santhosh-tekuri/jsonschema/v6` supports Draft-07 through Draft 2020-12
   - What's unclear: Which draft should game manifests target?
   - Recommendation: Use Draft 2020-12 (the latest) since `react-jsonschema-form` supports it and it is the current standard. Add `$schema` to manifests for clarity.

4. **Frontend schema consumption (Phase 6 dependency)**
   - What we know: The API will expose the raw schema; Phase 6 will render forms from it
   - What's unclear: Does the API need to transform the schema for the frontend, or pass it through raw?
   - Recommendation: Pass the schema through raw. `react-jsonschema-form` consumes standard JSON Schema directly. Any UI-specific hints (widget types, ordering) go in a separate `uiSchema` field in the manifest, which the API also passes through.

## Sources

### Primary (HIGH confidence)
- Existing codebase analysis: `internal/manifest/manifest.go`, `internal/api/handlers_games.go`, `internal/api/handlers_gameserver.go`, `games/minecraft.yaml`, `cmd/main.go` -- direct code review
- [santhosh-tekuri/jsonschema v6](https://pkg.go.dev/github.com/santhosh-tekuri/jsonschema/v6) -- API docs, version v6.0.2, published May 2025
- [itzg/docker-minecraft-server docs](https://docker-minecraft-server.readthedocs.io/en/latest/) -- environment variables reference
- [itzg/docker-minecraft-server GitHub](https://github.com/itzg/docker-minecraft-server) -- server properties configuration

### Secondary (MEDIUM confidence)
- [Google JSON Schema package blog post](https://opensource.googleblog.com/2026/01/a-json-schema-package-for-go.html) -- alternative library evaluation
- [Pterodactyl egg documentation](https://pterodactyl.io/community/config/eggs/creating_a_custom_egg.html) -- competitor pattern analysis
- [Helm JSON Schema and Generated Forms](https://codeengineered.com/blog/2020/helm-json-schema/) -- pattern validation for schema-driven forms
- [react-jsonschema-form](https://github.com/rjsf-team/react-jsonschema-form) -- frontend form generation library (Phase 6 dependency)

### Tertiary (LOW confidence)
- JSON Schema draft version compatibility between `santhosh-tekuri/jsonschema/v6` and `react-jsonschema-form` -- not directly verified, but both claim Draft 2020-12 support

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - `santhosh-tekuri/jsonschema/v6` is well-documented, v6.0.2 confirmed on pkg.go.dev, Apache-2.0 licensed
- Architecture: HIGH - Based on direct analysis of existing codebase; directory-per-game and inline schema are straightforward extensions of current patterns
- Pitfalls: HIGH - Identified from direct code review (string parameters, Dockerfile staging, schema compilation, loader contract changes)
- Minecraft reference: HIGH - `itzg/minecraft-server` environment variables verified against official documentation
- Frontend integration: MEDIUM - react-jsonschema-form pattern is well-established but Phase 6 implementation details are not yet decided

**Research date:** 2026-02-10
**Valid until:** 2026-03-12 (30 days - stable domain, well-established patterns)
