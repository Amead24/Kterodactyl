# Kterodactyl

## What This Is

A Kubernetes-native game server management panel — a modern, open-source alternative to Pterodactyl that replaces Wings, Docker, and Postgres with CRDs, a custom operator, and k8s-native primitives. Admins install via Helm chart, users pick a game, configure it, and get a running server at `<game>.<username>.domain.com`. Community-contributed game definitions make adding new games as simple as opening a PR.

Shipped as a single Go binary with embedded React SPA, installing via `helm install` with 50+ configurable values. Backed by a CI pipeline running Go unit/integration tests, Playwright E2E tests against a kind cluster, and unified GitHub Actions workflow.

## Core Value

Admins can deploy a single Helm chart and give their users self-service game server provisioning backed entirely by Kubernetes — no separate daemon, no Docker-in-Docker, no external database required.

## Requirements

### Validated

- ✓ Custom k8s operator (Go) with GameServer CRD that reconciles game server instances — v1.0
- ✓ Declarative game definition framework — folder-per-game with Dockerfile, parameter manifest, and metadata — v1.0
- ✓ Go REST API serving the frontend and managing operator interactions — v1.0
- ✓ React web UI where users browse games, configure parameters, launch servers, and get connection info — v1.0
- ✓ Dynamic UI driven by game parameter manifests (RJSF forms from JSON Schema) — v1.0
- ✓ User flow: pick game → configure → launch → wait → get connection info — v1.0
- ✓ DNS pattern: `<game>.<username>.domain.com` via Gateway API HTTPRoute — v1.0
- ✓ Auth: admin invite + basic signup with JWT sessions — v1.0
- ✓ Global resource limits (admin-configured max servers, CPU/RAM per server) — v1.0
- ✓ Prometheus metrics exposed for operator and API server — v1.0
- ✓ Backup system: on-demand + scheduled, stored in S3-compatible storage — v1.0
- ✓ Helm chart with opinionated defaults, configurable for Gateway API, storage classes, domain — v1.0
- ✓ Docusaurus documentation site with installation, usage, and reference — v1.0
- ✓ Mod support: users upload mods mounted via PersistentVolumes, server restarts on apply — v1.0
- ✓ WebSocket console with real-time log streaming and command execution — v1.0
- ✓ Namespace isolation with ResourceQuotas, LimitRanges, and NetworkPolicies — v1.0
- ✓ Kind-based test environment for reproducible local and CI testing — v1.1
- ✓ Playwright E2E tests covering happy-path user flows (auth, server CRUD) — v1.1
- ✓ Go API integration tests (httptest-based handler + lifecycle tests) — v1.1
- ✓ GitHub Actions CI pipeline running full test suite on PRs — v1.1

### Active

- [ ] Go test coverage reporting in CI
- [ ] Test backlog documenting untested features for future milestones
- [ ] Community game definition repo with PR-based contribution model
- [ ] OIDC/SSO integration (Google, Steam, Apple via Dex/Keycloak)

### Future

- Community game definition repo with PR-based contribution model
- OIDC/SSO integration (Google, Steam, Apple via Dex/Keycloak)
- Subuser RBAC (share server access with friends, granular permissions)
- Web-based file manager (edit configs in browser, upload/download)
- User can download their own backups
- Scheduled tasks (automate restarts, commands on schedule)

### Out of Scope

- Windows-native deployment — k8s only
- Built-in billing/payment system — admin manages monetization externally
- Real-time voice/chat between players — game servers handle this themselves
- Mobile app — web UI only for v1
- Running game servers outside Kubernetes — the entire value prop is k8s-native
- Multi-region orchestration — K8s federation complexity; run multiple panels instead
- Built-in mod installer UI — game-specific; mount mod directories instead
- Per-user resource quotas — global limits sufficient for v1; revisit at scale

## Context

