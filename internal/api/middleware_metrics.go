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

package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kterodactyl/kterodactyl/internal/metrics"
)

// statusRecorder wraps http.ResponseWriter to capture the response status code
// for metrics recording. It does NOT implement http.Flusher, http.Hijacker, or
// http.Pusher because the WebSocket console route is mounted outside the REST
// route group and will never hit this middleware.
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code before delegating to the wrapped ResponseWriter.
func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// metricsMiddleware is a chi-compatible middleware that records HTTP request
// metrics using Prometheus counters, histograms, and gauges defined in the
// metrics package. It uses chi route patterns (e.g., /api/v1/gameservers/{name})
// as labels to ensure low cardinality.
func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metrics.HTTPRequestsInFlight.Inc()
		defer metrics.HTTPRequestsInFlight.Dec()

		rec := &statusRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // default if WriteHeader is never called
		}

		start := time.Now()
		next.ServeHTTP(rec, r)
		duration := time.Since(start).Seconds()

		// Extract chi route pattern for low-cardinality label
		routePattern := chi.RouteContext(r.Context()).RoutePattern()
		if routePattern == "" {
			routePattern = "unknown"
		}

		method := r.Method
		statusCode := fmt.Sprintf("%d", rec.statusCode)

		metrics.HTTPRequestsTotal.WithLabelValues(method, routePattern, statusCode).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(method, routePattern, statusCode).Observe(duration)
	})
}
