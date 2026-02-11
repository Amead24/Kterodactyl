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
	"context"
	"net/http"

	"github.com/kterodactyl/kterodactyl/internal/auth"
	"github.com/kterodactyl/kterodactyl/internal/controller"
)

// namespaceFromContext extracts the user's namespace from JWT claims in the request context.
// Returns an empty string if no claims are present (middleware should prevent this).
func namespaceFromContext(r *http.Request) string {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil {
		return ""
	}
	return claims.Namespace
}

// usernameFromContext extracts the username from JWT claims in the request context.
// Returns an empty string if no claims are present (middleware should prevent this).
func usernameFromContext(r *http.Request) string {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil {
		return ""
	}
	return claims.Username
}

// loadAdminConfig loads the AdminConfig per-request from the operator namespace ConfigMap.
// This avoids staleness (Pitfall 3 from research) by reading fresh config each time.
func (s *Server) loadAdminConfig(ctx context.Context) (*controller.AdminConfig, error) {
	return controller.LoadAdminConfig(ctx, s.client, s.operatorNamespace)
}
