# Phase 10: Observability - Research

**Researched:** 2026-02-12
**Domain:** Prometheus metrics instrumentation for Go Kubernetes operators and HTTP APIs
**Confidence:** HIGH

## Summary

This phase adds Prometheus metrics to the kterodactyl operator and API server, then creates ServiceMonitor CRDs for Prometheus Operator autodiscovery. The project already has significant scaffolding in place: controller-runtime v0.23.1 exposes a metrics server on port :8443 (configured in `config/default/manager_metrics_patch.yaml`), prometheus/client_golang v1.23.2 is already a transitive dependency, and a ServiceMonitor template exists in `config/prometheus/monitor.yaml` (currently commented out in kustomization.yaml).

The work divides cleanly into three areas: (1) operator-side custom metrics (game server count by state as a GaugeVec, reconciliation duration as a Histogram -- noting that controller-runtime already exposes `controller_runtime_reconcile_time_seconds` but we want a custom one per game_type), (2) API server HTTP metrics (request rate, latency, error rate via chi middleware), and (3) ServiceMonitor CRDs. Both the operator and API server run in the same process, so all custom metrics register with the controller-runtime `metrics.Registry` and are served on the existing :8443 metrics endpoint.

The API server metrics require a custom chi middleware rather than the `go-chi/metrics` package because that package registers with the default prometheus registry, not the controller-runtime registry. Writing a simple ~40-line middleware that wraps `http.ResponseWriter` to capture status codes and records to a `HistogramVec` and `CounterVec` registered on `metrics.Registry` is the correct approach.

**Primary recommendation:** Register all custom metrics with `sigs.k8s.io/controller-runtime/pkg/metrics.Registry` using `init()` functions, write a thin chi middleware for HTTP metrics, and uncomment + adapt the existing ServiceMonitor scaffolding.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/prometheus/client_golang/prometheus` | v1.23.2 | Define metrics (Counter, Gauge, Histogram, *Vec variants) | THE Go Prometheus client; already a transitive dep |
| `sigs.k8s.io/controller-runtime/pkg/metrics` | v0.23.1 | Registry for all custom metrics in the operator process | Controller-runtime's metrics.Registry is the single source of truth |
| `monitoring.coreos.com/v1` ServiceMonitor CRD | - | Prometheus Operator autodiscovery | Industry standard for K8s metrics scraping |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/prometheus/client_golang/prometheus/promhttp` | v1.23.2 | NOT needed (controller-runtime serves metrics) | Only if you need a standalone metrics HTTP handler |
| `github.com/go-chi/metrics` | v0.1.1 | HTTP middleware for chi | NOT recommended -- uses default registry, not controller-runtime registry |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Custom chi middleware | `go-chi/metrics` v0.1.1 | Uses default prometheus registry; would require a separate /metrics endpoint or registry bridging. Custom middleware is ~40 lines and registers directly with controller-runtime registry. |
| Custom reconciliation histogram | Built-in `controller_runtime_reconcile_time_seconds` | Built-in exists but labels by controller name only; custom metric allows labeling by `game_type` and `server_state`. Use both -- keep built-in, add custom. |

**Installation:**
```bash
# No new dependencies needed -- prometheus/client_golang v1.23.2 is already
# a transitive dependency of controller-runtime v0.23.1
# Verify with:
go list -m github.com/prometheus/client_golang
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
  metrics/
    metrics.go          # All metric definitions + init() registration
  controller/
    gameserver_controller.go  # Record operator metrics during reconciliation
  api/
    middleware_metrics.go     # Chi middleware for HTTP request metrics
    routes.go                 # Add metrics middleware to router
config/
  prometheus/
    monitor.yaml              # ServiceMonitor for operator metrics (:8443)
    api_monitor.yaml          # ServiceMonitor for API server metrics (optional, same pod)
  default/
    kustomization.yaml        # Uncomment prometheus resources
```

