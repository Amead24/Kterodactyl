# Phase 6: Frontend UI - Research

**Researched:** 2026-02-11
**Domain:** React SPA frontend for Kubernetes game server management panel
**Confidence:** HIGH

## Summary

The frontend for Kterodactyl is a single-page application (SPA) behind authentication -- an admin panel / dashboard pattern where SEO is irrelevant and all interactions are authenticated API calls. The existing Go API server at `/api/v1/` provides all necessary endpoints (auth, game servers, games, admin) with JSON responses, CORS configured for `*`, and JWT Bearer token authentication.

The key architectural decision is **Vite + React SPA over Next.js**. Next.js's primary advantages (SSR, SSG, SEO, server components) provide zero value for an authenticated admin panel. Vite produces a clean static asset bundle (HTML/CSS/JS) that can be embedded directly into the Go binary using `//go:embed`, maintaining the single-binary deployment that already exists. This avoids the complexity of Next.js static export mode, its SPA limitations with dynamic routes, and the overhead of a Node.js build chain in what is fundamentally a Go project. The frontend static assets get served by the same Go HTTP server that serves the API, eliminating CORS entirely in production.

For dynamic form generation from JSON Schema (the GAME-04 requirement), `@rjsf/core` v6 with `@rjsf/shadcn` provides an official shadcn/ui theme for react-jsonschema-form. The existing `parameterSchema` in game manifests uses only JSON Schema draft-07 compatible features (enum, const, pattern, maxLength, default) despite the project documenting "Draft 2020-12" -- this means RJSF's default draft-07 validator works without configuration.

**Primary recommendation:** Use Vite + React + TypeScript with shadcn/ui components, TanStack Query for server state, Zustand for client state, and @rjsf/shadcn for dynamic form generation. Embed the built static assets into the Go binary with `//go:embed` and serve from the same HTTP server as the API.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| React | ^19 | UI framework | Industry standard, required by all deps |
| Vite | ^6 | Build tool and dev server | 60ms HMR, clean static output, replaced CRA as standard |
| TypeScript | ^5.7 | Type safety | Required for shadcn/ui, improves DX |
| react-router | ^7 | Client-side routing | Standard for React SPAs, nested routes, loaders |
| shadcn/ui | latest | UI component library | Copy-paste components built on Radix UI + Tailwind, 66k GitHub stars |
| Tailwind CSS | ^4 | Utility-first CSS | Required by shadcn/ui, zero-runtime |
| TanStack Query | ^5 | Server state (data fetching, caching) | De facto standard for async server state in React |
| @rjsf/core | ^6.2 | Dynamic form generation from JSON Schema | Only mature React library for JSON Schema forms |
| @rjsf/shadcn | ^6.2 | shadcn/ui theme for RJSF | Official theme, matches UI library |
| @rjsf/validator-ajv8 | ^6.2 | Schema validation for RJSF | Required validator for @rjsf/core v6 |
| Zustand | ^5 | Client state management | Lightweight (2KB), no boilerplate, selective subscriptions |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| lucide-react | latest | Icons | Paired with shadcn/ui by default |
| clsx + tailwind-merge | latest | Conditional class merging | Standard shadcn/ui utility (cn function) |
| date-fns | ^4 | Date formatting | For createdAt timestamps in server list |
| sonner | latest | Toast notifications | shadcn/ui's recommended toast library |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Vite + React | Next.js (static export) | Next.js adds SSR/SSG complexity with zero benefit for auth-only SPA. Static export has dynamic route limitations requiring server-side rewrites. Vite produces cleaner embeddable output. |
| Vite + React | Next.js (full) | Would require Node.js runtime in container, breaking single-binary architecture and distroless image |
| shadcn/ui | MUI / Mantine | shadcn/ui copies source into project (full control), pairs with Tailwind, has official RJSF theme |
| TanStack Query | SWR | TanStack Query has richer devtools, mutation support, and better cache invalidation patterns |
| Zustand | Redux Toolkit | Zustand is simpler for small apps; RTK is overkill for client state in a dashboard |
| react-router | TanStack Router | react-router is more mature and widely adopted; TanStack Router is newer but promising |

