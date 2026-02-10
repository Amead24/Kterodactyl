# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-02-09)

**Core value:** Admins can deploy a single Helm chart and give their users self-service game server provisioning backed entirely by Kubernetes
**Current focus:** Phase 2 in progress — networking and DNS routing

## Current Position

Phase: 2 of 12 (Networking & DNS)
Plan: 2 of 3 in current phase -- COMPLETE
Status: Executing Phase 2
Last activity: 2026-02-10 — Completed 02-02 (DNS controller with Service + HTTPRoute management)

Progress: [██░░░░░░░░] 12%

## Performance Metrics

**Velocity:**
- Total plans completed: 6
- Average duration: 4min
- Total execution time: 0.43 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01-operator-foundation | 4/4 | 20min | 5min |
| 02-networking-dns | 2/3 | 6min | 3min |

**Recent Trend:**
- Last 5 plans: 01-03 (6min), 01-04 (5min), 02-01 (3min), 02-02 (3min)
- Trend: Stable

*Updated after each plan completion*

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- Research phase completed: Identified 10 critical pitfalls, recommended 8-phase structure (expanded to 12 for comprehensive depth)
- Phase 1 must establish CRD versioning strategy and multi-tenant isolation foundation (expensive to retrofit later)
- Gateway API (HTTPRoute) selected over Ingress due to March 2026 retirement timeline
- GameServerState type defined in gameserver_types.go; constants and transitions in gameserver_lifecycle.go
- Kubebuilder v4.11.1 scaffolding conventions used (api/, internal/, cmd/) -- not custom pkg/ layout
- v1alpha1 marked as storageversion for future CRD versioning safety
- Extended ValidTransitions to include Ready->Error, Allocated->Error, Starting->Creating for Pod disappearance handling
- Pod RestartPolicy=Never; operator manages lifecycle, not kubelet
- LeaderElectionID set to kterodactyl-operator.kterodactyl.io
- AnnotationChangedPredicate used in event filter for allocation annotation detection
- AdminConfig loaded per reconciliation from ConfigMap (no operator restart needed for config changes)
- Operator works without admin ConfigMap by using sensible defaults
- NetworkPolicy allows DNS via kube-system and internet minus private ranges
- OperatorNamespace configurable via OPERATOR_NAMESPACE env var (default: kterodactyl-system)
- envtest cannot test Starting->Ready (no kubelet); kind cluster CI covers full lifecycle
- Manager-based test setup for true integration testing (watches, event filters, requeue all tested)
- Unique test namespaces per test case to prevent cross-test interference
- DNS name pattern: game.username.baseDomain (e.g., minecraft.alice.example.com)
- BaseDomain empty string means DNS routing is disabled (opt-in)
- Gateway API scheme registered in init() alongside existing CRD scheme
- Networking constants in separate networking.go file, not in labels.go
- DNS controller uses same patterns as GameServerReconciler: re-fetch before status updates, CreateOrUpdate, owner references
- Service and HTTPRoute share the GameServer name for consistent naming
- Cleanup logic explicitly deletes Service/HTTPRoute and clears status when leaving Ready/Allocated
- updateConnectionInfo skips status write when address unchanged to reduce API churn
- Dual-controller pattern: two reconcilers in same manager binary watching same CRD type with Named() disambiguation

### Pending Todos

- **TODO-01** (Phase 12): Write documentation explaining how Kterodactyl differs from Agones and Pterodactyl
- **TODO-02** (Testing): Create a Playwright script for CI/CD integration testing of features

### Blockers/Concerns

**Phase 1:**
- ~~CRD API design decisions (versioning strategy, state machine states) must be made early~~ RESOLVED in 01-01: v1alpha1 storageversion, 6-state machine
- Controller concurrency and rate limiting settings need production-ready configuration from start

**Phase 2:**
- Port allocation strategy (dynamic pool vs fixed ranges) needs design - critical pitfall identified in research
- ExternalDNS + cert-manager integration may need research during planning for split-horizon DNS patterns

**Phase 4:**
- Authentication mechanism decision needed (JWT only vs OIDC integration scope for v1)

## Session Continuity

Last session: 2026-02-10
Stopped at: Completed 02-02-PLAN.md (DNS controller with Service + HTTPRoute management)
Resume file: None
