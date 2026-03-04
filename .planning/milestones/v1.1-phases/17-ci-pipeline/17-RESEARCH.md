# Phase 17: CI Pipeline - Research

**Researched:** 2026-03-04
**Domain:** GitHub Actions CI/CD for Kubernetes operator with Go, Playwright, and kind
**Confidence:** HIGH

## Summary

Phase 17 unifies three separate GitHub Actions workflows (`lint.yml`, `test.yml`, `test-e2e.yml`) into a single `ci.yml` with job dependencies, adds Playwright browser tests as a CI stage, uploads failure artifacts, performs disk cleanup before kind cluster creation, and ensures kind cluster cleanup even on failure.

The project already has all test infrastructure in place (Makefile targets, kind config, Playwright config with CI-aware settings, hack scripts). The CI pipeline is a composition layer -- no new test code is needed, only a well-structured workflow YAML.

**Primary recommendation:** Create a single `.github/workflows/ci.yml` with five jobs (lint, unit-test, integration-test, e2e-test, playwright-test) using `needs:` for dependency ordering, `if: always()` for kind cleanup, `jlumbroso/free-disk-space` for disk reclamation, and `actions/upload-artifact@v4` for failure diagnostics.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CI-01 | Unified GitHub Actions workflow runs lint -> unit tests -> integration tests -> E2E tests -> Playwright tests with job dependencies | Job dependency chain via `needs:`, single `ci.yml` replacing three separate files |
| CI-02 | CI pipeline uploads Playwright traces, screenshots, and k8s logs as artifacts on failure | `actions/upload-artifact@v4` with `if: ${{ !cancelled() }}`, Playwright `test-results/` and `playwright-report/` dirs, kubectl logs capture step |
| CI-03 | CI pipeline performs disk cleanup before heavy steps to prevent space exhaustion | `jlumbroso/free-disk-space` action before kind cluster creation, frees ~25GB |
| CI-04 | Kind cluster is always cleaned up after E2E tests, even on failure | `if: always()` on cleanup step, NOT relying on Makefile chaining |
</phase_requirements>

## Standard Stack

### Core
| Tool | Version | Purpose | Why Standard |
|------|---------|---------|--------------|
| GitHub Actions | N/A | CI/CD platform | Already in use, three existing workflows |
| actions/checkout | v4 | Repository clone | Standard, already used in all workflows |
| actions/setup-go | v5 | Go toolchain | Standard, already used |
| actions/setup-node | v5 | Node.js for Playwright | Official GitHub action |
| actions/upload-artifact | v4 | Upload failure diagnostics | Official, current major version |
| helm/kind-action | v1 | Kind cluster management | Official Helm-maintained action for kind |
| golangci/golangci-lint-action | v8 | Go linting | Already used in lint.yml |
| jlumbroso/free-disk-space | main | Disk cleanup on runners | Widely used, frees ~25-31GB |

### Supporting
| Tool | Purpose | When to Use |
|------|---------|-------------|
| npx playwright install --with-deps | Install Chromium + OS deps | Playwright job only |
| kubectl | Capture pod logs on failure | E2E and Playwright jobs |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| jlumbroso/free-disk-space | Manual `sudo rm -rf` commands | Action is cleaner, well-maintained; manual gives finer control but more maintenance |
| helm/kind-action | Manual kind install (current approach in test-e2e.yml) | kind-action handles install+create+cleanup; manual gives exact version control |
| Single unified workflow | Separate workflows with workflow_run triggers | Single file is simpler, `needs:` gives clear dependency chain |

## Architecture Patterns

### Recommended Workflow Structure

```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [main]
  pull_request:

jobs:
  lint:          # No dependencies, runs first
  unit-test:     # needs: [lint]
  integration-test:  # needs: [lint]
  e2e-test:      # needs: [unit-test, integration-test]
  playwright:    # needs: [e2e-test] (shares kind cluster setup pattern)
```

### Pattern 1: Job Dependency Chain with Fail-Fast
**What:** Use `needs:` to create a DAG where lint failure skips all downstream jobs
**When to use:** Always -- this is the core CI-01 requirement
**Example:**
```yaml
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - uses: golangci/golangci-lint-action@v8
        with:
          version: v2.7.2

  unit-test:
    needs: [lint]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: make test

  integration-test:
    needs: [lint]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: make test-integration

  e2e-test:
    needs: [unit-test, integration-test]
    runs-on: ubuntu-latest
    steps:
      # disk cleanup, kind setup, test, cleanup...

  playwright:
    needs: [e2e-test]
    runs-on: ubuntu-latest
    steps:
      # disk cleanup, kind setup, playwright, cleanup, artifact upload...
```

