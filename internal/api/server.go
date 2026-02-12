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
	"time"

	"github.com/go-chi/chi/v5"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kterodactyl/kterodactyl/internal/auth"
	"github.com/kterodactyl/kterodactyl/internal/manifest"
)

// Config holds all dependencies needed to construct an API Server.
type Config struct {
	// Client is the controller-runtime Kubernetes client for CRD CRUD operations.
	Client client.Client

	// Clientset is the kubernetes.Clientset for pod logs/exec operations.
	Clientset *kubernetes.Clientset

	// RestConfig is the Kubernetes REST config for SPDY exec connections.
	RestConfig *rest.Config

	// MetricsClient is the typed client for the Kubernetes Metrics API.
	MetricsClient *metricsv.Clientset

	// JWTService handles token generation and validation.
	JWTService *auth.JWTService

	// UserStore provides user CRUD operations.
	UserStore auth.UserService

	// InviteService manages invitation token lifecycle.
	InviteService *auth.InviteService

	// ManifestLoader provides access to loaded game manifests.
	ManifestLoader *manifest.Loader

	// OperatorNamespace is the namespace where the operator is deployed (e.g., "kterodactyl-system").
	OperatorNamespace string

	// BindAddress is the address the HTTP server listens on (e.g., ":8080").
	BindAddress string
}

// Server is the REST API server that bridges users to the Kubernetes API.
// It uses a chi router with middleware stacks and scoped route groups.
type Server struct {
	client            client.Client
	clientset         *kubernetes.Clientset
	restConfig        *rest.Config
	metricsClient     *metricsv.Clientset
	jwtService        *auth.JWTService
	userStore         auth.UserService
	inviteService     *auth.InviteService
	authMiddleware    *auth.AuthMiddleware
	manifestLoader    *manifest.Loader
	operatorNamespace string
	router            chi.Router
	bindAddress       string
}

// NewServer creates a new API Server with the given configuration.
// It wires all dependencies, creates the auth middleware, and builds the router.
func NewServer(cfg Config) *Server {
	s := &Server{
		client:            cfg.Client,
		clientset:         cfg.Clientset,
		restConfig:        cfg.RestConfig,
		metricsClient:     cfg.MetricsClient,
		jwtService:        cfg.JWTService,
		userStore:         cfg.UserStore,
		inviteService:     cfg.InviteService,
		authMiddleware:    auth.NewAuthMiddleware(cfg.JWTService),
		manifestLoader:    cfg.ManifestLoader,
		operatorNamespace: cfg.OperatorNamespace,
		bindAddress:       cfg.BindAddress,
	}
	s.router = s.routes()
	return s
}

// HTTPServer returns a configured *http.Server ready for use with the controller-runtime manager.
func (s *Server) HTTPServer() *http.Server {
	return &http.Server{
		Addr:              s.bindAddress,
		Handler:           s.router,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}