### Pattern 1: Centralized Metric Registration
**What:** All metric definitions in a single `internal/metrics/metrics.go` file, registered via `init()` with controller-runtime's `metrics.Registry`.
**When to use:** Always -- prevents scattered metric definitions and duplicate registration panics.
**Example:**
```go
// Source: https://book.kubebuilder.io/reference/metrics + verified against codebase
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
    // Operator metrics (OBS-01)
    GameServersByState = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "kterodactyl_gameservers_by_state",
            Help: "Number of GameServer resources by state and game type.",
        },
        []string{"state", "game_type"},
    )

    ReconciliationDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "kterodactyl_reconciliation_duration_seconds",
            Help:    "Duration of GameServer reconciliation in seconds.",
            Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
        },
        []string{"controller"},
    )

    // API server metrics (OBS-02)
    HTTPRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "kterodactyl_http_requests_total",
            Help: "Total number of HTTP requests by method, path pattern, and status code.",
        },
        []string{"method", "route", "status_code"},
    )

    HTTPRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "kterodactyl_http_request_duration_seconds",
            Help:    "HTTP request duration in seconds.",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "route", "status_code"},
    )

    HTTPRequestsInFlight = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "kterodactyl_http_requests_inflight",
            Help: "Number of HTTP requests currently being served.",
        },
    )
)

func init() {
    crmetrics.Registry.MustRegister(
        GameServersByState,
        ReconciliationDuration,
        HTTPRequestsTotal,
        HTTPRequestDuration,
        HTTPRequestsInFlight,
    )
}
```

### Pattern 2: Chi Metrics Middleware (ResponseWriter Wrapper)
**What:** A chi-compatible middleware that wraps `http.ResponseWriter` to capture the status code, then records request duration and count to Prometheus metrics.
**When to use:** For all API routes (except WebSocket upgrades and health checks).
**Example:**
```go
// Source: Standard Go pattern for http middleware instrumentation
package api

import (
    "net/http"
    "strconv"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/kterodactyl/kterodactyl/internal/metrics"
)

// statusRecorder wraps http.ResponseWriter to capture the status code.
type statusRecorder struct {
    http.ResponseWriter
    statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
    r.statusCode = code
    r.ResponseWriter.WriteHeader(code)
}

// metricsMiddleware records HTTP request metrics for Prometheus.
func metricsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        metrics.HTTPRequestsInFlight.Inc()
        defer metrics.HTTPRequestsInFlight.Dec()

        start := time.Now()
        rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}

        next.ServeHTTP(rec, r)

        duration := time.Since(start).Seconds()
        // Use chi's route pattern for low cardinality (e.g., "/api/v1/gameservers/{name}")
        routePattern := chi.RouteContext(r.Context()).RoutePattern()
        if routePattern == "" {
            routePattern = "unknown"
        }
        status := strconv.Itoa(rec.statusCode)

        metrics.HTTPRequestsTotal.WithLabelValues(r.Method, routePattern, status).Inc()
        metrics.HTTPRequestDuration.WithLabelValues(r.Method, routePattern, status).Observe(duration)
    })
}
```

### Pattern 3: GameServer Count Gauge via Periodic Update
**What:** Update the `GameServersByState` gauge during each reconciliation by listing all GameServers and grouping by state + game_type.
**When to use:** In the reconciler, after successful state transitions.
**Important:** A simpler alternative is to increment/decrement on state transitions, but this drifts if reconciliations fail. A periodic list-and-set approach is more accurate.
**Example:**
```go
// Source: Standard operator pattern
func (r *GameServerReconciler) updateGameServerGauge(ctx context.Context) {
    gsList := &gamev1alpha1.GameServerList{}
    if err := r.List(ctx, gsList); err != nil {
        return // Don't fail reconciliation for metrics
    }

    // Reset all known combinations to zero, then set actual counts
    metrics.GameServersByState.Reset()

    counts := make(map[string]map[string]float64) // state -> game_type -> count
    for _, gs := range gsList.Items {
        state := string(gs.Status.State)
        gameType := gs.Spec.GameType
        if counts[state] == nil {
            counts[state] = make(map[string]float64)
        }
        counts[state][gameType]++
    }

    for state, gameTypes := range counts {
        for gameType, count := range gameTypes {
            metrics.GameServersByState.WithLabelValues(state, gameType).Set(count)
        }
    }
}
```

