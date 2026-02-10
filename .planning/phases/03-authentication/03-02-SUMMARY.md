---
phase: 03-authentication
plan: 02
subsystem: auth
tags: [jwt, hmac-sha256, kubernetes-secrets, signing-key, claims, token-refresh]

# Dependency graph
requires:
  - phase: 03-authentication-01
    provides: "User type, errors, password hashing (auth package foundation)"
  - phase: 01-operator-foundation
    provides: "AdminConfig, LoadAdminConfig, util.UserNamespace"
provides:
  - "JWTService with GenerateToken, ValidateToken, ShouldRefresh"
  - "KterodactylClaims with Username, Email, Role, Namespace"
  - "EnsureSigningKey for persistent JWT key in K8s Secret"
  - "AdminConfig auth fields (JWT expiry, invite expiry, SMTP, registration)"
  - "RBAC marker for Secrets CRUD"
affects: [03-authentication-03, 04-api-server, auth-middleware]

# Tech tracking
tech-stack:
  added: [golang-jwt/jwt/v5 v5.3.1]
  patterns: [HMAC-SHA256 JWT signing, K8s Secret-backed signing key persistence, auto-refresh threshold detection]

key-files:
  created: [internal/auth/jwt.go]
  modified: [internal/controller/gameserver_controller.go, go.mod, go.sum]

key-decisions:
  - "HMAC-SHA256 (HS256) chosen over RSA/ECDSA -- single service signs and verifies, simpler and sufficient for v1"
  - "2-hour refresh threshold for ShouldRefresh -- middleware can issue fresh tokens when expiry is within 2 hours"
  - "EnsureSigningKey as static function (not method) -- no JWTService needed to bootstrap key from K8s Secret"
  - "SMTPPassword intentionally excluded from AdminConfig ConfigMap -- read from separate Secret to avoid credential exposure"

patterns-established:
  - "JWT claims include namespace mapping (user-<username>) for request-scoped isolation"
  - "Signing key persisted in K8s Secret named kterodactyl-jwt-signing-key with kterodactyl.io labels"
  - "Token validation enforces method (HS256), issuer (kterodactyl), and expiration requirement"

# Metrics
duration: 8min
completed: 2026-02-10
---

# Phase 3 Plan 2: JWT Service and AdminConfig Auth Extensions Summary

**HMAC-SHA256 JWT service with K8s Secret-backed signing key persistence and AdminConfig auth/SMTP fields**

## Performance

- **Duration:** 8 min
- **Started:** 2026-02-10T22:21:12Z
- **Completed:** 2026-02-10T22:29:45Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- JWTService generates and validates HMAC-SHA256 JWT tokens with KterodactylClaims (Username, Email, Role, Namespace)
- EnsureSigningKey creates or loads a 256-bit signing key from K8s Secret for persistence across restarts
- ShouldRefresh detects tokens within 2 hours of expiry for auto-refresh by middleware
- AdminConfig extended with auth fields (JWTExpirationHours, InviteExpirationHours, RegistrationEnabled, PanelURL, SMTP)
- RBAC marker for Secrets CRUD added for user store and JWT key management

## Task Commits

Each task was committed atomically:

1. **Task 1: Create JWT service with signing key persistence** - `6d154f2` (feat)
2. **Task 2: Extend AdminConfig with auth fields and add Secrets RBAC** - `b1d9f76` (chore)

**Plan metadata:** (pending)

## Files Created/Modified
- `internal/auth/jwt.go` - JWT service: KterodactylClaims, JWTService, GenerateToken, ValidateToken, ShouldRefresh, EnsureSigningKey
- `internal/controller/gameserver_controller.go` - AdminConfig auth/SMTP fields, defaults, ConfigMap parsing, Secrets RBAC marker
- `go.mod` - Promoted golang-jwt/jwt/v5 and golang.org/x/crypto to direct dependencies
- `go.sum` - Updated checksums after go mod tidy

## Decisions Made
- HMAC-SHA256 (HS256) chosen for JWT signing -- appropriate for single-service architecture where the same process signs and verifies tokens
- 2-hour default refresh threshold -- balances session continuity with security (frontend gets fresh token in API responses)
- EnsureSigningKey is a static function taking client+namespace rather than a JWTService method -- allows bootstrapping the key before constructing the service
- SMTPPassword excluded from AdminConfig/ConfigMap -- stored in separate K8s Secret to prevent credential exposure in ConfigMap
- Token ID generated with crypto/rand (8 bytes, hex-encoded) for potential revocation tracking
- RegistrationEnabled defaults to true (invite-required registration, not open registration)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed duplicate User type declaration**
- **Found during:** Task 1 (JWT service creation)
- **Issue:** Plan specified defining a User struct in jwt.go, but auth.go (from plan 03-01) already defines User with more fields (PasswordHash, CreatedAt, InvitedBy)
- **Fix:** Removed the duplicate minimal User struct from jwt.go, using the existing User type from auth.go instead
- **Files modified:** internal/auth/jwt.go
- **Verification:** go build ./internal/auth/... compiles cleanly
- **Committed in:** 6d154f2 (Task 1 commit)

**2. [Rule 3 - Blocking] AdminConfig changes already committed by concurrent 03-01 execution**
- **Found during:** Task 2 (AdminConfig extension)
- **Issue:** Plan 03-01 (user store) was executed concurrently and already committed the identical AdminConfig auth fields, defaults, LoadAdminConfig parsing, and Secrets RBAC marker
- **Fix:** Verified the existing changes match the plan specification exactly. Committed only the go.mod/go.sum tidy changes
- **Files modified:** go.mod, go.sum (controller file was already at desired state)
- **Verification:** go build ./..., go test ./internal/controller/... both pass
- **Committed in:** b1d9f76 (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (1 bug, 1 blocking)
**Impact on plan:** Both deviations were necessary due to concurrent plan execution. No scope creep. All planned functionality delivered.

## Issues Encountered
- Go 1.18 was on PATH instead of Go 1.25.3 -- resolved by using /home/tony/sdk/go1.24/bin/go (which is actually Go 1.25.3)
- go.sum had missing entries after jwt dependency upgrade -- resolved with go mod tidy

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- JWT service ready for use by Phase 4 API server middleware
- Auth types (User, KterodactylClaims, JWTService) ready for integration
- AdminConfig auth fields ready for runtime configuration via ConfigMap
- Next plan (03-03) can build auth middleware and invitation flow on this foundation

## Self-Check: PASSED

- [x] internal/auth/jwt.go exists
- [x] internal/controller/gameserver_controller.go exists
- [x] 03-02-SUMMARY.md exists
- [x] Commit 6d154f2 exists (Task 1)
- [x] Commit b1d9f76 exists (Task 2)
- [x] go build ./... passes
- [x] go test ./internal/controller/... passes

---
*Phase: 03-authentication*
*Completed: 2026-02-10*
