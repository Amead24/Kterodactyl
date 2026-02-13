---
sidebar_position: 3
title: Networking
---

# Networking

Kterodactyl provides automatic DNS routing for game servers using the Kubernetes [Gateway API](https://gateway-api.sigs.k8s.io/). Each game server gets a unique hostname that players can use to connect.

## DNS Pattern

Game server DNS entries follow the pattern:

```
<gameType>.<username>.<baseDomain>
```

For example, if the base domain is `game.example.com`, a Minecraft server owned by user `alice` would be accessible at:

```
minecraft.alice.game.example.com
```

This pattern ensures unique, readable hostnames for every game server.

## Enabling DNS Routing

DNS routing is opt-in. Set `adminConfig.networking.baseDomain` to enable it:

```yaml
adminConfig:
  networking:
    baseDomain: "game.example.com"
```

When `baseDomain` is empty (the default), DNS routing is disabled entirely and no HTTPRoute or Service resources are created for game servers.

## Gateway API Configuration

Kterodactyl creates HTTPRoute resources that reference a parent Gateway. You need to configure which Gateway to use:

```yaml
adminConfig:
  networking:
    baseDomain: "game.example.com"
    gateway:
      name: "my-gateway"
      namespace: "gateway-system"
      controllerNamespace: "envoy-gateway-system"
```

| Setting | Purpose |
|---|---|
| `gateway.name` | Name of the Gateway resource that HTTPRoutes will reference as their parent |
| `gateway.namespace` | Namespace where the Gateway resource lives (defaults to the release namespace) |
| `gateway.controllerNamespace` | Namespace where the Gateway controller runs (used for cross-namespace reference grants) |

### Gateway Resource

You need a Gateway resource that accepts traffic for your base domain. Here is an example using Envoy Gateway:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: my-gateway
  namespace: gateway-system
spec:
  gatewayClassName: eg
  listeners:
    - name: http
      protocol: HTTP
      port: 80
      hostname: "*.game.example.com"
      allowedRoutes:
        namespaces:
          from: All
```

The Gateway must allow routes from the namespace where Kterodactyl creates HTTPRoutes.

## Wildcard DNS

For DNS routing to work, you need wildcard DNS pointing to your Gateway controller's external IP. All subdomains under your base domain must resolve to the same IP.

### Cloudflare Tunnel

Cloudflare Tunnel routes traffic through Cloudflare's network without exposing your cluster's IP address:

1. Install `cloudflared` in your cluster
2. Create a tunnel pointing to your Gateway controller service
3. Add a wildcard DNS record (`*.game.example.com`) as a CNAME to your tunnel hostname

### ExternalDNS

[ExternalDNS](https://github.com/kubernetes-sigs/external-dns) can automatically create DNS records based on Gateway resources and HTTPRoutes.

### Manual DNS

Create a wildcard A record or CNAME in your DNS provider:

- **A record:** `*.game.example.com` pointing to your Gateway's external IP
- **CNAME:** `*.game.example.com` pointing to your load balancer hostname

### Local DNS (Homelab)

For homelab setups where you only need local access, configure your local DNS server (e.g., PiHole, CoreDNS) with a wildcard entry pointing to your cluster node IP.

## NetworkPolicy

Kterodactyl creates NetworkPolicy resources for game server pods. The default policy:

- **Allows** DNS resolution via `kube-system` (for service discovery)
- **Allows** outbound internet traffic (for game updates, mod downloads)
- **Blocks** access to private IP ranges (prevents game servers from reaching internal cluster services)

Enable the NetworkPolicy for the operator's metrics endpoint:

```yaml
networkPolicy:
  enabled: true
```

## Connection Info

When DNS routing is active, the game server's connection address is stored in `status.address` on the GameServer custom resource:

```bash
kubectl get gameserver my-minecraft -o jsonpath='{.status.address}'
```

This address is also displayed in the web UI on the server detail page.
