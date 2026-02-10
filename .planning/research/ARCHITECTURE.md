# Architecture Research

**Domain:** Kubernetes-native game server management panel
**Researched:** 2026-02-09
**Confidence:** HIGH

## Standard Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                          USER INTERFACE LAYER                        │
├─────────────────────────────────────────────────────────────────────┤
│  ┌────────────────────────────────────────────────────────────┐     │
│  │  React/Next.js Frontend (Web UI)                           │     │
│  │  - Dynamic forms driven by game parameter manifests        │     │
│  │  - Real-time server status monitoring                      │     │
│  │  - User management and authentication                      │     │
│  └────────────┬───────────────────────────────────────────────┘     │
│               │ HTTP/REST                                            │
├───────────────┴──────────────────────────────────────────────────────┤
│                        API SERVER LAYER                              │
├─────────────────────────────────────────────────────────────────────┤
│  ┌────────────────────────────────────────────────────────────┐     │
│  │  Go REST API Server                                        │     │
│  │  - Authentication & Authorization                          │     │
│  │  - Game server CRUD operations                             │     │
│  │  - Backup management (on-demand + scheduled)               │     │
│  │  - User/tenant management                                  │     │
│  │  - Metrics aggregation & queries                           │     │
│  └──────┬──────────────────────────┬──────────────────────────┘     │
│         │ Kubernetes Client API     │ Direct database                │
├─────────┴───────────────────────────┴──────────────────────────────┤
│                        OPERATOR LAYER                                │
├─────────────────────────────────────────────────────────────────────┤
│  ┌───────────────────┐  ┌────────────────────┐  ┌──────────────┐   │
│  │ GameServer        │  │ Backup             │  │ DNS          │   │
│  │ Controller        │  │ Controller         │  │ Controller   │   │
│  │ - Reconciliation  │  │ - S3 operations    │  │ - Ingress/   │   │
│  │ - State machine   │  │ - CronJob mgmt     │  │   HTTPRoute  │   │
│  │ - Pod lifecycle   │  │ - Restore logic    │  │ - ExternalDNS│   │
│  └──────┬────────────┘  └──────┬─────────────┘  └──────┬───────┘   │
│         │                      │                        │           │
│         │  Watch/Update CRDs   │                        │           │
├─────────┴──────────────────────┴────────────────────────┴───────────┤
│                     KUBERNETES API SERVER                            │
├─────────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────┐               │
│  │ GameServer  │  │ Backup       │  │ Ingress/     │               │
│  │ CRD         │  │ CRD          │  │ HTTPRoute    │               │
│  └─────────────┘  └──────────────┘  └──────────────┘               │
├─────────────────────────────────────────────────────────────────────┤
│                      INFRASTRUCTURE LAYER                            │
├─────────────────────────────────────────────────────────────────────┤
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐            │
│  │ Game     │  │ Ingress  │  │ Metrics  │  │ Backup   │            │
│  │ Server   │  │ Control- │  │ Export-  │  │ Storage  │            │
│  │ Pods     │  │ ler      │  │ ers      │  │ (S3)     │            │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘            │
└─────────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Typical Implementation |
|-----------|----------------|------------------------|
| **Frontend** | User interface for managing game servers, dynamic forms, real-time monitoring | React 18+/Next.js with React Hook Form + Zod, JSON schema-driven forms, SSR/RSC patterns |
| **API Server** | REST API, authentication, business logic, operator interaction gateway | Go HTTP server (net/http or Gin/Echo), JWT auth, controller-runtime client for K8s interaction |
| **GameServer Controller** | Reconciles GameServer CRDs, manages Pod lifecycle, implements state machine (Ready→Allocated→Shutdown) | Go operator using controller-runtime/Kubebuilder, separate controller per CRD |
| **Backup Controller** | Manages scheduled/on-demand backups to S3, creates restore operations, manages CronJobs | Go operator, Velero-inspired pattern, finalizers for cleanup |
| **DNS Controller** | Creates Ingress/HTTPRoute resources, manages wildcard subdomain routing (game.username.domain.com) | Integrates with ExternalDNS or manages routes directly, Gateway API (HTTPRoute) recommended for 2026+ |
| **Metrics Pipeline** | Collects operator metrics, game server stats, exposes Prometheus endpoints | controller-runtime metrics + custom metrics, ServiceMonitor CRDs for Prometheus Operator |
| **Game Definition Framework** | Declarative game configs (Dockerfile + parameter manifest per game) | Folder structure with JSON/YAML manifests, drives dynamic form generation |
| **Helm Chart** | Packages entire system, manages CRD installation, provides configuration flexibility | Helm 3 chart with CRDs in crds/ directory, hooks for operator deployment, values for Ingress vs HTTPRoute toggle |

## Recommended Project Structure

**Recommendation: MONOREPO** (following Agones pattern)

Monorepo advantages for this use case:
- Shared Go types between operator and API server
- Unified build and release pipeline
- Consistent versioning across components
- Easier local development (single clone, one module)
- Game definition framework lives alongside code that consumes it

