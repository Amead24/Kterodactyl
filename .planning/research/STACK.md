# Technology Stack: v1.1 Testing & CI

**Project:** Kterodactyl
**Milestone:** v1.1 Testing & CI
**Researched:** 2026-02-17
**Overall confidence:** HIGH

## Context

v1.0 shipped with zero automated tests. This stack research covers ONLY the additions needed for:
1. Playwright E2E tests (browser-based happy path flows)
2. Go API integration tests (httptest-based, already partially implemented)
3. kind cluster test environment (already scaffolded by Kubebuilder)
4. GitHub Actions CI pipeline (skeleton workflows already exist)

The project already has Go 1.25.3, controller-runtime v0.23.1, Ginkgo v2/Gomega for operator E2E, a `web/` directory with Vite 7.3 + React 19, and three GitHub Actions workflow files. This research builds on what exists.

---

## Recommended Stack Additions

### E2E Testing (Frontend)

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| **@playwright/test** | ^1.58 | Browser E2E test runner | Industry standard for E2E testing. Faster than Cypress, native multi-browser support (Chromium, Firefox, WebKit), built-in auto-wait eliminates flaky selectors, first-class TypeScript support. The project already uses TypeScript in `web/`. Playwright's `webServer` config can launch the Go binary before tests, making it perfect for testing the embedded SPA. |
| **Chromium (via Playwright)** | bundled | Browser engine for CI | Run Chromium only in CI (not full multi-browser) to keep pipeline fast. Chromium covers the vast majority of users. Add Firefox/WebKit later if needed. |

### Go Testing (API Integration)

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| **net/http/httptest** | stdlib | HTTP test server | Already in use in `internal/api/helpers_test.go`. The project has a well-structured `testServer` pattern with fake K8s client, JWT service, and manifest loader. No new dependency needed -- extend the existing pattern. |
| **testing** | stdlib | Test framework | Project already uses stdlib `testing` with table-driven tests for API handlers. Consistent with existing patterns. Do NOT add testify -- the existing tests use raw `t.Errorf`/`t.Fatalf` and adding testify mid-project creates inconsistency for no real benefit. |
| **controller-runtime/pkg/client/fake** | v0.23.1 (existing) | Fake K8s client | Already used in `helpers_test.go` for mocking Kubernetes API. Provides typed, scheme-aware fake client. No additional dependency. |

### Kubernetes Test Environment

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| **kind** | v0.31.0 | Local K8s cluster in Docker | Already referenced in Makefile and existing E2E test suite. v0.31.0 is the latest release. Default node image is `kindest/node:v1.35.0`. For testing the operator against the same K8s version as production (v1.32), use `kindest/node:v1.32.11@sha256:5fc52d52a7b9574015299724bd68f183702956aa4a2116ae75a63cb574b35af8`. |
| **kindest/node** | v1.32.11 | K8s node image matching production | The production Talos cluster runs K8s v1.32.3. Testing against v1.32.x ensures API compatibility. Pin to SHA256 digest for reproducibility. |
| **envtest** | (existing, via controller-runtime) | Lightweight K8s API for unit tests | Already configured in Makefile for `make test`. Uses `setup-envtest` to download API server binaries. No changes needed. |

### CI Pipeline (GitHub Actions)

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| **actions/checkout** | v4 | Repository checkout | Already in use across all three workflow files. v4 is stable and sufficient. v5/v6 exist but require runner v2.327.1+; staying on v4 avoids runner compatibility issues with ubuntu-latest. |
| **actions/setup-go** | v5 | Go toolchain setup | Already in use. Uses `go-version-file: go.mod` which correctly reads Go 1.25.3. v5 is stable. |
| **actions/setup-node** | v5 | Node.js for Playwright | Needed for the new Playwright workflow. Use `node-version-file` pointing to a `.node-version` file or hardcode `node-version: 22` to match the Dockerfile's `node:22-alpine`. |
| **actions/upload-artifact** | v4 | Playwright report artifacts | Upload HTML test reports and trace files on failure. v4 is current and compatible with existing runner versions. |
| **golangci/golangci-lint-action** | v8 | Go linting | Already in use at v8 with golangci-lint v2.7.2. No changes needed. Note: v9 exists but requires runner v2.327.1+; v8 works with current ubuntu-latest. |

### Supporting Tools (Development)

