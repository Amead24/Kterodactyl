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

package controller

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/tools/remotecommand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/robfig/cron/v3"

	gamev1alpha1 "github.com/kterodactyl/kterodactyl/api/v1alpha1"
	"github.com/kterodactyl/kterodactyl/internal/util"
)

const (
	// s3CredentialsSecretName is the name of the Secret containing S3 credentials.
	s3CredentialsSecretName = "kterodactyl-s3-credentials"

	// backupTimeout is the maximum duration for a backup operation.
	backupTimeout = 30 * time.Minute

	// s3PartSize is the multipart upload part size (64MB).
	s3PartSize = 64 * 1024 * 1024
)

// BackupReconciler reconciles a Backup object.
type BackupReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	Recorder          record.EventRecorder
	OperatorNamespace string
	Clientset         *kubernetes.Clientset
	RestConfig        *rest.Config

	// s3Client is lazily initialized on first backup.
	s3Client *minio.Client
	s3Bucket string
}

// +kubebuilder:rbac:groups=game.kterodactyl.io,resources=backups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=game.kterodactyl.io,resources=backups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=game.kterodactyl.io,resources=backups/finalizers,verbs=update
// +kubebuilder:rbac:groups=game.kterodactyl.io,resources=gameservers,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=pods/exec,verbs=create
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile is the main reconciliation loop for Backup resources.
// It also handles schedule-triggered reconciliations from GameServer watches.
func (r *BackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Check if this is a schedule-triggered reconciliation from a GameServer watch.
	// Schedule events use synthetic names "schedule-<gameserver-name>".
	if strings.HasPrefix(req.Name, "schedule-") {
		gsName := strings.TrimPrefix(req.Name, "schedule-")
		gs := &gamev1alpha1.GameServer{}
		if err := r.Get(ctx, types.NamespacedName{Name: gsName, Namespace: req.Namespace}, gs); err != nil {
			if errors.IsNotFound(err) {
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, fmt.Errorf("failed to get GameServer for schedule: %w", err)
		}
		return r.reconcileGameServerSchedule(ctx, gs)
	}

	// Fetch the Backup CR
	backup := &gamev1alpha1.Backup{}
	if err := r.Get(ctx, req.NamespacedName, backup); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Backup resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get Backup: %w", err)
	}

	// Skip terminal states
	if gamev1alpha1.IsBackupTerminal(backup.Status.State) {
		return ctrl.Result{}, nil
	}

	// Load AdminConfig for S3 settings
	opNs := r.OperatorNamespace
	if opNs == "" {
		opNs = defaultOperatorNamespace
	}
	adminCfg, err := LoadAdminConfig(ctx, r.Client, opNs)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to load admin config: %w", err)
	}

	// Dispatch based on state
	switch backup.Status.State {
	case "", gamev1alpha1.BackupStatePending:
		return r.reconcilePending(ctx, backup, adminCfg)
	case gamev1alpha1.BackupStateInProgress:
		return r.reconcileInProgress(ctx, backup, adminCfg)
	default:
		log.Error(nil, "Unknown backup state", "state", backup.Status.State)
		return ctrl.Result{}, nil
	}
}

