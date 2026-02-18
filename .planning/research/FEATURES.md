# Feature Landscape: E2E CI/CD Test Suite

**Domain:** Testing infrastructure for Kubernetes-native web application (Go backend + React SPA + K8s operator)
**Researched:** 2026-02-17
**Confidence:** HIGH

## Current Test Coverage Audit

Before defining features, here is what already exists in the codebase:

| Test Layer | What Exists | Coverage Quality |
|------------|-------------|-----------------|
| **Controller unit tests** | Ginkgo/envtest suite for GameServerReconciler + DNSReconciler | Good: covers lifecycle states, Pod/Service/PVC creation, DNS routing |
| **API handler unit tests** | httptest-based tests for auth, gameserver CRUD, games, admin | Good: 7 files, table-driven, fake K8s client, JWT helpers |
| **E2E scaffold** | Kubebuilder default Ginkgo e2e (manager pod boot + metrics) | Minimal: only verifies controller-manager starts and serves metrics |
| **CI workflows** | `test.yml` (unit) and `test-e2e.yml` (e2e) on GitHub Actions | Basic: runs on push/PR, kind cluster setup, no Playwright, no frontend tests |
| **Untested handlers** | `handlers_console.go`, `handlers_mods.go`, `handlers_backups.go`, `handlers_metrics.go` | Zero coverage: WebSocket, file upload, S3 operations, proxy metrics |
| **Frontend tests** | None | Zero: no Playwright, no Vitest, no component tests |

**Key gap:** The existing e2e tests only verify the controller-manager boots. They do not exercise any user flows, API endpoints against a real cluster, or the React frontend at all. The API handler tests use fake K8s clients and httptest -- good for unit coverage but not integration validation.

## Table Stakes

Features users (developers, CI systems, reviewers) expect from a test suite of this nature. Missing these makes the suite feel incomplete or untrustworthy.

| Feature | Why Expected | Complexity | Dependencies on Existing Code |
|---------|--------------|------------|-------------------------------|
| **Kind cluster lifecycle management** | E2E tests for K8s operators need a real cluster; kind is the standard tool for CI | LOW | Extends existing `setup-test-e2e` / `cleanup-test-e2e` Makefile targets. Need to add Helm chart install, CRD deployment, image loading |
| **Go API integration tests with httptest** | Handler tests using fake clients miss real K8s behavior (watch, status subresource, finalizers) | MEDIUM | Builds on existing `helpers_test.go` patterns. Untested handlers: console, mods, backups, metrics need test files |
| **Playwright browser E2E tests** | React SPA has zero test coverage; login, server create/manage, and admin flows must be validated against a running backend | HIGH | Requires new `e2e/` directory in web or project root; depends on backend running in kind; needs npm/npx playwright install |
| **Authentication state fixtures** | Playwright tests that log in on every test are slow and brittle; storageState reuse is standard practice | LOW | Depends on Playwright setup; leverages existing `/api/v1/auth/login` endpoint. Need admin + user fixture states |
| **GitHub Actions CI pipeline** | PR checks that run the full suite give confidence to merge; existing workflows are too narrow | MEDIUM | Extends existing `test.yml` and `test-e2e.yml`; needs kind + Docker build + Helm install + Playwright install steps |
| **Test isolation and cleanup** | Each test must not leak state into others; namespaces, users, and game servers must be created/destroyed per test | LOW | Existing patterns in `helpers_test.go` (fresh `testServer` per test) are good; E2E needs analogous namespace cleanup |
| **Failure diagnostics collection** | When tests fail in CI, developers need logs, events, screenshots; existing AfterEach pattern collects k8s logs but needs extension | LOW | Existing `AfterEach` in `e2e_test.go` collects pod logs and events. Playwright adds screenshot/trace artifacts. GitHub Actions needs `upload-artifact` step |
| **Test tagging and selective execution** | Developers must run just unit, just API integration, or just e2e tests; not everything on every keystroke | LOW | Existing `//go:build e2e` tag pattern is correct. Need parallel `make test-api` target. Playwright has `--grep` and project filtering |
| **Makefile targets for all test tiers** | `make test`, `make test-api`, `make test-e2e`, `make test-playwright` -- developers expect one command per tier | LOW | Extends existing Makefile. `make test` already exists. Need new targets for integration and Playwright |
| **Coverage reporting** | Developers and PR reviewers need to see what percentage of code is tested; Go has built-in coverage; Playwright has istanbul | MEDIUM | Existing `make test` already writes `cover.out`. Need to merge with integration coverage. Playwright coverage is separate |

