# Phase 7: Console & Real-time Features - Research

**Researched:** 2026-02-11
**Domain:** WebSocket communication, Kubernetes pod log streaming, pod exec, metrics API
**Confidence:** HIGH

## Summary

Phase 7 requires three real-time features: (1) live console output from game server pods, (2) command input to running game servers, and (3) CPU/RAM/disk usage monitoring. All three involve bridging the browser to Kubernetes APIs through the existing Chi v5 HTTP server.

The architecture centers on a WebSocket endpoint per server that proxies Kubernetes pod log streams (for console output) and pod exec (for command input) through the Go API server. Resource metrics come from the Kubernetes Metrics API (`metrics.k8s.io/v1beta1`), which requires `metrics-server` to be installed in the cluster. The frontend will use a custom WebSocket hook (native browser WebSocket API, not `react-use-websocket` due to React 19 peer dependency issues) and `@xterm/xterm` for terminal rendering.

**Primary recommendation:** Use `gorilla/websocket` v1.5.3 for the Go WebSocket server, Kubernetes `client-go` pod log streaming with Follow=true for console output, `remotecommand` exec for command input, and `k8s.io/metrics` client for resource usage. Authenticate WebSocket connections via JWT token in query parameter during upgrade.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `gorilla/websocket` | v1.5.3 | Go WebSocket server | 42K+ importers, battle-tested, stable API, works seamlessly with Chi/net/http |
| `k8s.io/client-go` | v0.35.0 (match existing) | Pod logs streaming, exec | Official K8s Go client; provides GetLogs().Stream() and remotecommand |
| `k8s.io/metrics` | v0.35.0 (match existing) | Pod CPU/memory metrics | Official typed client for metrics.k8s.io API |
| `@xterm/xterm` | ^6.0.0 | Terminal rendering in browser | Standard web terminal component, 20K+ GitHub stars, active maintenance |
| `@xterm/addon-fit` | ^0.11.0 | Auto-resize terminal | Official addon for responsive terminal sizing |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `@xterm/addon-search` | ^0.16.0 | Terminal text search | Optional: searching console output history |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `gorilla/websocket` | `github.com/coder/websocket` | coder/websocket is more idiomatic with context support, but gorilla has vastly more ecosystem support and examples |
| `react-use-websocket` | Custom hook with native WebSocket | react-use-websocket v4.13.0 has React 18 peer dep, not React 19; custom hook is straightforward for our needs |
| `@xterm/xterm` | Plain `<pre>` with auto-scroll | xterm.js handles ANSI colors, scrollback, search, selection -- significant UX advantage for game consoles |

**Installation (Go):**
```bash
go get github.com/gorilla/websocket@v1.5.3
go get k8s.io/metrics@v0.35.0
```

**Installation (Frontend):**
```bash
npm install @xterm/xterm @xterm/addon-fit
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
  api/
    handlers_console.go     # WebSocket handler for console I/O
    handlers_metrics.go     # REST endpoint for resource metrics
    console_hub.go          # Connection management (hub pattern)
  controller/
    gameserver_controller.go  # Existing (add pods/log, pods/exec RBAC)
web/src/
  hooks/
    use-console.ts          # Custom WebSocket hook for console
    use-metrics.ts          # Polling hook for resource metrics
  components/
    console/
      terminal.tsx          # xterm.js terminal component
      console-panel.tsx     # Console panel with input/output
  pages/
    server-detail.tsx       # Extended with console tab
```

### Pattern 1: WebSocket Console Proxy (Hub Pattern)
**What:** Single WebSocket endpoint per server that multiplexes log output (downstream) and command input (upstream). The Go API server acts as a proxy between the browser WebSocket and Kubernetes pod APIs.
**When to use:** Always -- this is the core console architecture.

**Flow:**
```
Browser <--WebSocket--> API Server <--K8s API--> Pod
  |                        |
  | ws://host/api/v1/      | 1. GetLogs(Follow:true) -> stream output
  |   gameservers/{name}/  | 2. Exec(stdin) -> pipe commands
  |   console?token=JWT    |
```

