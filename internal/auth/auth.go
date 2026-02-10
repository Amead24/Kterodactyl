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
	"fmt"
	"regexp"
)

// Role constants for user authorization.
const (
	// RoleAdmin grants full access to all operations including user management.
	RoleAdmin = "admin"

	// RoleUser grants access to own resources only.
	RoleUser = "user"
)

// User represents an authenticated user in the system.
type User struct {
	// Username is the unique identifier for the user (DNS label safe).
	Username string

	// Email is the user's email address.
	Email string

	// PasswordHash is the Argon2id hash of the user's password in PHC string format.
	PasswordHash string

	// Role is the user's authorization role ("admin" or "user").
	Role string

	// CreatedAt is the RFC3339 timestamp of when the user was created.
	CreatedAt string

	// InvitedBy is the username of the admin who invited this user (empty if self-registered).
	InvitedBy string
}

// UserService defines the interface for user CRUD operations.
type UserService interface {
	// CreateUser creates a new user. Returns ErrUserExists if the username is taken.
	CreateUser(ctx context.Context, user *User) error

	// GetUser retrieves a user by username. Returns ErrUserNotFound if not found.
	GetUser(ctx context.Context, username string) (*User, error)

	// GetUserByEmail retrieves a user by email address. Returns ErrUserNotFound if not found.
	GetUserByEmail(ctx context.Context, email string) (*User, error)

	// ListUsers returns all users in the system.
	ListUsers(ctx context.Context) ([]*User, error)

	// DeleteUser removes a user by username. Returns ErrUserNotFound if not found.
	DeleteUser(ctx context.Context, username string) error
}

// dnsLabelRegex matches valid DNS labels: lowercase alphanumeric, may contain hyphens,
// must start and end with alphanumeric character.
var dnsLabelRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// reservedUsernames are names that cannot be used as usernames to prevent
// conflicts with system namespaces and resources.
var reservedUsernames = map[string]bool{
	"admin":    true,
	"system":   true,
	"operator": true,
	"default":  true,
	"kube":     true,
}

// ValidateUsername checks that a username is a valid DNS label and not a reserved name.
// Returns ErrInvalidUsername (wrapped with a descriptive message) on failure.
func ValidateUsername(username string) error {
	if len(username) == 0 {
		return fmt.Errorf("%w: username must not be empty", ErrInvalidUsername)
	}

	if len(username) > 63 {
		return fmt.Errorf("%w: username must be 63 characters or fewer", ErrInvalidUsername)
	}

	if !dnsLabelRegex.MatchString(username) {
		return fmt.Errorf("%w: username must be a valid DNS label (lowercase alphanumeric and hyphens, must start and end with alphanumeric)", ErrInvalidUsername)
	}

	if reservedUsernames[username] {
		return fmt.Errorf("%w: username %q is reserved", ErrInvalidUsername, username)
	}

	return nil
}
