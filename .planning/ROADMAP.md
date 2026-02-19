# Roadmap: Kterodactyl

## Milestones

- ✅ **v1.0 MVP** — Phases 1-12 (shipped 2026-02-13)
- 🚧 **v1.1 End-to-End CI/CD Test Suite** — Phases 13-18 (in progress)

## Phases

<details>
<summary>✅ v1.0 MVP (Phases 1-12) — SHIPPED 2026-02-13</summary>

- [x] Phase 1: Operator Foundation (4/4 plans) — completed 2026-02-10
- [x] Phase 2: Networking & DNS (3/3 plans) — completed 2026-02-10
- [x] Phase 3: Authentication (3/3 plans) — completed 2026-02-10
- [x] Phase 4: API Server Bridge (4/4 plans) — completed 2026-02-10
- [x] Phase 5: Game Definition Framework (2/2 plans) — completed 2026-02-11
- [x] Phase 6: Frontend UI (4/4 plans) — completed 2026-02-11
- [x] Phase 7: Console & Real-time Features (2/2 plans) — completed 2026-02-12
- [x] Phase 8: Mod Support (3/3 plans) — completed 2026-02-12
- [x] Phase 9: Backup System (3/3 plans) — completed 2026-02-12
- [x] Phase 10: Observability (2/2 plans) — completed 2026-02-12
- [x] Phase 11: Helm Packaging (2/2 plans) — completed 2026-02-12
- [x] Phase 12: Documentation (2/2 plans) — completed 2026-02-13

Full details: `.planning/milestones/v1.0-ROADMAP.md`

</details>

### 🚧 v1.1 End-to-End CI/CD Test Suite (In Progress)

**Milestone Goal:** Add a reproducible test suite (Playwright E2E + Go API integration) with a kind-based environment, then wire it into GitHub Actions CI.

- [x] **Phase 13: Go Test Foundation** - Fix envtest cached-client pattern, add handler-level httptest tests for untested endpoints, establish Makefile targets and test isolation (completed 2026-02-18)
- [x] **Phase 14: Go API Integration Tests** - Multi-step blackbox lifecycle tests in test/integration/ using httptest.NewServer with real TCP round-trips (completed 2026-02-18)
- [x] **Phase 15: Kind Cluster Environment** - Kind cluster lifecycle with Helm-based deployment, NodePort access, and Makefile targets for create/teardown (completed 2026-02-19)
- [x] **Phase 16: Playwright E2E Tests** - Browser tests against live app in kind covering auth, server CRUD, and admin flows (completed 2026-02-19)
- [ ] **Phase 17: CI Pipeline** - Unified GitHub Actions workflow running all test tiers with job dependencies, failure artifacts, and cleanup
- [ ] **Phase 18: Coverage and Test Backlog** - Go coverage reporting in CI and test backlog documenting untested features for future milestones

## Phase Details

### Phase 13: Go Test Foundation
**Goal**: Developers have a reliable, fast Go test suite covering all API handlers with proper isolation and selective execution
**Depends on**: Nothing (first phase of v1.1)
**Requirements**: INFRA-03, INFRA-04, GAPI-01, GAPI-02, GAPI-03
**Success Criteria** (what must be TRUE):
  1. `make test` runs all Go unit tests and passes; `make test-integration` runs integration tests separately
  2. Mod handler tests exercise upload and list flows and pass against a fake K8s client
  3. Backup handler tests exercise create, list, and restore flows and pass against a fake K8s client
  4. Metrics proxy handler tests pass against a fake K8s client
  5. Each test creates resources with unique names and cleans up after itself — running the suite twice in a row produces no state leakage
**Plans**: 3 plans

Plans:
- [ ] 13-01-PLAN.md — Fix envtest cached-client, extend test helpers, add Makefile test tier targets
- [ ] 13-02-PLAN.md — Write mod handler and metrics proxy handler tests
- [ ] 13-03-PLAN.md — Write backup handler tests (create, list, delete, restore, schedule)

### Phase 14: Go API Integration Tests
**Goal**: A blackbox integration test validates the full API lifecycle end-to-end without requiring a Kubernetes cluster
**Depends on**: Phase 13
**Requirements**: GAPI-04
**Success Criteria** (what must be TRUE):
  1. `make test-integration` executes a multi-step test that registers a user, creates a server, retrieves it, and deletes it — all via real HTTP round-trips
  2. The integration test lives in `test/integration/` as a separate Go package, exercising the API as an external consumer would
**Plans**: 1 plan

Plans:
- [ ] 14-01-PLAN.md — Integration test with TestAPILifecycle (register, create, get, delete) via httptest.NewServer + Makefile target

