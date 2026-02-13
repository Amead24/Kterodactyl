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
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/robfig/cron/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gamev1alpha1 "github.com/kterodactyl/kterodactyl/api/v1alpha1"
	"github.com/kterodactyl/kterodactyl/internal/controller"
	"github.com/kterodactyl/kterodactyl/internal/util"
)

const (
	// s3CredentialsSecretName is the name of the Secret containing S3 credentials.
	// Must match the constant in internal/controller/backup_controller.go.
	s3CredentialsSecretName = "kterodactyl-s3-credentials"
)

// BackupResponse is the API response format for a Backup.
type BackupResponse struct {
	Name           string `json:"name"`
	GameServerName string `json:"gameServerName"`
	State          string `json:"state"`
	S3Key          string `json:"s3Key,omitempty"`
	S3Bucket       string `json:"s3Bucket,omitempty"`
	Size           int64  `json:"size,omitempty"`
	StartedAt      string `json:"startedAt,omitempty"`
	CompletedAt    string `json:"completedAt,omitempty"`
	Message        string `json:"message,omitempty"`
	CreatedAt      string `json:"createdAt"`
}

// SetBackupScheduleRequest is the request body for setting a backup schedule.
type SetBackupScheduleRequest struct {
	Schedule  string `json:"schedule"`  // Cron expression, empty to disable
	Retention int    `json:"retention"` // Max backups to retain (0 = use admin default)
}

// backupToResponse maps a Backup CRD to its API response representation.
func backupToResponse(backup *gamev1alpha1.Backup) BackupResponse {
	resp := BackupResponse{
		Name:           backup.Name,
		GameServerName: backup.Spec.GameServerName,
		State:          string(backup.Status.State),
		S3Key:          backup.Status.S3Key,
		S3Bucket:       backup.Status.S3Bucket,
		Size:           backup.Status.Size,
		Message:        backup.Status.Message,
		CreatedAt:      backup.CreationTimestamp.Format(time.RFC3339),
	}

	if backup.Status.StartedAt != nil {
		resp.StartedAt = backup.Status.StartedAt.Format(time.RFC3339)
	}
	if backup.Status.CompletedAt != nil {
		resp.CompletedAt = backup.Status.CompletedAt.Format(time.RFC3339)
	}

	return resp
}

// handleCreateBackup triggers an on-demand backup for a GameServer.
//
// POST /api/v1/gameservers/{name}/backups
func (s *Server) handleCreateBackup(w http.ResponseWriter, r *http.Request) {
	ns := namespaceFromContext(r)
	if ns == "" {
		respondError(w, http.StatusUnauthorized, "no namespace in context")
		return
	}

	serverName := chi.URLParam(r, "name")
	ctx := r.Context()

	// Fetch the GameServer
	gs := &gamev1alpha1.GameServer{}
	if err := s.client.Get(ctx, client.ObjectKey{Name: serverName, Namespace: ns}, gs); err != nil {
		respondError(w, http.StatusNotFound, "game server not found: "+serverName)
		return
	}

	// Verify GameServer state is Ready or Allocated
	if gs.Status.State != gamev1alpha1.GameServerStateReady &&
		gs.Status.State != gamev1alpha1.GameServerStateAllocated {
		respondError(w, http.StatusConflict, "server must be running to create backup")
		return
	}

	// Check for existing InProgress backup
	backupList := &gamev1alpha1.BackupList{}
	if err := s.client.List(ctx, backupList, client.InNamespace(ns),
		client.MatchingLabels{util.LabelBackupGameServer: serverName}); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list backups")
		return
	}
	for i := range backupList.Items {
		if backupList.Items[i].Status.State == gamev1alpha1.BackupStateInProgress {
			respondError(w, http.StatusConflict, "backup already in progress")
			return
		}
	}

	// Create a Backup CR
	timestamp := time.Now().UTC().Format("20060102-150405")
	backupName := fmt.Sprintf("%s-backup-%s", serverName, timestamp)

	backup := &gamev1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupName,
			Namespace: ns,
			Labels: map[string]string{
				util.LabelBackupGameServer: serverName,
				util.LabelManagedBy:        util.ManagedByValue,
			},
		},
		Spec: gamev1alpha1.BackupSpec{
			GameServerName: serverName,
		},
	}

	if err := s.client.Create(ctx, backup); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create backup")
		return
	}

	// Set initial state via status subresource
	backup.Status.State = gamev1alpha1.BackupStatePending
	if err := s.client.Status().Update(ctx, backup); err != nil {
		respondError(w, http.StatusInternalServerError, "backup created but failed to set initial state")
		return
	}

	respondJSON(w, http.StatusCreated, backupToResponse(backup))
}

