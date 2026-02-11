---
phase: 04-api-server-bridge
plan: 03
subsystem: api
tags: [chi, rest-api, admin, invite, user-management, game-manifest, jwt]

# Dependency graph
requires:
  - phase: 04-01
    provides: "API server scaffold with chi router, manifest loader, middleware, response helpers"
  - phase: 03-03
    provides: "Auth middleware, InviteService, UserStore, JWTService"
provides:
  - "Game manifest list and detail API handlers"
  - "Admin invite creation handler with AdminConfig-driven expiration"
  - "Admin user list handler (excludes password hashes)"
  - "Admin user delete handler with self-delete protection"
  - "Shared test helpers (testServer, generateToken, doRequest)"
affects: [04-04, 05-panel-ui]

# Tech tracking
tech-stack:
  added: []
  patterns: ["UserResponse excludes PasswordHash for API safety", "Per-request AdminConfig loading for invite expiration", "testServer wrapper with JWT helper methods"]

key-files:
  created:
    - internal/api/handlers_games.go
    - internal/api/handlers_games_test.go
    - internal/api/handlers_admin.go
    - internal/api/handlers_admin_test.go
    - internal/api/helpers_test.go
  modified:
    - internal/api/routes.go
    - internal/api/handlers_auth_test.go

key-decisions:
  - "UserResponse struct explicitly excludes PasswordHash to prevent credential exposure in API responses"
  - "Admin invite handler loads AdminConfig per-request for InviteExpirationHours (defaults to 72h without ConfigMap)"
  - "Self-deletion prevention: admin cannot delete their own account via DELETE /admin/users/{username}"
  - "Shared test helpers consolidated into helpers_test.go with testServer wrapper pattern"
  - "GameResponse converts corev1.Protocol to plain string for JSON cleanliness"

patterns-established:
  - "testServer wrapper: encapsulates Server + JWTService with generateToken() and doRequest() helper methods"
  - "userToResponse mapping: explicitly maps User fields excluding PasswordHash (never use raw auth.User in responses)"
  - "gameManifestToResponse mapping: converts K8s types (Protocol) to API-friendly strings"

# Metrics
duration: 10min
completed: 2026-02-11
---

# Phase 4 Plan 3: Games and Admin Handlers Summary

**Game manifest list/detail handlers and admin handlers (invite, user list, user delete) with RequireAdmin enforcement and full test coverage**

## Performance

- **Duration:** 10 min
- **Started:** 2026-02-11T03:36:47Z
- **Completed:** 2026-02-11T03:47:28Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- Game manifest handlers return loaded manifests with display info, ports, and parameters
- Admin invite creation uses per-request AdminConfig for expiration hours (72h default)
- Admin user list explicitly excludes PasswordHash from all responses
- Admin user delete prevents self-deletion and returns proper error codes
- RequireAdmin middleware blocks all non-admin users with 403 on admin endpoints
- 11 new test cases covering all admin and game handler paths

## Task Commits

Tasks were committed by the background process across combined commits:

1. **Task 1: Game manifest handlers** - `23c824e` (feat) + `d46e77b` (refactor)
2. **Task 2: Admin handlers** - `bf8d75f` (feat)

## Files Created/Modified
- `internal/api/handlers_games.go` - Game manifest list and detail handlers with GameResponse/PortInfo types
- `internal/api/handlers_games_test.go` - Tests for list (auth enforcement, count, fields) and get (detail, 404)
- `internal/api/handlers_admin.go` - Admin invite creation, user list (safe response), user delete (self-delete protection)
- `internal/api/handlers_admin_test.go` - Tests for create invite (admin/non-admin/missing email), list users (password hash exclusion), delete user (exists/not-found/self/non-admin)
- `internal/api/helpers_test.go` - Shared test infrastructure: testServer, generateToken, doRequest, addAuthHeader, createTestUser, createTestInvite, createAdminConfigMap
- `internal/api/routes.go` - Removed admin handler stubs (replaced by real implementations)
- `internal/api/handlers_auth_test.go` - Refactored to use shared helpers from helpers_test.go

## Decisions Made
- UserResponse struct explicitly maps only safe fields (Username, Email, Role, CreatedAt, InvitedBy) to prevent PasswordHash exposure
- Admin invite handler loads AdminConfig per-request via s.loadAdminConfig() for InviteExpirationHours (defaults to 72h without ConfigMap)
- Self-deletion blocked with explicit username comparison: adminUsername == username returns 400 "cannot delete yourself"
- Test helpers consolidated into helpers_test.go to avoid duplication across handler test files
- GameResponse converts corev1.Protocol to string for JSON API cleanliness

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Go toolchain upgrade from 1.18 to 1.24.4**
- **Found during:** Task 1 (initial build verification)
- **Issue:** System Go 1.18 cannot parse go.mod with version 1.25.3
- **Fix:** Installed Go 1.24.4 to ~/.local/go which auto-bootstraps to 1.25.3 via GOTOOLCHAIN
- **Files modified:** None (environment change only)
- **Verification:** `go build ./internal/api/...` succeeds

**2. [Rule 3 - Blocking] Resolved test helper conflicts with existing Plan 02 code**
- **Found during:** Task 1 (test file creation)
- **Issue:** Plan 02 already executed and created handlers_auth_test.go with its own newTestServer. New helpers_test.go conflicted with existing declarations
- **Fix:** Consolidated all shared test helpers into helpers_test.go, removed duplicates from handlers_auth_test.go
- **Files modified:** internal/api/helpers_test.go, internal/api/handlers_auth_test.go
- **Verification:** All 28 API tests pass without redeclaration errors

**3. [Rule 3 - Blocking] Removed conflicting GameServer handler stubs**
- **Found during:** Task 2 (routes.go cleanup)
- **Issue:** Background process had already created handlers_gameserver.go with real implementations, conflicting with stubs in routes.go
- **Fix:** Removed all remaining stubs from routes.go since real implementations exist
- **Files modified:** internal/api/routes.go
- **Verification:** `go build ./internal/api/...` succeeds

---

**Total deviations:** 3 auto-fixed (3 blocking)
**Impact on plan:** All auto-fixes necessary to resolve environment and code conflicts. No scope creep.

## Issues Encountered
- Background linter process auto-committed changes during execution, preventing clean per-task atomic commits. Work is correctly committed but across combined commits rather than task-isolated ones.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All 13 API handler stubs are now replaced with real implementations
- Game, Admin, Auth, and GameServer CRUD handlers fully operational
- Ready for Plan 04-04 (if applicable) or Phase 5 (panel UI)
- All 28 API tests pass with full coverage of auth enforcement, error cases, and happy paths

## Self-Check: PASSED

All 7 files verified present. All 3 commit hashes verified in git log.

---
*Phase: 04-api-server-bridge*
*Completed: 2026-02-11*
