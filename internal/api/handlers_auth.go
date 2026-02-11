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
	"errors"
	"net/http"
	"time"

	"github.com/kterodactyl/kterodactyl/internal/auth"
)

// handleLogin authenticates a user with username/password and returns a JWT token.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := req.Validate(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx := r.Context()

	// Look up user by username
	user, err := s.userStore.GetUser(ctx, req.Username)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			// Don't reveal whether username or password is wrong
			respondError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to look up user")
		return
	}

	// Verify password
	match, err := auth.VerifyPassword(req.Password, user.PasswordHash)
	if err != nil || !match {
		respondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Generate JWT token
	token, err := s.jwtService.GenerateToken(user)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"token": token,
	})
}

// handleRegister creates a new user account using an invitation token and returns a JWT token.
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := req.Validate(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx := r.Context()

	// Check if registration is enabled via AdminConfig
	adminCfg, err := s.loadAdminConfig(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load configuration")
		return
	}
	if !adminCfg.RegistrationEnabled {
		respondError(w, http.StatusForbidden, "registration is disabled")
		return
	}

	// Redeem the invite token (single-use: validates and deletes)
	inviteEmail, err := s.inviteService.RedeemInvite(ctx, req.InviteToken)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidToken) {
			respondError(w, http.StatusBadRequest, "invalid invite token")
			return
		}
		if errors.Is(err, auth.ErrInviteExpired) {
			respondError(w, http.StatusBadRequest, "invite token has expired")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to validate invite token")
		return
	}

	// Validate username (DNS label, not reserved)
	if err := auth.ValidateUsername(req.Username); err != nil {
		if errors.Is(err, auth.ErrInvalidUsername) {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondError(w, http.StatusBadRequest, "invalid username")
		return
	}

	// Hash password
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to process password")
		return
	}

	// Create the user (email comes from the invite, not the request)
	user := &auth.User{
		Username:     req.Username,
		Email:        inviteEmail,
		PasswordHash: passwordHash,
		Role:         auth.RoleUser,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	if err := s.userStore.CreateUser(ctx, user); err != nil {
		if errors.Is(err, auth.ErrUserExists) {
			respondError(w, http.StatusConflict, "username already taken")
			return
		}
		if errors.Is(err, auth.ErrInvalidUsername) {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	// Generate JWT token for the new user
	token, err := s.jwtService.GenerateToken(user)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{
		"token":    token,
		"username": user.Username,
	})
}

// handleRefresh issues a new JWT token for the authenticated user.
// The auth middleware has already validated the existing token.
func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "no valid claims in context")
		return
	}

	// Reconstruct user from claims to generate a fresh token
	user := &auth.User{
		Username: claims.Username,
		Email:    claims.Email,
		Role:     claims.Role,
	}

	token, err := s.jwtService.GenerateToken(user)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"token": token,
	})
}
