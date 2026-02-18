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

func TestHandleDeleteBackup(t *testing.T) {
	t.Run("deletes backup as admin", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "admin", auth.RoleAdmin)

		createTestGameServerWithState(t, ts.client, "mc-server", "user-admin", "admin", "minecraft", gamev1alpha1.GameServerStateReady)
		createTestBackup(t, ts.client, "mc-server-backup-1", "user-admin", "mc-server", gamev1alpha1.BackupStateCompleted)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/gameservers/mc-server/backups/mc-server-backup-1/", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
		}

		// Verify backup is gone from fake client
		backup := &gamev1alpha1.Backup{}
		err := ts.client.Get(t.Context(), client.ObjectKey{Name: "mc-server-backup-1", Namespace: "user-admin"}, backup)
		if err == nil {
			t.Error("expected backup to be deleted from K8s")
		}
	})

	t.Run("non-admin returns 403", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		createTestGameServerWithState(t, ts.client, "mc-server", "user-alice", "alice", "minecraft", gamev1alpha1.GameServerStateReady)
		createTestBackup(t, ts.client, "mc-server-backup-1", "user-alice", "mc-server", gamev1alpha1.BackupStateCompleted)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/gameservers/mc-server/backups/mc-server-backup-1/", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
		}
	})

	t.Run("backup not found returns 404", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "admin", auth.RoleAdmin)

		createTestGameServerWithState(t, ts.client, "mc-server", "user-admin", "admin", "minecraft", gamev1alpha1.GameServerStateReady)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/gameservers/mc-server/backups/nonexistent/", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
		}
	})

	t.Run("backup from different server returns 404", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "admin", auth.RoleAdmin)

		createTestGameServerWithState(t, ts.client, "server-a", "user-admin", "admin", "minecraft", gamev1alpha1.GameServerStateReady)
		createTestGameServerWithState(t, ts.client, "server-b", "user-admin", "admin", "minecraft", gamev1alpha1.GameServerStateReady)
		createTestBackup(t, ts.client, "server-a-backup", "user-admin", "server-a", gamev1alpha1.BackupStateCompleted)

		// Try to delete server-a's backup via server-b's route
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/gameservers/server-b/backups/server-a-backup/", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
		}
	})
}

func TestHandleRestoreBackup(t *testing.T) {
	t.Run("server not found returns 404", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "admin", auth.RoleAdmin)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/gameservers/nonexistent/backups/some-backup/restore", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
		}
	})

	t.Run("backup not found returns 404", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "admin", auth.RoleAdmin)

		createTestGameServerWithState(t, ts.client, "mc-server", "user-admin", "admin", "minecraft", gamev1alpha1.GameServerStateReady)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/gameservers/mc-server/backups/nonexistent/restore", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
		}
	})

	t.Run("backup not completed returns 409", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "admin", auth.RoleAdmin)

		createTestGameServerWithState(t, ts.client, "mc-server", "user-admin", "admin", "minecraft", gamev1alpha1.GameServerStateReady)
		createTestBackup(t, ts.client, "mc-server-backup", "user-admin", "mc-server", gamev1alpha1.BackupStatePending)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/gameservers/mc-server/backups/mc-server-backup/restore", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusConflict {
			t.Errorf("expected status %d, got %d: %s", http.StatusConflict, rec.Code, rec.Body.String())
		}
	})

	t.Run("server not running returns 409", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "admin", auth.RoleAdmin)

		createTestGameServerWithState(t, ts.client, "mc-server", "user-admin", "admin", "minecraft", gamev1alpha1.GameServerStateShutdown)
		createTestBackup(t, ts.client, "mc-server-backup", "user-admin", "mc-server", gamev1alpha1.BackupStateCompleted)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/gameservers/mc-server/backups/mc-server-backup/restore", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusConflict {
			t.Errorf("expected status %d, got %d: %s", http.StatusConflict, rec.Code, rec.Body.String())
		}
	})

	t.Run("non-admin returns 403", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		createTestGameServerWithState(t, ts.client, "mc-server", "user-alice", "alice", "minecraft", gamev1alpha1.GameServerStateReady)
		createTestBackup(t, ts.client, "mc-server-backup", "user-alice", "mc-server", gamev1alpha1.BackupStateCompleted)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/gameservers/mc-server/backups/mc-server-backup/restore", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
		}
	})
}

func TestHandleSetBackupSchedule(t *testing.T) {
	t.Run("sets schedule as admin", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "admin", auth.RoleAdmin)

		createTestGameServerWithState(t, ts.client, "mc-server", "user-admin", "admin", "minecraft", gamev1alpha1.GameServerStateReady)

		body, _ := json.Marshal(map[string]interface{}{
			"schedule":  "0 3 * * *",
			"retention": 5,
		})

		req := httptest.NewRequest(http.MethodPut, "/api/v1/gameservers/mc-server/backup-schedule", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp["schedule"] != "0 3 * * *" {
			t.Errorf("expected schedule %q, got %v", "0 3 * * *", resp["schedule"])
		}
		if resp["retention"] != float64(5) {
			t.Errorf("expected retention 5, got %v", resp["retention"])
		}
	})

	t.Run("removes schedule when empty", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "admin", auth.RoleAdmin)

		createTestGameServerWithAnnotations(t, ts.client, "mc-server", "user-admin", "admin", "minecraft",
			gamev1alpha1.GameServerStateReady, map[string]string{
				util.AnnotationBackupSchedule:  "0 3 * * *",
				util.AnnotationBackupRetention: "5",
			})

		body, _ := json.Marshal(map[string]interface{}{
			"schedule": "",
		})

		req := httptest.NewRequest(http.MethodPut, "/api/v1/gameservers/mc-server/backup-schedule", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp["schedule"] != "" {
			t.Errorf("expected empty schedule, got %v", resp["schedule"])
		}
	})

	t.Run("invalid cron returns 400", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "admin", auth.RoleAdmin)

		createTestGameServerWithState(t, ts.client, "mc-server", "user-admin", "admin", "minecraft", gamev1alpha1.GameServerStateReady)

		body, _ := json.Marshal(map[string]interface{}{
			"schedule": "invalid",
		})

		req := httptest.NewRequest(http.MethodPut, "/api/v1/gameservers/mc-server/backup-schedule", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
		}
	})

	t.Run("non-admin returns 403", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		body, _ := json.Marshal(map[string]interface{}{
			"schedule":  "0 3 * * *",
			"retention": 5,
		})

		req := httptest.NewRequest(http.MethodPut, "/api/v1/gameservers/mc-server/backup-schedule", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
		}
	})

	t.Run("server not found returns 404", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "admin", auth.RoleAdmin)

		body, _ := json.Marshal(map[string]interface{}{
			"schedule":  "0 3 * * *",
			"retention": 5,
		})

		req := httptest.NewRequest(http.MethodPut, "/api/v1/gameservers/nonexistent/backup-schedule", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
		}
	})
}
