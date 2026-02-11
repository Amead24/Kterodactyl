---
phase: 06-frontend-ui
plan: 01
subsystem: ui, api
tags: [react, vite, typescript, tailwind, shadcn-ui, zustand, tanstack-query, rjsf, lifecycle-api]

# Dependency graph
requires:
  - phase: 04-api-server-bridge
    provides: "Go API server with auth, CRUD endpoints, manifest loader"
  - phase: 05-game-definition-framework
    provides: "Game manifests with parameterSchema for dynamic form generation"
provides:
  - "Vite + React + TypeScript frontend project scaffold in web/"
  - "API client wrapper (apiFetch) with JWT auth header injection and token refresh"
  - "Zustand auth store with JWT decode and user claims extraction"
  - "TypeScript types matching all Go API response shapes"
  - "Three lifecycle API endpoints: POST start/stop/restart"
  - "State machine transitions: Shutdown->Creating, Error->Creating"
affects: [06-02, 06-03, 06-04, 07-go-embed, build-pipeline]

# Tech tracking
tech-stack:
  added: [react-19, vite-7, typescript-5.9, tailwindcss-4, shadcn-ui, tanstack-query-5, zustand-5, rjsf-6, react-router-7, sonner, lucide-react, date-fns]
  patterns: [api-client-wrapper, zustand-auth-store, vite-dev-proxy, status-subresource-update]

key-files:
  created:
    - web/package.json
    - web/vite.config.ts
    - web/tsconfig.json
    - web/tsconfig.app.json
    - web/components.json
    - web/src/main.tsx
    - web/src/App.tsx
    - web/src/index.css
    - web/src/lib/utils.ts
    - web/src/types/api.ts
    - web/src/api/client.ts
    - web/src/stores/auth-store.ts
  modified:
    - .gitignore
    - api/v1alpha1/gameserver_lifecycle.go
    - internal/api/handlers_gameserver.go
    - internal/api/handlers_gameserver_test.go
    - internal/api/helpers_test.go
    - internal/api/routes.go

key-decisions:
  - "Tailwind CSS v4 with @tailwindcss/vite plugin (not PostCSS-based v3) -- shadcn init detected and configured automatically"
  - "ValidTransitions updated: Shutdown->Creating and Error->Creating for lifecycle API restart support"
  - "WithStatusSubresource added to fake client builder for proper status sub-resource update testing"
  - "Token stored in memory only (Zustand, no localStorage) per security best practices -- re-auth on refresh"
  - "Status().Update() pattern for lifecycle handlers -- separates spec from status updates in K8s"

patterns-established:
  - "apiFetch<T>() wrapper: all API calls go through client.ts for consistent auth header injection and token refresh"
  - "Zustand store pattern: create<State>((set) => ({...})) with JWT decode in setToken"
  - "Vite dev proxy: /api and /healthz proxied to localhost:8080 for same-origin development"
  - "createTestGameServerWithState helper: creates GameServer with specific lifecycle state for handler tests"
  - "Lifecycle endpoint pattern: check current state, return 409 for invalid transitions, Status().Update() for state changes"

# Metrics
duration: 8min
completed: 2026-02-11
---

# Phase 6 Plan 01: Frontend Scaffold and Lifecycle API Summary

**Vite React SPA scaffold with shadcn/ui, API client with JWT auth, Zustand auth store, and three Go lifecycle endpoints (start/stop/restart) with 11 tests**

## Performance

- **Duration:** 8 min
- **Started:** 2026-02-11T21:29:10Z
- **Completed:** 2026-02-11T21:37:39Z
- **Tasks:** 2
- **Files modified:** 22

## Accomplishments
- Fully functional Vite + React + TypeScript project scaffold with all dependencies installed and building to static assets
- API client wrapper with JWT Bearer token injection and automatic X-Refresh-Token handling
- Zustand auth store with JWT payload decode for user claims extraction
- TypeScript types matching all Go API response shapes (GameServerResponse, GameResponse, ListResponse, auth types)
- Three new lifecycle API endpoints (POST start/stop/restart) with proper state transition logic
- 11 new lifecycle handler tests covering success, conflict (409), and not-found (404) cases

## Task Commits

Each task was committed atomically:

1. **Task 1: Scaffold Vite React project with all dependencies** - `0b0038e` (feat)
2. **Task 2: Add lifecycle endpoints to Go API** - `13de755` (feat)

