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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gamev1alpha1 "github.com/kterodactyl/kterodactyl/api/v1alpha1"
	"github.com/kterodactyl/kterodactyl/internal/auth"
	"github.com/kterodactyl/kterodactyl/internal/manifest"
	"github.com/kterodactyl/kterodactyl/internal/util"
)

const (
	testOperatorNS = "kterodactyl-system"
)

// testServer wraps a Server with test helpers for making authenticated requests.
type testServer struct {
	*Server
	jwtService *auth.JWTService
}

// newTestServer creates a Server with a real chi router, manifest loader,
// fake K8s client, and JWT service suitable for tests.
func newTestServer(t *testing.T) *testServer {
	t.Helper()

	// JWT service with a deterministic test key
	signingKey := []byte("test-signing-key-32-bytes-long!!")
	jwtSvc := auth.NewJWTService(signingKey, 24*time.Hour)

	// Fake K8s client with corev1 and gamev1alpha1 schemes
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = gamev1alpha1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	// InviteService (uses fake client)
	inviteSvc := auth.NewInviteService(fakeClient, testOperatorNS, nil, "https://panel.test")

	// UserStore
	userStore := auth.NewUserStore(fakeClient, testOperatorNS)

	// Manifest loader with minecraft
	loader := defaultTestManifestLoader(t)

	srv := NewServer(Config{
		Client:            fakeClient,
		JWTService:        jwtSvc,
		UserStore:         userStore,
		InviteService:     inviteSvc,
		ManifestLoader:    loader,
		OperatorNamespace: testOperatorNS,
		BindAddress:       ":0",
	})

	return &testServer{
		Server:     srv,
		jwtService: jwtSvc,
	}
}

// generateToken creates a signed JWT for the given user role and username.
func (ts *testServer) generateToken(t *testing.T, username, role string) string {
	t.Helper()
	user := &auth.User{
		Username: username,
		Email:    username + "@test.com",
		Role:     role,
	}
	token, err := ts.jwtService.GenerateToken(user)
	if err != nil {
		t.Fatalf("failed to generate test token: %v", err)
	}
	return token
}

// doRequest executes an HTTP request against the test server's router.
func (ts *testServer) doRequest(r *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	ts.router.ServeHTTP(rr, r)
	return rr
}

// addAuthHeader sets the Authorization: Bearer header on the given request.
func addAuthHeader(req *http.Request, token string) {
	req.Header.Set("Authorization", "Bearer "+token)
}

// jsonBody creates a bytes.Reader from a JSON-encoded map.
func jsonBody(t *testing.T, data map[string]string) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("failed to marshal JSON body: %v", err)
	}
	return bytes.NewReader(b)
}

// createTestUser creates a user in the UserStore with the given credentials.
func createTestUser(t *testing.T, ts *testServer, username, email, password string) {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	user := &auth.User{
		Username:     username,
		Email:        email,
		PasswordHash: hash,
		Role:         auth.RoleUser,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if err := ts.userStore.CreateUser(t.Context(), user); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
}

// createTestInvite creates an invite Secret in the fake K8s client and returns the token.
func createTestInvite(t *testing.T, ts *testServer, email string, expired bool) string {
	t.Helper()
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		t.Fatalf("failed to generate test invite token: %v", err)
	}
	token := hex.EncodeToString(tokenBytes)

	expiresAt := time.Now().Add(72 * time.Hour)
	if expired {
		expiresAt = time.Now().Add(-1 * time.Hour)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "invite-" + token[:12],
			Namespace: testOperatorNS,
			Labels: map[string]string{
				util.LabelManagedByKterodactyl: util.ManagedByValue,
				auth.LabelResourceType:         auth.ResourceTypeInvite,
			},
			Annotations: map[string]string{
				auth.AnnotationExpiresAt: expiresAt.Format(time.RFC3339),
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"token":      []byte(token),
			"email":      []byte(email),
			"invited-by": []byte("admin"),
		},
	}

	if err := ts.client.Create(t.Context(), secret); err != nil {
		t.Fatalf("failed to create test invite: %v", err)
	}

	return token
}

// createAdminConfigMap creates the admin ConfigMap in the test namespace.
func createAdminConfigMap(t *testing.T, ts *testServer, registrationEnabled bool) {
	t.Helper()
	enabled := "true"
	if !registrationEnabled {
		enabled = "false"
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kterodactyl-admin-config",
			Namespace: testOperatorNS,
		},
		Data: map[string]string{
			"registrationEnabled": enabled,
		},
	}
	if err := ts.client.Create(t.Context(), cm); err != nil {
		t.Fatalf("failed to create admin configmap: %v", err)
	}
}

// defaultTestManifestLoader creates a Loader with a minecraft manifest using the
// directory-per-game structure. The manifest includes a parameterSchema section
// with constraints for EULA (const), TYPE (enum), and MAX_PLAYERS (pattern).
func defaultTestManifestLoader(t *testing.T) *manifest.Loader {
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
