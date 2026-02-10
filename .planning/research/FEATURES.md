# Feature Research: Game Server Management Panels

**Domain:** Kubernetes-native game server management panel
**Researched:** 2026-02-09
**Confidence:** MEDIUM

## Feature Landscape

### Table Stakes (Users Expect These)

Features users assume exist. Missing these = product feels incomplete.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Server Lifecycle Management** | Core function - start/stop/restart servers | LOW | Docker containers make this straightforward; Pterodactyl, AMP, TCAdmin all provide this |
| **Real-time Console Access** | Users need to execute commands and see output | MEDIUM | WebSocket-based console is standard; permissions must be granular |
| **File Manager (Web-based)** | Edit configs without SFTP knowledge | MEDIUM | In-browser editing, upload/download, search; all major panels have this |
| **SFTP Access** | Power users expect direct file access | LOW | Standard protocol, simple to expose with user credentials |
| **Resource Monitoring** | See CPU/RAM/disk usage in real-time | MEDIUM | Real-time metrics display; Pterodactyl and AMP provide live graphs |
| **Basic User Management** | Create users, assign to servers | LOW | Single-tenant hobby use expects at least owner + friends access |
| **Game Configuration UI** | Adjust settings without editing files | MEDIUM | Game-specific forms for common parameters; Pterodactyl eggs define these |
| **Server Installation** | One-click game server deployment | MEDIUM | Requires game definitions (Docker images + startup configs) |
| **Scheduled Tasks** | Automate restarts, backups, commands | MEDIUM | Cron-like scheduling; Pterodactyl, AMP, TCAdmin all support this |
| **Backup Creation** | Manual backup on-demand | MEDIUM | Compress server files, store locally or remotely |
| **Connection Information** | Easy-to-find IP:Port for players | LOW | Display prominently; Kterodactyl adds DNS names for better UX |

### Differentiators (Competitive Advantage)

Features that set the product apart. Not required, but valued.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Kubernetes-native Architecture** | True cloud-native scaling, GitOps-ready | HIGH | Kterodactyl's core differentiator; Agones does this for enterprise, but no hobbyist panel exists |
| **Declarative Game Definitions** | Community can PR new games easily | MEDIUM | Dockerfile + YAML manifest vs Pterodactyl's complex egg JSON; simpler contribution model |
| **Per-server DNS Names** | `minecraft.alice.domain.com` vs remembering IPs | MEDIUM | Massive UX win; no competitor does this; requires ingress/external-dns integration |
| **GitOps-Compatible CRDs** | Manage servers via `kubectl apply` | LOW | Automatic with CRD design; appeals to DevOps users |
| **Prometheus Metrics Export** | Native observability for ops teams | MEDIUM | Agones has this; traditional panels lack it; critical for cluster operators |
| **S3-Compatible Backup Storage** | Modern cloud storage vs local-only | MEDIUM | AMP/Pterodactyl support this, but not standard; important for cloud deployments |
| **Mod Support via PersistentVolumes** | Workshop/Nexus mods persist across restarts | MEDIUM | Separate PV for mods directory; cleaner than embedding in server data |
| **Custom Resource Limits (K8s)** | Enforce CPU/RAM per-server with K8s primitives | LOW | ResourceQuotas/LimitRanges; native to K8s, but novel for game panels |
| **Open-Source First** | No licensing costs, forkable, community-driven | N/A | Pterodactyl is OSS but complex; TCAdmin/AMP are paid; Agones is enterprise-focused |
| **Single Binary Operator** | Easy deployment vs multi-service stack | MEDIUM | Go operator compiles to single binary; simpler than Pterodactyl's PHP+Node+Go stack |

### Anti-Features (Commonly Requested, Often Problematic)

