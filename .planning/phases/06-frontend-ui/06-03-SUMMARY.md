---
phase: 06-frontend-ui
plan: 03
subsystem: ui
tags: [react, rjsf, tanstack-query, shadcn-ui, server-management, dynamic-forms, polling]

# Dependency graph
requires:
  - phase: 06-frontend-ui
    plan: 02
    provides: "Auth pages, app shell layout, game browser, route tree with server placeholders"
provides:
  - "Server API layer with CRUD and lifecycle (start/stop/restart) functions"
  - "TanStack Query hooks with 5s list polling and 2s detail polling"
  - "RJSF dynamic form component from game parameterSchema"
  - "Server list page with live status polling and server count"
  - "Create server page with two-step flow: game selection then config form"
  - "Server detail page with connection info, parameters, and lifecycle actions"
  - "Server status badge with color-coded state mapping"
affects: [06-04]

# Tech tracking
tech-stack:
  added: [rjsf-shadcn-forms, alert-dialog, dialog, table]
  patterns: [server-api-layer, tanstack-mutation-hooks, rjsf-dynamic-forms, status-polling, dns-label-validation, confirmation-dialogs]

key-files:
  created:
    - web/src/api/servers.ts
    - web/src/hooks/use-servers.ts
    - web/src/components/servers/server-status-badge.tsx
    - web/src/components/servers/server-card.tsx
    - web/src/components/servers/server-list.tsx
    - web/src/components/forms/game-config-form.tsx
    - web/src/pages/servers.tsx
    - web/src/pages/create-server.tsx
    - web/src/pages/server-detail.tsx
  modified:
    - web/src/App.tsx

key-decisions:
  - "IChangeEvent imported from @rjsf/core (not @rjsf/utils) for form submit handler typing"
  - "Draft-07 default validator used for RJSF -- schemas only use draft-07 features per research"
  - "Custom ServerStatusBadge with Tailwind classes instead of shadcn Badge variants for precise color control"

patterns-established:
  - "Server API layer: apiFetch wrapper for all server CRUD and lifecycle endpoints"
  - "TanStack Query mutations: invalidate both ['servers'] and ['servers', name] on lifecycle actions"
  - "RJSF form: withTheme(ShadcnTheme) pattern with draft-07 validator for dynamic game config"
  - "Two-step create flow: game selection via query param (?game=) then config form"
  - "Confirmation dialogs: AlertDialog for destructive actions (stop, restart, delete)"
  - "DNS label validation: regex + length check for server names"

# Metrics
duration: 5min
completed: 2026-02-11
---

# Phase 6 Plan 03: Server Management UI Summary

**Server CRUD with RJSF dynamic forms from parameterSchema, live status polling, connection info display, and lifecycle actions (start/stop/restart/delete)**

## Performance

- **Duration:** 5 min
- **Started:** 2026-02-11T21:47:42Z
- **Completed:** 2026-02-11T21:52:53Z
- **Tasks:** 2
- **Files modified:** 13

## Accomplishments
- Server API layer with 8 endpoints: list, get, create, update, delete, start, stop, restart
- TanStack Query hooks with automatic polling (5s list, 2s detail) and mutation cache invalidation
- RJSF dynamic form generation from game parameterSchema using @rjsf/shadcn theme
- Server list page with responsive card grid, status badges, quick actions, and empty state
- Two-step create server flow: game selection then RJSF configuration form with DNS label validation
- Server detail page with connection info (DNS address + port table), parameters table, and lifecycle action buttons with confirmation dialogs
- Server status badge with color-coded state mapping for all 6 states (Creating, Starting, Ready, Allocated, Shutdown, Error)

## Task Commits

Each task was committed atomically:

1. **Task 1: Server API layer, hooks, status badge, and RJSF form** - `7d9f60d` (feat)
2. **Task 2: Server list, creation flow, and detail pages** - `94f58eb` (feat)

## Files Created/Modified
- `web/src/api/servers.ts` - API functions for server CRUD and lifecycle endpoints
- `web/src/hooks/use-servers.ts` - TanStack Query hooks with polling and mutations
- `web/src/components/servers/server-status-badge.tsx` - Color-coded state badge component
- `web/src/components/servers/server-card.tsx` - Server card with status, connection info, and quick actions
- `web/src/components/servers/server-list.tsx` - Responsive server grid with loading/empty states
- `web/src/components/forms/game-config-form.tsx` - RJSF dynamic form from parameterSchema
- `web/src/pages/servers.tsx` - Server list page with count and create button
- `web/src/pages/create-server.tsx` - Two-step create flow: game selection then config form
- `web/src/pages/server-detail.tsx` - Detail page with connection info, parameters, and lifecycle actions
- `web/src/App.tsx` - Server routes wired to real page components (replacing placeholders)
- `web/src/components/ui/alert-dialog.tsx` - shadcn alert dialog component (added)
- `web/src/components/ui/dialog.tsx` - shadcn dialog component (added)
- `web/src/components/ui/table.tsx` - shadcn table component (added)

## Decisions Made
- **IChangeEvent from @rjsf/core:** The form submit event type `IChangeEvent` is exported from `@rjsf/core`, not `@rjsf/utils` as might be expected. Fixed import during Task 1.
- **Draft-07 default validator:** Per research (Pitfall 3), schemas only use draft-07 compatible features. Using RJSF's default Ajv validator without draft-2020-12 configuration.
- **Custom status badge over shadcn Badge variants:** Used Tailwind color classes directly for precise 6-state color mapping rather than being limited to shadcn's predefined badge variants.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed IChangeEvent import source**
- **Found during:** Task 1 (Creating GameConfigForm)
- **Issue:** Plan referenced importing `IChangeEvent` from `@rjsf/utils`, but the type is actually exported from `@rjsf/core`.
- **Fix:** Changed import to `import type { IChangeEvent } from '@rjsf/core'`
- **Files modified:** web/src/components/forms/game-config-form.tsx
- **Verification:** Build succeeds with correct type
- **Committed in:** 7d9f60d (Task 1 commit)

**2. [Rule 1 - Bug] Removed unused imports in create-server.tsx**
- **Found during:** Task 2 (Build verification)
- **Issue:** TypeScript strict mode flagged unused `Loader2` import and duplicate `useNavigate` reference in top-level component.
- **Fix:** Removed unused `Loader2` import and top-level `useNavigate` call (still used in ConfigureStep sub-component).
- **Files modified:** web/src/pages/create-server.tsx
- **Verification:** Build succeeds with zero type errors
- **Committed in:** 94f58eb (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (2 bugs)
**Impact on plan:** Minor type/import fixes. No scope creep. All planned functionality delivered.

## Issues Encountered
- Bundle size warning (946KB) from RJSF library inclusion -- expected for JSON Schema form generation, can be addressed with code splitting in a future optimization pass.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All server management UI complete: list, create, detail with lifecycle actions
- Server routes wired and building successfully
- Admin pages (users, invites) remain as placeholders, ready for Plan 04 implementation
- RJSF dynamic forms working for game parameterSchema consumption

---
*Phase: 06-frontend-ui*
*Completed: 2026-02-11*

## Self-Check: PASSED

- All 13 key files verified present on disk
- Both task commits verified in git log (7d9f60d, 94f58eb)
- Frontend build succeeds (npm run build) with zero type errors
