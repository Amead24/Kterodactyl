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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kterodactyl/kterodactyl/internal/util"
)

// Auth-specific label keys for user Secrets.
const (
	// LabelResourceType identifies the type of Kterodactyl resource stored in a Secret.
	LabelResourceType = "kterodactyl.io/resource-type"

	// LabelUserName labels a Secret with the username it belongs to.
	LabelUserName = "kterodactyl.io/user"

	// LabelRole labels a Secret with the user's role.
	LabelRole = "kterodactyl.io/role"
)

// Resource type values.
const (
	// ResourceTypeUser is the value for LabelResourceType on user Secrets.
	ResourceTypeUser = "user"
)

// UserStore implements UserService using Kubernetes Secrets as the backing store.
// Each user is stored as a Secret named "user-<username>" in the operator namespace.
type UserStore struct {
	client    client.Client
	namespace string // operator namespace (e.g., kterodactyl-system)
}

// NewUserStore creates a new UserStore that stores user data in the given namespace.
func NewUserStore(client client.Client, namespace string) *UserStore {
	return &UserStore{
		client:    client,
		namespace: namespace,
	}
}

// Compile-time check that UserStore implements UserService.
var _ UserService = (*UserStore)(nil)

// CreateUser creates a new user by storing their credentials in a Kubernetes Secret.
// The username is validated against DNS label rules before creation.
// Returns ErrUserExists if a user with the same username already exists.
func (s *UserStore) CreateUser(ctx context.Context, user *User) error {
	if err := ValidateUsername(user.Username); err != nil {
		return err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("user-%s", user.Username),
			Namespace: s.namespace,
			Labels: map[string]string{
				util.LabelManagedByKterodactyl: util.ManagedByValue,
				LabelResourceType:              ResourceTypeUser,
				LabelUserName:                  user.Username,
				LabelRole:                      user.Role,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"email":         []byte(user.Email),
			"password-hash": []byte(user.PasswordHash),
			"created-at":    []byte(user.CreatedAt),
			"invited-by":    []byte(user.InvitedBy),
		},
	}

	if err := s.client.Create(ctx, secret); err != nil {
		if errors.IsAlreadyExists(err) {
			return ErrUserExists
		}
		return fmt.Errorf("failed to create user secret: %w", err)
	}

	return nil
}

// GetUser retrieves a user by username from the Kubernetes Secret store.
// Returns ErrUserNotFound if the user does not exist.
func (s *UserStore) GetUser(ctx context.Context, username string) (*User, error) {
	secret := &corev1.Secret{}
	err := s.client.Get(ctx, client.ObjectKey{
		Name:      fmt.Sprintf("user-%s", username),
		Namespace: s.namespace,
	}, secret)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user secret: %w", err)
	}

	return userFromSecret(secret), nil
}

// GetUserByEmail retrieves a user by email address by listing all user Secrets
// and searching for a matching email. Returns ErrUserNotFound if no match is found.
func (s *UserStore) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	secrets := &corev1.SecretList{}
	err := s.client.List(ctx, secrets,
		client.InNamespace(s.namespace),
		client.MatchingLabels{
			util.LabelManagedByKterodactyl: util.ManagedByValue,
			LabelResourceType:              ResourceTypeUser,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list user secrets: %w", err)
	}

	for i := range secrets.Items {
		if string(secrets.Items[i].Data["email"]) == email {
			return userFromSecret(&secrets.Items[i]), nil
		}
	}

	return nil, ErrUserNotFound
}

// ListUsers returns all users stored in the operator namespace.
func (s *UserStore) ListUsers(ctx context.Context) ([]*User, error) {
	secrets := &corev1.SecretList{}
	err := s.client.List(ctx, secrets,
		client.InNamespace(s.namespace),
		client.MatchingLabels{
			util.LabelManagedByKterodactyl: util.ManagedByValue,
			LabelResourceType:              ResourceTypeUser,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list user secrets: %w", err)
	}

	users := make([]*User, 0, len(secrets.Items))
	for i := range secrets.Items {
		users = append(users, userFromSecret(&secrets.Items[i]))
	}

	return users, nil
}

// DeleteUser removes a user by deleting their Kubernetes Secret.
// Returns ErrUserNotFound if the user does not exist.
func (s *UserStore) DeleteUser(ctx context.Context, username string) error {
	secret := &corev1.Secret{}
	err := s.client.Get(ctx, client.ObjectKey{
		Name:      fmt.Sprintf("user-%s", username),
		Namespace: s.namespace,
	}, secret)
	if err != nil {
		if errors.IsNotFound(err) {
			return ErrUserNotFound
		}
		return fmt.Errorf("failed to get user secret for deletion: %w", err)
	}

	if err := s.client.Delete(ctx, secret); err != nil {
		return fmt.Errorf("failed to delete user secret: %w", err)
	}

	return nil
}

// userFromSecret extracts a User from a Kubernetes Secret's Data map and Labels.
func userFromSecret(secret *corev1.Secret) *User {
	return &User{
		Username:     secret.Labels[LabelUserName],
		Email:        string(secret.Data["email"]),
		PasswordHash: string(secret.Data["password-hash"]),
		Role:         secret.Labels[LabelRole],
		CreatedAt:    string(secret.Data["created-at"]),
		InvitedBy:    string(secret.Data["invited-by"]),
	}
}