**Go handler example:**
```go
// Source: gorilla/websocket docs + K8s client-go patterns
var upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
    CheckOrigin: func(r *http.Request) bool {
        // In production, validate against allowed origins
        // For same-origin SPA served via go:embed, origin matches
        return true
    },
}

func (s *Server) handleConsole(w http.ResponseWriter, r *http.Request) {
    // 1. Authenticate via query param (WebSocket can't send headers)
    token := r.URL.Query().Get("token")
    claims, err := s.jwtService.ValidateToken(token)
    if err != nil {
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }

    // 2. Verify server ownership
    name := chi.URLParam(r, "name")
    gs := &gamev1alpha1.GameServer{}
    if err := s.client.Get(r.Context(), client.ObjectKey{
        Name: name, Namespace: claims.Namespace,
    }, gs); err != nil {
        http.Error(w, "not found", http.StatusNotFound)
        return
    }

    // 3. Verify server is running
    if gs.Status.State != gamev1alpha1.GameServerStateReady &&
       gs.Status.State != gamev1alpha1.GameServerStateAllocated {
        http.Error(w, "server not running", http.StatusConflict)
        return
    }

    // 4. Upgrade to WebSocket
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }
    defer conn.Close()

    // 5. Start log streaming in goroutine, read commands from WS
    ctx, cancel := context.WithCancel(r.Context())
    defer cancel()

    go s.streamLogs(ctx, conn, gs)
    s.readCommands(ctx, conn, gs)
}
```

### Pattern 2: Pod Log Streaming via client-go
**What:** Use `kubernetes.Clientset` (created from the same `rest.Config` the manager uses) to call `GetLogs()` with `Follow: true`, then pipe the `io.ReadCloser` to WebSocket messages.
**When to use:** Console output streaming.

**Key detail:** The controller-runtime `client.Client` does NOT support pod log streaming. We need a `kubernetes.Clientset` created from `ctrl.GetConfigOrDie()`.

```go
// Source: k8s.io/client-go pod log streaming pattern
func (s *Server) streamLogs(ctx context.Context, conn *websocket.Conn, gs *gamev1alpha1.GameServer) {
    req := s.clientset.CoreV1().Pods(gs.Namespace).GetLogs(gs.Name, &corev1.PodLogOptions{
        Container: "gameserver",
        Follow:    true,
        TailLines: int64Ptr(100), // Start with last 100 lines
    })

    stream, err := req.Stream(ctx)
    if err != nil {
        conn.WriteJSON(map[string]string{"error": err.Error()})
        return
    }
    defer stream.Close()

    buf := make([]byte, 4096)
    for {
        n, err := stream.Read(buf)
        if n > 0 {
            if writeErr := conn.WriteMessage(websocket.TextMessage, buf[:n]); writeErr != nil {
                return
            }
        }
        if err != nil {
            if err == io.EOF {
                conn.WriteJSON(map[string]string{"event": "stream_ended"})
            }
            return
        }
    }
}
```

### Pattern 3: Command Execution via remotecommand
**What:** Use `client-go/tools/remotecommand` to exec a command in the pod's container, piping stdin from the WebSocket message.
**When to use:** Sending console commands to a running game server.

**Important:** Game servers that read from stdin (like Minecraft) can receive commands via exec. The command is passed as stdin to the running process.

```go
// Source: k8s.io/client-go/tools/remotecommand
func (s *Server) execCommand(ctx context.Context, gs *gamev1alpha1.GameServer, command string) error {
    req := s.clientset.CoreV1().RESTClient().Post().
        Resource("pods").
        Name(gs.Name).
        Namespace(gs.Namespace).
        SubResource("exec").
        VersionedParams(&corev1.PodExecOptions{
            Container: "gameserver",
            Command:   []string{"/bin/sh", "-c", fmt.Sprintf("echo '%s' > /proc/1/fd/0", command)},
            Stdin:     false,
            Stdout:    true,
            Stderr:    true,
        }, scheme.ParameterCodec)

    exec, err := remotecommand.NewSPDYExecutor(s.restConfig, "POST", req.URL())
    if err != nil {
        return err
    }

    var stdout, stderr bytes.Buffer
    return exec.StreamWithContext(ctx, remotecommand.StreamOptions{
        Stdout: &stdout,
        Stderr: &stderr,
    })
}
```