```
kterodactyl/
├── cmd/
│   ├── operator/           # Operator entrypoint
│   │   └── main.go
│   ├── apiserver/          # API server entrypoint
│   │   └── main.go
│   └── cli/                # Optional CLI tool
│       └── main.go
├── pkg/
│   ├── apis/               # CRD API definitions
│   │   └── v1alpha1/
│   │       ├── gameserver_types.go
│   │       ├── backup_types.go
│   │       └── zz_generated.deepcopy.go
│   ├── controllers/        # Operator controllers
│   │   ├── gameserver_controller.go
│   │   ├── backup_controller.go
│   │   └── dns_controller.go
│   ├── apiserver/          # API server handlers
│   │   ├── handlers/
│   │   ├── middleware/
│   │   └── routes.go
│   ├── reconciler/         # Shared reconciliation logic
│   │   ├── lifecycle.go    # State machine
│   │   └── predicates.go   # Event filters
│   └── util/               # Shared utilities
│       ├── k8s.go
│       └── s3.go
├── frontend/               # React/Next.js application
│   ├── app/                # App Router (Next.js 13+)
│   ├── components/
│   ├── lib/
│   │   ├── api-client.ts
│   │   └── form-schema.ts  # JSON schema types
│   └── package.json
├── config/
│   ├── crd/                # Generated CRD manifests
│   │   └── bases/
│   ├── rbac/               # RBAC manifests
│   ├── manager/            # Operator deployment
│   └── samples/            # Example CRs
├── helm/
│   └── kterodactyl/        # Helm chart
│       ├── Chart.yaml
│       ├── values.yaml
│       ├── crds/           # CRD definitions (installed first)
│       ├── templates/
│       │   ├── operator.yaml
│       │   ├── apiserver.yaml
│       │   ├── ingress.yaml
│       │   └── hooks/      # Pre-install/post-install
│       └── README.md
├── games/                  # Game definition framework
│   ├── minecraft/
│   │   ├── Dockerfile
│   │   └── manifest.yaml   # Parameters, ports, env vars
│   ├── valheim/
│   │   ├── Dockerfile
│   │   └── manifest.yaml
│   └── README.md
├── build/
│   ├── Dockerfile.operator # Multi-stage build
│   ├── Dockerfile.apiserver
│   └── .dockerignore
├── hack/                   # Build/dev scripts
│   ├── install-tools.sh
│   └── update-codegen.sh
├── docs/                   # Documentation
├── Makefile                # Build automation
├── go.mod
├── go.sum
└── PROJECT                 # Kubebuilder project file
```

### Structure Rationale

