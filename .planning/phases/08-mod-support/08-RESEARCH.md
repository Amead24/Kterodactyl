# Phase 8: Mod Support - Research

**Researched:** 2026-02-12
**Domain:** File upload, Kubernetes PersistentVolumeClaim management, pod exec file transfer, React file upload UI
**Confidence:** HIGH

## Summary

Phase 8 adds mod file upload capability to Kterodactyl. Users upload mod files (typically .jar files for Minecraft) through the UI, files are stored on a separate PersistentVolumeClaim (PVC) per game server, and the server automatically restarts after upload. This touches every layer of the stack: the CRD types (no current PVC/volume support), the controller (must create PVCs and mount them to pods), the API server (must handle multipart file uploads and copy files into the pod or PVC), and the frontend (must provide a file upload UI with progress feedback).

The core architectural decision is **how files move from the user's browser to the mod directory inside the game server container**. The two viable approaches are: (A) upload to the API server, then use exec-based tar streaming to copy into the running pod; or (B) upload to the API server, then write directly to a PVC via a helper mechanism. Approach A is recommended because it reuses the existing exec infrastructure (already proven by the console feature), works regardless of StorageClass capabilities, and does not require complex PVC access from the operator pod. The PVC is still used for persistence (mods survive pod restarts), but file transfer goes through tar-over-exec.

**Primary recommendation:** Use the existing exec infrastructure (remotecommand/SPDY) to stream uploaded mod files as tar archives into the game server pod's mod directory. Create a dedicated PVC per GameServer that mounts at a game-manifest-defined mod path (e.g., `/mods` for Minecraft). Add a `modPath` field to game manifests. The operator creates and owns the PVC, and the API server streams uploaded files into the running pod via tar-over-exec.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `archive/tar` | stdlib | Create tar archives for file streaming to pods | Used by kubectl cp internally; no external dependency needed |
| `k8s.io/client-go/tools/remotecommand` | v0.35.1 | SPDY exec for streaming tar into pods | Already in use for console feature; proven in this codebase |
| `net/http` (multipart) | stdlib | Parse multipart file uploads | Standard Go approach; no framework dependency |
| `react-dropzone` | ^14.3 | Drag-and-drop file upload in React | Most popular React file upload library; works with shadcn/ui |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `io` | stdlib | Pipe-based streaming (io.Pipe) | For connecting tar writer to exec stdin without buffering entire file |
| `path/filepath` | stdlib | Filename sanitization and validation | Prevent path traversal attacks on upload |
| `http.MaxBytesReader` | stdlib | Enforce upload size limits | Wrap request body before parsing multipart form |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| tar-over-exec | Init container + PVC direct write | More complex; requires pod restart to mount; can't add mods to running server |
| tar-over-exec | Shared volume between operator and game pod | Requires operator and game pod to share storage; breaks namespace isolation |
| react-dropzone | Native HTML5 file input | Less user-friendly; no drag-and-drop; no file preview |

### No New Go Dependencies Required
The Go side needs only standard library (`archive/tar`, `io`, `path/filepath`) plus already-imported packages (`k8s.io/client-go/tools/remotecommand`, `k8s.io/api/core/v1` for PVC types). No new Go dependencies to add to `go.mod`.

### Frontend Installation
```bash
cd web && npm install react-dropzone
```

## Architecture Patterns

### Recommended Changes to Existing Files

```
api/v1alpha1/
  gameserver_types.go          # (modify) no changes to CRD -- PVC is managed infrastructure
internal/
  api/
    handlers_mods.go           # (new) multipart upload handler, list/delete mod handlers
    request.go                 # (modify) no new request types needed -- multipart, not JSON
    routes.go                  # (modify) add mod routes under /{name}/mods
    server.go                  # (no change) already has all needed clients
  controller/
    gameserver_controller.go   # (modify) reconcilePod adds PVC + volume mount; add reconcilePVC
  manifest/
    manifest.go                # (modify) add ModPath field to GameManifest
games/minecraft/
    manifest.yaml              # (modify) add modPath: /mods
web/src/
  api/servers.ts               # (modify) add uploadMod, listMods, deleteMod functions
  api/client.ts                # (modify) add multipart fetch helper (no Content-Type header)
  types/api.ts                 # (modify) add ModFileResponse type
  hooks/use-mods.ts            # (new) React Query hooks for mod operations
  components/mods/
    mod-upload.tsx             # (new) drag-and-drop upload component
    mod-list.tsx               # (new) list installed mods with delete action
  pages/server-detail.tsx      # (modify) add Mods tab
```

