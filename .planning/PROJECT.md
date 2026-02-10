# Kterodactyl

## What This Is

A Kubernetes-native game server management panel — a modern, open-source alternative to Pterodactyl that replaces Wings, Docker, and Postgres with CRDs, a custom operator, and k8s-native primitives. Admins install via Helm chart, users pick a game, configure it, and get a running server at `<game>.<username>.domain.com`. Community-contributed game definitions make adding new games as simple as opening a PR.

## Core Value

Admins can deploy a single Helm chart and give their users self-service game server provisioning backed entirely by Kubernetes — no separate daemon, no Docker-in-Docker, no external database required.

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] Custom k8s operator (Go) with GameServer CRD that reconciles game server instances
- [ ] Declarative game definition framework — folder-per-game with Dockerfile, parameter manifest, and metadata
- [ ] Community game definition repo with PR-based contribution model
- [ ] Go REST API serving the frontend and managing operator interactions
- [ ] React/Next.js web UI where users browse games, configure parameters, launch servers, and get connection info
- [ ] Dynamic UI driven by game parameter manifests (ban lists, admin commands, world settings, etc.)
- [ ] User flow: pick game → configure → launch → wait → get connection info
- [ ] DNS pattern: `<game>.<username>.domain.com` via Ingress or HTTPRoute
- [ ] Auth: admin invite + basic signup for v1, extensible to OIDC/social login
- [ ] Global resource limits (admin-configured max servers, CPU/RAM per server)
- [ ] Prometheus metrics exposed for all operator and server activity
- [ ] Backup system: on-demand + scheduled, stored in S3-compatible storage
- [ ] Helm chart with opinionated defaults, customizable for Ingress vs HTTPRoute, auth backends, storage classes, etc.
- [ ] Docusaurus documentation site as first-class citizen, generated/linked from codebase
- [ ] Mod support: users upload mods mounted via PersistentVolumes, server restarts on apply
- [ ] User log viewing and self-service backup downloads (v2)

### Out of Scope

- Windows-native deployment — k8s only
- Built-in billing/payment system — admin manages monetization externally
- Real-time voice/chat between players — game servers handle this themselves
- Mobile app — web UI only for v1
- Running game servers outside Kubernetes — the entire value prop is k8s-native

## Context

**Pterodactyl's limitations:** Pterodactyl relies on Wings (a custom daemon), Docker directly, and Postgres. This means operators run a separate control plane outside k8s, can't leverage k8s scheduling/scaling/monitoring, and have to manage Wings alongside their cluster. Kterodactyl eliminates this by making everything a k8s resource.

**Target users:** Two key personas:
1. **Homelab admins** — friends who want to install Kterodactyl and invite a handful of friends to host game servers (2-10 users, 5-20 servers)
2. **Service operators** — running Kterodactyl as a revenue-generating service with potentially hundreds of users and servers

**Game server hosting model:** Most dedicated game servers require owning the game on Steam (via SteamCMD). Some admins may only host 2-3 games, others may host dozens. The declarative game framework must support both — admins include only the game definitions they want.

**Open source first:** This is a community project. The Helm chart, game definitions, documentation, and all code are open source. Extensibility and customization are not afterthoughts.

## Constraints

- **Operator language**: Go — required for first-class k8s controller-runtime support
- **API language**: Go — unified backend language, shares types with operator
- **Frontend**: React/Next.js — modern, large ecosystem, good for dynamic UIs
- **Documentation**: Docusaurus — React-based, versioned, standard for OSS projects
- **Kubernetes minimum**: 1.26+ (VolumeSnapshot API, HTTPRoute support)
- **Storage**: Requires a CSI driver that supports VolumeSnapshots for backup functionality
- **DNS**: Requires wildcard DNS entry pointing to cluster ingress

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| K8s operator with CRD over standalone daemon | Leverages k8s scheduling, monitoring, RBAC natively; no separate process to manage | — Pending |
| Go for operator + API | Controller-runtime is Go-native; unified backend language reduces complexity | — Pending |
| Folder-per-game declarative framework | Makes contributing games simple (Dockerfile + manifest); admins pick which games to include | — Pending |
| Community game defs via PR to main repo | Centralizes quality control; builds ecosystem; game defs are reviewed before merge | — Pending |
| Global resource limits (not per-user quotas) for v1 | Simpler to implement; sufficient for homelab and small service use cases | — Pending |
| Admin invite + basic signup for v1 auth | Covers both personas without OIDC complexity; extensible later via Dex/Keycloak | — Pending |
| S3-compatible backup storage | Works with MinIO (homelab) and AWS S3/GCS (cloud); universal interface | — Pending |
| Helm chart as primary install method | Standard k8s distribution; supports the customization matrix (ingress, auth, storage) | — Pending |

---
*Last updated: 2026-02-09 after initialization*