- **cmd/**: Separate entrypoints allow building distinct binaries while sharing code in pkg/
- **pkg/apis/**: Kubebuilder standard, generated code lives here alongside API types
- **pkg/controllers/**: One controller per CRD (best practice from Kubebuilder docs)
- **frontend/**: Colocated for unified releases, uses Next.js App Router for React Server Components
- **games/**: Declarative game definitions enable adding new games without code changes
- **helm/**: Single chart deploys entire stack, CRDs in crds/ directory per Helm 3 best practices
- **build/**: Multi-stage Dockerfiles for minimal distroless images

## Architectural Patterns

### Pattern 1: Operator Reconciliation Loop (Control Theory)

**What:** Operators continuously watch CRD state, compare to actual cluster state, and take action to converge them. This is level-based (desired state) not edge-based (events).

**When to use:** For all CRD management. The reconciliation pattern is the foundation of Kubernetes operators.

**Trade-offs:**
- Pro: Self-healing, eventual consistency, resilient to transient failures
- Con: Reconciliation can be slow (rate limited), requires idempotent logic

**Example:**
```go
func (r *GameServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // 1. Fetch the GameServer CR
    gameServer := &v1alpha1.GameServer{}
    if err := r.Get(ctx, req.NamespacedName, gameServer); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    // 2. Check desired vs actual state
    pod := &corev1.Pod{}
    err := r.Get(ctx, types.NamespacedName{
        Name: gameServer.Name,
        Namespace: gameServer.Namespace,
    }, pod)

    // 3. Converge: create missing resources
    if errors.IsNotFound(err) {
        pod := r.constructPodForGameServer(gameServer)
        if err := r.Create(ctx, pod); err != nil {
            return ctrl.Result{}, err
        }
        return ctrl.Result{Requeue: true}, nil
    }

    // 4. Update status to reflect reality
    if pod.Status.Phase == corev1.PodRunning && gameServer.Status.State != "Ready" {
        gameServer.Status.State = "Ready"
        return ctrl.Result{}, r.Status().Update(ctx, gameServer)
    }

    return ctrl.Result{}, nil
}
```

### Pattern 2: Event Filtering with Predicates

**What:** Filter watch events before they trigger reconciliation to reduce API load and improve performance. Only reconcile when meaningful changes occur (e.g., spec.generation changes, not every status update).

**When to use:** On all controllers. Aggressively filter events to prevent unnecessary reconciliations.

**Trade-offs:**
- Pro: Dramatically reduces CPU/API load (40%+ savings reported in production)
- Con: Requires careful predicate design to avoid missing important events

**Example:**
```go
func (r *GameServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&v1alpha1.GameServer{}).
        Owns(&corev1.Pod{}).
        WithEventFilter(predicate.Funcs{
            UpdateFunc: func(e event.UpdateEvent) bool {
                // Only reconcile on spec changes, not status updates
                oldGen := e.ObjectOld.GetGeneration()
                newGen := e.ObjectNew.GetGeneration()
                return oldGen != newGen
            },
            DeleteFunc: func(e event.DeleteEvent) bool {
                // Always reconcile deletes for cleanup
                return true
            },
        }).
        Complete(r)
}
```

### Pattern 3: State Machine for Game Server Lifecycle

**What:** Game servers transition through well-defined states: Creating → Ready → Allocated → Shutdown. State transitions are managed by the operator and reflected in CR status.

**When to use:** For GameServer CRD. Mirrors Agones' proven design.

**Trade-offs:**
- Pro: Clear semantics, prevents invalid transitions, enables allocation logic
- Con: More complex than simple on/off, requires careful state management

**Example:**
```go
type GameServerState string

const (
    StateCreating   GameServerState = "Creating"   // Pod being created
    StateReady      GameServerState = "Ready"      // Available for allocation
    StateAllocated  GameServerState = "Allocated"  // In use by players
    StateShutdown   GameServerState = "Shutdown"   // Terminating
    StateError      GameServerState = "Error"      // Unrecoverable failure
)

// GameServerStatus defines observed state
type GameServerStatus struct {
    State       GameServerState       `json:"state,omitempty"`
    Address     string                `json:"address,omitempty"`
    Ports       []GameServerPort      `json:"ports,omitempty"`
    Players     int32                 `json:"players,omitempty"`
    Conditions  []metav1.Condition    `json:"conditions,omitempty"`
}

// Transition logic (in controller)
func (r *GameServerReconciler) transitionState(gs *v1alpha1.GameServer, newState GameServerState) error {
    // Validate transitions
    validTransitions := map[GameServerState][]GameServerState{
        StateCreating:  {StateReady, StateError},
        StateReady:     {StateAllocated, StateShutdown},
        StateAllocated: {StateShutdown},
        StateShutdown:  {}, // Terminal state
        StateError:     {}, // Terminal state
    }

    if !sliceContains(validTransitions[gs.Status.State], newState) {
        return fmt.Errorf("invalid transition: %s -> %s", gs.Status.State, newState)
    }

    gs.Status.State = newState
    return nil
}
```

### Pattern 4: API Server as Kubernetes Client Gateway

**What:** REST API server acts as an authentication/authorization layer in front of the Kubernetes API. Exposes simplified, user-friendly endpoints while translating to K8s client operations.

**When to use:** Always. Never expose K8s API directly to end users.

**Trade-offs:**
- Pro: Custom auth (JWT), simplified API surface, business logic enforcement
- Con: Additional service to maintain, potential sync lag with cluster state

**Example:**
```go
// API Server Handler
func (h *Handler) CreateGameServer(c *gin.Context) {
    var req CreateGameServerRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    // Business logic validation
    if !h.validateGameType(req.GameType) {
        c.JSON(400, gin.H{"error": "unsupported game type"})
        return
    }

    // Construct GameServer CR
    gameServer := &v1alpha1.GameServer{
        ObjectMeta: metav1.ObjectMeta{
            Name:      req.Name,
            Namespace: h.userNamespace(c.GetString("userID")),
            Labels: map[string]string{
                "app.kubernetes.io/managed-by": "kterodactyl",
                "kterodactyl.io/game":          req.GameType,
                "kterodactyl.io/user":          c.GetString("userID"),
            },
        },
        Spec: v1alpha1.GameServerSpec{
            GameType: req.GameType,
            Resources: req.Resources,
        },
    }

    // Create via Kubernetes client
    if err := h.k8sClient.Create(c.Request.Context(), gameServer); err != nil {
        c.JSON(500, gin.H{"error": "failed to create game server"})
        return
    }

    c.JSON(201, gameServer)
}
```

### Pattern 5: Dynamic Forms from JSON Schema

**What:** Game parameter manifests define UI forms declaratively. Frontend generates forms at runtime from JSON schemas, eliminating need to code forms for each game.

**When to use:** For all game configuration UIs. Enables adding games without frontend changes.

**Trade-offs:**
- Pro: Extremely flexible, no frontend code per game, user-extensible
- Con: Complex forms require rich schema features, validation can be tricky

**Example:**
```typescript
// games/minecraft/manifest.yaml -> JSON schema
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Minecraft Server",
  "type": "object",
  "properties": {
    "server_port": {
      "type": "integer",
      "default": 25565,
      "minimum": 1024,
      "maximum": 65535
    },
    "max_players": {
      "type": "integer",
      "default": 20,
      "minimum": 1
    },
    "game_mode": {
      "type": "string",
      "enum": ["survival", "creative", "adventure", "spectator"],
      "default": "survival"
    }
  },
  "required": ["server_port"]
}

// React component (using react-hook-form + zod)
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { generateZodSchema } from "@/lib/json-schema-to-zod";

export function GameServerForm({ gameType }: { gameType: string }) {
  const schema = useMemo(() =>
    generateZodSchema(gameManifests[gameType].schema),
    [gameType]
  );

  const form = useForm({
    resolver: zodResolver(schema),
    defaultValues: schema.parse({}), // Applies defaults
  });

  return (
    <Form {...form}>
      {/* Render fields from schema */}
    </Form>
  );
}
```

### Pattern 6: Finalizers for External Resource Cleanup

**What:** Finalizers block CR deletion until cleanup completes. Used to delete S3 backups, external DNS records, or other resources outside K8s before removing CR.

**When to use:** When CRs own external resources that must be cleaned up.

**Trade-offs:**
- Pro: Prevents orphaned resources, ensures cleanup happens
- Con: Can cause stuck resources if finalizer logic fails

**Example:**
```go
const backupFinalizerName = "backup.kterodactyl.io/finalizer"

