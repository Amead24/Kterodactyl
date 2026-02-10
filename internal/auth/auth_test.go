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

package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kterodactyl/kterodactyl/internal/util"
)

// ============================================================================
// Password Tests
// ============================================================================

func TestHashPassword_ProducesPHCFormat(t *testing.T) {
	hash, err := HashPassword("testpassword123")
	if err != nil {
		t.Fatalf("HashPassword() returned error: %v", err)
	}

	// PHC format: $argon2id$v=19$m=65536,t=1,p=4$<base64-salt>$<base64-hash>
	pattern := regexp.MustCompile(`^\$argon2id\$v=19\$m=65536,t=1,p=4\$.+\$.+$`)
	if !pattern.MatchString(hash) {
		t.Errorf("HashPassword() = %q, does not match PHC format pattern", hash)
	}
}

func TestHashPassword_UniqueSalts(t *testing.T) {
	hash1, err := HashPassword("samepassword")
	if err != nil {
		t.Fatalf("HashPassword() first call returned error: %v", err)
	}

	hash2, err := HashPassword("samepassword")
	if err != nil {
		t.Fatalf("HashPassword() second call returned error: %v", err)
	}

	if hash1 == hash2 {
		t.Error("HashPassword() produced identical hashes for same password (salts should differ)")
	}
}

func TestVerifyPassword_CorrectPassword(t *testing.T) {
	password := "correcthorsebatterystaple"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() returned error: %v", err)
	}

	ok, err := VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("VerifyPassword() returned error: %v", err)
	}
	if !ok {
		t.Error("VerifyPassword() returned false for correct password")
	}
}

func TestVerifyPassword_WrongPassword(t *testing.T) {
	hash, err := HashPassword("realpassword")
	if err != nil {
		t.Fatalf("HashPassword() returned error: %v", err)
	}

	ok, err := VerifyPassword("wrongpassword", hash)
	if err != nil {
		t.Fatalf("VerifyPassword() returned error: %v", err)
	}
	if ok {
		t.Error("VerifyPassword() returned true for wrong password")
	}
}

func TestVerifyPassword_InvalidFormat(t *testing.T) {
	_, err := VerifyPassword("anypassword", "notavalidhash")
	if err == nil {
		t.Error("VerifyPassword() should return error for invalid hash format")
	}
}

// ============================================================================
// Username Validation Tests
// ============================================================================

func TestValidateUsername_ValidNames(t *testing.T) {
	validNames := []string{
		"alice",
		"bob-123",
		"a",
		"a-b-c",
		"player1",
		"test-user-42",
	}

	for _, name := range validNames {
		t.Run(name, func(t *testing.T) {
			if err := ValidateUsername(name); err != nil {
				t.Errorf("ValidateUsername(%q) returned error: %v", name, err)
			}
		})
	}
}

func TestValidateUsername_InvalidNames(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		reason string
	}{
		{"empty", "", "empty string"},
		{"uppercase", "UPPERCASE", "contains uppercase letters"},
		{"starts-with-dash", "-starts", "starts with dash"},
		{"ends-with-dash", "ends-", "ends with dash"},
		{"has-spaces", "has spaces", "contains spaces"},
		{"has-dots", "has.dots", "contains dots"},
		{"too-long", strings.Repeat("a", 64), "exceeds 63 character limit"},
		{"has-underscore", "has_underscore", "contains underscore"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUsername(tt.input)
			if err == nil {
				t.Errorf("ValidateUsername(%q) should reject: %s", tt.input, tt.reason)
			}
		})
	}
}

func TestValidateUsername_ReservedNames(t *testing.T) {
	reservedNames := []string{
		"admin",
		"system",
		"operator",
		"default",
		"kube",
	}

	for _, name := range reservedNames {
		t.Run(name, func(t *testing.T) {
			err := ValidateUsername(name)
			if err == nil {
				t.Errorf("ValidateUsername(%q) should reject reserved name", name)
			}
			if !strings.Contains(err.Error(), "reserved") {
				t.Errorf("ValidateUsername(%q) error should mention 'reserved', got: %v", name, err)
			}
		})
	}
}

// ============================================================================
// JWT Tests
// ============================================================================

func newTestJWTService(expiration time.Duration) *JWTService {
	// Use a deterministic test key
	key := []byte("test-signing-key-32-bytes-long!!")
	return NewJWTService(key, expiration)
}

func newTestUser() *User {
	return &User{
		Username: "alice",
		Email:    "alice@example.com",
		Role:     RoleUser,
	}
}