| Tool | Version | Purpose | Why |
|------|---------|---------|-----|
| **kind** (CLI) | v0.31.0 | Local cluster management | Install via `go install sigs.k8s.io/kind@v0.31.0` or download binary. Already referenced as `KIND` variable in Makefile. |
| **npx playwright install** | (via @playwright/test) | Browser binary management | Playwright bundles browser downloads. Run `npx playwright install --with-deps chromium` to install only Chromium + system deps. |

---

## What NOT to Add

| Technology | Why Skip |
|------------|----------|
| **Cypress** | Playwright is faster, has better auto-wait, native multi-browser, and better CI integration. Cypress has a commercial license for parallel execution. Playwright is fully open source. |
| **testcontainers-go** | The project already has kind for full cluster tests and envtest for lightweight API server tests. testcontainers adds complexity without benefit -- the test matrix is "fake client OR real cluster," not "individual containers." |
| **testify** | Existing API tests use stdlib `testing` with `t.Errorf`. Mixing assertion libraries creates inconsistency. The project already has Ginkgo/Gomega for operator E2E tests (Kubebuilder convention). Adding a third style is noise. |
| **Vitest** | The frontend is an embedded SPA tested via Playwright E2E flows against the real Go backend. Component-level React tests add maintenance burden with low value for a panel UI. If component tests become needed later, Vitest is the right choice, but not for v1.1. |
| **Docker Compose** | kind provides a full K8s cluster. Docker Compose would only test the API in isolation, which httptest already covers. The whole point of E2E is testing the deployed system. |
| **Selenium** | Legacy. Playwright supersedes it in every dimension: speed, API design, reliability, maintenance. |
| **k3s/k3d** | kind is already integrated, Kubebuilder-scaffolded, and standard for CI. Switching to k3d gains nothing and loses the existing Kubebuilder integration. |
| **Allure Reports** | Playwright's built-in HTML reporter is sufficient. Allure adds Java dependency and complexity for marginal benefit at this scale. |

---

## Integration Points with Existing Build System

### Makefile Additions Needed

```makefile
# Playwright E2E tests (new)
PLAYWRIGHT_DIR ?= e2e

.PHONY: test-playwright
test-playwright: build  ## Run Playwright E2E tests against built binary
    cd $(PLAYWRIGHT_DIR) && npx playwright test

.PHONY: test-playwright-ui
test-playwright-ui:  ## Run Playwright tests with interactive UI
    cd $(PLAYWRIGHT_DIR) && npx playwright test --ui
```

### Existing Makefile Targets (No Changes Needed)

| Target | What It Does | Status |
|--------|-------------|--------|
| `make test` | Runs Go unit/integration tests with envtest | Already works. Excludes `/e2e` directory. |
| `make test-e2e` | Runs Ginkgo operator E2E tests against kind | Already scaffolded. Needs custom test cases. |
| `make setup-test-e2e` | Creates kind cluster if not exists | Already works. |
| `make cleanup-test-e2e` | Deletes kind cluster | Already works. |
| `make lint` | Runs golangci-lint | Already works. |
| `make docker-build` | Builds container image | Already works. Used by E2E suite to load image into kind. |

### File Structure for New Testing Code

```
kterodactyl/
  e2e/                          # NEW: Playwright E2E tests
    package.json                # Separate from web/ -- test-only deps
    playwright.config.ts        # Playwright configuration
    tests/
      auth.spec.ts              # Login/register flows
      server-management.spec.ts # Create/start/stop server flows
      admin.spec.ts             # Admin panel flows
    fixtures/
      auth.ts                   # Shared authentication helpers
  internal/api/
    handlers_*_test.go          # EXISTING: Extend with more test cases
    helpers_test.go             # EXISTING: testServer infrastructure
  test/e2e/                     # EXISTING: Kubebuilder operator E2E (Ginkgo)
    e2e_test.go                 # Extend with CRD lifecycle tests
    e2e_suite_test.go           # Existing suite setup
  .github/workflows/
    test.yml                    # EXISTING: Go unit tests
    test-e2e.yml                # EXISTING: Operator E2E tests
    lint.yml                    # EXISTING: golangci-lint
    test-playwright.yml         # NEW: Playwright E2E workflow
```

**Key decision: Playwright lives in `e2e/` (root level), NOT inside `web/`.**

Rationale: Playwright tests the integrated system (Go backend + embedded SPA), not just the React frontend. The test suite needs to build the Go binary, start it, and test against it. Putting it in `web/` implies it's a frontend concern when it's actually a system concern. A root-level `e2e/` directory with its own `package.json` keeps test dependencies isolated from the production `web/` build.

