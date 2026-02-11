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

	"github.com/kterodactyl/kterodactyl/internal/auth"
)

func TestHandleCreateInvite(t *testing.T) {
	t.Run("admin creates invite successfully", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "myadmin", auth.RoleAdmin)

		body := jsonBody(t, map[string]string{"email": "newuser@test.com"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/invites", body)
		req.Header.Set("Content-Type", "application/json")
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body.String())
		}

		var resp map[string]string
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp["token"] == "" {
			t.Error("expected non-empty token in response")
		}
		if resp["email"] != "newuser@test.com" {
			t.Errorf("email = %q, want %q", resp["email"], "newuser@test.com")
		}
		if resp["expiresAt"] == "" {
			t.Error("expected non-empty expiresAt in response")
		}
	})

	t.Run("non-admin user gets 403", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "regularuser", auth.RoleUser)

		body := jsonBody(t, map[string]string{"email": "newuser@test.com"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/invites", body)
		req.Header.Set("Content-Type", "application/json")
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
		}
	})

	t.Run("missing email returns 400", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "myadmin", auth.RoleAdmin)

		body := jsonBody(t, map[string]string{})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/invites", body)
		req.Header.Set("Content-Type", "application/json")
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d; body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
		}
	})
}

func TestHandleListUsers(t *testing.T) {
	t.Run("admin with users returns list", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "myadmin", auth.RoleAdmin)

		// Create two test users
		createTestUser(t, ts, "alice", "alice@test.com", "pass1")
		createTestUser(t, ts, "bob", "bob@test.com", "pass2")

		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var resp struct {
			Data  []map[string]interface{} `json:"data"`
			Count int                      `json:"count"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Count != 2 {
			t.Errorf("count = %d, want 2", resp.Count)
		}
		if len(resp.Data) != 2 {
			t.Fatalf("data length = %d, want 2", len(resp.Data))
		}

		// Verify passwordHash is NOT present in any user response
		for _, u := range resp.Data {
			if _, ok := u["passwordHash"]; ok {
				t.Errorf("response should NOT contain passwordHash, but found it for user %v", u["username"])
			}
			if _, ok := u["password_hash"]; ok {
				t.Errorf("response should NOT contain password_hash, but found it for user %v", u["username"])
			}
		}
	})

	t.Run("non-admin gets 403", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "regularuser", auth.RoleUser)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
		}
	})
}

func TestHandleDeleteUser(t *testing.T) {
	t.Run("admin deletes existing user", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "myadmin", auth.RoleAdmin)

		// Create a user to delete
		createTestUser(t, ts, "alice", "alice@test.com", "pass1")

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/alice", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusNoContent {
			t.Errorf("status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body.String())
		}
	})

	t.Run("admin deletes non-existent user returns 404", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "myadmin", auth.RoleAdmin)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/ghost", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
		}

		var errResp ErrorResponse
		if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
			t.Fatalf("failed to decode error response: %v", err)
		}
		if errResp.Error != "user not found" {
			t.Errorf("error = %q, want %q", errResp.Error, "user not found")
		}
	})

	t.Run("admin cannot delete self", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "myadmin", auth.RoleAdmin)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/myadmin", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}

		var errResp ErrorResponse
		if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
			t.Fatalf("failed to decode error response: %v", err)
		}
		if errResp.Error != "cannot delete yourself" {
			t.Errorf("error = %q, want %q", errResp.Error, "cannot delete yourself")
		}
	})

	t.Run("non-admin gets 403", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "regularuser", auth.RoleUser)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/alice", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
		}
	})
}
