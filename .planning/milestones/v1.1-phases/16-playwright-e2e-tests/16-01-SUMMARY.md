---
phase: 16-playwright-e2e-tests
plan: 01
subsystem: testing
tags: [playwright, e2e, chromium, jwt, auth-fixtures, addInitScript, zustand]

# Dependency graph
requires:
  - phase: 15-kind-cluster-environment
    provides: Kind cluster with Helm-deployed Kterodactyl at localhost:8080
provides:
  - Playwright E2E project in e2e/ with Chromium-only config
  - Auth setup project that seeds admin via kubectl and authenticates both roles
  - Custom auth fixtures (adminPage/userPage) with JWT injection via addInitScript
  - hack/hash-password.go for Argon2id hashing using project auth package
  - E2E token injection support in auth-store.ts
  - make test-playwright target
affects: [16-02-PLAN, ci-pipeline]

# Tech tracking
tech-stack:
  added: ["@playwright/test ^1.58.2", "Chromium browser (headless)"]
  patterns: ["Setup project for auth seeding", "addInitScript for Zustand token injection", "Custom test.extend() fixtures for role-based auth"]

key-files:
  created:
    - e2e/package.json
    - e2e/playwright.config.ts
    - e2e/.gitignore
    - e2e/fixtures/auth.ts
    - e2e/tests/auth.setup.ts
    - hack/hash-password.go
  modified:
    - web/src/stores/auth-store.ts
    - Makefile

key-decisions:
  - "addInitScript + window.__KTERODACTYL_E2E_TOKEN for Zustand token injection (no persist middleware)"
  - "hack/hash-password.go over pre-computed hash constant (uses project auth package, always correct)"
  - "Setup project pattern (not globalSetup) for HTML report and trace integration"

patterns-established:
  - "E2E token injection: addInitScript sets window global, auth-store reads it on init"
  - "Admin seeding via kubectl Secret creation with dry-run + apply for idempotency"
  - "Role-based fixtures: adminPage and userPage via test.extend()"

requirements-completed: [PW-01, PW-02]

# Metrics
duration: 10min
completed: 2026-02-19
---

# Phase 16 Plan 01: Playwright E2E Project Init Summary

**Playwright E2E project with Chromium config, auth fixtures using addInitScript token injection, kubectl-based admin seeding, and hack/hash-password.go helper**

## Performance

- **Duration:** 10min
- **Started:** 2026-02-19T19:40:40Z
- **Completed:** 2026-02-19T19:50:33Z
- **Tasks:** 1
- **Files modified:** 9

## Accomplishments
- Initialized e2e/ Playwright project with @playwright/test v1.58.2 and Chromium-only config
- Created auth setup project that seeds admin via kubectl, logs in, creates invite, registers user, and writes token files
- Created custom auth fixtures (adminPage/userPage) that inject JWT tokens via addInitScript
- Added window.__KTERODACTYL_E2E_TOKEN support to auth-store.ts for E2E auth bypass
- Created hack/hash-password.go to generate Argon2id hashes using the project's auth package
- Updated Makefile test-playwright target to run Playwright suite

## Task Commits

Each task was committed atomically:

1. **Task 1: Initialize Playwright project, config, fixtures, setup, and app-side token injection** - `068daad` (feat)

## Files Created/Modified
- `e2e/package.json` - Playwright E2E project manifest
- `e2e/playwright.config.ts` - Chromium-only config with setup project dependency
- `e2e/.gitignore` - Ignores playwright/.auth/, test-results/, playwright-report/
- `e2e/fixtures/auth.ts` - Custom test.extend() with adminPage and userPage fixtures
- `e2e/tests/auth.setup.ts` - Setup project: seeds admin, authenticates both roles via API
- `hack/hash-password.go` - Argon2id hash generator using internal/auth package
- `web/src/stores/auth-store.ts` - Added E2E token injection via window.__KTERODACTYL_E2E_TOKEN
- `Makefile` - Updated test-playwright target to run cd e2e && npx playwright test
- `e2e/package-lock.json` - Lockfile for e2e/ dependencies

## Decisions Made
- Used addInitScript + window global for token injection instead of storageState (Zustand has no persist middleware, so storageState cannot restore auth)
- Created hack/hash-password.go instead of hardcoding a pre-computed hash (stays correct if password params change)
- Used setup project pattern (not globalSetup) for better HTML report and trace integration
- Auth setup uses kubectl dry-run + apply for idempotent admin seeding

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Go binary not on default PATH in sandbox environment; located at /home/tony/.local/go/bin/go. This does not affect CI where Go is on PATH.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Auth infrastructure ready for Plan 02 test specs (auth.spec.ts, servers.spec.ts)
- Setup project handles admin seeding, login, invite, registration
- Fixtures provide pre-authenticated adminPage and userPage for all test specs
- make test-playwright target ready (requires kind cluster from make test-e2e-setup)

## Self-Check: PASSED

All 6 created files verified on disk. Task commit 068daad verified in git log.

---
*Phase: 16-playwright-e2e-tests*
*Completed: 2026-02-19*