// reconcilePending handles the Pending state: validates configuration and transitions to InProgress.
func (r *BackupReconciler) reconcilePending(ctx context.Context, backup *gamev1alpha1.Backup, cfg *AdminConfig) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Reconciling Pending backup", "name", backup.Name, "gameserver", backup.Spec.GameServerName)

	// Initialize state if empty
	if backup.Status.State == "" {
		return r.transitionBackupState(ctx, backup, gamev1alpha1.BackupStatePending, "Initializing", "Backup queued")
	}

	// Validate S3 is configured
	if cfg.BackupS3Endpoint == "" {
		return r.transitionBackupState(ctx, backup, gamev1alpha1.BackupStateFailed, "S3NotConfigured", "S3 not configured: set backupS3Endpoint in admin config")
	}

	// Validate the referenced GameServer exists
	gs := &gamev1alpha1.GameServer{}
	gsKey := types.NamespacedName{Name: backup.Spec.GameServerName, Namespace: backup.Namespace}
	if err := r.Get(ctx, gsKey, gs); err != nil {
		if errors.IsNotFound(err) {
			return r.transitionBackupState(ctx, backup, gamev1alpha1.BackupStateFailed, "GameServerNotFound",
				fmt.Sprintf("GameServer %s not found", backup.Spec.GameServerName))
		}
		return ctrl.Result{}, fmt.Errorf("failed to get GameServer: %w", err)
	}

	// Check GameServer is running (Ready or Allocated)
	if gs.Status.State != gamev1alpha1.GameServerStateReady && gs.Status.State != gamev1alpha1.GameServerStateAllocated {
		return r.transitionBackupState(ctx, backup, gamev1alpha1.BackupStateFailed, "ServerNotRunning",
			fmt.Sprintf("GameServer must be Ready or Allocated, current state: %s", gs.Status.State))
	}

	// Check no other InProgress backup exists for this GameServer
	backupList := &gamev1alpha1.BackupList{}
	if err := r.List(ctx, backupList, client.InNamespace(backup.Namespace),
		client.MatchingLabels{util.LabelBackupGameServer: backup.Spec.GameServerName}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list backups: %w", err)
	}
	for i := range backupList.Items {
		other := &backupList.Items[i]
		if other.Name != backup.Name && other.Status.State == gamev1alpha1.BackupStateInProgress {
			return r.transitionBackupState(ctx, backup, gamev1alpha1.BackupStateFailed, "ConcurrentBackup",
				fmt.Sprintf("Another backup %s is already in progress for this server", other.Name))
		}
	}

	// Transition to InProgress
	now := metav1.Now()
	fresh := &gamev1alpha1.Backup{}
	if err := r.Get(ctx, types.NamespacedName{Name: backup.Name, Namespace: backup.Namespace}, fresh); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to re-fetch Backup: %w", err)
	}
	fresh.Status.State = gamev1alpha1.BackupStateInProgress
	fresh.Status.StartedAt = &now
	fresh.Status.Message = "Backup in progress"
	meta.SetStatusCondition(&fresh.Status.Conditions, metav1.Condition{
		Type:               gamev1alpha1.BackupConditionReady,
		Status:             metav1.ConditionFalse,
		Reason:             "BackupStarted",
		Message:            "Backup operation started",
		ObservedGeneration: fresh.Generation,
	})
	if err := r.Status().Update(ctx, fresh); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update backup status to InProgress: %w", err)
	}
	r.Recorder.Eventf(backup, corev1.EventTypeNormal, "BackupStarted", "Backup started for GameServer %s", backup.Spec.GameServerName)

	return ctrl.Result{Requeue: true}, nil
}

