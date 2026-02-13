---
phase: 11-helm-packaging
plan: 02
subsystem: infra
tags: [helm, rbac, configmap, servicemonitor, networkpolicy, kubernetes]

# Dependency graph
requires:
  - phase: 11-helm-packaging
    plan: 01
    provides: Chart scaffold (Chart.yaml, values.yaml, _helpers.tpl, Deployment, Services, CRDs)
  - phase: 01-operator-foundation
    provides: RBAC manifests in config/rbac/ and AdminConfig ConfigMap pattern
  - phase: 10-observability
    provides: ServiceMonitor and NetworkPolicy manifests in config/prometheus/ and config/network-policy/
provides:
  - Complete Helm chart with all RBAC templates (7 files) using fullname-prefixed names
  - AdminConfig ConfigMap with hardcoded name matching Go operator expectation
  - Conditional ServiceMonitor and NetworkPolicy templates
  - NOTES.txt with post-install instructions
  - Chart ready for helm install kterodactyl ./chart
affects: [12-documentation]

# Tech tracking
tech-stack:
  added: []
  patterns: [conditional-helm-templates, hardcoded-configmap-name, rbac-fullname-prefix]

key-files:
  created:
    - chart/templates/clusterrole.yaml
    - chart/templates/clusterrolebinding.yaml
    - chart/templates/role-leader-election.yaml
    - chart/templates/rolebinding-leader-election.yaml
    - chart/templates/clusterrole-metrics-auth.yaml
    - chart/templates/clusterrolebinding-metrics-auth.yaml
    - chart/templates/clusterrole-metrics-reader.yaml
    - chart/templates/configmap-admin.yaml
    - chart/templates/servicemonitor.yaml
    - chart/templates/networkpolicy.yaml
    - chart/templates/NOTES.txt
  modified: []

key-decisions:
  - "AdminConfig ConfigMap name hardcoded as kterodactyl-admin-config (not fullname-prefixed) to match Go operator hardcoded lookup"
  - "SMTP fields only rendered when smtp.host is set; backup fields only rendered when backup.enabled is true"
  - "gatewayNamespace defaults to Release.Namespace when not explicitly set in values"

patterns-established:
  - "Hardcoded ConfigMap name pattern: when Go code hardcodes a name, Helm template must match exactly"
  - "Conditional section pattern: {{- if .Values.X }} wraps entire YAML block for optional features"
  - "NOTES.txt conditional guidance: show secret creation commands only when corresponding features are enabled"

# Metrics
duration: 3min
completed: 2026-02-13
---

# Phase 11 Plan 02: Chart Completion Summary

**RBAC templates (7 files) with fullname-prefixed names, AdminConfig ConfigMap with hardcoded name, conditional ServiceMonitor/NetworkPolicy, and NOTES.txt with post-install guidance -- chart renders 12 default resources (14 with conditionals)**

## Performance

- **Duration:** 3 min
- **Started:** 2026-02-13T04:20:04Z
- **Completed:** 2026-02-13T04:23:38Z
- **Tasks:** 2
- **Files modified:** 11

## Accomplishments
- 7 RBAC templates converted from kustomize with fullname-prefixed names and identical rules to source manifests
- AdminConfig ConfigMap rendering all fields the Go operator reads (limits, quota, containerDefaults, networking, auth, smtp, storage, backup) with hardcoded name
- ServiceMonitor and NetworkPolicy render conditionally based on values.yaml flags
- NOTES.txt provides port-forward access instructions, secret creation guidance, CRD upgrade notes, and bootstrap command
- helm lint passes with 0 errors; helm template renders 12 default resources, 14 with all conditionals enabled

## Task Commits

Each task was committed atomically:

1. **Task 1: Create RBAC and AdminConfig ConfigMap templates** - `f39ca49` (feat)
2. **Task 2: Create conditional resources, NOTES.txt, and validate chart** - `42f948c` (feat)

## Files Created/Modified
- `chart/templates/clusterrole.yaml` - Manager ClusterRole with all operator RBAC rules (CRDs, Gateway API, networking)
- `chart/templates/clusterrolebinding.yaml` - Manager ClusterRoleBinding linking to ServiceAccount
- `chart/templates/role-leader-election.yaml` - Namespaced Role for configmaps, leases, events
- `chart/templates/rolebinding-leader-election.yaml` - Namespaced RoleBinding for leader election
- `chart/templates/clusterrole-metrics-auth.yaml` - ClusterRole for tokenreviews and subjectaccessreviews
- `chart/templates/clusterrolebinding-metrics-auth.yaml` - ClusterRoleBinding for metrics auth
- `chart/templates/clusterrole-metrics-reader.yaml` - ClusterRole for /metrics nonResourceURL access
- `chart/templates/configmap-admin.yaml` - AdminConfig ConfigMap with hardcoded name and all operator config fields
- `chart/templates/servicemonitor.yaml` - Prometheus ServiceMonitor (conditional on serviceMonitor.enabled)
- `chart/templates/networkpolicy.yaml` - Metrics NetworkPolicy (conditional on networkPolicy.enabled)
- `chart/templates/NOTES.txt` - Post-install instructions with access, secrets, CRDs, and bootstrap guidance

## Decisions Made
- AdminConfig ConfigMap name hardcoded as `kterodactyl-admin-config` to match the Go operator's hardcoded lookup at `internal/controller/gameserver_controller.go:67`
- SMTP fields conditionally rendered only when `smtp.host` is set to avoid empty ConfigMap entries
- Backup S3 fields conditionally rendered only when `backup.enabled` is true
- `gatewayNamespace` defaults to `Release.Namespace` when not explicitly configured, matching the Go operator default behavior

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Helm chart is complete and ready for `helm install kterodactyl ./chart`
- Chart renders all 14 resources needed for a full operator installation
- Phase 11 (Helm Packaging) is now complete; next is Phase 12 (Documentation)

## Self-Check: PASSED

All 11 created files verified present. Both task commits (f39ca49, 42f948c) verified in git log.

---
*Phase: 11-helm-packaging*
*Completed: 2026-02-13*