func TestJWTService_GenerateAndValidate(t *testing.T) {
	svc := newTestJWTService(24 * time.Hour)
	user := newTestUser()

	token, err := svc.GenerateToken(user)
	if err != nil {
		t.Fatalf("GenerateToken() returned error: %v", err)
	}

	if token == "" {
		t.Fatal("GenerateToken() returned empty token")
	}

	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken() returned error: %v", err)
	}

	if claims.Username != user.Username {
		t.Errorf("claims.Username = %q, want %q", claims.Username, user.Username)
	}
	if claims.Email != user.Email {
		t.Errorf("claims.Email = %q, want %q", claims.Email, user.Email)
	}
	if claims.Role != user.Role {
		t.Errorf("claims.Role = %q, want %q", claims.Role, user.Role)
	}
	if claims.Issuer != "kterodactyl" {
		t.Errorf("claims.Issuer = %q, want %q", claims.Issuer, "kterodactyl")
	}
}

func TestJWTService_ExpiredToken(t *testing.T) {
	// Create a service with 0-duration expiration (immediately expired)
	svc := newTestJWTService(0)
	user := newTestUser()

	token, err := svc.GenerateToken(user)
	if err != nil {
		t.Fatalf("GenerateToken() returned error: %v", err)
	}

	// Wait a tiny bit to ensure the token is expired
	time.Sleep(2 * time.Millisecond)

	_, err = svc.ValidateToken(token)
	if err == nil {
		t.Error("ValidateToken() should return error for expired token")
	}
}

func TestJWTService_InvalidSigningKey(t *testing.T) {
	key1 := []byte("key-one-that-is-32-bytes-long!!!")
	key2 := []byte("key-two-that-is-32-bytes-long!!!")

	svc1 := NewJWTService(key1, 24*time.Hour)
	svc2 := NewJWTService(key2, 24*time.Hour)

	user := newTestUser()

	token, err := svc1.GenerateToken(user)
	if err != nil {
		t.Fatalf("GenerateToken() returned error: %v", err)
	}

	_, err = svc2.ValidateToken(token)
	if err == nil {
		t.Error("ValidateToken() should return error when validating with different signing key")
	}
}

func TestJWTService_ShouldRefresh(t *testing.T) {
	// Service with 2-hour refresh threshold (default)
	svc := newTestJWTService(24 * time.Hour)
	user := newTestUser()

	// Token expiring in 24 hours should NOT need refresh
	token, err := svc.GenerateToken(user)
	if err != nil {
		t.Fatalf("GenerateToken() returned error: %v", err)
	}
	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken() returned error: %v", err)
	}
	if svc.ShouldRefresh(claims) {
		t.Error("ShouldRefresh() should return false for token expiring in 24 hours")
	}

	// Token expiring in 1 hour should need refresh (within 2-hour threshold)
	nearExpirySvc := newTestJWTService(1 * time.Hour)
	nearToken, err := nearExpirySvc.GenerateToken(user)
	if err != nil {
		t.Fatalf("GenerateToken() returned error: %v", err)
	}
	nearClaims, err := nearExpirySvc.ValidateToken(nearToken)
	if err != nil {
		t.Fatalf("ValidateToken() returned error: %v", err)
	}
	if !svc.ShouldRefresh(nearClaims) {
		t.Error("ShouldRefresh() should return true for token expiring in 1 hour")
	}
}

func TestJWTService_NamespaceClaim(t *testing.T) {
	svc := newTestJWTService(24 * time.Hour)
	user := newTestUser()

	token, err := svc.GenerateToken(user)
	if err != nil {
		t.Fatalf("GenerateToken() returned error: %v", err)
	}

	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken() returned error: %v", err)
	}

	expectedNamespace := util.UserNamespace(user.Username)
	if claims.Namespace != expectedNamespace {
		t.Errorf("claims.Namespace = %q, want %q", claims.Namespace, expectedNamespace)
	}
}

// ============================================================================
// Middleware Tests
// ============================================================================

