# Phase 11: Helm Packaging - Research

**Researched:** 2026-02-12
**Domain:** Helm chart packaging for Kubernetes operator (Kubebuilder project)
**Confidence:** HIGH

## Summary

Kterodactyl is a Kubebuilder v4.11.1 operator with two CRDs (GameServer, Backup), a single binary combining an operator controller-manager and an API server, multiple RBAC resources, a ConfigMap-based AdminConfig, several Secrets (JWT signing key, S3 credentials, SMTP credentials), ServiceMonitor for Prometheus, and network policies. All of these resources currently exist as kustomize manifests in the `config/` directory. This phase converts them into a Helm chart installable via a single `helm install` command.

The recommended approach is to create the Helm chart manually (not via helmify or the Kubebuilder helm/v2-alpha plugin) because: (1) the project has non-standard resources like the AdminConfig ConfigMap and API server Service that need careful templating, (2) helmify overwrites all template files on each run making manual customization fragile, and (3) the Kubebuilder helm/v2-alpha plugin is alpha-quality and places CRDs in templates/ rather than the conventional crds/ directory. A hand-crafted chart gives full control over the values.yaml schema, conditional logic for Gateway API vs Ingress, and storage class configurability.

CRDs should be placed in the `crds/` directory (Helm convention) for automatic installation ordering. CRDs in this directory are installed before templates, cannot be templated, and are never upgraded or deleted by Helm. This is acceptable because CRD upgrades for a v1alpha1 operator are manual administrative operations regardless.

**Primary recommendation:** Create a hand-crafted Helm chart at `chart/` in the project root, using `apiVersion: v2` in Chart.yaml, with CRDs in `crds/`, all operator resources as templates, and a well-structured `values.yaml` that exposes Gateway API vs Ingress toggle, storage class, domain, resource limits, and optional feature flags (metrics, SMTP, S3 backups).

## Standard Stack

### Core
| Tool | Version | Purpose | Why Standard |
|------|---------|---------|--------------|
| Helm | v4.1.0 (latest) / v3.x compatible | Chart packaging and installation | Industry standard K8s package manager; v4 supports apiVersion v2 charts |
| Chart API | v2 | Chart.yaml apiVersion | Required for Helm 3+, fully compatible with Helm 4 |

### Supporting
| Tool | Version | Purpose | When to Use |
|------|---------|---------|-------------|
| helm lint | built-in | Chart validation | Run before every commit to catch template errors |
| helm template | built-in | Dry-run rendering | Debug template output without cluster access |
| helm-unittest | latest | Unit testing chart templates | Validate conditional logic, value overrides |
| ct (chart-testing) | latest | Lint and install testing | CI/CD pipeline validation |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Hand-crafted chart | helmify | Helmify overwrites templates on re-run; manual customization lost; less control over values.yaml schema |
| Hand-crafted chart | Kubebuilder helm/v2-alpha | Alpha quality; places CRDs in templates/ not crds/; opinionated structure may not fit project needs |
| crds/ directory | templates/ for CRDs | Templates allow CRD upgrades via helm upgrade but violate Helm convention and risk ordering issues |
| Single chart | Separate CRD chart | Overkill for a single operator with 2 CRDs; adds deployment complexity |

## Architecture Patterns

### Recommended Chart Structure
```
chart/
  Chart.yaml                    # Chart metadata (apiVersion: v2)
  values.yaml                   # Default configuration values
  values.schema.json            # JSON Schema for values validation (optional but recommended)
  templates/
    _helpers.tpl                # Template helper functions (labels, names, selectors)
    NOTES.txt                   # Post-install usage instructions
    namespace.yaml              # Namespace (conditional, for non-OLM installs)
    deployment.yaml             # Operator + API server Deployment
    service.yaml                # API server Service (port 8080)
    service-metrics.yaml        # Metrics Service (port 8443)
    serviceaccount.yaml         # ServiceAccount
    clusterrole.yaml            # Main operator ClusterRole
    clusterrolebinding.yaml     # Main ClusterRoleBinding
    role-leader-election.yaml   # Leader election Role (namespaced)
    rolebinding-leader-election.yaml  # Leader election RoleBinding
    clusterrole-metrics-auth.yaml     # Metrics auth ClusterRole
    clusterrolebinding-metrics-auth.yaml  # Metrics auth ClusterRoleBinding
    clusterrole-metrics-reader.yaml   # Metrics reader ClusterRole
    configmap-admin.yaml        # AdminConfig ConfigMap
    servicemonitor.yaml         # Prometheus ServiceMonitor (conditional)
    networkpolicy.yaml          # Metrics NetworkPolicy (conditional)
  crds/
    game.kterodactyl.io_gameservers.yaml  # GameServer CRD (plain YAML, no templating)
    game.kterodactyl.io_backups.yaml      # Backup CRD (plain YAML, no templating)
```

