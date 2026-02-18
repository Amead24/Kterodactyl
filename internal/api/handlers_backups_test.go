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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	gamev1alpha1 "github.com/kterodactyl/kterodactyl/api/v1alpha1"
	"github.com/kterodactyl/kterodactyl/internal/auth"
	"github.com/kterodactyl/kterodactyl/internal/util"
)

func TestHandleCreateBackup(t *testing.T) {
	t.Run("creates backup for ready server", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		createTestGameServerWithState(t, ts.client, "mc-server", "user-alice", "alice", "minecraft", gamev1alpha1.GameServerStateReady)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/gameservers/mc-server/backups", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
		}

		var resp BackupResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.GameServerName != "mc-server" {
			t.Errorf("expected GameServerName %q, got %q", "mc-server", resp.GameServerName)
		}
		if resp.State != "Pending" {
			t.Errorf("expected State %q, got %q", "Pending", resp.State)
		}

		// Verify the Backup CR was actually created in the fake client
		backupList := &gamev1alpha1.BackupList{}
		if err := ts.client.List(t.Context(), backupList, client.InNamespace("user-alice"),
			client.MatchingLabels{util.LabelBackupGameServer: "mc-server"}); err != nil {
			t.Fatalf("failed to list backups: %v", err)
		}
		if len(backupList.Items) != 1 {
			t.Fatalf("expected 1 backup CR, got %d", len(backupList.Items))
		}
	})

	t.Run("server not found returns 404", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/gameservers/nonexistent/backups", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
		}
	})

	t.Run("server not running returns 409", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		createTestGameServerWithState(t, ts.client, "stopped-server", "user-alice", "alice", "minecraft", gamev1alpha1.GameServerStateShutdown)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/gameservers/stopped-server/backups", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusConflict {
			t.Errorf("expected status %d, got %d: %s", http.StatusConflict, rec.Code, rec.Body.String())
		}
	})

	t.Run("backup already in progress returns 409", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		createTestGameServerWithState(t, ts.client, "mc-server", "user-alice", "alice", "minecraft", gamev1alpha1.GameServerStateReady)
		createTestBackup(t, ts.client, "mc-server-backup-existing", "user-alice", "mc-server", gamev1alpha1.BackupStateInProgress)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/gameservers/mc-server/backups", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusConflict {
			t.Errorf("expected status %d, got %d: %s", http.StatusConflict, rec.Code, rec.Body.String())
		}
	})
}

func TestHandleListBackups(t *testing.T) {
	t.Run("returns empty list", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		createTestGameServerWithState(t, ts.client, "mc-server", "user-alice", "alice", "minecraft", gamev1alpha1.GameServerStateReady)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/gameservers/mc-server/backups", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
		}

		var resp struct {
			Data  []BackupResponse `json:"data"`
			Count int              `json:"count"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Count != 0 {
			t.Errorf("expected count 0, got %d", resp.Count)
		}
	})

	t.Run("returns sorted backups", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		createTestGameServerWithState(t, ts.client, "mc-server", "user-alice", "alice", "minecraft", gamev1alpha1.GameServerStateReady)
		createTestBackup(t, ts.client, "mc-server-backup-1", "user-alice", "mc-server", gamev1alpha1.BackupStateCompleted)
		createTestBackup(t, ts.client, "mc-server-backup-2", "user-alice", "mc-server", gamev1alpha1.BackupStatePending)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/gameservers/mc-server/backups", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
		}

		var resp struct {
			Data  []BackupResponse `json:"data"`
			Count int              `json:"count"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Count != 2 {
			t.Errorf("expected count 2, got %d", resp.Count)
		}
		if len(resp.Data) != 2 {
			t.Errorf("expected 2 items in data, got %d", len(resp.Data))
		}
	})

	t.Run("server not found returns 404", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/gameservers/nonexistent/backups", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
		}
	})
}