## Files Created/Modified
- `web/package.json` - Frontend project manifest with all 12+ dependencies
- `web/vite.config.ts` - Vite config with React plugin, Tailwind, path alias, and API proxy
- `web/tsconfig.json` / `web/tsconfig.app.json` - TypeScript config with @/ path alias
- `web/components.json` - shadcn/ui configuration (new-york style, CSS variables)
- `web/src/index.css` - Tailwind v4 CSS with shadcn/ui theme variables (light + dark)
- `web/src/lib/utils.ts` - cn() class merging utility from shadcn/ui
- `web/src/types/api.ts` - 13 TypeScript interfaces matching Go API exactly
- `web/src/api/client.ts` - apiFetch<T>() wrapper with auth headers, refresh, ApiError class
- `web/src/stores/auth-store.ts` - Zustand auth store with setToken/logout
- `web/src/App.tsx` - Minimal BrowserRouter placeholder with Kterodactyl heading
- `web/src/main.tsx` - React 19 entry with QueryClientProvider
- `web/index.html` - HTML entry point with Kterodactyl title
- `api/v1alpha1/gameserver_lifecycle.go` - Added Shutdown->Creating and Error->Creating transitions
- `internal/api/handlers_gameserver.go` - Three lifecycle handlers (start/stop/restart)
- `internal/api/routes.go` - Lifecycle route registration under /{name}
- `internal/api/handlers_gameserver_test.go` - 11 new lifecycle tests + createTestGameServerWithState helper
- `internal/api/helpers_test.go` - WithStatusSubresource for fake client
- `.gitignore` - Added web/node_modules/ and web/dist/

## Decisions Made
- **Tailwind CSS v4 over v3:** shadcn/ui init detected Vite and configured Tailwind v4 with @tailwindcss/vite plugin automatically. Uses @import "tailwindcss" syntax instead of @tailwind directives.
- **ValidTransitions expanded:** Shutdown and Error states now allow transition to Creating, enabling the lifecycle API to restart servers. IsTerminal() still returns true for Shutdown (operator semantics unchanged).
- **WithStatusSubresource for fake client:** Required for Status().Update() to work correctly in tests. This was missing from the existing test setup but didn't cause issues until the lifecycle handlers needed status updates.
- **Memory-only token storage:** JWT stored in Zustand (memory), not localStorage. Token lost on page refresh per security best practices (avoids XSS exposure).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Updated ValidTransitions for Shutdown->Creating and Error->Creating**
- **Found during:** Task 2 (Lifecycle endpoints)
- **Issue:** ValidTransitions had Shutdown as terminal (no transitions) and Error only allowing Shutdown. Start/restart endpoints need to transition these states to Creating.
- **Fix:** Added GameServerStateCreating to Shutdown and Error transition lists
- **Files modified:** api/v1alpha1/gameserver_lifecycle.go
- **Verification:** All lifecycle tests pass; existing tests unaffected
- **Committed in:** 13de755 (Task 2 commit)

**2. [Rule 3 - Blocking] Added WithStatusSubresource to fake client builder**
- **Found during:** Task 2 (Lifecycle tests)
- **Issue:** Fake client's Status().Update() returned "not found" without WithStatusSubresource registration
- **Fix:** Added WithStatusSubresource(&gamev1alpha1.GameServer{}) to fake client builder in helpers_test.go
- **Files modified:** internal/api/helpers_test.go
- **Verification:** All 56 API tests pass (existing + new lifecycle tests)
- **Committed in:** 13de755 (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (2 blocking issues)
**Impact on plan:** Both auto-fixes necessary for lifecycle endpoints to function. No scope creep. State machine extension is semantically correct -- the operator reconciler already handles Creating state by creating Pods.

## Issues Encountered
- shadcn/ui init required Tailwind CSS to be installed first (Vite template doesn't include it). Installed tailwindcss + @tailwindcss/vite before running shadcn init.
- Removed Vite scaffold artifacts (App.css, react.svg, vite.svg) that were replaced by the project files.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Frontend project scaffold ready for Plan 02 (Auth pages: login/register)
- API client and auth store ready for use in React components
- All required shadcn/ui and RJSF dependencies installed
- Lifecycle endpoints available for server management UI in Plan 03

---
*Phase: 06-frontend-ui*
*Completed: 2026-02-11*

## Self-Check: PASSED

- All 16 key files verified present on disk
- Both task commits verified in git log (0b0038e, 13de755)
- Frontend build succeeds (npm run build)
- All 56 Go API tests pass (including 11 new lifecycle tests)