### Pattern 1: Helm Standard Labels
**What:** Consistent labels on all resources using _helpers.tpl
**When to use:** Every template file
**Example:**
```yaml
# templates/_helpers.tpl
{{- define "kterodactyl.labels" -}}
helm.sh/chart: {{ include "kterodactyl.chart" . }}
app.kubernetes.io/name: {{ include "kterodactyl.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
control-plane: controller-manager
{{- end }}

{{- define "kterodactyl.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kterodactyl.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
control-plane: controller-manager
{{- end }}

{{- define "kterodactyl.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "kterodactyl.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{- define "kterodactyl.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "kterodactyl.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "kterodactyl.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}
```

### Pattern 2: Conditional Gateway API vs Ingress
**What:** Toggle networking mode via values.yaml
**When to use:** When the operator creates HTTPRoutes or Ingress resources
**Example:**
```yaml
# values.yaml
adminConfig:
  networking:
    # "gateway" (default) or "ingress" (future)
    mode: gateway
    baseDomain: ""
    gateway:
      name: "kterodactyl-gateway"
      namespace: "kterodactyl-system"
      controllerNamespace: "envoy-gateway-system"
```
Note: The operator itself handles Gateway API/Ingress resource creation at runtime via AdminConfig. The Helm chart's role is to populate the AdminConfig ConfigMap with the correct values and ensure the operator's RBAC includes the right permissions for whichever mode is selected.

### Pattern 3: Deployment with All Configurable Args
**What:** Operator Deployment with configurable command-line args, env vars, and volume mounts
**When to use:** The main deployment.yaml template
**Example:**
```yaml
# templates/deployment.yaml
spec:
  template:
    spec:
      containers:
      - name: manager
        image: "{{ .Values.image.repository }}:{{ .Values.image.tag | default .Chart.AppVersion }}"
        imagePullPolicy: {{ .Values.image.pullPolicy }}
        command:
        - /manager
        args:
        - --leader-elect
        - --health-probe-bind-address=:8081
        - --metrics-bind-address=:8443
        - --api-bind-address=:8080
        {{- with .Values.manager.extraArgs }}
        {{- toYaml . | nindent 8 }}
        {{- end }}
        env:
        - name: OPERATOR_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        ports:
        - name: api
          containerPort: 8080
          protocol: TCP
        - name: metrics
          containerPort: 8443
          protocol: TCP
        - name: health
          containerPort: 8081
          protocol: TCP
```

### Pattern 4: AdminConfig ConfigMap from Values
**What:** Render AdminConfig ConfigMap from structured values.yaml
**When to use:** configmap-admin.yaml template
**Example:**
```yaml
# templates/configmap-admin.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kterodactyl-admin-config
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "kterodactyl.labels" . | nindent 4 }}
data:
  maxServersGlobal: {{ .Values.adminConfig.limits.maxServersGlobal | quote }}
  maxServersPerUser: {{ .Values.adminConfig.limits.maxServersPerUser | quote }}
  baseDomain: {{ .Values.adminConfig.networking.baseDomain | quote }}
  gatewayName: {{ .Values.adminConfig.networking.gateway.name | quote }}
  gatewayNamespace: {{ .Values.adminConfig.networking.gateway.namespace | quote }}
  gatewayControllerNamespace: {{ .Values.adminConfig.networking.gateway.controllerNamespace | quote }}
  {{- /* Storage configuration */ -}}
  modStorageClass: {{ .Values.adminConfig.storage.modStorageClass | quote }}
  modStorageSize: {{ .Values.adminConfig.storage.modStorageSize | quote }}
  {{- /* S3 backup configuration */ -}}
  {{- if .Values.adminConfig.backup.enabled }}
  backupS3Endpoint: {{ .Values.adminConfig.backup.s3.endpoint | quote }}
  backupS3Bucket: {{ .Values.adminConfig.backup.s3.bucket | quote }}
  backupS3Region: {{ .Values.adminConfig.backup.s3.region | quote }}
  backupS3UseSSL: {{ .Values.adminConfig.backup.s3.useSSL | quote }}
  backupRetentionCount: {{ .Values.adminConfig.backup.retentionCount | quote }}
  {{- end }}
```