**Installation:**
```bash
# Initialize Vite project
npm create vite@latest web -- --template react-ts

# Core dependencies
npm install react-router @tanstack/react-query zustand

# RJSF for dynamic forms
npm install @rjsf/core @rjsf/utils @rjsf/validator-ajv8 @rjsf/shadcn

# UI (shadcn/ui is added via CLI, not npm)
npx shadcn@latest init
npm install sonner lucide-react date-fns
```

## Architecture Patterns

### Recommended Project Structure
```
web/
├── index.html               # Vite entry point
├── vite.config.ts           # Vite config (proxy to Go API in dev)
├── tsconfig.json
├── tailwind.config.ts
├── components.json          # shadcn/ui config
├── package.json
├── src/
│   ├── main.tsx             # React entry with providers
│   ├── App.tsx              # Router definition
│   ├── api/                 # API client layer
│   │   ├── client.ts        # fetch wrapper with auth token
│   │   ├── auth.ts          # login, register, refresh
│   │   ├── servers.ts       # CRUD + start/stop/restart
│   │   └── games.ts         # list games, get game
│   ├── hooks/               # TanStack Query hooks
│   │   ├── use-auth.ts      # Auth state + queries
│   │   ├── use-servers.ts   # Server list/detail queries + mutations
│   │   └── use-games.ts     # Game list/detail queries
│   ├── stores/              # Zustand stores
│   │   └── auth-store.ts    # JWT token, user claims, login/logout
│   ├── components/
│   │   ├── ui/              # shadcn/ui components (auto-generated)
│   │   ├── layout/          # Shell, sidebar, header
│   │   ├── servers/         # ServerCard, ServerList, ServerDetail
│   │   ├── games/           # GameCard, GameList
│   │   └── forms/           # GameConfigForm (RJSF wrapper)
│   ├── pages/               # Route page components
│   │   ├── login.tsx
│   │   ├── register.tsx
│   │   ├── dashboard.tsx
│   │   ├── servers.tsx
│   │   ├── server-detail.tsx
│   │   ├── create-server.tsx
│   │   └── admin/
│   │       ├── users.tsx
│   │       └── invites.tsx
│   ├── lib/                 # Utilities
│   │   ├── utils.ts         # cn() function for shadcn
│   │   └── jwt.ts           # Token decode, expiry check
│   └── types/               # TypeScript types matching API
│       ├── api.ts           # Response types
│       ├── server.ts        # GameServerResponse
│       └── game.ts          # GameResponse
└── dist/                    # Build output (git-ignored)
```

### Pattern 1: Embedded SPA in Go Binary
**What:** Build frontend to static assets, embed in Go binary with `//go:embed`, serve from same HTTP server as API.
**When to use:** Single-binary deployment target (Kubernetes, distroless image).
**Example:**
```go
// Source: Go embed docs + community patterns
package api

import (
    "embed"
    "io/fs"
    "net/http"
    "strings"
)

//go:embed all:web/dist
var frontendFS embed.FS

// serveSPA returns an http.Handler that serves the embedded SPA files.
// For any path not matching a static file, it falls back to index.html
// so that client-side routing works correctly.
func serveSPA() http.Handler {
    distFS, _ := fs.Sub(frontendFS, "web/dist")
    fileServer := http.FileServer(http.FS(distFS))

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Try to serve the file directly
        path := r.URL.Path

        // API routes are handled separately
        if strings.HasPrefix(path, "/api/") {
            http.NotFound(w, r)
            return
        }

        // Check if file exists in embedded FS
        f, err := distFS.Open(strings.TrimPrefix(path, "/"))
        if err != nil {
            // File not found: serve index.html for client-side routing
            index, _ := distFS.Open("index.html")
            defer index.Close()
            stat, _ := index.Stat()
            http.ServeContent(w, r, "index.html", stat.ModTime(), index.(io.ReadSeeker))
            return
        }
        f.Close()
        fileServer.ServeHTTP(w, r)
    })
}
```

