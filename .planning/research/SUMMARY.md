# Project Research Summary

**Project:** Kterodactyl
**Domain:** Kubernetes-native game server management platform
**Researched:** 2026-02-09
**Confidence:** HIGH

## Executive Summary

Kterodactyl is a Kubernetes-native game server management panel positioned between enterprise solutions (Agones) and hobbyist panels (Pterodactyl). Research shows this is a greenfield opportunity: no existing solution combines Kubernetes-native architecture with hobbyist-friendly UX. The recommended approach is a Go operator built with Kubebuilder v4 controlling GameServer CRDs, paired with a Next.js 15 admin UI and Go REST API server. This architecture provides enterprise-grade scalability while maintaining simplicity for homelab users.

The critical differentiator is per-server DNS names (minecraft.alice.domain.com) combined with declarative game definitions (Dockerfile + YAML manifests). This eliminates Pterodactyl's complex "egg" system while providing superior UX over IP-based connections. Research identifies 10 critical pitfalls, most notably UDP port allocation conflicts, CRD versioning without migration, and multi-tenant resource isolation failures. The MVP must address these architectural decisions early—retrofitting is expensive.

Confidence is HIGH across all research areas due to strong source quality (official Kubernetes docs, Agones architecture, operator best practices). The main risk is Gateway API adoption timing (Ingress NGINX retired March 2026), requiring immediate implementation of HTTPRoute support. Stack maturity is excellent: Kubebuilder v4 + controller-runtime v0.23.x + Next.js 15 + React 19 represents 2026 production-ready choices.

## Key Findings

### Recommended Stack

The stack centers on Kubernetes-native Go development with modern React for UI. Kubebuilder v4 is the industry standard for operators, providing scaffolding and integration with controller-runtime v0.23.x for reconciliation patterns. Gin (v1.10+) offers the best balance of maturity, performance (34k req/s), and developer experience for the REST API layer. Next.js 15 with React 19 provides server components, streaming, and optimized bundling essential for dynamic admin UIs.

**Core technologies:**
- **Kubebuilder v4.x**: Operator scaffolding and development — kubernetes-sigs standard with robust controller-runtime integration and excellent community support
- **Gin v1.10+**: REST API framework — 81k GitHub stars, 48% Go market share, mature ecosystem with gentler learning curve than alternatives
- **Next.js 15 + React 19**: Admin UI framework — production-ready with server components, auto-memoization, built-in optimizations for dynamic dashboards
- **Gateway API (HTTPRoute)**: Kubernetes networking — Ingress NGINX retired March 2026, HTTPRoute GA provides role-based design and better protocol support
- **Helm v4.0+**: Packaging and distribution — latest major version with improved CRD support, essential for complex operator deployments
- **Velero + MinIO**: Backup and restore — CNCF-standard backup with S3-compatible on-prem storage for air-gapped and cost-sensitive deployments
- **Prometheus Operator**: Observability — declarative ServiceMonitor CRDs for operator and game server metrics

**Critical version notes:**
- Go 1.24.6+ required (Kubebuilder v4 dependency)
- Kubernetes 1.30+ target (Kubebuilder v4 focus, 1.28+ supported with limitations)
- Gateway API requires Kubernetes 1.27+ for HTTPRoute GA
- Avoid Ingress API entirely (end-of-life November 2026 for security updates)

### Expected Features

Research analyzed 5 competitor categories: Pterodactyl (dominant OSS), AMP/TCAdmin (commercial), Agones (enterprise K8s), WindowsGSM/LinuxGSM (CLI tools). Table stakes are well-established: server lifecycle management, real-time console, file manager, SFTP, resource monitoring, and scheduled tasks. Users assume these exist—missing them makes the product feel incomplete.

