# Project Research Summary

**Project:** Kterodactyl v1.1 — End-to-End CI/CD Test Suite
**Domain:** Testing infrastructure for a Kubernetes-native game server operator (Go operator + React SPA)
**Researched:** 2026-02-17
**Confidence:** HIGH

## Executive Summary

Kterodactyl v1.0 shipped with a partially-functional but under-exercised test foundation: controller unit tests (Ginkgo/envtest), API handler unit tests (httptest with fake K8s client), a Kubebuilder-scaffolded E2E suite that only verifies the manager pod boots, and three GitHub Actions workflows with no Playwright, no frontend tests, and no real end-to-end coverage. The v1.1 milestone must close this gap with a layered test suite: Go API integration tests (extending existing httptest patterns into multi-step blackbox flows), a Playwright E2E suite (browser tests against a live Go + K8s backend deployed into kind), and a unified GitHub Actions CI pipeline that runs all layers in dependency order. No new production code is needed — all work is additive test infrastructure.

The recommended approach is to build the suite in five dependency-ordered phases: fix the Go test infrastructure first (live K8s client vs. cached client is a critical correctness issue in the existing envtest suite), then add Go API integration tests using the proven `testServer` pattern, then stand up the kind cluster with Helm-based deployment and NodePort access, then write Playwright E2E tests against the running app, and finally consolidate everything into a unified CI pipeline. Playwright lives in a top-level `e2e/` directory (not inside `web/`) with its own `package.json`, points at `http://localhost:8080` exposed by kind's `extraPortMappings`, and runs Chromium-only with `workers: 1` in CI. No new Go dependencies are needed — the only new package additions are `@playwright/test@^1.58` and a pinned kind v0.31.0 binary.

The single highest-severity risk is a cascading flakiness trap: the existing envtest suite uses the manager's cached K8s client for assertions, which causes intermittent stale-read failures that get masked with `time.Sleep` and eventually erode CI trust entirely. This must be fixed before writing any new tests. The second-highest risk is Playwright tests timing out on Kubernetes state transitions — the controller reconciliation loop is not instant, and Playwright's default 30-second timeout fires before pod scheduling completes. The mitigation is API-level polling via `toPass()` with explicit per-operation timeouts, not global timeout increases. Both risks are well-understood and preventable with documented patterns.

## Key Findings

### Recommended Stack

The project already has everything needed for Go testing — no new Go dependencies are required. The only new Go test tooling is the pattern of using `client.New(cfg, client.Options{Scheme: scheme})` (a live client, reads from API server) instead of `k8sManager.GetClient()` (a cached informer-backed client) for assertions in the envtest suite. For Playwright, the only addition is `@playwright/test@^1.58` in a new `e2e/` directory. Kind v0.31.0 is already referenced in the Makefile; it needs to be version-pinned (not `latest`) and configured with `kindest/node:v1.32.11` to match the production Talos cluster's K8s v1.32.x minor version. The new CI workflow uses action versions already in use elsewhere in the repo (`checkout@v4`, `setup-go@v5`, `upload-artifact@v4`) plus one new action: `setup-node@v5`.

**Core technologies:**
- `@playwright/test@^1.58`: Browser E2E test runner — industry standard, better than Cypress for CI (fully OSS, better auto-wait, native multi-browser, faster execution, no parallel paywall)
- `kind@v0.31.0` + `kindest/node:v1.32.11`: K8s test cluster — already scaffolded by Kubebuilder, pinned to match production K8s v1.32.x minor version
- `net/http/httptest` (stdlib): Go API integration tests — already proven in the codebase, zero new dependency
- `controller-runtime/pkg/client/fake` (existing v0.23.1): Fake K8s client for unit-scope API tests — already in use
- `actions/setup-node@v5`: Node.js setup for Playwright in CI — the only new GitHub Actions action
- Do NOT add: Cypress, testify, Vitest, k3d, Allure, Docker Compose, Selenium, or testcontainers-go

### Expected Features

