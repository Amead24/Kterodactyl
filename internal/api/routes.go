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
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"

	"github.com/kterodactyl/kterodactyl/internal/auth"
)

// routes builds and returns the chi router with all middleware stacks and route groups.
//
// CRITICAL: The 30-second timeout middleware is applied ONLY to REST API routes, not globally.
// WebSocket routes (console) are long-lived connections that must not be killed by the timeout.
func (s *Server) routes() chi.Router {
	r := chi.NewRouter()

	// Global middleware (order matters: outermost first)
	// NOTE: middleware.Timeout is NOT applied globally -- it's on REST routes only.
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// CORS at top level (Pitfall 4: must be top-level for preflight OPTIONS to work)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		ExposedHeaders:   []string{"X-Refresh-Token"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Global rate limit: 100 requests per minute per IP
	r.Use(httprate.LimitByIP(100, time.Minute))

	// Health routes (unauthenticated, no timeout needed)
	r.Get("/healthz", handleHealthz)
	r.Get("/readyz", handleReadyz)

	// Public auth routes with tighter per-endpoint rate limits
	r.With(httprate.LimitByIP(5, time.Minute)).Post("/api/v1/auth/login", s.handleLogin)
	r.With(httprate.LimitByIP(3, time.Minute)).Post("/api/v1/auth/register", s.handleRegister)

	// WebSocket routes (NO timeout -- long-lived connections)
	// Auth is handled inside the handler via JWT query parameter because WebSocket
	// upgrade requests cannot carry Authorization headers.
	r.Get("/api/v1/gameservers/{name}/console", s.handleConsole)

	// REST API routes WITH timeout and authentication
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.Timeout(30 * time.Second))
		r.Use(s.authMiddleware.Authenticate)

		// Auth management
		r.Post("/auth/refresh", s.handleRefresh)

		// Game manifest endpoints
		r.Get("/games", s.handleListGames)
		r.Get("/games/{gameType}", s.handleGetGame)

		// GameServer CRUD with create rate limit
		r.Route("/gameservers", func(r chi.Router) {
			r.Get("/", s.handleListGameServers)
			r.With(httprate.LimitByIP(10, time.Minute)).Post("/", s.handleCreateGameServer)
			r.Route("/{name}", func(r chi.Router) {
				r.Get("/", s.handleGetGameServer)
				r.Put("/", s.handleUpdateGameServer)
				r.Delete("/", s.handleDeleteGameServer)
				r.Post("/start", s.handleStartGameServer)
				r.Post("/stop", s.handleStopGameServer)
				r.Post("/restart", s.handleRestartGameServer)
				r.Get("/metrics", s.handleGetMetrics)
			})
		})

		// Admin routes (require admin role)
		r.Route("/admin", func(r chi.Router) {
			r.Use(auth.RequireAdmin)
			r.Post("/invites", s.handleCreateInvite)
			r.Get("/users", s.handleListUsers)
			r.Delete("/users/{username}", s.handleDeleteUser)
		})
	})

	// SPA catch-all: serve embedded frontend for any route not matched by API handlers.
	// Must be registered AFTER all API routes so they take priority.
	r.NotFound(serveSPA().ServeHTTP)

	return r
}
