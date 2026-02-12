---
phase: 07-console-realtime
plan: 02
subsystem: ui
tags: [xterm, websocket, react, console, metrics, tabs]

# Dependency graph
requires:
  - phase: 07-01
    provides: "WebSocket console endpoint and metrics REST endpoint on the Go backend"
  - phase: 06-frontend-ui
    provides: "React SPA with shadcn components, Zustand auth store, and server detail page"
provides:
  - "xterm.js terminal component with auto-fit and ResizeObserver"
  - "WebSocket console hook with JWT query param auth and exponential backoff reconnection"
  - "Metrics polling hook and CPU/memory display panel with progress bars"
  - "Tabbed server detail page with Overview, Console, and Resources tabs"
affects: [08-helm-packaging, 12-documentation]

# Tech tracking
tech-stack:
  added: ["@xterm/xterm", "@xterm/addon-fit", "shadcn tabs"]
  patterns: ["Native WebSocket hook with useRef for connection state", "Polling hook via useQuery refetchInterval", "Tabbed page layout with conditional tab visibility"]

key-files:
  created:
    - web/src/hooks/use-console.ts
    - web/src/hooks/use-metrics.ts
    - web/src/components/console/terminal.tsx
    - web/src/components/console/console-panel.tsx
    - web/src/components/servers/metrics-panel.tsx
    - web/src/components/ui/tabs.tsx
  modified:
    - web/src/types/api.ts
    - web/src/api/servers.ts
    - web/src/pages/server-detail.tsx
    - web/package.json

key-decisions:
  - "Native WebSocket API used instead of react-use-websocket to avoid React 19 peer dependency conflict"
  - "HTML input field for command entry instead of xterm onData for cleaner UX"
  - "Metrics unavailability shown as muted text, not error toast, since metrics-server may not be installed"

patterns-established:
  - "WebSocket hook pattern: useRef for connection + timeout, useCallback for stable handlers, intentionalClose flag for cleanup"
  - "Conditional tab visibility: tabs only rendered when server state matches required conditions"

# Metrics
duration: 4min
completed: 2026-02-12
---

# Phase 7 Plan 2: Console & Metrics Frontend Summary

**xterm.js console terminal with WebSocket streaming and CPU/memory metrics dashboard integrated via tabbed server detail page**

## Performance

- **Duration:** 4 min
- **Started:** 2026-02-12T13:05:18Z
- **Completed:** 2026-02-12T13:09:18Z
- **Tasks:** 3
- **Files modified:** 11

## Accomplishments
- xterm.js terminal renders real-time console output from WebSocket stream with auto-fit resizing
- Command input sends typed commands to running game servers via WebSocket JSON messages
- Connection status indicator (green/yellow/red) with exponential backoff reconnection (up to 5 retries)
- CPU and memory metrics displayed with progress bars, polling every 5 seconds, with graceful degradation
- Server detail page restructured with tabbed layout: Overview, Console, Resources
- Console and Resources tabs only visible when server is in Ready or Allocated state

## Task Commits

Each task was committed atomically:

1. **Task 1: Install xterm dependencies, create WebSocket console hook and terminal component** - `f7d97d5` (feat)
2. **Task 2: Create metrics hook, metrics panel, and add API types** - `88dac97` (feat)
3. **Task 3: Integrate console and metrics into server detail page with tabbed layout** - `ba055dd` (feat)

## Files Created/Modified
- `web/src/hooks/use-console.ts` - Custom WebSocket hook with JWT auth, reconnection, command sending
- `web/src/hooks/use-metrics.ts` - useQuery polling hook for server metrics endpoint
- `web/src/components/console/terminal.tsx` - xterm.js terminal with FitAddon and ResizeObserver
- `web/src/components/console/console-panel.tsx` - Console panel with status indicator and command input
- `web/src/components/servers/metrics-panel.tsx` - CPU/memory progress bars with graceful error handling
- `web/src/components/ui/tabs.tsx` - shadcn Tabs component
- `web/src/types/api.ts` - Added MetricsResponse interface
- `web/src/api/servers.ts` - Added getServerMetrics API function
- `web/src/pages/server-detail.tsx` - Refactored with tabbed layout for Overview/Console/Resources
- `web/package.json` - Added @xterm/xterm and @xterm/addon-fit dependencies

## Decisions Made
- Native WebSocket API used instead of react-use-websocket library to avoid React 19 peer dependency conflict
- HTML input field for command entry rather than xterm onData, providing a cleaner separated UX
- Metrics unavailability displayed as muted placeholder text rather than error toast, since metrics-server may not be installed in all clusters

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Phase 7 complete: both backend (07-01) and frontend (07-02) console and metrics features shipped
- Ready for Phase 8 (Helm packaging) which will bundle the full application stack
- All frontend components build successfully with Vite production build

## Self-Check: PASSED

All 9 key files verified present. All 3 task commit hashes verified in git log.

---
*Phase: 07-console-realtime*
*Completed: 2026-02-12*