**Must have (table stakes):**
- **Server lifecycle management** — start/stop/restart with state machine (Creating → Ready → Allocated → Shutdown)
- **Real-time console access** — WebSocket-based with granular permissions, standard across all panels
- **File manager (web-based)** — edit configs without SFTP knowledge, in-browser editing and upload/download
- **Resource monitoring** — real-time CPU/RAM/disk metrics display, live graphs expected
- **Basic user management** — create users, assign to servers, even single-tenant hobby use expects owner + friends access
- **Manual backup creation** — on-demand backups to local or remote storage

**Should have (competitive differentiators):**
- **Kubernetes-native architecture** — true cloud-native scaling, GitOps-ready CRDs, Agones does this for enterprise but no hobbyist panel exists
- **Per-server DNS names** — minecraft.alice.domain.com vs IP:port, massive UX win that no competitor provides
- **Declarative game definitions** — Dockerfile + YAML vs Pterodactyl's complex JSON eggs, enables community contributions with low barrier
- **Prometheus metrics export** — native observability for ops teams, Agones has this but traditional panels lack it
- **S3-compatible backup storage** — modern cloud storage integration (MinIO for on-prem, S3/GCS/Azure for cloud)
- **GitOps compatibility** — manage servers via kubectl apply, appeals to DevOps users

**Defer (v2+):**
- **Multi-tenancy (organizations)** — full isolation between user groups, valuable for larger deployments
- **Mod manager UI** — in-panel Workshop/Nexus integration is game-specific and maintenance-heavy
- **Fleet autoscaling** — Agones-style dynamic scaling, adds significant complexity
- **Billing integration** — anti-feature for core panel, document API for third-party billing instead

**Anti-features to avoid:**
- Built-in billing/WHMCS integration (massive scope, turns panel into hosting platform)
- Visual node editor for configs (breaks GitOps declarative model)
- Multi-region cluster orchestration (Kubernetes federation complexity rarely needed)

### Architecture Approach

The architecture follows Agones patterns adapted for hobbyist use cases. A monorepo structure (following Agones) enables shared Go types between operator and API server, unified releases, and simplified local development. Three primary layers: operator (controllers watching CRDs), API server (authentication gateway to K8s API), and frontend (Next.js UI consuming REST API).

**Major components:**
1. **GameServer Controller** — reconciles GameServer CRDs, manages Pod lifecycle, implements state machine (Creating → Ready → Allocated → Shutdown), uses level-based reconciliation with event filtering predicates
2. **API Server (Go + Gin)** — authentication/authorization layer, validates JWT tokens, loads game manifests from declarative definitions, creates GameServer CRs via controller-runtime client, never exposes K8s API directly
3. **DNS Controller** — watches GameServer CRDs, creates Ingress/HTTPRoute resources for wildcard subdomain routing (game.username.domain.com pattern), integrates with ExternalDNS for automatic DNS record creation
4. **Backup Controller** — manages S3 upload/download operations, creates CronJobs for scheduled backups, implements finalizers for cleanup, Velero-inspired pattern
5. **Frontend (Next.js 15)** — dynamic form generation from JSON schemas (game parameter manifests drive UI), React Server Components for performance, WebSocket for real-time console and status updates

**Critical architectural patterns:**
- **Reconciliation loop (control theory)** — level-based desired state enforcement, not event-driven handlers, ensures self-healing and eventual consistency
- **Event filtering with predicates** — 40%+ reduction in reconciliation load by filtering status updates, only reconcile on spec.generation changes and deletes
- **State machine for lifecycle** — explicit transitions prevent invalid states, enables allocation logic, mirrors Agones proven design
- **Finalizers for external cleanup** — ensures S3 backups and external DNS records deleted before CR removal, prevents orphaned resources
- **Status conditions for observability** — standard Kubernetes Conditions enable kubectl/Prometheus monitoring without custom parsing

### Critical Pitfalls

Research identified 10 critical pitfalls from Kubernetes operator development, Agones production experience, and game server hosting domain. Most stem from underestimating Kubernetes' eventual consistency model or misunderstanding CRD lifecycle management.

1. **CRD Storage Version Removal Without Migration** — removing a stored CRD version causes immediate data loss; all existing resources become inaccessible. Requires StorageVersionMigration strategy from Phase 1, cannot retrofit after v1alpha1 → v1beta1 transition. Prevention: implement hub-and-spoke conversion model early.

