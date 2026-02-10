# Requirements: Kterodactyl

**Defined:** 2026-02-09
**Core Value:** Admins can deploy a single Helm chart and give their users self-service game server provisioning backed entirely by Kubernetes

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Operator & Infrastructure

- [ ] **OPER-01**: Operator creates and manages GameServer CRD with v1alpha1 API
- [ ] **OPER-02**: GameServer follows state machine lifecycle (Creating → Ready → Allocated → Shutdown)
- [ ] **OPER-03**: User can start, stop, restart, and delete game servers via API
- [ ] **OPER-04**: Admin can set global resource limits (max servers, CPU/RAM per server)
- [ ] **OPER-05**: Each user's servers run in isolated namespace with ResourceQuotas and NetworkPolicies
- [ ] **OPER-06**: GameServer CRDs are GitOps-compatible (manageable via `kubectl apply`)
- [ ] **OPER-07**: Operator deploys as a single binary with leader election for HA

### Networking & DNS

- [ ] **NET-01**: Each game server is accessible at `<game>.<username>.domain.com`
- [ ] **NET-02**: DNS Controller creates Gateway API HTTPRoute resources for wildcard routing
- [ ] **NET-03**: ExternalDNS integration automatically provisions DNS records
- [ ] **NET-04**: User sees connection info (DNS name + port) in UI after server is ready

### Authentication

- [ ] **AUTH-01**: Admin can invite users via email
- [ ] **AUTH-02**: User can self-register with email and password
- [ ] **AUTH-03**: User sessions persist via JWT tokens across browser refresh
- [ ] **AUTH-04**: User can only access and manage their own game servers

### Game Definitions

- [ ] **GAME-01**: Games are defined declaratively (Dockerfile + manifest.yaml per game in games/ directory)
- [ ] **GAME-02**: Game manifest defines configurable parameters with JSON schema (ports, env vars, settings)
- [ ] **GAME-03**: Minecraft Java Edition ships as reference game definition
- [ ] **GAME-04**: UI dynamically generates configuration forms from game manifest schemas
- [ ] **GAME-05**: Documentation covers how to contribute new game definitions via PR

### Console & Monitoring

- [ ] **CONS-01**: User can view real-time server console output via WebSocket
- [ ] **CONS-02**: User can send commands to running game server via console
- [ ] **CONS-03**: User can see CPU, RAM, and disk usage for their servers in real-time

### Mod Support

- [ ] **MOD-01**: User can upload mod files to a game server via the UI
- [ ] **MOD-02**: Mods are stored on a separate PersistentVolume mounted to the game server container
- [ ] **MOD-03**: Server automatically restarts after mod upload completes

### Backups

- [ ] **BKUP-01**: User can trigger an on-demand backup of their game server
- [ ] **BKUP-02**: Admin can configure scheduled backups via cron schedule
- [ ] **BKUP-03**: Backups are stored in S3-compatible storage (MinIO, AWS S3, GCS)
- [ ] **BKUP-04**: Backup CRD tracks backup status, size, and S3 location
- [ ] **BKUP-05**: Admin can restore a game server from a backup

### Observability

- [ ] **OBS-01**: Operator exposes Prometheus metrics (game server count by state, reconciliation latency)
- [ ] **OBS-02**: API server exposes Prometheus metrics (request rate, latency, error rate)
- [ ] **OBS-03**: ServiceMonitor CRDs are created for Prometheus Operator autodiscovery
- [ ] **OBS-04**: Metrics use low-cardinality labels only (game_type, server_state — not user_id or pod_name)

### Helm Chart

- [ ] **HELM-01**: Kterodactyl installs via a single `helm install` command
- [ ] **HELM-02**: Helm values are configurable for Gateway API vs Ingress, storage class, and domain
- [ ] **HELM-03**: CRDs install via Helm crds/ directory with proper ordering
- [ ] **HELM-04**: Chart supports both homelab (single node) and multi-node cluster deployments

### Documentation