**Note:** The exact command execution method depends on the game server type. Some game servers:
- Read from stdin directly (Minecraft Java docker image with `CREATE_CONSOLE_IN_PIPE=true`)
- Use RCON protocol (Minecraft, many Source engine games)
- Have no console input support

For Phase 7, implement the stdin-pipe approach as the default, since it works with the most game server containers. RCON support can be added later as a game-manifest-level configuration.

### Pattern 4: Resource Metrics via Metrics API
**What:** Poll the Kubernetes Metrics API for pod CPU/memory usage. Disk usage comes from kubelet stats or PVC inspection.
**When to use:** Resource usage display.

```go
// Source: k8s.io/metrics client pattern
func (s *Server) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
    ns := namespaceFromContext(r)
    name := chi.URLParam(r, "name")

    podMetrics, err := s.metricsClient.MetricsV1beta1().
        PodMetricses(ns).
        Get(r.Context(), name, metav1.GetOptions{})
    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to get metrics")
        return
    }

    if len(podMetrics.Containers) == 0 {
        respondError(w, http.StatusNotFound, "no container metrics")
        return
    }

    container := podMetrics.Containers[0] // "gameserver" container
    respondJSON(w, http.StatusOK, MetricsResponse{
        CPU:    container.Usage.Cpu().MilliValue(),
        Memory: container.Usage.Memory().Value() / (1024 * 1024), // MiB
    })
}
```

### Pattern 5: WebSocket Authentication via Query Parameter
**What:** Since WebSocket connections cannot send custom headers during the upgrade handshake from browser JavaScript, pass the JWT token as a query parameter.
**When to use:** Always for browser-initiated WebSocket connections.

```typescript
// Frontend: connect with token
const token = useAuthStore.getState().token;
const ws = new WebSocket(
  `${wsBaseUrl}/api/v1/gameservers/${name}/console?token=${token}`
);
```

**Security considerations:**
- Token appears in server access logs -- configure logging to exclude query params for WS routes
- Token appears in browser history -- mitigated by SPA (no navigation)
- Use WSS (TLS) in production to prevent token interception
- Short-lived tokens (already 24h) limit exposure window

### Anti-Patterns to Avoid
- **Polling for console output:** Never poll a REST endpoint for log lines. WebSocket streaming is mandatory for real-time console UX.
- **Storing full log history in Go memory:** Stream logs through the WebSocket, let xterm.js handle scrollback buffer. Do not accumulate logs server-side.
- **Using controller-runtime client for logs/exec:** The `client.Client` interface does not support pod log streaming or exec subresources. Always use `kubernetes.Clientset` for these operations.
- **Sharing a single WebSocket for all servers:** Each server console should have its own WebSocket connection. Multiplexing adds complexity with no benefit (users view one console at a time).
- **Forgetting to close K8s streams on WebSocket disconnect:** Always wire WebSocket close to cancel the context that controls the log stream and exec operations.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| WebSocket protocol handling | Custom WebSocket frame parsing | `gorilla/websocket` | Protocol is complex (frames, masking, ping/pong, close handshake) |
| Terminal rendering with ANSI | Custom ANSI parser + DOM renderer | `@xterm/xterm` | ANSI escape codes have 100+ sequences; xterm.js handles all of them |
| Pod log streaming | Custom HTTP long-poll or API scraping | `client-go GetLogs(Follow:true)` | K8s log API handles pod restarts, container selection, tail lines |
| Pod exec/attach | Raw HTTP upgrade to K8s API | `client-go remotecommand` | SPDY/WebSocket protocol negotiation is version-dependent |
| WebSocket reconnection | Manual retry with setTimeout | Custom hook with exponential backoff | Need connection state tracking, cleanup, reconnect on visibility change |
| Metrics collection | Scraping kubelet directly | `k8s.io/metrics` typed client | Metrics API is the standard interface, metrics-server aggregates from kubelets |

**Key insight:** Every feature in this phase involves bridging two different streaming/real-time protocols (browser WebSocket <-> Kubernetes streaming APIs). The libraries exist to handle each side; the implementation work is in the proxy layer connecting them.

