# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-02-17)

**Core value:** Admins can deploy a single Helm chart and give their users self-service game server provisioning backed entirely by Kubernetes
**Current focus:** v1.1 End-to-End CI/CD Test Suite — Phase 16 (Playwright E2E Tests)

## Current Position

**Phase:** 16 of 18 (Playwright E2E Tests)
**Current Plan:** Not started
**Total Plans in Phase:** 2
**Status:** Milestone complete
**Last Activity:** 2026-02-19

Progress: [███████░░░] 71% (v1.1)

## Performance Metrics

**v1.0 Velocity:**
- Total plans completed: 34
- Average duration: 5min
- Total execution time: 2.76 hours

**v1.1 Velocity:**
- Total plans completed: 5
- Average duration: 4min
- Total execution time: 0.37 hours

| Phase | Plan | Duration | Tasks | Files |
|-------|------|----------|-------|-------|
| 16 | 02 | 2min | 2 | 2 |
| 16 | 01 | 10min | 1 | 9 |
| 15 | 01 | 2min | 2 | 6 |
| 14 | 01 | 5min | 2 | 2 |
| 13 | 01 | 4min | 2 | 3 |
| 13 | 03 | 3min | 2 | 1 |
| 13 | 02 | 3min | 2 | 2 |

## Accumulated Context

### Decisions

Key decisions logged in PROJECT.md Key Decisions table (14 decisions from v1.0, all marked good).

v1.1 decisions:
- Playwright in top-level `e2e/` directory (not inside `web/`)
- Kind with NodePort + extraPortMappings over kubectl port-forward
- Chromium-only, workers: 1 in CI
- Fix envtest cached-client pattern before writing new tests
- Unified CI pipeline replacing separate workflow files
- [Phase 13]: Reordered suite_test.go BeforeSuite: manager before namespace for cached client availability
- [Phase 13]: Restore happy path not tested (S3/exec scope); all validation paths covered
- [Phase 13]: Nil body for upload mod tests (validation rejects before ParseMultipartForm)
- [Phase 14]: Build tag //go:build integration for test isolation (matches e2e convention)
- [Phase 14]: Single sequential TestAPILifecycle (causally dependent steps)
- [Phase 14]: Blackbox map[string]interface{} responses (no imported types)
- [Phase 15]: listenAddress 0.0.0.0 for WSL2 compatibility (not 127.0.0.1)
- [Phase 15]: Port chain 30080->8080: kind containerPort matches nodePort, hostPort matches curl target
- [Phase 15]: pullPolicy Never mandatory for kind-loaded images (no registry)
- [Phase 15]: Coexist with existing setup-test-e2e/cleanup-test-e2e targets (no removal)
- [Phase 16]: addInitScript + window.__KTERODACTYL_E2E_TOKEN for Zustand token injection (no persist middleware)
- [Phase 16]: hack/hash-password.go over pre-computed hash constant (uses project auth package, always correct)
- [Phase 16]: Setup project pattern (not globalSetup) for HTML report and trace integration
- [Phase 16]: page.request.post for invite API call in signup test (browser context, not separate HTTP client)
- [Phase 16]: test.describe.serial for server CRUD to ensure create-before-list-before-delete ordering

### Pending Todos

- **TODO-02** (Testing): Create a Playwright script for CI/CD integration testing of features

### Blockers/Concerns

None active.

**Tech debt from v1.0 (non-blocking):**
- DNS requires human testing with live Gateway API controller and ExternalDNS
- Relative path `"games/"` in cmd/main.go relies on container WORKDIR
- handleUploadMod and handleRestoreBackup bypass IsValidTransition guard
- Duplicate s3CredentialsSecretName constant in controller and API handler

## Session Continuity

Last session: 2026-02-19
Stopped at: Completed 16-02-PLAN.md (Phase 16 complete)
Resume file: .planning/phases/16-playwright-e2e-tests/16-02-SUMMARY.md