- [ ] **DOCS-01**: Docusaurus site covers installation, configuration, and usage
- [ ] **DOCS-02**: Game definition contribution guide with Minecraft example walkthrough
- [ ] **DOCS-03**: Helm values reference with all configurable options documented
- [ ] **DOCS-04**: Architecture overview for contributors

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Authentication

- **AUTH-05**: OIDC/SSO integration (Google, Steam, Apple via Dex/Keycloak)
- **AUTH-06**: Subuser RBAC (share server access with friends, granular permissions)

### File Management

- **FILE-01**: Web-based file manager (edit configs in browser, upload/download)
- **FILE-02**: SFTP access for power users

### User Features

- **USER-01**: User can view server logs and download log files
- **USER-02**: User can download their own backups
- **USER-03**: Scheduled tasks (automate restarts, commands on schedule)

### Scale

- **SCALE-01**: Multi-tenancy with organizations
- **SCALE-02**: Per-user resource quotas (individual budgets)

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Billing integration | Massive scope; document API for third-party billing instead |
| Built-in mod installer UI | Game-specific; 100+ integrations; mount mod directories instead |
| Visual node editor | Over-engineered; breaks GitOps declarative model |
| Multi-region orchestration | K8s federation complexity; run multiple panels instead |
| Real-time player list | Requires per-game protocol integration; use Grafana |
| In-panel voice chat | Scope creep; Discord exists |
| Custom DNS management | Security risk; admin configures ExternalDNS once |
| Mobile app | Web UI only for v1 |
| Windows-native deployment | K8s only |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| OPER-01 | Phase 1 | Pending |
| OPER-02 | Phase 1 | Pending |
| OPER-03 | Phase 1 | Pending |
| OPER-04 | Phase 1 | Pending |
| OPER-05 | Phase 1 | Pending |
| OPER-06 | Phase 1 | Pending |
| OPER-07 | Phase 1 | Pending |
| NET-01 | Phase 2 | Pending |
| NET-02 | Phase 2 | Pending |
| NET-03 | Phase 2 | Pending |
| NET-04 | Phase 2 | Pending |
| AUTH-01 | Phase 3 | Pending |
| AUTH-02 | Phase 3 | Pending |
| AUTH-03 | Phase 3 | Pending |
| AUTH-04 | Phase 3 | Pending |
| GAME-01 | Phase 5 | Pending |
| GAME-02 | Phase 5 | Pending |
| GAME-03 | Phase 5 | Pending |
| GAME-04 | Phase 5 | Pending |
| GAME-05 | Phase 5 | Pending |
| CONS-01 | Phase 7 | Pending |
| CONS-02 | Phase 7 | Pending |
| CONS-03 | Phase 7 | Pending |
| MOD-01 | Phase 8 | Pending |
| MOD-02 | Phase 8 | Pending |
| MOD-03 | Phase 8 | Pending |
| BKUP-01 | Phase 9 | Pending |
| BKUP-02 | Phase 9 | Pending |
| BKUP-03 | Phase 9 | Pending |
| BKUP-04 | Phase 9 | Pending |
| BKUP-05 | Phase 9 | Pending |
| OBS-01 | Phase 10 | Pending |
| OBS-02 | Phase 10 | Pending |
| OBS-03 | Phase 10 | Pending |
| OBS-04 | Phase 10 | Pending |
| HELM-01 | Phase 11 | Pending |
| HELM-02 | Phase 11 | Pending |
| HELM-03 | Phase 11 | Pending |
| HELM-04 | Phase 11 | Pending |
| DOCS-01 | Phase 12 | Pending |
| DOCS-02 | Phase 12 | Pending |
| DOCS-03 | Phase 12 | Pending |
| DOCS-04 | Phase 12 | Pending |

**Coverage:**
- v1 requirements: 43 total
- Mapped to phases: 43
- Unmapped: 0

**Note:** Phase 4 (API Server Bridge) and Phase 6 (Frontend UI) are infrastructure requirements that enable multiple feature requirements but don't map to specific requirement IDs. They deliver the API gateway and user interface that other requirements depend on.

---
*Requirements defined: 2026-02-09*
*Last updated: 2026-02-09 after roadmap creation*