### Pattern 2: JWT Auth with Token Refresh
**What:** Store JWT in Zustand store (memory), intercept X-Refresh-Token header from API responses to auto-refresh.
**When to use:** All authenticated API calls.
**Example:**
```typescript
// Source: Project-specific pattern matching existing Go auth middleware
// api/client.ts
import { useAuthStore } from '../stores/auth-store';

const API_BASE = '/api/v1';

export async function apiFetch<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const token = useAuthStore.getState().token;

  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...options.headers,
    },
  });

  // Auto-refresh: if server sends a refreshed token, update store
  const refreshToken = res.headers.get('X-Refresh-Token');
  if (refreshToken) {
    useAuthStore.getState().setToken(refreshToken);
  }

  if (!res.ok) {
    const error = await res.json().catch(() => ({ error: res.statusText }));
    throw new ApiError(res.status, error.error || 'Request failed');
  }

  if (res.status === 204) return undefined as T;
  return res.json();
}
```

### Pattern 3: Dynamic Form from parameterSchema
**What:** Use @rjsf/shadcn to render game configuration forms from the API's parameterSchema field.
**When to use:** Create/edit game server forms.
**Example:**
```typescript
// Source: RJSF docs + project game manifest schema structure
import { withTheme } from '@rjsf/core';
import { Theme as ShadcnTheme } from '@rjsf/shadcn';
import validator from '@rjsf/validator-ajv8';
import type { RJSFSchema } from '@rjsf/utils';

const Form = withTheme(ShadcnTheme);

interface GameConfigFormProps {
  parameterSchema: RJSFSchema;
  defaultParameters: Record<string, string>;
  onSubmit: (parameters: Record<string, string>) => void;
}

export function GameConfigForm({
  parameterSchema,
  defaultParameters,
  onSubmit
}: GameConfigFormProps) {
  return (
    <Form
      schema={parameterSchema}
      formData={defaultParameters}
      validator={validator}
      onSubmit={({ formData }) => onSubmit(formData)}
      // All fields are type: string in the schema (env vars)
      // Enums render as selects, patterns show validation errors
    />
  );
}
```

### Pattern 4: TanStack Query for Server State
**What:** Use TanStack Query hooks for all API data with automatic refetching for server status updates.
**When to use:** All API data fetching.
**Example:**
```typescript
// Source: TanStack Query docs
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '../api/client';

export function useServers() {
  return useQuery({
    queryKey: ['servers'],
    queryFn: () => apiFetch<{ data: GameServerResponse[]; count: number }>('/gameservers'),
    refetchInterval: 5000, // Poll every 5s for status updates
  });
}

export function useCreateServer() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateGameServerRequest) =>
      apiFetch<GameServerResponse>('/gameservers', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['servers'] });
    },
  });
}
```

### Anti-Patterns to Avoid
- **Storing JWT in localStorage:** Vulnerable to XSS. Store in Zustand (memory) instead. Token is lost on page refresh but that's acceptable -- user re-authenticates.
- **Polling every route for server status:** Only poll on pages that display server status (dashboard, server detail). Use `refetchInterval` selectively, not globally.
- **Giant monolithic components:** Keep page components thin; extract logic into hooks (TanStack Query) and UI into shadcn components.
- **Direct fetch() calls in components:** Always go through the `apiFetch` wrapper for consistent auth header injection and token refresh handling.
- **Ignoring the existing API response shape:** The API wraps lists in `{ data: [], count: N }` and errors in `{ error: "message" }`. TypeScript types must match exactly.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Dynamic forms from JSON Schema | Custom form renderer | @rjsf/core + @rjsf/shadcn | JSON Schema has dozens of edge cases (oneOf, conditionals, nested objects, arrays). RJSF has years of battle-testing |
| Server state caching | Custom fetch + useState | TanStack Query | Cache invalidation, background refetch, deduplication, retry, loading/error states |
| UI component library | Custom buttons/inputs/modals | shadcn/ui | Accessible (Radix primitives), themed, responsive, extensive component catalog |
| Client-side routing | Custom history.pushState | react-router | Nested layouts, route guards, URL params, lazy loading |
| Toast/notification system | Custom toast component | sonner | Animation, stacking, auto-dismiss, accessible |
| CSS utility framework | Custom CSS / CSS modules | Tailwind CSS | Consistent design tokens, responsive utilities, purged in production |

