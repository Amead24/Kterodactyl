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

func TestHandleListGames(t *testing.T) {
	ts := newTestServer(t)
	token := ts.generateToken(t, "alice", auth.RoleUser)

	t.Run("authenticated request returns all games", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/games", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var resp struct {
			Data  []GameResponse `json:"data"`
			Count int            `json:"count"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Count != 1 {
			t.Errorf("count = %d, want 1", resp.Count)
		}
		if len(resp.Data) != 1 {
			t.Fatalf("data length = %d, want 1", len(resp.Data))
		}

		mc := resp.Data[0]
		if mc.Name != "minecraft" {
			t.Errorf("data[0].name = %q, want %q", mc.Name, "minecraft")
		}
		if mc.DisplayName != "Minecraft Java Edition" {
			t.Errorf("data[0].displayName = %q, want %q", mc.DisplayName, "Minecraft Java Edition")
		}
		if mc.Image != "itzg/minecraft-server:latest" {
			t.Errorf("data[0].image = %q, want %q", mc.Image, "itzg/minecraft-server:latest")
		}
		if len(mc.Ports) != 1 {
			t.Fatalf("data[0].ports length = %d, want 1", len(mc.Ports))
		}
		if mc.Ports[0].ContainerPort != 25565 {
			t.Errorf("data[0].ports[0].containerPort = %d, want %d", mc.Ports[0].ContainerPort, 25565)
		}
		if mc.Ports[0].Protocol != "TCP" {
			t.Errorf("data[0].ports[0].protocol = %q, want %q", mc.Ports[0].Protocol, "TCP")
		}
		if mc.ParameterSchema == nil {
			t.Error("data[0].parameterSchema should not be nil")
		}
	})

	t.Run("unauthenticated request returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/games", nil)
		// No Authorization header
		rec := ts.doRequest(req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})
}

func TestHandleGetGame(t *testing.T) {
	ts := newTestServer(t)
	token := ts.generateToken(t, "bob", auth.RoleUser)

	t.Run("existing game type returns details with parameterSchema", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/games/minecraft", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var resp GameResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Name != "minecraft" {
			t.Errorf("name = %q, want %q", resp.Name, "minecraft")
		}
		if resp.DisplayName != "Minecraft Java Edition" {
			t.Errorf("displayName = %q, want %q", resp.DisplayName, "Minecraft Java Edition")
		}
		if resp.Image != "itzg/minecraft-server:latest" {
			t.Errorf("image = %q, want %q", resp.Image, "itzg/minecraft-server:latest")
		}
		if len(resp.Ports) != 1 {
			t.Fatalf("ports length = %d, want 1", len(resp.Ports))
		}
		if resp.Ports[0].Name != "game" {
			t.Errorf("ports[0].name = %q, want %q", resp.Ports[0].Name, "game")
		}
		if resp.Parameters["EULA"] != "TRUE" {
			t.Errorf("parameters[EULA] = %q, want %q", resp.Parameters["EULA"], "TRUE")
		}

		// Verify parameterSchema is present with expected properties
		if resp.ParameterSchema == nil {
			t.Fatal("parameterSchema should not be nil")
		}
		props, ok := resp.ParameterSchema["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("parameterSchema.properties should be a map")
		}
		if _, ok := props["EULA"]; !ok {
			t.Error("parameterSchema.properties should contain EULA")
		}
		if _, ok := props["TYPE"]; !ok {
			t.Error("parameterSchema.properties should contain TYPE")
		}
		if _, ok := props["MAX_PLAYERS"]; !ok {
			t.Error("parameterSchema.properties should contain MAX_PLAYERS")
		}
	})

	t.Run("unknown game type returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/games/nonexistent", nil)
		addAuthHeader(req, token)
		rec := ts.doRequest(req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
		}

		var errResp ErrorResponse
		if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
			t.Fatalf("failed to decode error response: %v", err)
		}

		if errResp.Error != "game type not found: nonexistent" {
			t.Errorf("error = %q, want %q", errResp.Error, "game type not found: nonexistent")
		}
	})
}