### Pattern 2: Always-Run Cleanup
**What:** Use `if: always()` on cleanup steps so kind cluster is deleted even when tests fail
**When to use:** Any step that provisions infrastructure (CI-04)
**Example:**
```yaml
- name: Run Playwright tests
  run: cd e2e && npx playwright test

- name: Cleanup kind cluster
  if: always()
  run: kind delete cluster --name kterodactyl-test-e2e
```

### Pattern 3: Conditional Artifact Upload
**What:** Upload artifacts when tests fail using `if: ${{ !cancelled() }}`
**When to use:** Playwright trace/screenshot upload, k8s log capture (CI-02)
**Example:**
```yaml
- name: Capture k8s logs on failure
  if: failure()
  run: |
    kubectl get pods -n kterodactyl-system -o wide || true
    kubectl logs -l app.kubernetes.io/name=kterodactyl -n kterodactyl-system --tail=200 > /tmp/k8s-pod-logs.txt 2>&1 || true
    kubectl describe pods -n kterodactyl-system > /tmp/k8s-pod-describe.txt 2>&1 || true

- name: Upload Playwright report
  if: ${{ !cancelled() }}
  uses: actions/upload-artifact@v4
  with:
    name: playwright-report
    path: e2e/playwright-report/
    retention-days: 14

- name: Upload Playwright test results
  if: ${{ !cancelled() }}
  uses: actions/upload-artifact@v4
  with:
    name: playwright-test-results
    path: e2e/test-results/
    retention-days: 14

- name: Upload k8s logs
  if: failure()
  uses: actions/upload-artifact@v4
  with:
    name: k8s-logs
    path: /tmp/k8s-*.txt
    retention-days: 14
```

### Pattern 4: Disk Cleanup Before Kind
**What:** Remove unused preinstalled software to free ~25GB before Docker-heavy steps
**When to use:** Jobs that build Docker images and create kind clusters (CI-03)
**Example:**
```yaml
- name: Free disk space
  uses: jlumbroso/free-disk-space@main
  with:
    tool-cache: false      # Keep -- Go/Node need it
    android: true          # +14GB
    dotnet: true           # +2.7GB
    haskell: true          # +0GB (already removed on newer images)
    large-packages: true   # +5.3GB
    docker-images: false   # Keep -- kind needs Docker
    swap-storage: false    # Keep -- builds may need swap
```

**CRITICAL:** Do NOT remove `docker-images` -- kind requires Docker. Do NOT remove `tool-cache` if using `actions/setup-go` or `actions/setup-node` in the same job.

### Anti-Patterns to Avoid
- **Relying on Makefile chaining for cleanup:** `make test-e2e` calls `make cleanup-test-e2e` at the end, but if the test step fails, cleanup is skipped. Always use `if: always()` in the workflow.
- **Using separate workflows with workflow_run:** Creates complex inter-workflow dependencies, harder to reason about, and `needs:` in a single workflow is cleaner.
- **Installing Playwright browsers in every job:** Only the playwright job needs browsers. Use `npx playwright install --with-deps chromium` (not all browsers).
- **Combining e2e-test and playwright in one job:** They have different failure artifacts and different dependencies (Go-only vs Go+Node+Chromium). Separate jobs give clearer failure signals.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Disk space cleanup | Custom rm -rf scripts | jlumbroso/free-disk-space | Maintained action, handles edge cases, well-tested on various runner images |
| Kind cluster lifecycle | Custom curl/install scripts | helm/kind-action OR existing Makefile targets | Already have Makefile targets; kind-action adds install convenience |
| Artifact upload | Custom artifact scripts | actions/upload-artifact@v4 | Official action, handles compression, retention, permissions |
| Playwright browser install | Manual apt-get + download | npx playwright install --with-deps | Handles all OS deps automatically |

**Key insight:** The existing Makefile targets (`test-e2e-setup`, `test-e2e-teardown`, `test`, `test-integration`, `test-playwright`) already encapsulate the test commands. The CI workflow should call these targets, not duplicate their logic.

## Common Pitfalls

### Pitfall 1: Kind Cluster Not Cleaned Up on Failure
**What goes wrong:** Kind cluster persists on the runner (wastes resources, though runner is ephemeral)
**Why it happens:** Makefile `test-e2e` runs cleanup after tests, but if tests fail, Make exits before reaching cleanup
**How to avoid:** Use `if: always()` on the cleanup step in the workflow, separate from the test step
**Warning signs:** CI runner hitting resource limits on subsequent steps