2. **UDP Port Allocation Conflicts** — Kubernetes lacks automatic hostPort assignment, causing bind failures when multiple game servers request same port on same node. Requires dynamic port allocation strategy (NodePortPool CRD tracking per-node allocations) designed before GameServer reconciliation implementation in Phase 2.

3. **Non-Idempotent Reconciliation Logic** — controllers that aren't idempotent create duplicate resources, fail on retries, get stuck in crash loops. Must use CreateOrUpdate() pattern, always reconcile ALL resources regardless of triggering event. Code review checklist item for Phase 1.

4. **Multi-Tenant Resource Isolation Failures** — namespace-only isolation insufficient for production, one user's server becomes "noisy neighbor." Requires ResourceQuotas per namespace, LimitRanges for defaults, NetworkPolicies preventing cross-namespace pod communication. Implement namespace-per-user pattern in Phase 1, tier-based allocation in Phase 3.

5. **Prometheus Cardinality Explosion** — high-cardinality labels (user IDs, pod names, session IDs) create millions of time series, crashing Prometheus. Must use low-cardinality labels only (game_type, server_state, user_tier), design metrics schema with cardinality budgets before Phase 3 observability implementation.

6. **Operator Leader Election Split-Brain** — two replicas both thinking they're leader causes duplicate reconciliations and resource thrashing. Configure tolerations with short tolerationSeconds (30-60s), set lease duration 15s with 10s renew deadline. Production-ready leader election timings required from Phase 1.

7. **Graceful Shutdown State Loss** — game servers lose player progress on termination when SIGTERM not handled or terminationGracePeriodSeconds insufficient. Requires preStop hooks, appropriate grace periods per game type (60-120s for stateful games), readiness probe returning false during shutdown. Document and test SIGTERM handling in Phase 2 game definitions.

8. **Wildcard DNS + Cert-Manager Conflicts** — ExternalDNS and cert-manager create conflicting TXT records for ACME challenges. Requires unique ownership IDs, DNS provider with recursive challenge support, split-horizon configuration with external resolver. Design DNS and certificate strategy upfront in Phase 2.

9. **Over-Templatized Helm Charts** — excessive conditionals create unmaintainability nightmares, debugging template errors takes hours. Keep templates simple, push variability to values.yaml, resist "maybe we'll need this" features. Start minimal in Phase 2 chart, add complexity only when proven necessary.

10. **CRD Data vs Application Data Confusion** — storing user passwords or billing info in GameServer CRs violates etcd design (size limits, no ACID, poor query performance). CRDs only for operational state (game type, resources, status), application data in PostgreSQL. Architecture boundary established in Phase 1.

## Implications for Roadmap

Research suggests 7-phase structure based on dependency analysis from ARCHITECTURE.md and critical path identification. Early phases establish operator foundation and address architectural pitfalls that are expensive to retrofit. Middle phases add competitive features and complete table stakes. Later phases implement advanced features once core value is validated.

### Phase 1: Core Operator Foundation
**Rationale:** GameServer CRD and basic controller must exist before any other components can function. This phase establishes architectural decisions (CRD versioning strategy, reconciliation patterns, leader election) that are difficult to change later. Implements namespace-per-user pattern for multi-tenant isolation.

**Delivers:** Kubebuilder v4 project scaffold, GameServer CRD with v1alpha1 API, basic reconciliation loop (creates Pods for GameServers), state machine implementation (Creating → Ready → Shutdown), status conditions for observability, conversion webhook infrastructure.

**Addresses:**
- CRD versioning without migration (Pitfall 1) — scaffold conversion webhooks from start
- Non-idempotent reconciliation (Pitfall 3) — implement CreateOrUpdate pattern, code review checklist
- Leader election split-brain (Pitfall 6) — production-ready timings (30s tolerations, 15s lease)
- Multi-tenant isolation foundation (Pitfall 4) — namespace-per-user, ResourceQuota validation