func (r *BackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    backup := &v1alpha1.Backup{}
    if err := r.Get(ctx, req.NamespacedName, backup); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    // Handle deletion
    if !backup.DeletionTimestamp.IsZero() {
        if controllerutil.ContainsFinalizer(backup, backupFinalizerName) {
            // Delete S3 objects
            if err := r.s3Client.DeleteBackup(ctx, backup); err != nil {
                return ctrl.Result{}, err
            }

            // Remove finalizer after successful cleanup
            controllerutil.RemoveFinalizer(backup, backupFinalizerName)
            if err := r.Update(ctx, backup); err != nil {
                return ctrl.Result{}, err
            }
        }
        return ctrl.Result{}, nil
    }

    // Add finalizer if not present
    if !controllerutil.ContainsFinalizer(backup, backupFinalizerName) {
        controllerutil.AddFinalizer(backup, backupFinalizerName)
        if err := r.Update(ctx, backup); err != nil {
            return ctrl.Result{}, err
        }
    }

    // Normal reconciliation logic...
    return ctrl.Result{}, nil
}
```

### Pattern 7: Status Conditions for Observability

**What:** Use standard Kubernetes Conditions in CR status to provide human-readable state. Enables monitoring, alerting, and kubectl/UI display without parsing custom fields.

**When to use:** On all CRDs. Follow Kubernetes API conventions for condition types.

**Trade-offs:**
- Pro: Standard tooling works (kubectl, Prometheus rules, dashboards)
- Con: Slightly more verbose than custom status fields

**Example:**
```go
import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/api/meta"
)

// Standard condition types
const (
    ConditionTypeReady     = "Ready"
    ConditionTypeAvailable = "Available"
    ConditionTypeProgressing = "Progressing"
    ConditionTypeDegraded  = "Degraded"
)

// Update status with condition
func (r *GameServerReconciler) setCondition(gs *v1alpha1.GameServer, condType string, status metav1.ConditionStatus, reason, message string) {
    meta.SetStatusCondition(&gs.Status.Conditions, metav1.Condition{
        Type:               condType,
        Status:             status,
        ObservedGeneration: gs.Generation,
        LastTransitionTime: metav1.Now(),
        Reason:             reason,
        Message:            message,
    })
}

// Example usage
if pod.Status.Phase == corev1.PodRunning {
    r.setCondition(gameServer, ConditionTypeReady, metav1.ConditionTrue,
        "PodRunning", "Game server pod is running")
} else {
    r.setCondition(gameServer, ConditionTypeReady, metav1.ConditionFalse,
        "PodNotReady", "Waiting for game server pod to start")
}
```

## Data Flow

### Request Flow: User Creates Game Server

```
[User in Web UI]
    ↓ (1) POST /api/gameservers
[API Server]
    ↓ (2) JWT validation, authz check
[API Server Handler]
    ↓ (3) Load game manifest, validate params
[API Server Handler]
    ↓ (4) k8sClient.Create(GameServer CR)
[Kubernetes API Server]
    ↓ (5) Persist to etcd, emit event
[GameServer Controller Watch]
    ↓ (6) Event filtered by predicate
[GameServer Controller Reconcile]
    ↓ (7) Create Pod, Service, Ingress/HTTPRoute
[Kubernetes Scheduler]
    ↓ (8) Assign Pod to node
[Kubelet]
    ↓ (9) Pull image, start container
[Game Server Pod]
    ↓ (10) Health check passes
[GameServer Controller]
    ↓ (11) Update CR status: State=Ready, Address=IP
[API Server Long Poll / WebSocket]
    ↓ (12) Push status update
[Frontend]
    ↓ (13) Display "Server Ready" with connect info
```

### State Management: Operator Reconciliation

```
[Kubernetes API Server]
    ↓ (watch) GameServer CRD events
