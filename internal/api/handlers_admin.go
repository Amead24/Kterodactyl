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

	"github.com/go-chi/chi/v5"

	"github.com/kterodactyl/kterodactyl/internal/auth"
)

// UserResponse is the API response format for a user (deliberately excludes PasswordHash).
type UserResponse struct {
	Username  string `json:"username"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	CreatedAt string `json:"createdAt"`
	InvitedBy string `json:"invitedBy"`
}

// userToResponse maps an auth.User to its safe API response representation.
// Explicitly excludes PasswordHash to prevent credential exposure.
func userToResponse(u *auth.User) *UserResponse {
	return &UserResponse{
		Username:  u.Username,
		Email:     u.Email,
		Role:      u.Role,
		CreatedAt: u.CreatedAt,
		InvitedBy: u.InvitedBy,
	}
}

// handleCreateInvite creates a new invitation token for the given email address.
// The AdminConfig is loaded per-request to read the current invite expiration hours.
func (s *Server) handleCreateInvite(w http.ResponseWriter, r *http.Request) {
	var req CreateInviteRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := req.Validate(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx := r.Context()

	// Load AdminConfig per-request for invite expiration hours
	cfg, err := s.loadAdminConfig(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load configuration")
		return
	}

	// Get inviting admin's username from JWT claims
	username := usernameFromContext(r)

	// Create the invite
	invite, err := s.inviteService.CreateInvite(ctx, req.Email, username, cfg.InviteExpirationHours)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create invite")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{
		"token":     invite.Token,
		"email":     invite.Email,
		"expiresAt": invite.ExpiresAt,
	})
}

// handleListUsers returns all registered users (excluding password hashes).
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.userStore.ListUsers(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list users")
		return
	}

	responses := make([]*UserResponse, len(users))
	for i, u := range users {
		responses[i] = userToResponse(u)
	}

	respondList(w, http.StatusOK, responses, len(responses))
}

// handleDeleteUser deletes a user by username. Prevents self-deletion.
func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	if username == "" {
		respondError(w, http.StatusBadRequest, "username is required")
		return
	}

	// Prevent self-deletion
	adminUsername := usernameFromContext(r)
	if adminUsername == username {
		respondError(w, http.StatusBadRequest, "cannot delete yourself")
		return
	}

	err := s.userStore.DeleteUser(r.Context(), username)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			respondError(w, http.StatusNotFound, "user not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