### Pattern 1: Multipart Upload Handler (Go)
**What:** API handler receives multipart file upload, validates file, streams it to pod via tar-over-exec.
**When to use:** POST /api/v1/gameservers/{name}/mods endpoint.
**Key details:**
- Use `http.MaxBytesReader` to cap upload size (e.g., 100MB) BEFORE parsing
- Use `r.ParseMultipartForm(32 << 20)` to limit in-memory buffering to 32MB
- Extract file from `r.FormFile("file")`
- Validate filename: sanitize path separators, reject `..`, check extension
- Use `io.Pipe()` + `archive/tar` to stream file data into pod via SPDY exec
- The tar command in the pod: `["tar", "-xf", "-", "-C", "<modPath>"]`
- Return mod file metadata (name, size) on success
- After successful upload, trigger server restart via status update to Creating state

### Pattern 2: PVC Creation in Controller (Go)
**What:** Controller creates a PVC per GameServer during the Creating phase, mounts it to the pod.
**When to use:** In `reconcilePod` when building the Pod spec.
**Key details:**
- PVC name: `<gameserver-name>-mods`
- PVC is owned by the GameServer CR (OwnerReference) so it gets garbage collected on delete
- PVC size comes from AdminConfig (e.g., `modStorageSize: 1Gi` default)
- StorageClassName from AdminConfig (empty = cluster default)
- Volume mount path comes from the game manifest's `modPath` field
- If `modPath` is empty in the manifest, no PVC is created (game doesn't support mods)

### Pattern 3: Tar-Over-Exec File Transfer (Go)
**What:** Stream a file into a running pod by piping a tar archive through exec stdin.
**When to use:** After receiving an uploaded file, before triggering restart.
**Example (conceptual Go):
```go
// Create pipe: tar writer -> exec stdin reader
reader, writer := io.Pipe()

// Goroutine: write file as tar archive
go func() {
    defer writer.Close()
    tw := tar.NewWriter(writer)
    defer tw.Close()
    tw.WriteHeader(&tar.Header{
        Name: sanitizedFilename,
        Size: fileSize,
        Mode: 0644,
    })
    io.Copy(tw, uploadedFile)
}()

// Execute tar extraction in pod
req := clientset.CoreV1().RESTClient().Post().
    Resource("pods").Name(podName).Namespace(namespace).
    SubResource("exec").
    VersionedParams(&corev1.PodExecOptions{
        Container: "gameserver",
        Command:   []string{"tar", "-xf", "-", "-C", modPath},
        Stdin:     true,
        Stdout:    true,
        Stderr:    true,
    }, scheme.ParameterCodec)

exec, _ := remotecommand.NewSPDYExecutor(restConfig, "POST", req.URL())
exec.StreamWithContext(ctx, remotecommand.StreamOptions{
    Stdin:  reader,
    Stdout: &stdout,
    Stderr: &stderr,
})
```
This pattern is directly derived from how `kubectl cp` works internally. Source: [kubectl cp implementation](https://github.com/kubernetes/kubectl/blob/master/pkg/cmd/cp/cp.go).

### Pattern 4: Frontend Multipart Upload
**What:** Upload file using FormData (not JSON) with progress tracking.
**When to use:** Mod upload UI component.
**Key details:**
- Must NOT set Content-Type header (browser auto-sets with boundary for multipart)
- Current `apiFetch` hardcodes `Content-Type: application/json` -- need a new `apiUpload` helper
- Use `XMLHttpRequest` or `fetch` with `FormData` body
- Progress tracking via `XMLHttpRequest.upload.onprogress` (fetch API lacks upload progress)

### Anti-Patterns to Avoid
- **Buffering entire file in memory:** Use `io.Pipe` streaming, not `ioutil.ReadAll`. Mod files can be 50MB+.
- **Skipping filename sanitization:** Path traversal via `../../etc/passwd` in filename. Always `filepath.Base()` and reject `..`.
- **PVC without OwnerReference:** PVCs would leak on GameServer deletion if not owned. Use `ctrl.SetControllerReference`.
- **Hardcoding mod path:** Different games use different mod directories. Put it in the manifest.
- **Uploading to stopped servers:** tar-over-exec requires a running pod. Either reject uploads to non-running servers, or start the server first.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Tar archive creation | Custom binary format | `archive/tar` stdlib | Tar format is complex; headers, padding, checksums |
| File upload drag-and-drop | Custom drag events | `react-dropzone` | Browser drag-and-drop API is notoriously inconsistent across browsers |
| Upload size limiting | Manual Content-Length check | `http.MaxBytesReader` | Handles chunked encoding, connection closure, error propagation |
| File-to-pod transfer | Custom WebSocket file protocol | `remotecommand` + tar | kubectl cp's battle-tested pattern; handles SPDY/WebSocket negotiation |

**Key insight:** The tar-over-exec pattern is how `kubectl cp` works. Reimplementing file transfer over WebSocket or a custom protocol would be fragile and untested. The K8s ecosystem already solved this problem.

## Common Pitfalls

### Pitfall 1: Content-Type Header on Multipart Upload
**What goes wrong:** Frontend sends `Content-Type: application/json` with FormData body, causing Go to fail on `ParseMultipartForm`.
**Why it happens:** The existing `apiFetch` client hardcodes `Content-Type: application/json`.
**How to avoid:** Create a separate `apiUpload` function that does NOT set Content-Type. The browser automatically sets `Content-Type: multipart/form-data; boundary=...` when the body is a FormData object.
**Warning signs:** "no multipart boundary param in Content-Type" error from Go.

### Pitfall 2: PVC Not Cleaned Up on GameServer Deletion
**What goes wrong:** PVCs persist after GameServer is deleted, consuming storage quota.
**Why it happens:** PVC was not created with OwnerReference pointing to the GameServer.
**How to avoid:** Use `ctrl.SetControllerReference(gs, pvc, r.Scheme)` when creating the PVC so K8s garbage collection handles cleanup.
**Warning signs:** Orphaned PVCs in user namespaces after server deletion.

### Pitfall 3: Upload to Non-Running Server
**What goes wrong:** tar-over-exec fails because the pod doesn't exist or isn't running.
**Why it happens:** User tries to upload mods to a Shutdown/Error/Creating server.
**How to avoid:** Check GameServer state before upload. Only allow uploads when state is Ready or Allocated (pod is running).
**Warning signs:** "container not found" or "pod not running" errors from exec.

### Pitfall 4: RBAC Missing for PVCs
**What goes wrong:** Controller fails to create PVCs with "forbidden" error.
**Why it happens:** Current RBAC markers don't include `persistentvolumeclaims` permission.
**How to avoid:** Add kubebuilder RBAC marker: `// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete`
**Warning signs:** "persistentvolumeclaims is forbidden" in controller logs.

### Pitfall 5: Mod File Size Exhausting Server Memory
**What goes wrong:** A 500MB mod upload causes the API server to OOM.
**Why it happens:** `ParseMultipartForm` reads entire file into memory if maxMemory is too high.
**How to avoid:** Use `http.MaxBytesReader` to reject oversized requests early, then `ParseMultipartForm(32 << 20)` to limit memory. Stream file data via `io.Pipe` instead of reading it all into a buffer.
**Warning signs:** API server memory spike during uploads, eventual OOM kill.

### Pitfall 6: Race Between Upload and Restart
**What goes wrong:** File is partially written when restart triggers, causing corrupted mod.
**Why it happens:** Restart is triggered before tar-over-exec completes.
**How to avoid:** Only trigger restart AFTER the exec stream completes successfully. This is naturally handled by making restart the last step in the handler.
**Warning signs:** Corrupted or partial mod files after upload.

### Pitfall 7: itzg/minecraft-server Mod Directory Sync
**What goes wrong:** Mods uploaded to `/mods` PVC don't appear in `/data/mods` where the server reads them.
**Why it happens:** The itzg/minecraft-server image copies files from `/mods` to `/data/mods` at startup. Uploading after startup may not trigger re-sync.
**How to avoid:** The restart after upload handles this -- the server re-syncs mods from `/mods` to `/data/mods` on every startup. This is by design.
**Warning signs:** Mods in `/mods` but not in `/data/mods`; server restart resolves it.

## Code Examples

### Go: Multipart Upload Handler
```go
// Source: Go stdlib net/http + archive/tar + k8s client-go remotecommand
func (s *Server) handleUploadMod(w http.ResponseWriter, r *http.Request) {
    ns := namespaceFromContext(r)
    name := chi.URLParam(r, "name")

    // 1. Enforce upload size limit
    r.Body = http.MaxBytesReader(w, r.Body, 100<<20) // 100MB max

    // 2. Parse multipart form
    if err := r.ParseMultipartForm(32 << 20); err != nil {
        respondError(w, http.StatusBadRequest, "file too large or invalid form data")
        return
    }

    file, header, err := r.FormFile("file")
    if err != nil {
        respondError(w, http.StatusBadRequest, "missing file field")
        return
    }
    defer file.Close()

    // 3. Sanitize filename
    filename := filepath.Base(header.Filename)
    if filename == "." || filename == ".." || strings.Contains(filename, "/") {
        respondError(w, http.StatusBadRequest, "invalid filename")
        return
    }

    // 4. Verify server is running, get mod path from manifest
    // ... (fetch GameServer, check state, look up manifest modPath) ...

    // 5. Stream file to pod via tar-over-exec
    // ... (io.Pipe + tar.NewWriter + remotecommand.NewSPDYExecutor) ...

    // 6. Trigger restart
    // ... (set state to Creating via Status().Update()) ...
}
```

### Go: PVC Creation in Controller
```go
// Source: k8s.io/api/core/v1 PersistentVolumeClaim types
func (r *GameServerReconciler) reconcileModPVC(ctx context.Context, gs *gamev1alpha1.GameServer, modStorageSize resource.Quantity) error {
    pvc := &corev1.PersistentVolumeClaim{
        ObjectMeta: metav1.ObjectMeta{
            Name:      gs.Name + "-mods",
            Namespace: gs.Namespace,
        },
    }
    _, err := controllerutil.CreateOrUpdate(ctx, r.Client, pvc, func() error {
        if err := ctrl.SetControllerReference(gs, pvc, r.Scheme); err != nil {
            return err
        }
        pvc.Spec = corev1.PersistentVolumeClaimSpec{
            AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
            Resources: corev1.VolumeResourceRequirements{
                Requests: corev1.ResourceList{
                    corev1.ResourceStorage: modStorageSize,
                },
            },
        }
        return nil
    })
    return err
}
```

### Go: Pod Spec with Mod Volume Mount
```go
// In reconcilePod, add volume + volume mount when modPath is set
pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
    Name: "mods",
    VolumeSource: corev1.VolumeSource{
        PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
            ClaimName: gs.Name + "-mods",
        },
    },
})
pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
    Name:      "mods",
    MountPath: modPath, // e.g., "/mods" from manifest
})
```

### Frontend: Upload Helper (no Content-Type header)
```typescript
// Source: standard fetch API with FormData
export async function apiUpload<T>(
  path: string,
  file: File,
  onProgress?: (percent: number) => void,
): Promise<T> {
  const token = useAuthStore.getState().token;
  const formData = new FormData();
  formData.append('file', file);

  // Use XMLHttpRequest for upload progress support
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open('POST', `${API_BASE}${path}`);
    if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`);
    // DO NOT set Content-Type -- browser sets it with boundary

    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable && onProgress) {
        onProgress(Math.round((e.loaded / e.total) * 100));
      }
    };

    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve(JSON.parse(xhr.responseText));
      } else {
        reject(new ApiError(xhr.status, xhr.responseText));
      }
    };

    xhr.onerror = () => reject(new Error('Upload failed'));
    xhr.send(formData);
  });
}
```

### Game Manifest: modPath field
```yaml
# games/minecraft/manifest.yaml -- add modPath
name: minecraft
displayName: "Minecraft Java Edition"
image: itzg/minecraft-server:latest
modPath: /mods        # <-- NEW: where mods PVC mounts in the container
ports:
  - name: game
    containerPort: 25565
    protocol: TCP
