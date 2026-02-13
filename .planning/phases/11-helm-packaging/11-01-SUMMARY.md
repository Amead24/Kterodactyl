---
phase: 11-helm-packaging
plan: 01
subsystem: infra
tags: [helm, chart, kubernetes, deployment, service, crd]

# Dependency graph
requires:
  - phase: 01-operator-foundation
    provides: CRD definitions (GameServer, Backup)
  - phase: 10-observability
    provides: Metrics endpoint on port 8443
provides:
  - Helm chart scaffold at chart/ with Chart.yaml, values.yaml, _helpers.tpl
  - CRDs in chart/crds/ (plain YAML, no templating)
  - Deployment template with all operator args, ports, probes, security context
  - API Service template on port 8080
  - Metrics Service template on port 8443 (conditional)
  - ServiceAccount template with conditional creation
affects: [11-helm-packaging]

# Tech tracking
tech-stack:
  added: [helm-chart-v2]
  patterns: [helm-standard-labels, helm-selector-labels, conditional-templates, values-driven-deployment]

key-files:
  created:
    - chart/Chart.yaml
    - chart/values.yaml
    - chart/templates/_helpers.tpl
    - chart/templates/deployment.yaml
    - chart/templates/serviceaccount.yaml
    - chart/templates/service.yaml
    - chart/templates/service-metrics.yaml
    - chart/crds/game.kterodactyl.io_gameservers.yaml
    - chart/crds/game.kterodactyl.io_backups.yaml
  modified: []

key-decisions:
  - "Hand-crafted Helm chart over helmify/Kubebuilder helm plugin for full control"
  - "CRDs in crds/ directory as plain YAML (no templating per Helm convention)"
  - "API Service added (not in kustomize) to expose port 8080 for user access"
  - "OPERATOR_NAMESPACE via downward API fieldRef (always matches deployment namespace)"

patterns-established:
  - "Helm helpers: kterodactyl.fullname, labels, selectorLabels, serviceAccountName"
  - "Conditional template pattern: {{- if .Values.X }} for optional resources"
  - "Values-driven deployment: all configurable settings exposed in values.yaml"

# Metrics
duration: 2min
completed: 2026-02-13
---

# Phase 11 Plan 01: Chart Foundation Summary

**Helm chart scaffold with Chart.yaml (v2), complete values.yaml covering all AdminConfig fields, standard template helpers, CRDs, Deployment with 3 ports/probes/security, and API+Metrics Services**

## Performance

- **Duration:** 2 min
- **Started:** 2026-02-13T04:15:56Z
- **Completed:** 2026-02-13T04:17:44Z
- **Tasks:** 2
- **Files modified:** 9

## Accomplishments
- Chart scaffold with Chart.yaml (apiVersion v2), values.yaml with all AdminConfig fields, and _helpers.tpl with standard Helm helpers
- CRDs copied verbatim from config/crd/bases/ to chart/crds/ (plain YAML, zero template directives)
- Deployment template with leader-elect, health-probe, metrics, and API bind addresses, 3 named ports, liveness/readiness probes, pod and container security contexts
- API Service on port 8080 (new resource not in kustomize) and conditional Metrics Service on port 8443

## Task Commits

Each task was committed atomically:

1. **Task 1: Create chart scaffold with Chart.yaml, values.yaml, and _helpers.tpl** - `20d8d6f` (feat)
2. **Task 2: Create Deployment, ServiceAccount, and Service templates** - `8472ad8` (feat)

## Files Created/Modified
- `chart/Chart.yaml` - Chart metadata (apiVersion v2, kubeVersion >=1.28.0)
- `chart/values.yaml` - All configurable settings with sensible defaults (image, manager, apiService, metrics, adminConfig)
- `chart/templates/_helpers.tpl` - Standard Helm helpers (name, fullname, chart, labels, selectorLabels, serviceAccountName)
- `chart/templates/deployment.yaml` - Operator Deployment with all args, ports, probes, security, scheduling
- `chart/templates/serviceaccount.yaml` - Conditional ServiceAccount with optional annotations
- `chart/templates/service.yaml` - API server Service (port 8080)
- `chart/templates/service-metrics.yaml` - Conditional Metrics Service (port 8443)
- `chart/crds/game.kterodactyl.io_gameservers.yaml` - GameServer CRD (verbatim copy)
- `chart/crds/game.kterodactyl.io_backups.yaml` - Backup CRD (verbatim copy)

## Decisions Made
- Hand-crafted Helm chart over helmify/Kubebuilder helm plugin for full control over values.yaml schema and template structure
- CRDs placed in crds/ directory as plain YAML per Helm convention (no templating, automatic install ordering)
- API Service added as new resource (not in kustomize config) to expose the API server on port 8080
- OPERATOR_NAMESPACE set via downward API fieldRef to always match the deployment namespace (avoids Pitfall 2)
- control-plane: controller-manager included in selectorLabels to maintain compatibility with existing kustomize selectors

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - helm CLI not installed for `helm lint`/`helm template` verification, so templates were validated manually (balanced braces, correct .Values references, no template directives in CRDs).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Chart foundation ready for Plan 02 (RBAC, AdminConfig ConfigMap, ServiceMonitor, NetworkPolicy, NOTES.txt)
- All helpers and values structure in place for remaining templates to reference

## Self-Check: PASSED

All 9 created files verified present. Both task commits (20d8d6f, 8472ad8) verified in git log.

---
*Phase: 11-helm-packaging*
*Completed: 2026-02-13*
