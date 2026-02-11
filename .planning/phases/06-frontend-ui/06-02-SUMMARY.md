---
phase: 06-frontend-ui
plan: 02
subsystem: ui
tags: [react, shadcn-ui, tanstack-query, react-router, zustand, sonner, auth, sidebar, games]

# Dependency graph
requires:
  - phase: 06-frontend-ui
    plan: 01
    provides: "Vite + React scaffold, API client, auth store, TypeScript types"
provides:
  - "Login page with username/password authentication"
  - "Register page with invite token registration"
  - "ProtectedRoute and AdminRoute guards"
  - "App shell layout with collapsible sidebar and header"
  - "Game browser page with TanStack Query hooks"
  - "Full route tree covering all planned application paths"
affects: [06-03, 06-04]

# Tech tracking
tech-stack:
  added: [shadcn-sidebar, shadcn-card, shadcn-badge, sonner-toasts]
  patterns: [protected-route-guard, app-shell-layout, tanstack-query-hooks, game-card-grid]

key-files:
  created:
    - web/src/api/auth.ts
    - web/src/api/games.ts
    - web/src/hooks/use-auth.ts
    - web/src/hooks/use-games.ts
    - web/src/pages/login.tsx
    - web/src/pages/register.tsx
    - web/src/pages/dashboard.tsx
    - web/src/pages/games.tsx
    - web/src/components/auth/protected-route.tsx
    - web/src/components/layout/app-shell.tsx
    - web/src/components/layout/header.tsx
    - web/src/components/layout/sidebar-nav.tsx
    - web/src/components/games/game-card.tsx
    - web/src/components/games/game-list.tsx
  modified:
    - web/src/App.tsx
    - web/src/main.tsx
    - web/src/components/ui/sonner.tsx

key-decisions:
  - "Sidebar component named sidebar-nav.tsx to avoid conflict with shadcn ui/sidebar.tsx"
  - "Sonner component simplified to remove next-themes dependency (not needed in Vite SPA)"
  - "shadcn toast component deprecated in favor of sonner -- used sonner directly"

patterns-established:
  - "ProtectedRoute: checks useAuthStore token, redirects to /login if null, renders Outlet"
  - "AdminRoute: checks useAuthStore user.role, redirects to / if not admin, renders Outlet"
  - "AppShell layout: SidebarProvider > AppSidebar + SidebarInset(Header + Outlet)"
  - "TanStack Query hooks: useGames/useGame with 5min staleTime for rarely-changing data"
  - "GameCard pattern: card with game info, badge for type, Create Server CTA linking to /servers/create?game={name}"

# Metrics
duration: 4min
completed: 2026-02-11
---

# Phase 6 Plan 02: Auth Pages, App Layout, and Game Browser Summary

**Login/register forms with protected route guards, sidebar navigation app shell, and game browser page with TanStack Query data fetching**

## Performance

- **Duration:** 4 min
- **Started:** 2026-02-11T21:40:33Z
- **Completed:** 2026-02-11T21:44:48Z
- **Tasks:** 2
- **Files modified:** 31

## Accomplishments
- Login and register pages with form validation, error toasts via sonner, and mutation-based submission
- Protected route guards (ProtectedRoute and AdminRoute) redirecting unauthenticated/unauthorized users
- Full app shell layout with collapsible sidebar navigation, header with user info and logout
- Game browser page with responsive card grid, loading skeletons, and empty state handling
- Complete route tree covering login, register, dashboard, games, servers, admin paths
- TanStack Query hooks for game list and detail with 5-minute stale time

## Task Commits

Each task was committed atomically:

1. **Task 1: Auth pages, protected routes, and app layout** - `3e1943c` (feat)
2. **Task 2: Game browser with TanStack Query hooks** - `ae7e488` (feat)

