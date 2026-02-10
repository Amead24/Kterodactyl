# Stack Research

**Domain:** Kubernetes-native game server management platform
**Researched:** 2026-02-09
**Confidence:** HIGH

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| **Kubebuilder** | v4.x | Kubernetes operator scaffolding & development | Industry standard from kubernetes-sigs, provides robust scaffolding with controller-runtime integration, excellent community support, and natural integration with Kubernetes ecosystem. In 2025-2026, Kubebuilder v4 is the mature choice for Go-based operators. |
| **controller-runtime** | v0.23.x | Core reconciliation engine for operators | Foundation library used by both Kubebuilder and Operator SDK. Provides battle-tested reconciliation patterns, client libraries, and best practices. Direct dependency of Kubebuilder but understanding it is critical for production operators. |
| **Gin** | v1.10.0+ | Go REST API framework | Most mature Go web framework with 81k+ GitHub stars, 48% market share among Go developers. Martini-like API with excellent performance (34k req/s), extensive middleware ecosystem, and gentler learning curve than alternatives. Best balance of maturity, performance, and developer experience. |
| **Next.js** | v15.x | React framework for admin UI | Production-ready with React 19 support, built-in server components, optimized bundling via Turbopack, and excellent TypeScript integration. Next.js 15 provides modern routing (App Router), streaming, and performance improvements essential for dynamic admin UIs. |
| **React** | v19.x | Frontend UI library | Latest stable release with improved compiler (auto-memoization), new hooks (useActionState, useFormStatus, useOptimistic), and better form handling. React 19 + Next.js 15 is the 2025-2026 standard for production-grade admin panels. |
| **TypeScript** | v5.x | Type-safe JavaScript | Industry standard for React applications, enforces type safety, reduces runtime errors, improves maintainability. Strict mode with explicit typing is essential for scalable admin UIs. |
| **Helm** | v4.0.0+ | Kubernetes package manager | Latest major version with improved templating, better CRD support, and refined best practices. Essential for distributing complex operators with customizable deployments (Ingress vs Gateway API, auth backends, storage classes). |
| **Docusaurus** | v3.9.x | Documentation site generator | Meta-backed, production-ready static site generator with MDX support, versioning, internationalization, and Algolia search integration. Standard choice for open-source Kubernetes projects (used by Redux, Kubernetes ecosystem projects). React-based for customization. |
| **Go** | v1.24.6+ | Operator & API implementation language | Required by Kubebuilder v4, excellent concurrency primitives, native Kubernetes client libraries, and strong ecosystem for cloud-native development. Industry standard for Kubernetes operators. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| **slog** | stdlib (Go 1.21+) | Structured logging | Standard library solution for new Go 1.21+ projects. Provides structured logging without external dependencies, good balance of features and simplicity. Use for most logging needs in operators and APIs. |
| **Zap** | Latest | High-performance structured logging | When logging performance is critical (4-20x faster than alternatives). Use in hot paths or high-throughput reconciliation loops where even small performance gains matter. |
| **shadcn/ui** | Latest | React component library | Pre-built, accessible, customizable components for admin UI. Built with Radix UI primitives and Tailwind CSS. Use for rapid admin panel development with consistent design system. Not a package dependency—copy components into your codebase. |
| **Tailwind CSS** | v4.x | Utility-first CSS framework | Modern styling approach for Next.js admin UIs. Pairs well with shadcn/ui, provides dark mode support, responsive design utilities. Use for consistent, maintainable styling. |
| **Velero** | Latest | Kubernetes backup & restore | CNCF project for cluster-wide backup/restore of resources and persistent volumes. Use for scheduled and on-demand backups. Integrates with S3-compatible storage (MinIO, AWS S3, etc.). |
| **MinIO** | Latest | S3-compatible object storage | Open-source S3-compatible storage that can run in-cluster or on-premises. Use for air-gapped environments, cost reduction vs cloud storage, or when data sovereignty is required. Pairs perfectly with Velero. |
| **prometheus-operator** | Latest | Metrics collection & monitoring | CNCF standard for Kubernetes-native Prometheus deployments. Use for exposing operator metrics, API metrics, and cluster observability. Provides ServiceMonitor CRDs for declarative scrape configuration. |
| **cert-manager** | v1.19.1+ | TLS certificate management | Automates certificate provisioning and renewal for webhooks (required for admission controllers in operators). Industry standard for Kubernetes TLS automation. |
| **Gateway API** | GA (HTTPRoute) | Modern Kubernetes ingress | Successor to Ingress API, provides role-based design (infra teams manage Gateway, app teams manage HTTPRoutes), better protocol support (HTTP, TCP, UDP, gRPC), and advanced routing. **Critical**: Ingress NGINX retired March 2026, no security updates post-November 2026 for AKS. Migrate to Gateway API now. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| **kubectl** | Kubernetes CLI | Essential for operator development and testing |
| **kind** | Local Kubernetes clusters | Fast local testing of operators and Helm charts |
| **kustomize** | Kubernetes manifest customization | Built into kubectl, useful for environment-specific overlays |
| **goreleaser** | Go release automation | Automates building, packaging, and releasing operator binaries |
| **golangci-lint** | Go linting | Comprehensive linter with 50+ linters, standard for Go projects |
| **envtest** | Integration testing | controller-runtime's testing framework, spins up real API server for operator testing |
| **pnpm** | Node.js package manager | Faster and more disk-efficient than npm/yarn for frontend dependencies |

