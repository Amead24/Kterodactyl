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
	"net/http"

	"github.com/go-chi/chi/v5"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gamev1alpha1 "github.com/kterodactyl/kterodactyl/api/v1alpha1"
)

// MetricsResponse is the API response format for game server resource metrics.
type MetricsResponse struct {
	CPU            int64 `json:"cpu"`            // millicores
	MemoryMiB      int64 `json:"memoryMiB"`      // MiB
	CPULimit       int64 `json:"cpuLimit"`       // millicores from spec
	MemoryLimitMiB int64 `json:"memoryLimitMiB"` // MiB from spec
}

// handleGetMetrics returns CPU and memory usage from the Kubernetes Metrics API
// for the game server pod.
//
// GET /api/v1/gameservers/{name}/metrics
func (s *Server) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	ns := namespaceFromContext(r)
	if ns == "" {
		respondError(w, http.StatusUnauthorized, "no namespace in context")
		return
	}

	name := chi.URLParam(r, "name")
	ctx := r.Context()

	// Verify GameServer exists and belongs to user
	gs := &gamev1alpha1.GameServer{}
	if err := s.client.Get(ctx, client.ObjectKey{Name: name, Namespace: ns}, gs); err != nil {
		if k8serrors.IsNotFound(err) {
			respondError(w, http.StatusNotFound, "game server not found: "+name)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get game server")
		return
	}

	// Check if metrics client is available
	if s.metricsClient == nil {
		respondError(w, http.StatusServiceUnavailable, "metrics unavailable")
		return
	}

	// Fetch pod metrics from Metrics API
	podMetrics, err := s.metricsClient.MetricsV1beta1().PodMetricses(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			respondError(w, http.StatusNotFound, "metrics not found for server: "+name)
			return
		}
		// Metrics API unavailable (e.g., no metrics-server installed)
		respondError(w, http.StatusServiceUnavailable, "metrics unavailable")
		return
	}

	// Extract CPU and memory from the gameserver container
	var cpuMillis, memoryBytes int64
	for _, container := range podMetrics.Containers {
		if container.Name == "gameserver" {
			cpuMillis = container.Usage.Cpu().MilliValue()
			memoryBytes = container.Usage.Memory().Value()
			break
		}
	}

	// Extract resource limits from the GameServer spec
	var cpuLimitMillis, memoryLimitBytes int64
	if gs.Spec.Resources.Limits != nil {
		if cpuLimit, ok := gs.Spec.Resources.Limits[corev1.ResourceCPU]; ok {
			cpuLimitMillis = cpuLimit.MilliValue()
		}
		if memLimit, ok := gs.Spec.Resources.Limits[corev1.ResourceMemory]; ok {
			memoryLimitBytes = memLimit.Value()
		}
	}

	respondJSON(w, http.StatusOK, MetricsResponse{
		CPU:            cpuMillis,
		MemoryMiB:      memoryBytes / (1024 * 1024),
		CPULimit:       cpuLimitMillis,
		MemoryLimitMiB: memoryLimitBytes / (1024 * 1024),
	})
}