### Phase 15: Kind Cluster Environment
**Goal**: Developers can spin up a complete Kterodactyl environment in kind for local and CI testing with a single command
**Depends on**: Phase 13
**Requirements**: INFRA-01, INFRA-02
**Success Criteria** (what must be TRUE):
  1. `make test-e2e-setup` (or equivalent target) creates a kind cluster, builds and loads the operator image, installs via Helm, and waits for readiness — app is accessible at localhost:8080
  2. `make test-e2e-teardown` (or equivalent target) deletes the kind cluster and all associated resources cleanly
  3. A developer can tear down and recreate the environment repeatedly without manual cleanup steps
**Plans**: 1 plan

Plans:
- [ ] 15-01-PLAN.md — Kind cluster lifecycle with Helm deployment, NodePort access, and Makefile targets

### Phase 16: Playwright E2E Tests
**Goal**: Browser-based tests verify core user journeys against a live Kterodactyl deployment in kind
**Depends on**: Phase 15
**Requirements**: PW-01, PW-02, PW-03, PW-04, PW-05
**Success Criteria** (what must be TRUE):
  1. `make test-playwright` runs the Playwright suite from the `e2e/` directory against the kind-deployed app
  2. Auth fixtures provide pre-authenticated browser contexts for admin and regular-user roles without per-test login
  3. A test signs up a new user, logs in, and verifies the dashboard loads
  4. A test creates a game server and verifies it appears in the server list
  5. A test deletes a game server and verifies it is removed from the server list
**Plans**: 2 plans

Plans:
- [ ] 16-01-PLAN.md — Playwright project init, auth fixtures, setup project, app-side token injection, Makefile target
- [ ] 16-02-PLAN.md — Auth E2E tests (login, sign up) and server CRUD E2E tests (create, list, delete)

### Phase 17: CI Pipeline
**Goal**: Every pull request automatically runs the full test suite with clear pass/fail status and failure diagnostics
**Depends on**: Phase 14, Phase 16
**Requirements**: CI-01, CI-02, CI-03, CI-04
**Success Criteria** (what must be TRUE):
  1. A unified `.github/workflows/ci.yml` runs lint, unit tests, integration tests, E2E tests, and Playwright tests in dependency order — a lint failure skips downstream jobs
  2. When Playwright tests fail, traces, screenshots, and Kubernetes pod logs are uploaded as downloadable GitHub Actions artifacts
  3. CI performs disk cleanup before kind cluster creation to prevent runner disk exhaustion
  4. The kind cluster is always deleted after E2E tests complete, even when tests fail
**Plans**: TBD

Plans:
- [ ] 17-01: TBD

### Phase 18: Coverage and Test Backlog
**Goal**: Test coverage is measurable and gaps are explicitly documented for future milestones
**Depends on**: Phase 17
**Requirements**: COV-01, COV-02
**Success Criteria** (what must be TRUE):
  1. CI output includes a Go test coverage percentage and the coverage report is accessible as a CI artifact or log output
  2. A `TEST_BACKLOG.md` document lists all untested features (WebSocket console, visual regression, mod E2E, backup E2E, admin E2E, multi-browser) with priority rankings for future milestones
**Plans**: TBD

Plans:
- [ ] 18-01: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 13 -> 14 -> 15 -> 16 -> 17 -> 18
(Phases 14 and 15 can execute in parallel — 14 depends on 13 only, 15 depends on 13 only)

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Operator Foundation | v1.0 | 4/4 | Complete | 2026-02-10 |
| 2. Networking & DNS | v1.0 | 3/3 | Complete | 2026-02-10 |
| 3. Authentication | v1.0 | 3/3 | Complete | 2026-02-10 |
| 4. API Server Bridge | v1.0 | 4/4 | Complete | 2026-02-10 |
| 5. Game Definition Framework | v1.0 | 2/2 | Complete | 2026-02-11 |
| 6. Frontend UI | v1.0 | 4/4 | Complete | 2026-02-11 |
| 7. Console & Real-time Features | v1.0 | 2/2 | Complete | 2026-02-12 |
| 8. Mod Support | v1.0 | 3/3 | Complete | 2026-02-12 |
| 9. Backup System | v1.0 | 3/3 | Complete | 2026-02-12 |
| 10. Observability | v1.0 | 2/2 | Complete | 2026-02-12 |
| 11. Helm Packaging | v1.0 | 2/2 | Complete | 2026-02-12 |
| 12. Documentation | v1.0 | 2/2 | Complete | 2026-02-13 |
| 13. Go Test Foundation | 3/3 | Complete    | 2026-02-18 | - |
| 14. Go API Integration Tests | 1/1 | Complete    | 2026-02-18 | - |
| 15. Kind Cluster Environment | 1/1 | Complete    | 2026-02-19 | - |
| 16. Playwright E2E Tests | 2/2 | Complete   | 2026-02-19 | - |
| 17. CI Pipeline | v1.1 | 0/? | Not started | - |
| 18. Coverage and Test Backlog | v1.1 | 0/? | Not started | - |