**Key insight:** This is a dashboard/admin panel -- a solved problem space. Nearly every UI pattern (data tables, forms, cards, modals, sidebars) has a polished shadcn/ui component. The unique value is in the RJSF integration with game manifests, not in reinventing UI primitives.

## Common Pitfalls

### Pitfall 1: SPA Routing with Embedded Filesystem
**What goes wrong:** Client-side routes (e.g., `/servers/my-mc-server`) return 404 when the user refreshes the page or navigates directly, because the Go file server looks for a physical file at that path.
**Why it happens:** SPAs use client-side routing; the server must return `index.html` for all non-file paths.
**How to avoid:** The Go SPA handler MUST fall back to serving `index.html` for any path that doesn't match a real file in the embedded filesystem. API routes (`/api/v1/`) must be registered BEFORE the SPA catch-all.
**Warning signs:** 404 errors on page refresh, blank page on direct URL navigation.

### Pitfall 2: CORS Configuration After Embedding
**What goes wrong:** Developers leave the development CORS configuration (`AllowedOrigins: ["*"]`) in production when it's no longer needed because the frontend is served from the same origin.
**Why it happens:** In development, Vite dev server runs on a different port (e.g., :5173) than the Go API (:8080), requiring CORS. In production with embedded frontend, same-origin eliminates CORS needs.
**How to avoid:** Use Vite's dev proxy (`vite.config.ts` proxy option) to proxy `/api/v1` to the Go server during development. This way, even in dev, requests appear same-origin. The existing CORS configuration can serve as a fallback for external API consumers.
**Warning signs:** Unnecessary CORS headers in production responses.

### Pitfall 3: JSON Schema Draft Mismatch
**What goes wrong:** Configuring RJSF's Ajv validator for draft-2020-12 when the actual schemas use draft-07 features, causing validation errors or form rendering issues.
**Why it happens:** The project documentation says "JSON Schema (Draft 2020-12)" but the actual Minecraft parameterSchema uses only draft-07 compatible features (enum, const, pattern, maxLength, default, type, required). The Go-side validator (santhosh-tekuri/jsonschema/v6) supports 2020-12, but the schema content is simple enough for draft-07.
**How to avoid:** Use RJSF's default validator (`@rjsf/validator-ajv8` with default draft-07 mode). Do NOT pass `AjvClass: Ajv2020` unless schemas actually use 2020-12 specific features (like `$dynamicRef`, `prefixItems`, etc.). If a future schema adds `$schema: "https://json-schema.org/draft/2020-12/schema"`, then switch to the Ajv2020 class.
**Warning signs:** Form validation errors that don't match Go-side validation, missing form fields.

### Pitfall 4: Server State Polling Performance
**What goes wrong:** Polling all game servers every 1 second to show "real-time" status updates, causing excessive API calls and Kubernetes API load.
**Why it happens:** Desire for real-time UX without implementing WebSockets.
**How to avoid:** Use 5-second polling interval via TanStack Query `refetchInterval` on the server list page. On individual server detail pages, can poll more aggressively (2s). Disable polling on pages that don't show server status. Consider that game server state changes are relatively slow (pod creation takes seconds to minutes).
**Warning signs:** High API request volume in network tab, slow UI when many servers exist.