**Shipped v1.1** with 19,133 LOC (13,210 Go + 5,923 TS/TSX) across 17 phases total.
**Tech stack:** Go (controller-runtime, chi v5), React (Vite, Tailwind, shadcn, RJSF), Docusaurus v3, Playwright, kind.
**Architecture:** Single binary — operator + API + embedded SPA. Dual controllers (GameServer + DNS) in one manager.
**Testing:** 16 Go test files (envtest + httptest), 2 Playwright E2E specs, kind cluster test environment, unified GitHub Actions CI.
**Game support:** Minecraft Java Edition ships as reference game; extensible via folder-per-game manifests with JSON Schema parameter validation.
**Infrastructure:** Talos K8s cluster, Cilium CNI, Cloudflare Tunnel for domain routing.

**Known tech debt:**
- DNS requires human testing with live Gateway API controller and ExternalDNS
- Relative path `"games/"` in cmd/main.go relies on container WORKDIR
- handleUploadMod and handleRestoreBackup bypass IsValidTransition guard
- Duplicate s3CredentialsSecretName constant across controller and API handler
- Go test coverage not yet reported in CI (COV-01 deferred from v1.1)
- No formal test backlog document (COV-02 deferred from v1.1)

## Constraints

- **Operator language**: Go — required for first-class k8s controller-runtime support
- **API language**: Go — unified backend language, shares types with operator
- **Frontend**: React (Vite) — modern, large ecosystem, good for dynamic UIs
- **Documentation**: Docusaurus — React-based, versioned, standard for OSS projects
- **Kubernetes minimum**: 1.26+ (VolumeSnapshot API, HTTPRoute support)
- **Storage**: Requires a CSI driver that supports VolumeSnapshots for backup functionality
- **DNS**: Requires wildcard DNS entry pointing to cluster ingress

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| K8s operator with CRD over standalone daemon | Leverages k8s scheduling, monitoring, RBAC natively; no separate process to manage | ✓ Good — 6-state lifecycle works well |
| Go for operator + API | Controller-runtime is Go-native; unified backend language reduces complexity | ✓ Good — single binary with embedded SPA |
| Folder-per-game declarative framework | Makes contributing games simple (Dockerfile + manifest); admins pick which games to include | ✓ Good — JSON Schema enables dynamic forms |
| Gateway API (HTTPRoute) over Ingress | Ingress retirement timeline March 2026; future-proof | ✓ Good — cleaner routing model |
| Global resource limits (not per-user quotas) for v1 | Simpler to implement; sufficient for homelab and small service use cases | ✓ Good — adequate for target audience |
| Admin invite + basic signup for v1 auth | Covers both personas without OIDC complexity; extensible later via Dex/Keycloak | ✓ Good — Argon2id + JWT proven secure |
| S3-compatible backup storage | Works with MinIO (homelab) and AWS S3/GCS (cloud); universal interface | ✓ Good — minio-go works across providers |
| Helm chart as primary install method | Standard k8s distribution; supports the customization matrix | ✓ Good — 50+ configurable values |
| Vite + React over Next.js for frontend | SPA embedded in Go binary; no SSR needed; simpler build pipeline | ✓ Good — go:embed integration clean |
| Pod RestartPolicy=Never; operator manages lifecycle | Full control over server state machine; kubelet doesn't interfere | ✓ Good — predictable state transitions |
| Dual-controller pattern in single manager | DNS controller watches same CRD; Named() disambiguation | ✓ Good — no inter-process communication |
| Operator-driven backup over CronJob | Avoids cross-namespace credential distribution; simpler security model | ✓ Good — annotation-based scheduling works |
| Chi v5 router with httprate | Lightweight, composable middleware, good chi ecosystem | ✓ Good — clean middleware chains |
| RJSF for dynamic forms | Automatic form generation from JSON Schema; no custom form code per game | ✓ Good — Draft-07 validator sufficient |

## Milestones

- **v1.0 MVP** — Shipped 2026-02-13 (12 phases, 34 plans)
- **v1.1 End-to-End CI/CD Test Suite** — Shipped 2026-03-04 (5 phases, 8 plans)

See `.planning/MILESTONES.md` for full details.

---
*Last updated: 2026-03-04 after v1.1 milestone*