## Files Created/Modified
- `web/src/api/auth.ts` - API functions for login, register, and token refresh
- `web/src/api/games.ts` - API functions for listing games and fetching by type
- `web/src/hooks/use-auth.ts` - TanStack Query mutations for login/register plus useLogout
- `web/src/hooks/use-games.ts` - TanStack Query hooks for game list and detail
- `web/src/pages/login.tsx` - Card-based login form with username/password
- `web/src/pages/register.tsx` - Card-based registration form with invite token
- `web/src/pages/dashboard.tsx` - Welcome page with server count placeholder and create CTA
- `web/src/pages/games.tsx` - Game browser page rendering GameList component
- `web/src/components/auth/protected-route.tsx` - ProtectedRoute and AdminRoute guards
- `web/src/components/layout/app-shell.tsx` - Main layout with sidebar + header + outlet
- `web/src/components/layout/header.tsx` - Header with branding, user info, logout button
- `web/src/components/layout/sidebar-nav.tsx` - Navigation sidebar with admin section
- `web/src/components/games/game-card.tsx` - Game card with info, badge, and create button
- `web/src/components/games/game-list.tsx` - Responsive game grid with loading/empty states
- `web/src/App.tsx` - Full route tree with protected/admin routes
- `web/src/main.tsx` - Added Toaster from sonner
- `web/src/components/ui/sonner.tsx` - Fixed to remove next-themes dependency

## Decisions Made
- **Sidebar named sidebar-nav.tsx:** Avoids filename collision with shadcn's ui/sidebar.tsx component. Same functionality, clearer distinction.
- **Sonner over toast:** shadcn's toast component is deprecated in favor of sonner. Used sonner directly and fixed the generated component to remove the next-themes dependency (not applicable in Vite SPA).
- **Games page placeholder in Task 1:** Created minimal placeholder so App.tsx imports compile, then replaced with full implementation in Task 2.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed shadcn sonner component next-themes dependency**
- **Found during:** Task 1 (Installing shadcn components)
- **Issue:** shadcn's generated sonner.tsx imports `useTheme` from `next-themes` which is a Next.js-specific package. In a Vite SPA there is no theme provider from next-themes.
- **Fix:** Simplified the Toaster component to remove the useTheme call and next-themes import. Toaster works without theme detection (defaults to system).
- **Files modified:** web/src/components/ui/sonner.tsx
- **Verification:** Build succeeds, no import errors
- **Committed in:** 3e1943c (Task 1 commit)

**2. [Rule 3 - Blocking] Created sidebar-nav.tsx instead of sidebar.tsx**
- **Found during:** Task 1 (Creating sidebar navigation)
- **Issue:** Plan specified `sidebar.tsx` but shadcn already installed `components/ui/sidebar.tsx`. Creating another `sidebar.tsx` in layout/ would be confusing and the plan intent was a navigation sidebar, not a replacement of the UI primitive.
- **Fix:** Named the navigation component `sidebar-nav.tsx` with export `AppSidebar` to distinguish from the UI primitive.
- **Files modified:** web/src/components/layout/sidebar-nav.tsx
- **Verification:** Build succeeds, AppShell correctly imports AppSidebar
- **Committed in:** 3e1943c (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (1 bug, 1 blocking)
**Impact on plan:** Both fixes necessary for correct build. No scope creep. Sidebar naming is a minor structural difference that improves clarity.

## Issues Encountered
- shadcn `toast` component is deprecated -- used `sonner` component instead (shadcn CLI rejects `toast` with error message)

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Auth pages and app layout ready for Plan 03 (Server management UI)
- Game browser wired to TanStack Query, ready for live API data
- Server list/detail/create pages are placeholder routes, ready for Plan 03 implementation
- Admin user/invite pages are placeholder routes, ready for Plan 04 implementation

---
*Phase: 06-frontend-ui*
*Completed: 2026-02-11*

## Self-Check: PASSED

- All 18 key files verified present on disk
- Both task commits verified in git log (3e1943c, ae7e488)
- Frontend build succeeds (npm run build) with zero type errors
