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
	"github.com/kterodactyl/kterodactyl/internal/util"
)

func TestHandleListMods(t *testing.T) {
	t.Run("server not found returns 404", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/gameservers/nonexistent/mods/", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
		}
	})

	t.Run("server not running returns 409", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		createTestGameServerWithState(t, ts.client, "shutdown-srv", "user-alice", "alice", "minecraft", gamev1alpha1.GameServerStateShutdown)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/gameservers/shutdown-srv/mods/", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected status %d, got %d: %s", http.StatusConflict, rec.Code, rec.Body.String())
		}
	})

	t.Run("no mod path returns 400", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		// Create a Ready server WITHOUT mod path annotation
		createTestGameServerWithState(t, ts.client, "no-mods-srv", "user-alice", "alice", "minecraft", gamev1alpha1.GameServerStateReady)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/gameservers/no-mods-srv/mods/", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
		}
	})

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		ts := newTestServer(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/gameservers/any-server/mods/", nil)
		// No auth header
		rec := ts.doRequest(req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, rec.Code, rec.Body.String())
		}
	})
}

func TestHandleUploadMod(t *testing.T) {
	t.Run("server not found returns 404", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/gameservers/nonexistent/mods/", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
		}
	})

	t.Run("server not running returns 409", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		createTestGameServerWithState(t, ts.client, "shutdown-srv", "user-alice", "alice", "minecraft", gamev1alpha1.GameServerStateShutdown)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/gameservers/shutdown-srv/mods/", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected status %d, got %d: %s", http.StatusConflict, rec.Code, rec.Body.String())
		}
	})

	t.Run("no mod path returns 400", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		// Create a Ready server WITHOUT mod path annotation
		createTestGameServerWithState(t, ts.client, "no-mods-srv", "user-alice", "alice", "minecraft", gamev1alpha1.GameServerStateReady)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/gameservers/no-mods-srv/mods/", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
		}
	})
}

func TestHandleDeleteMod(t *testing.T) {
	t.Run("server not found returns 404", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/gameservers/nonexistent/mods/test.jar", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
		}
	})

	t.Run("server not running returns 409", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		createTestGameServerWithState(t, ts.client, "shutdown-srv", "user-alice", "alice", "minecraft", gamev1alpha1.GameServerStateShutdown)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/gameservers/shutdown-srv/mods/test.jar", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected status %d, got %d: %s", http.StatusConflict, rec.Code, rec.Body.String())
		}
	})

	t.Run("no mod path returns 400", func(t *testing.T) {
		ts := newTestServer(t)
		token := ts.generateToken(t, "alice", auth.RoleUser)

		// Create a Ready server WITHOUT mod path annotation
		createTestGameServerWithState(t, ts.client, "no-mods-srv", "user-alice", "alice", "minecraft", gamev1alpha1.GameServerStateReady)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/gameservers/no-mods-srv/mods/test.jar", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
		}
	})
}

func TestParseLsOutput(t *testing.T) {
	t.Run("parses standard ls -la output", func(t *testing.T) {
		input := `total 24
drwxr-xr-x  2 root root 4096 Jan 10 12:00 .
drwxr-xr-x  5 root root 4096 Jan 10 12:00 ..
-rw-r--r--  1 root root 1234 Jan 10 12:00 example-mod.jar
-rw-r--r--  1 root root 5678 Jan 10 12:00 another-mod.jar`

		mods := parseLsOutput(input)

		if len(mods) != 2 {
			t.Fatalf("expected 2 mods, got %d", len(mods))
		}

		// Verify first file
		found := false
		for _, m := range mods {
			if m.Name == "example-mod.jar" {
				found = true
				if m.Size != 1234 {
					t.Errorf("expected size 1234 for example-mod.jar, got %d", m.Size)
				}
			}
		}
		if !found {
			t.Error("expected to find example-mod.jar in output")
		}

		// Verify second file
		found = false
		for _, m := range mods {
			if m.Name == "another-mod.jar" {
				found = true
				if m.Size != 5678 {
					t.Errorf("expected size 5678 for another-mod.jar, got %d", m.Size)
				}
			}
		}
		if !found {
			t.Error("expected to find another-mod.jar in output")
		}
	})

	t.Run("skips dot directories", func(t *testing.T) {
		input := `total 8
drwxr-xr-x  2 root root 4096 Jan 10 12:00 .
drwxr-xr-x  5 root root 4096 Jan 10 12:00 ..`

		mods := parseLsOutput(input)

		if len(mods) != 0 {
			t.Fatalf("expected 0 mods (dot dirs excluded), got %d", len(mods))
		}
	})

	t.Run("empty output", func(t *testing.T) {
		mods := parseLsOutput("")

		if mods != nil && len(mods) != 0 {
			t.Fatalf("expected nil or empty slice, got %d entries", len(mods))
		}
	})

	t.Run("single file", func(t *testing.T) {
		input := `total 4
-rw-r--r--  1 root root 9999 Jan 10 12:00 only-mod.jar`

		mods := parseLsOutput(input)

		if len(mods) != 1 {
			t.Fatalf("expected 1 mod, got %d", len(mods))
		}
		if mods[0].Name != "only-mod.jar" {
			t.Errorf("expected name %q, got %q", "only-mod.jar", mods[0].Name)
		}
		if mods[0].Size != 9999 {
			t.Errorf("expected size 9999, got %d", mods[0].Size)
		}
	})
}

// Ensure util.AnnotationModPath is used properly (compile-time check).
var _ = util.AnnotationModPath