**Critical decisions in this phase:**
- CRD API design (versioning strategy affects entire lifecycle)
- State machine states and valid transitions
- Controller concurrency and rate limiting settings

### Phase 2: Networking and DNS
**Rationale:** Per-server DNS is the core differentiator. Must be implemented early to validate Kubernetes-native approach superiority over IP-based connection strings. Port allocation strategy must be designed before GameServer reconciliation complexity grows.

**Delivers:** DNS Controller creating Ingress/HTTPRoute resources, wildcard subdomain routing (game.username.domain.com), ExternalDNS integration, dynamic port allocation strategy (NodePortPool or equivalent), Gateway API HTTPRoute support (Ingress as fallback only).

**Addresses:**
- UDP port allocation conflicts (Pitfall 2) — implement dynamic port allocation with per-node tracking
- Wildcard DNS + cert-manager conflicts (Pitfall 8) — unique ownership IDs, split-horizon configuration
- Gateway API migration (Stack critical note) — HTTPRoute as default, Ingress deprecated path

**Critical decisions in this phase:**
- Port allocation strategy (dynamic pool vs fixed ranges)
- DNS pattern (*.user.domain vs per-server subdomains)
- Certificate management approach (wildcard per user vs per server)

**Research flag:** May need /gsd:research-phase for ExternalDNS + cert-manager integration patterns in production environments.

### Phase 3: API Server Bridge
**Rationale:** API server acts as user-facing gateway to Kubernetes. Must exist after CRDs and networking established but before frontend implementation. Authentication and authorization decisions made here affect all user-facing features.

**Delivers:** Go REST API server with Gin framework, Kubernetes client integration (controller-runtime client.Client), JWT authentication and validation, game manifest loading from games/ directory, GameServer CRUD endpoints, user context and namespace mapping.

**Addresses:**
- CRD data vs application data (Pitfall 10) — PostgreSQL for user/auth data, K8s API only for operational state
- API server as K8s client gateway (Architecture Pattern 4) — never expose K8s API directly
- Multi-tenant authorization — user can only access their namespace

**Critical decisions in this phase:**
- Authentication mechanism (JWT, OIDC, both?)
- User-to-namespace mapping strategy
- Rate limiting and quota enforcement layer

### Phase 4: Game Definition Framework
**Rationale:** Declarative game definitions enable community contributions and differentiate from Pterodactyl's complex egg system. Must exist before frontend can generate dynamic forms. Single game (Minecraft) sufficient for MVP validation.

**Delivers:** games/ directory structure with Dockerfile + manifest.yaml per game, JSON schema for parameter validation, Minecraft as reference implementation, documentation for contributing new games.

**Addresses:**
- Simple community contributions (Feature differentiator) — low-barrier PR process vs complex JSON
- Dynamic form generation foundation (Architecture Pattern 5) — JSON schemas drive UI

**Critical decisions in this phase:**
- Manifest schema design (parameters, ports, env vars, resource hints)
- Game versioning strategy
- Licensing metadata requirements (Pitfall: SteamCMD legal compliance)

**Research flag:** Standard patterns for game definitions, skip /gsd:research-phase.

### Phase 5: Frontend (Admin UI)
**Rationale:** Frontend consumes API server endpoints and provides user-facing UX. Comes after API server and game definitions exist to enable dynamic form generation from manifests.

**Delivers:** Next.js 15 scaffold with App Router, JSON schema to form conversion (react-hook-form + zod), game server list/detail views, create game server form (dynamic based on game type), real-time status updates via WebSocket or polling.

**Addresses:**
- Dynamic forms from JSON schema (Architecture Pattern 5) — eliminates per-game frontend code
- Table stakes UX (Feature research) — server lifecycle, resource monitoring, connection info display
- UX pitfalls from research — detailed status conditions, DNS propagation visibility, user-friendly errors