**Must have (table stakes for v1.1):**
- Kind cluster lifecycle management with Helm chart deploy, image loading, and CRD installation — the foundation everything else depends on
- Go API integration tests for currently untested handlers (`handlers_mods.go`, `handlers_backups.go`, `handlers_metrics.go`) using existing `testServer` patterns — fills biggest coverage gap with zero new infrastructure
- Playwright installation and configuration (`playwright.config.ts`, Chromium browser install, `baseURL` pointing at kind-exposed app) — unlocks all browser testing
- Playwright auth fixtures using `storageState` for admin and regular-user roles — required by all authenticated tests, prevents per-test login overhead
- Core Playwright user journey tests: login, create server, view server list, start/stop server, delete server, admin panel — the primary deliverable of v1.1
- GitHub Actions unified pipeline: `lint -> unit-test -> integration-test -> e2e-test` with job `needs` dependencies — makes the suite actually enforce quality on PRs
- Test backlog tracking: a markdown file listing all untested features with priority for future sprints

**Should have (differentiators, defer if scope is tight):**
- Go binary coverage via `go build -cover` + `GOCOVERDIR` to measure real integration coverage, not just unit coverage
- Playwright visual regression baselines via `toHaveScreenshot()` — valuable for admin panel stability but requires stable UI first
- Parallel CI job execution (matrix strategy) to reduce pipeline from ~15min to ~8min
- Helm chart validation tests (`helm template` + dry-run)

**Defer to post-v1.1:**
- WebSocket console E2E test — inherently flaky due to Pod exec + xterm.js rendering timing; needs mock echo server strategy and dedicated design
- Full multi-browser matrix (Firefox, WebKit) — Chromium alone covers the user base at this project scale
- Testkube or in-cluster test execution — overkill complexity for a single-project suite

### Architecture Approach

The test suite adds three new layers to the existing codebase without modifying any production code. Layer 1 (unit + controller) already exists via `internal/api/*_test.go` and `internal/controller/*_test.go` with envtest. Layer 2 (API integration) is new: a `test/integration/` Go package using `httptest.NewServer` (real TCP round-trips, not just a recorder) for multi-step blackbox flows against a fake K8s client. Layer 3 (E2E) replaces the scaffold-only Ginkgo suite with a combined approach: Ginkgo verifies the operator (pod running, CRDs, metrics), and Playwright in `e2e/` verifies user experience (browser flows against the full stack deployed into kind via Helm). The key infrastructure decision is NodePort + `kind extraPortMappings` (containerPort 30080 -> hostPort 8080) over `kubectl port-forward`, which is fragile in long-running CI jobs.

**Major components:**
1. `test/integration/` — New Go httptest-based API integration tests (blackbox, real TCP, fake K8s client, multi-step flows)
2. `e2e/` — New Playwright TypeScript test suite (Chromium, `baseURL: http://localhost:8080`, Helm-deployed app in kind)
3. `hack/kind-config.yaml` + `hack/ci-values.yaml` + `hack/wait-for-ready.sh` — CI environment scaffolding for kind cluster setup and readiness gating
4. `.github/workflows/ci.yml` — Unified pipeline replacing the three separate workflow files (consolidate, not accumulate)
5. `test/e2e/` — Existing Ginkgo suite updated: switch from kustomize to Helm deploy, add operator-level test cases
6. `chart/templates/service.yaml` — Minor chart modification to support conditional `nodePort` field (backward-compatible)

### Critical Pitfalls

1. **EnvTest cached client causes flaky assertions** — The existing `suite_test.go` likely uses `k8sManager.GetClient()` (informer-cached, not live). Replace with `client.New(cfg, client.Options{Scheme: scheme})` before writing any new controller tests. Always use `Eventually` + re-fetch for all post-mutation assertions, never bare `Expect` after Create/Update/Delete.

2. **Playwright timing out on Kubernetes state transitions** — Default 30-second Playwright timeout fires before K8s pod scheduling and reconciliation completes. Use `toPass({ timeout: 90_000, intervals: [2_000] })` with API-level polling, set per-test `test.setTimeout(120_000)` for K8s operations, and pre-pull any game images into kind to eliminate image-pull latency from CI timing.

3. **GitHub Actions disk space exhaustion** — Docker images (Go builder, Node.js, kind node image, operator image) plus containerd storage can exhaust the ~22GB runner limit. Add a disk cleanup step (`sudo rm -rf /opt/hostedtoolcache /usr/local/lib/android /usr/share/dotnet && docker system prune -af`) before kind cluster creation.

