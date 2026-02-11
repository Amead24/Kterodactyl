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
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/kterodactyl/kterodactyl/internal/manifest"
)

// GameResponse is the API response format for a game manifest.
type GameResponse struct {
	Name            string                 `json:"name"`
	DisplayName     string                 `json:"displayName"`
	Image           string                 `json:"image"`
	Ports           []PortInfo             `json:"ports"`
	Parameters      map[string]string      `json:"parameters"`
	ParameterSchema map[string]interface{} `json:"parameterSchema,omitempty"`
}

// PortInfo is the API response format for a game server port.
type PortInfo struct {
	Name          string `json:"name"`
	ContainerPort int32  `json:"containerPort"`
	Protocol      string `json:"protocol"`
}

// gameManifestToResponse maps a GameManifest to its API response representation.
// Converts corev1.Protocol to a plain string for JSON cleanliness.
func gameManifestToResponse(m *manifest.GameManifest) *GameResponse {
	ports := make([]PortInfo, len(m.Ports))
	for i, p := range m.Ports {
		ports[i] = PortInfo{
			Name:          p.Name,
			ContainerPort: p.ContainerPort,
			Protocol:      string(p.Protocol),
		}
	}

	params := m.Parameters
	if params == nil {
		params = map[string]string{}
	}

	return &GameResponse{
		Name:            m.Name,
		DisplayName:     m.DisplayName,
		Image:           m.Image,
		Ports:           ports,
		Parameters:      params,
		ParameterSchema: m.ParameterSchema,
	}
}

// handleListGames returns all loaded game manifests.
func (s *Server) handleListGames(w http.ResponseWriter, _ *http.Request) {
	games := s.manifestLoader.List()
	responses := make([]*GameResponse, len(games))
	for i, g := range games {
		responses[i] = gameManifestToResponse(g)
	}
	respondList(w, http.StatusOK, responses, len(responses))
}

// handleGetGame returns a specific game manifest by gameType.
func (s *Server) handleGetGame(w http.ResponseWriter, r *http.Request) {
	gameType := chi.URLParam(r, "gameType")
	m, ok := s.manifestLoader.Get(gameType)
	if !ok {
		respondError(w, http.StatusNotFound, "game type not found: "+gameType)
		return
	}
	respondJSON(w, http.StatusOK, gameManifestToResponse(m))
}