[Informer Cache] ← (shared across all controllers)
    ↓ (predicate filter)
[Work Queue] ← (rate limited, deduplicated)
    ↓ (worker goroutine)
[Reconcile Function]
    ↓ (read) Get GameServer CR from cache
    ↓ (read) Get owned resources (Pod, Service) from cache
    ↓ (compare) Desired state vs actual state
    ↓ (write) Create/Update/Delete via API client
[Kubernetes API Server]
    ↓ (persist)
[etcd]
    ↓ (watch event)
[Informer Cache] ← (loop continues)
```

### Key Data Flows

1. **User → API Server → Operator:** HTTP request creates CR, operator watches and acts
2. **Operator → Kubernetes API:** Controller reads from cache, writes through direct client (controller-runtime DelegatingClient pattern)
3. **Metrics Collection:** controller-runtime metrics + custom metrics → Prometheus scrape → ServiceMonitor CRD
4. **Backup Flow:** CronJob triggers → Backup Controller creates Job → Job writes to S3 → Backup CR status updated
5. **DNS Provisioning:** GameServer created → DNS Controller reconciles → Creates Ingress/HTTPRoute with host `{game}.{user}.domain.com` → ExternalDNS creates DNS record

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| **5-20 servers (homelab)** | Single replica operator/API server, shared cache works fine, SQLite for API server metadata, minimal resource requests |
| **20-100 servers (small business)** | Still single replica, start tuning rate limiters, PostgreSQL recommended over SQLite, add resource limits, enable metrics/monitoring |
| **100-500 servers (medium deployment)** | Consider API server horizontal scaling (stateless), operator can stay single replica, optimize predicate filters aggressively, use separate controller per CRD with tuned MaxConcurrentReconciles, dedicated node pool for game servers |
| **500+ servers (large scale)** | Multi-replica API servers behind load balancer, leader-elected operator replicas, shard game servers across namespaces, consider FleetAutoscaler pattern from Agones, implement allocation service for efficient server assignment |

### Scaling Priorities

1. **First bottleneck (100-200 servers):** Reconciliation rate limits. Solution: Aggressive event filtering with predicates, tune controller-runtime rate limiters, increase MaxConcurrentReconciles per controller.

2. **Second bottleneck (300-500 servers):** API server database. Solution: Move from SQLite to PostgreSQL, implement connection pooling, add read replicas if needed, cache frequently accessed data.

3. **Third bottleneck (500+ servers):** Operator cache memory. Solution: Use label selectors on cache to watch only relevant namespaces, implement cache field selectors (controller-runtime design proposal), consider running multiple operator instances with namespace sharding.

## Anti-Patterns

### Anti-Pattern 1: Single Controller for Multiple CRDs

**What people do:** Create one controller that reconciles GameServer, Backup, and DNS CRDs to "simplify" the codebase.

**Why it's wrong:** Violates single responsibility principle, creates complex synchronization, prevents independent rate limiting, causes cascading failures, makes debugging harder. Kubebuilder explicitly warns against this.

**Do this instead:** One controller per CRD. Share logic via helper functions in pkg/reconciler/ if needed.

### Anti-Pattern 2: Polling Instead of Watching

**What people do:** API server polls Kubernetes API every N seconds to get game server status instead of using watch/informer.

**Why it's wrong:** Massive API load, stale data between polls, scales terribly (O(n) requests per poll interval).

**Do this instead:** Use controller-runtime's shared cache/informer pattern. Subscribe to watch events. Let the cache handle efficient watching.

### Anti-Pattern 3: Storing User Data in CRDs

**What people do:** Put user passwords, billing info, or other application data in GameServer CR spec/status.

**Why it's wrong:** CRDs are for operational state, not application databases. etcd has size limits, no ACID guarantees, poor query performance. Security risk if RBAC is misconfigured.

**Do this instead:** Use external database (PostgreSQL) for application data. CRDs only store operational state (game type, resource requests, status).

### Anti-Pattern 4: Using Ingress in 2026+

**What people do:** Continue using Kubernetes Ingress API for new deployments.

**Why it's wrong:** Ingress NGINX reaches end-of-life March 2026 (no security updates). Gateway API (HTTPRoute) is the successor with richer features.

**Do this instead:** Use Gateway API with HTTPRoute for new deployments. Make it configurable in Helm chart (values.networking.api: "ingress" | "gateway") for backward compatibility, but default to Gateway API.

### Anti-Pattern 5: No Event Filtering

**What people do:** Let controllers reconcile on every watch event (status updates, owner references changes, etc.).

**Why it's wrong:** 60%+ of reconciliations are no-ops. Wastes CPU, increases API load, causes rate limiting.

**Do this instead:** Always use predicates. At minimum, filter on Generation changes for spec updates and always reconcile deletes.

### Anti-Pattern 6: Dockerfile Without Multi-Stage Builds

**What people do:** Build Go operators with full Go toolchain in final image, or use scratch without understanding static linking.

**Why it's wrong:** 1GB+ images when they should be <20MB. Includes compilers/shells that attackers can use. Slow image pulls.

**Do this instead:** Multi-stage build with distroless base. Set CGO_ENABLED=0 for static binary. Use ko for even simpler builds with automatic SBOM generation.

### Anti-Pattern 7: CRDs in Helm Templates

**What people do:** Put CRDs in templates/ directory with regular manifests.

**Why it's wrong:** Helm 3 has special crds/ directory that installs CRDs first and never deletes them. Templates can't have install ordering guarantees.

**Do this instead:** Always put CRDs in crds/ directory. Use pre-install hooks only if you need Job-based CRD installation (rare).

### Anti-Pattern 8: Blocking Reconcile on External Calls

**What people do:** Reconcile function makes synchronous HTTP calls to external APIs, waits for S3 uploads, etc.

**Why it's wrong:** Blocks reconciliation queue, prevents other resources from being processed, can cause timeouts and controller restarts under load.

**Do this instead:** Offload long-running tasks to Jobs. Reconcile function creates Job CR, requeues, and checks Job status on next reconcile. Or use Go goroutines with proper context handling.

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| **S3-Compatible Storage** | AWS SDK for Go, credentials via Secret | Use for backups. Velero pattern: Job creates backup, uploads, updates CR status. |
| **Prometheus** | ServiceMonitor CRD, /metrics endpoint | Operator exposes controller-runtime metrics + custom game server metrics. Prometheus Operator scrapes. |
| **ExternalDNS** | Annotate Ingress/HTTPRoute resources | If ExternalDNS is installed, it watches Ingress/HTTPRoute and creates DNS records automatically. |
| **PostgreSQL** | pgx driver, migrations via golang-migrate | API server metadata (users, auth tokens). Not for operational state. |
| **Container Registry** | Kubernetes imagePullSecrets | Game server images pulled by kubelet. Operator doesn't interact directly. |
| **Identity Provider (OAuth)** | JWT validation, optional OIDC integration | API server verifies tokens. Can integrate with Dex, Keycloak, etc. |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| **Frontend ↔ API Server** | HTTP REST, JWT bearer tokens | Stateless. API server validates JWT, loads user context, proxies K8s operations. |
| **API Server ↔ Kubernetes API** | controller-runtime client.Client | Uses service account token, in-cluster config. Respects RBAC. |
| **Operator ↔ Kubernetes API** | Watch (informer cache) + direct writes | DelegatingClient: reads from cache, writes to API server. |
| **DNS Controller ↔ ExternalDNS** | Kubernetes resources (Ingress/HTTPRoute annotations) | Decoupled. DNS Controller creates Ingress, ExternalDNS watches and creates DNS. |
| **GameServer Controller ↔ DNS Controller** | Ownership via OwnerReferences | DNS Controller watches GameServer CRs, creates Ingress/HTTPRoute with owner reference. |
| **Backup Controller ↔ CronJobs** | Controller creates CronJob CRs | CronJob triggers Job, Job runs backup script, updates Backup CR status. |

## Build Order and Dependencies

### Suggested Build Order

Building a K8s operator-based system requires respecting dependencies between components. Here's the recommended order:

**Phase 1: Core Operator Foundation**
1. **Project scaffolding** (Kubebuilder init, create API types)
2. **GameServer CRD + basic controller** (no-op reconcile, just logging)
3. **Pod lifecycle management** (controller creates Pods for GameServers)
4. **State machine implementation** (Creating → Ready → Allocated → Shutdown)
5. **Status conditions** (add observability before complexity grows)

**Phase 2: API Server Bridge**
6. **Go REST API server** (basic CRUD endpoints, stub responses)
7. **Kubernetes client integration** (API server creates GameServer CRs)
8. **Authentication** (JWT validation, user context)
9. **Game manifest loading** (read games/ directory, validate JSON schemas)

**Phase 3: User Interface**
10. **Frontend scaffold** (Next.js, routing, API client)
11. **JSON schema to form conversion** (react-hook-form + zod)
12. **Game server list/detail views** (read-only first)
13. **Create game server form** (dynamic based on game type)

**Phase 4: DNS and Networking**
14. **DNS Controller** (watches GameServer, creates Ingress/HTTPRoute)
15. **Gateway API integration** (HTTPRoute support, configurable via Helm)
16. **Wildcard subdomain routing** (test game.user.domain.com pattern)

**Phase 5: Backup and Durability**
17. **Backup CRD + controller** (S3 upload logic, finalizers)
18. **CronJob integration** (scheduled backups)
19. **Restore operations** (API endpoint → Backup CR with restore spec)

**Phase 6: Metrics and Monitoring**
20. **Prometheus metrics** (controller-runtime + custom metrics)
21. **ServiceMonitor CRDs** (Prometheus Operator integration)
22. **Grafana dashboards** (optional, for visualization)

**Phase 7: Packaging and Deployment**
23. **Helm chart creation** (structure with crds/ directory)
24. **Multi-stage Dockerfiles** (operator + API server)
25. **CI/CD pipeline** (build, test, push images, release Helm chart)

### Key Dependencies

```
GameServer Controller
    ↓ (must exist before)