## Differentiators

Features that elevate this beyond a basic test setup. Not strictly required but significantly improve quality and developer experience.

| Feature | Value Proposition | Complexity | Dependencies on Existing Code |
|---------|-------------------|------------|-------------------------------|
| **Go binary coverage via `go build -cover`** | Measures actual code coverage when the operator runs E2E tests against a kind cluster, not just unit test coverage | MEDIUM | Go 1.20+ feature. Build binary with `-cover`, set `GOCOVERDIR`, merge with `go tool covdata`. Requires modifying Dockerfile or build step for test image |
| **Playwright visual regression baselines** | Catch unintended UI changes by comparing screenshots; especially valuable for admin panel and RJSF dynamic forms | MEDIUM | Needs `toHaveScreenshot()` baseline images committed to repo. Initial setup is per-page; maintenance is low after baselines exist |
| **Test backlog tracking system** | Markdown file or GitHub issues tracking which features/handlers lack tests, with priority and assignee. Creates accountability | LOW | Pure documentation; references existing handler files. Updates as coverage grows |
| **Playwright API + UI combined tests** | Playwright can make API calls to set up state, then verify UI reflects it. Faster than doing everything through the browser | LOW | Uses Playwright `request` context to call existing REST API (create server via API, verify dashboard via browser) |
| **Parallel test execution in CI** | Run Go tests, Playwright tests, and linting concurrently in GitHub Actions matrix; reduces total CI time from ~15min to ~8min | MEDIUM | Requires restructuring workflow into jobs with dependency graph. Kind cluster setup is shared overhead |
| **Helm chart validation tests** | Verify `helm template` renders correctly, `helm install --dry-run` succeeds, and chart values produce expected manifests | LOW | Depends on existing `chart/` directory. Uses `helm template` + snapshot comparison or OPA/conftest policy checks |
| **WebSocket console E2E test** | Verify the xterm.js console connects, receives log output, and survives reconnection. This is the most complex untested feature | HIGH | Depends on real GameServer pod running in kind (not just CRD). Playwright has WebSocket support but xterm testing is non-trivial |
| **Flake detection and retry strategy** | CI flakes erode trust in the suite; implement retry logic for known-flaky K8s operations (pod scheduling, DNS propagation) | LOW | Playwright has `retries` config. Go tests use `Eventually()` from Gomega already. Need consistent timeout/polling constants |

## Anti-Features

Features that seem valuable but would hurt the test suite's reliability, maintainability, or CI performance.

| Anti-Feature | Why It Seems Good | Why Avoid | What to Do Instead |
|--------------|-------------------|-----------|-------------------|
| **Running Playwright inside the kind cluster (Testkube)** | "Test where you deploy" -- Testkube markets this heavily | Adds massive infrastructure complexity, makes local development harder, debugging is painful in-cluster. Overkill for a single-project test suite | Run Playwright on the GitHub Actions runner or locally, pointing at the kind cluster's port-forwarded services |
| **Full browser matrix (Chrome + Firefox + Safari)** | "Test all browsers" -- standard advice | Game server panels are internal tools, not public websites. Multi-browser adds 3x CI time for near-zero user value. React + SPA means browser differences are minimal | Test Chromium only. Add Firefox/WebKit if users report browser-specific bugs |
| **E2E tests for every API endpoint** | "100% E2E coverage" -- sounds thorough | E2E tests are slow (kind + docker + network). Most API behavior is already well-tested with httptest unit tests. E2E should test integration paths, not re-test unit-tested logic | E2E tests cover happy-path user journeys (signup -> create server -> manage -> delete). Edge cases stay in httptest |
| **Mocking Kubernetes in Playwright tests** | "Faster tests without real cluster" -- MSW (Mock Service Worker) temptation | Defeats the purpose of E2E. The whole point is testing real K8s behavior. Mocked K8s hides real bugs (timing, RBAC, resource limits) | Use real kind cluster for E2E. Use httptest for fast API unit tests |
| **Cypress instead of Playwright** | "Cypress has more tutorials" | Playwright has surpassed Cypress in adoption (235% growth 2025), better multi-tab support, native API testing, faster execution, better CI integration. Cypress's dashboard paywall and slower execution are drawbacks | Use Playwright. It handles both API and browser testing in one framework |
| **Database seeding fixtures** | "Pre-populate test data" | Kterodactyl stores users in K8s Secrets and servers as CRDs -- there is no database. K8s fixtures are created via API calls. Database seeding patterns do not apply | Use API calls or kubectl to create test state. Playwright auth fixtures handle user state |
| **Component testing (Storybook/Playwright CT)** | "Test React components in isolation" | Adds build tooling complexity. The codebase uses shadcn/ui components that are thin wrappers. Value is in integration, not component isolation | If component tests are wanted later, use Vitest + React Testing Library. Not part of this E2E milestone |
| **Test environment parity with production** | "Match production exactly" -- Talos cluster, Cilium CNI, Gateway API controller | Production has Talos + Cilium + Cloudflare Tunnels. Reproducing this in CI is extremely expensive and fragile. Kind with default CNI tests the operator logic that matters | Use kind with standard configuration. Document any known behavioral differences between kind and Talos |
| **Auto-generating tests from OpenAPI spec** | "Generate test cases from API definition" | Kterodactyl does not have an OpenAPI spec. Creating one just for test generation inverts the dependency. The manually written tests are better tailored to business logic | Write tests manually. If an OpenAPI spec is needed later, generate it from code, not the other way around |

