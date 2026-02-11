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
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gamev1alpha1 "github.com/kterodactyl/kterodactyl/api/v1alpha1"
	"github.com/kterodactyl/kterodactyl/internal/auth"
	"github.com/kterodactyl/kterodactyl/internal/util"
)

// createTestGameServer creates a GameServer CR in the fake K8s client.
func createTestGameServer(t *testing.T, k8sClient client.Client, name, namespace, owner, gameType string) {
	t.Helper()
	gs := &gamev1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    util.GameServerLabels(owner, gameType),
		},
		Spec: gamev1alpha1.GameServerSpec{
			GameType: gameType,
			Image:    "itzg/minecraft-server:latest",
			Ports: []gamev1alpha1.GameServerPort{
				{Name: "game", ContainerPort: 25565, Protocol: corev1.ProtocolTCP},
			},
			Parameters: map[string]string{
				"EULA": "TRUE",
			},
		},
	}
	if err := k8sClient.Create(t.Context(), gs); err != nil {
		t.Fatalf("failed to create test gameserver: %v", err)
	}
}

func TestHandleListGameServers(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/gameservers/", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
		}

		var resp struct {
			Data  []GameServerResponse `json:"data"`
			Count int                  `json:"count"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Count != 0 {
			t.Errorf("expected count 0, got %d", resp.Count)
		}
		if len(resp.Data) != 0 {
			t.Errorf("expected empty data, got %d items", len(resp.Data))
		}
	})

	t.Run("two servers in user namespace", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		// Create two GameServers in alice's namespace
		createTestGameServer(t, ts.client, "mc-server-1", "user-alice", "alice", "minecraft")
		createTestGameServer(t, ts.client, "mc-server-2", "user-alice", "alice", "minecraft")

		req := httptest.NewRequest(http.MethodGet, "/api/v1/gameservers/", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
		}

		var resp struct {
			Data  []GameServerResponse `json:"data"`
			Count int                  `json:"count"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Count != 2 {
			t.Errorf("expected count 2, got %d", resp.Count)
		}
	})

	t.Run("servers in other namespace not returned (namespace isolation)", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		// Create a server in bob's namespace -- should NOT appear in alice's list
		createTestGameServer(t, ts.client, "bob-server", "user-bob", "bob", "minecraft")
		// Create a server in alice's namespace -- should appear
		createTestGameServer(t, ts.client, "alice-server", "user-alice", "alice", "minecraft")

		req := httptest.NewRequest(http.MethodGet, "/api/v1/gameservers/", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
		}

		var resp struct {
			Data  []GameServerResponse `json:"data"`
			Count int                  `json:"count"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Count != 1 {
			t.Errorf("expected count 1 (only alice's server), got %d", resp.Count)
		}
		if len(resp.Data) > 0 && resp.Data[0].Name != "alice-server" {
			t.Errorf("expected alice-server, got %q", resp.Data[0].Name)
		}
	})
}

func TestHandleCreateGameServer(t *testing.T) {
	t.Run("valid creation", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		body := map[string]interface{}{
			"name":       "my-mc",
			"gameType":   "minecraft",
			"parameters": map[string]string{"DIFFICULTY": "hard"},
		}
		b, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/gameservers/", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
		}

		var resp GameServerResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Name != "my-mc" {
			t.Errorf("expected name %q, got %q", "my-mc", resp.Name)
		}
		if resp.GameType != "minecraft" {
			t.Errorf("expected gameType %q, got %q", "minecraft", resp.GameType)
		}
		if resp.CreatedAt == "" {
			t.Error("expected non-empty createdAt")
		}
		// Verify parameters include both manifest defaults and user overrides
		if resp.Parameters["EULA"] != "TRUE" {
			t.Errorf("expected EULA=TRUE from manifest defaults, got %q", resp.Parameters["EULA"])
		}
		if resp.Parameters["DIFFICULTY"] != "hard" {
			t.Errorf("expected DIFFICULTY=hard from user override, got %q", resp.Parameters["DIFFICULTY"])
		}

		// Verify the GameServer was actually created in K8s with correct labels
		gs := &gamev1alpha1.GameServer{}
		if err := ts.client.Get(t.Context(), client.ObjectKey{Name: "my-mc", Namespace: "user-alice"}, gs); err != nil {
			t.Fatalf("failed to get created gameserver from K8s: %v", err)
		}
		if gs.Labels[util.LabelOwner] != "alice" {
			t.Errorf("expected owner label %q, got %q", "alice", gs.Labels[util.LabelOwner])
		}
		if gs.Labels[util.LabelGame] != "minecraft" {
			t.Errorf("expected game label %q, got %q", "minecraft", gs.Labels[util.LabelGame])
		}
	})

	t.Run("unknown game type", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		body := map[string]string{"name": "my-server", "gameType": "unknowngame"}
		b, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/gameservers/", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
		}

		var resp ErrorResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode error: %v", err)
		}
		if resp.Error != "unknown game type: unknowngame" {
			t.Errorf("expected error %q, got %q", "unknown game type: unknowngame", resp.Error)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		body := map[string]string{"gameType": "minecraft"}
		b, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/gameservers/", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
		}
	})

	t.Run("duplicate name", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		// Pre-create a server with the same name
		createTestGameServer(t, ts.client, "existing", "user-alice", "alice", "minecraft")

		body := map[string]string{"name": "existing", "gameType": "minecraft"}
		b, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/gameservers/", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusConflict {
			t.Errorf("expected status %d, got %d: %s", http.StatusConflict, rec.Code, rec.Body.String())
		}

		var resp ErrorResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode error: %v", err)
		}
		if resp.Error != "game server with this name already exists" {
			t.Errorf("expected error about duplicate name, got %q", resp.Error)
		}
	})
}

func TestHandleGetGameServer(t *testing.T) {
	t.Run("existing server", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		createTestGameServer(t, ts.client, "my-server", "user-alice", "alice", "minecraft")

		req := httptest.NewRequest(http.MethodGet, "/api/v1/gameservers/my-server/", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
		}

		var resp GameServerResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Name != "my-server" {
			t.Errorf("expected name %q, got %q", "my-server", resp.Name)
		}
		if resp.GameType != "minecraft" {
			t.Errorf("expected gameType %q, got %q", "minecraft", resp.GameType)
		}
	})

	t.Run("not found", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/gameservers/nonexistent/", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
		}
	})
}

func TestHandleUpdateGameServer(t *testing.T) {
	t.Run("valid update", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		createTestGameServer(t, ts.client, "my-server", "user-alice", "alice", "minecraft")

		body := map[string]interface{}{
			"parameters": map[string]string{"DIFFICULTY": "peaceful"},
		}
		b, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/gameservers/my-server/", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
		}

		var resp GameServerResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Original parameter should be preserved
		if resp.Parameters["EULA"] != "TRUE" {
			t.Errorf("expected EULA=TRUE preserved, got %q", resp.Parameters["EULA"])
		}
		// New parameter should be added
		if resp.Parameters["DIFFICULTY"] != "peaceful" {
			t.Errorf("expected DIFFICULTY=peaceful, got %q", resp.Parameters["DIFFICULTY"])
		}
	})

	t.Run("not found", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		body := map[string]interface{}{
			"parameters": map[string]string{"DIFFICULTY": "hard"},
		}
		b, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/gameservers/nonexistent/", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
		}
	})
}

func TestHandleDeleteGameServer(t *testing.T) {
	t.Run("existing server", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		createTestGameServer(t, ts.client, "my-server", "user-alice", "alice", "minecraft")

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/gameservers/my-server/", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusNoContent {
			t.Errorf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
		}

		// Verify it was actually deleted
		gs := &gamev1alpha1.GameServer{}
		err := ts.client.Get(t.Context(), client.ObjectKey{Name: "my-server", Namespace: "user-alice"}, gs)
		if err == nil {
			t.Error("expected GameServer to be deleted from K8s")
		}
	})

	t.Run("not found", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/gameservers/nonexistent/", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
		}
	})
}