// reconcileInProgress handles the InProgress state: performs the actual backup.
func (r *BackupReconciler) reconcileInProgress(ctx context.Context, backup *gamev1alpha1.Backup, cfg *AdminConfig) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Reconciling InProgress backup", "name", backup.Name)

	// Fetch the referenced GameServer
	gs := &gamev1alpha1.GameServer{}
	gsKey := types.NamespacedName{Name: backup.Spec.GameServerName, Namespace: backup.Namespace}
	if err := r.Get(ctx, gsKey, gs); err != nil {
		if errors.IsNotFound(err) {
			return r.transitionBackupState(ctx, backup, gamev1alpha1.BackupStateFailed, "GameServerNotFound",
				fmt.Sprintf("GameServer %s not found", backup.Spec.GameServerName))
		}
		return ctrl.Result{}, fmt.Errorf("failed to get GameServer: %w", err)
	}

	// Check GameServer state is still running
	if gs.Status.State != gamev1alpha1.GameServerStateReady && gs.Status.State != gamev1alpha1.GameServerStateAllocated {
		return r.transitionBackupState(ctx, backup, gamev1alpha1.BackupStateFailed, "ServerNotRunning",
			fmt.Sprintf("GameServer is no longer running (state: %s)", gs.Status.State))
	}

	// Determine backup path
	backupPath := gs.Annotations[util.AnnotationBackupPath]
	if backupPath == "" {
		backupPath = "/data" // default
	}
	if len(backup.Spec.BackupPaths) > 0 {
		backupPath = backup.Spec.BackupPaths[0]
	}

	// Initialize S3 client
	if err := r.ensureS3Client(ctx, cfg); err != nil {
		return r.transitionBackupState(ctx, backup, gamev1alpha1.BackupStateFailed, "S3ClientError",
			fmt.Sprintf("Failed to initialize S3 client: %v", err))
	}

	// Auto-create bucket if not exists
	exists, err := r.s3Client.BucketExists(ctx, r.s3Bucket)
	if err != nil {
		return r.transitionBackupState(ctx, backup, gamev1alpha1.BackupStateFailed, "S3BucketCheckError",
			fmt.Sprintf("Failed to check S3 bucket: %v", err))
	}
	if !exists {
		if err := r.s3Client.MakeBucket(ctx, r.s3Bucket, minio.MakeBucketOptions{Region: cfg.BackupS3Region}); err != nil {
			return r.transitionBackupState(ctx, backup, gamev1alpha1.BackupStateFailed, "S3BucketCreateError",
				fmt.Sprintf("Failed to create S3 bucket: %v", err))
		}
		log.Info("Created S3 bucket", "bucket", r.s3Bucket)
	}

	// Perform the backup
	s3Key := fmt.Sprintf("backups/%s/%s/%s.tar.gz",
		gs.Namespace, gs.Name, time.Now().UTC().Format("20060102-150405"))

	backupCtx, cancel := context.WithTimeout(ctx, backupTimeout)
	defer cancel()

	uploadInfo, err := r.performBackup(backupCtx, gs.Namespace, gs.Name, backupPath, s3Key)
	if err != nil {
		log.Error(err, "Backup failed", "gameserver", gs.Name)
		return r.transitionBackupState(ctx, backup, gamev1alpha1.BackupStateFailed, "BackupFailed",
			fmt.Sprintf("Backup failed: %v", err))
	}

	// Success -- transition to Completed
	now := metav1.Now()
	fresh := &gamev1alpha1.Backup{}
	if err := r.Get(ctx, types.NamespacedName{Name: backup.Name, Namespace: backup.Namespace}, fresh); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to re-fetch Backup: %w", err)
	}
	fresh.Status.State = gamev1alpha1.BackupStateCompleted
	fresh.Status.S3Key = s3Key
	fresh.Status.S3Bucket = r.s3Bucket
	fresh.Status.Size = uploadInfo.Size
	fresh.Status.CompletedAt = &now
	fresh.Status.Message = "Backup completed successfully"
	meta.SetStatusCondition(&fresh.Status.Conditions, metav1.Condition{
		Type:               gamev1alpha1.BackupConditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             "BackupCompleted",
		Message:            fmt.Sprintf("Backup completed, size: %d bytes", uploadInfo.Size),
		ObservedGeneration: fresh.Generation,
	})
	if err := r.Status().Update(ctx, fresh); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update backup status to Completed: %w", err)
	}

	r.Recorder.Eventf(backup, corev1.EventTypeNormal, "BackupCompleted",
		"Backup completed for GameServer %s (size: %d bytes, key: %s)", gs.Name, uploadInfo.Size, s3Key)
	log.Info("Backup completed", "gameserver", gs.Name, "s3Key", s3Key, "size", uploadInfo.Size)

	return ctrl.Result{}, nil
}

// performBackup executes the backup pipeline: exec tar in pod -> gzip -> S3 upload.
func (r *BackupReconciler) performBackup(ctx context.Context, namespace, podName, backupPath, s3Key string) (minio.UploadInfo, error) {
	// Set up pipe: exec stdout -> gzip -> S3 upload
	pr, pw := io.Pipe()

	// Background goroutine: exec tar in pod, pipe through gzip to pipe writer
	errCh := make(chan error, 1)
	go func() {
		defer pw.Close()

		gzWriter := gzip.NewWriter(pw)
		defer gzWriter.Close()

		err := r.execTarFromPod(ctx, namespace, podName, backupPath, gzWriter)
		if err != nil {
			// Close gzWriter before pw to flush any buffered data
			gzWriter.Close()
			pw.CloseWithError(fmt.Errorf("tar exec failed: %w", err))
		}
		errCh <- err
	}()

	// Main goroutine: upload pipe reader to S3
	info, uploadErr := r.s3Client.PutObject(ctx, r.s3Bucket, s3Key, pr, -1,
		minio.PutObjectOptions{
			ContentType: "application/gzip",
			PartSize:    s3PartSize,
		})

	// Check exec goroutine error
	execErr := <-errCh
	if uploadErr != nil {
		return minio.UploadInfo{}, fmt.Errorf("S3 upload failed: %w", uploadErr)
	}
	if execErr != nil {
		return minio.UploadInfo{}, fmt.Errorf("tar exec failed: %w", execErr)
	}

	return info, nil
}