4. **Kind cluster not cleaned up after CI failures** — Kind cluster cleanup only in the happy path means self-hosted runners and reruns get "cluster already exists" errors. Use `if: always()` on the cleanup step and a run-ID-prefixed cluster name (`kterodactyl-e2e-${{ github.run_id }}`).

5. **Brittle Playwright selectors coupled to React component internals** — Tests using CSS class selectors or `:nth-child` break on every UI refactor. Add `data-testid` attributes to all interactive and state-indicating React elements before writing any Playwright tests. Convention: `data-testid="page-{name}"`, `data-testid="{entity}-{action}-btn"`, `data-testid="{entity}-status"`.

## Implications for Roadmap

Based on research, the natural phase structure follows the dependency graph identified in FEATURES.md: each layer enables the next, and pitfalls in earlier layers corrupt later ones if not addressed first. This is infrastructure work where ordering matters more than in feature development.

### Phase 1: Go Test Infrastructure Hardening

**Rationale:** The existing envtest suite has a known-bad pattern (cached client assertions) that will cause flaky failures in any new tests built on top of it. Fix the foundation before building on it. This phase has no external dependencies and unblocks all other Go testing work.
**Delivers:** Reliable controller test suite with live client assertions; unique namespace isolation using random suffixes; correct `Eventually` + re-fetch patterns on all post-mutation assertions; explicit `go test -timeout 20m` in Makefile. Also adds test files for currently untested API handlers (`mods`, `backups`, `metrics`) using the existing `testServer` patterns without requiring any new infrastructure.
**Addresses:** Go API integration tests for untested handlers, test isolation and cleanup, test tagging and selective execution via `//go:build e2e` tags.
**Avoids:** Pitfall 1 (cached client), Pitfall 2 (namespace contamination), Pitfall 12 (test timeout defaults kill long-running controller tests).
**Research flag:** Standard patterns — documented Kubebuilder practice, no additional research needed.

### Phase 2: Go API Integration Tests

**Rationale:** Integration tests (`test/integration/` package, blackbox, real TCP via `httptest.NewServer`) fill the coverage gap between unit tests (single handler, fake client recorder) and full E2E (full stack in kind). They run fast with no K8s cluster, cover multi-step flows, and validate the full API contract. Building this before kind setup keeps the feedback loop tight and catches regressions cheaply.
**Delivers:** Multi-step API lifecycle tests (register -> login -> create server -> list -> start -> stop -> delete) covering state transitions and cross-handler consistency. A `test/integration/helpers_test.go` mirroring the existing `internal/api/helpers_test.go` pattern but in a separate blackbox package using `httptest.NewServer` (real TCP). Naming convention `api-test-*` established to prevent data collision with other test layers.
**Uses:** `net/http/httptest` stdlib, `controller-runtime/pkg/client/fake`, existing `api.NewServer()` and `Server.HTTPServer()` constructors — zero new dependencies.
**Implements:** `test/integration/` component. The fake vs. real client coverage gap is acknowledged and explicitly documented rather than silently accepted — critical paths are validated in E2E (Phase 4), unit paths stay with the fake client.
**Avoids:** Pitfall 9 (fake/real client gap — explicitly documented, not silently accepted), Pitfall 14 (test data naming collisions — `api-test-*` prefix convention).
**Research flag:** Standard patterns — direct extension of existing codebase patterns, no additional research needed.

### Phase 3: Kind Cluster E2E Environment

