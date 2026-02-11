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
	"fmt"
	"net/http"
)

// decodeJSON reads the request body and decodes it into the given value.
// Extra fields in the JSON body are accepted gracefully (no DisallowUnknownFields).
func decodeJSON(r *http.Request, v interface{}) error {
	if r.Body == nil {
		return fmt.Errorf("request body is empty")
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

// CreateGameServerRequest is the request body for creating a new GameServer.
type CreateGameServerRequest struct {
	Name       string            `json:"name"`
	GameType   string            `json:"gameType"`
	Parameters map[string]string `json:"parameters,omitempty"`
}

// Validate checks that required fields are present and returns a descriptive error if not.
func (r *CreateGameServerRequest) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("name is required")
	}
	if r.GameType == "" {
		return fmt.Errorf("gameType is required")
	}
	return nil
}

// LoginRequest is the request body for user login.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Validate checks that required fields are present.
func (r *LoginRequest) Validate() error {
	if r.Username == "" {
		return fmt.Errorf("username is required")
	}
	if r.Password == "" {
		return fmt.Errorf("password is required")
	}
	return nil
}

// RegisterRequest is the request body for user registration.
type RegisterRequest struct {
	Username    string `json:"username"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	InviteToken string `json:"inviteToken"`
}

// Validate checks that required fields are present.
func (r *RegisterRequest) Validate() error {
	if r.Username == "" {
		return fmt.Errorf("username is required")
	}
	if r.Email == "" {
		return fmt.Errorf("email is required")
	}
	if r.Password == "" {
		return fmt.Errorf("password is required")
	}
	if r.InviteToken == "" {
		return fmt.Errorf("inviteToken is required")
	}
	return nil
}

// CreateInviteRequest is the request body for creating an invitation.
type CreateInviteRequest struct {
	Email string `json:"email"`
}

// Validate checks that required fields are present.
func (r *CreateInviteRequest) Validate() error {
	if r.Email == "" {
		return fmt.Errorf("email is required")
	}
	return nil
}

// UpdateGameServerRequest is the request body for updating a GameServer.
type UpdateGameServerRequest struct {
	Parameters map[string]string `json:"parameters,omitempty"`
}

// Validate checks that the request contains valid update data.
func (r *UpdateGameServerRequest) Validate() error {
	// Parameters are optional but the request should have something to update
	if len(r.Parameters) == 0 {
		return fmt.Errorf("at least one field must be provided for update")
	}
	return nil
}