## Common Pitfalls

### Pitfall 1: WebSocket Connection Blocked by Middleware Timeout
**What goes wrong:** The existing Chi middleware stack includes `middleware.Timeout(30 * time.Second)`. This will kill WebSocket connections after 30 seconds.
**Why it happens:** Middleware.Timeout wraps the handler in a context with a deadline, which cancels the WebSocket connection.
**How to avoid:** Register the WebSocket route BEFORE the timeout middleware, or use a route group without the timeout middleware for WebSocket endpoints.
**Warning signs:** WebSocket connections drop exactly at 30 seconds.

### Pitfall 2: Goroutine Leak from Unclosed Log Streams
**What goes wrong:** When a WebSocket client disconnects, the goroutine reading from the K8s log stream continues running forever because nothing signals it to stop.
**Why it happens:** The log stream's `Read()` blocks waiting for new data. Without context cancellation, it never returns.
**How to avoid:** Use a shared `context.WithCancel()`. Cancel the context when: (a) WebSocket read returns an error, (b) WebSocket close message received, (c) server shutdown. Wire pong handler to detect stale connections.
**Warning signs:** Memory/goroutine count grows over time; `runtime.NumGoroutine()` increases without decreasing.

### Pitfall 3: Race Condition on WebSocket Write
**What goes wrong:** Multiple goroutines writing to the same WebSocket connection simultaneously causes panics. gorilla/websocket connections support one concurrent reader and one concurrent writer but NOT multiple concurrent writers.
**Why it happens:** Log streaming goroutine and command response both write to the same `*websocket.Conn`.
**How to avoid:** Use a write mutex (`sync.Mutex`) or a dedicated write goroutine with a channel. The channel approach is cleaner:
```go
writeCh := make(chan []byte, 256)
go func() {
    for msg := range writeCh {
        conn.WriteMessage(websocket.TextMessage, msg)
    }
}()
```
**Warning signs:** Intermittent panics with "concurrent write to websocket connection".

### Pitfall 4: CORS/Origin Issues with WebSocket Upgrade
**What goes wrong:** WebSocket upgrade fails with 403 because `gorilla/websocket`'s default `CheckOrigin` rejects cross-origin requests.
**Why it happens:** Default CheckOrigin returns false if Origin header doesn't match Host header. During development with Vite proxy, Origin may differ.
**How to avoid:** Since the SPA is served from the same origin via go:embed, CheckOrigin can return true (same-origin policy already protects). In development, Vite proxy handles this.
**Warning signs:** WebSocket upgrade fails with HTTP 403.

### Pitfall 5: Pod Not Found After Server Restart
**What goes wrong:** WebSocket connects, but GetLogs fails because the pod doesn't exist yet (server is in Creating/Starting state).
**Why it happens:** User opens console while server is starting; pod may not exist or may not be Running.
**How to avoid:** Check GameServer state before allowing WebSocket upgrade. Only allow console connections when state is Ready or Allocated. Return clear error message for other states.
**Warning signs:** "pod not found" errors in console handler.

### Pitfall 6: Metrics Server Not Installed
**What goes wrong:** Metrics API calls fail with "the server could not find the requested resource" because metrics-server is not deployed.
**Why it happens:** `metrics-server` is not installed by default on all K8s distributions (though Talos includes it).
**How to avoid:** Handle metrics API errors gracefully. Show "metrics unavailable" in UI rather than error. Check for metrics-server availability at startup and log a warning.
**Warning signs:** 404 responses from metrics API.

### Pitfall 7: Disk Usage Not Available via Metrics API
**What goes wrong:** CPU and memory metrics work, but disk usage is not returned by the Metrics API.
**Why it happens:** The Kubernetes Metrics API (metrics.k8s.io) only exposes CPU and memory. Disk/storage usage is exposed via kubelet stats summary API or Prometheus metrics, not the standard Metrics API.
**How to avoid:** For disk usage, either: (a) query kubelet stats summary API directly, (b) exec `df` in the container, or (c) use Prometheus with `kubelet_volume_stats_*` metrics. For Phase 7 MVP, showing CPU and memory is sufficient; disk usage can use exec-based `df` as a simpler approach.
**Warning signs:** No disk field in PodMetrics response.

