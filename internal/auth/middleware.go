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
	"strings"
)

// contextKey is an unexported type used for context value keys to prevent collisions.
type contextKey string

// ContextKeyUser is the context key used to store authenticated user claims.
const ContextKeyUser contextKey = "user"

// AuthMiddleware provides HTTP middleware for JWT-based authentication.
type AuthMiddleware struct {
	jwtService *JWTService
}

// NewAuthMiddleware creates a new AuthMiddleware with the given JWTService.
func NewAuthMiddleware(jwtService *JWTService) *AuthMiddleware {
	return &AuthMiddleware{
		jwtService: jwtService,
	}
}

// Authenticate returns an HTTP middleware that validates JWT tokens from the Authorization header.
// On success, the authenticated user's claims are stored in the request context.
// If the token is nearing expiry, a refreshed token is set in the X-Refresh-Token response header.
func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)
		if token == "" {
			http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
			return
		}

		claims, err := m.jwtService.ValidateToken(token)
		if err != nil {
			http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), ContextKeyUser, claims)

		// If the token is nearing expiry, issue a refreshed token
		if m.jwtService.ShouldRefresh(claims) {
			user := &User{
				Username: claims.Username,
				Email:    claims.Email,
				Role:     claims.Role,
			}
			if refreshedToken, err := m.jwtService.GenerateToken(user); err == nil {
				w.Header().Set("X-Refresh-Token", refreshedToken)
			}
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin returns an HTTP middleware that restricts access to admin users only.
// Must be used after Authenticate middleware to ensure claims are in the context.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := GetUserFromContext(r.Context())
		if claims == nil || claims.Role != RoleAdmin {
			http.Error(w, `{"error":"admin access required"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// extractBearerToken extracts the JWT token from the Authorization header.
// Returns an empty string if the header is missing or does not have the "Bearer " prefix.
func extractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(authHeader, "Bearer ")
}

// GetUserFromContext extracts the authenticated user's claims from the request context.
// Returns nil if no claims are present (e.g., unauthenticated request).
func GetUserFromContext(ctx context.Context) *KterodactylClaims {
	claims, ok := ctx.Value(ContextKeyUser).(*KterodactylClaims)
	if !ok {
		return nil
	}
	return claims
}