### Pitfall 2: Disk Exhaustion During Docker Build + Kind Load
**What goes wrong:** "No space left on device" during `docker build` or `kind load docker-image`
**Why it happens:** ubuntu-latest runners have ~25GB free; Docker build (Go + Node layers) + kind node image + app image can exceed this
**How to avoid:** Run `jlumbroso/free-disk-space` before Docker-heavy steps; do NOT remove `docker-images` or `tool-cache`
**Warning signs:** Build failures with ENOSPC errors

### Pitfall 3: Playwright Artifacts Not Captured
**What goes wrong:** Tests fail but no traces/screenshots are uploaded
**Why it happens:** Using `if: success()` (default) instead of `if: ${{ !cancelled() }}` on artifact upload step
**How to avoid:** Always use `if: ${{ !cancelled() }}` for artifact upload steps
**Warning signs:** Test failures with no downloadable reports in GitHub Actions UI

### Pitfall 4: Missing Playwright OS Dependencies
**What goes wrong:** Chromium fails to launch with missing shared library errors
**Why it happens:** Using `npx playwright install` without `--with-deps`
**How to avoid:** Always use `npx playwright install --with-deps chromium`
**Warning signs:** Error messages about libgbm, libasound2, or other shared libraries

### Pitfall 5: Helm Not Available for Kind Setup
**What goes wrong:** `helm install` fails because Helm is not installed
**Why it happens:** ubuntu-latest images include Helm, but version may vary
**How to avoid:** ubuntu-latest includes Helm 3.x pre-installed; verify with `helm version` step if concerned
**Warning signs:** "helm: command not found"

### Pitfall 6: kubectl Context Not Set After Kind Creation
**What goes wrong:** kubectl commands fail because context points to wrong cluster
**Why it happens:** kind sets kubectl context automatically, but if using kind-action, verify
**How to avoid:** `make test-e2e-setup` uses kind which auto-sets context; explicit `kubectl cluster-info` verification step
**Warning signs:** "The connection to the server was refused"

## Code Examples

### Complete E2E + Playwright Job (Key Pattern)
```yaml
playwright:
  needs: [e2e-test]
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4

    - name: Free disk space
      uses: jlumbroso/free-disk-space@main
      with:
        tool-cache: false
        android: true
        dotnet: true
        haskell: true
        large-packages: true
        docker-images: false
        swap-storage: false

    - uses: actions/setup-go@v5
      with:
        go-version-file: go.mod

    - uses: actions/setup-node@v5
      with:
        node-version: lts/*

    - name: Install Playwright browsers
      working-directory: e2e
      run: |
        npm ci
        npx playwright install --with-deps chromium

    - name: Setup kind cluster with Helm
      run: |
        curl -Lo ./kind https://kind.sigs.k8s.io/dl/latest/kind-linux-amd64
        chmod +x ./kind
        sudo mv ./kind /usr/local/bin/kind
        make test-e2e-setup

    - name: Run Playwright tests
      working-directory: e2e
      run: npx playwright test

    - name: Capture k8s logs
      if: failure()
      run: |
        mkdir -p /tmp/k8s-logs
        kubectl get pods -A -o wide > /tmp/k8s-logs/pods.txt 2>&1 || true
        kubectl logs -l app.kubernetes.io/name=kterodactyl \
          -n kterodactyl-system --tail=500 \
          > /tmp/k8s-logs/app-logs.txt 2>&1 || true
        kubectl describe pods -n kterodactyl-system \
          > /tmp/k8s-logs/pod-describe.txt 2>&1 || true

    - name: Upload Playwright report
      if: ${{ !cancelled() }}
      uses: actions/upload-artifact@v4
      with:
        name: playwright-report
        path: e2e/playwright-report/
        retention-days: 14

    - name: Upload Playwright test results
      if: ${{ !cancelled() }}
      uses: actions/upload-artifact@v4
      with:
        name: playwright-test-results
        path: e2e/test-results/
        retention-days: 14

    - name: Upload k8s logs
      if: failure()
      uses: actions/upload-artifact@v4
      with:
        name: k8s-logs
        path: /tmp/k8s-logs/
        retention-days: 14

    - name: Cleanup kind cluster
      if: always()
      run: kind delete cluster --name kterodactyl-test-e2e
```

