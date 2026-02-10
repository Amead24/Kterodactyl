# Phase 2: Networking & DNS - Research

**Researched:** 2026-02-10
**Domain:** Kubernetes networking, Gateway API, ExternalDNS, game server port exposure
**Confidence:** HIGH

## Summary

Phase 2 transforms the Phase 1 GameServer operator into a network-accessible system where each game server gets a human-readable DNS name (`game.username.domain.com`) and is reachable by players. This requires solving three distinct networking problems: (1) HTTP routing for the future web panel/console traffic via Gateway API HTTPRoute, (2) raw TCP/UDP port exposure for actual game protocol traffic, and (3) automated DNS record provisioning via ExternalDNS.

A critical architectural finding is that Gateway API wildcard hostnames only support a single wildcard label (`*.example.com`) -- nested wildcards like `*.*.domain.com` are explicitly prohibited by the spec. This means the `game.username.domain.com` pattern requires creating **individual HTTPRoute resources per GameServer** with explicit hostnames, not a single wildcard route. This is actually the correct approach because it gives ExternalDNS a concrete hostname per route to provision DNS records for.

The recommended architecture separates concerns: the existing GameServer controller creates a **Service per GameServer** (ClusterIP for HTTP routing, NodePort for game traffic), and a new **DNS controller** watches GameServer resources and creates HTTPRoute + Service resources with the correct DNS names. ExternalDNS watches the HTTPRoute resources and automatically provisions DNS A/CNAME records. This follows the Kubernetes operator pattern of "one controller per CRD" while extending it to "one controller per concern."

**Primary recommendation:** Implement a DNS controller as a second reconciler in the same operator binary that watches GameServer CRs and creates per-server Services and HTTPRoute resources. Use NodePort Services for game traffic (TCP/UDP) and HTTPRoute for HTTP traffic. Configure ExternalDNS with `--source=gateway-httproute` to auto-provision DNS records.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| sigs.k8s.io/gateway-api | v1.4.1 | Go types for Gateway API (HTTPRoute, Gateway) | Official Gateway API Go module; HTTPRoute is GA (v1); replaces deprecated Ingress |
| sigs.k8s.io/controller-runtime | v0.23.1 (already in go.mod) | Controller framework for building the DNS controller | Already used by Phase 1; provides Watches, Owns, reconciler patterns |
| ExternalDNS | v0.16+ (cluster deployment) | Automatic DNS record provisioning from HTTPRoute hostnames | kubernetes-sigs project; watches HTTPRoute and creates DNS records automatically |
| cert-manager | v1.19+ (cluster deployment) | TLS certificate automation for wildcard certs | Industry standard for K8s TLS; needed for HTTPS on `*.domain.com` |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| Gateway controller (e.g., Envoy Gateway, NGINX Gateway Fabric, Cilium) | Implementation-dependent | Processes Gateway and HTTPRoute resources into actual load balancer config | Required infrastructure; admin deploys as prerequisite; operator does not manage this |
| MetalLB | Latest (cluster deployment) | LoadBalancer Service support for bare-metal/homelab | Only for on-prem/homelab clusters without cloud LoadBalancer support |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| HTTPRoute (Gateway API) | Ingress | Ingress NGINX retired March 2026; no new features; Gateway API is the successor |
| NodePort Service per GameServer | hostPort on Pod | NodePort is more flexible (no scheduling constraints), but adds a network hop; hostPort locks pod to one-per-node-per-port |
| ExternalDNS auto-provisioning | Manual DNS records | ExternalDNS is the standard; manual approach does not scale |
| Per-server HTTPRoute | Single wildcard HTTPRoute | Gateway API only supports `*.domain.com`, not `*.*.domain.com`; per-server routes are required |

### Go Module Addition

```bash
go get sigs.k8s.io/gateway-api@v1.4.1
```

## Architecture Patterns

### Recommended Project Structure (additions to Phase 1)

```
internal/
  controller/
    gameserver_controller.go       # Existing Phase 1 controller
    dns_controller.go              # NEW: DNS/networking controller
    dns_controller_test.go         # NEW: DNS controller tests
    suite_test.go                  # Existing (update for new controller)
  util/
    labels.go                      # Existing (add networking labels/annotations)
    networking.go                  # NEW: DNS name construction, port helpers
config/
  rbac/
    role.yaml                      # Update: add Gateway API + Service RBAC
  samples/
    game_v1alpha1_gameserver.yaml  # Update: show connection info in status
```

