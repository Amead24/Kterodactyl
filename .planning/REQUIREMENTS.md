# Requirements: Kterodactyl

**Defined:** 2026-02-18
**Core Value:** Admins can deploy a single Helm chart and give their users self-service game server provisioning backed entirely by Kubernetes

## v1.1 Requirements

Requirements for v1.1 End-to-End CI/CD Test Suite. Each maps to roadmap phases.

### Test Infrastructure

- [x] **INFRA-01**: Developer can create a kind cluster with Helm-deployed Kterodactyl via a single Makefile target
- [x] **INFRA-02**: Developer can tear down the test environment via a single Makefile target
- [x] **INFRA-03**: Developer can run each test tier independently (make test, make test-integration, make test-e2e, make test-playwright)
- [x] **INFRA-04**: Each test creates and cleans up its own resources without leaking state to other tests

### Go API Tests

- [x] **GAPI-01**: Mod handler endpoints have httptest-based integration tests covering upload and list flows
- [x] **GAPI-02**: Backup handler endpoints have httptest-based integration tests covering create, list, and restore flows
- [x] **GAPI-03**: Metrics proxy handler has httptest-based integration tests
- [x] **GAPI-04**: Multi-step API flow test validates the full lifecycle: register user -> create server -> get server -> delete server

### Playwright E2E

- [x] **PW-01**: Playwright project is initialized with config, auth fixtures, and Chromium-only setup in a top-level `e2e/` directory
- [x] **PW-02**: Auth fixture creates storageState for admin and regular user roles
- [x] **PW-03**: User can sign up, log in, and see the dashboard in an E2E test
- [x] **PW-04**: User can create a game server and see it in the server list in an E2E test
- [x] **PW-05**: User can delete a game server in an E2E test

### CI Pipeline

- [ ] **CI-01**: Unified GitHub Actions workflow runs lint -> unit tests -> integration tests -> E2E tests -> Playwright tests with job dependencies
- [ ] **CI-02**: CI pipeline uploads Playwright traces, screenshots, and k8s logs as artifacts on failure
- [ ] **CI-03**: CI pipeline performs disk cleanup before heavy steps to prevent space exhaustion
- [ ] **CI-04**: Kind cluster is always cleaned up after E2E tests, even on failure

### Coverage Tracking

- [ ] **COV-01**: Go test coverage report is generated and accessible in CI output
- [ ] **COV-02**: Test backlog document lists all untested features with priority for future milestones

## Future Requirements

Deferred to future milestones. Tracked but not in current roadmap.

### Extended E2E Coverage

- **PW-06**: User can manage mods (upload, list, delete) in an E2E test
- **PW-07**: User can manage backups (create, list, restore) in an E2E test
- **PW-08**: Admin can invite users and manage settings in an E2E test
- **PW-09**: WebSocket console connects and streams logs in an E2E test
- **PW-10**: Visual regression baselines for key pages (dashboard, server detail, admin)

### Extended Go Tests

- **GAPI-05**: WebSocket console handler has integration tests for connect, stream, and command execution
- **GAPI-06**: Go binary coverage via `go build -cover` for E2E test runs

### CI Enhancements

- **CI-05**: Parallel CI job execution via matrix strategy to reduce total pipeline time
- **CI-06**: Helm chart validation tests (helm template + dry-run)

### Test Quality

- **COV-03**: data-testid attributes on all interactive React components for stable Playwright selectors
- **COV-04**: Flake detection and retry strategy with consistent timeout/polling constants

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Multi-browser testing (Firefox, WebKit) | Internal tool, Chromium sufficient; add if browser-specific bugs reported |
| Component testing (Storybook/Vitest) | Value is in integration, not component isolation; shadcn wrappers are thin |
| Running Playwright inside kind cluster (Testkube) | Massive complexity for single-project suite; run on host instead |
| Mocking Kubernetes in Playwright tests | Defeats E2E purpose; real kind cluster catches real bugs |
| Test environment parity with production (Talos/Cilium) | Prohibitively expensive in CI; kind tests operator logic that matters |
| Auto-generating tests from OpenAPI spec | No OpenAPI spec exists; manual tests are better tailored to business logic |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| INFRA-01 | Phase 15 | Complete |
| INFRA-02 | Phase 15 | Complete |
| INFRA-03 | Phase 13 | Complete |
| INFRA-04 | Phase 13 | Complete |
| GAPI-01 | Phase 13 | Complete |
| GAPI-02 | Phase 13 | Complete |
| GAPI-03 | Phase 13 | Complete |
| GAPI-04 | Phase 14 | Complete |
| PW-01 | Phase 16 | Complete |
| PW-02 | Phase 16 | Complete |
| PW-03 | Phase 16 | Complete |
| PW-04 | Phase 16 | Complete |
| PW-05 | Phase 16 | Complete |
| CI-01 | Phase 17 | Pending |
| CI-02 | Phase 17 | Pending |
| CI-03 | Phase 17 | Pending |
| CI-04 | Phase 17 | Pending |
| COV-01 | Phase 18 | Pending |
| COV-02 | Phase 18 | Pending |

**Coverage:**
- v1.1 requirements: 19 total
- Mapped to phases: 19
- Unmapped: 0

---
*Requirements defined: 2026-02-18*
*Last updated: 2026-02-18 after roadmap creation*
