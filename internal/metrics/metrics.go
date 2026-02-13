/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package metrics defines all Prometheus metrics for the kterodactyl operator
// and API server. Metrics are registered with the controller-runtime metrics
// registry (not the default prometheus registry) so they are served on the
// manager's metrics endpoint.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// GameServersByState tracks the current count of GameServer resources
	// by lifecycle state and game type. Labels are low-cardinality:
	// state (6 enum values), game_type (bounded by game definitions).
	GameServersByState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kterodactyl_gameservers_by_state",
			Help: "Number of GameServer resources by state and game type.",
		},
		[]string{"state", "game_type"},
	)

	// ReconciliationDuration tracks the duration of reconciliation loops
	// in seconds. Labels: controller (gameserver, backup).
	ReconciliationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kterodactyl_reconciliation_duration_seconds",
			Help:    "Duration of reconciliation in seconds.",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
		},
		[]string{"controller"},
	)

	// HTTPRequestsTotal tracks the total number of HTTP requests handled
	// by the API server. Labels: method, route (chi route pattern), status_code.
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kterodactyl_http_requests_total",
			Help: "Total number of HTTP requests by method, route pattern, and status code.",
		},
		[]string{"method", "route", "status_code"},
	)

	// HTTPRequestDuration tracks HTTP request latency in seconds.
	// Labels: method, route (chi route pattern), status_code.
	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kterodactyl_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route", "status_code"},
	)

	// HTTPRequestsInFlight tracks the number of HTTP requests currently
	// being served concurrently.
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
