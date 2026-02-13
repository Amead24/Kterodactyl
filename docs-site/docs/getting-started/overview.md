---
sidebar_position: 1
title: Overview
---

# What is Kterodactyl?

Kterodactyl is a **Kubernetes-native game server management panel**. It provides self-service provisioning of dedicated game servers backed entirely by Kubernetes primitives. Think of it as an open-source alternative to [Pterodactyl](https://pterodactyl.io/) rebuilt from the ground up for the Kubernetes ecosystem.

## How Kterodactyl Differs from Pterodactyl

Pterodactyl uses a traditional architecture: a PHP Laravel panel, Wings daemon for node management, Docker containers for game servers, and PostgreSQL for data storage. This stack requires managing multiple services across multiple machines.

Kterodactyl replaces this entire stack with Kubernetes-native components:

| Pterodactyl Component | Kterodactyl Equivalent |
|---|---|
| Laravel Panel + PHP | Go REST API with embedded React SPA |
| Wings Daemon | Kubernetes Operator (controller-runtime) |
| Docker Containers | Kubernetes Pods managed via CRDs |
| PostgreSQL Database | Kubernetes Secrets and ConfigMaps |
| Nginx Reverse Proxy | Gateway API (HTTPRoute resources) |
| File Manager | Mod upload API with PersistentVolumeClaims |
| Backup via Wings | S3-compatible storage via operator |

The result is a single binary that acts as both operator and API server, deployed via a single Helm chart, with all state stored in the Kubernetes API.

## How Kterodactyl Differs from Agones

[Agones](https://agones.dev/) is a Kubernetes controller for hosting, scaling, and orchestrating **multiplayer** game server fleets. It focuses on fleet management, allocation, and autoscaling for real-time multiplayer games (e.g., match-based FPS games).

Kterodactyl serves a different purpose: **self-service provisioning of dedicated servers**. While Agones is designed for game studios running thousands of short-lived match servers, Kterodactyl is designed for homelab admins and small service operators who want to let their users create and manage long-running dedicated servers (e.g., a persistent Minecraft survival world).

| Aspect | Agones | Kterodactyl |
|---|---|---|
| Primary use case | Multiplayer fleet scaling | Self-service dedicated servers |
| Target audience | Game studios, large operators | Homelab admins, small operators |
| Server lifetime | Short-lived (match-based) | Long-lived (persistent worlds) |
| User interaction | Automated allocation | User-driven via web UI |
| Game support | Custom integration per game | Community YAML manifests |
| Backup support | Not included | Built-in S3 backups |

## Who Is Kterodactyl For?

### Homelab Administrators

You run a Kubernetes cluster at home and want to give your friends and family the ability to spin up game servers on demand. You want a clean web UI that non-technical users can navigate, without giving them `kubectl` access.

### Small Service Operators

You provide game server hosting for a community and want a self-service panel that scales with Kubernetes. You need user management, resource quotas, and the ability to add new game types by contributing a YAML manifest.

## Key Features

- **CRD-Based Lifecycle Management** -- Game servers are Kubernetes custom resources with a 6-state machine (Creating, Starting, Ready, Allocated, Shutdown, Error). The operator manages the full lifecycle.

- **Community Game Definitions** -- Games are defined as YAML manifests with embedded JSON Schema for parameters. Adding a new game requires no code changes -- just a manifest file.

- **Dynamic UI from JSON Schema** -- The React frontend generates server configuration forms directly from the JSON Schema in game manifests. New games get configuration forms automatically.

- **Gateway API Routing** -- Each game server gets a DNS entry following the pattern `game.username.baseDomain` via HTTPRoute resources. Players connect using memorable hostnames.

- **JWT Authentication** -- User management with invite-based registration, admin roles, and JWT tokens. Users are stored as Kubernetes Secrets.

- **Mod Support** -- Per-server mod storage backed by PersistentVolumeClaims. Upload, list, and delete mods through the API and UI.

- **S3-Compatible Backups** -- On-demand and scheduled backups to any S3-compatible storage (MinIO for homelab, AWS S3 for cloud). Automatic retention management.

- **Prometheus Metrics** -- Operator and API metrics exposed for monitoring. Tracks game servers by state, reconciliation duration, and HTTP request metrics.

- **Helm Chart Installation** -- Everything deploys with a single `helm install` command. The operator, API server, embedded SPA frontend, CRDs, and RBAC are all included.
