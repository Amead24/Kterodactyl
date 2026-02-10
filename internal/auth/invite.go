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

	mail "github.com/wneessen/go-mail"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kterodactyl/kterodactyl/internal/util"
)

const (
	// inviteTokenLength is the length in bytes of the random invite token (hex-encoded to 64 chars).
	inviteTokenLength = 32

	// inviteSecretPrefix is the prefix for invite Secret names.
	inviteSecretPrefix = "invite-"

	// ResourceTypeInvite is the value for LabelResourceType on invite Secrets.
	ResourceTypeInvite = "invite"

	// AnnotationExpiresAt is the annotation key for invite token expiration time.
	AnnotationExpiresAt = "kterodactyl.io/expires-at"
)

// SMTPConfig holds the SMTP server configuration for sending invitation emails.
type SMTPConfig struct {
	Host     string
	Port     int
	From     string
	Username string
	Password string
}

// InviteService manages invitation token lifecycle: creation, storage, email delivery, and redemption.
type InviteService struct {
	client    client.Client
	namespace string
	smtp      *SMTPConfig // nil if SMTP not configured
	panelURL  string
}

// NewInviteService creates a new InviteService. The smtp parameter can be nil if email is not configured,
// in which case invite links are returned directly instead of being emailed.
func NewInviteService(client client.Client, namespace string, smtp *SMTPConfig, panelURL string) *InviteService {
	return &InviteService{
		client:    client,
		namespace: namespace,
		smtp:      smtp,
		panelURL:  panelURL,
	}
}

// Invitation represents a created invitation token with metadata.
type Invitation struct {
	Token     string
	Email     string
	InvitedBy string
	ExpiresAt string
}

// CreateInvite generates a new invitation token, stores it as a Kubernetes Secret,
// and optionally sends an email with the registration link.
// Returns the Invitation (including the full token for link generation).
func (s *InviteService) CreateInvite(ctx context.Context, email, invitedBy string, expirationHours int) (*Invitation, error) {
	logger := log.FromContext(ctx)

	// Generate a cryptographically random 32-byte token, hex-encoded to 64 characters
	tokenBytes := make([]byte, inviteTokenLength)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate invite token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	expiresAt := time.Now().Add(time.Duration(expirationHours) * time.Hour)
	expiresAtStr := expiresAt.Format(time.RFC3339)

	// Create K8s Secret named invite-<first-12-chars-of-token>
	secretName := inviteSecretPrefix + token[:12]
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: s.namespace,
			Labels: map[string]string{
				util.LabelManagedByKterodactyl: util.ManagedByValue,
				LabelResourceType:              ResourceTypeInvite,
			},
			Annotations: map[string]string{
				AnnotationExpiresAt: expiresAtStr,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"token":      []byte(token),
			"email":      []byte(email),
			"invited-by": []byte(invitedBy),
		},
	}

	if err := s.client.Create(ctx, secret); err != nil {
		return nil, fmt.Errorf("failed to create invite secret: %w", err)
	}

	invitation := &Invitation{
		Token:     token,
		Email:     email,
		InvitedBy: invitedBy,
		ExpiresAt: expiresAtStr,
	}

	// Send email if SMTP is configured
	if s.smtp != nil {
		if err := s.sendInviteEmail(email, token); err != nil {
			logger.Error(err, "failed to send invite email, invite still created", "email", email)
		}
	} else {
		logger.Info("SMTP not configured, returning invite link in response", "email", email)
	}

	return invitation, nil
}

// RedeemInvite validates and redeems an invitation token.
// On success, the invite Secret is deleted (single-use) and the associated email is returned.
// Returns ErrInvalidToken if the token is not found, or ErrInviteExpired if it has expired.
func (s *InviteService) RedeemInvite(ctx context.Context, token string) (string, error) {
	// List all invite Secrets
	secrets := &corev1.SecretList{}
	if err := s.client.List(ctx, secrets,
		client.InNamespace(s.namespace),
		client.MatchingLabels{
			util.LabelManagedByKterodactyl: util.ManagedByValue,
			LabelResourceType:              ResourceTypeInvite,
		},
	); err != nil {
		return "", fmt.Errorf("failed to list invite secrets: %w", err)
	}

	// Find the Secret where Data["token"] matches the provided token
	var matchedSecret *corev1.Secret
	for i := range secrets.Items {
		if string(secrets.Items[i].Data["token"]) == token {
			matchedSecret = &secrets.Items[i]
			break
		}
	}

	if matchedSecret == nil {
		return "", ErrInvalidToken
	}

	// Check expiration
	expiresAtStr, ok := matchedSecret.Annotations[AnnotationExpiresAt]
	if ok {
		expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
		if err == nil && time.Now().After(expiresAt) {
			// Token has expired -- delete the Secret and return error
			_ = s.client.Delete(ctx, matchedSecret)
			return "", ErrInviteExpired
		}
	}

	// Extract the email before deleting (single-use: validate-then-delete)
	email := string(matchedSecret.Data["email"])

	// Delete the Secret immediately to enforce single-use
	if err := s.client.Delete(ctx, matchedSecret); err != nil {
		return "", fmt.Errorf("failed to delete redeemed invite secret: %w", err)
	}

	return email, nil
}

// sendInviteEmail sends an invitation email with a registration link using SMTP.
func (s *InviteService) sendInviteEmail(email, token string) error {
	registrationLink := fmt.Sprintf("%s/register?token=%s", s.panelURL, token)

	msg := mail.NewMsg()
	if err := msg.From(s.smtp.From); err != nil {
		return fmt.Errorf("failed to set from address: %w", err)
	}
	if err := msg.To(email); err != nil {
		return fmt.Errorf("failed to set to address: %w", err)
	}
	msg.Subject("You've been invited to Kterodactyl")

	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html>
<body>
<h2>You've been invited to Kterodactyl!</h2>
<p>Click the link below to create your account and start managing your game servers:</p>
<p><a href="%s">Register Now</a></p>
<p>If you didn't expect this invitation, you can safely ignore this email.</p>
</body>
</html>`, registrationLink)

	msg.SetBodyString(mail.TypeTextHTML, htmlBody)

	client, err := mail.NewClient(s.smtp.Host,
		mail.WithPort(s.smtp.Port),
		mail.WithTLSPolicy(mail.TLSOpportunistic),
	)
	if err != nil {
		return fmt.Errorf("failed to create mail client: %w", err)
	}

	if s.smtp.Username != "" {
		client.SetSMTPAuth(mail.SMTPAuthAutoDiscover)
		client.SetUsername(s.smtp.Username)
		client.SetPassword(s.smtp.Password)
	}

	if err := client.DialAndSend(msg); err != nil {
		return fmt.Errorf("failed to send invite email: %w", err)
	}

	return nil
}