### Pitfall 5: Missing Start/Stop/Restart Endpoints
**What goes wrong:** Building UI for start/stop/restart actions that reference API endpoints that don't exist yet.
**Why it happens:** The phase description mentions "User can stop, restart, and delete their game servers" and the additional_context lists `POST /api/servers/{id}/start`, `POST /api/servers/{id}/stop`, `POST /api/servers/{id}/restart` as "existing API endpoints" -- but these endpoints are NOT implemented in the current codebase. Only CRUD (create/get/list/update/delete) exists.
**How to avoid:** The frontend must either: (a) implement lifecycle actions through the existing update/delete endpoints (delete = stop+remove), or (b) these endpoints need to be added to the Go API first. The planner must account for this gap.
**Warning signs:** 404 responses when trying to start/stop/restart servers.

### Pitfall 6: embed Directive Path Issues
**What goes wrong:** The `//go:embed` directive doesn't find the frontend build output because the path is wrong or the build output doesn't exist at compile time.
**Why it happens:** `//go:embed` paths are relative to the Go source file containing the directive. If the directive is in `internal/api/spa.go`, the path must be relative from there to the build output.
**How to avoid:** Place the embed directive in a file at the project root (e.g., `cmd/frontend.go`) or use a path like `../../web/dist` from a nested package. Alternatively, place the `web/` directory at the project root and reference it from there. The Makefile must build the frontend BEFORE `go build`.
**Warning signs:** Build errors about missing embedded files, empty SPA in production.

## Code Examples

Verified patterns from official sources:

### TypeScript Types Matching API Responses
```typescript
// Source: Matching internal/api/handlers_gameserver.go GameServerResponse
export interface GameServerResponse {
  name: string;
  gameType: string;
  state: 'Creating' | 'Starting' | 'Ready' | 'Allocated' | 'Shutdown' | 'Error';
  address?: string;
  ports?: PortResponse[];
  parameters?: Record<string, string>;
  createdAt: string;
}

export interface PortResponse {
  name: string;
  port: number;
  protocol: string;
}

// Source: Matching internal/api/handlers_games.go GameResponse
export interface GameResponse {
  name: string;
  displayName: string;
  image: string;
  ports: PortInfo[];
  parameters: Record<string, string>;
  parameterSchema?: Record<string, unknown>; // JSON Schema object
}

export interface PortInfo {
  name: string;
  containerPort: number;
  protocol: string;
}

// API list wrapper
export interface ListResponse<T> {
  data: T[];
  count: number;
}

// API error response
export interface ErrorResponse {
  error: string;
  details?: string;
}

// Source: Matching internal/api/request.go
export interface CreateGameServerRequest {
  name: string;
  gameType: string;
  parameters?: Record<string, string>;
}

export interface LoginRequest {
  username: string;
  password: string;
}

export interface RegisterRequest {
  username: string;
  email: string;
  password: string;
  inviteToken: string;
}

export interface LoginResponse {
  token: string;
}

export interface RegisterResponse {
  token: string;
  username: string;
}
```

### Zustand Auth Store
```typescript
// Source: Zustand v5 docs pattern
import { create } from 'zustand';

interface AuthState {
  token: string | null;
  user: { username: string; email: string; role: string } | null;
  setToken: (token: string) => void;
  logout: () => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  token: null,
  user: null,
  setToken: (token) => {
    // Decode JWT payload (base64) to extract claims
    const payload = JSON.parse(atob(token.split('.')[1]));
    set({
      token,
      user: {
        username: payload.username,
        email: payload.email,
        role: payload.role,
      },
    });
  },
  logout: () => set({ token: null, user: null }),
}));
```

### Vite Dev Proxy Configuration
```typescript
// vite.config.ts
// Source: Vite docs - server.proxy
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'path';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/healthz': {
        target: 'http://localhost:8080',
      },
    },
  },
});
```

