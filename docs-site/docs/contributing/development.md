---
sidebar_position: 2
---

# Development Guide

This guide covers local development setup, project structure, key Makefile targets, and the build pipeline.

## Prerequisites

- **Go** 1.24+ ([install](https://go.dev/dl/))
- **Node.js** 20+ ([install](https://nodejs.org/)) -- for frontend and documentation builds
- **Docker** 17.03+ -- for container image builds
- **kubectl** -- for interacting with Kubernetes clusters
- **Helm** 3.x -- for chart development and testing

## Clone and Setup

```bash
git clone https://github.com/kterodactyl/kterodactyl.git
cd kterodactyl
```

Go dependencies are managed via `go.mod` and downloaded automatically on first build. Frontend dependencies are installed via `npm ci` during the frontend build step.

## Project Structure

```
kterodactyl/
  api/
    v1alpha1/               # CRD type definitions (GameServer, Backup)
      gameserver_types.go   # GameServerSpec, GameServerStatus, GameServerPort
      backup_types.go       # BackupSpec, BackupStatus
      gameserver_lifecycle.go  # State constants, valid transitions, helpers
  cmd/
    main.go                 # Manager entrypoint (controllers + API server)
  internal/
    controller/             # Kubernetes reconcilers
      gameserver_controller.go   # GameServer lifecycle management
      dns_controller.go          # Service and HTTPRoute creation
      backup_controller.go       # Backup execution (tar -> S3)
    api/                    # REST API server
      server.go             # Chi v5 router setup
      routes.go             # Route definitions and middleware
      handlers_auth.go      # Login, register, refresh
      handlers_gameserver.go # CRUD, lifecycle, mods
      handlers_games.go     # Game manifest listing
      handlers_admin.go     # User management, invites
      spa.go                # Embedded SPA serving
    auth/                   # Authentication library
      jwt.go                # JWT token creation and validation
      user.go               # User management (K8s Secrets)
      middleware.go         # HTTP auth middleware
    manifest/               # Game definition loader
      loader.go             # Reads games/ directory, compiles schemas
    metrics/                # Prometheus metrics
      metrics.go            # All 5 metric definitions
  web/                      # React SPA (Vite + Tailwind + shadcn)
    src/
      pages/                # Route-based page components
      components/           # Shared UI components
      hooks/                # Custom React hooks (auth, WebSocket)
      stores/               # Zustand state management
  chart/                    # Helm chart
    Chart.yaml
    values.yaml
    crds/                   # CRD YAML manifests
    templates/              # Kubernetes resource templates
  games/                    # Game definitions
    minecraft/              # Reference implementation
      manifest.yaml
      Dockerfile
  config/                   # Kubebuilder configuration
    crd/                    # Generated CRD bases
    rbac/                   # RBAC role definitions
    manager/                # Manager deployment config
    samples/                # Example custom resources
  docs-site/                # Docusaurus documentation site
  docs/                     # Legacy documentation (game-definitions.md)
```

For detailed architecture information, see the [Architecture Overview](/docs/contributing/architecture).

## Makefile Targets

The project uses a Makefile for all build, test, and deployment operations.

### Build

| Target | Description |
|--------|-------------|
| `make build` | Full build: generate manifests, build frontend, compile Go binary to `bin/manager` |
| `make build-frontend` | Build the React SPA and copy output to `internal/api/frontend/` for embedding |
| `make docker-build` | Build the multi-stage Docker image (frontend + Go binary + distroless base) |
| `make docker-push` | Push the built container image to the registry |

### Development

| Target | Description |
|--------|-------------|
| `make dev-frontend` | Run the Vite dev server with hot reload for frontend development |
| `make run` | Run the operator locally (connects to the cluster in your kubeconfig) |
| `make fmt` | Run `go fmt` on all Go source files |
| `make vet` | Run `go vet` on all Go source files |
| `make lint` | Run `golangci-lint` on the codebase |

### Testing

| Target | Description |
|--------|-------------|
| `make test` | Run all Go tests with envtest (excludes e2e). Generates `cover.out` coverage report. |
| `make test-e2e` | Run end-to-end tests using a Kind cluster |

### Code Generation

| Target | Description |
|--------|-------------|
| `make manifests` | Regenerate CRD YAML, RBAC roles, and webhook configurations from Go markers |
| `make generate` | Regenerate `DeepCopy`, `DeepCopyInto`, and `DeepCopyObject` methods |

### Deployment

| Target | Description |
|--------|-------------|
| `make install` | Install CRDs into the cluster specified by your kubeconfig |
| `make deploy` | Deploy the operator to the cluster using Kustomize |
| `make uninstall` | Remove CRDs from the cluster |
| `make undeploy` | Remove the operator deployment from the cluster |

## Build Pipeline

The project uses a multi-stage Dockerfile:

1. **Frontend stage** (`node:22-alpine`): Installs npm dependencies and builds the React SPA with Vite
2. **Go builder stage** (`golang`): Copies the built frontend assets into the embed directory, downloads Go modules, and compiles the manager binary
3. **Production stage** (`gcr.io/distroless/static`): Copies only the compiled binary into a minimal base image

The frontend SPA is embedded into the Go binary using `go:embed` via `internal/api/spa.go`. This means the final container image contains a single binary that serves both the API and the web UI.

```bash
# Build and tag the image
make docker-build IMG=ghcr.io/kterodactyl/kterodactyl:dev

# Push to registry
make docker-push IMG=ghcr.io/kterodactyl/kterodactyl:dev
```

## Frontend Development

The React frontend lives in `web/` and uses:

- **Vite** for bundling and hot module replacement
- **Tailwind CSS v4** for styling
- **shadcn/ui** for component primitives
- **Zustand** for state management (JWT stored in memory only)
- **react-jsonschema-form** for dynamic game configuration forms

To develop the frontend with hot reload:

```bash
make dev-frontend
```

This starts the Vite dev server which proxies API requests to the operator running on your cluster (via port-forward or local `make run`).

## Testing

### Unit and Integration Tests

```bash
make test
```

This runs all Go tests with envtest, which provides a real API server and etcd for testing controller logic without a full cluster. Tests use:

- **Ginkgo/Gomega** for controller tests (Kubebuilder convention)
- **Standard Go testing** for auth and utility packages
- **Manager-based test setup** for integration tests with watches and event filters
- **Unique test namespaces** per test case to prevent cross-test interference

### What envtest Cannot Test

envtest does not run kubelet, so Pod status transitions (Starting to Ready) cannot be tested. These transitions are covered by e2e tests using Kind clusters.

## Adding a CRD Field

When modifying CRD types in `api/v1alpha1/`:

1. Edit the type definitions (e.g., `gameserver_types.go`)
2. Add kubebuilder markers for validation, defaults, and print columns
3. Run `make manifests` to regenerate CRD YAML
4. Run `make generate` to regenerate DeepCopy methods
5. Update the Helm chart CRDs: copy from `config/crd/bases/` to `chart/crds/`
6. Run `make test` to verify nothing is broken
