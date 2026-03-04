---
phase: 17
slug: ci-pipeline
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-04
---

# Phase 17 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | GitHub Actions (workflow YAML validation via `actionlint` + manual trigger) |
| **Config file** | `.github/workflows/ci.yml` |
| **Quick run command** | `actionlint .github/workflows/ci.yml` |
| **Full suite command** | `gh workflow run ci.yml --ref $(git branch --show-current)` |
| **Estimated runtime** | ~2 seconds (lint) / ~15 min (full CI run) |

---

## Sampling Rate

- **After every task commit:** Run `actionlint .github/workflows/ci.yml`
- **After every plan wave:** Validate YAML structure and job dependency graph
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 2 seconds (actionlint)

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 17-01-01 | 01 | 1 | CI-01 | lint | `actionlint .github/workflows/ci.yml` | ❌ W0 | ⬜ pending |
| 17-01-02 | 01 | 1 | CI-02 | lint | `actionlint .github/workflows/ci.yml` | ❌ W0 | ⬜ pending |
| 17-01-03 | 01 | 1 | CI-03 | lint | `actionlint .github/workflows/ci.yml` | ❌ W0 | ⬜ pending |
| 17-01-04 | 01 | 1 | CI-04 | lint | `actionlint .github/workflows/ci.yml` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `actionlint` installed — CI workflow YAML linter
- [ ] `.github/workflows/ci.yml` — unified workflow file (created by plan)

*Existing test infrastructure (make lint, make test, make test-integration, make test-e2e-*, make test-playwright) covers all runtime verification.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Full CI run passes on PR | CI-01 | Requires GitHub Actions runner | Push branch, open PR, verify all jobs pass |
| Failure artifacts downloadable | CI-02 | Requires actual test failure in CI | Introduce intentional failure, verify artifacts uploaded |
| Disk cleanup prevents exhaustion | CI-03 | Requires runner disk state | Monitor runner disk usage in CI logs |
| Kind cleanup on failure | CI-04 | Requires test failure mid-run | Verify cleanup step runs with `if: always()` |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 2s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