Features that seem good but create problems.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| **Billing Integration** | Commercial hosts want WHMCS/Blesta | Adds massive scope; turns panel into hosting platform | Document API for third-party billing; focus on core panel |
| **Built-in Mod Installer UI** | "One-click Workshop mods" | Game-specific; 100+ games = 100+ integrations; maintenance nightmare | Document how to mount mod directories; let users handle Workshop CLI tools |
| **Visual Node Editor** | "Drag-drop server configs" | Over-engineered for YAML editing; breaks GitOps declarative model | Provide great YAML examples + validation; CLI users will love declarative approach |
| **Multi-Region Orchestration** | "Manage servers across 5 clusters" | Kubernetes federation is complex and rarely needed | Single cluster per panel instance; users can run multiple panels |
| **Real-time Player List** | "See who's online" | Requires game-specific protocol integration for 100+ games | Expose metrics endpoint; users query via Grafana or game-specific tools |
| **In-Panel Voice Chat** | "Discord alternative" | Massive scope creep; Discord already exists | Integrate with Discord bot for notifications instead |
| **Custom DNS Management** | "Let users add TXT records" | Security nightmare; out of scope for game panel | Provide DNS pattern docs; cluster admin configures external-dns once |

## Feature Dependencies

```
Server Lifecycle Management (foundational)
    ├──requires──> GameServer CRD
    │
    └──enables──> Scheduled Tasks
                  Real-time Console Access
                  Resource Monitoring

User Management (foundational)
    ├──requires──> Authentication System
    │
    └──enables──> RBAC
                  Multi-tenancy
                  Subuser Access

Game Definitions (foundational)
    ├──requires──> Dockerfile + Manifest Schema
    │
    └──enables──> Server Installation
                  Game Configuration UI
                  Community Contributions

File Manager
    ├──requires──> SFTP Access (underlying)
    │
    └──enhances──> Game Configuration

Backup Creation
    ├──requires──> S3 Integration (optional)
    │
    └──enables──> Scheduled Backups
                  Disaster Recovery

Prometheus Metrics
    ├──requires──> Operator Instrumentation
    │
    └──enables──> Grafana Dashboards
                  Alerting
```

### Dependency Notes

- **GameServer CRD is foundational** — Without this, there's no Kubernetes-native server representation. Must be in Phase 1.
- **Authentication gates all user features** — Basic auth/signup must exist before RBAC, multi-tenancy, or subusers.
- **Game definitions enable variety** — Without declarative game definitions, only one game type is supported. Critical for community contributions.
- **SFTP is lower-level than file manager** — File manager is a UI over SFTP/file access; SFTP should exist first for power users.
- **Backup storage affects scheduling** — S3 integration should exist before automated backups to avoid local-disk-only limitation.
- **Metrics must be instrumented early** — Prometheus metrics hard to retrofit; should be in operator from start.

## MVP Definition

### Launch With (v1.0)

Minimum viable product — what's needed to validate the concept with homelab users.

- [x] **GameServer CRD + Operator** — Core Kubernetes-native infrastructure
- [x] **Single Game Support (Minecraft)** — Proves declarative game definition works
- [x] **Web UI: Create/Start/Stop/Delete Servers** — Basic lifecycle management
- [x] **Real-time Console Access** — Essential for debugging and administration
- [x] **Basic Auth + User Signup** — Simple user management (admin can invite)
- [x] **Per-server DNS Names** — Key differentiator, validates k8s-native approach
- [x] **Resource Limits (CPU/RAM/Disk)** — Global limits prevent cluster abuse
- [x] **Connection Info Display** — Show DNS + port to users
- [x] **Manual Backup Creation** — On-demand backups to local storage
- [ ] **Documentation: Installation + Game Definition Guide** — Enable early adopters

**Rationale:** This is the absolute minimum to let homelab users spin up a Minecraft server with a better UX than Pterodactyl. Validates Kubernetes-native approach and DNS differentiation.

### Add After Validation (v1.x)

Features to add once core is working and users confirm value.

- [ ] **Multi-Game Support** — Add 3-5 popular games (Valheim, Terraria, Palworld)
- [ ] **SFTP Access** — Direct file access for power users
- [ ] **Web-based File Manager** — Edit configs without SFTP client
- [ ] **Scheduled Tasks** — Automate restarts, backups
- [ ] **S3-Compatible Backup Storage** — Cloud backup support
- [ ] **Prometheus Metrics Export** — Observability for cluster operators
- [ ] **RBAC (Subusers)** — Share server access with friends
- [ ] **Game Configuration UI** — Forms for common game settings
- [ ] **Community Game Definitions (PR process)** — Accept first community contributions
- [ ] **OIDC Authentication** — SSO for users with existing identity providers

**Trigger for adding:** 10+ active homelab users successfully running servers, positive feedback on core UX, at least 2 requests for multi-game support.