func TestAuthMiddleware_ValidToken(t *testing.T) {
	svc := newTestJWTService(24 * time.Hour)
	user := newTestUser()
	mw := NewAuthMiddleware(svc)

	token, err := svc.GenerateToken(user)
	if err != nil {
		t.Fatalf("GenerateToken() returned error: %v", err)
	}

	// Create a test handler that checks context for claims
	var gotClaims *KterodactylClaims
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotClaims = GetUserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	mw.Authenticate(nextHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("response status = %d, want %d", rec.Code, http.StatusOK)
	}

	if gotClaims == nil {
		t.Fatal("claims not found in context")
	}
	if gotClaims.Username != user.Username {
		t.Errorf("context claims.Username = %q, want %q", gotClaims.Username, user.Username)
	}
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	svc := newTestJWTService(24 * time.Hour)
	mw := NewAuthMiddleware(svc)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called when Authorization header is missing")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()

	mw.Authenticate(nextHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("response status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "missing authorization header") {
		t.Errorf("response body = %q, should contain 'missing authorization header'", body)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	svc := newTestJWTService(24 * time.Hour)
	mw := NewAuthMiddleware(svc)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called with invalid token")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer totally-invalid-token")
	rec := httptest.NewRecorder()

	mw.Authenticate(nextHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("response status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "invalid or expired token") {
		t.Errorf("response body = %q, should contain 'invalid or expired token'", body)
	}
}

func TestAuthMiddleware_RefreshHeader(t *testing.T) {
	// Create a service with 1-hour expiration (within 2-hour refresh threshold)
	svc := newTestJWTService(1 * time.Hour)
	user := newTestUser()
	mw := NewAuthMiddleware(svc)

	token, err := svc.GenerateToken(user)
	if err != nil {
		t.Fatalf("GenerateToken() returned error: %v", err)
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	mw.Authenticate(nextHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("response status = %d, want %d", rec.Code, http.StatusOK)
	}

	refreshToken := rec.Header().Get("X-Refresh-Token")
	if refreshToken == "" {
		t.Error("X-Refresh-Token header should be set for near-expiry token")
	}

	// Verify the refresh token is valid
	refreshClaims, err := svc.ValidateToken(refreshToken)
	if err != nil {
		t.Fatalf("refreshed token validation failed: %v", err)
	}
	if refreshClaims.Username != user.Username {
		t.Errorf("refreshed token username = %q, want %q", refreshClaims.Username, user.Username)
	}
}

func TestRequireAdmin_AdminUser(t *testing.T) {
	svc := newTestJWTService(24 * time.Hour)
	adminUser := &User{
		Username: "superadmin",
		Email:    "admin@example.com",
		Role:     RoleAdmin,
	}
	mw := NewAuthMiddleware(svc)

	token, err := svc.GenerateToken(adminUser)
	if err != nil {
		t.Fatalf("GenerateToken() returned error: %v", err)
	}

	var adminHandlerCalled bool
	adminHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		adminHandlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Chain: Authenticate -> RequireAdmin -> adminHandler
	handler := mw.Authenticate(RequireAdmin(adminHandler))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("response status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !adminHandlerCalled {
		t.Error("admin handler should have been called for admin user")
	}
}

func TestRequireAdmin_NonAdminUser(t *testing.T) {
	svc := newTestJWTService(24 * time.Hour)
	regularUser := newTestUser() // RoleUser
	mw := NewAuthMiddleware(svc)

	token, err := svc.GenerateToken(regularUser)
	if err != nil {
		t.Fatalf("GenerateToken() returned error: %v", err)
	}

	adminHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("admin handler should not be called for non-admin user")
	})

	// Chain: Authenticate -> RequireAdmin -> adminHandler
	handler := mw.Authenticate(RequireAdmin(adminHandler))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("response status = %d, want %d", rec.Code, http.StatusForbidden)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "admin access required") {
		t.Errorf("response body = %q, should contain 'admin access required'", body)
	}
}

// ============================================================================
// GetUserFromContext Tests
// ============================================================================

func TestGetUserFromContext_NoClaims(t *testing.T) {
	ctx := context.Background()
	claims := GetUserFromContext(ctx)
	if claims != nil {
		t.Error("GetUserFromContext() should return nil for context without claims")
	}
}

func TestGetUserFromContext_WithClaims(t *testing.T) {
	expectedClaims := &KterodactylClaims{
		Username:  "testuser",
		Email:     "test@example.com",
		Role:      RoleUser,
		Namespace: "user-testuser",
	}

	ctx := context.WithValue(context.Background(), ContextKeyUser, expectedClaims)
	claims := GetUserFromContext(ctx)
	if claims == nil {
		t.Fatal("GetUserFromContext() returned nil")
	}
	if claims.Username != expectedClaims.Username {
		t.Errorf("claims.Username = %q, want %q", claims.Username, expectedClaims.Username)
	}
}