## Feature Dependencies

```
Kind Cluster Lifecycle (foundation)
    |-- requires --> Docker, kind binary, Helm chart, container image build
    |
    +-- enables --> Go E2E Tests (CRD/controller validation)
    |               Playwright E2E Tests (full-stack browser tests)
    |               Helm Chart Validation Tests

Go API Integration Tests (layer 2)
    |-- requires --> Existing httptest helpers, untested handler stubs
    |-- depends on -> Kind Cluster (for real-cluster integration variant)
    |
    +-- enables --> Coverage reporting, test backlog tracking

Playwright Setup (layer 2)
    |-- requires --> Node.js, Playwright browsers, playwright.config.ts
    |-- depends on -> Running backend (via kind or local process)
    |
    +-- enables --> Auth Fixtures (storageState)
    |               Browser E2E Tests (user flows)
    |               Visual Regression (screenshot baselines)

Auth Fixtures (layer 3)
    |-- requires --> Playwright setup, running /api/v1/auth/login endpoint
    |
    +-- enables --> All authenticated browser tests (server CRUD, admin, console)

GitHub Actions Pipeline (layer 3)
    |-- requires --> Kind lifecycle, Go tests, Playwright setup
    |
    +-- enables --> PR checks, failure artifact collection, parallel execution

Test Backlog (layer 4, can start anytime)
    |-- requires --> Nothing (pure documentation)
    |
    +-- enables --> Coverage gap tracking, prioritization
```

### Dependency Notes

- **Kind cluster is foundational**: Without a real cluster, neither Go E2E nor Playwright E2E tests can validate real K8s behavior. Must be first.
- **Go API integration tests can begin immediately**: Extending existing httptest patterns for untested handlers (mods, backups, console, metrics) does not require kind. Real-cluster variants do.
- **Playwright depends on a running backend**: Either port-forwarded from kind or run locally. The `webServer` config in Playwright can start the backend automatically.
- **Auth fixtures depend on Playwright setup**: Cannot create storageState files without Playwright installed and a backend to authenticate against.
- **GitHub Actions ties everything together**: The pipeline definition depends on all test tiers being defined so it can orchestrate them.

## MVP Recommendation

### Must Build (v1.1 Test Suite Core)

Prioritize these features. Without them, the test suite provides no real value over what already exists.

1. **Kind cluster lifecycle in Makefile** -- `make test-e2e-setup`, `make test-e2e-teardown` with Helm chart deploy, image load, CRD install. LOW complexity. Foundation for everything.
2. **Go API integration tests for untested handlers** -- Add test files for `handlers_mods.go`, `handlers_backups.go`, `handlers_metrics.go`. Skip `handlers_console.go` (WebSocket needs different approach). MEDIUM complexity. Fills biggest coverage gap using existing patterns.
3. **Playwright installation and configuration** -- `playwright.config.ts`, browser install, `webServer` config to launch backend. MEDIUM complexity. Unlocks all browser testing.
4. **Playwright auth fixtures** -- storageState for admin and regular user roles. LOW complexity once Playwright is set up. Required by all authenticated tests.
5. **Core Playwright user journey tests** -- Login, create server, view server list, manage server (start/stop), delete server, admin panel. HIGH complexity (most test code). This is the primary deliverable.
6. **GitHub Actions pipeline** -- Single workflow that runs: lint -> unit tests -> build image -> kind setup -> Go e2e -> Playwright e2e -> teardown. MEDIUM complexity. Makes the suite actually useful on PRs.
7. **Test backlog tracking** -- Markdown file listing all untested features with priority. LOW complexity. Creates roadmap for future test coverage.