# ... rest unchanged
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| kubectl cp (tar-over-exec) | Same (still tar-over-exec) | Stable since K8s 1.5+ | Pattern is mature and unchanged |
| StaticProvisioner PV | Dynamic PVC provisioning | K8s 1.6+ | StorageClass handles PV creation automatically |
| Manual file input | react-dropzone | 2020+ | Drag-and-drop is expected UX for file uploads |

**No deprecated patterns in play.** The tar-over-exec pattern, PVC dynamic provisioning, and multipart form uploads are all stable, well-documented approaches.

## Open Questions

1. **Max mod file size limit**
   - What we know: Minecraft mods are typically 1-50MB each; modpacks can be 200MB+
   - What's unclear: What's a reasonable per-upload limit for this homelab context?
   - Recommendation: Default to 100MB per file. Make configurable via AdminConfig `maxModUploadSize`.

2. **Mod storage size per server**
   - What we know: PVC size must be specified at creation time. ResourceQuota limits total storage.
   - What's unclear: How much storage per server? Current quota is 50Gi total per user, 5 PVCs.
   - Recommendation: Default to 1Gi per server mod PVC. Make configurable via AdminConfig `modStorageSize`.

3. **StorageClass selection**
   - What we know: Talos cluster likely has a default StorageClass. PVC spec can leave StorageClassName empty to use default.
   - What's unclear: Which StorageClass is available on the homelab cluster.
   - Recommendation: Leave StorageClassName empty (use cluster default). Add optional `modStorageClass` AdminConfig field for override.

