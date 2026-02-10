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

import "errors"

// Sentinel errors for authentication operations.
var (
	// ErrUserExists is returned when attempting to create a user that already exists.
	ErrUserExists = errors.New("user already exists")

	// ErrUserNotFound is returned when a requested user does not exist.
	ErrUserNotFound = errors.New("user not found")

	// ErrInvalidCredentials is returned when authentication fails due to wrong credentials.
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrInvalidToken is returned when a JWT token is malformed or has an invalid signature.
	ErrInvalidToken = errors.New("invalid token")

	// ErrTokenExpired is returned when a JWT token has expired.
	ErrTokenExpired = errors.New("token expired")

	// ErrInviteExpired is returned when an invitation token has expired.
	ErrInviteExpired = errors.New("invitation expired")

	// ErrInviteUsed is returned when an invitation token has already been redeemed.
	ErrInviteUsed = errors.New("invitation already used")

	// ErrInvalidUsername is returned when a username fails validation.
	ErrInvalidUsername = errors.New("invalid username")

	// ErrRegistrationDisabled is returned when self-registration is not enabled.
	ErrRegistrationDisabled = errors.New("registration is disabled")
)
