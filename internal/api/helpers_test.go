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
	"os"
	"path/filepath"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gamev1alpha1 "github.com/kterodactyl/kterodactyl/api/v1alpha1"
	"github.com/kterodactyl/kterodactyl/internal/auth"
	"github.com/kterodactyl/kterodactyl/internal/manifest"
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

// createTestToken generates a valid JWT token for the given user details.
func createTestToken(t *testing.T, jwtSvc *auth.JWTService, username, email, role string) string {
	t.Helper()
	user := &auth.User{
		Username: username,
		Email:    email,
		Role:     role,
	}
	token, err := jwtSvc.GenerateToken(user)
	if err != nil {
		t.Fatalf("failed to generate test token: %v", err)
	}
	return token
}

// defaultTestManifestLoader creates a Loader with a minecraft manifest.
func defaultTestManifestLoader(t *testing.T) *manifest.Loader {
	t.Helper()
	dir := t.TempDir()

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
`
	if err := os.WriteFile(filepath.Join(dir, "minecraft.yaml"), []byte(minecraft), 0644); err != nil {
		t.Fatal(err)
	}

	loader, err := manifest.LoadFromDirectory(dir)
	if err != nil {
		t.Fatalf("failed to create test manifest loader: %v", err)
	}
	return loader
}