// performRestore executes the restore pipeline: S3 download -> gunzip -> tar into pod.
func (r *BackupReconciler) performRestore(ctx context.Context, namespace, podName, backupPath, s3Bucket, s3Key string) error {
	// Download from S3
	obj, err := r.s3Client.GetObject(ctx, s3Bucket, s3Key, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("S3 download failed: %w", err)
	}
	defer obj.Close()

	// Decompress gzip
	gzReader, err := gzip.NewReader(obj)
	if err != nil {
		return fmt.Errorf("gzip decompression failed: %w", err)
	}
	defer gzReader.Close()

	// Exec tar extraction in pod
	if err := r.execTarIntoPod(ctx, namespace, podName, backupPath, gzReader); err != nil {
		return fmt.Errorf("tar restore failed: %w", err)
	}

	return nil
}

// ensureS3Client lazily initializes the S3 client from AdminConfig and Secret credentials.
func (r *BackupReconciler) ensureS3Client(ctx context.Context, cfg *AdminConfig) error {
	if r.s3Client != nil {
		return nil
	}

	// Load S3 credentials from Secret
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{Name: s3CredentialsSecretName, Namespace: r.OperatorNamespace}
	if err := r.Get(ctx, secretKey, secret); err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("S3 credentials secret %s not found in namespace %s", s3CredentialsSecretName, r.OperatorNamespace)
		}
		return fmt.Errorf("failed to get S3 credentials secret: %w", err)
	}

	accessKeyID := string(secret.Data["accessKeyID"])
	secretAccessKey := string(secret.Data["secretAccessKey"])
	if accessKeyID == "" || secretAccessKey == "" {
		return fmt.Errorf("S3 credentials secret missing accessKeyID or secretAccessKey")
	}

	// Create minio client
	client, err := minio.New(cfg.BackupS3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: cfg.BackupS3UseSSL,
	})
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %w", err)
	}

	r.s3Client = client
	r.s3Bucket = cfg.BackupS3Bucket

	return nil
}

// execTarFromPod executes tar in the pod and writes the tar output to the provided writer.
func (r *BackupReconciler) execTarFromPod(ctx context.Context, namespace, podName, backupPath string, stdout io.Writer) error {
	req := r.Clientset.CoreV1().RESTClient().Post().
		Resource("pods").Name(podName).Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: "gameserver",
			Command:   []string{"tar", "-cf", "-", "-C", backupPath, "."},
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(r.RestConfig, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("exec setup failed: %w", err)
	}

	var stderr strings.Builder
	if err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: stdout,
		Stderr: &stderr,
	}); err != nil {
		stderrStr := stderr.String()
		if stderrStr != "" {
			return fmt.Errorf("exec failed: %w (stderr: %s)", err, stderrStr)
		}
		return fmt.Errorf("exec failed: %w", err)
	}

	return nil
}

// execTarIntoPod executes tar extraction in the pod, reading from the provided reader.
func (r *BackupReconciler) execTarIntoPod(ctx context.Context, namespace, podName, backupPath string, stdin io.Reader) error {
	req := r.Clientset.CoreV1().RESTClient().Post().
		Resource("pods").Name(podName).Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: "gameserver",
			Command:   []string{"tar", "-xf", "-", "-C", backupPath},
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(r.RestConfig, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("exec setup failed: %w", err)
	}

	var stdout, stderr strings.Builder
	if err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: &stdout,
		Stderr: &stderr,
	}); err != nil {
		stderrStr := stderr.String()
		if stderrStr != "" {
			return fmt.Errorf("exec failed: %w (stderr: %s)", err, stderrStr)
		}
		return fmt.Errorf("exec failed: %w", err)
	}

	return nil
}