### Anti-Patterns to Avoid
- **Hardcoding namespace in templates:** Use `{{ .Release.Namespace }}` everywhere; never hardcode `kterodactyl-system`
- **Templating CRDs:** CRD files in `crds/` directory MUST be plain YAML; never add Go template directives
- **Putting secrets in values.yaml defaults:** Never put actual passwords/keys in values.yaml; provide empty defaults and document that users must create Secrets
- **Using `helm.sh/hook` for CRDs:** The old crd-install hook pattern from Helm 2 is deprecated; use the crds/ directory
- **Duplicating RBAC from kustomize verbatim:** Kustomize uses namePrefix; Helm templates should use `{{ include "kterodactyl.fullname" . }}` helper instead
- **Missing NOTES.txt:** Always include post-install instructions showing users how to verify the installation and access the panel

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Label generation | Custom label strings in each template | `_helpers.tpl` with named templates | Consistency, DRY, follows Helm conventions |
| Name prefixing | Manual name construction per resource | `{{ include "kterodactyl.fullname" . }}` helper | Handles nameOverride, fullnameOverride, 63-char truncation |
| CRD installation ordering | Pre-install hooks or manual kubectl apply | Helm `crds/` directory | Automatic, reliable, Helm-native CRD ordering |
| Values validation | Manual validation in NOTES.txt | `values.schema.json` (JSON Schema) | Helm validates values before rendering templates |
| Chart testing | Manual helm template inspection | helm-unittest plugin | Repeatable, CI-friendly, catches regressions |

**Key insight:** Helm has built-in conventions for almost every common chart pattern. Fighting these conventions (e.g., templating CRDs, custom naming schemes, manual ordering) creates maintenance burden and confuses users who expect standard Helm behavior.

## Common Pitfalls

### Pitfall 1: CRDs Not Updated on helm upgrade
**What goes wrong:** After upgrading the chart with new CRD fields, CRs using new fields are rejected because the CRD in the cluster is stale.
**Why it happens:** Helm never upgrades CRDs from the `crds/` directory by design.
**How to avoid:** Document in NOTES.txt and README that CRD upgrades require manual `kubectl apply -f crds/`. Include a Makefile target `make install-crds` that applies CRDs from the chart directory. For v1alpha1, this is acceptable -- CRD changes are expected to be manual.
**Warning signs:** New fields in GameServer/Backup spec cause validation errors after chart upgrade.

### Pitfall 2: Namespace Mismatch Between Chart and Hardcoded Values
**What goes wrong:** Operator deployed to namespace X but AdminConfig references `kterodactyl-system` hardcoded in defaults.
**Why it happens:** The operator reads OPERATOR_NAMESPACE env var and uses it to find ConfigMaps/Secrets. If values.yaml defaults reference a different namespace, things break.
**How to avoid:** Use `{{ .Release.Namespace }}` for all namespace references. Set `OPERATOR_NAMESPACE` via downward API (`metadata.namespace` fieldRef) so it always matches the deployment namespace.
**Warning signs:** Operator logs show "ConfigMap not found" or "Secret not found" errors.

### Pitfall 3: Forgotten API Server Service
**What goes wrong:** The operator Deployment exists but users cannot reach the web UI or REST API.
**Why it happens:** The existing kustomize config has a metrics Service but no API server Service (port 8080). The chart must add this.
**How to avoid:** Create a dedicated Service template for the API server on port 8080, separate from the metrics Service on port 8443.
**Warning signs:** `kubectl get svc -n kterodactyl-system` shows only the metrics service.

