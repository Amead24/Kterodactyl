# Milestones

## v1.0 MVP (Shipped: 2026-02-13)

**Delivered:** Kubernetes-native game server management panel with CRD operator, REST API, React UI, mod support, S3 backups, Prometheus metrics, Helm chart, and Docusaurus documentation.

**Phases completed:** 12 phases, 34 plans, 74 tasks
**Timeline:** 4 days (2026-02-09 to 2026-02-13)
**Codebase:** 28,299 LOC (12,043 Go + 16,256 TypeScript/TSX)
**Commits:** 144 (533 files changed, 82,536 insertions)
**Git range:** feat(01-01) to feat(12-02)

**Key accomplishments:**
- GameServer CRD with 6-state lifecycle, reconciliation controller, and namespace isolation with ResourceQuotas
- Gateway API networking with DNS controller creating Services and HTTPRoutes per server
- Authentication layer with Argon2id hashing, JWT sessions, admin invite system
- Chi v5 REST API with 16 endpoints bridging users to Kubernetes
- Declarative game framework with JSON Schema validation and Minecraft reference game
- React SPA with RJSF dynamic forms, server management, admin UI, embedded in Go binary
- WebSocket console with real-time log streaming and resource metrics
- Mod support with PVC storage and drag-and-drop uploads
- S3-backed backup system with cron scheduling and restore
- Prometheus metrics with ServiceMonitor autodiscovery
- Production-ready Helm chart with RBAC, CRDs, and 50+ configurable values
- 18-page Docusaurus documentation site with architecture diagrams

**Tech debt carried forward:**
- DNS requires human testing with live Gateway API controller and ExternalDNS
- Relative path `"games/"` in cmd/main.go relies on container WORKDIR
- handleUploadMod and handleRestoreBackup bypass IsValidTransition guard
- Duplicate s3CredentialsSecretName constant in controller and API handler

---

