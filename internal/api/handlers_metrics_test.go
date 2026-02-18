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
	"net/http/httptest"
	"testing"

	gamev1alpha1 "github.com/kterodactyl/kterodactyl/api/v1alpha1"
	"github.com/kterodactyl/kterodactyl/internal/auth"
)

func TestHandleGetMetrics(t *testing.T) {
	t.Run("server not found returns 404", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/gameservers/nonexistent/metrics", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
		}
	})

	t.Run("metrics unavailable returns 503", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		// Create a Ready GameServer -- the handler will find it, then check metricsClient.
		// newTestServer() does NOT set metricsClient, so it defaults to nil.
		// The handler checks s.metricsClient == nil and returns 503.
		createTestGameServerWithState(t, ts.client, "ready-srv", "user-alice", "alice", "minecraft", gamev1alpha1.GameServerStateReady)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/gameservers/ready-srv/metrics", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected status %d, got %d: %s", http.StatusServiceUnavailable, rec.Code, rec.Body.String())
		}
	})

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		ts := newTestServer(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/gameservers/any-server/metrics", nil)
		// No auth header
		rec := ts.doRequest(req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, rec.Code, rec.Body.String())
		}
	})
}