DNS Controller (watches GameServer CRs)

GameServer Controller + Game Manifests
    ↓ (provides data for)
API Server (loads manifests, creates CRs)

API Server
    ↓ (provides endpoints for)
Frontend (consumes REST API)

Backup Controller
    ↓ (requires)
S3 Configuration + Finalizers (for cleanup)

All Controllers
    ↓ (require)
CRDs Installed (Helm crds/ directory ensures this)

Operator + API Server Images
    ↓ (packaged by)
Helm Chart (deploys entire system)
```

### Critical Path

The critical path for MVP (minimum viable product):

1. GameServer CRD + Controller (core functionality)
2. API Server + K8s client (user-facing interface)
3. Frontend + dynamic forms (usability)
4. Helm chart (deployability)

Everything else can be added incrementally after MVP.

## CRD Design Reference

### GameServer CRD

```yaml
apiVersion: kterodactyl.io/v1alpha1
kind: GameServer
metadata:
  name: minecraft-johndoe-1
  namespace: kterodactyl
spec:
  # What to run
  gameType: minecraft         # References games/minecraft/
  version: "1.20.4"           # Game version

  # Resource allocation
  resources:
    requests:
      memory: "2Gi"
      cpu: "1000m"
    limits:
      memory: "4Gi"
      cpu: "2000m"

  # Game-specific parameters (validated against manifest schema)
  parameters:
    server_port: 25565
    max_players: 20
    game_mode: "survival"

  # Networking
  networking:
    subdomain: "minecraft-johndoe-1"  # Creates minecraft-johndoe-1.{user}.domain.com

  # Lifecycle
  autoShutdown:
    enabled: true
    idleTimeout: "30m"