### Anti-Patterns to Avoid
- **High-cardinality labels:** NEVER use pod names, user IDs, server names, or namespace names as metric labels. This causes Prometheus cardinality explosion. Use only `state`, `game_type`, `controller`, `method`, `route`, `status_code`.
- **Using the default prometheus registry:** The `go-chi/metrics` package and `prometheus.MustRegister()` use the default registry. Controller-runtime uses `metrics.Registry`. Mixing registries means metrics from one won't appear on the other's endpoint.
- **Blocking reconciliation on metric updates:** Never let metric recording errors fail a reconciliation. Wrap metric operations in defensive code that logs errors but doesn't return them.
- **Recording metrics after response is sent:** The chi middleware must record AFTER `next.ServeHTTP()` returns to capture the actual status code and duration.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Prometheus metric types | Custom counter/gauge/histogram structs | `prometheus.NewCounterVec`, `NewGaugeVec`, `NewHistogramVec` | Thread-safe, tested, Prometheus-wire-format compatible |
| Metrics HTTP endpoint | Custom `/metrics` handler | Controller-runtime metrics server (port :8443) | Already configured, handles TLS, auth, content negotiation |
| ServiceMonitor CRD | Raw Prometheus scrape config | `monitoring.coreos.com/v1` ServiceMonitor | Prometheus Operator standard; already scaffolded in `config/prometheus/` |
| HTTP response code capture | Manual tracking in each handler | `statusRecorder` wrapper middleware | One place, all routes, no per-handler changes |
| Histogram bucket selection | Arbitrary bucket values | `prometheus.DefBuckets` for HTTP, custom `{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}` for reconciliation | DefBuckets are well-tested for HTTP latency; reconciliation buckets match operator community practice |

**Key insight:** Controller-runtime already provides the metrics infrastructure (registry, HTTP server, TLS, auth). The task is to register custom collectors, not build a metrics system.

## Common Pitfalls

### Pitfall 1: Registering with Wrong Prometheus Registry
**What goes wrong:** Metrics registered with `prometheus.MustRegister()` (default registry) don't appear on the controller-runtime metrics endpoint (:8443).
**Why it happens:** `prometheus.MustRegister()` uses `prometheus.DefaultRegisterer`, but controller-runtime creates its own `metrics.Registry = prometheus.NewRegistry()`.
**How to avoid:** Always use `crmetrics.Registry.MustRegister()` where `crmetrics` is `sigs.k8s.io/controller-runtime/pkg/metrics`.
**Warning signs:** Metrics work in unit tests (which may use default registry) but are missing when scraping the actual operator.

### Pitfall 2: High-Cardinality Label Explosion
**What goes wrong:** Prometheus OOMs or becomes extremely slow; scrape timeouts occur.
**Why it happens:** Using pod names, user IDs, or server names as metric labels creates unbounded cardinality. Each unique label combination is a separate time series.
**How to avoid:** Only use bounded, enumerable values as labels: `state` (6 values), `game_type` (handful of games), `method` (GET/POST/PUT/DELETE), `route` (finite route patterns), `status_code` (grouped: 2xx/4xx/5xx or specific codes). Never use `pod_name`, `user_id`, `server_name`, `namespace`.
**Warning signs:** `prometheus_tsdb_head_series` growing unboundedly.

### Pitfall 3: statusRecorder Not Implementing Optional Interfaces
**What goes wrong:** WebSocket upgrades fail, streaming responses break, or `http.Flusher`/`http.Hijacker` stop working.
**Why it happens:** The `statusRecorder` wraps `http.ResponseWriter` but doesn't implement `http.Flusher`, `http.Hijacker`, etc.
**How to avoid:** Either (a) skip the metrics middleware for WebSocket routes (mount it only on REST routes), or (b) implement `Flush()`, `Hijack()` etc. by delegating to the underlying writer. In this project, the WebSocket route (`/api/v1/gameservers/{name}/console`) is already mounted OUTSIDE the REST API route group, so this is not a concern if the middleware is applied inside the REST group only.
**Warning signs:** WebSocket connections fail after adding middleware.