### Existing Config Already CI-Aware
The Playwright config at `e2e/playwright.config.ts` already handles CI:
```typescript
forbidOnly: !!process.env.CI,        // Fails if .only left in
retries: process.env.CI ? 1 : 0,     // One retry in CI
reporter: process.env.CI ? 'github' : 'html',  // GitHub reporter in CI
trace: 'on-first-retry',             // Captures trace on retry
screenshot: 'only-on-failure',       // Captures screenshot on failure
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Separate workflow files per test tier | Single unified workflow with `needs:` | Current best practice | Clearer dependency chain, single status check |
| actions/upload-artifact@v3 | actions/upload-artifact@v4 | 2024 | v4 has better performance, immutable artifacts |
| Manual disk cleanup scripts | jlumbroso/free-disk-space action | 2023+ | Reliable, maintained, handles runner image changes |
| `if: always()` on artifact upload | `if: ${{ !cancelled() }}` | Best practice | Doesn't upload on manual cancellation |

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | GitHub Actions (workflow YAML) |
| Config file | `.github/workflows/ci.yml` (to be created) |
| Quick run command | N/A -- CI validates itself by running on PR |
| Full suite command | Push to a PR branch and observe workflow run |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CI-01 | Unified workflow with job dependencies | manual-only | Open PR, verify all 5 jobs run in order | N/A |
| CI-02 | Artifact upload on failure | manual-only | Trigger a deliberate test failure, verify artifacts downloadable | N/A |
| CI-03 | Disk cleanup before kind | manual-only | Check workflow logs for free-disk-space step output | N/A |
| CI-04 | Kind cleanup on failure | manual-only | Check workflow logs for cleanup step running after failed test | N/A |

**Justification for manual-only:** CI pipeline configuration is infrastructure-as-code that validates itself by running. There is no meaningful way to unit test a GitHub Actions workflow YAML locally. Verification is done by observing actual CI runs.

### Sampling Rate
- **Per task commit:** Push to PR, observe workflow execution
- **Per wave merge:** Full PR cycle with all jobs passing
- **Phase gate:** At least one successful full pipeline run with all 5 jobs green

### Wave 0 Gaps
None -- no test infrastructure needed. The workflow file IS the deliverable.

## Open Questions

1. **Should the old workflow files be deleted or kept?**
   - What we know: Three separate files exist (`lint.yml`, `test.yml`, `test-e2e.yml`)
   - What's unclear: Whether to delete them (cleaner) or keep alongside unified workflow (safer rollback)
   - Recommendation: Delete them. Having both causes duplicate CI runs. The unified workflow replaces all three.

2. **Should e2e-test (Go/Ginkgo) and playwright share a kind cluster in one job?**
   - What we know: Both need a kind cluster with Helm-deployed Kterodactyl
   - What's unclear: Whether combining saves meaningful time vs. isolation benefits
   - Recommendation: Keep separate jobs. Different failure artifacts, different toolchains. The Docker image build + kind load is fast (~2-3 min). Isolation gives clearer failure diagnostics.

3. **Integration tests need a kind cluster too?**
   - What we know: `make test-integration` runs `go test -tags integration ./test/integration/...` which hits a live API
   - What's unclear: Whether integration tests need the full Helm-deployed kind cluster or just Go unit-style tests
   - Recommendation: Check if integration tests require a running cluster. If yes, they should share the e2e-test job's cluster setup pattern. If no (httptest-based), they can run standalone.

## Sources

### Primary (HIGH confidence)
- Existing workflow files: `.github/workflows/lint.yml`, `test.yml`, `test-e2e.yml` -- current CI setup
- Existing Makefile -- all test targets and kind cluster management
- `e2e/playwright.config.ts` -- CI-aware Playwright configuration
- `hack/kind-config.yaml`, `hack/ci-values.yaml`, `hack/wait-for-ready.sh` -- kind cluster setup
- [Playwright CI docs](https://playwright.dev/docs/ci-intro) -- official GitHub Actions setup guide

### Secondary (MEDIUM confidence)
- [jlumbroso/free-disk-space](https://github.com/jlumbroso/free-disk-space) -- disk cleanup action docs
- [helm/kind-action](https://github.com/helm/kind-action) -- kind GitHub Action (v1, kind v0.31.0 default)
- [GitHub Actions runner-images disk discussion](https://github.com/actions/runner-images/discussions/9329) -- ubuntu-latest disk space details

### Tertiary (LOW confidence)
- None -- all findings verified against official sources

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - using only official GitHub Actions and well-established community actions
- Architecture: HIGH - straightforward workflow YAML composition, all test targets already exist
- Pitfalls: HIGH - well-documented issues with disk space and artifact capture in CI

**Research date:** 2026-03-04
**Valid until:** 2026-04-04 (stable domain, GitHub Actions conventions change slowly)