## Installation

### Operator & API (Go)

```bash
# Initialize Kubebuilder project (v4)
kubebuilder init --domain kterodactyl.io --repo github.com/yourusername/kterodactyl

# Create API
kubebuilder create api --group game --version v1alpha1 --kind GameServer

# Install Go dependencies
go mod download

# Development dependencies
go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### Frontend (Next.js + TypeScript)

```bash
# Create Next.js app with TypeScript
pnpm create next-app@latest --typescript --tailwind --app

# Add shadcn/ui
pnpm dlx shadcn-ui@latest init

# Core UI components
pnpm dlx shadcn-ui@latest add button card table form input select

# Add additional dependencies
pnpm add @tanstack/react-query @tanstack/react-table
pnpm add -D @types/node @types/react @types/react-dom
```

### Helm Chart

```bash
# Create Helm chart
helm create kterodactyl-operator

# Lint chart
helm lint ./kterodactyl-operator

# Template validation
helm template kterodactyl ./kterodactyl-operator --debug
```

### Documentation (Docusaurus)

```bash
# Create Docusaurus site
pnpm create docusaurus@latest docs classic --typescript

# Add versioning support (for stable/beta docs)
pnpm run docusaurus docs:version 1.0.0

# Add search (Algolia integration in docusaurus.config.js)
```

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| **Kubebuilder** | Operator SDK | Use Operator SDK when you need OLM (Operator Lifecycle Manager) integration, multi-language operators (Ansible/Helm-based), or OperatorHub publishing. For pure Go operators, Kubebuilder is simpler and more aligned with kubernetes-sigs. |
| **Kubebuilder** | controller-runtime directly | Only if you need complete control and are building a highly custom controller that doesn't fit the operator pattern. Requires deep Kubernetes expertise. 99% of projects should use Kubebuilder. |
| **Gin** | Echo | Use Echo if you prefer its centralized error handling and request validation patterns. Performance is nearly identical (34k req/s). Echo has ~30k stars vs Gin's 81k. |
| **Gin** | Fiber | Use Fiber if absolute performance is the #1 priority (36k req/s vs 34k) and you're comfortable with Express.js-style API. Trade-off: less Go standard library compatibility, smaller ecosystem. |
| **Gin** | Chi | Use Chi if you want a minimalist router that stays closer to net/http. Chi is fully stdlib-compatible but provides less structure than Gin. Good for small services or teams that prefer stdlib patterns. |
| **Next.js 15** | Vite + React | Use Vite if you don't need SSR/SSG and want faster development server. Next.js provides more out-of-the-box (routing, API routes, optimizations) for admin dashboards. |
| **Velero + MinIO** | Custom backup solution | Only if you have very specific requirements that Velero doesn't support. Velero is battle-tested and CNCF-backed. |
| **MinIO** | AWS S3 / Cloud storage | Use cloud storage if you're already cloud-native and cost isn't a concern. MinIO is better for on-premises, air-gapped, or cost-sensitive deployments. Both are S3-compatible. |
| **slog** | Zap | Use Zap in performance-critical paths or high-throughput services. slog is 95% as fast but zero external dependencies. |
| **slog** | Zerolog | Use Zerolog if you need absolute cutting-edge performance and JSON-first logging. slog is now preferred for new Go 1.21+ projects due to stdlib integration. |
| **Gateway API** | Ingress | **Do not use Ingress for new projects in 2026.** Ingress NGINX is retired (March 2026), with no security updates after November 2026. Gateway API is GA and the future of Kubernetes networking. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| **Ingress API** | Retired March 2026, no security updates post-November 2026 for AKS. Frozen—all new features go to Gateway API. Limited to HTTP/HTTPS, can't handle TCP/UDP/gRPC. | **Gateway API** (HTTPRoute for HTTP/HTTPS, TCPRoute for game server traffic if needed) |
| **Operator SDK without Kubebuilder** | Operator SDK uses Kubebuilder under the hood for Go projects. Unless you need OLM/OperatorHub/Ansible/Helm operators, Kubebuilder is more direct and simpler. | **Kubebuilder v4** for pure Go operators |
| **React 18 + Next.js 14** | React 19 and Next.js 15 are stable and production-ready in 2025-2026. Older versions miss compiler improvements, server components enhancements, and performance gains. | **React 19 + Next.js 15** |
| **JavaScript (no TypeScript)** | TypeScript is now the standard for production React applications. Reduces runtime errors, improves maintainability, and provides better IDE support. No reason to avoid it in 2026. | **TypeScript 5.x** with strict mode |
| **Logrus** | Unmaintained, superseded by slog (stdlib) and Zap/Zerolog for performance. 4-20x slower than modern alternatives. | **slog** (stdlib) for most use cases, **Zap** for high performance |
| **Helm v2 or v3** | Helm v4 is stable and available. v2 is deprecated, v3 lacks v4 improvements. | **Helm v4.0.0+** |
| **Docusaurus v2 or v1** | Docusaurus v3 (3.9.x) is current with better performance, React 19 support, and modern features. | **Docusaurus v3.9.x** |
| **Custom CRD validation (code)** | OpenAPI validation in CRD spec is declarative, validated by API server, and more maintainable than admission webhooks for simple validation. | **OpenAPI validation in CRD spec**, admission webhooks only for complex cross-field validation |
| **Manual Prometheus setup** | prometheus-operator provides declarative ServiceMonitor CRDs and automates Prometheus deployment. Manual setup is error-prone. | **prometheus-operator** with ServiceMonitor CRDs |

## Stack Patterns by Variant

### Pattern 1: Cloud-Native Deployment (AWS/GCP/Azure)

**When:** Deploying to managed Kubernetes (EKS, GKE, AKS)

**Stack Adjustments:**
- Use cloud-provider LoadBalancer for Gateway API
- Use cloud object storage (S3, GCS, Azure Blob) instead of MinIO for Velero backups
- Leverage cloud-provider CSI drivers for PersistentVolumes
- Use cloud IAM for authentication (IRSA on AWS, Workload Identity on GCP)

**Rationale:** Cloud providers offer managed services that reduce operational burden and integrate seamlessly with Kubernetes.

### Pattern 2: On-Premises / Bare Metal Deployment

**When:** Deploying to self-managed Kubernetes clusters (on-prem datacenters, homelabs)

**Stack Adjustments:**
- Deploy MinIO in-cluster or adjacent for S3-compatible storage
- Use MetalLB for LoadBalancer services (Gateway API)
- Use local-path-provisioner or OpenEBS for PersistentVolumes
- Use Dex or Keycloak for authentication (OIDC provider)

**Rationale:** On-premises environments require self-hosted alternatives to cloud-managed services. MinIO provides S3 compatibility without cloud costs.

### Pattern 3: Air-Gapped / Secure Environments

**When:** Compliance requirements prevent internet access (DoD, finance, healthcare)

**Stack Adjustments:**
- All container images must be mirrored to private registry
- MinIO is mandatory (cannot use cloud storage)
- Use internal Helm chart repository (Harbor, Artifactory, or ChartMuseum)
- Use internal documentation hosting (self-hosted Docusaurus)
- Use internal Git server (GitLab, Gitea) for GitOps

**Rationale:** Air-gapped environments require all dependencies to be internalized. MinIO, Harbor, and self-hosted tools enable complete stack isolation.

### Pattern 4: Multi-Tenant SaaS

**When:** Offering Kterodactyl as a managed service with multiple customers

**Stack Adjustments:**
- Implement tenant isolation at the namespace level
- Use namespace-scoped operators (if feasible) or tenant filtering in cluster-scoped operator
- Add tenant authentication layer (API keys, OIDC per-tenant)
- Use separate Velero backup schedules per tenant
- Implement resource quotas and RBAC per tenant

**Rationale:** Multi-tenancy requires strong isolation, authentication, and resource management. Kubernetes namespaces provide baseline isolation, but additional RBAC and quota enforcement are critical.

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|-----------------|-------|
| Kubebuilder v4.x | controller-runtime v0.23.x | Kubebuilder v4 requires controller-runtime v0.18+, latest v0.23.x recommended |
| Kubebuilder v4.x | Kubernetes 1.30+ | Kubebuilder v4 targets Kubernetes 1.30+, supports 1.28+ with some limitations |
| Kubebuilder v4.x | Go 1.24.6+ | Kubebuilder v4 requires Go 1.21+ for slog support, 1.24+ recommended |
| Next.js 15 | React 19 | Next.js 15 officially supports React 19 RC/stable, backwards compatible with React 18 via Pages Router |
| Next.js 15 | Node.js 18.18+ | Next.js 15 requires Node.js 18.18 or later (Node.js 20+ recommended) |
| TypeScript 5.x | React 19 | Fully compatible, use `@types/react@19` for correct typings |
| Velero | Kubernetes 1.21+ | Velero supports Kubernetes 1.21+, test with target cluster version |
| Gateway API | Kubernetes 1.27+ | HTTPRoute GA in Gateway API v1.0, requires Kubernetes 1.27+ for full feature support |
| cert-manager v1.19+ | Kubernetes 1.26+ | cert-manager v1.19+ requires Kubernetes 1.26+ |
| Helm v4.0.0 | Kubernetes 1.29+ | Helm v4 supports Kubernetes 1.29-1.32 (verify latest compatibility matrix) |

## Confidence Assessment

| Technology | Confidence | Source |
|------------|-----------|---------|
| Kubebuilder v4 | **HIGH** | Official kubernetes-sigs project, verified current releases via GitHub |
| controller-runtime v0.23 | **HIGH** | Verified via GitHub releases and Kubebuilder compatibility matrix |
| Gin v1.10+ | **HIGH** | Verified via GitHub releases, 81k stars, 48% market share in 2025 research |
| Next.js 15 + React 19 | **HIGH** | Official Next.js documentation confirms v15 with React 19 support |
| TypeScript 5.x | **HIGH** | Industry standard, official TypeScript releases verified |
| Helm v4 | **HIGH** | Official Helm documentation confirms v4.0.0 release |
| Docusaurus v3.9.x | **HIGH** | Official Docusaurus website confirms v3.9.2 current release |
| Gateway API (HTTPRoute GA) | **HIGH** | Verified via official Gateway API docs, Ingress retirement confirmed by Kubernetes SIG Network |
| slog (Go stdlib) | **HIGH** | Part of Go 1.21+ standard library, official Go documentation |
| Velero + MinIO | **HIGH** | CNCF projects, widely adopted for Kubernetes backup solutions |
| shadcn/ui | **MEDIUM** | Popular in community (18+ production-ready templates in 2025-2026), not official React project |
| prometheus-operator | **HIGH** | CNCF project, standard for Kubernetes Prometheus deployments |

## Sources

### Operator Frameworks
- [Kubebuilder vs Operator SDK discussion](https://github.com/operator-framework/operator-sdk/issues/1758)
- [ITNEXT: Developing Kubernetes Operators](https://itnext.io/developing-kubernetes-operators-9eb5f8230a72)
- [Operator SDK FAQ](https://sdk.operatorframework.io/docs/faqs/)
- [Kubebuilder GitHub releases](https://github.com/kubernetes-sigs/kubebuilder/releases)
- [OuterByte: Kubernetes Operators 2025 Guide](https://outerbyte.com/kubernetes-operators-2025-guide/)
- [Operator SDK Best Practices](https://sdk.operatorframework.io/docs/best-practices/common-recommendation/)
- [Kubebuilder Book: Good Practices](https://book.kubebuilder.io/reference/good-practices)

### Go API Frameworks
- [Medium: Top 6 Go Web Frameworks for 2025](https://medium.com/@yashbatra11111/top-6-go-web-frameworks-for-2025-which-one-should-you-choose-5821f31a2010)
- [LogRocket: Best Go Frameworks 2025](https://blog.logrocket.com/top-go-frameworks-2025/)
- [JHK InfoTech: Golang Web Framework Comparison](https://www.jhkinfotech.com/blog/golang-web-framework)
- [Medium: Gin vs Fiber vs Echo Performance](https://medium.com/deno-the-complete-reference/go-gin-vs-fiber-vs-echo-how-much-performance-difference-is-really-there-for-a-real-world-use-1ed29d6a3e4d)
- [Encore: Best Go Backend Frameworks 2026](https://encore.dev/articles/best-go-backend-frameworks)

### Next.js & React
- [Next.js 15 Release](https://nextjs.org/blog/next-15)
- [Next.js Templates: Admin Dashboards 2026](https://nextjstemplates.com/blog/admin-dashboard-templates)
- [DEV: Free Next.js Admin Dashboards 2025](https://dev.to/vinishbhaskar/free-nexts-admin-dashboard-55ko)
- [Next.js Server Components Docs](https://nextjs.org/docs/app/getting-started/server-and-client-components)
- [Coder Trove: React Server Components 2025](https://www.codertrove.com/articles/react-server-components-2025-nextjs-performance)
- [Medium: React 19 TypeScript Best Practices 2025](https://medium.com/@CodersWorld99/react-19-typescript-best-practices-the-new-rules-every-developer-must-follow-in-2025-3a74f63a0baf)

### Helm
- [Helm Official Documentation](https://helm.sh/docs)
- [Helm Chart Best Practices](https://helm.sh/docs/chart_best_practices/)
- [Atmosly: Helm Charts 2026 Guide](https://atmosly.com/knowledge/helm-charts-in-kubernetes-definitive-guide-for-2025)
- [Carlos Neto: Helm Best Practices](https://carlosneto.dev/blog/2025/2025-02-25-helm-best-practices/)
- [Prequel: Helm Chart Reliability 2025](https://www.prequel.dev/blog-post/the-real-state-of-helm-chart-reliability-2025-hidden-risks-in-100-open-source-charts)

### Docusaurus
- [Docusaurus Official Site](https://docusaurus.io/)
- [Docusaurus GitHub](https://github.com/facebook/docusaurus)
- [Meta Open Source: Docusaurus](https://opensource.fb.com/projects/docusaurus/)
- [Hackmamba: Top Open-Source Documentation Tools 2026](https://hackmamba.io/technical-documentation/top-5-open-source-documentation-development-platforms-of-2024/)

### Kubernetes Backup & Storage
- [LevenzonLabs: Velero, MinIO, and Kubernetes Backups](https://www.levenzonlabs.com/posts/post-07-21-2025v2/)
- [MicroK8s: Backup with Velero](https://microk8s.io/docs/velero)
- [Velero Documentation](https://velero.io/docs/main/contributions/minio/)
- [Backblaze: Object Storage and Kubernetes](https://www.backblaze.com/blog/5-tools-to-integrate-object-storage-and-kubernetes/)

### Gateway API & Networking
- [Kong: Gateway API vs Ingress](https://konghq.com/blog/engineering/gateway-api-vs-ingress)
- [Microsoft: Ingress to Gateway API Migration](https://techcommunity.microsoft.com/blog/azurearchitectureblog/from-ingress-to-gateway-api-a-pragmatic-path-forward-and-why-it-matters-now/4489779)
- [Tigera: Kubernetes Ingress vs Gateway API](https://www.tigera.io/blog/is-it-time-to-migrate-a-practical-look-at-kubernetes-ingress-vs-gateway-api/)
- [Gateway API Official Docs](https://gateway-api.sigs.k8s.io/)
- [CNCF: Understanding Gateway API](https://www.cncf.io/blog/2025/05/02/understanding-kubernetes-gateway-api-a-modern-approach-to-traffic-management/)

### Logging
- [Dash0: Best Go Logging Tools 2025](https://www.dash0.com/faq/best-go-logging-tools-in-2025-a-comprehensive-guide)
- [Uptrace: Golang Logging Libraries 2025](https://uptrace.dev/blog/golang-logging)
- [Better Stack: Logging in Go with Slog](https://betterstack.com/community/guides/logging/logging-in-go/)
- [Last9: Golang Logging Guide](https://last9.io/blog/golang-logging-guide-for-developers/)
- [Leapcell: High-Performance Structured Logging](https://leapcell.io/blog/high-performance-structured-logging-in-go-with-slog-and-zerolog)

### Prometheus & Observability
- [Better Stack: Prometheus Best Practices](https://betterstack.com/community/guides/monitoring/prometheus-best-practices/)
- [Operator SDK: Observability Best Practices](https://sdk.operatorframework.io/docs/best-practices/observability-best-practices/)
- [Spacelift: Prometheus Operator Tutorial](https://spacelift.io/blog/prometheus-operator)
- [CNCF: Prometheus Labels Best Practices](https://www.cncf.io/blog/2025/07/22/prometheus-labels-understanding-and-best-practices/)

### shadcn/ui
- [shadcn/ui Template Gallery](https://www.shadcn.io/template/category/dashboard)
- [AdminLTE: Shadcn Admin Dashboard Templates](https://adminlte.io/blog/shadcn-admin-dashboard-templates/)
- [GitHub: next-shadcn-dashboard-starter](https://github.com/Kiranism/next-shadcn-dashboard-starter)
- [Shadcn UI Kit](https://shadcnuikit.com/)

### Game Server Management
- [GitHub: Agones](https://github.com/googleforgames/agones)
- [Google Cloud: Introducing Agones](https://cloud.google.com/blog/products/containers-kubernetes/introducing-agones-open-source-multiplayer-dedicated-game-server-hosting-built-on-kubernetes)
- [Oreate AI: Agones Future of Game Server Management](https://www.oreateai.com/blog/exploring-agones-the-future-of-game-server-management-on-kubernetes/09f5f5e6ac5c3043102abcbb81d0826b)

---
*Stack research for: Kterodactyl - Kubernetes-native game server management platform*
*Researched: 2026-02-09*
*Researcher: gsd-project-researcher agent*