### Future Consideration (v2.0+)

Features to defer until product-market fit is established.

- [ ] **Multi-tenancy (Organizations)** — Full isolation between user groups
- [ ] **Mod Manager UI** — In-panel Workshop/Nexus integration
- [ ] **Advanced RBAC Policies** — Per-resource permission granularity
- [ ] **Fleet Autoscaling** — Agones-style dynamic scaling
- [ ] **Automated Backup Rotation** — Smart retention policies
- [ ] **Audit Logging** — Compliance and security tracking
- [ ] **Webhook Integrations** — Discord, Slack, etc. notifications
- [ ] **Custom Game Definition Validation** — CI/CD for community PRs
- [ ] **High Availability Operator** — Multi-replica operator deployment

**Why defer:** These are valuable for larger deployments (hosting providers, large clans) but add significant complexity. Validate core value proposition with hobbyists first.

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority | Phase |
|---------|------------|---------------------|----------|-------|
| GameServer CRD + Operator | HIGH | HIGH | P1 | v1.0 |
| Server Lifecycle (Start/Stop) | HIGH | MEDIUM | P1 | v1.0 |
| Real-time Console | HIGH | MEDIUM | P1 | v1.0 |
| Per-server DNS | HIGH | MEDIUM | P1 | v1.0 |
| Basic Auth + Signup | HIGH | LOW | P1 | v1.0 |
| Single Game Support | HIGH | MEDIUM | P1 | v1.0 |
| Manual Backups | MEDIUM | MEDIUM | P1 | v1.0 |
| Resource Limits | HIGH | LOW | P1 | v1.0 |
| Multi-Game Support | HIGH | MEDIUM | P2 | v1.1 |
| SFTP Access | MEDIUM | LOW | P2 | v1.1 |
| File Manager (Web) | HIGH | MEDIUM | P2 | v1.2 |
| Scheduled Tasks | MEDIUM | MEDIUM | P2 | v1.2 |
| S3 Backups | MEDIUM | MEDIUM | P2 | v1.3 |
| Prometheus Metrics | MEDIUM | MEDIUM | P2 | v1.3 |
| RBAC (Subusers) | MEDIUM | HIGH | P2 | v1.4 |
| Game Config UI | MEDIUM | HIGH | P2 | v1.5 |
| OIDC Auth | LOW | MEDIUM | P3 | v2.0 |
| Multi-tenancy | LOW | HIGH | P3 | v2.0 |
| Fleet Autoscaling | LOW | HIGH | P3 | v2.1 |
| Mod Manager UI | MEDIUM | HIGH | P3 | v2.2 |

**Priority key:**
- P1: Must have for launch — validates core concept
- P2: Should have — completes feature parity with competitors
- P3: Nice to have — advanced use cases, defer until PMF

## Competitor Feature Analysis