## Code Examples

### WebSocket Route Registration (avoiding middleware timeout)
```go
// Source: Chi v5 route groups + gorilla/websocket pattern
func (s *Server) routes() chi.Router {
    r := chi.NewRouter()

    // Global middleware
    r.Use(middleware.RequestID)
    r.Use(middleware.RealIP)
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)
    // NOTE: Do NOT apply middleware.Timeout globally if WebSocket routes exist

    r.Use(cors.Handler(cors.Options{...}))
    r.Use(httprate.LimitByIP(100, time.Minute))

    // Health routes
    r.Get("/healthz", handleHealthz)
    r.Get("/readyz", handleReadyz)

    // REST API routes WITH timeout
    r.Route("/api/v1", func(r chi.Router) {
        // Apply timeout only to REST routes
        r.Use(middleware.Timeout(30 * time.Second))
        r.Use(s.authMiddleware.Authenticate)
        // ... existing routes ...

        r.Route("/gameservers", func(r chi.Router) {
            // ... existing CRUD routes ...
            r.Route("/{name}", func(r chi.Router) {
                // ... existing handlers ...
                r.Get("/metrics", s.handleGetMetrics)
            })
        })
    })

    // WebSocket routes WITHOUT timeout (separate group)
    r.Route("/api/v1/gameservers/{name}/console", func(r chi.Router) {
        // Auth handled in handler via query param
        // No timeout middleware -- connections are long-lived
        r.Get("/", s.handleConsole)
    })

    r.NotFound(serveSPA().ServeHTTP)
    return r
}
```

### Custom React WebSocket Hook
```typescript
// Source: Native WebSocket API + React patterns
// Not using react-use-websocket due to React 19 peer dep conflict

import { useEffect, useRef, useCallback, useState } from 'react';
import { useAuthStore } from '@/stores/auth-store';

type ConnectionStatus = 'connecting' | 'connected' | 'disconnected' | 'error';

interface UseConsoleOptions {
  serverName: string;
  onMessage: (data: string) => void;
  enabled?: boolean;
}

export function useConsole({ serverName, onMessage, enabled = true }: UseConsoleOptions) {
  const wsRef = useRef<WebSocket | null>(null);
  const [status, setStatus] = useState<ConnectionStatus>('disconnected');
  const reconnectTimeoutRef = useRef<number>();
  const reconnectCountRef = useRef(0);
  const MAX_RECONNECTS = 5;

  const connect = useCallback(() => {
    const token = useAuthStore.getState().token;
    if (!token || !enabled) return;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${protocol}//${window.location.host}/api/v1/gameservers/${serverName}/console?token=${token}`;

    setStatus('connecting');
    const ws = new WebSocket(url);

    ws.onopen = () => {
      setStatus('connected');
      reconnectCountRef.current = 0;
    };

    ws.onmessage = (event) => {
      onMessage(event.data);
    };

    ws.onclose = (event) => {
      setStatus('disconnected');
      wsRef.current = null;

      // Reconnect with exponential backoff (unless intentional close)
      if (event.code !== 1000 && reconnectCountRef.current < MAX_RECONNECTS) {
        const delay = Math.min(1000 * Math.pow(2, reconnectCountRef.current), 30000);
        reconnectCountRef.current++;
        reconnectTimeoutRef.current = window.setTimeout(connect, delay);
      }
    };

    ws.onerror = () => {
      setStatus('error');
    };

    wsRef.current = ws;
  }, [serverName, onMessage, enabled]);

  const sendCommand = useCallback((command: string) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: 'command', data: command }));
    }
  }, []);

  const disconnect = useCallback(() => {
    clearTimeout(reconnectTimeoutRef.current);
    reconnectCountRef.current = MAX_RECONNECTS; // prevent reconnect
    wsRef.current?.close(1000);
  }, []);

  useEffect(() => {
    if (enabled) connect();
    return () => {
      clearTimeout(reconnectTimeoutRef.current);
      wsRef.current?.close(1000);
    };
  }, [connect, enabled]);

  return { status, sendCommand, disconnect };
}
```

### xterm.js Terminal Component
```tsx
// Source: @xterm/xterm docs + React integration pattern
import { useEffect, useRef } from 'react';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';