### Pitfall 4: Duplicate Metric Registration Panics
**What goes wrong:** `MustRegister` panics with "duplicate metrics collector registration attempted".
**Why it happens:** Calling `MustRegister` more than once for the same metric, often due to tests running in parallel or multiple init() functions registering the same collector.
**How to avoid:** Define all metrics in a single `internal/metrics/metrics.go` file with one `init()`. In tests, either use `prometheus.NewRegistry()` locally or accept that the global registration happens once.
**Warning signs:** Panics during test runs or operator startup.

### Pitfall 5: Gauge Reset Causing Gaps
**What goes wrong:** `GameServersByState.Reset()` followed by `.Set()` creates a brief window where all values are 0, which Prometheus records.
**Why it happens:** The reset-and-set is not atomic.
**How to avoid:** For game server count gauges, either (a) accept the brief zero (Prometheus scrape interval >> reset duration, so it's unlikely to be captured), or (b) use a map to track old values and only delete label combinations that no longer exist. Option (a) is fine for this project.
**Warning signs:** Occasional zero-dips in dashboards at scrape boundaries.

### Pitfall 6: ServiceMonitor Namespace Mismatch
**What goes wrong:** Prometheus never scrapes the operator because the ServiceMonitor's selector doesn't match or Prometheus doesn't watch the operator's namespace.
**Why it happens:** Prometheus Operator is configured with `serviceMonitorNamespaceSelector` and `serviceMonitorSelector`. If the ServiceMonitor is in a namespace Prometheus doesn't watch, it's ignored.
**How to avoid:** Ensure the ServiceMonitor is in the same namespace as the operator (kterodactyl-system) and that Prometheus is configured to watch that namespace. The existing scaffolding puts it in `system` which kustomize rewrites to `kterodactyl-system`.
**Warning signs:** ServiceMonitor exists but no targets appear in Prometheus UI.

## Code Examples

Verified patterns from official sources:

### Registering Custom Metrics with Controller-Runtime
```go
// Source: https://book.kubebuilder.io/reference/metrics
// Source: https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/metrics (v0.23.1)
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
    GameServersByState = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "kterodactyl_gameservers_by_state",
            Help: "Number of GameServer resources by state and game type.",
        },
        []string{"state", "game_type"},
    )
)

func init() {
    crmetrics.Registry.MustRegister(GameServersByState)
}
```

### Recording Reconciliation Duration
```go
// Source: Standard prometheus/client_golang histogram pattern
func (r *GameServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    start := time.Now()
    defer func() {
        duration := time.Since(start).Seconds()
        metrics.ReconciliationDuration.WithLabelValues("gameserver").Observe(duration)
    }()

    // ... existing reconciliation logic ...
}
```

### ServiceMonitor for Operator Metrics
```yaml
# Source: config/prometheus/monitor.yaml (Kubebuilder scaffolded, adapted)
# This already exists -- needs to be uncommented in config/default/kustomization.yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  labels:
    control-plane: controller-manager
    app.kubernetes.io/name: kterodactyl
    app.kubernetes.io/managed-by: kustomize
  name: controller-manager-metrics-monitor
  namespace: system  # Kustomize rewrites to kterodactyl-system
spec:
  endpoints:
    - path: /metrics
      port: https
      scheme: https
      bearerTokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
      tlsConfig:
        insecureSkipVerify: true  # Acceptable for homelab; use cert-manager for prod
  selector:
    matchLabels:
      control-plane: controller-manager
      app.kubernetes.io/name: kterodactyl
```

### ServiceMonitor for API Server (if separate Service)
```yaml
# Only needed if API server is exposed as a separate K8s Service
# In the current architecture, API runs on :8080 in the same pod
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: api-server-metrics-monitor
  namespace: system
spec:
  endpoints:
    - path: /metrics
      port: api  # Would need a Service with this port name
      scheme: http
  selector:
    matchLabels:
      app.kubernetes.io/name: kterodactyl
      app.kubernetes.io/component: api-server
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `prometheus.MustRegister()` in controllers | `metrics.Registry.MustRegister()` | controller-runtime v0.5.0+ | Must use CR registry, not default |
| Separate `/metrics` HTTP handler | Manager-integrated metrics server | controller-runtime v0.15+ (metricsserver pkg) | No need to run a separate HTTP server for metrics |
| Manual Prometheus scrape config | ServiceMonitor/PodMonitor CRDs | Prometheus Operator stable since 2020 | Declarative, auto-updated scraping |
| Metrics on HTTP (:8080) | Metrics on HTTPS (:8443) with auth | Kubebuilder v4 default | More secure but requires TLS config awareness |

**Deprecated/outdated:**
- `metrics.DefaultBindAddress`: No longer used; configure via `metricsserver.Options` in manager setup
- Manual `/metrics` endpoint with `promhttp.Handler()`: Not needed when using controller-runtime manager

## Open Questions

1. **API Server Metrics Endpoint Strategy**
   - What we know: The API server runs on :8080 (same pod as operator). Custom metrics registered with `metrics.Registry` are served on the operator's :8443 endpoint. This means ALL metrics (operator + API) are scraped from :8443.
   - What's unclear: Whether we also want a `/metrics` endpoint on the API server (:8080) for direct access. This would require `promhttp.HandlerFor(crmetrics.Registry, ...)`.
   - Recommendation: Keep it simple -- all metrics on :8443 via controller-runtime. One scrape endpoint, one ServiceMonitor. If direct API metrics access is needed later, add a read-only `/metrics` route to the chi router.

2. **Prometheus Operator Availability**
   - What we know: The homelab runs Talos + Cilium. ServiceMonitor CRDs require Prometheus Operator to be installed.
   - What's unclear: Whether Prometheus Operator is currently deployed on the cluster.
   - Recommendation: Create the ServiceMonitor manifests regardless. They are harmless if Prometheus Operator is not installed (they're just CRD instances that nothing watches). Document that Prometheus Operator must be installed for monitoring to work.

3. **Metrics Scrape Security**
   - What we know: The default Kubebuilder scaffolding uses HTTPS with `insecureSkipVerify: true` and bearer token auth. The current `cmd/main.go` sets `secureMetrics: true` by default and uses `filters.WithAuthenticationAndAuthorization`.
   - What's unclear: Whether the homelab Prometheus has RBAC to authenticate to the metrics endpoint.
   - Recommendation: For homelab, consider setting `--metrics-secure=false` to use HTTP on :8443 if RBAC is not configured, OR ensure Prometheus ServiceAccount has the right ClusterRoleBinding. The scaffolded RBAC in `config/rbac/metrics_reader_role.yaml` + `metrics_auth_role.yaml` should handle this.

## Sources

### Primary (HIGH confidence)
- `sigs.k8s.io/controller-runtime/pkg/metrics` v0.23.1 - pkg.go.dev documentation, Registry API, MustRegister pattern
- `github.com/prometheus/client_golang/prometheus` v1.23.2 - pkg.go.dev, CounterVec/GaugeVec/HistogramVec APIs
- Kubebuilder Book metrics reference - Default metrics list, custom metric registration pattern
- Project codebase analysis - `cmd/main.go`, `config/prometheus/`, `config/default/kustomization.yaml`

### Secondary (MEDIUM confidence)
- Prometheus Operator ServiceMonitor API - monitoring.coreos.com/v1 spec
- Histogram bucket best practices - prometheus.io/docs/practices/histograms/
- go-chi/metrics v0.1.1 - pkg.go.dev (evaluated and rejected due to registry mismatch)

### Tertiary (LOW confidence)
- None -- all findings verified against official documentation or codebase

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - prometheus/client_golang and controller-runtime metrics are well-documented, already present as dependencies
- Architecture: HIGH - Pattern verified against codebase; single-process architecture with controller-runtime registry is standard
- Pitfalls: HIGH - Registry mismatch is a well-documented issue; cardinality best practices are stable Prometheus guidance
- ServiceMonitor: MEDIUM - Scaffolding exists and is standard, but depends on Prometheus Operator being installed on cluster

**Research date:** 2026-02-12
**Valid until:** 2026-03-14 (30 days -- stable domain, libraries are mature)
