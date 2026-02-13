/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package api

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gamev1alpha1 "github.com/kterodactyl/kterodactyl/api/v1alpha1"
	"github.com/kterodactyl/kterodactyl/internal/util"
)

// ModFileResponse is the API response for a mod file.
type ModFileResponse struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// execInPod executes a command inside a pod container via SPDY exec.
// It returns the captured stdout, stderr, and any error.
// Pass a non-nil stdin reader for commands that require input (e.g., tar extraction).
func (s *Server) execInPod(ctx context.Context, namespace, podName string, command []string, stdin io.Reader) (string, string, error) {
	req := s.clientset.CoreV1().RESTClient().Post().
		Resource("pods").Name(podName).Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: "gameserver",
			Command:   command,
			Stdin:     stdin != nil,
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(s.restConfig, "POST", req.URL())
	if err != nil {
		return "", "", fmt.Errorf("exec setup failed: %w", err)
	}

	var stdout, stderr bytes.Buffer
	streamOpts := remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	}
	if stdin != nil {
		streamOpts.Stdin = stdin
	}

	if err := exec.StreamWithContext(ctx, streamOpts); err != nil {
		return stdout.String(), stderr.String(), err
	}

	return stdout.String(), stderr.String(), nil
}

// handleUploadMod handles multipart file upload of a mod to a running game server pod.
// The file is streamed into the pod via tar-over-exec.
//
// POST /api/v1/gameservers/{name}/mods
func (s *Server) handleUploadMod(w http.ResponseWriter, r *http.Request) {
	ns := namespaceFromContext(r)
	if ns == "" {
		respondError(w, http.StatusUnauthorized, "no namespace in context")
		return
	}

	serverName := chi.URLParam(r, "name")
	ctx := r.Context()

	// Fetch GameServer
	gs := &gamev1alpha1.GameServer{}
	if err := s.client.Get(ctx, client.ObjectKey{Name: serverName, Namespace: ns}, gs); err != nil {
		respondError(w, http.StatusNotFound, "game server not found: "+serverName)
		return
	}

	// Verify server is running
	if gs.Status.State != gamev1alpha1.GameServerStateReady &&
		gs.Status.State != gamev1alpha1.GameServerStateAllocated {
		respondError(w, http.StatusConflict, "server must be running to upload mods")
		return
	}

	// Read mod path annotation
	modPath := gs.Annotations[util.AnnotationModPath]
	if modPath == "" {
		respondError(w, http.StatusBadRequest, "game does not support mods")
		return
	}

	// Apply upload size limit (100MB)
	r.Body = http.MaxBytesReader(w, r.Body, 100<<20)

	// Parse multipart form (32MB memory buffer)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "file too large or invalid form data")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "missing file field in form data")
		return
	}
	defer file.Close()

	// Sanitize filename
	filename := filepath.Base(header.Filename)
	if filename == "." || filename == ".." || strings.ContainsAny(filename, "/\\") {
		respondError(w, http.StatusBadRequest, "invalid filename")
		return
	}

	// Create tar pipe: goroutine writes tar archive, main thread reads it into exec stdin
	pr, pw := io.Pipe()

	errCh := make(chan error, 1)
	go func() {
		defer pw.Close()
		tw := tar.NewWriter(pw)
		defer tw.Close()

		if err := tw.WriteHeader(&tar.Header{
			Name:    filename,
			Size:    header.Size,
			Mode:    0644,
			ModTime: time.Now(),
		}); err != nil {
			errCh <- fmt.Errorf("tar header write failed: %w", err)
			return
		}

		if _, err := io.Copy(tw, file); err != nil {
			errCh <- fmt.Errorf("tar data write failed: %w", err)
			return
		}

		errCh <- nil
	}()

	// Execute tar extraction in the pod
	_, stderr, execErr := s.execInPod(ctx, ns, serverName, []string{"tar", "-xf", "-", "-C", modPath}, pr)

	// Check tar writer goroutine error
	if tarErr := <-errCh; tarErr != nil && execErr == nil {
		respondError(w, http.StatusInternalServerError, "failed to stream mod file: "+tarErr.Error())
		return
	}

	if execErr != nil {
		respondError(w, http.StatusInternalServerError, "failed to upload mod: "+strings.TrimSpace(stderr))
		return
	}

	// Trigger server restart after successful upload
	// Re-fetch the GameServer before status update (re-fetch pattern)
	if err := s.client.Get(ctx, client.ObjectKey{Name: serverName, Namespace: ns}, gs); err != nil {
		respondError(w, http.StatusInternalServerError, "mod uploaded but failed to restart server")
		return
	}
	gs.Status.State = gamev1alpha1.GameServerStateCreating
	if err := s.client.Status().Update(ctx, gs); err != nil {
		respondError(w, http.StatusInternalServerError, "mod uploaded but failed to restart server")
		return
	}

	respondJSON(w, http.StatusOK, ModFileResponse{
		Name: filename,
		Size: header.Size,
	})
}