interface TerminalPanelProps {
  onData?: (data: string) => void;
}

export function TerminalPanel({ onData }: TerminalPanelProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const terminalRef = useRef<Terminal | null>(null);

  useEffect(() => {
    if (!containerRef.current) return;

    const terminal = new Terminal({
      cursorBlink: true,
      fontSize: 14,
      fontFamily: 'JetBrains Mono, Fira Code, monospace',
      theme: {
        background: '#1a1b26',
        foreground: '#a9b1d6',
      },
      scrollback: 5000,
      convertEol: true, // Convert \n to \r\n for proper line breaks
    });

    const fitAddon = new FitAddon();
    terminal.loadAddon(fitAddon);
    terminal.open(containerRef.current);
    fitAddon.fit();

    // Handle user input (command typing)
    if (onData) {
      terminal.onData(onData);
    }

    // Resize on window resize
    const observer = new ResizeObserver(() => fitAddon.fit());
    observer.observe(containerRef.current);

    terminalRef.current = terminal;

    return () => {
      observer.disconnect();
      terminal.dispose();
    };
  }, [onData]);

  return <div ref={containerRef} className="h-full w-full" />;
}

// To write data to terminal from parent:
// terminalRef.current?.write(data);
```

### RBAC Markers for Pod Logs and Exec
```go
// Source: Kubernetes RBAC documentation
// Add to gameserver_controller.go or a new console_controller.go:

