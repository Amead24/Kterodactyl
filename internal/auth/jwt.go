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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kterodactyl/kterodactyl/internal/util"
)

const (
	// jwtKeySecretName is the name of the Kubernetes Secret that stores the JWT signing key.
	jwtKeySecretName = "kterodactyl-jwt-signing-key"

	// defaultRefreshThreshold is the default duration before token expiry when
	// a token should be refreshed (2 hours).
	defaultRefreshThreshold = 2 * time.Hour

	// signingKeyLength is the length in bytes of the JWT signing key (256 bits).
	signingKeyLength = 32

	// tokenIDLength is the length in bytes of the random token ID (hex-encoded to 16 chars).
	tokenIDLength = 8
)

// KterodactylClaims extends jwt.RegisteredClaims with Kterodactyl-specific fields.
type KterodactylClaims struct {
	jwt.RegisteredClaims
	Username  string `json:"username"`
	Email     string `json:"email"`
	Role      string `json:"role"`      // "admin" or "user"
	Namespace string `json:"namespace"` // "user-<username>"
}

// JWTService handles JWT token generation, validation, and refresh detection.
type JWTService struct {
	signingKey       []byte
	tokenExpiration  time.Duration
	refreshThreshold time.Duration
}

// NewJWTService creates a new JWTService with the given signing key and token expiration.
func NewJWTService(signingKey []byte, expiration time.Duration) *JWTService {
	return &JWTService{
		signingKey:       signingKey,
		tokenExpiration:  expiration,
		refreshThreshold: defaultRefreshThreshold,
	}
}

// GenerateToken creates a signed JWT token for the given user.
func (s *JWTService) GenerateToken(user *User) (string, error) {
	tokenID, err := generateTokenID()
	if err != nil {
		return "", fmt.Errorf("failed to generate token ID: %w", err)
	}

	now := time.Now()
	claims := &KterodactylClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.Username,
			Issuer:    "kterodactyl",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.tokenExpiration)),
			ID:        tokenID,
		},
		Username:  user.Username,
		Email:     user.Email,
		Role:      user.Role,
		Namespace: util.UserNamespace(user.Username),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.signingKey)
}

// ValidateToken parses and validates a JWT token string, returning the claims on success.
// Validation enforces HS256 signing method, "kterodactyl" issuer, and expiration requirement.
func (s *JWTService) ValidateToken(tokenString string) (*KterodactylClaims, error) {
	claims := &KterodactylClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return s.signingKey, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer("kterodactyl"),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("token is not valid")
	}

	return claims, nil
}

// ShouldRefresh returns true if the token expires within the refresh threshold.
// Used by middleware to decide if a refreshed token should be issued.
func (s *JWTService) ShouldRefresh(claims *KterodactylClaims) bool {
	if claims.ExpiresAt == nil {
		return false
	}
	return time.Until(claims.ExpiresAt.Time) <= s.refreshThreshold
}

// EnsureSigningKey loads the JWT signing key from a Kubernetes Secret, or creates one if it doesn't exist.
// The key is stored in the specified namespace as a Secret named "kterodactyl-jwt-signing-key".
func EnsureSigningKey(ctx context.Context, c client.Client, namespace string) ([]byte, error) {
	secret := &corev1.Secret{}
	err := c.Get(ctx, types.NamespacedName{
		Name:      jwtKeySecretName,
		Namespace: namespace,
	}, secret)

	if err != nil {
		if errors.IsNotFound(err) {
			// Generate a new 256-bit signing key
			key := make([]byte, signingKeyLength)
			if _, err := rand.Read(key); err != nil {
				return nil, fmt.Errorf("failed to generate signing key: %w", err)
			}

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      jwtKeySecretName,
					Namespace: namespace,
					Labels: map[string]string{
						"kterodactyl.io/managed-by":    "kterodactyl",
						"kterodactyl.io/resource-type": "jwt-key",
					},
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"signing-key": key,
				},
			}

			if err := c.Create(ctx, secret); err != nil {
				return nil, fmt.Errorf("failed to create signing key secret: %w", err)
			}

			return key, nil
		}
		return nil, fmt.Errorf("failed to get signing key secret: %w", err)
	}

	key, ok := secret.Data["signing-key"]
	if !ok {
		return nil, fmt.Errorf("signing key secret exists but missing 'signing-key' data field")
	}

	return key, nil
}

// generateTokenID generates a cryptographically random token ID (8 bytes, hex-encoded to 16 chars).
func generateTokenID() (string, error) {
	b := make([]byte, tokenIDLength)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}