### Pattern 1: Separate DNS Controller Watching GameServer CRs

**What:** A second controller in the same operator binary that watches GameServer CRs and reconciles networking resources (Service, HTTPRoute). It does NOT modify the GameServer CR -- only reads it and creates owned networking resources.

**When to use:** Always for Phase 2. This is the standard Kubernetes pattern: one controller per concern. The GameServer controller manages Pod lifecycle; the DNS controller manages networking.

**Example:**
```go
// Source: Kubebuilder pattern + Gateway API types
// internal/controller/dns_controller.go

import (
    gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
    gamev1alpha1 "github.com/kterodactyl/kterodactyl/api/v1alpha1"
)

type DNSReconciler struct {
    client.Client
    Scheme       *runtime.Scheme
    Recorder     record.EventRecorder
    BaseDomain   string  // e.g., "example.com"
    GatewayName  string  // Name of the Gateway resource to attach HTTPRoutes to
    GatewayNs    string  // Namespace of the Gateway
}

func (r *DNSReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    gs := &gamev1alpha1.GameServer{}
    if err := r.Get(ctx, req.NamespacedName, gs); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    // Only create networking for Ready/Allocated servers
    if gs.Status.State != gamev1alpha1.GameServerStateReady &&
       gs.Status.State != gamev1alpha1.GameServerStateAllocated {
        return ctrl.Result{}, nil
    }

    // Build DNS name: <game>.<username>.<baseDomain>
    owner := gs.Labels[util.LabelOwner]
    dnsName := fmt.Sprintf("%s.%s.%s", gs.Spec.GameType, owner, r.BaseDomain)

    // Ensure Service for the GameServer
    if err := r.ensureService(ctx, gs); err != nil {
        return ctrl.Result{}, err
    }

    // Ensure HTTPRoute pointing to the Service
    if err := r.ensureHTTPRoute(ctx, gs, dnsName); err != nil {
        return ctrl.Result{}, err
    }

    // Update GameServer status with connection info
    return r.updateConnectionInfo(ctx, gs, dnsName)
}
```

### Pattern 2: Per-GameServer Service + HTTPRoute

**What:** For each GameServer, the DNS controller creates:
1. A ClusterIP Service selecting the GameServer's Pod (for HTTPRoute backend)
2. An HTTPRoute with the explicit hostname `game.username.domain.com`
3. Optionally a NodePort Service for raw TCP/UDP game traffic

**When to use:** For every GameServer that reaches Ready state.

**Example:**
```go
// Source: Gateway API v1 spec + controller-runtime CreateOrUpdate
func (r *DNSReconciler) ensureHTTPRoute(ctx context.Context, gs *gamev1alpha1.GameServer, hostname string) error {
    route := &gatewayv1.HTTPRoute{
        ObjectMeta: metav1.ObjectMeta{
            Name:      gs.Name,
            Namespace: gs.Namespace,
        },
    }

    _, err := controllerutil.CreateOrUpdate(ctx, r.Client, route, func() error {
        // Set owner reference so HTTPRoute is cleaned up when GameServer is deleted
        if err := ctrl.SetControllerReference(gs, route, r.Scheme); err != nil {
            return err
        }

        hn := gatewayv1.Hostname(hostname)
        ns := gatewayv1.Namespace(r.GatewayNs)
        sectionName := gatewayv1.SectionName("http")

        route.Spec = gatewayv1.HTTPRouteSpec{
            CommonRouteSpec: gatewayv1.CommonRouteSpec{
                ParentRefs: []gatewayv1.ParentReference{
                    {
                        Name:        gatewayv1.ObjectName(r.GatewayName),
                        Namespace:   &ns,
                        SectionName: &sectionName,
                    },
                },
            },
            Hostnames: []gatewayv1.Hostname{hn},
            Rules: []gatewayv1.HTTPRouteRule{
                {
                    BackendRefs: []gatewayv1.HTTPBackendRef{
                        {
                            BackendRef: gatewayv1.BackendRef{
                                BackendObjectReference: gatewayv1.BackendObjectReference{
                                    Name: gatewayv1.ObjectName(gs.Name),
                                    Port: ptrTo(gatewayv1.PortNumber(8080)), // HTTP management port
                                },
                            },
                        },
                    },
                },
            },
        }
        return nil
    })
    return err
}
```