| Feature | Pterodactyl | AMP | TCAdmin | Agones | WindowsGSM/LinuxGSM | Kterodactyl |
|---------|-------------|-----|---------|--------|---------------------|-------------|
| **Architecture** | PHP/React/Go, Docker | C#, multi-platform | .NET, Windows/Linux | Go, K8s-native | CLI tools | Go operator, K8s-native |
| **Open Source** | ✅ Yes (MIT) | ❌ Paid | ❌ Paid | ✅ Yes (Apache 2.0) | ✅ Yes | ✅ Yes (planned MIT) |
| **Server Lifecycle** | ✅ Full | ✅ Full | ✅ Full | ✅ Full | ✅ Full | ✅ Planned |
| **Real-time Console** | ✅ Yes | ✅ Yes | ✅ Yes | ❌ SDK only | ❌ CLI | ✅ Planned |
| **Web File Manager** | ✅ Yes | ✅ Yes | ✅ Yes | ❌ N/A | ❌ CLI | ✅ Planned v1.x |
| **SFTP Access** | ✅ Yes | ✅ Yes | ✅ Yes | ❌ N/A | ✅ Yes | ✅ Planned v1.x |
| **Scheduled Tasks** | ✅ Yes | ✅ Yes | ✅ Yes | ❌ N/A | ✅ Yes | ✅ Planned v1.x |
| **Backups** | ✅ Manual + Scheduled | ✅ Auto + S3 | ✅ Full | ❌ External | ✅ Manual | ✅ Planned (S3 v1.x) |
| **User Management** | ✅ Subusers + RBAC | ✅ Full RBAC | ✅ Full RBAC | ❌ K8s RBAC | ❌ Single user | ✅ Basic v1.0, RBAC v1.x |
| **Multi-tenancy** | ✅ Yes | ✅ Yes | ✅ Yes | ✅ Namespaces | ❌ No | ✅ Planned v2.0 |
| **Resource Limits** | ✅ Per-server | ✅ Per-server | ✅ Per-server | ✅ K8s native | ❌ No | ✅ K8s native v1.0 |
| **Monitoring** | ⚠️ Basic | ✅ Advanced | ✅ Advanced | ✅ Prometheus | ❌ No | ✅ Prometheus v1.x |
| **API Access** | ✅ REST API | ✅ REST API | ✅ REST API | ✅ K8s API | ❌ No | ✅ K8s API + REST (planned) |
| **Game Definitions** | ⚠️ Eggs (complex JSON) | ⚠️ Modules (proprietary) | ⚠️ Configs (proprietary) | ❌ N/A | ⚠️ Scripts (fragmented) | ✅ Dockerfile + YAML (simple) |
| **Community Contributions** | ✅ Yes (high barrier) | ❌ No | ❌ No | ❌ N/A | ⚠️ Fragmented | ✅ PR to main repo (low barrier) |
| **DNS per Server** | ❌ No | ❌ No | ❌ No | ❌ No | ❌ No | ✅ Yes (unique!) |
| **Kubernetes-native** | ❌ No (Docker) | ❌ No | ❌ No | ✅ Yes (enterprise) | ❌ No | ✅ Yes (hobbyist-focused) |
| **Billing Integration** | ⚠️ Via modules | ✅ Built-in | ✅ Built-in | ❌ N/A | ❌ No | ❌ Anti-feature |
| **Target Audience** | Hosting providers | Hosters + hobbyists | Enterprise hosters | K8s operators | Single-server admins | Homelab to clusters |

### Competitive Positioning

**Pterodactyl** is the dominant open-source panel but:
- Not Kubernetes-native (Docker-based)
- Complex multi-service architecture (PHP panel + Go daemon + React frontend)
- Egg system has high contribution barrier (complex JSON schemas)
- No built-in DNS per server

**AMP/TCAdmin** are commercial, feature-rich, but:
- Paid licensing (per-server or monthly)
- Not Kubernetes-native
- Closed-source, no community game definitions

**Agones** is Kubernetes-native but:
- Enterprise-focused (Google-backed, targets game studios)
- No web UI, no hobbyist UX
- Requires deep Kubernetes knowledge
- Lacks basic panel features (console, file manager, backups)

**WindowsGSM/LinuxGSM** are lightweight but:
- CLI-only, no web interface
- Single-server focus, no multi-tenancy
- Fragmented game support (community scripts)

**Kterodactyl's differentiation:**
- **Kubernetes-native for hobbyists** — Brings Agones-level architecture to homelab users
- **Per-server DNS names** — Unique UX feature no competitor has
- **Simple game definitions** — Dockerfile + YAML vs complex JSON/modules
- **Open-source first** — No licensing costs, community-driven
- **GitOps-ready** — Manage servers with `kubectl apply` (appeals to DevOps crowd)

## Research Confidence Assessment

| Area | Confidence | Source Quality | Notes |
|------|------------|----------------|-------|
| **Pterodactyl features** | HIGH | Official docs + GitHub + community reviews | Well-documented, mature project; feature set confirmed across multiple sources |
| **AMP features** | MEDIUM | Official site + comparison articles | Less detailed docs publicly available; confirmed via hosting provider reviews |
| **TCAdmin features** | MEDIUM | Official site + user forums | Limited public documentation; confirmed as industry standard for commercial hosts |
| **Agones features** | HIGH | Official docs + Google blog | Excellent documentation; confirmed K8s-native architecture and limitations |
| **WindowsGSM/LinuxGSM** | MEDIUM | GitHub repos + community forums | Open-source, but feature set varies by community contributions |
| **Table stakes vs differentiators** | MEDIUM | Cross-referenced 5+ panels | Consistent patterns emerge, but "expected" varies by user segment (hobbyist vs commercial host) |
| **Anti-features** | LOW | Training data + logical analysis | No authoritative source; based on scope creep analysis and competitor positioning |