// +kubebuilder:rbac:groups="",resources=pods/log,verbs=get
// +kubebuilder:rbac:groups="",resources=pods/exec,verbs=create
```

### Metrics Response Type
```go
// Source: k8s.io/metrics API response structure
type MetricsResponse struct {
    CPU       int64  `json:"cpu"`       // millicores (e.g., 500 = 0.5 CPU)
    MemoryMiB int64  `json:"memoryMiB"` // MiB
    DiskMiB   *int64 `json:"diskMiB,omitempty"` // MiB, nil if unavailable
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| SPDY for exec/attach | WebSocket (K8s 1.31+, beta) | K8s 1.29-1.31 | `remotecommand` package handles fallback automatically |
| `xterm` (npm) | `@xterm/xterm` (scoped) | xterm.js v5+ | Import path changed; old `xterm` package is deprecated |
| `react-use-websocket` | Custom hook (for React 19) | React 19 (Dec 2024) | Library has React 18 peer dep; custom hook avoids dep conflict |
| k8s.io/metrics v1beta1 | Still v1beta1 | Stable for years | No v1 GA yet; v1beta1 is the standard |

**Deprecated/outdated:**
- `xterm` npm package: Use `@xterm/xterm` instead (scoped package)
- SPDY-only exec: `remotecommand` now prefers WebSocket with SPDY fallback
- `nhooyr.io/websocket`: Moved to `github.com/coder/websocket`

## Open Questions

1. **Game server stdin support variability**
   - What we know: Minecraft Docker images support stdin via `CREATE_CONSOLE_IN_PIPE=true`. Many game servers have stdin support.
   - What's unclear: Not all game server images accept stdin. Some require RCON or custom protocols.
   - Recommendation: Implement stdin-pipe as default. Add an optional `consoleType` field to game manifests later (stdin, rcon, none). For Phase 7 MVP, stdin is sufficient.

2. **Disk usage metrics source**
   - What we know: Kubernetes Metrics API only exposes CPU and memory. Disk usage requires either kubelet stats API, exec-based `df`, or Prometheus.
   - What's unclear: Whether the Talos cluster exposes kubelet stats summary API.
   - Recommendation: For Phase 7 MVP, show CPU and memory from Metrics API. Add disk usage via exec-based `df -h /data` as a separate REST endpoint (not real-time). Mark disk as "optional" in the UI if unavailable.

3. **metrics-server availability on Talos**
   - What we know: Talos v1.11.2 should include metrics-server by default.
   - What's unclear: Whether it's configured and accessible in the specific cluster.
   - Recommendation: Add a startup check that tests metrics API availability. Log a warning if unavailable. UI shows "metrics unavailable" gracefully.

4. **WebSocket through Cloudflare Tunnel**
   - What we know: Cloudflare Tunnel supports WebSocket connections.
   - What's unclear: Whether there are timeout or message size limits that could affect long-lived console connections.
   - Recommendation: Implement ping/pong keepalive (gorilla/websocket supports this natively). Set ping interval to 30 seconds to keep the connection alive through any proxy.

## Dependency Changes

### New Go dependencies
```
github.com/gorilla/websocket v1.5.3
k8s.io/metrics v0.35.0  (new)
```

### New RBAC requirements
```
pods/log   - get      (pod log streaming)
pods/exec  - create   (command execution)
```

### New frontend dependencies
```
@xterm/xterm      ^6.0.0
@xterm/addon-fit  ^0.11.0
```

### Config changes
- `kubernetes.Clientset` must be created alongside controller-runtime `client.Client` in `cmd/main.go`
- `rest.Config` must be passed to the API server for creating exec executors
- `metricsv1beta1.MetricsV1beta1Client` must be created and passed to the API server

## API Design

### New Endpoints
| Method | Path | Auth | Type | Purpose |
|--------|------|------|------|---------|
| GET | `/api/v1/gameservers/{name}/console` | JWT query param | WebSocket | Console I/O stream |
| GET | `/api/v1/gameservers/{name}/metrics` | JWT header | REST | Current resource usage |

### WebSocket Message Format
```typescript
// Client -> Server (commands)
{ "type": "command", "data": "say Hello World" }
{ "type": "resize", "cols": 120, "rows": 40 }

// Server -> Client (output)
// Plain text messages (log output) -- no JSON wrapper for performance
// Or JSON for control messages:
{ "type": "error", "data": "pod not found" }
{ "type": "connected", "data": "streaming logs..." }
{ "type": "stream_ended" }
```

### Metrics Response
```json
{
  "cpu": 250,
  "memoryMiB": 1024,
  "cpuLimit": 2000,
  "memoryLimitMiB": 4096,
  "diskMiB": null
}
```

## Sources

### Primary (HIGH confidence)
- [gorilla/websocket GitHub](https://github.com/gorilla/websocket) - v1.5.3 release, API stability
- [k8s.io/client-go/tools/remotecommand](https://pkg.go.dev/k8s.io/client-go/tools/remotecommand) - Executor API, WebSocket/SPDY support
- [k8s.io/metrics v1beta1 client](https://pkg.go.dev/k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1) - PodMetrics typed client
- [@xterm/xterm npm](https://www.npmjs.com/package/@xterm/xterm) - v6.0.0 latest
- [Kubernetes SPDY to WebSocket transition blog](https://kubernetes.io/blog/2024/08/20/websockets-transition/) - Beta in K8s 1.31

### Secondary (MEDIUM confidence)
- [controller-runtime issue #611](https://github.com/kubernetes-sigs/controller-runtime/issues/611) - Confirmed client.Client does not support pod logs
- [react-use-websocket issue #256](https://github.com/robtaussig/react-use-websocket/issues/256) - React 19 compatibility discussion
- [Kubernetes RBAC for logs/exec](https://medium.com/@ManagedKube/kubernetes-rbac-giving-permissions-for-logging-and-port-forwarding-882694c91927)

### Tertiary (LOW confidence)
- Disk usage approach via kubelet stats -- needs validation on Talos cluster
- Cloudflare Tunnel WebSocket timeout limits -- needs testing

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - gorilla/websocket and client-go are well-established, verified via official docs
- Architecture: HIGH - WebSocket proxy pattern is standard for K8s-backed web consoles
- Console I/O (logs+exec): HIGH - Verified via client-go docs and K8s API documentation
- Metrics API: MEDIUM - v1beta1 is standard but availability depends on cluster config
- Disk usage: LOW - No standard API; approach needs validation
- Frontend terminal: HIGH - xterm.js is the de facto standard for web terminals

**Research date:** 2026-02-11
**Valid until:** 2026-04-11 (90 days - these are stable technologies)