**Critical decisions in this phase:**
- Form generation library (react-hook-form + zod vs alternatives)
- Real-time update mechanism (WebSocket vs SSE vs polling)
- State management strategy (React Query, Zustand, or built-in)

### Phase 6: Backup and Persistence
**Rationale:** Manual backups are table stakes. S3-compatible storage is differentiator over local-only. Comes after core functionality works to enable backup testing with real game servers.

**Delivers:** Backup CRD with controller, S3 upload/download operations (AWS SDK + MinIO compatibility), CronJob integration for scheduled backups, finalizers for cleanup (delete S3 objects on Backup CR deletion), on-demand backup API endpoint, backup listing in UI.

**Addresses:**
- Manual backup creation (Feature table stakes)
- S3-compatible storage (Feature differentiator)
- Finalizers for external cleanup (Architecture Pattern 6)
- Graceful shutdown integration (Pitfall 7) — operator-triggered backup on termination

**Critical decisions in this phase:**
- Backup format (tar.gz, Velero-compatible, custom?)
- Retention policy implementation
- Restore operation UX (API-driven vs manual)

**Research flag:** Standard patterns from Velero, skip /gsd:research-phase.

### Phase 7: Observability and Metrics
**Rationale:** Prometheus metrics differentiate from traditional panels and enable cluster operator buy-in. Must come after core features work to avoid premature optimization. Metrics schema designed with cardinality budgets prevents Pitfall 5.

**Delivers:** Prometheus metrics endpoints on operator and API server, controller-runtime default metrics + custom game server metrics (game_servers_total by state/type, reconciliation latency), ServiceMonitor CRDs for Prometheus Operator, low-cardinality label design (game_type, server_state, user_tier — NOT user_id or pod_name).

**Addresses:**
- Prometheus metrics export (Feature differentiator)
- Cardinality explosion (Pitfall 5) — design metrics schema with bounded cardinality
- Observability best practices (Architecture) — status conditions + metrics enable monitoring

**Critical decisions in this phase:**
- Metric naming conventions
- Label cardinality budget per metric
- Grafana dashboard strategy (bundled vs user-created)

**Research flag:** Standard patterns from Operator SDK observability docs, skip /gsd:research-phase.

### Phase 8: Helm Chart and Packaging
**Rationale:** Helm chart packages entire system for distribution. Comes after all components exist and integration tested. Simple chart initially, resist over-templating (Pitfall 6).

**Delivers:** Helm v4.0 chart with crds/ directory for CRD installation, operator deployment with leader election, API server deployment, configurable networking (Ingress vs HTTPRoute toggle), values.yaml with clear documentation, hooks for database migrations if needed.

**Addresses:**
- Over-templatized Helm charts (Pitfall 6) — start minimal, add complexity only when necessary
- CRDs in crds/ directory (Architecture anti-pattern 7) — proper installation ordering

