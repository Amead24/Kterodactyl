---
phase: 17-ci-pipeline
verified: 2026-03-04T20:00:00Z
status: passed
score: 6/6 must-haves verified
gaps: []
human_verification:
  - test: "Push a PR with a lint error and confirm downstream jobs are skipped"
    expected: "unit-test and integration-test jobs show 'skipped' status in GitHub Actions"
    why_human: "Job-skip behavior from lint failure requires a real GitHub Actions run to confirm"
  - test: "Push a PR where Playwright tests fail and confirm artifacts are downloadable"
    expected: "playwright-report and playwright-test-results artifacts appear under the Actions run summary"
    why_human: "Artifact upload path correctness requires an actual CI execution"
---

# Phase 17: CI Pipeline Verification Report

**Phase Goal:** Every pull request automatically runs the full test suite with clear pass/fail status and failure diagnostics
**Verified:** 2026-03-04T20:00:00Z
**Status:** passed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A single ci.yml workflow runs lint, unit-test, integration-test, e2e-test, and playwright jobs in dependency order | VERIFIED | ci.yml lines 9-202: 5 jobs present; needs chain: lint <- (unit-test, integration-test) <- e2e-test <- playwright confirmed at lines 28, 44, 60, 119 |
| 2 | Lint failure skips all downstream jobs | VERIFIED | unit-test and integration-test both declare `needs: [lint]` (lines 28, 44); GitHub Actions skips dependents when a needed job fails |
| 3 | Playwright traces, screenshots, and k8s pod logs are uploaded as artifacts on failure | VERIFIED | playwright-report (line 178, `if: !cancelled()`), playwright-test-results (line 185, `if: !cancelled()`), playwright-k8s-logs (line 193, `if: failure()`) all present; trace and screenshot capture governed by playwright.config.ts (`trace: 'on-first-retry'`, `screenshot: 'only-on-failure'`) |
| 4 | Disk cleanup runs before kind cluster creation to free space | VERIFIED | `jlumbroso/free-disk-space@main` is the first step in both e2e-test (line 64) and playwright (line 123) jobs, before any kind install step |
| 5 | Kind cluster is always deleted after tests, even on failure | VERIFIED | Both e2e-test (line 114) and playwright (line 201) have `if: always()` on `kind delete cluster --name kterodactyl-test-e2e` |
| 6 | Old separate workflow files are removed so CI does not run duplicate pipelines | VERIFIED | `ls .github/workflows/` returns only `ci.yml`; git commit 5fdb60d confirms lint.yml, test.yml, test-e2e.yml were deleted |

**Score:** 6/6 truths verified

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `.github/workflows/ci.yml` | Unified CI pipeline with 5 jobs and dependency chain | VERIFIED | File exists, 202 lines (exceeds min_lines: 120), contains `needs:` at 4 locations |
| `.github/workflows/lint.yml` | Deleted — must not exist | VERIFIED | Absent from filesystem; deleted in commit 5fdb60d |
| `.github/workflows/test.yml` | Deleted — must not exist | VERIFIED | Absent from filesystem; deleted in commit 5fdb60d |
| `.github/workflows/test-e2e.yml` | Deleted — must not exist | VERIFIED | Absent from filesystem; deleted in commit 5fdb60d |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `.github/workflows/ci.yml` | `Makefile` | `make test`, `make test-integration`, `make test-e2e-setup` | WIRED | Lines 40, 56, 89, 159 call make targets; all targets confirmed in Makefile (lines 61, 65, 103) |
| `.github/workflows/ci.yml (playwright job)` | `e2e/playwright.config.ts` | `npx playwright test` respects CI env var | WIRED | Line 163 invokes `npx playwright test`; playwright.config.ts uses `process.env.CI` for `forbidOnly`, `retries`, and `reporter` — CI var is set automatically by GitHub Actions |
| `.github/workflows/ci.yml (artifact upload)` | `actions/upload-artifact@v4` | Conditional upload on failure/cancellation | WIRED | Lines 107, 178, 185, 193 all use `actions/upload-artifact@v4`; conditionals are `failure()`, `!cancelled()`, `always()` as required |

---

### Requirements Coverage

| Requirement | Description | Status | Evidence |
|-------------|-------------|--------|----------|
| CI-01 | Unified GitHub Actions workflow runs lint -> unit tests -> integration tests -> E2E tests -> Playwright tests with job dependencies | SATISFIED | 5-job ci.yml with correct needs chain; lint failure gates all downstream jobs |
| CI-02 | CI pipeline uploads Playwright traces, screenshots, and k8s logs as artifacts on failure | SATISFIED | playwright-report, playwright-test-results uploaded on `!cancelled()`; playwright-k8s-logs and e2e-k8s-logs uploaded on `failure()`; playwright.config.ts captures traces on retry and screenshots on failure |
| CI-03 | CI pipeline performs disk cleanup before heavy steps to prevent space exhaustion | SATISFIED | `jlumbroso/free-disk-space@main` is first step in e2e-test and playwright jobs, before kind install |
| CI-04 | Kind cluster is always cleaned up after E2E tests, even on failure | SATISFIED | `if: always()` on `kind delete cluster` in both e2e-test (line 114) and playwright (line 201) |

No orphaned requirements: REQUIREMENTS.md maps CI-01, CI-02, CI-03, CI-04 to Phase 17 only, and all four are satisfied. No additional Phase 17 requirements exist in REQUIREMENTS.md beyond these four.

---

### Anti-Patterns Found

No anti-patterns detected in `.github/workflows/ci.yml`:
- No TODO/FIXME/placeholder comments
- No stub implementations
- No empty handlers or return-null patterns (not applicable to YAML workflows)
- `make test-e2e` is correctly NOT used in the e2e-test job (plan explicitly required using `make test-e2e-setup` + explicit go test command to allow `if: always()` cleanup separation)

---

### Human Verification Required

#### 1. Lint failure gates downstream jobs

**Test:** Open a PR that introduces a Go lint error (e.g., unused variable or missing error check)
**Expected:** GitHub Actions shows the lint job as failed and unit-test, integration-test, e2e-test, playwright jobs all show as skipped/not run
**Why human:** GitHub Actions job-skip cascade behavior requires an actual CI run to confirm; cannot be determined from static YAML inspection alone

#### 2. Playwright failure artifacts are downloadable

**Test:** Open a PR where at least one Playwright test fails (or manually cancel a run mid-Playwright-job)
**Expected:** The Actions run summary shows downloadable artifacts: playwright-report, playwright-test-results (on any non-cancellation outcome) and playwright-k8s-logs (on failure)
**Why human:** Artifact upload conditional logic (`!cancelled()` vs `failure()`) requires a real run and real GitHub UI to verify artifact appearance

---

### Gaps Summary

No gaps. All six observable truths are verified against the actual codebase. The CI workflow file exists with substantive content (202 lines), is fully wired to Makefile targets and Playwright config, all required artifact uploads are present with correct conditionals, disk cleanup precedes kind steps in both cluster-using jobs, kind cleanup uses `if: always()` in both jobs, and the three old workflow files are confirmed deleted from the filesystem and from git history.

Two items require human verification via an actual GitHub Actions run: lint-failure job-skip cascade and Playwright artifact downloadability. These are behavioral confirmations that static analysis cannot substitute for.

---

_Verified: 2026-03-04T20:00:00Z_
_Verifier: Claude (gsd-verifier)_
