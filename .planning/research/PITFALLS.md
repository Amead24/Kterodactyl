# Pitfalls Research

**Domain:** K8s-native game server management panel
**Researched:** 2026-02-09
**Confidence:** HIGH

## Critical Pitfalls

### Pitfall 1: CRD Storage Version Removal Without Migration

**What goes wrong:**
Removing a CRD version that is listed as a stored version on existing CRDs causes immediate data loss. All existing custom resources stored in that version become inaccessible or corrupted.

**Why it happens:**
Developers treat CRD versioning like API versioning and assume removing an old version is safe after deprecation. They don't realize that etcd still stores resources in the old schema format.

**How to avoid:**
1. Introduce new CRD version while keeping storage version stable
2. Create StorageVersionMigration to convert all stored resources from old to new version
3. Verify all resources migrated using `kubectl get <crd> -o jsonpath='{.items[*].metadata.managedFields[*].apiVersion}'`
4. Update CRD status to mark old version as non-stored
5. Only then remove old version from CRD spec

**Warning signs:**
- CRD updates showing "stored version in use" errors
- Resources returning old API versions unexpectedly
- Version field in CRD status shows versions not marked as stored

**Phase to address:**
Phase 1 (Core CRD Design) - Implement StorageVersionMigration strategy from start

---

### Pitfall 2: Missing Conversion Webhooks for Schema Changes

**What goes wrong:**
Schema changes across CRD versions break without conversion webhooks. Old stored resources can't be retrieved in new versions, causing 500 errors. Even if old versions aren't actively served, conversion hooks are required to convert stored old-version CRs in etcd to the latest version.

**Why it happens:**
Teams underestimate the need for conversion logic, assuming structural schema migrations handle it automatically. They don't realize Kubernetes needs explicit instructions to convert between versions.

**How to avoid:**
- Implement hub-and-spoke conversion model: all versions convert through one internal "hub" version
- Add conversion webhook server alongside operator from v1alpha1 → v1beta1 transition
- Test conversion in both directions (old→new and new→old) for roundtrip fidelity
- Use kubebuilder markers: `// +kubebuilder:storageversion` and conversion webhook boilerplate

**Warning signs:**
- Error "conversion webhook for <crd> not found"
- Resources can't be listed after CRD upgrade
- `kubectl get` works but API calls return conversion errors

**Phase to address:**
Phase 1 (Core CRD Design) - Scaffold conversion webhook infrastructure even if v1alpha1 doesn't need it yet

---

### Pitfall 3: Non-Idempotent Reconciliation Logic

**What goes wrong:**
Controllers that aren't idempotent create duplicate resources, fail on retries, or get stuck in crash loops. Reconciliation can be triggered multiple times for the same resource, and non-idempotent logic causes divergence between desired and actual state.

**Why it happens:**
Developers write reconciliation like event handlers ("something changed, do X") instead of declarative state enforcement ("desired state is Y, make it so"). Event-driven selective reconciliation is tempting but violates controller-runtime design principles.

**How to avoid:**
1. Always reconcile ALL resources, regardless of triggering event
2. Read current state, compare to desired state, calculate diff, apply changes
3. Make operations idempotent: creating existing resources = no-op, deleting missing resources = no-op
4. Use `controllerutil.CreateOrUpdate()` instead of separate Create/Update calls
5. Avoid storing state in memory - always read from API server

**Warning signs:**
- Duplicate game server pods created on every reconciliation
- "AlreadyExists" errors in controller logs
- Resources requiring manual deletion to unstick reconciliation
- Reconciliation succeeds once but fails on retry

**Phase to address:**
Phase 1 (Operator Core) - Code review checklist item: "Is this reconciliation idempotent?"

---

### Pitfall 4: UDP Port Allocation Conflicts and Exhaustion

**What goes wrong:**
Game servers can't bind to ports, preventing players from connecting. Kubernetes lacks automatic hostPort assignment - ports must be specified by user, causing conflicts. With hostNetwork: true, if ReplicaController specifies 3 replicas on 2 nodes with fixed port 9999, only 2 pods succeed.

**Why it happens:**
Teams use hostNetwork or hostPort for low latency without implementing port allocation strategy. They assume Kubernetes handles port conflicts like it does with ClusterIP services.