### Defer (Future Test Enhancements)

- **Go binary coverage (`go build -cover`)**: Valuable but adds build complexity. Do after core suite works.
- **Visual regression baselines**: Requires stable UI. Add after Playwright tests are green.
- **WebSocket console E2E test**: Requires real GameServer pod running (image pull, startup time). Complex and slow. Add after core flows work.
- **Parallel CI execution**: Optimize after the serial pipeline proves reliable.
- **Helm chart validation tests**: Nice but low priority. The chart already works in production.

## Complexity Budget

Estimated effort for the MVP test features, based on codebase analysis:

| Feature | Files to Create/Modify | Estimated LOC | Risk |
|---------|----------------------|---------------|------|
| Kind lifecycle | Makefile + kind-config.yaml + test scripts | ~150 | LOW: well-understood pattern |
| Go API integration tests | 4 new test files + helpers | ~600 | LOW: follows existing `helpers_test.go` pattern |
| Playwright setup | playwright.config.ts + package.json + fixtures | ~200 | MEDIUM: first-time setup, browser install in CI |
| Playwright auth fixtures | auth.setup.ts + 2 storageState files | ~100 | LOW: well-documented Playwright pattern |
| Playwright E2E tests | 5-7 test spec files | ~800 | HIGH: most code, depends on real UI behavior |
| GitHub Actions pipeline | 1-2 workflow YAML files | ~200 | MEDIUM: complex orchestration, debugging CI is slow |
| Test backlog | 1 markdown file | ~100 | LOW: documentation only |
| **Total** | **~15-20 files** | **~2,150** | |

## Sources

### Playwright
- [Playwright Official Docs - Authentication](https://playwright.dev/docs/auth) -- storageState pattern for login reuse
- [Playwright CI Integration](https://playwright.dev/docs/ci) -- GitHub Actions setup
- [Playwright Web Server Config](https://playwright.dev/docs/test-webserver) -- auto-start backend before tests
- [Playwright API Testing](https://playwright.dev/docs/api-testing) -- combined API + browser testing

### Kubernetes E2E Testing
- [Kubebuilder E2E Testing Docs](https://github.com/kubernetes-sigs/kubebuilder/blob/master/docs/testing/e2e.md) -- scaffolded e2e patterns
- [Running K8s E2E Tests with Kind and GitHub Actions](https://radu-matei.com/blog/kubernetes-e2e-github-actions/) -- kind + GH Actions setup
- [Testing Kubernetes Operators with GitHub Actions and Kind](https://medium.com/codex/testing-kubernetes-operators-using-github-actions-and-kind-c4086d37dd30) -- operator-specific CI patterns
- [Kubebuilder Issue #5155](https://github.com/kubernetes-sigs/kubebuilder/issues/5155) -- replacing kubectl calls with controller-runtime client in e2e

### Go Coverage
- [Go Official: Coverage for Integration Tests](https://go.dev/doc/build-cover) -- `go build -cover` documentation
- [Go Blog: Integration Test Coverage](https://go.dev/blog/integration-test-coverage) -- `GOCOVERDIR` and `go tool covdata` workflow
- [Go 1.20 Coverage for K8s Apps](https://www.mgasch.com/2023/02/go-e2e/) -- practical guide for K8s operator coverage

### Testing Best Practices
- [BrowserStack: Playwright Best Practices 2026](https://www.browserstack.com/guide/playwright-best-practices) -- role-based locators, test isolation
- [Microsoft Engineering Playbook: E2E Testing](https://microsoft.github.io/code-with-engineering-playbook/automated-testing/e2e-testing/) -- testing pyramid, CI integration
- [EnvTest Practical Guide](https://blog.marcnuri.com/go-testing-kubernetes-applications-envtest) -- Go K8s testing with envtest

---
*Feature research for: Kterodactyl v1.1 E2E CI/CD Test Suite*
*Researched: 2026-02-17*
*Researcher: GSD Project Researcher Agent*