### Pitfall 4: RBAC Resources Missing the fullname Pattern
**What goes wrong:** Installing two releases in the same cluster causes ClusterRole/ClusterRoleBinding name collisions.
**Why it happens:** ClusterRoles are cluster-scoped; if names don't include the release name, multiple installations conflict.
**How to avoid:** Use `{{ include "kterodactyl.fullname" . }}-manager-role` pattern for all cluster-scoped resources. Include release namespace in ClusterRoleBinding subjects.
**Warning signs:** `helm install` in a second namespace fails with "already exists" errors on ClusterRole.

### Pitfall 5: SecurityContext Missing from Deployment Template
**What goes wrong:** Pod fails to start on clusters with restricted Pod Security Standards.
**Why it happens:** Forgetting to copy the securityContext from the existing kustomize deployment.
**How to avoid:** Include both pod-level and container-level securityContext in the Deployment template. Make them configurable via values.yaml but with secure defaults (runAsNonRoot: true, readOnlyRootFilesystem: true, drop ALL capabilities).
**Warning signs:** Pod stuck in `CreateContainerError` or `CrashLoopBackOff` with security policy violations.

### Pitfall 6: ConfigMap AdminConfig Name Must Be Exact
**What goes wrong:** Operator cannot find its config; falls back to defaults silently.
**Why it happens:** The operator code hardcodes `kterodactyl-admin-config` as the ConfigMap name. If the Helm chart generates a different name (e.g., with release prefix), the operator will not find it.
**How to avoid:** The AdminConfig ConfigMap name MUST be `kterodactyl-admin-config` (hardcoded in Go source at `internal/controller/gameserver_controller.go:67`). Do NOT apply the fullname prefix to this resource. Same applies to Secret names: `kterodactyl-jwt-signing-key` and `kterodactyl-s3-credentials`.
**Warning signs:** Operator starts fine but uses all default values regardless of ConfigMap contents.

### Pitfall 7: Games Directory Not Mounted
**What goes wrong:** Operator logs "failed to load game manifests" and crashes.
**Why it happens:** The Dockerfile copies `games/` to `/games` in the image. The manifest loader reads from `games/` (relative path). If the working directory in the container is not `/`, the loader won't find the directory.
**How to avoid:** Verify the Dockerfile's WORKDIR is `/` (it is). The Helm chart just needs the image to be correctly built with games baked in. No volume mount needed for the games directory since it's embedded in the container image.
**Warning signs:** Startup crash with "failed to load game manifests from games/ directory".

## Code Examples

Verified patterns from official Helm docs and project source:

### Complete values.yaml Structure
```yaml
# values.yaml - Kterodactyl Helm Chart

# -- Number of operator replicas (should be 1 with leader election)
replicaCount: 1

image:
  # -- Container image repository
  repository: ghcr.io/kterodactyl/kterodactyl
  # -- Image pull policy
  pullPolicy: IfNotPresent
  # -- Overrides the image tag (default is the chart appVersion)
  tag: ""

imagePullSecrets: []
nameOverride: ""
fullnameOverride: ""

serviceAccount:
  # -- Create a ServiceAccount
  create: true
  # -- Annotations for the ServiceAccount
  annotations: {}
  # -- ServiceAccount name override
  name: ""

# -- Manager (operator + API server) configuration
manager:
  # -- Extra command-line arguments for the manager binary
  extraArgs: []
  resources:
    limits:
      cpu: 500m
      memory: 128Mi
    requests:
      cpu: 10m
      memory: 64Mi
  # -- Pod-level security context
  podSecurityContext:
    runAsNonRoot: true
    seccompProfile:
      type: RuntimeDefault
  # -- Container-level security context
  securityContext:
    readOnlyRootFilesystem: true
    allowPrivilegeEscalation: false
    capabilities:
      drop:
      - ALL

# -- Node scheduling constraints
nodeSelector: {}
tolerations: []
affinity: {}

# -- API server Service configuration
apiService:
  # -- Service type (ClusterIP for internal, LoadBalancer for direct access)
  type: ClusterIP
  # -- API server port
  port: 8080

# -- Metrics configuration
metrics:
  # -- Enable metrics endpoint
  enabled: true
  service:
    # -- Metrics port
    port: 8443

# -- Prometheus ServiceMonitor
serviceMonitor:
  # -- Create a ServiceMonitor resource
  enabled: false
  # -- Additional labels for ServiceMonitor
  labels: {}
  # -- Scrape interval
  interval: ""
  # -- Scrape timeout
  scrapeTimeout: ""

# -- Network policies
networkPolicy:
  # -- Create NetworkPolicy for metrics traffic
  enabled: false

# -- AdminConfig values (populate the kterodactyl-admin-config ConfigMap)
adminConfig:
  limits:
    maxServersGlobal: "100"
    maxServersPerUser: "5"
  quota:
    cpuRequests: "4"
    cpuLimits: "8"
    memoryRequests: "8Gi"
    memoryLimits: "16Gi"
    pods: "5"
    pvcs: "5"
    storage: "50Gi"
  containerDefaults:
    cpu: "2"
    memory: "4Gi"
    requestCPU: "500m"
    requestMemory: "1Gi"
    maxCPU: "4"
    maxMemory: "8Gi"
    minCPU: "100m"
    minMemory: "128Mi"
  networking:
    baseDomain: ""
    gateway:
      name: "kterodactyl-gateway"
      namespace: ""  # defaults to Release.Namespace if empty
      controllerNamespace: "envoy-gateway-system"
  auth:
    jwtExpirationHours: "24"
    inviteExpirationHours: "72"
    registrationEnabled: "true"
    panelURL: ""
  smtp:
    host: ""
    port: "587"
    username: ""
    from: ""
  storage:
    modStorageClass: ""
    modStorageSize: "1Gi"
  backup:
    enabled: false
    s3:
      endpoint: ""
      bucket: "kterodactyl-backups"
      region: "us-east-1"
      useSSL: "false"
    retentionCount: "5"
```