// handleListBackups lists all backups for a specific GameServer.
//
// GET /api/v1/gameservers/{name}/backups
func (s *Server) handleListBackups(w http.ResponseWriter, r *http.Request) {
	ns := namespaceFromContext(r)
	if ns == "" {
		respondError(w, http.StatusUnauthorized, "no namespace in context")
		return
	}

	serverName := chi.URLParam(r, "name")
	ctx := r.Context()

	// Fetch the GameServer (ownership check)
	gs := &gamev1alpha1.GameServer{}
	if err := s.client.Get(ctx, client.ObjectKey{Name: serverName, Namespace: ns}, gs); err != nil {
		respondError(w, http.StatusNotFound, "game server not found: "+serverName)
		return
	}

	// List Backup CRs with label selector
	backupList := &gamev1alpha1.BackupList{}
	if err := s.client.List(ctx, backupList, client.InNamespace(ns),
		client.MatchingLabels{util.LabelBackupGameServer: serverName}); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list backups")
		return
	}

	// Sort by creation time (newest first)
	sort.Slice(backupList.Items, func(i, j int) bool {
		return backupList.Items[i].CreationTimestamp.After(backupList.Items[j].CreationTimestamp.Time)
	})

	// Map to response
	responses := make([]BackupResponse, len(backupList.Items))
	for i := range backupList.Items {
		responses[i] = backupToResponse(&backupList.Items[i])
	}

	respondList(w, http.StatusOK, responses, len(responses))
}

// handleDeleteBackup deletes a Backup CR.
// Admin-only endpoint (RequireAdmin middleware applied in routes).
//
// DELETE /api/v1/gameservers/{name}/backups/{backupName}
func (s *Server) handleDeleteBackup(w http.ResponseWriter, r *http.Request) {
	ns := namespaceFromContext(r)
	if ns == "" {
		respondError(w, http.StatusUnauthorized, "no namespace in context")
		return
	}

	serverName := chi.URLParam(r, "name")
	backupName := chi.URLParam(r, "backupName")
	ctx := r.Context()

	// Fetch the Backup CR
	backup := &gamev1alpha1.Backup{}
	if err := s.client.Get(ctx, client.ObjectKey{Name: backupName, Namespace: ns}, backup); err != nil {
		respondError(w, http.StatusNotFound, "backup not found: "+backupName)
		return
	}

	// Verify the Backup belongs to the specified GameServer (prevent cross-server access)
	if backup.Spec.GameServerName != serverName {
		respondError(w, http.StatusNotFound, "backup not found for this game server")
		return
	}

	// Delete the Backup CR
	// NOTE: S3 object cleanup is handled by the BackupReconciler's finalizer or future enhancement.
	// Orphan S3 objects are acceptable for homelab v1.
	if err := s.client.Delete(ctx, backup); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete backup")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleRestoreBackup restores a GameServer from a completed backup.
// Admin-only endpoint.
//
// POST /api/v1/gameservers/{name}/backups/{backupName}/restore
func (s *Server) handleRestoreBackup(w http.ResponseWriter, r *http.Request) {
	ns := namespaceFromContext(r)
	if ns == "" {
		respondError(w, http.StatusUnauthorized, "no namespace in context")
		return
	}

	serverName := chi.URLParam(r, "name")
	backupName := chi.URLParam(r, "backupName")
	ctx := r.Context()

	// Fetch the GameServer
	gs := &gamev1alpha1.GameServer{}
	if err := s.client.Get(ctx, client.ObjectKey{Name: serverName, Namespace: ns}, gs); err != nil {
		respondError(w, http.StatusNotFound, "game server not found: "+serverName)
		return
	}

	// Fetch the Backup CR
	backup := &gamev1alpha1.Backup{}
	if err := s.client.Get(ctx, client.ObjectKey{Name: backupName, Namespace: ns}, backup); err != nil {
		respondError(w, http.StatusNotFound, "backup not found: "+backupName)
		return
	}

	// Verify Backup belongs to this GameServer
	if backup.Spec.GameServerName != serverName {
		respondError(w, http.StatusNotFound, "backup not found for this game server")
		return
	}

	// Verify Backup state is Completed
	if backup.Status.State != gamev1alpha1.BackupStateCompleted {
		respondError(w, http.StatusConflict, "backup must be in Completed state to restore")
		return
	}

	// Verify GameServer state is Ready or Allocated
	if gs.Status.State != gamev1alpha1.GameServerStateReady &&
		gs.Status.State != gamev1alpha1.GameServerStateAllocated {
		respondError(w, http.StatusConflict, "server must be running to restore from backup")
		return
	}

	// Load AdminConfig for S3 settings
	adminCfg, err := s.loadAdminConfig(ctx)
	if err != nil || adminCfg.BackupS3Endpoint == "" {
		respondError(w, http.StatusServiceUnavailable, "S3 backup storage not configured")
		return
	}

	// Create S3 client for download
	s3Client, err := s.createS3Client(ctx, adminCfg)
	if err != nil {
		respondError(w, http.StatusServiceUnavailable, "failed to initialize S3 client: "+err.Error())
		return
	}

	// Determine backup path
	backupPath := gs.Annotations[util.AnnotationBackupPath]
	if backupPath == "" {
		backupPath = "/data" // default
	}

	// Perform restore: S3 GetObject -> gzip.NewReader -> exec tar -xf - -C <backupPath>
	obj, err := s3Client.GetObject(ctx, backup.Status.S3Bucket, backup.Status.S3Key, minio.GetObjectOptions{})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to download backup from S3")
		return
	}
	defer obj.Close()

	gzReader, err := gzip.NewReader(obj)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to decompress backup")
		return
	}
	defer gzReader.Close()

	// Exec tar extraction in the pod (reuse execInPod from handlers_mods.go)
	_, stderr, execErr := s.execInPod(ctx, ns, serverName, []string{"tar", "-xf", "-", "-C", backupPath}, gzReader)
	if execErr != nil {
		respondError(w, http.StatusInternalServerError, "failed to restore backup: "+strings.TrimSpace(stderr))
		return
	}

	// After successful restore, restart the server
	// Re-fetch the GameServer before status update (re-fetch pattern)
	if err := s.client.Get(ctx, client.ObjectKey{Name: serverName, Namespace: ns}, gs); err != nil {
		respondJSON(w, http.StatusOK, map[string]string{
			"message": fmt.Sprintf("restored from backup %s, but failed to restart server", backupName),
		})
		return
	}
	gs.Status.State = gamev1alpha1.GameServerStateCreating
	if err := s.client.Status().Update(ctx, gs); err != nil {
		respondJSON(w, http.StatusOK, map[string]string{
			"message": fmt.Sprintf("restored from backup %s, but failed to restart server", backupName),
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": fmt.Sprintf("restored from backup %s, server restarting", backupName),
	})
}