// transitionBackupState transitions a Backup to a new state with proper condition updates and events.
func (r *BackupReconciler) transitionBackupState(ctx context.Context, backup *gamev1alpha1.Backup, newState gamev1alpha1.BackupState, reason, message string) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	oldState := backup.Status.State

	log.Info("Transitioning backup state", "name", backup.Name, "from", oldState, "to", newState, "reason", reason)

	// Re-fetch to avoid conflicts
	fresh := &gamev1alpha1.Backup{}
	if err := r.Get(ctx, types.NamespacedName{Name: backup.Name, Namespace: backup.Namespace}, fresh); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to re-fetch Backup for status update: %w", err)
	}

	fresh.Status.State = newState
	fresh.Status.Message = message

	// Update conditions based on new state
	switch newState {
	case gamev1alpha1.BackupStatePending:
		meta.SetStatusCondition(&fresh.Status.Conditions, metav1.Condition{
			Type:               gamev1alpha1.BackupConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             reason,
			Message:            message,
			ObservedGeneration: fresh.Generation,
		})
	case gamev1alpha1.BackupStateFailed:
		meta.SetStatusCondition(&fresh.Status.Conditions, metav1.Condition{
			Type:               gamev1alpha1.BackupConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             reason,
			Message:            message,
			ObservedGeneration: fresh.Generation,
		})
		if fresh.Status.CompletedAt == nil {
			now := metav1.Now()
			fresh.Status.CompletedAt = &now
		}
	}

	if err := r.Status().Update(ctx, fresh); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update backup status: %w", err)
	}

	eventType := corev1.EventTypeNormal
	if newState == gamev1alpha1.BackupStateFailed {
		eventType = corev1.EventTypeWarning
	}
	r.Recorder.Eventf(backup, eventType, "StateChanged", "Backup state changed from %s to %s: %s", oldState, newState, message)

	// Requeue for non-terminal states
	if !gamev1alpha1.IsBackupTerminal(newState) {
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{}, nil
}

// reconcileGameServerSchedule handles scheduled backup creation for a GameServer.
func (r *BackupReconciler) reconcileGameServerSchedule(ctx context.Context, gs *gamev1alpha1.GameServer) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Check if schedule annotation exists
	schedule := gs.Annotations[util.AnnotationBackupSchedule]
	if schedule == "" {
		return ctrl.Result{}, nil
	}

	// Only schedule backups for running servers
	if gs.Status.State != gamev1alpha1.GameServerStateReady && gs.Status.State != gamev1alpha1.GameServerStateAllocated {
		return ctrl.Result{}, nil
	}

	// Parse cron expression
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sched, err := parser.Parse(schedule)
	if err != nil {
		log.Error(err, "Invalid backup schedule cron expression", "schedule", schedule, "gameserver", gs.Name)
		return ctrl.Result{}, nil // Don't requeue for invalid cron
	}

	// Check last backup time
	now := time.Now().UTC()
	var lastBackup time.Time
	if lastStr := gs.Annotations[util.AnnotationLastBackupTime]; lastStr != "" {
		if t, err := time.Parse(time.RFC3339, lastStr); err == nil {
			lastBackup = t
		}
	}

	// Calculate next scheduled time
	var nextTime time.Time
	if lastBackup.IsZero() {
		// No previous backup -- check if we should create one now
		// Use creation time as reference
		nextTime = sched.Next(gs.CreationTimestamp.Time)
	} else {
		nextTime = sched.Next(lastBackup)
	}

	// If not yet due, requeue for next scheduled time
	if now.Before(nextTime) {
		requeueAfter := nextTime.Sub(now)
		log.Info("Next scheduled backup not due yet", "gameserver", gs.Name, "next", nextTime, "requeueAfter", requeueAfter)
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}

	// Time for a backup -- create a Backup CR
	backupName := fmt.Sprintf("%s-scheduled-%s", gs.Name, now.Format("20060102-150405"))
	backup := &gamev1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupName,
			Namespace: gs.Namespace,
			Labels: map[string]string{
				util.LabelBackupGameServer:    gs.Name,
				util.LabelManagedByKterodactyl: util.ManagedByValue,
			},
		},
		Spec: gamev1alpha1.BackupSpec{
			GameServerName: gs.Name,
		},
	}

	if err := r.Create(ctx, backup); err != nil {
		if errors.IsAlreadyExists(err) {
			log.Info("Scheduled backup already exists", "name", backupName)
		} else {
			return ctrl.Result{}, fmt.Errorf("failed to create scheduled backup: %w", err)
		}
	} else {
		log.Info("Created scheduled backup", "backup", backupName, "gameserver", gs.Name)
		r.Recorder.Eventf(gs, corev1.EventTypeNormal, "ScheduledBackup", "Created scheduled backup %s", backupName)
	}

	// Update last backup time annotation
	freshGS := &gamev1alpha1.GameServer{}
	if err := r.Get(ctx, types.NamespacedName{Name: gs.Name, Namespace: gs.Namespace}, freshGS); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to re-fetch GameServer: %w", err)
	}
	if freshGS.Annotations == nil {
		freshGS.Annotations = make(map[string]string)
	}
	freshGS.Annotations[util.AnnotationLastBackupTime] = now.Format(time.RFC3339)
	if err := r.Update(ctx, freshGS); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update last backup time: %w", err)
	}

	// Enforce retention
	opNs := r.OperatorNamespace
	if opNs == "" {
		opNs = defaultOperatorNamespace
	}
	adminCfg, err := LoadAdminConfig(ctx, r.Client, opNs)
	if err != nil {
		log.Error(err, "Failed to load admin config for retention check")
	} else {
		retentionCount := adminCfg.BackupRetentionCount
		if retStr := gs.Annotations[util.AnnotationBackupRetention]; retStr != "" {
			if n, err := strconv.Atoi(retStr); err == nil {
				retentionCount = n
			}
		}
		if err := r.enforceRetention(ctx, gs.Namespace, gs.Name, retentionCount); err != nil {
			log.Error(err, "Failed to enforce backup retention", "gameserver", gs.Name)
		}
	}

	// Requeue for next scheduled time
	nextAfterNow := sched.Next(now)
	return ctrl.Result{RequeueAfter: nextAfterNow.Sub(now)}, nil
}

