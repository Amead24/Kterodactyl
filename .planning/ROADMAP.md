# Roadmap: Kterodactyl

## Overview

Kterodactyl delivers a Kubernetes-native game server management panel in 12 phases, starting from operator foundation and CRD design, building through networking and authentication, adding the API and UI layers, then completing with mod support, backups, observability, packaging, and documentation. Each phase delivers a coherent, verifiable capability that builds toward the core value: admins deploy a single Helm chart and users get self-service game server provisioning backed entirely by Kubernetes.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Operator Foundation** - GameServer CRD and basic reconciliation controller
- [x] **Phase 2: Networking & DNS** - Per-server DNS names and routing infrastructure
- [x] **Phase 3: Authentication** - User invite system and session management
- [x] **Phase 4: API Server Bridge** - Go REST API gateway to Kubernetes
- [x] **Phase 5: Game Definition Framework** - Declarative game manifests and Minecraft reference
- [x] **Phase 6: Frontend UI** - React/Next.js admin interface with dynamic forms
- [x] **Phase 7: Console & Real-time Features** - WebSocket console and resource monitoring
- [x] **Phase 8: Mod Support** - User mod uploads with persistent storage
- [x] **Phase 9: Backup System** - S3-compatible backups and restore functionality
- [ ] **Phase 10: Observability** - Prometheus metrics for operator and servers
- [ ] **Phase 11: Helm Packaging** - Production-ready Helm chart for installation
- [ ] **Phase 12: Documentation** - Docusaurus site with guides and reference docs

## Phase Details

### Phase 1: Operator Foundation
**Goal**: GameServer CRD exists with a working reconciliation controller that creates and manages game server Pods
**Depends on**: Nothing (first phase)
**Requirements**: OPER-01, OPER-02, OPER-03, OPER-04, OPER-05, OPER-06, OPER-07
**Success Criteria** (what must be TRUE):
  1. Developer can create a GameServer CR via kubectl and operator reconciles it into a running Pod
  2. GameServer follows state machine lifecycle (Creating → Ready → Allocated → Shutdown)
  3. User can start, stop, restart, and delete game servers via kubectl
  4. Each user's servers run in isolated namespace with ResourceQuotas applied
  5. Operator runs with leader election enabled for high availability
**Plans:** 4 plans

Plans:
- [x] 01-01-PLAN.md — Scaffold Kubebuilder v4 project with GameServer CRD types and state machine
- [x] 01-02-PLAN.md — Implement GameServer controller reconciliation with Pod management
- [x] 01-03-PLAN.md — Add namespace isolation (ResourceQuota, LimitRange, NetworkPolicy) and admin ConfigMap
- [x] 01-04-PLAN.md — Integration tests with envtest and production readiness verification

### Phase 2: Networking & DNS
**Goal**: Each game server is accessible at a human-readable DNS name following the pattern game.username.domain.com
**Depends on**: Phase 1
**Requirements**: NET-01, NET-02, NET-03, NET-04
**Success Criteria** (what must be TRUE):
  1. Game server is accessible at `<game>.<username>.domain.com` DNS name
  2. DNS Controller automatically creates HTTPRoute resources for wildcard routing
  3. ExternalDNS provisions DNS records without manual intervention
  4. User sees connection info (DNS name and port) after server reaches Ready state
**Plans:** 3 plans

Plans:
- [x] 02-01-PLAN.md — Add Gateway API dependency, networking utilities, and admin config extensions
- [x] 02-02-PLAN.md — Implement DNS controller with Service and HTTPRoute management
- [x] 02-03-PLAN.md — Update NetworkPolicy for gateway traffic and add integration tests

### Phase 3: Authentication
**Goal**: Admin can invite users and users can manage their own authenticated sessions
**Depends on**: Phase 2
**Requirements**: AUTH-01, AUTH-02, AUTH-03, AUTH-04
**Success Criteria** (what must be TRUE):
  1. Admin can send email invitations to new users
  2. User can self-register with email and password
  3. User stays logged in across browser refresh via JWT token
  4. User can only access and manage their own game servers (isolation enforced)
**Plans:** 3 plans