**How to avoid:**
- Implement dynamic port allocation (like Agones DynamicPort strategy)
- Use port pool per node tracked in CRD status (e.g., NodePortPool CRD)
- Prefer host port forwarding over hostNetwork for namespace isolation and security
- Pre-allocate port ranges per game type to avoid exhaustion
- Track allocated ports in GameServer status to prevent double-booking
- Set pod anti-affinity for game servers requiring same port on different nodes

**Warning signs:**
- Pods stuck in "ContainerCreating" with port binding errors
- Game server pods assigned to nodes then immediately evicted
- Port conflict errors in kubelet logs
- CNI plugin doesn't support hostPort (like Terway)

**Phase to address:**
Phase 2 (Networking) - Design port allocation strategy before implementing GameServer reconciliation

---

### Pitfall 5: Inadequate Multi-Tenant Resource Isolation

**What goes wrong:**
One user's game server becomes a "noisy neighbor" consuming excessive CPU/memory, causing other users' game servers to lag or crash. In worst cases, tenant A can access or modify tenant B's game servers or data.

**Why it happens:**
Teams rely solely on Kubernetes namespaces for isolation, which provide logical separation but not OS-level or resource isolation. They don't implement ResourceQuotas, LimitRanges, or NetworkPolicies.

**How to avoid:**
1. Namespace per user (not per game server) - `<username>-games` namespace
2. ResourceQuota per namespace with CPU/memory limits based on user tier
3. LimitRange to set default resource requests/limits for game server pods
4. NetworkPolicy to prevent cross-namespace pod communication (except system namespaces)
5. PodSecurityPolicy/PodSecurityAdmission to prevent privileged containers
6. Consider node selectors for paid tiers (sole tenant nodes) vs free tiers (shared nodes)
7. Use cgroups enforcement via Kubernetes QoS classes (Guaranteed > Burstable > BestEffort)

**Warning signs:**
- Game server performance degrades when other servers on same node are active
- OOMKilled pods when node reaches capacity (no limits set)
- Cross-tenant network communication possible when testing with netcat
- Users can create game servers that use more resources than their tier allows

**Phase to address:**
Phase 1 (Operator Core) - Implement namespace-per-user pattern and ResourceQuota webhook validation
Phase 3 (Multi-tenancy) - Add NetworkPolicies and tier-based resource allocation

---

### Pitfall 6: Over-Templatized Helm Charts

**What goes wrong:**
Helm charts become unmaintainable complexity nightmares with excessive conditionals, nested loops, and template functions handling every edge case. Debugging template rendering errors takes hours. New contributors can't understand the chart.

**Why it happens:**
Teams over-customize charts early, adding options for hypothetical future scenarios. Each deployment environment gets its own conditional logic path instead of using multiple values files.

**How to avoid:**
- Keep templates simple - push variability into values.yaml
- Resist adding conditionals for "maybe we'll need this" features
- Use `values.yaml` comments to document every option
- Break complex charts into sub-charts (e.g., operator, UI, dependencies)
- Avoid template functions spanning 20+ lines - extract to helper chart
- Use `helm lint` and `helm template --debug` in CI
- Test with multiple values files (prod, dev, homelab) to prevent logic sprawl

**Warning signs:**
- Template files exceeding 200 lines
- More than 3 levels of nested `{{- if }}` conditionals
- Template rendering takes >5 seconds
- Pull requests changing charts require 30+ minute reviews
- "It works in dev but not prod" despite identical Docker images

**Phase to address:**
Phase 2 (Helm Chart) - Start with minimal chart, add complexity only when proven necessary
Phase 4+ (Refinement) - Regular chart complexity audits and refactoring sprints

---

### Pitfall 7: Cardinality Explosion in Prometheus Metrics

**What goes wrong:**
Prometheus crashes due to excessive time series from high-cardinality labels. Grafana dashboards timeout. Query performance degrades to unusable. Metrics include unique identifiers like game-server-uuid or player-session-id as labels, creating millions of time series.

**Why it happens:**
Developers treat Prometheus labels like log fields, adding high-cardinality dimensions (user IDs, pod names, IP addresses) without understanding cardinality impact. Kubernetes' ephemeral nature (pods changing state frequently) multiplies the problem.

**How to avoid:**
1. Use low-cardinality labels: game_type, server_state, user_tier (NOT user_id, pod_name, session_id)
2. Keep label values bounded: use "OTHER" bucket for rare values
3. Set up cardinality monitoring: use Cardinality Explorer dashboard (Grafana ID 11304)
4. Implement metric label guidelines in operator code review
5. Use recording rules to pre-aggregate high-cardinality metrics
6. Consider horizontal scaling: shard Prometheus by namespace or game type

