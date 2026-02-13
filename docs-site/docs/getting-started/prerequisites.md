---
sidebar_position: 2
title: Prerequisites
---

# Prerequisites

Before installing Kterodactyl, ensure you have the following components available in your environment.

## Required

### Kubernetes Cluster (v1.26+)

Kterodactyl requires a Kubernetes cluster running version 1.26 or later. Any conformant distribution works, including:

- **Homelab:** Talos, k3s, microk8s, kind
- **Cloud:** EKS, GKE, AKS

Verify your cluster version:

```bash
kubectl version --short
```

### Helm 3.x

The Kterodactyl Helm chart requires Helm 3. Install from the [official Helm documentation](https://helm.sh/docs/intro/install/).

```bash
helm version
```

### kubectl

You need `kubectl` configured to communicate with your target cluster.

```bash
kubectl cluster-info
```

### Gateway API CRDs

Kterodactyl uses the Kubernetes [Gateway API](https://gateway-api.sigs.k8s.io/) for DNS routing. The Gateway API CRDs must be installed in your cluster before deploying Kterodactyl.

Install the standard Gateway API CRDs:

```bash
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.1/standard-install.yaml
```

You also need a Gateway API implementation (controller) such as:

- [Envoy Gateway](https://gateway.envoyproxy.io/)
- [Cilium](https://docs.cilium.io/en/stable/network/servicemesh/gateway-api/gateway-api/)
- [NGINX Gateway Fabric](https://docs.nginx.com/nginx-gateway-fabric/)

:::info DNS Routing is Optional
If you set `adminConfig.networking.baseDomain` to an empty string, DNS routing is disabled entirely. Gateway API CRDs are only required if you want automatic DNS hostnames for game servers.
:::

### Wildcard DNS Entry

For DNS routing to work, you need a wildcard DNS entry pointing to your cluster ingress. For example, if your base domain is `game.example.com`, you need `*.game.example.com` resolving to your Gateway controller's external IP.

Options for providing wildcard DNS:

- **Cloudflare Tunnel** -- Route traffic through Cloudflare without exposing your IP
- **ExternalDNS** -- Automatically manage DNS records in your DNS provider
- **Manual DNS** -- Create a wildcard A/CNAME record pointing to your ingress IP
- **PiHole / Local DNS** -- For homelab-only access, add local DNS entries

## Optional

### S3-Compatible Storage (for Backups)

If you want to enable game server backups, you need access to S3-compatible object storage:

- **Homelab:** [MinIO](https://min.io/) is a lightweight S3-compatible server you can run in your cluster
- **Cloud:** AWS S3, Google Cloud Storage, DigitalOcean Spaces, Backblaze B2

Kterodactyl auto-creates the backup bucket on first use, so you only need credentials and an endpoint.

### SMTP Server (for Email Invitations)

Kterodactyl supports sending invite emails to new users. This requires an SMTP server:

- **Homelab:** Any SMTP relay (Mailhog for testing)
- **Cloud:** SendGrid, Mailgun, AWS SES, Gmail SMTP

:::tip SMTP is Not Required
Without SMTP configuration, invite links are returned directly in the API response. Admins can copy and share the link manually.
:::