---

## Installation

### Playwright E2E Setup

```bash
# Create the e2e directory and initialize
mkdir -p e2e
cd e2e

# Initialize with Playwright
npm init -y
npm install -D @playwright/test@^1.58

# Install Chromium browser binary (CI will use --with-deps)
npx playwright install chromium
```

**e2e/package.json** (minimal):
```json
{
  "name": "kterodactyl-e2e",
  "private": true,
  "scripts": {
    "test": "playwright test",
    "test:ui": "playwright test --ui",
    "test:headed": "playwright test --headed",
    "report": "playwright show-report"
  },
  "devDependencies": {
    "@playwright/test": "^1.58"
  }
}
```

### kind (for local development)

```bash
# Install kind v0.31.0
go install sigs.k8s.io/kind@v0.31.0

# Or download binary directly (CI approach)
curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.31.0/kind-linux-amd64
chmod +x ./kind
sudo mv ./kind /usr/local/bin/kind
```

### Go Dependencies (no new additions)

```bash
# All Go test dependencies already present:
# - net/http/httptest (stdlib)
# - testing (stdlib)
# - controller-runtime/pkg/client/fake (existing)
# - onsi/ginkgo/v2 + onsi/gomega (existing)
#
# No `go get` needed for v1.1 testing.
```

---

## GitHub Actions Workflow Versions

### Current Workflows (Keep As-Is)

| Workflow | File | Actions Used | Status |
|----------|------|-------------|--------|
| Tests | `test.yml` | checkout@v4, setup-go@v5 | Working. Runs `make test`. |
| E2E Tests | `test-e2e.yml` | checkout@v4, setup-go@v5 | Working but tests are scaffold-only. |
| Lint | `lint.yml` | checkout@v4, setup-go@v5, golangci-lint-action@v8 | Working. |

### New Workflow Needed

| Workflow | File | Actions Needed | Trigger |
|----------|------|---------------|---------|
| Playwright E2E | `test-playwright.yml` | checkout@v4, setup-go@v5, setup-node@v5, upload-artifact@v4 | push + PR to main |

### Recommended Playwright CI Workflow Structure

```yaml
name: Playwright E2E Tests
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  playwright:
    name: Playwright E2E
    runs-on: ubuntu-latest
    timeout-minutes: 30
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - uses: actions/setup-node@v5
        with:
          node-version: 22

      - name: Build frontend
        run: cd web && npm ci && npm run build

      - name: Build Go binary
        run: |
          cp -r web/dist internal/api/frontend
          go build -o bin/manager cmd/main.go

      - name: Install Playwright
        working-directory: e2e
        run: |
          npm ci
          npx playwright install --with-deps chromium

      - name: Run Playwright tests
        working-directory: e2e
        run: npx playwright test

      - name: Upload report
        uses: actions/upload-artifact@v4
        if: ${{ !cancelled() }}
        with:
          name: playwright-report
          path: e2e/playwright-report/
          retention-days: 14
```

---

## Playwright Configuration Recommendations

```typescript
// e2e/playwright.config.ts
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  fullyParallel: false, // Sequential for shared server state
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1, // Single worker -- tests share server state
  reporter: process.env.CI
    ? [['html', { open: 'never' }], ['github']]
    : [['html', { open: 'on-failure' }]],

  use: {
    baseURL: process.env.BASE_URL || 'http://localhost:8080',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },

  webServer: {
    command: '../bin/manager --api-only', // Start the Go binary
    url: 'http://localhost:8080/healthz',
    reuseExistingServer: !process.env.CI,
    timeout: 30_000,
    stdout: 'pipe',
    stderr: 'pipe',
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
```

**Key configuration decisions:**
- **`workers: 1`** -- Tests modify shared state (create users, servers). Parallel execution causes flaky tests. Start sequential, optimize later.
- **`fullyParallel: false`** -- Same reason. The Go backend has shared state.
- **`trace: 'retain-on-failure'`** -- Traces are invaluable for debugging CI failures but expensive to store for passing tests.
- **`reporter: 'github'`** in CI -- Annotates PR with test failures directly in the GitHub UI.
- **`webServer`** -- Playwright manages the Go process lifecycle. Starts it before tests, kills it after.
- **Chromium only** -- One browser in CI keeps the pipeline under 10 minutes. The SPA uses standard web APIs with no browser-specific code.

---