### NOTES.txt Post-Install Instructions
```
# templates/NOTES.txt
Kterodactyl has been installed!

1. Get the API server URL:
{{- if eq .Values.apiService.type "ClusterIP" }}
  kubectl port-forward -n {{ .Release.Namespace }} svc/{{ include "kterodactyl.fullname" . }}-api {{ .Values.apiService.port }}:{{ .Values.apiService.port }}
  Then open: http://localhost:{{ .Values.apiService.port }}
{{- else if eq .Values.apiService.type "LoadBalancer" }}
  NOTE: It may take a few minutes for the LoadBalancer IP to be available.
  kubectl get svc -n {{ .Release.Namespace }} {{ include "kterodactyl.fullname" . }}-api -w
{{- end }}

2. Create the first admin user:
  The JWT signing key is auto-generated on first start.
  You can bootstrap the admin via the operator pod directly.

{{- if .Values.adminConfig.backup.enabled }}

3. S3 Backup Configuration:
  Create the S3 credentials Secret:
  kubectl create secret generic kterodactyl-s3-credentials \
    --namespace {{ .Release.Namespace }} \
    -from-literal=access-key=YOUR_ACCESS_KEY \
    --from-literal=secret-key=YOUR_SECRET_KEY
{{- end }}

{{- if .Values.adminConfig.smtp.host }}

4. SMTP Configuration:
  SMTP password must be provided via Secret:
  kubectl create secret generic kterodactyl-smtp-credentials \
    --namespace {{ .Release.Namespace }} \
    --from-literal=password=YOUR_SMTP_PASSWORD
{{- end }}

CRD Upgrades:
  Helm does not upgrade CRDs automatically. After chart upgrades with CRD changes:
  kubectl apply -f chart/crds/
```