**Rationale:** The kind cluster is the prerequisite for both Playwright tests and the updated Ginkgo operator E2E tests. Setting up the infrastructure correctly — port mappings, image loading strategy, Helm values override, and a readiness wait script — before writing any E2E test code prevents having to retrofit infrastructure decisions into a working suite.
**Delivers:** `hack/kind-config.yaml` (NodePort + extraPortMappings: containerPort 30080 -> hostPort 8080), `hack/ci-values.yaml` (Helm CI overrides: `image.pullPolicy: Never`, `apiService.type: NodePort`, Gateway API disabled), `hack/wait-for-ready.sh` (readiness gate using `kubectl wait` then curl loop), updated Ginkgo suite switching from kustomize to Helm install, Makefile targets (`test-e2e-full`), and the minor `chart/templates/service.yaml` modification to support conditional `nodePort`.
**Avoids:** Pitfall 3 (image loading timeout — plan the image caching strategy; local registry is the fallback if `kind load` proves too slow), Pitfall 5 (disk space — add cleanup step to the workflow from day one), Pitfall 7 (cluster not cleaned up — `if: always()` and run-ID cluster names from the start), Pitfall 13 (Docker Hub rate limits — use stub/lightweight game images in E2E tests).
**Research flag:** Medium complexity — NodePort + extraPortMappings is well-documented; if image loading proves too slow in initial CI testing, a local Docker registry spike (~2h) is the known alternative. Decide after measuring actual build times.

### Phase 4: Playwright E2E Tests

**Rationale:** With kind running and the auth infrastructure designed, Playwright tests can be written against real behavior. This is the highest-LOC phase (~800 lines) and the primary deliverable of v1.1. Auth fixtures must come first because every authenticated test spec depends on them.
**Delivers:** `e2e/` directory with `playwright.config.ts` (Chromium-only, `workers: 1`, `retries: 2`, `baseURL: http://localhost:8080`, no `webServer` config — app runs in kind), `e2e/package.json`, auth fixtures (`e2e/fixtures/auth.fixture.ts` using API calls + `localStorage` injection to set zustand auth state, `storageState` for admin and regular-user roles), `e2e/helpers/api-client.ts` (direct API calls for test setup bypassing UI), and test specs: `auth.spec.ts`, `gameserver-crud.spec.ts`, `admin.spec.ts`, `health.spec.ts`. Screenshot and trace artifacts uploaded on failure via `upload-artifact@v4`.
**Addresses:** Core Playwright user journey tests (primary deliverable), auth fixtures, failure diagnostics (screenshot/trace on failure).
**Avoids:** Pitfall 4 (K8s state wait — `toPass()` API polling, `test.setTimeout(120_000)` for K8s operations), Pitfall 6 (auth state not shared — `storageState` global setup, not per-test login), Pitfall 8 (WebSocket console — deferred to post-v1.1 with mock echo server design), Pitfall 10 (brittle selectors — `data-testid` retrofit to React components before writing tests), Pitfall 11 (CI workers — hardcode `workers: 1` in CI).
**Research flag:** The `data-testid` retrofit to existing `web/src/` React components is implementation scoping work that must happen before test writing begins. The auth fixture's `localStorage` injection strategy should be confirmed against `web/src/store/` to verify the zustand storage key before implementation.

### Phase 5: Unified GitHub Actions CI Pipeline

**Rationale:** Wire all test layers into a single `ci.yml` with explicit job dependencies. Consolidating the three existing workflow files (`test.yml`, `test-e2e.yml`, `lint.yml`) plus the new Playwright workflow into one pipeline with `needs` ordering prevents the current problem of expensive E2E tests running even when lint fails, and makes CI status unambiguous.
**Delivers:** `.github/workflows/ci.yml` with jobs: `lint` (Go + frontend) -> `test-unit` (`make test`) -> `test-integration` (`go test ./test/integration/`) -> `test-e2e` (kind cluster + Helm install + Playwright). Disk cleanup step before kind. `if: always()` kind cluster teardown. `timeout-minutes: 30` on E2E job. Docker layer caching via GHA cache. `upload-artifact@v4` for Playwright report on failure. Run-ID-prefixed cluster name.
**Addresses:** GitHub Actions pipeline (table stakes), failure diagnostics collection, test tagging via job-level re-run.
**Avoids:** Pitfall 5 (disk space — cleanup step as first step in E2E job), Pitfall 7 (cluster cleanup — `if: always()`, run-ID cluster name), Pitfall 13 (Docker Hub rate limits — stub images in E2E, or add Docker Hub auth secrets).
**Research flag:** Standard GitHub Actions patterns — all action versions already in use in the repo, no additional research needed.

### Phase 6: Test Backlog and Coverage Reporting