### Route Guard for Protected Routes
```typescript
// Source: react-router v7 patterns
import { Navigate, Outlet } from 'react-router';
import { useAuthStore } from '../stores/auth-store';

export function ProtectedRoute() {
  const token = useAuthStore((s) => s.token);
  if (!token) return <Navigate to="/login" replace />;
  return <Outlet />;
}

export function AdminRoute() {
  const user = useAuthStore((s) => s.user);
  if (user?.role !== 'admin') return <Navigate to="/" replace />;
  return <Outlet />;
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Create React App (CRA) | Vite | 2023-2024 | CRA deprecated, Vite is now React's recommended build tool |
| Redux + Redux Toolkit | Zustand / Jotai for small apps | 2023+ | Less boilerplate for simple state; RTK still valid for complex apps |
| Custom fetch + useState | TanStack Query | 2022+ | Eliminates custom caching, loading, error handling code |
| CSS Modules / styled-components | Tailwind CSS | 2023+ | Utility-first became dominant, especially with shadcn/ui |
| Material UI (MUI) dominance | shadcn/ui + Radix | 2023+ | shadcn/ui's copy-paste model gives full control, faster adoption |
| react-jsonschema-form v5 | @rjsf v6 with @rjsf/shadcn | 2025+ | Official shadcn theme, React 18+ required, deprecated packages removed |
| Next.js Pages Router | Next.js App Router / Vite for SPAs | 2024+ | App Router for full-stack Next.js; Vite for pure SPAs |

**Deprecated/outdated:**
- Create React App: Officially deprecated, removed from React docs
- @rjsf/material-ui: Removed in RJSF v6, replaced by @rjsf/mui
- @rjsf/bootstrap-4: Removed in RJSF v6, replaced by @rjsf/react-bootstrap
- enumNames in JSON Schema for RJSF: Deprecated, use oneOf with const+title instead

## Open Questions

1. **Start/Stop/Restart API Endpoints**
   - What we know: The phase description and additional_context list these as "existing API endpoints" but they are NOT in the current codebase. Only CRUD endpoints exist.
   - What's unclear: Whether these need to be added to the Go API as part of Phase 6 or if this is a prerequisite from a different phase.
   - Recommendation: The planner should include a sub-plan to add `POST /api/v1/gameservers/{name}/start`, `POST /api/v1/gameservers/{name}/stop`, `POST /api/v1/gameservers/{name}/restart` endpoints to the Go API, or mark this as a blocker. Lifecycle actions could map to: stop = annotation/label change triggering reconciliation, restart = delete pod (controller recreates), but this needs design.

2. **Token Persistence Across Page Refreshes**
   - What we know: Zustand stores are in-memory by default. On page refresh, the user loses their session.
   - What's unclear: Whether the user experience of re-logging in on refresh is acceptable for a homelab tool.
   - Recommendation: Start with in-memory only (most secure). If UX is poor, add Zustand `persist` middleware with `sessionStorage` (cleared on tab close) as a follow-up. Avoid `localStorage` for JWT storage.

3. **Real-Time Status Updates (Polling vs WebSocket)**
   - What we know: TanStack Query polling at 5s intervals works for dashboard UX. The Go API currently has no WebSocket support.
   - What's unclear: Whether 5s polling is responsive enough for the "User sees connection info after server is ready" requirement.
   - Recommendation: Start with polling. WebSocket support can be added to the Go API later if polling latency is unacceptable. Game server pod creation typically takes 10-60 seconds, so 5s polling catches state changes within one poll cycle.

4. **Frontend Build Integration with Go Build**
   - What we know: The Dockerfile currently builds Go only. The Makefile has `make build` for Go. Frontend build needs to happen BEFORE Go build.
   - What's unclear: Exact Makefile/Dockerfile modifications needed to chain the builds.
   - Recommendation: Add a `make build-frontend` target, modify `make build` to depend on it, update Dockerfile with a Node.js build stage. The `.dockerignore` needs updating to include `web/` directory. The `//go:embed` directive needs the `web/dist` directory to exist at Go compile time.