### Chart.yaml
```yaml
apiVersion: v2
name: kterodactyl
description: Self-service game server management for Kubernetes
type: application
version: 0.1.0
appVersion: "0.1.0"
kubeVersion: ">=1.28.0"
keywords:
  - kubernetes
  - operator
  - game-server
  - pterodactyl
home: https://github.com/kterodactyl/kterodactyl
sources:
  - https://github.com/kterodactyl/kterodactyl
maintainers:
  - name: Tony Mead
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Helm 3 | Helm 4 (v4.1.0) | Nov 2025 | Chart apiVersion v2 still works; no chart structure changes needed |
| crd-install hooks (Helm 2) | crds/ directory (Helm 3+) | Helm 3.0 (2019) | Plain YAML CRDs, automatic ordering, no hooks |
| Manual YAML manifests | Helm chart with values.yaml | This phase | Single command install, configurable deployments |
| Kustomize overlays | Helm values overrides | This phase | Better UX for end users, standard packaging format |

**Deprecated/outdated:**
- `crd-install` hook: Removed in Helm 3; use crds/ directory instead
- Helm 2 Tiller: Removed; Helm 3+ is client-only
- Kubebuilder helm/v2-alpha CRD placement in templates/: Goes against Helm convention; use crds/ directory

## Open Questions

1. **Image registry and tag for initial release**
   - What we know: The Dockerfile builds the image, Makefile uses `controller:latest` default
   - What's unclear: What registry to push to (ghcr.io, Docker Hub, private)
   - Recommendation: Default to `ghcr.io/kterodactyl/kterodactyl` in values.yaml with `tag: ""` that defaults to chart appVersion. User overrides via `--set image.repository=...`

2. **Gateway resource creation in chart vs operator runtime**
   - What we know: The operator creates HTTPRoutes dynamically at runtime based on AdminConfig. The Gateway resource itself is assumed to exist.
   - What's unclear: Should the chart include an optional Gateway resource template, or should users create their own Gateway?
   - Recommendation: Do NOT include a Gateway resource in the chart. The Gateway is infrastructure managed by the cluster operator (e.g., Cilium Gateway API, Envoy Gateway). The chart only configures the AdminConfig to reference an existing Gateway. Document this in NOTES.txt.

3. **SMTP and S3 credential Secrets -- create via chart or document manual creation?**
   - What we know: Secret names are hardcoded in Go source (`kterodactyl-s3-credentials`, `kterodactyl-jwt-signing-key`). JWT key is auto-created by the operator if missing.
   - What's unclear: Should the chart template these Secrets with user-provided values, or instruct users to create them manually?
   - Recommendation: Do NOT template Secrets with credentials in the chart (security anti-pattern -- values would appear in Helm release metadata). Instead, document manual Secret creation in NOTES.txt. The JWT signing key is auto-provisioned by the operator.

4. **Homelab vs multi-node configuration differences**
   - What we know: Single-node Talos cluster with Cilium. No scheduling constraints needed for single node.
   - What's unclear: What specific multi-node features to enable
   - Recommendation: Expose `nodeSelector`, `tolerations`, and `affinity` as empty defaults in values.yaml. Single-node works with defaults (no constraints). Multi-node users add node selectors as needed. `replicaCount: 1` default with leader election enabled covers both cases.

## Sources

### Primary (HIGH confidence)
- [Helm Official Docs - Custom Resource Definitions](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/) - CRD best practices, crds/ directory behavior
- [Helm Official Docs - Charts](https://helm.sh/docs/topics/charts/) - Chart structure, CRD lifecycle
- [Helm 4 Released](https://helm.sh/blog/helm-4-released/) - Helm 4 changes, v2 chart compatibility
- [Helm 4 Overview](https://helm.sh/docs/overview/) - Breaking changes, migration
- Project source code: `config/` directory (all kustomize manifests examined)
- Project source code: `internal/controller/gameserver_controller.go` (AdminConfig struct, hardcoded names)
- Project source code: `internal/auth/jwt.go` (JWT signing key Secret name)
- Project source code: `cmd/main.go` (CLI flags, env vars, bootstrap sequence)

### Secondary (MEDIUM confidence)
- [Kubebuilder Helm v2-alpha Plugin](https://book.kubebuilder.io/plugins/available/helm-v2-alpha) - Alternative approach, CRD placement rationale
- [helmify](https://github.com/arttor/helmify) - Kustomize-to-Helm conversion tool capabilities and limitations
- [Helm CRD Installation and Upgrades](https://oneuptime.com/blog/post/2026-01-17-helm-crd-installation-upgrades/view) - CRD handling approaches comparison
- [Helm Releases - GitHub](https://github.com/helm/helm/releases) - Current Helm version (v4.1.0)

### Tertiary (LOW confidence)
- None - all findings verified with primary or secondary sources

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Helm chart structure is well-documented with stable conventions
- Architecture: HIGH - Chart structure follows established Helm best practices; project resources thoroughly examined
- Pitfalls: HIGH - CRD upgrade limitations, hardcoded names, namespace issues all verified in source code and official docs

**Research date:** 2026-02-12
**Valid until:** 2026-03-14 (30 days - Helm chart conventions are stable)
