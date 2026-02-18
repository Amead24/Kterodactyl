//go:build integration

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

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gamev1alpha1 "github.com/kterodactyl/kterodactyl/api/v1alpha1"
	"github.com/kterodactyl/kterodactyl/internal/api"
	"github.com/kterodactyl/kterodactyl/internal/auth"
	"github.com/kterodactyl/kterodactyl/internal/manifest"
)

const testOperatorNS = "kterodactyl-system"

// setupTestServer creates an httptest.Server wrapping the full API router with
// all middleware (auth, rate limiting, CORS, etc.) and fake K8s backends.
// Returns the live test server and the invite service for pre-seeding invites.
func setupTestServer(t *testing.T) (*httptest.Server, *auth.InviteService) {
	t.Helper()

	// JWT service with a deterministic test key
	signingKey := []byte("test-signing-key-32-bytes-long!!")
	jwtSvc := auth.NewJWTService(signingKey, 24*time.Hour)

	// Fake K8s client with corev1 and gamev1alpha1 schemes
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = gamev1alpha1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&gamev1alpha1.GameServer{}).
		Build()

	// InviteService (uses fake client, nil SMTP)
	inviteSvc := auth.NewInviteService(fakeClient, testOperatorNS, nil, "https://panel.test")

	// UserStore
	userStore := auth.NewUserStore(fakeClient, testOperatorNS)

	// Manifest loader with minecraft game type
	loader := createTestManifestLoader(t)

	srv := api.NewServer(api.Config{
		Client:            fakeClient,
		JWTService:        jwtSvc,
		UserStore:         userStore,
		InviteService:     inviteSvc,
		ManifestLoader:    loader,
		OperatorNamespace: testOperatorNS,
		BindAddress:       ":0",
	})

	ts := httptest.NewServer(srv.HTTPServer().Handler)
	t.Cleanup(ts.Close)

	return ts, inviteSvc
}

// createTestManifestLoader creates a manifest.Loader with a minecraft manifest
// including parameterSchema for EULA (const) and TYPE (enum) validation.
func createTestManifestLoader(t *testing.T) *manifest.Loader {
	t.Helper()
	dir := t.TempDir()

	// Create minecraft subdirectory (directory-per-game structure)
	mcDir := filepath.Join(dir, "minecraft")
	if err := os.MkdirAll(mcDir, 0755); err != nil {
		t.Fatal(err)
	}

	minecraft := `name: minecraft
displayName: Minecraft Java Edition
image: itzg/minecraft-server:latest
ports:
  - name: game
    containerPort: 25565
    protocol: TCP
parameters:
  EULA: "TRUE"
  TYPE: VANILLA
resources:
  requests:
    memory: "1Gi"
    cpu: "500m"
  limits:
    memory: "2Gi"
    cpu: "1000m"
parameterSchema:
  type: object
  properties:
    EULA:
      type: string
      title: "EULA Agreement"
      description: "Must be TRUE to accept the Minecraft EULA"
      const: "TRUE"
    TYPE:
      type: string
      title: "Server Type"
      description: "Minecraft server implementation"
      enum: ["VANILLA", "PAPER", "SPIGOT"]
      default: "VANILLA"
    MAX_PLAYERS:
      type: string
      title: "Max Players"
      description: "Maximum number of concurrent players"
      pattern: "^[1-9][0-9]*$"
      default: "20"
  required:
    - EULA
    - TYPE
`
	if err := os.WriteFile(filepath.Join(mcDir, "manifest.yaml"), []byte(minecraft), 0644); err != nil {
		t.Fatal(err)
	}

	loader, err := manifest.LoadFromDirectory(dir)
	if err != nil {
		t.Fatalf("failed to create test manifest loader: %v", err)
	}
	return loader
}

// jsonPost marshals body to JSON and POSTs it to the given URL.
func jsonPost(t *testing.T, client *http.Client, url string, body interface{}) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal body: %v", err)
	}
	resp, err := client.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST %s failed: %v", url, err)
	}
	return resp
}