Plans:
- [x] 03-01-PLAN.md — Auth types, Argon2id password hashing, and K8s Secret user store
- [x] 03-02-PLAN.md — JWT service with signing key persistence and AdminConfig auth extensions
- [x] 03-03-PLAN.md — HTTP auth middleware, invitation flow, and unit tests

### Phase 4: API Server Bridge
**Goal**: Go REST API server acts as authenticated gateway between users and Kubernetes API
**Depends on**: Phase 3
**Requirements**: (Infrastructure requirement - enables GAME and UI phases)
**Success Criteria** (what must be TRUE):
  1. API server validates JWT tokens and maps users to namespaces
  2. User can create, read, update, and delete GameServer resources via REST API
  3. API server loads game manifests from games/ directory
  4. API server never exposes Kubernetes API directly to users
  5. Rate limiting prevents resource exhaustion attacks
**Plans:** 4 plans

Plans:
- [x] 04-01-PLAN.md — Game manifest loader and API server scaffold with chi router
- [x] 04-02-PLAN.md — Auth and GameServer CRUD handlers with tests
- [x] 04-03-PLAN.md — Game manifest and admin handlers with tests
- [x] 04-04-PLAN.md — Manager integration and full build verification

### Phase 5: Game Definition Framework
**Goal**: Games are defined declaratively with Dockerfile and manifest, enabling community contributions
**Depends on**: Phase 4
**Requirements**: GAME-01, GAME-02, GAME-03, GAME-04, GAME-05
**Success Criteria** (what must be TRUE):
  1. Game definitions exist in games/ directory with Dockerfile and manifest.yaml per game
  2. Game manifest defines configurable parameters using JSON schema
  3. Minecraft Java Edition works as reference game definition
  4. UI generates configuration forms automatically from game manifest schemas
  5. Documentation clearly explains how to contribute new game definitions via PR
**Plans:** 2 plans

Plans:
- [x] 05-01-PLAN.md — Manifest evolution with directory-per-game structure, JSON Schema validation, and Minecraft reference
- [x] 05-02-PLAN.md — API schema integration, parameter validation on create/update, tests, contribution docs, and Dockerfile

### Phase 6: Frontend UI
**Goal**: Users interact with Kterodactyl through a modern Vite + React SPA embedded in the Go binary
**Depends on**: Phase 4, Phase 5
**Requirements**: (Infrastructure requirement - user-facing interface)
**Success Criteria** (what must be TRUE):
  1. User can browse available games in the UI
  2. User can configure game parameters using dynamically-generated forms
  3. User can launch a game server and see status updates
  4. User sees connection info (DNS name and port) after server is ready
  5. User can stop, restart, and delete their game servers from the UI
**Plans:** 4 plans

Plans:
- [x] 06-01-PLAN.md — Scaffold Vite React project and add lifecycle API endpoints
- [x] 06-02-PLAN.md — Auth pages, app layout, and game browser
- [x] 06-03-PLAN.md — Server creation with RJSF dynamic forms and server dashboard
- [x] 06-04-PLAN.md — Go embed SPA handler, admin pages, and build pipeline

### Phase 7: Console & Real-time Features
**Goal**: Users can view console output and monitor resource usage in real-time
**Depends on**: Phase 6
**Requirements**: CONS-01, CONS-02, CONS-03
**Success Criteria** (what must be TRUE):
  1. User sees real-time server console output via WebSocket connection
  2. User can send commands to running game server via console input
  3. User sees current CPU, RAM, and disk usage for their servers
**Plans:** 2 plans

Plans:
- [x] 07-01-PLAN.md — WebSocket console proxy, metrics REST endpoint, and K8s client infrastructure
- [x] 07-02-PLAN.md — xterm.js terminal UI, metrics display, and server detail page tabs

### Phase 8: Mod Support
**Goal**: Users can upload and apply mods to their game servers
**Depends on**: Phase 7
**Requirements**: MOD-01, MOD-02, MOD-03
**Success Criteria** (what must be TRUE):
  1. User can upload mod files to a game server via the UI
  2. Mods persist on a separate PersistentVolume mounted to the game server container
  3. Server automatically restarts after mod upload completes
**Plans:** 3 plans