**Overall Confidence:** MEDIUM

**Limitations:**
- Most sources are from early 2026; panels evolve quickly
- Commercial panels (AMP/TCAdmin) have limited public documentation
- "Table stakes" is subjective — differs for hobbyists vs hosting providers
- Anti-features based on analysis, not direct user feedback

**Verification Recommendations:**
- Interview 3-5 homelab users currently running Pterodactyl
- Test-drive AMP/TCAdmin trial versions for first-hand feature audit
- Survey r/homelab for "must-have" vs "nice-to-have" panel features

## Sources

### Pterodactyl
- [Pterodactyl Official Site](https://pterodactyl.io/)
- [GitHub - Pterodactyl Panel](https://github.com/pterodactyl/panel)
- [Pterodactyl Introduction Docs](https://pterodactyl.io/project/introduction.html)
- [Creating Custom Pterodactyl Eggs](https://pterodactyl.io/community/config/eggs/creating_a_custom_egg.html)
- [Pterodactyl API Documentation](https://pterodactyl-api-docs.netvpx.com/)
- [How to Create Sub-users in Pterodactyl](https://www.lazerhosting.com/billing/knowledgebase/5/How-to-Create-Sub-users-in-Pterodactyl-A-Comprehensive-Guide-for-Beginners.html)

### AMP (Application Management Panel)
- [AMP Official Site](https://cubecoders.com/AMP)
- [AMP vs. Pterodactyl Comparison](https://blog.atomicnetworks.co/cloud/comparisons/amp-vs-pterodactyl)
- [Installing AMP for Game Server Management](https://www.linode.com/docs/guides/installing-amp-game-server-management-panel/)

### TCAdmin
- [TCAdmin Official Site](https://www.tcadmin.com/)
- [TCAdmin Features](https://www.tcadmin.com/features/)
- [TCAdmin Documentation](https://docs.tcadmin.com)

### Agones
- [Agones Official Site](https://agones.dev/site/)
- [Agones Overview](https://agones.dev/site/docs/overview/)
- [GitHub - Agones](https://github.com/googleforgames/agones)
- [Introducing Agones - Google Cloud Blog](https://cloud.google.com/blog/products/containers-kubernetes/introducing-agones-open-source-multiplayer-dedicated-game-server-hosting-built-on-kubernetes)

### WindowsGSM/LinuxGSM
- [LinuxGSM Official Site](https://linuxgsm.com/)
- [GitHub - LinuxGSM](https://github.com/GameServerManagers/LinuxGSM)
- [GitHub - WindowsGSM](https://github.com/WindowsGSM/WindowsGSM)
- [WindowsGSM Documentation](https://docs.windowsgsm.com/)

### General Panel Features & Comparisons
- [Top 10+ Best Pterodactyl Alternatives in 2026](https://satisfyhost.com/blog/best-pterodactyl-alternatives/)
- [Best Game Server Control Panels](https://www.ghostcap.com/game-server-control-panels)
- [Benchmarking Pterodactyl vs Other Control Panels](https://lazerhosting.com/billing/knowledgebase/42/Benchmarking-Pterodactyl-vs-Other-Control-Panels.html?language=english)
- [The Beginner Guide To Game Server Control Panels](https://topserver.network/the-beginners-guide-to-game-server-control-panels-for-game-hosting/)

### Specific Feature Research
- [Game Server Panel Mod Management](https://steamcommunity.com/sharedfiles/filedetails/?id=3422448677)
- [Server Quotas & Load Balancing - GSP-Panel](https://wiki.gsp-panel.com/features:server_quotas_load_balancing)
- [Server Monitoring with Prometheus and Grafana](https://www.cherryservers.com/blog/server-monitoring-prometheus-grafana)
- [Game Panel Database Management](https://xgamingserver.com/blog/how-to-use-and-connect-to-the-game-panel-database-using-mysql-workbench/)
- [Accessing Files via SFTP and File Manager](https://pingperfect.com/knowledgebase/19/Accessing-your-files--File-manager--FTP--SFTP.html)

---
*Feature research for: Kterodactyl - Kubernetes-native Game Server Management Panel*
*Researched: 2026-02-09*
*Researcher: GSD Project Researcher Agent*