// handleListMods lists mod files installed on a running game server pod.
//
// GET /api/v1/gameservers/{name}/mods
func (s *Server) handleListMods(w http.ResponseWriter, r *http.Request) {
	ns := namespaceFromContext(r)
	if ns == "" {
		respondError(w, http.StatusUnauthorized, "no namespace in context")
		return
	}

	serverName := chi.URLParam(r, "name")
	ctx := r.Context()

	// Fetch GameServer
	gs := &gamev1alpha1.GameServer{}
	if err := s.client.Get(ctx, client.ObjectKey{Name: serverName, Namespace: ns}, gs); err != nil {
		respondError(w, http.StatusNotFound, "game server not found: "+serverName)
		return
	}

	// Verify server is running
	if gs.Status.State != gamev1alpha1.GameServerStateReady &&
		gs.Status.State != gamev1alpha1.GameServerStateAllocated {
		respondError(w, http.StatusConflict, "server must be running to list mods")
		return
	}

	// Read mod path annotation
	modPath := gs.Annotations[util.AnnotationModPath]
	if modPath == "" {
		respondError(w, http.StatusBadRequest, "game does not support mods")
		return
	}

	// Execute ls in the pod
	stdout, _, err := s.execInPod(ctx, ns, serverName, []string{"ls", "-la", modPath}, nil)
	if err != nil {
		// Directory empty or doesn't exist yet -- return empty list
		respondList(w, http.StatusOK, []ModFileResponse{}, 0)
		return
	}

	// Parse ls -la output
	mods := parseLsOutput(stdout)
	respondList(w, http.StatusOK, mods, len(mods))
}

// parseLsOutput parses the output of `ls -la` into ModFileResponse entries.
// Skips the "total" line and entries for "." and "..".
func parseLsOutput(output string) []ModFileResponse {
	var mods []ModFileResponse
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		// Skip "total" line
		if strings.HasPrefix(line, "total") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		name := fields[len(fields)-1]
		// Skip . and .. directories
		if name == "." || name == ".." {
			continue
		}
		size, err := strconv.ParseInt(fields[4], 10, 64)
		if err != nil {
			size = 0
		}
		mods = append(mods, ModFileResponse{
			Name: name,
			Size: size,
		})
	}
	return mods
}

// handleDeleteMod deletes a specific mod file from a running game server pod.
//
// DELETE /api/v1/gameservers/{name}/mods/{filename}
func (s *Server) handleDeleteMod(w http.ResponseWriter, r *http.Request) {
	ns := namespaceFromContext(r)
	if ns == "" {
		respondError(w, http.StatusUnauthorized, "no namespace in context")
		return
	}

	serverName := chi.URLParam(r, "name")
	filename := chi.URLParam(r, "filename")
	ctx := r.Context()

	// Sanitize filename
	filename = filepath.Base(filename)
	if filename == "." || filename == ".." || strings.ContainsAny(filename, "/\\") {
		respondError(w, http.StatusBadRequest, "invalid filename")
		return
	}

	// Fetch GameServer
	gs := &gamev1alpha1.GameServer{}
	if err := s.client.Get(ctx, client.ObjectKey{Name: serverName, Namespace: ns}, gs); err != nil {
		respondError(w, http.StatusNotFound, "game server not found: "+serverName)
		return
	}

	// Verify server is running
	if gs.Status.State != gamev1alpha1.GameServerStateReady &&
		gs.Status.State != gamev1alpha1.GameServerStateAllocated {
		respondError(w, http.StatusConflict, "server must be running to delete mods")
		return
	}

	// Read mod path annotation
	modPath := gs.Annotations[util.AnnotationModPath]
	if modPath == "" {
		respondError(w, http.StatusBadRequest, "game does not support mods")
		return
	}

	// Execute rm in the pod
	_, stderr, err := s.execInPod(ctx, ns, serverName, []string{"rm", "-f", modPath + "/" + filename}, nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete mod: "+strings.TrimSpace(stderr))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