**Warning signs:**
- Prometheus pod OOMKilled or restarting frequently
- Cardinality warnings in Prometheus logs: "high cardinality metric"
- Queries taking >30s for simple aggregations
- Metrics scraped once but never queried (dead series accumulation)
- Time series count growing unbounded over time

**Phase to address:**
Phase 3 (Observability) - Design metrics schema with cardinality budgets per metric before implementation
Phase 4+ (Scale) - Add cardinality alerts and dashboards to detect explosion early

---

### Pitfall 8: Operator Leader Election Split-Brain

**What goes wrong:**
Two operator replicas both think they're leader, reconciling the same resources simultaneously. This creates race conditions: duplicate game servers spawned, conflicting status updates, and resource thrashing. In network partition scenarios, old leader doesn't step down while new leader is elected.

**Why it happens:**
Leader election configured with insufficient tolerations for node failures. When leader pod is on unresponsive node, pod isn't deleted automatically - old leader keeps lock while new leader can't be elected. Default garbage collection timing causes 5+ minute delays in re-election.

**How to avoid:**
1. Configure `node.kubernetes.io/unreachable` and `node.kubernetes.io/not-ready` tolerations with short `tolerationSeconds` (30-60s)
2. Set lease duration and renew deadline appropriately: 15s duration, 10s renew deadline, 2s retry period
3. Implement staleness detection: release lock if no heartbeat in 30s
4. Test network partition scenarios: simulate node isolation and verify re-election timing
5. Add leader election metrics: leader_election_leader (boolean), leader_election_transitions_total (counter)
6. Use Kubernetes 1.26+ coordinated leader election improvements

**Warning signs:**
- Multiple operator pods log "starting reconciliation" for same resource
- GameServer status updates conflicting (lastUpdated timestamp ping-ponging)
- Two operators incrementing same counter causing skipped numbers
- Operator unresponsive for 5+ minutes during node drain
- "lease already held by another holder" errors with long delays

**Phase to address:**
Phase 1 (Operator Core) - Configure leader election with production-ready timings from start
Phase 4 (HA Testing) - Chaos engineering tests for network partitions and node failures

---

### Pitfall 9: Graceful Shutdown State Loss for Game Servers

**What goes wrong:**
Players lose progress when game servers are terminated. Pod receives SIGTERM but game server doesn't save world state before shutdown. 30-second grace period expires, Kubernetes sends SIGKILL, and data is lost mid-write.

**Why it happens:**
Game server containers don't handle SIGTERM properly. Developers assume terminationGracePeriodSeconds default (30s) is sufficient, but saving large world state takes 60+ seconds. No preStop hook implemented to coordinate shutdown sequence.

**How to avoid:**
1. Game server application must handle SIGTERM: stop accepting connections, finish in-flight requests, save state, close DB connections
2. Set appropriate `terminationGracePeriodSeconds` per game type (60-120s for large world games, 30s for session games)
3. Implement preStop hook to trigger graceful save: `preStop: exec: command: ["/bin/sh", "-c", "kill -TERM 1 && sleep 10"]`
4. Expose readiness probe that returns false during shutdown to stop new traffic
5. Add backup mechanism: operator watches for terminated pods and triggers emergency save
6. Test shutdown: send SIGTERM manually and verify state saves within grace period

**Warning signs:**
- Player complaints about lost progress after "server restart"
- Database corruption errors on game server restart
- Game server processes showing SIGKILL in logs (not SIGTERM)
- terminationGracePeriodSeconds timing out (grace period exceeded)
- File writes incomplete (partial JSON files, truncated SQLite databases)

**Phase to address:**
Phase 2 (Game Server Integration) - Document and test SIGTERM handling requirements for each game definition
Phase 3 (Backup Integration) - Implement operator-triggered backup on termination

---

### Pitfall 10: Wildcard DNS and Cert-Manager TXT Record Conflicts

**What goes wrong:**
Cert-manager DNS-01 challenges fail or timeout. Both ExternalDNS and cert-manager create conflicting TXT records for `_acme-challenge.domain.com`. With split-horizon DNS, cert-manager updates external DNS but validates against internal DNS, causing perpetual "Waiting for DNS-01 challenge propagation" state.

