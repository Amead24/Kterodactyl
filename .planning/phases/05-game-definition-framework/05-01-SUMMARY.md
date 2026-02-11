---
phase: 05-game-definition-framework
plan: 01
subsystem: manifest
tags: [jsonschema, yaml, validation, game-manifest, directory-per-game]

# Dependency graph
requires:
  - phase: 04-api-server-bridge
    provides: "Manifest loader (LoadFromDirectory), GameManifest struct, API handlers serving game definitions"
provides:
  - "Directory-per-game structure (games/<name>/manifest.yaml + Dockerfile)"
  - "JSON Schema parameter definitions embedded in YAML manifests"
  - "Schema compilation at load time via santhosh-tekuri/jsonschema/v6"
  - "ValidateParameters method for server-side parameter validation"
  - "Minecraft reference game with 10 configurable parameters and full JSON Schema"
affects: [05-02-api-schema-integration, 06-frontend, game-contributions]

# Tech tracking
tech-stack:
  added: ["github.com/santhosh-tekuri/jsonschema/v6 v6.0.2"]
  patterns: ["directory-per-game manifest structure", "JSON Schema embedded in YAML", "schema compiled once at load time", "ValidateParameters against compiled schema"]

key-files:
  created:
    - "internal/manifest/validate.go"
    - "games/minecraft/manifest.yaml"
    - "games/minecraft/Dockerfile"
  modified:
    - "internal/manifest/manifest.go"
    - "internal/manifest/manifest_test.go"
    - "go.mod"
    - "go.sum"

key-decisions:
  - "Schema URL uses simple path (games/<name>/parameterSchema.json) not JSON pointer fragment -- jsonschema v6 Compile resolves fragments as JSON pointers within the document"
  - "All parameters use type: string in JSON Schema because env vars are always strings -- constraints via enum, pattern, const, maxLength"
  - "Schemas compiled once during LoadFromDirectory, stored as compiledSchema on GameManifest -- no per-request compilation"

patterns-established:
  - "Directory-per-game: each game is games/<name>/ with manifest.yaml and optional Dockerfile"
  - "ParameterSchema as map[string]interface{} on GameManifest for JSON-serializable raw schema"
  - "compiledSchema as unexported field for efficient pre-compiled validation"
  - "writeManifest test helper creates game subdirectory + manifest.yaml in one call"

# Metrics
duration: 4min
completed: 2026-02-11
---

# Phase 5 Plan 1: Game Definition Framework Summary

**Directory-per-game manifest structure with JSON Schema parameter validation using santhosh-tekuri/jsonschema/v6 and Minecraft reference game with 10 schema-validated parameters**

## Performance

- **Duration:** 4 min
- **Started:** 2026-02-11T13:16:58Z
- **Completed:** 2026-02-11T13:21:43Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- Migrated from flat YAML files to directory-per-game structure (games/minecraft/ with manifest.yaml + Dockerfile)
- Added JSON Schema (Draft 2020-12) parameter definitions embedded in YAML manifests, compiled at load time
- Created ValidateParameters method and HasSchema accessor for server-side parameter validation
- Built comprehensive Minecraft reference game with 10 parameters (EULA, TYPE, DIFFICULTY, MODE, MAX_PLAYERS, MEMORY, MOTD, PVP, SEED, ONLINE_MODE) and full JSON Schema constraints
- 13 passing tests covering directory loading, schema compilation, validation (valid/invalid/required), error cases, and edge cases

## Task Commits

Each task was committed atomically:

1. **Task 1: Migrate to directory-per-game structure with JSON Schema support** - `4e6efbb` (feat)
2. **Task 2: Update manifest tests for directory-based loading and schema validation** - `19af912` (test)

## Files Created/Modified
- `internal/manifest/manifest.go` - Updated LoadFromDirectory to scan subdirectories, added ParameterSchema/compiledSchema fields, JSON Schema compilation during loading
- `internal/manifest/validate.go` - New file with ValidateParameters method and HasSchema accessor
- `internal/manifest/manifest_test.go` - Rewritten with 13 test functions covering directory loading, schema validation, error cases
- `games/minecraft/manifest.yaml` - Full Minecraft reference manifest with 10 parameters and parameterSchema section
- `games/minecraft/Dockerfile` - Thin wrapper over itzg/minecraft-server:latest
- `go.mod` - Added santhosh-tekuri/jsonschema/v6 v6.0.2 dependency
- `go.sum` - Updated checksums

## Decisions Made
- Used simple URL path (`games/<name>/parameterSchema.json`) instead of JSON pointer fragment for schema resource URL -- jsonschema v6's Compile resolves `#/path` as a JSON pointer within the document, which fails when the document IS the schema rather than a wrapper
- All parameter schema properties use `type: string` since env vars are always strings -- constraints applied via enum, pattern, const, maxLength
- Schemas compiled once during LoadFromDirectory and stored on GameManifest -- no per-request compilation overhead

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed JSON Schema resource URL fragment causing compilation failure**
- **Found during:** Task 2 (running tests)
- **Issue:** Schema URL `games/<name>/manifest.yaml#/parameterSchema` used a JSON pointer fragment, but the document added via AddResource was the schema itself (not a document containing a parameterSchema key). The compiler tried to resolve `/parameterSchema` as a JSON pointer within the schema and failed.
- **Fix:** Changed schema URL to `games/<name>/parameterSchema.json` (no fragment) since the added resource IS the schema document
- **Files modified:** `internal/manifest/manifest.go`
- **Verification:** All 13 tests pass, go build clean
- **Committed in:** `19af912` (part of Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Essential fix for schema compilation to work. No scope creep.

## Issues Encountered
- jsonschema v6 `UnmarshalJSON` returns `(any, error)` not just `any` -- fixed during Task 1 build verification before committing

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Manifest package ready for Plan 02 API integration (GameResponse.ParameterSchema, ValidateParameters in create/update handlers)
- LoadFromDirectory now provides ParameterSchema and compiledSchema on every GameManifest
- Minecraft reference game complete with full schema for API endpoint testing

---
*Phase: 05-game-definition-framework*
*Completed: 2026-02-11*

## Self-Check: PASSED

All files verified present, old file confirmed deleted, both commit hashes found in git log.