**Rationale:** With the suite running, document what is and is not covered. This creates explicit accountability for post-v1.1 work and ensures the coverage gaps (WebSocket console, visual regression, file upload, S3 operations) are tracked rather than forgotten. Low effort, high communication value.
**Delivers:** `TEST_BACKLOG.md` listing all untested features by priority (WebSocket console, file upload via `handlers_console.go`, S3 backup operations, visual regression baselines, multi-browser matrix, Go binary coverage via `-cover`). Go coverage report integration: `make test` already produces `cover.out`; this phase merges it with integration test coverage using `go tool covdata`.
**Addresses:** Test backlog tracking and coverage reporting (both table stakes features from FEATURES.md MVP list).
**Research flag:** Pure documentation plus minor CI config — no research needed.

### Phase Ordering Rationale

- Phases 1-2 run entirely without K8s infrastructure — they validate the Go test foundation and provide a fast feedback loop during development. The cached-client bug fix in Phase 1 must precede all other test writing to prevent propagating a known-bad pattern.
- Phase 3 is the hard prerequisite for both Phase 4 (Playwright) and the updated Ginkgo operator E2E — the kind cluster and Helm deployment pipeline must be validated before any test code that depends on them.
- Phase 4 is the highest-value deliverable but also the most dependent: kind cluster running (Phase 3), `data-testid` attributes added to React components, and auth fixtures working. Writing Playwright tests before the infrastructure is solid leads to debugging environment issues, not test bugs.
- Phase 5 ties everything together. Doing it last means the pipeline definition reflects the final test architecture rather than being retrofitted as each phase completes — attempting a unified CI pipeline before the individual test commands are stable creates a debugging nightmare.
- Phase 6 is intentionally last — you cannot accurately document what is and isn't tested until the test suite exists.

### Research Flags

Phases needing deeper research during planning:
- **Phase 3 (Kind cluster):** If `kind load docker-image` proves too slow during initial CI testing (it can add 3-5 minutes for large images), a local Docker registry approach is the known mitigation. This is a ~2h spike, not a research question — the architecture file documents both options and the decision point is measured build time.
- **Phase 4 (Playwright):** The auth fixture's `localStorage` injection should be verified against `web/src/store/` before committing to that approach. If the app uses HttpOnly cookies rather than localStorage for JWT storage, the fixture strategy changes. This is a 30-minute code read, not a research question.

Phases with well-documented standard patterns (skip additional research):
- **Phase 1:** Documented Kubebuilder patterns for live vs. cached client; directly observable in existing `suite_test.go`.
- **Phase 2:** Direct extension of existing `internal/api/helpers_test.go` — same patterns, different package boundary.
- **Phase 5:** Standard GitHub Actions job dependency patterns; all action versions already in active use in this repo.
- **Phase 6:** Pure documentation and minor CI config additions.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All technologies are version-pinned and verified against the existing `go.mod`, `package.json`, workflow files, and Makefile. No speculative choices — every tool either already exists in the project or is the documented standard for its category. |
| Features | HIGH | Feature set is derived from direct codebase audit (what files exist, what has test coverage, what does not) plus domain best practices for testing K8s operators. The coverage gap analysis is based on actual file inspection, not inference. |
| Architecture | HIGH | Core patterns (httptest integration tests, NodePort + kind extraPortMappings, Playwright against deployed app rather than dev server) are documented and well-established. The `HTTPServer().Handler` integration point is verified against the actual `internal/api/server.go` structure. The anti-pattern of running Playwright's `webServer` with the Go binary is correctly ruled out — the binary requires a K8s cluster to start. |
| Pitfalls | HIGH | Top pitfalls are verified against official Kubebuilder documentation, kind GitHub issues with reproduction cases, and Playwright documentation. The cached-client flakiness pitfall is directly observable in the existing `suite_test.go` scaffold. |

**Overall confidence:** HIGH

### Gaps to Address