**Why it happens:**
Both tools manage TXT records without coordination. Teams implement `<game>.<username>.domain.com` pattern requiring wildcard certs (`*.*.domain.com`) but DNS provider doesn't support nested wildcards. Split-horizon DNS setup isn't accounted for in cert-manager configuration.

**How to avoid:**
1. Specify unique ownership IDs: ExternalDNS `txt-owner-id` and cert-manager `--cluster-resource-namespace` to prevent conflicts
2. Use DNS provider with recursive ACME challenge support (Route53, CloudFlare work well; some don't)
3. For split-horizon: configure cert-manager to use external DNS resolver: `--dns01-recursive-nameservers=8.8.8.8:53`
4. Consider certificate strategy: one wildcard per user (`*.alice.domain.com`) instead of one for all (`*.*.domain.com`)
5. Implement DNS propagation checks: monitor `_acme-challenge` TXT record visibility before challenge
6. Set longer DNS propagation wait time: `propagationTimeout: 180s` in issuer config

**Warning signs:**
- Cert-manager challenges stuck in "pending" state for >5 minutes
- Multiple TXT records for same `_acme-challenge` subdomain
- External DNS resolver shows different TXT records than internal
- Certificate renewals failing but manual `certbot` succeeds
- "too many authorizations" errors from Let's Encrypt due to retries

**Phase to address:**
Phase 2 (DNS/Ingress) - Design DNS and certificate strategy with split-horizon and wildcard implications
Phase 3 (External DNS) - Integration testing with both ExternalDNS and cert-manager before production

---

## Technical Debt Patterns

Shortcuts that seem reasonable but create long-term problems.

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Single controller managing GameServer + User + Backup CRDs | Faster initial development, shared code | Violates Single Responsibility, reconciliation complexity, hard to test, difficult to extend | Never - split from start |
| Hardcoded port ranges (30000-30100) instead of dynamic allocation | Simple, works for MVP | Port exhaustion at scale, no per-node tracking, conflicts with other services | Homelab-only deployments with <10 servers |
| No conversion webhooks until v1 | Skip webhook server complexity early | Breaking changes block upgrades, forced downtime for migrations, data loss risk | Never for greenfield - webhook server is boilerplate |
| Storing allocated state in operator memory instead of CRD status | Faster reconciliation, no API writes | State lost on operator restart, split-brain in HA, no observability | Single-replica dev environments only |
| ResourceQuotas set manually instead of tier-based automation | Quick setup, flexible | Configuration drift, human error, no enforcement of tier limits | POC with 1-2 users |
| Backup to local PV instead of S3 | No cloud dependencies, simpler | No disaster recovery, node failure = data loss, no cross-cluster restore | Development only, never staging/prod |
| `terminationGracePeriodSeconds: 30` default for all games | Standard Kubernetes behavior | Data loss for stateful games, player complaints | Stateless session-based games with no persistence |
| Community game definitions without sandboxing/validation | Fast community growth, more games | Security risk (malicious images), support burden (broken definitions), quality issues | Curated contributions with manual review |

---

## Integration Gotchas

Common mistakes when connecting to external services.

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| SteamCMD | Assuming SteamCMD in container is legal for all games | Verify each game's server hosting license - some prohibit specific hosting types; document licensing in game definition schema; implement license metadata field |
| S3-compatible storage | Not handling eventual consistency for backup listings | Use strong consistency S3 regions (us-east-1, eu-central-1); implement retry logic with exponential backoff; verify object exists after PutObject before marking backup complete |
| Prometheus | Scraping all GameServer pods individually | Use Prometheus Operator ServiceMonitor with label selectors; aggregate metrics at namespace level; implement federation for multi-cluster |
| External-DNS | Assuming DNS changes are immediate | Set `external-dns.alpha.kubernetes.io/ttl: "60"` annotation; implement readiness gates waiting for DNS propagation; health check DNS resolution before marking GameServer ready |
| Docker registries | Public rate limits (Docker Hub 100 pulls/6hrs) | Use registry mirrors; implement ImagePullSecrets for authenticated pulls; consider self-hosted registry or cloud provider registry (ECR, GCR, ACR) |
| Cloud provider load balancers | Creating LoadBalancer service per GameServer | Use single LoadBalancer with NodePort, track port allocations; or use MetalLB for on-prem; cloud LBs cost $15-30/mo each - unsustainable at scale |
| Cert-manager | DNS provider credentials in plaintext ConfigMap | Store in Secret; use ExternalSecrets Operator to sync from Vault/AWS Secrets Manager; rotate credentials periodically; use IAM roles where possible (AWS IRSA, GCP Workload Identity) |
| RBAC | Granting cluster-admin to operator ServiceAccount | Use least privilege: namespace-scoped Role for GameServers, ClusterRole only for CRDs/Nodes; use `--leader-elect-resource-namespace` to scope lease to single namespace |

---

## Performance Traps

Patterns that work at small scale but fail as usage grows.

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Listing all GameServers in cluster on every reconciliation | Reconciliation latency increases linearly with server count; API server throttling | Use field selectors and label selectors; implement pagination for large lists; cache informers; watch specific resources instead of list-all | >1000 GameServers |
| Creating dedicated namespace per GameServer | Simple isolation model, clear boundaries | Namespace sprawl (k8s degrades >5000 namespaces); etcd pressure; apiserver watch connection limits | >2000 GameServers |
| Syncing all user game servers to CRD status array | Complete view in single kubectl get | CRD size limit (1.5MB); etcd write amplification; reconciliation loops on large updates | >500 servers per user |
| No index on GameServer.spec.gameType in informer cache | Query convenience | Full cache scan on every allocation request; O(n) lookup time | >5000 GameServers |
| Backup all game servers to S3 in parallel on timer | Simple cron schedule | S3 rate limits; network saturation; node disk I/O bottleneck; thundering herd | >100 concurrent backups |
| Single Prometheus instance for all metrics | Standard setup | Cardinality limits; OOM crashes; query timeouts; sampling bias when dropping metrics | >1M active time series |
| Using finalizers for every GameServer cleanup | Guaranteed cleanup before deletion | Finalizer backlog during mass deletion; operator bottleneck; slow namespace deletion | >1000 simultaneous deletions |
| Reconciling all GameServers on User CRD update | Consistency guarantee | Reconciliation storm; API throttling; reconcile queue backlog | User with >100 GameServers |

---

## Security Mistakes

Domain-specific security issues beyond general web security.

| Mistake | Risk | Prevention |
|---------|------|------------|
| Allowing user-specified container images in GameServer CRD | Remote code execution; cryptomining; data exfiltration; DDoS participation | Restrict to curated registry; implement admission webhook validating allowed images; maintain allow-list of community-verified images; scan images with Trivy/Clair in CI |
| No network policy between GameServer pods | Player A connects to Player B's server directly; cross-tenant attacks; packet sniffing | Default-deny NetworkPolicy; allow only from ingress and within same namespace; block pod-to-pod for different users |
| Backup secrets stored in GameServer namespace | Users with namespace access can read S3 credentials | Store backup credentials in operator namespace; use ServiceAccount token projection; implement Secrets Store CSI Driver; use cloud IAM roles (IRSA, Workload Identity) |
| Game server runs as root in container | Container escape = node compromise; privilege escalation | Set `runAsNonRoot: true` and `runAsUser: 1000` in PodSecurityPolicy; use restricted PSA profile; enforce in admission webhook |
| Allowing privileged containers in game definitions | Node kernel access; access host devices; bypass seccomp/AppArmor | PodSecurityPolicy with `privileged: false`; PSA enforce restricted profile; reject privileged in validating webhook |
| No resource limits on game servers | DoS via resource exhaustion; noisy neighbor; cluster instability | LimitRange per namespace; admission webhook requiring requests/limits; set defaults in game definition template |
| User-provided environment variables injected without sanitization | Credential injection; SSRF; command injection in startup scripts | Validate env vars against allow-list; strip sensitive patterns (AWS_, KUBE_); use admission webhook to filter |
| Allowing LoadBalancer service type | Cost explosion; IP exhaustion; cloud quota limits | Admission webhook blocking LoadBalancer in user namespaces; use NodePort or ingress-based routing |

---

## UX Pitfalls

Common user experience mistakes in this domain.

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| GameServer "Pending" state with no reason | User doesn't know why server won't start; creates support tickets | Add detailed status conditions: ImagePullBackOff, InsufficientResources, PortConflict; surface in UI prominently |
| Backup restoration as kubectl apply YAML | Non-technical users can't restore; manual error-prone process | Implement restore button in UI; create BackupRestore CRD; handle restoration logic in operator |
| DNS propagation delay invisible to users | User tries to connect immediately after creation, sees DNS_PROBE_FINISHED_NXDOMAIN, thinks system is broken | Show "DNS propagating (30s remaining)" status in UI; poll DNS resolution; show IP as fallback connection option |
| No feedback during long operations (SteamCMD download) | User sees "Creating..." for 10 minutes, assumes it's stuck, cancels and retries | Stream logs to UI via WebSocket; show progress: "Downloading CS:GO (2.3GB / 15GB)"; percentage complete in GameServer status |
| Error messages like "reconciliation failed" | Meaningless to end users | User-friendly messages: "Game server couldn't start because your account has reached the 5 server limit. Upgrade to Pro for unlimited servers." |
| No confirmation on destructive actions | User deletes game server thinking they can restore; data loss | "Delete server? This will permanently delete world data. Type server name to confirm." confirmation UI; implement soft-delete with 7-day retention |
| Port number required in connection string | Confusing (why do I need :27015?); error-prone (users forget) | Use SRV records where supported; detect game type and auto-select port; provide copy-to-clipboard connection string |
| No visibility into game server console | Debugging startup failures requires kubectl logs | Embedded console viewer in UI; tail last 100 lines; real-time streaming; search/filter capability |

---

## "Looks Done But Isn't" Checklist

Things that appear complete but are missing critical pieces.

- [ ] **CRD Versioning:** Often missing StorageVersionMigration implementation - verify conversion webhooks exist and roundtrip testing is in CI
- [ ] **Operator HA:** Often missing leader election timeout configuration and network partition testing - verify tolerationSeconds and lease renewal timings configured
- [ ] **Port Allocation:** Often missing port conflict detection and exhaustion handling - verify port pool tracking and allocation logic prevent double-booking
- [ ] **Graceful Shutdown:** Often missing preStop hooks and appropriate terminationGracePeriodSeconds - verify SIGTERM handling tested with actual game servers
- [ ] **Backup Reliability:** Often missing corruption detection and restore testing - verify backups are restorable and periodic restoration tests in CI
- [ ] **Multi-tenancy:** Often missing NetworkPolicies and ResourceQuotas - verify tenant isolation with penetration testing (can tenant A access tenant B?)
- [ ] **DNS Propagation:** Often missing readiness gates waiting for DNS - verify GameServer not marked "Ready" until DNS resolves from external resolver
- [ ] **Metrics Cardinality:** Often missing cardinality budgets and alerts - verify Prometheus metrics don't use high-cardinality labels and monitoring set up
- [ ] **RBAC Least Privilege:** Often using cluster-admin when namespace scope sufficient - verify operator ServiceAccount has minimal permissions
- [ ] **Error Messages:** Often generic errors without actionable guidance - verify error messages are user-friendly and include next steps
- [ ] **Finalizer Cleanup:** Often missing finalizer timeout/fallback logic - verify stuck resources can be cleaned up without manual kubectl patch
- [ ] **Helm Chart Testing:** Often tested only in minikube - verify chart deploys to multi-node cluster with production-like networking
- [ ] **Game Definition Schema:** Often missing license metadata and resource requirements - verify community contributions include necessary metadata for safety/billing
- [ ] **TLS Certificate Automation:** Often missing renewal monitoring - verify cert-manager renewals work and alerts fire on failure
- [ ] **S3 Backup Cleanup:** Often missing retention policy and orphan cleanup - verify old backups deleted and orphaned files cleaned up

---

## Recovery Strategies

When pitfalls occur despite prevention, how to recover.

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| CRD storage version removed prematurely | HIGH | Restore old CRD version from git; apply to cluster; manually patch stored resources to new version; create StorageVersionMigration; verify all resources accessible; remove old version |
| Port allocation conflicts on all nodes | MEDIUM | Drain nodes one by one; restart operator to reset port allocation state; implement port allocation CRD tracking allocated ports; redeploy game servers with new allocations |
| Prometheus cardinality explosion | MEDIUM | Identify high-cardinality metrics with `topk(20, count by (__name__)({__name__=~".+"}))` query; temporarily drop problematic metrics via relabeling; fix metric labels in code; roll out new operator version; reset Prometheus data |
| Leader election split-brain | LOW | Delete both operator pods to force clean re-election; verify lease resource cleared; check for competing operators in different namespaces; ensure only one operator deployment exists |
| Wildcard certificate expired | LOW | Delete Certificate resource to force reissue; check cert-manager logs for DNS propagation issues; manually verify TXT record created; trigger renewal with `cmctl renew <cert>`; check Let's Encrypt rate limits |
| Finalizers stuck preventing deletion | LOW | Identify stuck resources with `kubectl get <resource> -o json | jq '.metadata.finalizers'`; verify controller for finalizer is running; remove finalizer with `kubectl patch <resource> -p '{"metadata":{"finalizers":null}}' --type=merge`; manually clean up dependent resources |
| Backup corruption detected | HIGH | Attempt restore from previous backup; check S3 bucket versioning; verify game server saved state before operator backup triggered; contact user about data loss; implement corruption detection in backup process |
| Multi-tenant isolation breach | HIGH | Immediately isolate affected namespaces with NetworkPolicy; audit access logs for unauthorized access; rotate credentials; review RBAC permissions; notify affected users; implement additional security controls |
| Helm chart rendering failure in production | MEDIUM | Roll back to previous chart version; test new chart with production values in staging; identify problematic template logic; fix and re-test; implement chart testing in CI with all values files |
| GameServer stuck in Terminating state | LOW | Check for finalizers; verify operator is processing deletion; manually remove finalizer if operator is functioning; force delete pod if needed: `kubectl delete pod <pod> --force --grace-period=0`; investigate why operator didn't clean up |

---

## Pitfall-to-Phase Mapping

How roadmap phases should address these pitfalls.

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| CRD storage version removal | Phase 1: Core CRD Design | CI tests StorageVersionMigration before CRD version removal allowed |
| Missing conversion webhooks | Phase 1: Core CRD Design | Integration test creates v1alpha1 resource, reads as v1beta1, verifies fields match |
| Non-idempotent reconciliation | Phase 1: Operator Core | Unit test: reconcile twice with same input, verify identical output and no errors |
| UDP port allocation conflicts | Phase 2: Networking | Load test: create 100 GameServers simultaneously, verify no port conflicts in status |
| Multi-tenant resource isolation | Phase 1: Operator Core + Phase 3: Multi-tenancy | Penetration test: from tenant A pod, attempt to access tenant B GameServer (should fail) |
| Over-templatized Helm charts | Phase 2: Helm Chart + continuous | Complexity metrics in CI: fail if template >200 lines or >3 nested conditionals |
| Cardinality explosion | Phase 3: Observability | Alert on >100k active time series; dashboard showing top-10 metrics by cardinality |
| Leader election split-brain | Phase 1: Operator Core + Phase 4: HA Testing | Chaos test: network partition node with leader, verify new leader elected <60s |
| Graceful shutdown state loss | Phase 2: Game Server Integration | Integration test: SIGTERM game server pod, verify state saved before terminationGracePeriod expires |
| Wildcard DNS + cert-manager conflicts | Phase 2: DNS/Ingress + Phase 3: External DNS | Integration test: create GameServer, verify DNS resolves and TLS cert issued <5min |
| Works in minikube, fails in production | All phases | CI runs full test suite on multi-node cluster (kind/k3s with 3 nodes minimum) |
| Port management with hostPort/hostNetwork | Phase 2: Networking | Test: deploy to cluster with CNI that doesn't support hostPort, verify graceful degradation |
| RBAC over-privileged operator | Phase 1: Operator Core | RBAC audit: verify operator has no cluster-admin, uses namespace-scoped Roles where possible |
| Finalizers blocking deletion | Phase 1: Operator Core + Phase 3: Reliability | Test: operator pod killed during GameServer deletion, verify finalizer removed after restart |
| Backup to S3 reliability | Phase 3: Backup Integration | Test: corrupt backup file, trigger restore, verify corruption detected with actionable error |

---

## Sources

### CRD Versioning and Operator Best Practices
- [Kubernetes CRD: the versioning joy - DEV Community](https://dev.to/jotak/kubernetes-crd-the-versioning-joy-6g0)
- [Operator Best Practices | Operator SDK](https://sdk.operatorframework.io/docs/best-practices/best-practices/)
- [K8s CRD Versioning - NAIS Handbook](https://handbook.nais.io/technical/k8s_crd_versioning/)
- [Common recommendations and suggestions | Operator SDK](https://sdk.operatorframework.io/docs/best-practices/common-recommendation/)
- [Good Practices - The Kubebuilder Book](https://book.kubebuilder.io/reference/good-practices)

### Game Server Networking
- [Agones Series – Part 2: Address and Port of the Game Server - Alibaba Cloud](https://www.alibabacloud.com/blog/agones-series-part-2-address-and-port-of-the-game-server_599427)
- [How to route UDP traffic into Kubernetes | Amazon Web Services](https://aws.amazon.com/blogs/containers/how-to-route-udp-traffic-into-kubernetes/)
- [Network | OpenKruise](https://openkruise.io/kruisegame/user-manuals/network)
- [Frequently Asked Questions | Agones](https://agones.dev/site/docs/faq/)

### Multi-Tenancy and Security
- [Multi-tenancy | Kubernetes](https://kubernetes.io/docs/concepts/security/multi-tenancy/)
- [Best Practices for Achieving Isolation in Kubernetes Multi-Tenant Environments | Loft Labs](https://www.vcluster.com/blog/best-practices-for-achieving-isolation-in-kubernetes-multi-tenant-environments)
- [Role Based Access Control Good Practices | Kubernetes](https://kubernetes.io/docs/concepts/security/rbac-good-practices/)
- [Kubernetes RBAC Security Pitfalls – Certitude Blog](https://certitude.consulting/blog/en/kubernetes-rbac-security-pitfalls/)

### Helm Charts
- [Helm Charts: Development Practices from a Programmer's Perspective](https://carlosneto.dev/blog/2025/2025-02-25-helm-best-practices/)
- [Best Practices | Helm](https://helm.sh/docs/chart_best_practices/)

### Agones and Game Server Management
- [Troubleshooting | Agones](https://agones.dev/site/docs/guides/troubleshooting/)
- [Hands-On With Agones and Google Cloud Game Servers](https://www.fairwinds.com/blog/hands-on-with-agones-google-cloud-game-servers)

### Kubernetes Operations
- [StatefulSets | Kubernetes](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/)
- [A Practical Guide to Kubernetes Stateful Backup and Recovery - The New Stack](https://thenewstack.io/a-practical-guide-to-kubernetes-stateful-backup-and-recovery/)
- [Simplifying DNS Automation with ExternalDNS and cert-manager](https://komodor.com/blog/simplifying-dns-automation-with-externaldns-and-cert-manager/)
- [Using Finalizers to Control Deletion | Kubernetes](https://kubernetes.io/blog/2021/05/14/using-finalizers-to-control-deletion/)
- [How to Manage Kubernetes Finalizers and Stuck Resources | Kubernetes Recipe Book](https://kubernetes.recipes/recipes/troubleshooting/stuck-resources-finalizers/)

### Prometheus and Observability
- [How to manage high cardinality metrics in Prometheus and Kubernetes | Grafana Labs](https://grafana.com/blog/2022/10/20/how-to-manage-high-cardinality-metrics-in-prometheus-and-kubernetes/)
- [Optimizing Prometheus Storage: Handling High-Cardinality Metrics at Scale | Platform Engineers](https://medium.com/@platform.engineers/optimizing-prometheus-storage-handling-high-cardinality-metrics-at-scale-31140c92a7e4)

### Graceful Shutdown
- [Gracefully Terminating Pods in Kubernetes: Handling SIGTERM | Amila De Silva](https://jaadds.medium.com/gracefully-terminating-pods-in-kubernetes-handling-sigterm-fb0d60c7e983)
- [Kubernetes best practices: terminating with grace | Google Cloud Blog](https://cloud.google.com/blog/products/containers-kubernetes/kubernetes-best-practices-terminating-with-grace)

### Leader Election and High Availability
- [Implementing Leader Election in Kubernetes: A Practical Approach | Manish Kaushik](https://medium.com/@manish.kaushik_52893/implementing-leader-election-in-kubernetes-a-practical-approach-for-single-pod-execution-34aa5fb003dd)
- [Leader election in Kubernetes using client-go | Mayank Shah](https://itnext.io/leader-election-in-kubernetes-using-client-go-a19cbe7a9a85)

### Minikube vs Production
- [MiniKube VS Production : Need Production - Discuss Kubernetes](https://discuss.kubernetes.io/t/minikube-vs-production-need-production/19249)
- [What Is Minikube? | Sysdig](https://www.sysdig.com/learn-cloud-native/what-is-minikube)

### OpenAPI and CRD Schema
- [How Kubernetes Validates Custom Resources | Daniel Mangum](https://danielmangum.com/posts/how-kubernetes-validates-custom-resources/)
- [Future of CRDs: Structural Schemas | Kubernetes](https://kubernetes.io/blog/2019/06/20/crd-structural-schema/)

---
*Pitfalls research for: Kterodactyl - Kubernetes-native game server management panel*
*Researched: 2026-02-09*