### Pattern 3: GameServer Status Update with Connection Info (NET-04)

**What:** After creating networking resources, update the GameServer status with the DNS name and port information so users can see connection info.

**When to use:** After HTTPRoute and Service are created for a Ready GameServer.

**Example:**
```go
func (r *DNSReconciler) updateConnectionInfo(ctx context.Context, gs *gamev1alpha1.GameServer, dnsName string) (ctrl.Result, error) {
    fresh := &gamev1alpha1.GameServer{}
    if err := r.Get(ctx, types.NamespacedName{Name: gs.Name, Namespace: gs.Namespace}, fresh); err != nil {
        return ctrl.Result{}, err
    }

    // Set the DNS address in status
    fresh.Status.Address = dnsName

    // Ports come from the Service (NodePort for game traffic)
    // This is populated from the actual allocated NodePort
    if err := r.Status().Update(ctx, fresh); err != nil {
        return ctrl.Result{}, err
    }

    return ctrl.Result{}, nil
}
```

### Pattern 4: Cross-Namespace HTTPRoute with ReferenceGrant

**What:** GameServer pods live in per-user namespaces (`user-<username>`), but the Gateway lives in the operator namespace. HTTPRoutes in user namespaces can attach to a Gateway in another namespace (allowed by Gateway `allowedRoutes`). However, if the HTTPRoute references a Service in the same namespace as the GameServer, no ReferenceGrant is needed since both the route and backend are co-located.

**When to use:** Always. This is the standard multi-tenant Gateway API pattern.

**Example:**
```yaml
# Gateway in kterodactyl-system namespace
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: kterodactyl-gateway
  namespace: kterodactyl-system
spec:
  gatewayClassName: eg  # or nginx, cilium, etc.
  listeners:
  - name: http
    protocol: HTTP
    port: 80
    allowedRoutes:
      namespaces:
        from: Selector
        selector:
          matchLabels:
            kterodactyl.io/managed-by: kterodactyl
  - name: https
    protocol: HTTPS
    port: 443
    tls:
      mode: Terminate
      certificateRefs:
      - name: wildcard-cert
    allowedRoutes:
      namespaces:
        from: Selector
        selector:
          matchLabels:
            kterodactyl.io/managed-by: kterodactyl

---
# HTTPRoute in user namespace (no ReferenceGrant needed for same-namespace backend)
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: minecraft-server-1
  namespace: user-alice
spec:
  parentRefs:
  - name: kterodactyl-gateway
    namespace: kterodactyl-system
    sectionName: http
  hostnames:
  - minecraft.alice.example.com
  rules:
  - backendRefs:
    - name: minecraft-server-1
      port: 8080
```

### Anti-Patterns to Avoid

- **Single wildcard HTTPRoute for all servers:** Gateway API does not support `*.*.domain.com`. Creating one wildcard route cannot route to different backends based on the second subdomain label. Create one HTTPRoute per GameServer instead.

- **hostPort for game traffic:** While Agones uses hostPort, it restricts pod scheduling (only one pod per port per node) and requires a port allocator. NodePort Services are simpler for a homelab-scale project and avoid scheduling constraints.

- **DNS controller modifying GameServer spec:** The DNS controller should only read the GameServer CR and update its status. It should not modify spec fields. Each controller owns its own resources.

- **Creating HTTPRoute before GameServer is Ready:** Only create networking resources when the GameServer has a running Pod. Creating routes for non-existent backends wastes resources and can cause health check failures.

- **Putting the HTTPRoute in the operator namespace:** The HTTPRoute should be in the same namespace as the GameServer and its Service, to avoid needing ReferenceGrant for the backend Service reference.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| DNS record provisioning | Custom DNS API client per provider (Route53, Cloudflare, etc.) | ExternalDNS with `--source=gateway-httproute` | ExternalDNS supports 30+ DNS providers, handles record lifecycle, is battle-tested |
| TLS certificate management | Custom ACME client or manual cert management | cert-manager with wildcard cert per user subdomain | cert-manager handles ACME challenges, auto-renewal, and cert distribution |
| HTTP load balancing and routing | Custom reverse proxy or nginx config generation | Gateway controller (Envoy Gateway, NGINX Gateway Fabric, Cilium) | These are production-grade, support Gateway API natively, and handle all the edge cases |
| Port allocation tracking | Custom port pool tracking CRD | Kubernetes NodePort Service (auto-allocated by kube-proxy) | Kubernetes already manages NodePort allocation in the 30000-32767 range, avoiding conflicts |
| DNS name construction | Inline string formatting scattered across codebase | Centralized `util/networking.go` with `GameServerDNSName()` | Single source of truth for DNS naming pattern; easy to change format later |

