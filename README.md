# Kterodactyl

Self-service game server management for Kubernetes.

Kterodactyl is a Kubernetes operator that lets admins deploy a single Helm chart and give their users self-service game server provisioning through a web UI. Users browse available games, configure server parameters via dynamically generated forms, and launch dedicated game servers -- all backed by Kubernetes custom resources, Gateway API routing, and S3-compatible backup storage.

## Features

- **Self-service web UI** -- React SPA with game browser, server dashboard, real-time console, and mod upload
- **Dynamic configuration forms** -- JSON Schema in game manifests drives automatic form generation per game type
- **6-state lifecycle management** -- Creating, Starting, Ready, Allocated, Shutdown, Error with operator-managed transitions
- **Namespace-per-user isolation** -- Each user's servers run in a dedicated Kubernetes namespace
- **Gateway API routing** -- Automatic DNS names (`game.user.baseDomain`) via HTTPRoute resources
- **S3-compatible backups** -- On-demand and scheduled backups with admin restore capability
- **Extensible game definitions** -- Add new games by contributing a `manifest.yaml` and `Dockerfile`
- **Prometheus metrics** -- Operator and API metrics for monitoring with ServiceMonitor support

## Quick Start

```bash
# Install CRDs and operator
helm install kterodactyl oci://ghcr.io/kterodactyl/charts/kterodactyl \
  --namespace kterodactyl-system --create-namespace

# Port-forward the API
kubectl port-forward -n kterodactyl-system svc/kterodactyl-api 8080:8080

# Bootstrap the first admin user
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "changeme", "email": "admin@example.com"}'

# Open the UI
open http://localhost:8080
```

## Documentation

Full documentation is available in the [docs-site/](docs-site/) directory:

- [Getting Started](docs-site/docs/getting-started/overview.md) -- Overview, prerequisites, installation
- [Configuration](docs-site/docs/configuration/helm-values.md) -- Helm values, admin config, networking, backups, auth
- [Usage Guides](docs-site/docs/usage/creating-servers.md) -- Creating servers, lifecycle management, backups, admin tasks
- [Contributing](docs-site/docs/contributing/game-definitions.md) -- Game definitions, development setup, architecture
- [API Reference](docs-site/docs/reference/api-endpoints.md) -- REST endpoints, CRD specs, Prometheus metrics

To build and serve the documentation site locally:

```bash
cd docs-site && npm install && npm run start
```

## Development

```bash
# Full build (frontend + Go binary)
make build

# Run Go tests with envtest
make test

# Build container image
make docker-build IMG=ghcr.io/kterodactyl/kterodactyl:dev

# Run frontend dev server with hot reload
make dev-frontend
```

See the [Development Guide](docs-site/docs/contributing/development.md) for the complete list of Makefile targets and project structure.

## Contributing Game Definitions

Adding a new game is as simple as creating a `games/<name>/` directory with a `manifest.yaml` and `Dockerfile`. See the [Game Definitions Guide](docs-site/docs/contributing/game-definitions.md) for a complete walkthrough using the Minecraft reference implementation.

## License

Copyright 2026. Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for details.
# Kterodactyl