4. **Listing mods (reading from pod)**
   - What we know: We need a GET endpoint to list installed mods. This requires exec into the pod to `ls` the mod directory.
   - What's unclear: Performance of ls-over-exec for large mod directories.
   - Recommendation: Simple `ls -la <modPath>` exec, parse output. Mod directories rarely have >100 files.

5. **Deleting individual mods**
   - What we know: Users may want to remove a specific mod without clearing all mods.
   - What's unclear: Whether to support individual mod deletion in phase 8 or defer.
   - Recommendation: Include it -- `rm <modPath>/<filename>` via exec is trivial. No restart needed for deletion (server restart optional, user-triggered).

6. **Multiple file upload**
   - What we know: Users often install multiple mods at once.
   - What's unclear: Whether to handle multiple files in a single request or require one-at-a-time.
   - Recommendation: Support multiple files per upload request. Tar archive naturally supports multiple entries. Auto-restart only once after all files are transferred.

## Sources

### Primary (HIGH confidence)
- Codebase analysis: `internal/controller/gameserver_controller.go` -- current Pod creation, PVC quota handling, RBAC markers
- Codebase analysis: `internal/api/handlers_console.go` -- existing remotecommand/SPDY exec pattern
- Codebase analysis: `internal/manifest/manifest.go` -- GameManifest structure, loading patterns
- Codebase analysis: `web/src/api/client.ts` -- hardcoded Content-Type that needs modification
- [kubectl cp source](https://github.com/kubernetes/kubectl/blob/master/pkg/cmd/cp/cp.go) -- tar-over-exec reference implementation
- [itzg/docker-minecraft-server mod docs](https://github.com/itzg/docker-minecraft-server/blob/master/docs/mods-and-plugins/index.md) -- `/mods` directory, startup sync behavior

### Secondary (MEDIUM confidence)
- [Kubernetes PVC documentation](https://kubernetes.io/docs/concepts/storage/persistent-volumes/) -- PVC spec, dynamic provisioning, OwnerReference behavior
- [Go multipart upload patterns](https://freshman.tech/file-upload-golang/) -- MaxBytesReader, ParseMultipartForm, FormFile
- [react-dropzone GitHub](https://github.com/diragb/shadcn-dropzone) -- shadcn/ui compatible dropzone component

### Tertiary (LOW confidence)
- Upload progress tracking -- XMLHttpRequest is needed because fetch API lacks upload progress events. This is a well-known limitation but should be verified against latest browser APIs.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- All libraries are already in the codebase or Go stdlib; React dropzone is well-established
- Architecture: HIGH -- tar-over-exec is proven by kubectl cp and already partially implemented in this codebase (console exec); PVC creation follows existing CreateOrUpdate pattern
- Pitfalls: HIGH -- Identified from codebase analysis (Content-Type hardcoding, missing RBAC, itzg sync behavior)

**Research date:** 2026-02-12
**Valid until:** 2026-03-12 (stable patterns, no fast-moving dependencies)
