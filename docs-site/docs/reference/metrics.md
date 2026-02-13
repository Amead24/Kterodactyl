---
sidebar_position: 3
---

# Metrics Reference

Kterodactyl exposes 5 Prometheus metrics covering operator health and API performance. All metrics are registered with the controller-runtime metrics registry and served on the manager's metrics endpoint (port 8443, HTTPS).

## Operator Metrics

### kterodactyl_gameservers_by_state

**Type:** Gauge

Tracks the current count of GameServer resources by lifecycle state and game type.

| Label | Values | Description |
|-------|--------|-------------|
| `state` | `Creating`, `Starting`, `Ready`, `Allocated`, `Shutdown`, `Error` | GameServer lifecycle state |
| `game_type` | Game names (e.g., `minecraft`) | Game type identifier |

**Example PromQL:**

```promql
# Total servers currently running (Ready or Allocated)
sum(kterodactyl_gameservers_by_state{state=~"Ready|Allocated"})

# Servers in error state by game type
kterodactyl_gameservers_by_state{state="Error"}

# Total servers across all states
sum(kterodactyl_gameservers_by_state)
```

### kterodactyl_reconciliation_duration_seconds

**Type:** Histogram

Tracks the duration of reconciliation loops in seconds.

| Label | Values | Description |
|-------|--------|-------------|
| `controller` | `gameserver`, `backup` | Which controller performed the reconciliation |

**Buckets:** 0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0 seconds

**Example PromQL:**

```promql
# p99 reconciliation latency for the gameserver controller
histogram_quantile(0.99, sum(rate(kterodactyl_reconciliation_duration_seconds_bucket{controller="gameserver"}[5m])) by (le))

# Average reconciliation duration by controller
rate(kterodactyl_reconciliation_duration_seconds_sum[5m]) / rate(kterodactyl_reconciliation_duration_seconds_count[5m])

# Reconciliation rate (reconciliations per second)
sum(rate(kterodactyl_reconciliation_duration_seconds_count[5m])) by (controller)
```

## API Metrics

### kterodactyl_http_requests_total

**Type:** Counter

Tracks the total number of HTTP requests handled by the API server.

| Label | Values | Description |
|-------|--------|-------------|
| `method` | `GET`, `POST`, `PUT`, `DELETE` | HTTP method |
| `route` | Chi route patterns (e.g., `/api/v1/gameservers/{name}`) | Route pattern (low cardinality) |
| `status_code` | `200`, `201`, `400`, `401`, `404`, `500`, etc. | HTTP response status code |

**Example PromQL:**

```promql
# Total request rate
sum(rate(kterodactyl_http_requests_total[5m]))

# Error rate (5xx responses)
sum(rate(kterodactyl_http_requests_total{status_code=~"5.."}[5m]))

# Error percentage
sum(rate(kterodactyl_http_requests_total{status_code=~"5.."}[5m])) / sum(rate(kterodactyl_http_requests_total[5m])) * 100

# Request rate by endpoint
sum(rate(kterodactyl_http_requests_total[5m])) by (method, route)
```

### kterodactyl_http_request_duration_seconds

**Type:** Histogram

Tracks HTTP request latency in seconds.

| Label | Values | Description |
|-------|--------|-------------|
| `method` | `GET`, `POST`, `PUT`, `DELETE` | HTTP method |
| `route` | Chi route patterns | Route pattern (low cardinality) |
| `status_code` | HTTP status codes | Response status code |

**Buckets:** Prometheus default buckets (0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0)

**Example PromQL:**

```promql
# p99 request latency across all endpoints
histogram_quantile(0.99, sum(rate(kterodactyl_http_request_duration_seconds_bucket[5m])) by (le))

# p99 latency for server creation
histogram_quantile(0.99, sum(rate(kterodactyl_http_request_duration_seconds_bucket{method="POST", route="/api/v1/gameservers"}[5m])) by (le))

# Average request duration by route
sum(rate(kterodactyl_http_request_duration_seconds_sum[5m])) by (route) / sum(rate(kterodactyl_http_request_duration_seconds_count[5m])) by (route)
```

### kterodactyl_http_requests_inflight

**Type:** Gauge

Tracks the number of HTTP requests currently being served concurrently. No labels.

**Example PromQL:**

```promql
# Current in-flight requests
kterodactyl_http_requests_inflight

# Peak in-flight requests over the last hour
max_over_time(kterodactyl_http_requests_inflight[1h])
```

## Scraping Configuration

Metrics are served on the manager's metrics endpoint at port 8443 with HTTPS. If you are using Prometheus Operator, the Helm chart includes a `ServiceMonitor` resource:

```yaml
# Enable in Helm values
serviceMonitor:
  enabled: true
```

For manual Prometheus configuration, add a scrape config:

```yaml
scrape_configs:
  - job_name: kterodactyl
    scheme: https
    tls_config:
      insecure_skip_verify: true
    kubernetes_sd_configs:
      - role: service
        namespaces:
          names:
            - kterodactyl-system
    relabel_configs:
      - source_labels: [__meta_kubernetes_service_name]
        regex: kterodactyl-controller-manager-metrics-service
        action: keep
```

## Common Monitoring Scenarios

### Alert: High Error Rate

```promql
# Alert when API error rate exceeds 5% over 5 minutes
sum(rate(kterodactyl_http_requests_total{status_code=~"5.."}[5m]))
  / sum(rate(kterodactyl_http_requests_total[5m]))
  > 0.05
```

### Alert: Slow Reconciliation

```promql
# Alert when p99 reconciliation takes more than 5 seconds
histogram_quantile(0.99,
  sum(rate(kterodactyl_reconciliation_duration_seconds_bucket[5m])) by (le, controller)
) > 5
```

### Alert: Servers in Error State

```promql
# Alert when any servers are stuck in Error state
sum(kterodactyl_gameservers_by_state{state="Error"}) > 0
```

### Dashboard: API Overview

Key panels for a Grafana dashboard:

- **Request Rate**: `sum(rate(kterodactyl_http_requests_total[5m]))` (Graph)
- **Error Rate**: `sum(rate(kterodactyl_http_requests_total{status_code=~"5.."}[5m]))` (Graph)
- **p99 Latency**: `histogram_quantile(0.99, sum(rate(kterodactyl_http_request_duration_seconds_bucket[5m])) by (le))` (Graph)
- **In-Flight Requests**: `kterodactyl_http_requests_inflight` (Gauge)
- **Server Count**: `sum(kterodactyl_gameservers_by_state) by (state)` (Stacked bar)