**Key insight:** The Kubernetes ecosystem already has mature solutions for every networking concern in this phase. The operator's job is to create the right Kubernetes resources (Service, HTTPRoute) with the right labels and annotations -- ExternalDNS and the Gateway controller handle the actual networking.

## Common Pitfalls

### Pitfall 1: Nested Wildcard Hostnames in Gateway API

**What goes wrong:** Developer creates a single HTTPRoute with `*.*.domain.com` expecting it to match `game.username.domain.com`. The Gateway controller rejects it because nested wildcards are not supported.
**Why it happens:** Confusion between DNS wildcards (which support `*.*.domain.com` at some providers) and Gateway API hostname matching (which only allows `*.domain.com`).
**How to avoid:** Create one HTTPRoute per GameServer with the explicit hostname (`minecraft.alice.domain.com`). This is actually better because ExternalDNS creates a precise DNS record per hostname.
**Warning signs:** HTTPRoute stuck in "not accepted" state; Gateway controller logs showing "invalid hostname" errors.

### Pitfall 2: Cross-Namespace Gateway Attachment Denied

**What goes wrong:** HTTPRoutes in user namespaces cannot attach to the Gateway because `allowedRoutes` is not configured to permit cross-namespace attachment.
**Why it happens:** Gateway `allowedRoutes.namespaces.from` defaults to `Same` (only routes in the Gateway's namespace can attach). For multi-tenant setups, it must be set to `Selector` or `All`.
**How to avoid:** Configure the Gateway with `allowedRoutes.namespaces.from: Selector` and use a label selector that matches user namespaces (e.g., `kterodactyl.io/managed-by: kterodactyl`). The namespace labels are already set by Phase 1's `ensureUserNamespace()`.
**Warning signs:** HTTPRoute status shows `Accepted: False` with reason `NotAllowedByListeners`.

### Pitfall 3: Service Not Found for HTTPRoute Backend

**What goes wrong:** HTTPRoute references a Service that does not exist yet, causing the Gateway controller to report the route as invalid.
**Why it happens:** The DNS controller creates the HTTPRoute before creating the Service, or creates them in the wrong order.
**How to avoid:** Use `controllerutil.CreateOrUpdate` for both Service and HTTPRoute, and create the Service BEFORE the HTTPRoute in the reconciliation loop.
**Warning signs:** HTTPRoute status shows `ResolvedRefs: False`; backend shows "service not found".

### Pitfall 4: ExternalDNS Not Watching HTTPRoutes

**What goes wrong:** DNS records are never created despite HTTPRoutes existing with valid hostnames.
**Why it happens:** ExternalDNS is deployed with `--source=service` or `--source=ingress` but not `--source=gateway-httproute`. Or ExternalDNS lacks RBAC to read HTTPRoute resources.
**How to avoid:** Ensure ExternalDNS deployment includes `--source=gateway-httproute` and has RBAC for `httproutes` in the `gateway.networking.k8s.io` API group. Document this as a prerequisite in operator installation guide.
**Warning signs:** ExternalDNS logs show no HTTPRoute discoveries; `external-dns` pod events show RBAC errors.

### Pitfall 5: Port Confusion Between HTTP and Game Traffic

**What goes wrong:** Users try to connect their game client to the HTTP port (80/443), or the HTTPRoute is configured to route to the game protocol port instead of the HTTP management port.
**Why it happens:** Game servers expose multiple ports (e.g., Minecraft: TCP 25565 for game, potentially HTTP 8080 for RCON web). The operator conflates these.
**How to avoid:** Clearly separate concerns: HTTPRoute routes HTTP traffic (web console, future API) to an HTTP port on the Service. Game protocol traffic (TCP/UDP) is exposed via a separate NodePort Service. The GameServer status shows both the DNS name (for HTTP) and the NodePort (for game client connections).
**Warning signs:** Game clients cannot connect despite DNS resolving correctly; HTTP 502 errors when Gateway tries to proxy game protocol traffic.

### Pitfall 6: DNS Propagation Delay Not Communicated to Users

**What goes wrong:** User sees "Ready" state and tries to connect immediately, but DNS has not propagated yet. They get NXDOMAIN errors and think the system is broken.
**Why it happens:** DNS record creation and propagation take 30-120 seconds depending on provider and TTL settings.
**How to avoid:** Add a status condition on the GameServer CR (e.g., `DNSReady`) that tracks whether the DNS record has been provisioned. Set `external-dns.alpha.kubernetes.io/ttl: "60"` on HTTPRoutes for fast propagation. Show the IP address as a fallback connection option.
**Warning signs:** User reports "server shows Ready but I can't connect"; DNS lookups for the hostname return NXDOMAIN.

### Pitfall 7: NetworkPolicy Blocking Gateway Traffic

**What goes wrong:** The Gateway controller cannot reach GameServer pods because the Phase 1 NetworkPolicy blocks traffic from outside the user namespace (except operator namespace and kube-system).
**Why it happens:** Phase 1's NetworkPolicy is restrictive by design. The Gateway controller's data plane pods run in a different namespace (e.g., `envoy-gateway-system`).
**How to avoid:** Update the NetworkPolicy to also allow ingress from the Gateway controller's namespace. Add this as a configurable value (different Gateway implementations use different namespaces).
**Warning signs:** 502 errors from the Gateway; Gateway controller health checks failing for backends.

## Code Examples

Verified patterns from official sources:

### DNS Name Construction Utility

```go
// internal/util/networking.go

package util

import "fmt"

// GameServerDNSName constructs the DNS name for a GameServer.
// Pattern: <gameType>.<owner>.<baseDomain>
// Example: minecraft.alice.example.com
func GameServerDNSName(gameType, owner, baseDomain string) string {
    return fmt.Sprintf("%s.%s.%s", gameType, owner, baseDomain)
}

// Networking-related label and annotation constants.
const (
    // AnnotationDNSName stores the computed DNS name for the GameServer.
    AnnotationDNSName = "kterodactyl.io/dns-name"

    // LabelHTTPRouteOwner links an HTTPRoute back to its GameServer.
    LabelHTTPRouteOwner = "kterodactyl.io/gameserver"
)
```

### Creating a ClusterIP Service for HTTPRoute Backend

```go
// Source: controller-runtime CreateOrUpdate pattern
func (r *DNSReconciler) ensureService(ctx context.Context, gs *gamev1alpha1.GameServer) error {
    svc := &corev1.Service{
        ObjectMeta: metav1.ObjectMeta{
            Name:      gs.Name,
            Namespace: gs.Namespace,
        },
    }

    _, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
        if err := ctrl.SetControllerReference(gs, svc, r.Scheme); err != nil {
            return err
        }

        svc.Labels = map[string]string{
            util.LabelManagedBy:      util.ManagedByValue,
            util.LabelHTTPRouteOwner: gs.Name,
        }

        // Build service ports from GameServer spec
        var ports []corev1.ServicePort
        for _, p := range gs.Spec.Ports {
            ports = append(ports, corev1.ServicePort{
                Name:       p.Name,
                Port:       p.ContainerPort,
                TargetPort: intstr.FromInt32(p.ContainerPort),
                Protocol:   p.Protocol,
            })
        }

        svc.Spec = corev1.ServiceSpec{
            Type: corev1.ServiceTypeClusterIP,
            Selector: map[string]string{
                util.LabelOwner: gs.Labels[util.LabelOwner],
                util.LabelGame:  gs.Spec.GameType,
                util.LabelName:  util.AppNameValue,
            },
            Ports: ports,
        }
        return nil
    })
    return err
}
```

### Registering the DNS Controller with the Manager

```go
// Source: Kubebuilder SetupWithManager pattern
func (r *DNSReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&gamev1alpha1.GameServer{}).
        Owns(&corev1.Service{}).
        Owns(&gatewayv1.HTTPRoute{}).
        WithEventFilter(predicate.Or(
            predicate.GenerationChangedPredicate{},
            predicate.AnnotationChangedPredicate{},
        )).
        Named("dns").
        Complete(r)
}
```

### Adding Gateway API Scheme Registration

```go
// cmd/main.go - add to existing scheme registration
import (
    gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func init() {
    // Existing scheme registrations...
    utilruntime.Must(gatewayv1.Install(scheme))
}
```

### ExternalDNS Annotations on HTTPRoute

```go
// Set ExternalDNS annotations on the HTTPRoute for DNS provisioning
route.Annotations = map[string]string{
    "external-dns.alpha.kubernetes.io/ttl":      "60",
    // Hostname is automatically read from spec.hostnames by ExternalDNS
    // Target comes from the Gateway's external-dns.alpha.kubernetes.io/target annotation
}
```

### RBAC Markers for the DNS Controller

```go
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes/status,verbs=get
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Ingress API + Ingress Controller | Gateway API (HTTPRoute) + Gateway Controller | GA: Oct 2023; Ingress retired March 2026 | Must use Gateway API for new projects. Ingress NGINX has no security updates after Nov 2026. |
| `*.*.domain.com` nested DNS wildcards | Per-resource HTTPRoute with explicit hostname | Gateway API v1.0 spec | Nested wildcards were never supported in Gateway API. Design around per-resource routes. |
| hostPort for game server port exposure (Agones pattern) | NodePort Service per GameServer | N/A (both valid) | NodePort is simpler for homelab scale; hostPort better for thousands of servers needing minimal latency |
| Manual DNS record management | ExternalDNS with Gateway API source | ExternalDNS v0.14+ added gateway-httproute source | Fully automated DNS lifecycle tied to HTTPRoute resources |
| Ingress annotations for ExternalDNS | Separate annotation placement (target on Gateway, others on Route) | ExternalDNS Gateway API source design | Annotations split between Gateway (target) and Route (TTL, provider-specific) |

**Deprecated/outdated:**
- **Ingress API:** Retired March 2026. Do not use for new projects.
- **ExternalDNS `--source=ingress`:** Still works but should be replaced with `--source=gateway-httproute` for new deployments.
- **TCPRoute/UDPRoute (v1alpha2):** Still experimental. For game server TCP/UDP traffic, use NodePort Services directly rather than waiting for these to mature.

## Open Questions

1. **Game Traffic Exposure Strategy (NodePort vs LoadBalancer vs hostPort)**
   - What we know: NodePort is the simplest approach for homelab scale. Each game server gets a Service with a NodePort in the 30000-32767 range. Players connect to `<node-ip>:<nodeport>`.
   - What's unclear: Should the DNS name resolve to the game traffic NodePort, or only to the HTTP Gateway? For game clients, they need the specific port number regardless. The DNS name provides the hostname, but the port must be communicated separately.
   - Recommendation: Use the DNS name (`game.username.domain.com`) for the human-readable address. The GameServer status exposes both the DNS name and the NodePort number. Game clients connect to `game.username.domain.com:30XXX`. The Gateway handles HTTP traffic on standard ports (80/443) for future web console access. For Phase 2, focus on NodePort Services and status updates. Revisit when Phase 7 (Console) needs HTTP routing.

2. **Gateway Controller Prerequisite**
   - What we know: The operator creates HTTPRoute resources, but a Gateway controller (Envoy Gateway, NGINX Gateway Fabric, Cilium, etc.) must be installed in the cluster to process them.
   - What's unclear: Which Gateway controller to recommend or require. Different controllers have different installation methods and feature sets.
   - Recommendation: Document this as a prerequisite. Do not bundle a Gateway controller. Provide sample Gateway YAML for popular implementations (Envoy Gateway, NGINX Gateway Fabric). Test with Envoy Gateway in CI (it is the reference implementation).

3. **cert-manager Integration Timing**
   - What we know: HTTPS requires TLS certificates. cert-manager can provision wildcard certs (`*.alice.domain.com`) via DNS-01 ACME challenge.
   - What's unclear: Whether to implement cert-manager integration in Phase 2 or defer to a later phase. The wildcard cert + cert-manager + ExternalDNS + DNS-01 challenge interaction is complex (see Pitfall 10 in PITFALLS.md).
   - Recommendation: Defer TLS/cert-manager to Phase 11 (Helm Packaging) or create a dedicated sub-phase. Phase 2 focuses on HTTP (port 80) routing and DNS. This reduces scope and avoids the cert-manager/ExternalDNS TXT record conflict pitfall.

4. **BaseDomain Configuration**
   - What we know: The DNS pattern requires a configurable base domain (e.g., `example.com`).
   - What's unclear: Where to store this configuration -- in the admin ConfigMap, as an environment variable, or as a CRD field.
   - Recommendation: Add `baseDomain` and `gatewayName` to the existing `kterodactyl-admin-config` ConfigMap. The DNS controller reads these on each reconciliation (same pattern as Phase 1's AdminConfig). This keeps configuration centralized and changeable without operator restart.

5. **NetworkPolicy Update for Gateway Traffic**
   - What we know: Phase 1's NetworkPolicy blocks ingress from outside the user namespace (except operator namespace and kube-system). The Gateway controller's data plane needs to reach game server pods.
   - What's unclear: The Gateway controller namespace varies by implementation (e.g., `envoy-gateway-system`, `nginx-gateway`, etc.).
   - Recommendation: Add a configurable `gatewayNamespace` to the admin ConfigMap. Update the NetworkPolicy to also allow ingress from this namespace. Default to `envoy-gateway-system`.

## Sources

### Primary (HIGH confidence)
- [Gateway API Spec Reference](https://gateway-api.sigs.k8s.io/reference/spec/) - Hostname regex pattern, wildcard rules, HTTPRoute spec
- [Gateway API HTTPRoute](https://gateway-api.sigs.k8s.io/api-types/httproute/) - Route attachment, hostname matching, cross-namespace
- [Gateway API TCP Routing Guide](https://gateway-api.sigs.k8s.io/guides/tcp/) - TCPRoute (experimental), Gateway listener config
- [Gateway API ReferenceGrant](https://gateway-api.sigs.k8s.io/api-types/referencegrant/) - Cross-namespace backend references
- [sigs.k8s.io/gateway-api v1.4.1 Go package](https://pkg.go.dev/sigs.k8s.io/gateway-api) - Go types, import paths, API versions
- [ExternalDNS Gateway API Route Sources](https://kubernetes-sigs.github.io/external-dns/latest/docs/sources/gateway-api/) - Annotation placement, hostname discovery, RBAC
- [ExternalDNS Annotations Reference](https://kubernetes-sigs.github.io/external-dns/latest/docs/annotations/annotations/) - TTL, target, provider-specific annotations
- [Agones GameServer Specification](https://agones.dev/site/docs/reference/gameserver/) - Port policies (Dynamic, Static, Passthrough)
- [Kubebuilder Watching Resources](https://book.kubebuilder.io/reference/watching-resources) - For, Owns, Watches patterns
- [controller-runtime handler package](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/handler) - EnqueueRequestForOwner

### Secondary (MEDIUM confidence)
- [Kong: Sending Traffic Across Namespaces with Gateway API](https://konghq.com/blog/engineering/sending-traffic-across-namespaces-with-gateway-api) - Cross-namespace patterns
- [OneUpTime: How to Implement Kubernetes Gateway API](https://oneuptime.com/blog/post/2026-01-19-kubernetes-gateway-api-ingress-successor/view) - Gateway API migration guide
- [OneUpTime: Kubernetes External DNS Setup](https://oneuptime.com/blog/post/2026-01-19-kubernetes-external-dns-automatic-records/view) - ExternalDNS with Gateway API
- [Alibaba Cloud: Agones Series Part 2 - Address and Port](https://www.alibabacloud.com/blog/agones-series-part-2-address-and-port-of-the-game-server_599427) - Game server networking patterns
- [AWS: How to Route UDP Traffic into Kubernetes](https://aws.amazon.com/blogs/containers/how-to-route-udp-traffic-into-kubernetes/) - UDP exposure strategies
- [Kubernetes NodePort Dynamic and Static Allocation](https://kubernetes.io/blog/2023/05/11/nodeport-dynamic-and-static-allocation/) - NodePort allocation strategy

### Tertiary (LOW confidence)
- [GitHub: rmb938/hostport-allocator](https://github.com/rmb938/hostport-allocator) - Alternative hostPort allocation operator (not verified for production use)
- [Kelsey Hightower: dynamic-ports-tutorial](https://github.com/kelseyhightower/dynamic-ports-tutorial) - Prototype only, not production pattern

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Gateway API v1 is GA, ExternalDNS is kubernetes-sigs, Go module versions verified on pkg.go.dev
- Architecture: HIGH - Pattern follows standard Kubernetes operator practices (separate controllers, owned resources, CreateOrUpdate)
- Pitfalls: HIGH - Wildcard limitation verified against official Gateway API spec regex; cross-namespace patterns verified against official docs
- Port strategy: MEDIUM - NodePort is well-understood but the "best" approach for game servers at scale is debatable; NodePort is correct for homelab scope

**Research date:** 2026-02-10
**Valid until:** 2026-04-10 (Gateway API is stable; ExternalDNS evolves slowly)