status:
  state: Ready                # Creating|Ready|Allocated|Shutdown|Error
  address: "192.168.1.100"
  ports:
    - name: game
      port: 25565
      protocol: TCP
  players: 0
  conditions:
    - type: Ready
      status: "True"
      lastTransitionTime: "2026-02-09T12:00:00Z"
      reason: PodRunning
      message: "Game server is ready for connections"
```

### Backup CRD

```yaml
apiVersion: kterodactyl.io/v1alpha1
kind: Backup
metadata:
  name: minecraft-backup-20260209
  namespace: kterodactyl
  finalizers:
    - backup.kterodactyl.io/finalizer  # Ensures S3 cleanup
spec:
  gameServerRef:
    name: minecraft-johndoe-1
  destination:
    s3:
      bucket: kterodactyl-backups
      path: /backups/minecraft-johndoe-1/
  schedule: "0 3 * * *"       # Optional: cron schedule for automated backups
  retention:
    keepLast: 7               # Keep last 7 backups

status:
  phase: Completed            # Pending|Running|Completed|Failed
  startTime: "2026-02-09T03:00:00Z"
  completionTime: "2026-02-09T03:05:23Z"
  size: "1.2GB"
  s3Key: "backups/minecraft-johndoe-1/20260209-030000.tar.gz"
  conditions:
    - type: Completed
      status: "True"
      lastTransitionTime: "2026-02-09T03:05:23Z"
      reason: UploadSuccessful
      message: "Backup uploaded to S3"