// jsonPostAuth marshals body to JSON and POSTs it with an Authorization: Bearer header.
func jsonPostAuth(t *testing.T, client *http.Client, url string, body interface{}, token string) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST %s failed: %v", url, err)
	}
	return resp
}

// jsonGetAuth sends a GET request with an Authorization: Bearer header.
func jsonGetAuth(t *testing.T, client *http.Client, url, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s failed: %v", url, err)
	}
	return resp
}

// jsonDeleteAuth sends a DELETE request with an Authorization: Bearer header.
func jsonDeleteAuth(t *testing.T, client *http.Client, url, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s failed: %v", url, err)
	}
	return resp
}

// assertStatus checks the response status code and fatals with body diagnostics on mismatch.
func assertStatus(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status %d, got %d: %s", expected, resp.StatusCode, string(body))
	}
}

// decodeJSONResponse decodes the response body into a map for blackbox assertion.
func decodeJSONResponse(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	defer resp.Body.Close()
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
	return result
}

// TestAPILifecycle exercises the full API lifecycle via real HTTP round-trips:
// register -> create game server -> get game server -> delete game server -> verify deleted.
func TestAPILifecycle(t *testing.T) {
	ts, inviteSvc := setupTestServer(t)
	client := ts.Client()

	// Pre-seed: create invite token for registration
	ctx := context.Background()
	invite, err := inviteSvc.CreateInvite(ctx, "alice@test.com", "bootstrap", 72)
	if err != nil {
		t.Fatalf("failed to create invite: %v", err)
	}

	// Step 1: Register user
	regBody := map[string]string{
		"username":    "alice",
		"email":       "alice@test.com",
		"password":    "securepassword123",
		"inviteToken": invite.Token,
	}
	resp := jsonPost(t, client, ts.URL+"/api/v1/auth/register", regBody)
	assertStatus(t, resp, http.StatusCreated)
	regResult := decodeJSONResponse(t, resp)
	token, ok := regResult["token"].(string)
	if !ok || token == "" {
		t.Fatal("expected non-empty JWT token from registration response")
	}
	t.Log("Step 1 PASS: Register returned 201 with JWT token")

	// Step 2: Create game server
	createBody := map[string]interface{}{
		"name":     "my-mc-server",
		"gameType": "minecraft",
		"parameters": map[string]string{
			"EULA": "TRUE",
			"TYPE": "VANILLA",
		},
	}
	resp = jsonPostAuth(t, client, ts.URL+"/api/v1/gameservers", createBody, token)
	assertStatus(t, resp, http.StatusCreated)
	createResult := decodeJSONResponse(t, resp)
	if createResult["name"] != "my-mc-server" {
		t.Errorf("expected name 'my-mc-server', got %v", createResult["name"])
	}
	if createResult["gameType"] != "minecraft" {
		t.Errorf("expected gameType 'minecraft', got %v", createResult["gameType"])
	}
	t.Log("Step 2 PASS: Create game server returned 201 with correct name and gameType")

	// Step 3: Get game server
	resp = jsonGetAuth(t, client, ts.URL+"/api/v1/gameservers/my-mc-server", token)
	assertStatus(t, resp, http.StatusOK)
	getResult := decodeJSONResponse(t, resp)
	if getResult["name"] != "my-mc-server" {
		t.Errorf("expected name 'my-mc-server', got %v", getResult["name"])
	}
	if getResult["gameType"] != "minecraft" {
		t.Errorf("expected gameType 'minecraft', got %v", getResult["gameType"])
	}
	t.Log("Step 3 PASS: Get game server returned 200 with correct fields")

	// Step 4: Delete game server
	resp = jsonDeleteAuth(t, client, ts.URL+"/api/v1/gameservers/my-mc-server", token)
	assertStatus(t, resp, http.StatusNoContent)
	t.Log("Step 4 PASS: Delete game server returned 204")

	// Step 5: Verify deleted
	resp = jsonGetAuth(t, client, ts.URL+"/api/v1/gameservers/my-mc-server", token)
	assertStatus(t, resp, http.StatusNotFound)
	t.Log("Step 5 PASS: Get deleted game server returned 404")
}
