# Milestones

## v1.1 End-to-End CI/CD Test Suite (Shipped: 2026-03-04)

**Delivered:** Reproducible test suite with Go API integration tests, Playwright E2E browser tests, kind-based test environment, and unified GitHub Actions CI pipeline.

**Phases completed:** 5 phases (13-17), 8 plans, 15 tasks
**Timeline:** 15 days (2026-02-18 to 2026-03-04)
**Codebase:** 13,210 Go + 5,923 TS/TSX LOC (19,133 total)
**Commits:** 39 (57 files changed, +7,544 / -161)
**Git range:** 6c1fdb4 → 4068e70
**Test files:** 16 Go test files + 2 Playwright spec files

**Key accomplishments:**
- Fixed envtest cached-client pattern and built handler-level httptest tests for mod, backup, and metrics endpoints
- Created multi-step API lifecycle integration test (register → create → get → delete) via httptest.NewServer
- Kind cluster environment with Helm deployment, NodePort access, and single-command setup/teardown
- Playwright E2E test suite with auth fixtures (admin + user roles), signup/login, and server CRUD tests
- Unified GitHub Actions CI pipeline with 5-job dependency chain, failure artifact uploads, and guaranteed cleanup

### Known Gaps

- **COV-01**: Go test coverage report not yet generated in CI (deferred — Phase 18 unexecuted)
- **COV-02**: Test backlog document not yet created (deferred — Phase 18 unexecuted)

---

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