## kind Cluster Configuration for Operator E2E

The existing `test-e2e.yml` downloads kind from `latest`. Pin it for reproducibility:

```yaml
# In .github/workflows/test-e2e.yml
- name: Install kind v0.31.0
  run: |
    curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.31.0/kind-linux-amd64
    chmod +x ./kind
    sudo mv ./kind /usr/local/bin/kind
```

For custom kind cluster config (if needed for Gateway API testing):

```yaml
# kind-config.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    image: kindest/node:v1.32.11@sha256:5fc52d52a7b9574015299724bd68f183702956aa4a2116ae75a63cb574b35af8
```

---

## Version Compatibility Matrix

| Component | Version | Constraint | Source |
|-----------|---------|-----------|--------|
| Go | 1.25.3 | Set in go.mod | Existing |
| Node.js | 22 | Match Dockerfile `node:22-alpine` | Existing |
| K8s (production) | v1.32.3 | Talos cluster | Existing |
| K8s (test/kind) | v1.32.11 | Match production minor version | kind v0.31.0 images |
| controller-runtime | v0.23.1 | Set in go.mod | Existing |
| Ginkgo | v2.27.2 | Set in go.mod | Existing |
| Gomega | v1.38.2 | Set in go.mod | Existing |
| Playwright | ^1.58 | Latest stable | npm registry |
| kind | v0.31.0 | Latest stable | GitHub releases |
| golangci-lint | v2.7.2 | Set in Makefile | Existing |

---

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| E2E Runner | Playwright | Cypress | Cypress is slower, commercial license for parallel, worse multi-browser. Playwright is Microsoft-backed, fully OSS, faster, better auto-wait. |
| E2E Runner | Playwright | Selenium/WebDriver | Legacy API, slow, brittle. Playwright was built to replace it. |
| Go Assertions | stdlib testing | testify | Project already uses stdlib `t.Errorf` pattern. Mixing creates inconsistency. Ginkgo/Gomega is used for operator tests (Kubebuilder convention). |
| K8s Test Cluster | kind | k3d/k3s | kind is already scaffolded by Kubebuilder, integrated into Makefile, and standard for CI. k3d is comparable but switching provides no benefit. |
| K8s Test Cluster | kind | minikube | minikube is heavier (VM-based by default), slower to start, worse CI support. kind is Docker-native and purpose-built for CI. |
| K8s Fake Client | controller-runtime/fake | client-go/fake | controller-runtime fake is typed and scheme-aware. Already in use. client-go fake is lower-level. |
| Component Tests | Skip (for v1.1) | Vitest | Playwright covers the integrated happy paths. Component tests for a panel UI have low ROI vs. E2E. Revisit if the frontend grows complex. |
| CI Reporter | Playwright built-in HTML | Allure | Allure requires Java, adds complexity. Playwright HTML reporter + GitHub annotations is sufficient. |
| Browser in CI | Chromium only | Multi-browser | The SPA uses standard web APIs. Testing 3 browsers triples CI time for negligible coverage gain at this scale. |

---

## Sources

- [Playwright installation docs](https://playwright.dev/docs/intro) -- HIGH confidence (official docs, verified Feb 2026)
- [Playwright CI setup](https://playwright.dev/docs/ci-intro) -- HIGH confidence (official docs)
- [Playwright webServer config](https://playwright.dev/docs/test-webserver) -- HIGH confidence (official docs)
- [@playwright/test npm](https://www.npmjs.com/package/@playwright/test) -- HIGH confidence (v1.58.2 latest as of Feb 2026)
- [kind v0.31.0 release](https://github.com/kubernetes-sigs/kind/releases/tag/v0.31.0) -- HIGH confidence (official release)
- [kind quick start](https://kind.sigs.k8s.io/docs/user/quick-start/) -- HIGH confidence (official docs)
- [Kubebuilder kind integration](https://book.kubebuilder.io/reference/kind) -- HIGH confidence (official docs)
- [Go Wiki: Table-Driven Tests](https://go.dev/wiki/TableDrivenTests) -- HIGH confidence (official Go wiki)
- [golangci-lint-action releases](https://github.com/golangci/golangci-lint-action/releases) -- HIGH confidence (official repo)
- [GitHub Actions setup-go](https://github.com/actions/setup-go) -- HIGH confidence (official repo, v5/v6 available)
- [GitHub Actions setup-node](https://github.com/actions/setup-node) -- HIGH confidence (official repo)