Plans:
- [x] 08-01-PLAN.md — Manifest modPath field, controller PVC creation, volume mounting, AdminConfig mod storage
- [x] 08-02-PLAN.md — API mod handlers (upload/list/delete via tar-over-exec) and route registration
- [x] 08-03-PLAN.md — Frontend mod UI with drag-and-drop upload, mod list, and Mods tab

### Phase 9: Backup System
**Goal**: Users can create backups and admins can restore from them using S3-compatible storage
**Depends on**: Phase 8
**Requirements**: BKUP-01, BKUP-02, BKUP-03, BKUP-04, BKUP-05
**Success Criteria** (what must be TRUE):
  1. User can trigger on-demand backup of their game server
  2. Admin can configure scheduled backups via cron schedule
  3. Backups are stored successfully in S3-compatible storage (MinIO, AWS S3, GCS)
  4. Backup status, size, and S3 location are tracked in Backup CRD
  5. Admin can restore a game server from a backup
**Plans:** 3 plans

Plans:
- [x] 09-01-PLAN.md — Backup CRD types, BackupReconciler with S3 backup/restore, backupPath manifest field, AdminConfig S3 config
- [x] 09-02-PLAN.md — Backup API handlers (create, list, delete, restore, schedule) and route registration
- [x] 09-03-PLAN.md — Frontend backup UI with trigger button, backup list, restore dialog, and Backups tab

### Phase 10: Observability
**Goal**: Operators and game servers expose Prometheus metrics for monitoring
**Depends on**: Phase 9
**Requirements**: OBS-01, OBS-02, OBS-03, OBS-04
**Success Criteria** (what must be TRUE):
  1. Operator exposes Prometheus metrics (game server count by state, reconciliation latency)
  2. API server exposes Prometheus metrics (request rate, latency, error rate)
  3. ServiceMonitor CRDs exist for Prometheus Operator autodiscovery
  4. All metrics use low-cardinality labels only (no user IDs or pod names)
**Plans:** 2 plans

Plans:
- [ ] 10-01-PLAN.md — Centralized metric definitions and operator reconciler instrumentation
- [ ] 10-02-PLAN.md — API HTTP metrics middleware, route wiring, and ServiceMonitor enablement

### Phase 11: Helm Packaging
**Goal**: Kterodactyl installs via a single helm install command with proper defaults
**Depends on**: Phase 10
**Requirements**: HELM-01, HELM-02, HELM-03, HELM-04
**Success Criteria** (what must be TRUE):
  1. Kterodactyl installs successfully via `helm install kterodactyl ./chart`
  2. Helm values support configuration of Gateway API vs Ingress, storage class, and domain
  3. CRDs install via crds/ directory with proper ordering
  4. Chart works on both single-node homelab and multi-node cluster deployments
**Plans**: TBD

Plans:
- [ ] TBD during planning

### Phase 12: Documentation
**Goal**: Users and contributors have comprehensive Docusaurus documentation
**Depends on**: Phase 11
**Requirements**: DOCS-01, DOCS-02, DOCS-03, DOCS-04
**Success Criteria** (what must be TRUE):
  1. Docusaurus site covers installation, configuration, and usage workflows
  2. Game definition contribution guide exists with Minecraft example walkthrough
  3. Helm values reference documents all configurable options
  4. Architecture overview exists for contributors
**Plans**: TBD

Plans:
- [ ] TBD during planning

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9 → 10 → 11 → 12

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Operator Foundation | 4/4 | ✓ Complete | 2026-02-10 |
| 2. Networking & DNS | 3/3 | ✓ Complete | 2026-02-10 |
| 3. Authentication | 3/3 | ✓ Complete | 2026-02-10 |
| 4. API Server Bridge | 4/4 | ✓ Complete | 2026-02-10 |
| 5. Game Definition Framework | 2/2 | ✓ Complete | 2026-02-11 |
| 6. Frontend UI | 4/4 | ✓ Complete | 2026-02-11 |
| 7. Console & Real-time Features | 2/2 | ✓ Complete | 2026-02-12 |
| 8. Mod Support | 3/3 | ✓ Complete | 2026-02-12 |
| 9. Backup System | 3/3 | ✓ Complete | 2026-02-12 |
| 10. Observability | 0/2 | Not started | - |
| 11. Helm Packaging | 0/TBD | Not started | - |
| 12. Documentation | 0/TBD | Not started | - |
