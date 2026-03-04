# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.1 — End-to-End CI/CD Test Suite

**Shipped:** 2026-03-04
**Phases:** 5 | **Plans:** 8

### What Was Built
- Go test foundation: envtest fix, handler-level httptest tests for mod/backup/metrics endpoints
- API lifecycle integration test: register → create → get → delete via real HTTP round-trips
- Kind cluster test environment with Helm deployment and single-command setup/teardown
- Playwright E2E suite with auth fixtures, signup/login, and server CRUD tests
- Unified GitHub Actions CI pipeline with 5-job dependency chain and failure diagnostics

### What Worked
- Fixing envtest cached-client pattern first (Phase 13) unblocked all subsequent test phases cleanly
- Kind + NodePort + extraPortMappings approach gave reliable localhost access without port-forward fragility
- Playwright setup project pattern (not globalSetup) integrated well with HTML reports and traces
- Single ci.yml approach eliminated duplicate CI runs from multiple workflow files
- Phase parallelization (14 + 15 independent after 13) shortened the critical path

### What Was Inefficient
- Phase 18 (Coverage and Test Backlog) was scoped but never executed — could have been deferred during planning rather than at milestone completion
- Gap between Phase 16 completion (Feb 19) and Phase 17 execution (Mar 4) — 2 weeks idle

### Patterns Established
- `//go:build integration` build tag for test tier isolation
- `test.describe.serial` in Playwright for causally dependent CRUD operations
- `addInitScript` + `window.__KTERODACTYL_E2E_TOKEN` for Zustand token injection in E2E
- `if: always()` on kind cluster cleanup in CI; `if: !cancelled()` on artifact uploads
- `jlumbroso/free-disk-space` before Docker-heavy CI jobs

### Key Lessons
1. Plan realistic milestone scope — Phase 18 was low-value enough to skip, suggesting it shouldn't have been in v1.1
2. Blackbox integration tests (`map[string]interface{}` responses, no imported types) are more realistic but slightly harder to debug
3. Kind cluster setup is fast enough for CI (~2 min) but disk space is the real constraint on GitHub runners

### Cost Observations
- Model mix: quality profile (opus-dominant)
- Plan execution averaged 4 min per plan (8 plans, ~32 min total execution)
- Notable: very fast milestone — 8 plans in ~0.5 hours of execution time

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Phases | Plans | Key Change |
|-----------|--------|-------|------------|
| v1.0 MVP | 12 | 34 | Initial project build — operator, API, UI, full stack |
| v1.1 CI/CD | 5 | 8 | Testing infrastructure — smaller scope, faster phases |

### Cumulative Quality

| Milestone | Go Test Files | Playwright Specs | CI Pipeline |
|-----------|---------------|------------------|-------------|
| v1.0 | 8 | 0 | None |
| v1.1 | 16 | 2 | Unified 5-job CI |

### Top Lessons (Verified Across Milestones)

1. Fix foundational patterns first (v1.0: CRD before controllers; v1.1: envtest before handler tests)
2. Single-binary / single-file patterns reduce operational complexity (v1.0: embedded SPA; v1.1: unified ci.yml)
