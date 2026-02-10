---
phase: 03-authentication
plan: 03
subsystem: auth
tags: [http-middleware, jwt-auth, invitation-tokens, kubernetes-secrets, go-mail, smtp, bearer-auth]

# Dependency graph
requires:
  - phase: 03-authentication-01
    provides: "User type, errors, password hashing, UserStore, ValidateUsername"
  - phase: 03-authentication-02
    provides: "JWTService, KterodactylClaims, GenerateToken, ValidateToken, ShouldRefresh"
  - phase: 01-operator-foundation
    provides: "util.UserNamespace, util.LabelManagedByKterodactyl, util.ManagedByValue"
provides:
  - "AuthMiddleware with JWT Bearer token extraction and validation"
  - "Auto-refresh: X-Refresh-Token header when token nears expiry"
  - "RequireAdmin middleware for admin-only endpoints"
  - "GetUserFromContext helper for extracting claims from request context"
  - "InviteService for invitation token creation, K8s Secret storage, email delivery, and redemption"
  - "SMTPConfig struct for email configuration"
  - "Comprehensive unit tests for password, JWT, middleware, and validation"
affects: [04-api-server, api-routes, user-registration]

# Tech tracking
tech-stack:
  added: [github.com/wneessen/go-mail v0.7.2]
  patterns: [bearer-token-middleware, context-value-claims, invite-token-as-k8s-secret, auto-refresh-header]

key-files:
  created:
    - internal/auth/middleware.go
    - internal/auth/invite.go
    - internal/auth/auth_test.go
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "RequireAdmin is a standalone function (not AuthMiddleware method) for cleaner middleware chaining"
  - "Error responses use JSON-like format for consistency with Phase 4 API"
  - "Invite Secret named invite-<first-12-chars-of-token> for uniqueness with readability"
  - "SMTP failure on invite creation logs error but does not fail the invite (invite link still returned)"
  - "go-mail with TLSOpportunistic and SMTPAuthAutoDiscover for maximum SMTP server compatibility"

patterns-established:
  - "Bearer token middleware: Extract -> Validate -> Context -> (Auto-refresh) -> Next"
  - "Context key as unexported type with exported constant: prevents key collisions across packages"
  - "Invite token lifecycle: Create Secret -> Email/Return link -> Redeem (validate + delete)"
  - "Standard Go testing for auth package (not Ginkgo) -- standalone library tests without K8s envtest"

# Metrics
duration: 4min
completed: 2026-02-10
---

# Phase 3 Plan 3: Auth Middleware, Invitations, and Unit Tests Summary

**HTTP auth middleware with Bearer JWT validation and auto-refresh, K8s Secret-backed invite token flow with optional SMTP email, and 21 unit tests covering password/JWT/middleware/validation**

## Performance

- **Duration:** 4 min
- **Started:** 2026-02-10T22:32:15Z
- **Completed:** 2026-02-10T22:36:22Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- AuthMiddleware validates JWT from Bearer header, stores claims in request context, auto-issues refresh tokens for near-expiry tokens
- RequireAdmin middleware blocks non-admin users with 403, works as standard HTTP middleware chain
- InviteService creates cryptographic invite tokens stored as K8s Secrets with expiration annotations, sends email via go-mail or returns link
- 21 unit tests covering password hashing (PHC format, unique salts, correct/incorrect verification), username validation (valid/invalid/reserved), JWT (generate/validate, expired, wrong key, namespace claim, refresh), and middleware (valid/missing/invalid token, refresh header, admin/non-admin)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create auth middleware and invite service** - `d9b5955` (feat)
2. **Task 2: Create comprehensive auth package unit tests** - `633b051` (test)

**Plan metadata:** (pending)

## Files Created/Modified
- `internal/auth/middleware.go` - AuthMiddleware (JWT Bearer extraction, validation, auto-refresh), RequireAdmin, GetUserFromContext
- `internal/auth/invite.go` - InviteService (CreateInvite, RedeemInvite), SMTPConfig, sendInviteEmail via go-mail
- `internal/auth/auth_test.go` - 21 unit tests: password (5), username validation (3 groups), JWT (5), middleware (6), context helpers (2)
- `go.mod` - Added github.com/wneessen/go-mail v0.7.2
- `go.sum` - Updated checksums

## Decisions Made
- RequireAdmin is a standalone function rather than an AuthMiddleware method -- allows standard `mw.Authenticate(RequireAdmin(handler))` chaining without coupling
- Error responses use JSON-like string format (`{"error":"..."}`) for consistency with Phase 4 API server expectations
- Invite Secret naming uses `invite-<first-12-chars-of-token>` -- balances Secret name uniqueness with human readability in `kubectl get secrets`
- SMTP send failure during invite creation is logged but does not fail the invite -- the invite token is still created and the link can be returned to the admin
- go-mail configured with TLSOpportunistic (falls back to unencrypted) and SMTPAuthAutoDiscover for maximum SMTP server compatibility
- Standard Go testing used for auth package (not Ginkgo) -- auth is a standalone library; Ginkgo reserved for controller integration tests using envtest

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- System Go binary (`/usr/local/go/bin/go`) is Go 1.18 which cannot parse go.mod with Go 1.25.3 -- resolved by using `/home/tony/sdk/go1.24/bin/go` (actually Go 1.25.3, directory name is misleading)

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Auth package complete: all 6 files (auth.go, errors.go, password.go, store.go, jwt.go, middleware.go, invite.go) provide a full authentication layer
- Ready for Phase 4 API server to wire up: AuthMiddleware for route protection, InviteService for user onboarding, UserStore for user CRUD
- InviteService and UserStore require K8s client (integration-tested in Phase 4 with envtest, not unit-tested here)
- Phase 3 authentication is fully complete (3/3 plans done)

## Self-Check: PASSED

- [x] internal/auth/middleware.go exists
- [x] internal/auth/invite.go exists
- [x] internal/auth/auth_test.go exists
- [x] 03-03-SUMMARY.md exists
- [x] Commit d9b5955 exists (Task 1)
- [x] Commit 633b051 exists (Task 2)
- [x] go build ./... passes
- [x] go test ./internal/auth/... passes (21/21)
- [x] go vet ./internal/auth/... passes

---
*Phase: 03-authentication*
*Completed: 2026-02-10*