// enforceRetention deletes old backups beyond the retention count.
func (r *BackupReconciler) enforceRetention(ctx context.Context, namespace, gameServerName string, maxCount int) error {
	log := logf.FromContext(ctx)

	if maxCount <= 0 {
		return nil
	}

	// List all completed backups for this GameServer
	backupList := &gamev1alpha1.BackupList{}
	if err := r.List(ctx, backupList, client.InNamespace(namespace),
		client.MatchingLabels{util.LabelBackupGameServer: gameServerName}); err != nil {
		return fmt.Errorf("failed to list backups for retention: %w", err)
	}

	// Filter to completed backups only
	var completed []gamev1alpha1.Backup
	for _, b := range backupList.Items {
		if b.Status.State == gamev1alpha1.BackupStateCompleted {
			completed = append(completed, b)
		}
	}

	if len(completed) <= maxCount {
		return nil
	}

	// Sort by creation time (newest first)
	sort.Slice(completed, func(i, j int) bool {
		return completed[i].CreationTimestamp.After(completed[j].CreationTimestamp.Time)
	})

	// Delete oldest beyond retention count
	for i := maxCount; i < len(completed); i++ {
		old := &completed[i]
		log.Info("Deleting old backup for retention", "backup", old.Name, "gameserver", gameServerName)

		// Delete S3 object if present
		if old.Status.S3Key != "" && old.Status.S3Bucket != "" && r.s3Client != nil {
			if err := r.s3Client.RemoveObject(ctx, old.Status.S3Bucket, old.Status.S3Key, minio.RemoveObjectOptions{}); err != nil {
				log.Error(err, "Failed to delete S3 object during retention cleanup", "key", old.Status.S3Key)
				// Continue with CR deletion even if S3 delete fails
			}
		}

		// Delete the Backup CR
		if err := r.Delete(ctx, old); err != nil && !errors.IsNotFound(err) {
			log.Error(err, "Failed to delete old backup CR", "backup", old.Name)
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gamev1alpha1.Backup{}).
		Watches(&gamev1alpha1.GameServer{}, handler.EnqueueRequestsFromMapFunc(
			func(ctx context.Context, obj client.Object) []reconcile.Request {
				gs, ok := obj.(*gamev1alpha1.GameServer)
				if !ok {
					return nil
				}
				// Only enqueue if GameServer has a backup schedule annotation
				if gs.Annotations[util.AnnotationBackupSchedule] == "" {
					return nil
				}
				// Create a synthetic reconcile request; the reconciler will detect
				// it's a GameServer schedule event via the name pattern
				return []reconcile.Request{
					{
						NamespacedName: types.NamespacedName{
							Name:      "schedule-" + gs.Name,
							Namespace: gs.Namespace,
						},
					},
				}
			},
		)).
		Named("backup").
		Complete(r)
}