```

## Sources

### Agones Architecture
- [Agones Overview Documentation](https://agones.dev/site/docs/overview/)
- [Agones GitHub Repository](https://github.com/googleforgames/agones)
- [Agones – a Kubernetes-centric Game Server Toolkit](https://tavant.com/blog/agones-kubernetes-centric-game-server-toolkit/)
- [Hands-On With Agones and Google Cloud Game Servers](https://www.fairwinds.com/blog/hands-on-with-agones-google-cloud-game-servers)

### Kubernetes Operator Patterns
- [Operator pattern | Kubernetes](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
- [Exploring Kubernetes Operator Pattern](https://iximiuz.com/en/posts/kubernetes-operator-pattern/)
- [Operator Best Practices | Operator SDK](https://sdk.operatorframework.io/docs/best-practices/best-practices/)
- [Good Practices - The Kubebuilder Book](https://book.kubebuilder.io/reference/good-practices)
- [Kubernetes Operators in 2025: Best Practices, Patterns, and Real-World Insights](https://outerbyte.com/kubernetes-operators-2025-guide/)

### CRD and Reconciliation
- [Beyond YAML: Building Kubernetes Operators with CRDs and the Reconciliation Loop](https://dev.to/naveens16/beyond-yaml-building-kubernetes-operators-with-crds-and-the-reconciliation-loop-524d)
- [Custom Resources | Kubernetes](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
- [Common recommendations and suggestions | Operator SDK](https://sdk.operatorframework.io/docs/best-practices/common-recommendation/)

### Helm and Deployment
- [Helm Charts in Kubernetes – 2026 Guide](https://atmosly.com/knowledge/helm-charts-in-kubernetes-definitive-guide-for-2025)
- [Custom Resource Definitions | Helm](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/)
- [Chart Hooks | Helm](https://helm.sh/docs/topics/charts_hooks/)
- [How to Handle Helm CRD Installation and Upgrades](https://oneuptime.com/blog/post/2026-01-17-helm-crd-installation-upgrades/view)

### Networking and DNS
- [Gateway API vs Ingress: The Future of Kubernetes Networking](https://konghq.com/blog/engineering/gateway-api-vs-ingress)
- [Kubernetes Is Moving On From ingress‑nginx: How Are You Planning Your 2026 Migration?](https://www.fairwinds.com/blog/kubernetes-ingress-nginx-2026-migration)
- [Migrating from Ingress - Kubernetes Gateway API](https://gateway-api.sigs.k8s.io/guides/getting-started/migrating-from-ingress/)
- [External DNS in Kubernetes: Pros, Cons, and Critical Best Practices](https://komodor.com/learn/external-dns-in-kubernetes-pros-cons-and-critical-best-practices/)

### Frontend Architecture
- [Building Dynamic Forms in React with JSON Schema and Material-UI](https://codup.co/blog/building-dynamic-forms-in-react-with-json-schema-and-material-ui/)
- [React JSON Schema Form GitHub](https://github.com/rjsf-team/react-jsonschema-form)
- [React Stack Patterns](https://www.patterns.dev/react/react-2026/)

### Backup and Storage
- [Backup and restore your Amazon EKS cluster resources using Velero](https://aws.amazon.com/blogs/containers/backup-and-restore-your-amazon-eks-cluster-resources-using-velero/)
- [Kubernetes Backup and Restore: A Comprehensive Guide](https://medium.com/@tadbiri2012/kubernetes-backup-and-restore-a-comprehensive-guide-5ac011e15297)

### Metrics and Observability
- [Metrics - The Kubebuilder Book](https://book.kubebuilder.io/reference/metrics)
- [Monitor your Kubernetes operators to keep applications running smoothly](https://www.datadoghq.com/blog/kubernetes-operator-performance/)
- [Kubernetes Observability and Monitoring Trends in 2026](https://www.usdsi.org/data-science-insights/kubernetes-observability-and-monitoring-trends-in-2026)

### Build and Container Images
- [How to Containerize Go Apps with Multi-Stage Dockerfiles](https://oneuptime.com/blog/post/2026-01-07-go-docker-multi-stage/view)
- [Optimizing Docker Images with Multi-Stage Builds and Distroless Approach](https://dev.to/suzuki0430/optimizing-docker-images-with-multi-stage-builds-and-distroless-approach-h0l)
- [GoogleContainerTools/distroless GitHub](https://github.com/GoogleContainerTools/distroless)
- [Migrating from Dockerfile - ko](https://ko.build/advanced/migrating-from-dockerfile/)

### Controller-Runtime Implementation
- [Kubernetes Controllers at Scale: Clients, Caches, Conflicts, Patches Explained](https://medium.com/@timebertt/kubernetes-controllers-at-scale-clients-caches-conflicts-patches-explained-aa0f7a8b4332)
- [Understanding the controller-runtime Cache Seriously](https://dev.to/shuheiktgw/understanding-the-controller-runtime-cache-seriously-3c2k)
- [Using Predicates for Event Filtering with Operator SDK](https://sdk.operatorframework.io/docs/building-operators/golang/references/event-filtering/)
- [Watching Resources - The Kubebuilder Book](https://book.kubebuilder.io/reference/watching-resources)

### Finalizers and Cleanup
- [Finalizers | Kubernetes](https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/)
- [Using Finalizers to Control Deletion](https://kubernetes.io/blog/2021/05/14/using-finalizers-to-control-deletion/)
- [Using Finalizers - The Kubebuilder Book](https://book.kubebuilder.io/reference/using-finalizers)

### Status Conditions
- [community/api-conventions.md at master · kubernetes/community](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)
- [What the heck are Conditions in Kubernetes controllers?](https://maelvls.dev/kubernetes-conditions/)
- [Storing State of Kubernetes Resources with Conditions](https://heidloff.net/article/storing-state-status-kubernetes-resources-conditions-operators-go/)

### Monorepo Strategy
- [Monorepo Guide: Manage Repositories & Microservices](https://www.aviator.co/blog/monorepo-a-hands-on-guide-for-managing-repositories-and-microservices/)
- [Benefits and challenges of monorepo development practices](https://circleci.com/blog/monorepo-dev-practices/)
- [Monorepo vs Multirepo: which development strategy for your company?](https://news.infomaniak.com/en/multirepo-vs-microservices/)

### Pterodactyl Reference
- [Pterodactyl Panel GitHub](https://github.com/pterodactyl/panel)
- [Pterodactyl Introduction](https://pterodactyl.io/project/introduction.html)

---
*Architecture research for: Kterodactyl - Kubernetes-native game server management panel*
*Researched: 2026-02-09*