**Critical decisions in this phase:**
- Chart complexity boundaries (what's configurable vs hardcoded)
- Sub-chart strategy (monolithic vs operator + UI split)
- Upgrade strategy for CRDs

**Research flag:** Standard patterns from Helm docs, skip /gsd:research-phase.

### Phase Ordering Rationale

**Dependency-driven sequencing:**
- GameServer CRD must exist before DNS Controller can watch it (Phase 1 → 2)
- API server requires CRDs and game manifests to create resources (Phase 1, 4 → 3)
- Frontend consumes API endpoints and game schemas (Phase 3, 4 → 5)
- Backup requires running game servers to test against (Phase 1 → 6)
- Metrics are last because they require stable components to instrument (all → 7)
- Helm packages completed system (all → 8)

**Pitfall-driven ordering:**
- CRD versioning strategy must be Phase 1 (Pitfall 1) — retrofitting conversion webhooks is expensive
- Port allocation design must be Phase 2 (Pitfall 2) — affects core reconciliation logic
- Multi-tenant isolation (namespace pattern, ResourceQuotas) must be Phase 1 (Pitfall 4) — architectural foundation
- Cardinality budget must be before metrics implementation (Phase 7) to prevent explosion (Pitfall 5)

**Value-driven acceleration:**
- Phases 1-5 deliver MVP — user can create Minecraft server with DNS name via web UI
- Phase 2 (DNS) prioritized early because it's the core differentiator vs competitors
- Phase 4 (game definitions) before Phase 5 (frontend) enables dynamic forms without per-game code

### Research Flags

**Phases likely needing /gsd:research-phase during planning:**
- **Phase 2 (Networking):** ExternalDNS + cert-manager integration patterns may require research for split-horizon DNS and wildcard certificate edge cases
- **Phase 3 (API Server):** OIDC provider integration if beyond basic JWT needs research on provider-specific quirks (Dex, Keycloak, Auth0)

**Phases with standard patterns (skip research-phase):**
- **Phase 1 (Operator Core):** Kubebuilder documentation is excellent, operator patterns well-established
- **Phase 4 (Game Definitions):** Dockerfile + YAML is straightforward, no novel integration
- **Phase 5 (Frontend):** Next.js and React Hook Form documentation comprehensive, JSON schema conversion libraries mature
- **Phase 6 (Backup):** Velero patterns directly applicable, S3 SDK well-documented
- **Phase 7 (Observability):** Prometheus Operator patterns standard, controller-runtime metrics documented
- **Phase 8 (Helm):** Helm best practices well-documented, operator Helm patterns established

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | **HIGH** | All core technologies verified via official documentation: Kubebuilder v4 from kubernetes-sigs GitHub, Gin from official benchmarks and community usage data, Next.js 15 + React 19 from official release notes, Gateway API retirement timeline confirmed by Kubernetes SIG Network and Microsoft Azure blog |
| Features | **MEDIUM-HIGH** | Table stakes and differentiators cross-referenced across 5+ competitor categories (Pterodactyl, AMP, TCAdmin, Agones, WindowsGSM). Pterodactyl and Agones feature sets confirmed via official docs and GitHub. Commercial panels (AMP/TCAdmin) less transparent but confirmed via hosting provider reviews. "Expected" varies by user segment (hobbyist vs commercial host) |
| Architecture | **HIGH** | Architecture patterns verified via official Kubernetes operator documentation, Kubebuilder Book, Operator SDK best practices, and Agones production architecture (Google-backed with multi-year track record). Monorepo structure confirmed via Agones repository analysis. Controller-runtime patterns from official docs and production operator case studies |
| Pitfalls | **HIGH** | Pitfalls sourced from Kubernetes operator best practices (official), Agones troubleshooting documentation (production experience), operator SDK common recommendations, and multi-tenant security guides. CRD versioning pitfalls from kubernetes/community API conventions. Port allocation issues from Agones networking documentation and Alibaba Cloud production reports |

**Overall confidence:** **HIGH**

Research quality is strong across all areas due to authoritative sources: kubernetes-sigs projects (Kubebuilder, Gateway API), CNCF projects (Agones, Velero, Prometheus), official framework documentation (Next.js, Gin), and production operator case studies. The main limitation is commercial panel features (AMP/TCAdmin) relying on secondary sources, but competitive analysis triangulates findings across multiple panel types.

### Gaps to Address

**During planning:**
- **Port allocation implementation details** — research identifies the pitfall but specific implementation (NodePortPool CRD vs other strategies) needs design during Phase 2 planning. Agones uses DynamicPort strategy which is one proven approach.
- **User authentication scope** — basic JWT is clear, but OIDC provider integration specifics depend on deployment environment (cloud vs on-prem, which IdP). Defer detailed OIDC research to Phase 3 unless initial requirements specify it.
- **Game definition licensing metadata** — research flags SteamCMD legal compliance but schema design needs legal review during Phase 4. Some games prohibit specific hosting types.

**During execution:**
- **Split-horizon DNS testing** — research identifies cert-manager + ExternalDNS conflicts in split-horizon setups, but production testing required during Phase 2 to validate configuration. Use external DNS resolver (8.8.8.8) for cert-manager challenges.
- **Cardinality monitoring setup** — research provides cardinality budgets and anti-patterns, but specific thresholds need tuning based on actual deployment scale during Phase 7. Use Grafana Cardinality Explorer dashboard (ID 11304).
- **Helm chart complexity boundaries** — research warns against over-templating but specific "what to make configurable" decisions depend on real deployment feedback during Phase 8. Start minimal, add only proven-necessary options.

## Sources

### Primary (HIGH confidence)

**Kubernetes and Operators:**
- Official Kubernetes documentation (CRDs, Operator pattern, Multi-tenancy, RBAC, Finalizers)
- kubernetes-sigs/kubebuilder GitHub repository and official Kubebuilder Book
- Operator SDK official documentation (best practices, observability, event filtering)
- kubernetes/community API conventions (status conditions, versioning)
- controller-runtime documentation and implementation guides

**Gateway API and Networking:**
- Official Gateway API documentation (HTTPRoute GA, migration from Ingress)
- Kubernetes SIG Network announcements (Ingress retirement timeline)
- Microsoft Azure blog: "From Ingress to Gateway API" (March 2026 retirement confirmation)
- ExternalDNS documentation and integration patterns
- cert-manager official documentation

**Agones (Game Server Reference Architecture):**
- Official Agones documentation (overview, architecture, FAQ, troubleshooting)
- Agones GitHub repository (architecture analysis, production patterns)
- Google Cloud blog: "Introducing Agones" (design rationale)
- Alibaba Cloud: "Agones Series Part 2: Address and Port of Game Server" (networking pitfalls)

**Stack Technologies:**
- Official Go documentation (slog, standard library)
- Gin GitHub repository (81k stars, performance benchmarks)
- Next.js official documentation (v15 release notes, React 19 support)
- React official documentation (v19 features, server components)
- Helm official documentation (v4.0 release, CRD best practices, chart hooks)
- Velero documentation (CNCF project, MinIO integration)
- Prometheus Operator documentation (ServiceMonitor CRDs)

### Secondary (MEDIUM confidence)

**Competitor Analysis:**
- Pterodactyl official site, GitHub repository, and documentation (introduction, egg creation)
- AMP (CubeCoders) official site and feature documentation
- TCAdmin official site and documentation
- WindowsGSM and LinuxGSM GitHub repositories
- Comparison articles: "AMP vs Pterodactyl" (Atomic Networks), "Best Pterodactyl Alternatives" (SatisfyHost)

**Technology Comparisons:**
- Medium: "Top 6 Go Web Frameworks for 2025" (Gin market share data)
- LogRocket: "Best Go Frameworks 2025"
- Medium: "Gin vs Fiber vs Echo Performance" (benchmark comparisons)
- Next.js Templates: "Admin Dashboard Templates 2026" (shadcn/ui ecosystem)

**Best Practices and Patterns:**
- ITNEXT: "Developing Kubernetes Operators"
- OuterByte: "Kubernetes Operators 2025 Guide"
- Medium: "Kubernetes Controllers at Scale" (controller-runtime deep dive)
- DEV: "Beyond YAML: Building Kubernetes Operators with CRDs"
- Carlos Neto: "Helm Best Practices" (2025 update)

### Tertiary (LOW confidence)

**Pitfall Identification:**
- DEV Community: "Kubernetes CRD: the versioning joy" (versioning pitfalls, community experience)
- Medium: "Implementing Leader Election in Kubernetes" (split-brain scenarios)
- Grafana Labs blog: "Managing High Cardinality Metrics" (cardinality explosion patterns)
- Fairwinds blog: "Hands-On With Agones" (production lessons learned)

**Emerging Patterns:**
- Patterns.dev: "React Stack Patterns" (2026 frontend trends)
- Codup: "Building Dynamic Forms in React with JSON Schema" (form generation patterns)
- Various technical blogs on Kubernetes backup strategies, monitoring approaches, multi-tenancy

---
*Research completed: 2026-02-09*
*Ready for roadmap: yes*