// handleSetBackupSchedule sets or removes a backup schedule on a GameServer.
// Admin-only endpoint.
//
// PUT /api/v1/gameservers/{name}/backup-schedule
func (s *Server) handleSetBackupSchedule(w http.ResponseWriter, r *http.Request) {
	ns := namespaceFromContext(r)
	if ns == "" {
		respondError(w, http.StatusUnauthorized, "no namespace in context")
		return
	}

	serverName := chi.URLParam(r, "name")
	ctx := r.Context()

	var req SetBackupScheduleRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Fetch the GameServer
	gs := &gamev1alpha1.GameServer{}
	if err := s.client.Get(ctx, client.ObjectKey{Name: serverName, Namespace: ns}, gs); err != nil {
		respondError(w, http.StatusNotFound, "game server not found: "+serverName)
		return
	}

	if gs.Annotations == nil {
		gs.Annotations = make(map[string]string)
	}

	if req.Schedule == "" {
		// Remove schedule and retention annotations
		delete(gs.Annotations, util.AnnotationBackupSchedule)
		delete(gs.Annotations, util.AnnotationBackupRetention)
	} else {
		// Validate cron expression
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		if _, err := parser.Parse(req.Schedule); err != nil {
			respondError(w, http.StatusBadRequest, "invalid cron expression: "+err.Error())
			return
		}

		gs.Annotations[util.AnnotationBackupSchedule] = req.Schedule
		if req.Retention > 0 {
			gs.Annotations[util.AnnotationBackupRetention] = strconv.Itoa(req.Retention)
		} else {
			delete(gs.Annotations, util.AnnotationBackupRetention)
		}
	}

	// Update the GameServer
	if err := s.client.Update(ctx, gs); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update backup schedule")
		return
	}

	// Build response
	schedule := gs.Annotations[util.AnnotationBackupSchedule]
	retention := 0
	if retStr := gs.Annotations[util.AnnotationBackupRetention]; retStr != "" {
		if n, err := strconv.Atoi(retStr); err == nil {
			retention = n
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"gameServer": serverName,
		"schedule":   schedule,
		"retention":  retention,
	})
}

// createS3Client creates a new minio S3 client using AdminConfig and S3 credentials Secret.
// This is the API-server-side equivalent of BackupReconciler.ensureS3Client.
func (s *Server) createS3Client(ctx context.Context, cfg *controller.AdminConfig) (*minio.Client, error) {
	// Load S3 credentials from Secret
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{Name: s3CredentialsSecretName, Namespace: s.operatorNamespace}
	if err := s.client.Get(ctx, secretKey, secret); err != nil {
		return nil, fmt.Errorf("S3 credentials secret not found")
	}

	accessKeyID := string(secret.Data["accessKeyID"])
	secretAccessKey := string(secret.Data["secretAccessKey"])
	if accessKeyID == "" || secretAccessKey == "" {
		return nil, fmt.Errorf("S3 credentials secret missing accessKeyID or secretAccessKey")
	}

	minioClient, err := minio.New(cfg.BackupS3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: cfg.BackupS3UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %w", err)
	}

	return minioClient, nil
}