5. **Admin Panel Scope**
   - What we know: API has admin endpoints: `POST /api/v1/admin/invites`, `GET /api/v1/admin/users`, `DELETE /api/v1/admin/users/{username}`.
   - What's unclear: How much admin functionality to include in the frontend for Phase 6 vs later.
   - Recommendation: Include basic admin pages (user list, invite creation) since the API already supports them and the success criteria don't explicitly exclude admin features. Keep them simple -- data table + invite form.

## Sources

### Primary (HIGH confidence)
- Go codebase analysis: `internal/api/routes.go`, `internal/api/handlers_gameserver.go`, `internal/api/handlers_games.go`, `internal/api/handlers_auth.go`, `internal/api/handlers_admin.go`, `internal/api/request.go`, `internal/api/response.go` -- verified exact API shape, response types, route structure
- Go codebase analysis: `games/minecraft/manifest.yaml` -- verified parameterSchema structure uses draft-07 features only
- Go codebase analysis: `Dockerfile`, `Makefile`, `.dockerignore`, `cmd/main.go` -- verified build pipeline and deployment architecture
- [Next.js SPA Guide](https://nextjs.org/docs/app/guides/single-page-applications) -- Official docs on SPA patterns, static export, catch-all routes
- [Next.js Static Exports](https://nextjs.org/docs/app/guides/static-exports) -- Official docs on limitations
- [Go embed package docs](https://pkg.go.dev/embed) -- Official Go docs for embed directive

### Secondary (MEDIUM confidence)
- [@rjsf/shadcn npm](https://www.npmjs.com/package/@rjsf/shadcn) -- Version 6.2.x available, official shadcn theme for RJSF
- [@rjsf/core npm](https://www.npmjs.com/package/@rjsf/core) -- Version 6.2.5 latest
- [RJSF Validation Docs](https://rjsf-team.github.io/react-jsonschema-form/docs/usage/validation/) -- Ajv2020 class for draft-2020-12, default is draft-07
- [RJSF Draft 2020-12 Issue #3750](https://github.com/rjsf-team/react-jsonschema-form/issues/3750) -- "draft-2020-12 has breaking changes and hasn't been fully tested with @rjsf"
- [Serving SPAs from Go](https://hackandsla.sh/posts/2021-11-06-serve-spa-from-go/) -- Intercept-404 pattern for SPA fallback
- [Portable apps with Go and Next.js](https://v0x.nl/articles/portable-apps-go-nextjs/) -- go:embed with fs.Sub pattern
- [TanStack Query Overview](https://tanstack.com/query/v5/docs/framework/react/overview) -- Official docs, v5 patterns
- [Zustand GitHub](https://github.com/pmndrs/zustand) -- v5 features, persist middleware
- [shadcn/ui Installation](https://ui.shadcn.com/docs/installation/next) -- Official docs for framework setup

### Tertiary (LOW confidence)
- Web search results for "Vite vs Next.js for admin panels" -- multiple blog sources agreeing Vite SPA is preferred for authenticated dashboards
- Web search results for Next.js 16 availability -- Current stable appears to be Next.js 16.1 (Dec 2025), but Vite recommendation makes this moot

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All libraries verified via npm, official docs, and community adoption metrics. shadcn/ui + RJSF integration confirmed via @rjsf/shadcn package existence.
- Architecture: HIGH - Embed-in-Go pattern well-documented with multiple production examples. API shape verified from actual codebase.
- Pitfalls: HIGH - Verified by reading actual codebase (missing endpoints, schema draft mismatch, embed path issues). SPA routing fallback is a known, well-documented challenge.
- Dynamic forms (RJSF): MEDIUM - @rjsf/shadcn exists but version info suggests recent release. Draft 2020-12 support is explicitly flagged as incomplete by RJSF maintainers, but our schemas don't need it.

**Research date:** 2026-02-11
**Valid until:** 2026-03-13 (30 days -- stable ecosystem, React/Vite/shadcn not fast-moving in breaking ways)
