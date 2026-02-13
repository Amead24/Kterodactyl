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
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gamev1alpha1 "github.com/kterodactyl/kterodactyl/api/v1alpha1"
	"github.com/kterodactyl/kterodactyl/internal/util"
)

// GameServerResponse is the API response format for a GameServer.
// Raw K8s objects are never exposed directly to API consumers.
type GameServerResponse struct {
	Name       string            `json:"name"`
	GameType   string            `json:"gameType"`
	State      string            `json:"state"`
	Address    string            `json:"address,omitempty"`
	Ports      []PortResponse    `json:"ports,omitempty"`
	Parameters map[string]string `json:"parameters,omitempty"`
	CreatedAt  string            `json:"createdAt"`
}

// PortResponse is the API response format for a game server port.
type PortResponse struct {
	Name     string `json:"name"`
	Port     int32  `json:"port"`
	Protocol string `json:"protocol"`
}

// gameServerToResponse maps a GameServer CRD to its API response representation.
func gameServerToResponse(gs *gamev1alpha1.GameServer) *GameServerResponse {
	ports := make([]PortResponse, len(gs.Status.Ports))
	for i, p := range gs.Status.Ports {
		ports[i] = PortResponse{
			Name:     p.Name,
			Port:     p.Port,
			Protocol: string(p.Protocol),
		}
	}

	params := gs.Spec.Parameters
	if params == nil {
		params = map[string]string{}
	}

	return &GameServerResponse{
		Name:       gs.Name,
		GameType:   gs.Spec.GameType,
		State:      string(gs.Status.State),
		Address:    gs.Status.Address,
		Ports:      ports,
		Parameters: params,
		CreatedAt:  gs.CreationTimestamp.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// handleListGameServers returns all GameServers in the user's namespace.
func (s *Server) handleListGameServers(w http.ResponseWriter, r *http.Request) {
	ns := namespaceFromContext(r)
	if ns == "" {
		respondError(w, http.StatusUnauthorized, "no namespace in context")
		return
	}

	ctx := r.Context()
	list := &gamev1alpha1.GameServerList{}
	if err := s.client.List(ctx, list, client.InNamespace(ns)); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list game servers")
		return
	}

	responses := make([]*GameServerResponse, len(list.Items))
	for i := range list.Items {
		responses[i] = gameServerToResponse(&list.Items[i])
	}

	respondList(w, http.StatusOK, responses, len(responses))
}

// handleCreateGameServer creates a new GameServer in the user's namespace.
func (s *Server) handleCreateGameServer(w http.ResponseWriter, r *http.Request) {
	ns := namespaceFromContext(r)
	username := usernameFromContext(r)
	if ns == "" || username == "" {
		respondError(w, http.StatusUnauthorized, "no namespace or username in context")
		return
	}

	var req CreateGameServerRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := req.Validate(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Look up game manifest for image, ports, resources, default parameters
	m, ok := s.manifestLoader.Get(req.GameType)
	if !ok {
		respondError(w, http.StatusBadRequest, "unknown game type: "+req.GameType)
		return
	}

	// Merge manifest default parameters with user-provided overrides
	parameters := mergeMaps(m.Parameters, req.Parameters)

	// Validate merged parameters against the game's JSON Schema
	if err := m.ValidateParameters(parameters); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	gs := &gamev1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: ns,
			Labels:    util.GameServerLabels(username, req.GameType),
		},
		Spec: gamev1alpha1.GameServerSpec{
			GameType:   req.GameType,
			Image:      m.Image,
			Ports:      m.Ports,
			Resources:  m.Resources,
			Parameters: parameters,
		},
	}

	// Set mod path annotation if game supports mods
	if m.ModPath != "" {
		if gs.Annotations == nil {
			gs.Annotations = make(map[string]string)
		}
		gs.Annotations[util.AnnotationModPath] = m.ModPath
	}

	// Set backup path annotation from manifest (same pattern as modPath)
	if m.BackupPath != "" {
		if gs.Annotations == nil {
			gs.Annotations = make(map[string]string)
		}
		gs.Annotations[util.AnnotationBackupPath] = m.BackupPath
	}

	ctx := r.Context()
	if err := s.client.Create(ctx, gs); err != nil {
		if k8serrors.IsAlreadyExists(err) {
			respondError(w, http.StatusConflict, "game server with this name already exists")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to create game server")
		return
	}

	// Return the spec immediately; status fields populate asynchronously via the operator
	respondJSON(w, http.StatusCreated, gameServerToResponse(gs))
}

// handleGetGameServer returns a specific GameServer by name in the user's namespace.
func (s *Server) handleGetGameServer(w http.ResponseWriter, r *http.Request) {
	ns := namespaceFromContext(r)
	if ns == "" {
		respondError(w, http.StatusUnauthorized, "no namespace in context")
		return
	}

	name := chi.URLParam(r, "name")
	ctx := r.Context()

	gs := &gamev1alpha1.GameServer{}
	if err := s.client.Get(ctx, client.ObjectKey{Name: name, Namespace: ns}, gs); err != nil {
		if k8serrors.IsNotFound(err) {
			respondError(w, http.StatusNotFound, "game server not found: "+name)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get game server")
		return
	}

	respondJSON(w, http.StatusOK, gameServerToResponse(gs))
}

// handleUpdateGameServer updates the parameters of a GameServer in the user's namespace.
// Only Parameters can be updated; GameType, Image, etc. are immutable after creation.
func (s *Server) handleUpdateGameServer(w http.ResponseWriter, r *http.Request) {
	ns := namespaceFromContext(r)
	if ns == "" {
		respondError(w, http.StatusUnauthorized, "no namespace in context")
		return
	}

	name := chi.URLParam(r, "name")
	ctx := r.Context()

	var req UpdateGameServerRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := req.Validate(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Fetch the existing GameServer
	gs := &gamev1alpha1.GameServer{}
	if err := s.client.Get(ctx, client.ObjectKey{Name: name, Namespace: ns}, gs); err != nil {
		if k8serrors.IsNotFound(err) {
			respondError(w, http.StatusNotFound, "game server not found: "+name)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get game server")
		return
	}

	// Merge existing parameters with update request
	gs.Spec.Parameters = mergeMaps(gs.Spec.Parameters, req.Parameters)

	// Validate merged parameters against the game's JSON Schema.
	// If the manifest is not found (game definition removed after server creation),
	// skip validation rather than blocking updates.
	if m, ok := s.manifestLoader.Get(gs.Spec.GameType); ok {
		if err := m.ValidateParameters(gs.Spec.Parameters); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	if err := s.client.Update(ctx, gs); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update game server")
		return
	}

	respondJSON(w, http.StatusOK, gameServerToResponse(gs))
}

// handleDeleteGameServer deletes a GameServer from the user's namespace.
func (s *Server) handleDeleteGameServer(w http.ResponseWriter, r *http.Request) {
	ns := namespaceFromContext(r)
	if ns == "" {
		respondError(w, http.StatusUnauthorized, "no namespace in context")
		return
	}

	name := chi.URLParam(r, "name")
	ctx := r.Context()

	gs := &gamev1alpha1.GameServer{}
	if err := s.client.Get(ctx, client.ObjectKey{Name: name, Namespace: ns}, gs); err != nil {
		if k8serrors.IsNotFound(err) {
			respondError(w, http.StatusNotFound, "game server not found: "+name)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get game server")
		return
	}

	if err := s.client.Delete(ctx, gs); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete game server")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleStartGameServer sets a Shutdown or Error GameServer's state back to Creating.
// POST /api/v1/gameservers/{name}/start
func (s *Server) handleStartGameServer(w http.ResponseWriter, r *http.Request) {
	ns := namespaceFromContext(r)
	if ns == "" {
		respondError(w, http.StatusUnauthorized, "no namespace in context")
		return
	}

	name := chi.URLParam(r, "name")
	ctx := r.Context()

	gs := &gamev1alpha1.GameServer{}
	if err := s.client.Get(ctx, client.ObjectKey{Name: name, Namespace: ns}, gs); err != nil {
		if k8serrors.IsNotFound(err) {
			respondError(w, http.StatusNotFound, "game server not found: "+name)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get game server")
		return
	}

	switch gs.Status.State {
	case gamev1alpha1.GameServerStateCreating,
		gamev1alpha1.GameServerStateStarting,
		gamev1alpha1.GameServerStateReady,
		gamev1alpha1.GameServerStateAllocated:
		respondError(w, http.StatusConflict, "server is already running")
		return
	case gamev1alpha1.GameServerStateShutdown, gamev1alpha1.GameServerStateError:
		gs.Status.State = gamev1alpha1.GameServerStateCreating
	default:
		gs.Status.State = gamev1alpha1.GameServerStateCreating
	}

	if err := s.client.Status().Update(ctx, gs); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update game server state")
		return
	}

	respondJSON(w, http.StatusOK, gameServerToResponse(gs))
}

// handleStopGameServer sets a GameServer's state to Shutdown.
// POST /api/v1/gameservers/{name}/stop
func (s *Server) handleStopGameServer(w http.ResponseWriter, r *http.Request) {
	ns := namespaceFromContext(r)
	if ns == "" {
		respondError(w, http.StatusUnauthorized, "no namespace in context")
		return
	}

	name := chi.URLParam(r, "name")
	ctx := r.Context()

	gs := &gamev1alpha1.GameServer{}
	if err := s.client.Get(ctx, client.ObjectKey{Name: name, Namespace: ns}, gs); err != nil {
		if k8serrors.IsNotFound(err) {
			respondError(w, http.StatusNotFound, "game server not found: "+name)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get game server")
		return
	}

	switch gs.Status.State {
	case gamev1alpha1.GameServerStateShutdown:
		respondError(w, http.StatusConflict, "server is already stopped")
		return
	case gamev1alpha1.GameServerStateError:
		respondError(w, http.StatusConflict, "server is in error state")
		return
	}

	gs.Status.State = gamev1alpha1.GameServerStateShutdown

	if err := s.client.Status().Update(ctx, gs); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update game server state")
		return
	}

	respondJSON(w, http.StatusOK, gameServerToResponse(gs))
}

// handleRestartGameServer restarts a GameServer by setting its state back to Creating.
// For Shutdown/Error servers, transitions directly to Creating.
// For Ready/Allocated servers, transitions to Creating (operator will recreate the Pod).
// POST /api/v1/gameservers/{name}/restart
func (s *Server) handleRestartGameServer(w http.ResponseWriter, r *http.Request) {
	ns := namespaceFromContext(r)
	if ns == "" {
		respondError(w, http.StatusUnauthorized, "no namespace in context")
		return
	}

	name := chi.URLParam(r, "name")
	ctx := r.Context()

	gs := &gamev1alpha1.GameServer{}
	if err := s.client.Get(ctx, client.ObjectKey{Name: name, Namespace: ns}, gs); err != nil {
		if k8serrors.IsNotFound(err) {
			respondError(w, http.StatusNotFound, "game server not found: "+name)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get game server")
		return
	}

	// All states can restart: set back to Creating so the operator reconciler creates a new Pod
	gs.Status.State = gamev1alpha1.GameServerStateCreating

	if err := s.client.Status().Update(ctx, gs); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update game server state")
		return
	}

	respondJSON(w, http.StatusOK, gameServerToResponse(gs))
}

// mergeMaps creates a new map by copying base and overlaying override values.
// Returns an empty map (not nil) when both inputs are nil.
func mergeMaps(base, override map[string]string) map[string]string {
	result := make(map[string]string, len(base)+len(override))
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		result[k] = v
	}
	return result
}