- **Auth mechanism confirmation:** Research documents that Kterodactyl uses JWT stored in `localStorage` via zustand persist middleware (`auth-storage` key). This should be verified against `web/src/store/` before implementing the Playwright auth fixture. If the storage key or JSON structure differs from what is documented, the fixture will silently fail to authenticate without obvious error output.
- **Existing test flakiness baseline:** The cached-client issue is inferred from the Kubebuilder scaffold pattern. Running `ginkgo --until-it-fails` against the existing controller suite before Phase 1 begins will confirm whether flakiness is actively occurring and establish a baseline success rate.
- **Docker image size measurement:** The PITFALLS research notes the operator image may be 200MB+ uncompressed. Actual size should be measured with `docker image inspect` before choosing between `kind load docker-image` (simpler) and a local registry (faster but more setup). Don't add the local registry complexity unless the measured build time requires it.
- **Helm chart `nodePort` field:** ARCHITECTURE.md recommends a conditional `nodePort` field in `chart/templates/service.yaml`. This is a minor production chart change — verify backward compatibility (when `nodePort` is not set, Kubernetes assigns one automatically) before Phase 3 begins.

## Sources

### Primary (HIGH confidence)
- [Playwright official docs](https://playwright.dev/docs/intro) — installation, CI setup, auth/storageState, auto-waiting, webServer config
- [kind official docs + v0.31.0 release](https://kind.sigs.k8s.io/docs/user/quick-start/) — cluster config, extraPortMappings, image loading
- [Kubebuilder Book: Writing Tests](https://book.kubebuilder.io/cronjob-tutorial/writing-tests) — envtest patterns, live vs. cached client
- [Kubebuilder Book: EnvTest Reference](https://book.kubebuilder.io/reference/envtest) — namespace deletion limitation in envtest
- [Operator SDK testing docs](https://sdk.operatorframework.io/docs/building-operators/golang/testing/) — fake vs. real client coverage gap
- [Go Wiki: Table-Driven Tests](https://go.dev/wiki/TableDrivenTests) — Go testing conventions
- [Go 1.20+ Coverage docs](https://go.dev/doc/build-cover) — `go build -cover` and `GOCOVERDIR`
- [GitHub Actions action repos](https://github.com/actions/) — checkout@v4, setup-go@v5, setup-node@v5, upload-artifact@v4
- [@playwright/test npm](https://www.npmjs.com/package/@playwright/test) — v1.58.2 latest as of Feb 2026

### Secondary (MEDIUM confidence)
- [Testing Kubernetes Operators with GitHub Actions and Kind - Medium/CodeX](https://medium.com/codex/testing-kubernetes-operators-using-github-actions-and-kind-c4086d37dd30) — kind + GH Actions CI patterns
- [InfraCloud EnvTest Guide](https://www.infracloud.io/blogs/testing-kubernetes-operator-envtest/) — envtest test client best practices
- [SuperOrbital: Testing Production Controllers](https://superorbital.io/blog/testing-production-controllers/) — live vs. cached client decision rationale
- [Semaphore: Flaky Playwright Tests](https://semaphore.io/blog/flaky-tests-playwright) — flake prevention strategies
- [BrowserStack: Playwright Best Practices 2026](https://www.browserstack.com/guide/playwright-best-practices) — role-based locators, test isolation
- [Elaichenkov: 17 Playwright Mistakes](https://elaichenkov.github.io/posts/17-playwright-testing-mistakes-you-should-avoid/) — brittle selector patterns to avoid
- [Gerald on IT: GitHub Actions Disk Cleanup](https://www.geraldonit.com/mastering-disk-space-on-github-actions-runners-a-deep-dive-into-cleanup-strategies-for-x64-and-arm64-runners/) — disk space management strategies
- [Go Blog: Integration Test Coverage](https://go.dev/blog/integration-test-coverage) — `GOCOVERDIR` and `go tool covdata` workflow

### Tertiary (MEDIUM-LOW confidence)
- [iximiuz: kind load docker-image deep dive](https://iximiuz.com/en/posts/kubernetes-kind-load-docker-image/) — image loading performance analysis
- [kind issue #3002](https://github.com/kubernetes-sigs/kind/issues/3002) — `kind load` slow performance reports and alternatives
- [Exposing NodePort in kind cluster](https://scriptcrunch.com/expose-nodeport-kind-cluster/) — NodePort access pattern (community blog; cross-verified against kind official docs)
- [GitHub community: disk space on runners](https://github.com/orgs/community/discussions/25678) — disk exhaustion reports and cleanup approaches

---
*Research completed: 2026-02-17*
*Ready for roadmap: yes*
