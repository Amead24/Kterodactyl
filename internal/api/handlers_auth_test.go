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
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kterodactyl/kterodactyl/internal/auth"
	"github.com/kterodactyl/kterodactyl/internal/util"
)

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

// jsonBody creates a bytes.Reader from a JSON-encoded map.
func jsonBody(t *testing.T, data map[string]string) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("failed to marshal JSON body: %v", err)
	}
	return bytes.NewReader(b)
}

func TestHandleLogin(t *testing.T) {
	tests := []struct {
		name           string
		body           map[string]string
		setupUser      bool
		expectedStatus int
		expectedError  string
		expectToken    bool
	}{
		{
			name:           "valid credentials",
			body:           map[string]string{"username": "alice", "password": "correctpassword"},
			setupUser:      true,
			expectedStatus: http.StatusOK,
			expectToken:    true,
		},
		{
			name:           "missing username",
			body:           map[string]string{"password": "somepassword"},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "username is required",
		},
		{
			name:           "missing password",
			body:           map[string]string{"username": "alice"},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "password is required",
		},
		{
			name:           "wrong password",
			body:           map[string]string{"username": "alice", "password": "wrongpassword"},
			setupUser:      true,
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "invalid credentials",
		},
		{
			name:           "unknown user",
			body:           map[string]string{"username": "nobody", "password": "somepassword"},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "invalid credentials",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := newTestServer(t)

			if tc.setupUser {
				createTestUser(t, ts, "alice", "alice@test.com", "correctpassword")
			}

			body := jsonBody(t, tc.body)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", body)
			req.Header.Set("Content-Type", "application/json")
			rec := ts.doRequest(req)

			if rec.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d: %s", tc.expectedStatus, rec.Code, rec.Body.String())
			}

			var resp map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			if tc.expectToken {
				if _, ok := resp["token"]; !ok {
					t.Error("expected token in response")
				}
			}

			if tc.expectedError != "" {
				if errMsg, ok := resp["error"].(string); !ok || errMsg != tc.expectedError {
					t.Errorf("expected error %q, got %q", tc.expectedError, resp["error"])
				}
			}
		})
	}
}

func TestHandleLoginInvalidJSON(t *testing.T) {
	ts := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader([]byte("{invalid")))
	req.Header.Set("Content-Type", "application/json")
	rec := ts.doRequest(req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleRegister(t *testing.T) {
	tests := []struct {
		name           string
		body           map[string]string
		setupInvite    bool
		expiredInvite  bool
		disableReg     bool
		setupExisting  bool
		expectedStatus int
		expectedError  string
		expectToken    bool
		expectUsername bool
	}{
		{
			name: "valid registration",
			body: map[string]string{
				"username":    "bob",
				"email":       "bob@test.com",
				"password":    "securepassword",
				"inviteToken": "PLACEHOLDER",
			},
			setupInvite:    true,
			expectedStatus: http.StatusCreated,
			expectToken:    true,
			expectUsername: true,
		},
		{
			name: "missing invite token",
			body: map[string]string{
				"username": "bob",
				"email":    "bob@test.com",
				"password": "securepassword",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "inviteToken is required",
		},
		{
			name: "invalid invite token",
			body: map[string]string{
				"username":    "bob",
				"email":       "bob@test.com",
				"password":    "securepassword",
				"inviteToken": "nonexistent-token",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid invite token",
		},
		{
			name: "expired invite",
			body: map[string]string{
				"username":    "bob",
				"email":       "bob@test.com",
				"password":    "securepassword",
				"inviteToken": "PLACEHOLDER",
			},
			setupInvite:    true,
			expiredInvite:  true,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invite token has expired",
		},
		{
			name: "username taken",
			body: map[string]string{
				"username":    "alice",
				"email":       "alice@test.com",
				"password":    "securepassword",
				"inviteToken": "PLACEHOLDER",
			},
			setupInvite:    true,
			setupExisting:  true,
			expectedStatus: http.StatusConflict,
			expectedError:  "username already taken",
		},
		{
			name: "invalid username (reserved)",
			body: map[string]string{
				"username":    "admin",
				"email":       "admin@test.com",
				"password":    "securepassword",
				"inviteToken": "PLACEHOLDER",
			},
			setupInvite:    true,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing fields",
			body: map[string]string{
				"username": "bob",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "registration disabled",
			body: map[string]string{
				"username":    "bob",
				"email":       "bob@test.com",
				"password":    "securepassword",
				"inviteToken": "some-token",
			},
			disableReg:     true,
			expectedStatus: http.StatusForbidden,
			expectedError:  "registration is disabled",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := newTestServer(t)

			if tc.disableReg {
				createAdminConfigMap(t, ts, false)
			}

			if tc.setupExisting {
				createTestUser(t, ts, "alice", "alice@test.com", "existingpass")
			}

			body := tc.body
			if tc.setupInvite {
				token := createTestInvite(t, ts, "bob@test.com", tc.expiredInvite)
				if body["inviteToken"] == "PLACEHOLDER" {
					bodyCopy := make(map[string]string, len(body))
					for k, v := range body {
						bodyCopy[k] = v
					}
					bodyCopy["inviteToken"] = token
					body = bodyCopy
				}
			}

			b := jsonBody(t, body)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", b)
			req.Header.Set("Content-Type", "application/json")
			rec := ts.doRequest(req)

			if rec.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d: %s", tc.expectedStatus, rec.Code, rec.Body.String())
			}

			var resp map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			if tc.expectToken {
				if _, ok := resp["token"]; !ok {
					t.Error("expected token in response")
				}
			}

			if tc.expectUsername {
				if username, ok := resp["username"].(string); !ok || username == "" {
					t.Error("expected username in response")
				}
			}

			if tc.expectedError != "" {
				if errMsg, ok := resp["error"].(string); !ok || errMsg != tc.expectedError {
					t.Errorf("expected error %q, got %q", tc.expectedError, resp["error"])
				}
			}
		})
	}
}

func TestHandleRefresh(t *testing.T) {
	ts := newTestServer(t)

	token := ts.generateToken(t, "alice", auth.RoleUser)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	addAuthHeader(req, token)
	rec := ts.doRequest(req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if _, ok := resp["token"]; !ok {
		t.Error("expected new token in response")
	}

	newToken, ok := resp["token"].(string)
	if !ok {
		t.Fatal("token is not a string")
	}
	if newToken == token {
		t.Error("expected refreshed token to be different from original")
	}
}
