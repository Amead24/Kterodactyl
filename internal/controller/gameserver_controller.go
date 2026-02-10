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
	"context"
	"fmt"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	gamev1alpha1 "github.com/kterodactyl/kterodactyl/api/v1alpha1"
	"github.com/kterodactyl/kterodactyl/internal/util"
)

const (
	// requeueDelay is the standard requeue delay for state transitions.
	requeueDelay = 5 * time.Second

	// startingTimeout is the maximum time a GameServer can be in Starting state.
	startingTimeout = 5 * time.Minute

	// shutdownGracePeriod is the grace period for Pod deletion during shutdown.
	shutdownGracePeriod int64 = 30

	// defaultMaxServersGlobal is the default maximum number of GameServers cluster-wide.
	defaultMaxServersGlobal = 100

	// defaultMaxServersPerUser is the default maximum number of GameServers per user.
	defaultMaxServersPerUser = 5

	// defaultOperatorNamespace is the default namespace where the operator is deployed.
	defaultOperatorNamespace = "kterodactyl-system"

	// adminConfigMapName is the name of the admin configuration ConfigMap.
	adminConfigMapName = "kterodactyl-admin-config"
)

// GameServerReconciler reconciles a GameServer object.
type GameServerReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	Recorder          record.EventRecorder
	OperatorNamespace string
}

// AdminConfig holds admin-configurable resource limits loaded from a ConfigMap.
type AdminConfig struct {
	// Server count limits
	MaxServersGlobal  int
	MaxServersPerUser int

	// Per-user namespace quota limits
	QuotaCPURequests    resource.Quantity
	QuotaCPULimits      resource.Quantity
	QuotaMemoryRequests resource.Quantity
	QuotaMemoryLimits   resource.Quantity
	QuotaPods           resource.Quantity
	QuotaPVCs           resource.Quantity
	QuotaStorage        resource.Quantity

	// Per-container limits (LimitRange)
	DefaultCPU           resource.Quantity
	DefaultMemory        resource.Quantity
	DefaultRequestCPU    resource.Quantity
	DefaultRequestMemory resource.Quantity
	MaxCPU               resource.Quantity
	MaxMemory            resource.Quantity
	MinCPU               resource.Quantity
	MinMemory            resource.Quantity

	// Networking / DNS configuration
	BaseDomain                 string // Base domain for DNS names (empty = DNS disabled)
	GatewayName                string // Name of the Gateway resource HTTPRoutes attach to
	GatewayNamespace           string // Namespace where the Gateway lives
	GatewayControllerNamespace string // Namespace of the gateway controller data plane (for NetworkPolicy)
}

// DefaultAdminConfig returns an AdminConfig with sensible default values.
// These defaults are used when the ConfigMap does not exist.
func DefaultAdminConfig() *AdminConfig {
	return &AdminConfig{
		MaxServersGlobal:           defaultMaxServersGlobal,
		MaxServersPerUser:          defaultMaxServersPerUser,
		QuotaCPURequests:           resource.MustParse("4"),
		QuotaCPULimits:             resource.MustParse("8"),
		QuotaMemoryRequests:        resource.MustParse("8Gi"),
		QuotaMemoryLimits:          resource.MustParse("16Gi"),
		QuotaPods:                  resource.MustParse("5"),
		QuotaPVCs:                  resource.MustParse("5"),
		QuotaStorage:               resource.MustParse("50Gi"),
		DefaultCPU:                 resource.MustParse("2"),
		DefaultMemory:              resource.MustParse("4Gi"),
		DefaultRequestCPU:          resource.MustParse("500m"),
		DefaultRequestMemory:       resource.MustParse("1Gi"),
		MaxCPU:                     resource.MustParse("4"),
		MaxMemory:                  resource.MustParse("8Gi"),
		MinCPU:                     resource.MustParse("100m"),
		MinMemory:                  resource.MustParse("128Mi"),
		BaseDomain:                 "",                      // Empty means DNS disabled
		GatewayName:                "kterodactyl-gateway",
		GatewayNamespace:           "kterodactyl-system",
		GatewayControllerNamespace: "envoy-gateway-system",
	}
}

// LoadAdminConfig reads the admin config ConfigMap from the operator namespace.
// Returns defaults if the ConfigMap does not exist.
func LoadAdminConfig(ctx context.Context, c client.Client, namespace string) (*AdminConfig, error) {
	cfg := DefaultAdminConfig()

	cm := &corev1.ConfigMap{}
	err := c.Get(ctx, types.NamespacedName{
		Name:      adminConfigMapName,
		Namespace: namespace,
	}, cm)
	if err != nil {
		if errors.IsNotFound(err) {
			// ConfigMap not found; return defaults (operator works without it)
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to get admin config ConfigMap: %w", err)
	}

	// Parse integer fields
	if v, ok := cm.Data["maxServersGlobal"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxServersGlobal = n
		}
	}
	if v, ok := cm.Data["maxServersPerUser"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxServersPerUser = n
		}
	}

	// Parse resource quantities
	parseQuantity := func(key string, target *resource.Quantity) {
		if v, ok := cm.Data[key]; ok {
			if q, err := resource.ParseQuantity(v); err == nil {
				*target = q
			}
		}
	}

	parseQuantity("quotaCPURequests", &cfg.QuotaCPURequests)
	parseQuantity("quotaCPULimits", &cfg.QuotaCPULimits)
	parseQuantity("quotaMemoryRequests", &cfg.QuotaMemoryRequests)
	parseQuantity("quotaMemoryLimits", &cfg.QuotaMemoryLimits)
	parseQuantity("quotaPods", &cfg.QuotaPods)
	parseQuantity("quotaPVCs", &cfg.QuotaPVCs)
	parseQuantity("quotaStorage", &cfg.QuotaStorage)
	parseQuantity("defaultCPU", &cfg.DefaultCPU)
	parseQuantity("defaultMemory", &cfg.DefaultMemory)
	parseQuantity("defaultRequestCPU", &cfg.DefaultRequestCPU)
	parseQuantity("defaultRequestMemory", &cfg.DefaultRequestMemory)
	parseQuantity("maxCPU", &cfg.MaxCPU)
	parseQuantity("maxMemory", &cfg.MaxMemory)
	parseQuantity("minCPU", &cfg.MinCPU)
	parseQuantity("minMemory", &cfg.MinMemory)

	// Parse networking / DNS fields
	if v, ok := cm.Data["baseDomain"]; ok {
		cfg.BaseDomain = v
	}
	if v, ok := cm.Data["gatewayName"]; ok && v != "" {
		cfg.GatewayName = v
	}
	if v, ok := cm.Data["gatewayNamespace"]; ok && v != "" {
		cfg.GatewayNamespace = v
	}
	if v, ok := cm.Data["gatewayControllerNamespace"]; ok && v != "" {
		cfg.GatewayControllerNamespace = v
	}

	return cfg, nil
}

// +kubebuilder:rbac:groups=game.kterodactyl.io,resources=gameservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=game.kterodactyl.io,resources=gameservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=game.kterodactyl.io,resources=gameservers/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=resourcequotas,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=limitranges,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is the main reconciliation loop for GameServer resources.
// It manages the full lifecycle: creation, state transitions, Pod management,
// and cleanup via finalizers.
func (r *GameServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Fetch the GameServer CR
	gs := &gamev1alpha1.GameServer{}
	if err := r.Get(ctx, req.NamespacedName, gs); err != nil {
		if errors.IsNotFound(err) {
			// CR was deleted between queue and reconcile; nothing to do
			log.Info("GameServer resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get GameServer: %w", err)
	}

	// 2. Handle deletion: if DeletionTimestamp is set, clean up
	if !gs.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, gs)
	}

	// 3. Add finalizer if not present
	if !controllerutil.ContainsFinalizer(gs, gamev1alpha1.FinalizerName) {
		controllerutil.AddFinalizer(gs, gamev1alpha1.FinalizerName)
		if err := r.Update(ctx, gs); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// 4. Initialize state if empty
	if gs.Status.State == "" {
		return r.initializeState(ctx, gs)
	}

	// 5. Dispatch to state-specific reconciler
	switch gs.Status.State {
	case gamev1alpha1.GameServerStateCreating:
		return r.reconcileCreating(ctx, gs)
	case gamev1alpha1.GameServerStateStarting:
		return r.reconcileStarting(ctx, gs)
	case gamev1alpha1.GameServerStateReady:
		return r.reconcileReady(ctx, gs)
	case gamev1alpha1.GameServerStateAllocated:
		return r.reconcileAllocated(ctx, gs)
	case gamev1alpha1.GameServerStateShutdown:
		return r.reconcileShutdown(ctx, gs)
	case gamev1alpha1.GameServerStateError:
		return r.reconcileError(ctx, gs)
	default:
		log.Error(nil, "Unknown state", "state", gs.Status.State)
		return ctrl.Result{}, nil
	}
}

// initializeState sets the initial Creating state on a new GameServer.
func (r *GameServerReconciler) initializeState(ctx context.Context, gs *gamev1alpha1.GameServer) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Initializing GameServer state", "name", gs.Name)

	gs.Status.State = gamev1alpha1.GameServerStateCreating
	meta.SetStatusCondition(&gs.Status.Conditions, metav1.Condition{
		Type:               gamev1alpha1.TypeProgressing,
		Status:             metav1.ConditionTrue,
		Reason:             "Initializing",
		Message:            "GameServer is being created",
		ObservedGeneration: gs.Generation,
	})
	meta.SetStatusCondition(&gs.Status.Conditions, metav1.Condition{
		Type:               gamev1alpha1.TypeReady,
		Status:             metav1.ConditionFalse,
		Reason:             "Initializing",
		Message:            "GameServer is not yet ready",
		ObservedGeneration: gs.Generation,
	})

	if err := r.Status().Update(ctx, gs); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to set initial state: %w", err)
	}

	r.Recorder.Event(gs, corev1.EventTypeNormal, "StateChanged", "GameServer state set to Creating")
	return ctrl.Result{Requeue: true}, nil
}

// reconcileCreating handles the Creating state: validates labels, loads admin config,
// checks server limits, ensures namespace isolation, creates/updates Pod, and transitions to Starting.
func (r *GameServerReconciler) reconcileCreating(ctx context.Context, gs *gamev1alpha1.GameServer) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Reconciling Creating state", "name", gs.Name)

	// Extract owner from label
	owner := gs.Labels[util.LabelOwner]
	if owner == "" {
		log.Error(nil, "Missing owner label", "label", util.LabelOwner)
		return r.transitionState(ctx, gs, gamev1alpha1.GameServerStateError, "MissingOwnerLabel", "GameServer is missing required owner label")
	}

	// Load admin configuration from ConfigMap (returns defaults if not found)
	opNs := r.OperatorNamespace
	if opNs == "" {
		opNs = defaultOperatorNamespace
	}
	adminCfg, err := LoadAdminConfig(ctx, r.Client, opNs)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to load admin config: %w", err)
	}

	// Check global server count limit
	gsList := &gamev1alpha1.GameServerList{}
	if err := r.List(ctx, gsList); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list GameServers for global count check: %w", err)
	}
	if len(gsList.Items) > adminCfg.MaxServersGlobal {
		log.Info("Global server limit exceeded", "count", len(gsList.Items), "limit", adminCfg.MaxServersGlobal)
		return r.transitionState(ctx, gs, gamev1alpha1.GameServerStateError, "GlobalLimitExceeded",
			fmt.Sprintf("Global server limit of %d exceeded (current: %d)", adminCfg.MaxServersGlobal, len(gsList.Items)))
	}

	// Check per-user server count limit
	userNamespace := util.UserNamespace(owner)
	userGsList := &gamev1alpha1.GameServerList{}
	if err := r.List(ctx, userGsList, client.InNamespace(userNamespace)); err != nil {
		// If namespace doesn't exist yet, that's fine -- zero servers
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to list GameServers for per-user count check: %w", err)
		}
	}
	if len(userGsList.Items) > adminCfg.MaxServersPerUser {
		log.Info("Per-user server limit exceeded", "user", owner, "count", len(userGsList.Items), "limit", adminCfg.MaxServersPerUser)
		return r.transitionState(ctx, gs, gamev1alpha1.GameServerStateError, "UserLimitExceeded",
			fmt.Sprintf("Per-user server limit of %d exceeded for user %s (current: %d)", adminCfg.MaxServersPerUser, owner, len(userGsList.Items)))
	}

	// Ensure user namespace with ResourceQuota, LimitRange, and NetworkPolicy
	if err := r.ensureUserNamespace(ctx, owner, adminCfg); err != nil {
		log.Error(err, "Failed to ensure user namespace", "owner", owner)
		return r.transitionState(ctx, gs, gamev1alpha1.GameServerStateError, "NamespaceSetupFailed",
			fmt.Sprintf("Failed to set up user namespace: %v", err))
	}

	// Create or update the Pod
	if err := r.reconcilePod(ctx, gs); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile Pod: %w", err)
	}

	// Transition to Starting
	return r.transitionState(ctx, gs, gamev1alpha1.GameServerStateStarting, "PodCreated", "Pod created, waiting for it to become ready")
}

// reconcileStarting handles the Starting state: checks Pod status, transitions to Ready or Error.
func (r *GameServerReconciler) reconcileStarting(ctx context.Context, gs *gamev1alpha1.GameServer) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Reconciling Starting state", "name", gs.Name)

	// Fetch the owned Pod
	pod := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{Name: gs.Name, Namespace: gs.Namespace}, pod)
	if err != nil {
		if errors.IsNotFound(err) {
			// Pod was deleted externally; transition back to Creating to recreate
			log.Info("Pod not found while Starting, transitioning back to Creating")
			return r.transitionState(ctx, gs, gamev1alpha1.GameServerStateCreating, "PodDeleted", "Pod was deleted externally, recreating")
		}
		return ctrl.Result{}, fmt.Errorf("failed to get Pod: %w", err)
	}

	// Check if Pod is Running and all containers ready
	if pod.Status.Phase == corev1.PodRunning && isPodReady(pod) {
		log.Info("Pod is ready, transitioning to Ready")
		return r.transitionState(ctx, gs, gamev1alpha1.GameServerStateReady, "PodReady", "Pod is running and all containers are ready")
	}

	// Check for failure conditions
	if pod.Status.Phase == corev1.PodFailed {
		log.Info("Pod has failed, transitioning to Error")
		return r.transitionState(ctx, gs, gamev1alpha1.GameServerStateError, "PodFailed", fmt.Sprintf("Pod failed with reason: %s", pod.Status.Reason))
	}

	// Check for starting timeout (>5 minutes in Starting state)
	if gs.Status.Conditions != nil {
		for _, c := range gs.Status.Conditions {
			if c.Type == gamev1alpha1.TypeProgressing && c.Status == metav1.ConditionTrue {
				if time.Since(c.LastTransitionTime.Time) > startingTimeout {
					log.Info("Starting timeout exceeded, transitioning to Error")
					return r.transitionState(ctx, gs, gamev1alpha1.GameServerStateError, "StartingTimeout", "Pod did not become ready within 5 minutes")
				}
			}
		}
	}

	// Still starting; requeue to check again
	return ctrl.Result{RequeueAfter: requeueDelay}, nil
}

// reconcileReady handles the Ready state: verifies Pod, checks for allocation, updates status.
func (r *GameServerReconciler) reconcileReady(ctx context.Context, gs *gamev1alpha1.GameServer) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Reconciling Ready state", "name", gs.Name)

	// Verify Pod still exists and is Running
	pod := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{Name: gs.Name, Namespace: gs.Namespace}, pod)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Pod not found while Ready, transitioning to Error")
			return r.transitionState(ctx, gs, gamev1alpha1.GameServerStateError, "PodNotFound", "Pod disappeared while GameServer was Ready")
		}
		return ctrl.Result{}, fmt.Errorf("failed to get Pod: %w", err)
	}

	if pod.Status.Phase != corev1.PodRunning || !isPodReady(pod) {
		log.Info("Pod is no longer running/ready, transitioning to Error")
		return r.transitionState(ctx, gs, gamev1alpha1.GameServerStateError, "PodNotReady", "Pod is no longer running or ready")
	}

	// Check for allocation annotation
	if gs.Annotations != nil && gs.Annotations[util.AnnotationAllocated] == "true" {
		log.Info("GameServer has allocation annotation, transitioning to Allocated")
		return r.transitionState(ctx, gs, gamev1alpha1.GameServerStateAllocated, "Allocated", "GameServer has been allocated")
	}

	// Update status with Pod IP and ports
	if err := r.updateReadyStatus(ctx, gs, pod); err != nil {
		return ctrl.Result{}, err
	}

	// No requeue needed; watch events will trigger reconciliation
	return ctrl.Result{}, nil
}

// reconcileAllocated handles the Allocated state: verifies Pod, checks for deallocation/shutdown.
func (r *GameServerReconciler) reconcileAllocated(ctx context.Context, gs *gamev1alpha1.GameServer) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Reconciling Allocated state", "name", gs.Name)

	// Verify Pod still exists and is Running
	pod := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{Name: gs.Name, Namespace: gs.Namespace}, pod)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Pod not found while Allocated, transitioning to Error")
			return r.transitionState(ctx, gs, gamev1alpha1.GameServerStateError, "PodNotFound", "Pod disappeared while GameServer was Allocated")
		}
		return ctrl.Result{}, fmt.Errorf("failed to get Pod: %w", err)
	}

	if pod.Status.Phase != corev1.PodRunning || !isPodReady(pod) {
		log.Info("Pod is no longer running/ready while Allocated, transitioning to Error")
		return r.transitionState(ctx, gs, gamev1alpha1.GameServerStateError, "PodNotReady", "Pod is no longer running or ready while Allocated")
	}

	// Check if allocation annotation removed (deallocated)
	if gs.Annotations == nil || gs.Annotations[util.AnnotationAllocated] != "true" {
		log.Info("Allocation annotation removed, transitioning back to Ready")
		return r.transitionState(ctx, gs, gamev1alpha1.GameServerStateReady, "Deallocated", "GameServer has been deallocated")
	}

	// No requeue needed
	return ctrl.Result{}, nil
}

// reconcileShutdown handles the Shutdown state: deletes owned Pod with grace period.
func (r *GameServerReconciler) reconcileShutdown(ctx context.Context, gs *gamev1alpha1.GameServer) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Reconciling Shutdown state", "name", gs.Name)

	// Delete the owned Pod if it exists
	pod := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{Name: gs.Name, Namespace: gs.Namespace}, pod)
	if err != nil {
		if errors.IsNotFound(err) {
			// Pod already gone; nothing more to do
			log.Info("Pod already deleted in Shutdown state")
		} else {
			return ctrl.Result{}, fmt.Errorf("failed to get Pod during shutdown: %w", err)
		}
	} else {
		// Delete with grace period
		gracePeriod := shutdownGracePeriod
		if err := r.Delete(ctx, pod, &client.DeleteOptions{
			GracePeriodSeconds: &gracePeriod,
		}); err != nil && !errors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to delete Pod during shutdown: %w", err)
		}
		r.Recorder.Event(gs, corev1.EventTypeNormal, "PodDeleted", "Pod deleted during shutdown")
	}

	// Update condition
	if err := r.refreshAndUpdateCondition(ctx, gs, gamev1alpha1.TypeReady, metav1.ConditionFalse, "ShuttingDown", "GameServer is shutting down"); err != nil {
		return ctrl.Result{}, err
	}

	// No requeue; GameServer CR stays in Shutdown state until user deletes it
	return ctrl.Result{}, nil
}

// reconcileError handles the Error state: logs error, sets degraded condition.
func (r *GameServerReconciler) reconcileError(ctx context.Context, gs *gamev1alpha1.GameServer) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("GameServer in Error state", "name", gs.Name, "conditions", gs.Status.Conditions)

	// Set Degraded condition
	if err := r.refreshAndUpdateCondition(ctx, gs, gamev1alpha1.TypeDegraded, metav1.ConditionTrue, "Error", "GameServer has encountered an error"); err != nil {
		return ctrl.Result{}, err
	}

	// No requeue; manual intervention required (user can delete and recreate)
	return ctrl.Result{}, nil
}

// handleDeletion handles cleanup when a GameServer is being deleted.
func (r *GameServerReconciler) handleDeletion(ctx context.Context, gs *gamev1alpha1.GameServer) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if controllerutil.ContainsFinalizer(gs, gamev1alpha1.FinalizerName) {
		log.Info("Handling deletion for GameServer", "name", gs.Name)

		// Delete owned Pod if it still exists
		pod := &corev1.Pod{}
		err := r.Get(ctx, types.NamespacedName{Name: gs.Name, Namespace: gs.Namespace}, pod)
		if err == nil {
			if delErr := r.Delete(ctx, pod); delErr != nil && !errors.IsNotFound(delErr) {
				return ctrl.Result{}, fmt.Errorf("failed to delete Pod during finalization: %w", delErr)
			}
			log.Info("Deleted owned Pod during finalization", "pod", pod.Name)
		} else if !errors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to get Pod during finalization: %w", err)
		}

		// Remove finalizer
		controllerutil.RemoveFinalizer(gs, gamev1alpha1.FinalizerName)
		if err := r.Update(ctx, gs); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}

		r.Recorder.Event(gs, corev1.EventTypeNormal, "Finalized", "GameServer finalized and resources cleaned up")
		log.Info("Successfully finalized GameServer", "name", gs.Name)
	}

	return ctrl.Result{}, nil
}

// reconcilePod creates or updates the Pod for a GameServer using CreateOrUpdate.
func (r *GameServerReconciler) reconcilePod(ctx context.Context, gs *gamev1alpha1.GameServer) error {
	log := logf.FromContext(ctx)

	owner := gs.Labels[util.LabelOwner]
	gameType := gs.Spec.GameType

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gs.Name,
			Namespace: gs.Namespace,
		},
	}

	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, pod, func() error {
		// Set labels
		pod.Labels = util.GameServerLabels(owner, gameType)

		// Set owner reference
		if err := ctrl.SetControllerReference(gs, pod, r.Scheme); err != nil {
			return fmt.Errorf("failed to set owner reference: %w", err)
		}

		// Build container ports
		var containerPorts []corev1.ContainerPort
		for _, p := range gs.Spec.Ports {
			containerPorts = append(containerPorts, corev1.ContainerPort{
				Name:          p.Name,
				ContainerPort: p.ContainerPort,
				Protocol:      p.Protocol,
			})
		}

		// Build environment variables from Parameters
		var envVars []corev1.EnvVar
		for k, v := range gs.Spec.Parameters {
			envVars = append(envVars, corev1.EnvVar{
				Name:  k,
				Value: v,
			})
		}

		// Define the Pod spec
		pod.Spec = corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:      "gameserver",
					Image:     gs.Spec.Image,
					Ports:     containerPorts,
					Resources: gs.Spec.Resources,
					Env:       envVars,
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create/update Pod: %w", err)
	}

	log.Info("Pod reconciled", "name", pod.Name, "result", result)
	r.Recorder.Eventf(gs, corev1.EventTypeNormal, "PodReconciled", "Pod %s %s", pod.Name, result)
	return nil
}

// ensureUserNamespace creates or updates a user namespace with ResourceQuota, LimitRange, and NetworkPolicy.
// The namespace is NOT owned by any GameServer (namespaces are cluster-scoped, GameServer is namespace-scoped).
func (r *GameServerReconciler) ensureUserNamespace(ctx context.Context, username string, cfg *AdminConfig) error {
	log := logf.FromContext(ctx)
	namespaceName := util.UserNamespace(username)
	log.Info("Ensuring user namespace", "namespace", namespaceName, "user", username)

	// Create or update the namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
		},
	}

	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, ns, func() error {
		if ns.Labels == nil {
			ns.Labels = make(map[string]string)
		}
		ns.Labels[util.LabelManagedByKterodactyl] = util.ManagedByValue
		ns.Labels[util.LabelUser] = username
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create/update namespace %s: %w", namespaceName, err)
	}
	log.Info("Namespace reconciled", "namespace", namespaceName, "result", result)

	// Ensure ResourceQuota, LimitRange, and NetworkPolicy in the namespace
	if err := r.ensureResourceQuota(ctx, namespaceName, cfg); err != nil {
		return fmt.Errorf("failed to ensure ResourceQuota in %s: %w", namespaceName, err)
	}
	if err := r.ensureLimitRange(ctx, namespaceName, cfg); err != nil {
		return fmt.Errorf("failed to ensure LimitRange in %s: %w", namespaceName, err)
	}
	if err := r.ensureNetworkPolicy(ctx, namespaceName); err != nil {
		return fmt.Errorf("failed to ensure NetworkPolicy in %s: %w", namespaceName, err)
	}

	return nil
}

// ensureResourceQuota creates or updates the ResourceQuota in a user namespace using admin config values.
func (r *GameServerReconciler) ensureResourceQuota(ctx context.Context, namespace string, cfg *AdminConfig) error {
	quota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "user-quota",
			Namespace: namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, quota, func() error {
		quota.Spec = corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				corev1.ResourceRequestsCPU:            cfg.QuotaCPURequests,
				corev1.ResourceRequestsMemory:         cfg.QuotaMemoryRequests,
				corev1.ResourceLimitsCPU:              cfg.QuotaCPULimits,
				corev1.ResourceLimitsMemory:           cfg.QuotaMemoryLimits,
				corev1.ResourcePods:                   cfg.QuotaPods,
				corev1.ResourcePersistentVolumeClaims: cfg.QuotaPVCs,
				corev1.ResourceRequestsStorage:        cfg.QuotaStorage,
			},
		}
		return nil
	})
	return err
}

// ensureLimitRange creates or updates the LimitRange in a user namespace using admin config values.
func (r *GameServerReconciler) ensureLimitRange(ctx context.Context, namespace string, cfg *AdminConfig) error {
	lr := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gameserver-limits",
			Namespace: namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, lr, func() error {
		lr.Spec = corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{
				{
					Type: corev1.LimitTypeContainer,
					Default: corev1.ResourceList{
						corev1.ResourceCPU:    cfg.DefaultCPU,
						corev1.ResourceMemory: cfg.DefaultMemory,
					},
					DefaultRequest: corev1.ResourceList{
						corev1.ResourceCPU:    cfg.DefaultRequestCPU,
						corev1.ResourceMemory: cfg.DefaultRequestMemory,
					},
					Max: corev1.ResourceList{
						corev1.ResourceCPU:    cfg.MaxCPU,
						corev1.ResourceMemory: cfg.MaxMemory,
					},
					Min: corev1.ResourceList{
						corev1.ResourceCPU:    cfg.MinCPU,
						corev1.ResourceMemory: cfg.MinMemory,
					},
				},
			},
		}
		return nil
	})
	return err
}

// ensureNetworkPolicy creates or updates the NetworkPolicy in a user namespace.
// Rules: deny cross-namespace, allow same namespace, allow from operator namespace,
// allow DNS (kube-system port 53), allow internet (block private ranges).
func (r *GameServerReconciler) ensureNetworkPolicy(ctx context.Context, namespace string) error {
	np := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deny-cross-namespace",
			Namespace: namespace,
		},
	}

	dnsPort := intstr.FromInt32(53)

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, np, func() error {
		np.Spec = networkingv1.NetworkPolicySpec{
			// Apply to all pods in the namespace
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					// Allow from same namespace
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{},
						},
					},
				},
				{
					// Allow from kterodactyl-system namespace
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": r.operatorNs(),
								},
							},
						},
					},
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					// Allow to same namespace
					To: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{},
						},
					},
				},
				{
					// Allow DNS to kube-system
					To: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": "kube-system",
								},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: ptrTo(corev1.ProtocolTCP),
							Port:     &dnsPort,
						},
						{
							Protocol: ptrTo(corev1.ProtocolUDP),
							Port:     &dnsPort,
						},
					},
				},
				{
					// Allow internet (0.0.0.0/0) but block private ranges
					To: []networkingv1.NetworkPolicyPeer{
						{
							IPBlock: &networkingv1.IPBlock{
								CIDR: "0.0.0.0/0",
								Except: []string{
									"10.0.0.0/8",
									"172.16.0.0/12",
									"192.168.0.0/16",
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

// operatorNs returns the operator namespace, falling back to the default.
func (r *GameServerReconciler) operatorNs() string {
	if r.OperatorNamespace != "" {
		return r.OperatorNamespace
	}
	return defaultOperatorNamespace
}

// ptrTo returns a pointer to the given value.
func ptrTo[T any](v T) *T {
	return &v
}

// transitionState transitions a GameServer to a new state with proper condition updates and events.
func (r *GameServerReconciler) transitionState(ctx context.Context, gs *gamev1alpha1.GameServer, newState gamev1alpha1.GameServerState, reason, message string) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	oldState := gs.Status.State

	// Validate the transition
	if !gamev1alpha1.IsValidTransition(oldState, newState) {
		log.Error(nil, "Invalid state transition", "from", oldState, "to", newState)
		return ctrl.Result{}, fmt.Errorf("invalid state transition from %s to %s", oldState, newState)
	}

	log.Info("Transitioning state", "from", oldState, "to", newState, "reason", reason)

	// Re-fetch to avoid conflicts
	fresh := &gamev1alpha1.GameServer{}
	if err := r.Get(ctx, types.NamespacedName{Name: gs.Name, Namespace: gs.Namespace}, fresh); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to re-fetch GameServer for status update: %w", err)
	}

	fresh.Status.State = newState

	// Update conditions based on new state
	switch newState {
	case gamev1alpha1.GameServerStateCreating, gamev1alpha1.GameServerStateStarting:
		meta.SetStatusCondition(&fresh.Status.Conditions, metav1.Condition{
			Type:               gamev1alpha1.TypeProgressing,
			Status:             metav1.ConditionTrue,
			Reason:             reason,
			Message:            message,
			ObservedGeneration: fresh.Generation,
		})
		meta.SetStatusCondition(&fresh.Status.Conditions, metav1.Condition{
			Type:               gamev1alpha1.TypeReady,
			Status:             metav1.ConditionFalse,
			Reason:             reason,
			Message:            message,
			ObservedGeneration: fresh.Generation,
		})
	case gamev1alpha1.GameServerStateReady:
		meta.SetStatusCondition(&fresh.Status.Conditions, metav1.Condition{
			Type:               gamev1alpha1.TypeReady,
			Status:             metav1.ConditionTrue,
			Reason:             reason,
			Message:            message,
			ObservedGeneration: fresh.Generation,
		})
		meta.SetStatusCondition(&fresh.Status.Conditions, metav1.Condition{
			Type:               gamev1alpha1.TypeProgressing,
			Status:             metav1.ConditionFalse,
			Reason:             reason,
			Message:            message,
			ObservedGeneration: fresh.Generation,
		})
	case gamev1alpha1.GameServerStateAllocated:
		meta.SetStatusCondition(&fresh.Status.Conditions, metav1.Condition{
			Type:               gamev1alpha1.TypeReady,
			Status:             metav1.ConditionTrue,
			Reason:             reason,
			Message:            message,
			ObservedGeneration: fresh.Generation,
		})
	case gamev1alpha1.GameServerStateShutdown:
		meta.SetStatusCondition(&fresh.Status.Conditions, metav1.Condition{
			Type:               gamev1alpha1.TypeReady,
			Status:             metav1.ConditionFalse,
			Reason:             "ShuttingDown",
			Message:            "GameServer is shutting down",
			ObservedGeneration: fresh.Generation,
		})
		meta.SetStatusCondition(&fresh.Status.Conditions, metav1.Condition{
			Type:               gamev1alpha1.TypeProgressing,
			Status:             metav1.ConditionFalse,
			Reason:             "ShuttingDown",
			Message:            "GameServer is shutting down",
			ObservedGeneration: fresh.Generation,
		})
	case gamev1alpha1.GameServerStateError:
		meta.SetStatusCondition(&fresh.Status.Conditions, metav1.Condition{
			Type:               gamev1alpha1.TypeDegraded,
			Status:             metav1.ConditionTrue,
			Reason:             reason,
			Message:            message,
			ObservedGeneration: fresh.Generation,
		})
		meta.SetStatusCondition(&fresh.Status.Conditions, metav1.Condition{
			Type:               gamev1alpha1.TypeReady,
			Status:             metav1.ConditionFalse,
			Reason:             reason,
			Message:            message,
			ObservedGeneration: fresh.Generation,
		})
		meta.SetStatusCondition(&fresh.Status.Conditions, metav1.Condition{
			Type:               gamev1alpha1.TypeProgressing,
			Status:             metav1.ConditionFalse,
			Reason:             reason,
			Message:            message,
			ObservedGeneration: fresh.Generation,
		})
	}

	if err := r.Status().Update(ctx, fresh); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update status for state transition: %w", err)
	}

	r.Recorder.Eventf(gs, corev1.EventTypeNormal, "StateChanged", "State changed from %s to %s: %s", oldState, newState, message)

	// Determine requeue behavior based on new state
	switch newState {
	case gamev1alpha1.GameServerStateCreating:
		return ctrl.Result{Requeue: true}, nil
	case gamev1alpha1.GameServerStateStarting:
		return ctrl.Result{RequeueAfter: requeueDelay}, nil
	default:
		return ctrl.Result{Requeue: true}, nil
	}
}

// updateReadyStatus updates the GameServer status with Pod IP and port information.
func (r *GameServerReconciler) updateReadyStatus(ctx context.Context, gs *gamev1alpha1.GameServer, pod *corev1.Pod) error {
	// Re-fetch to avoid conflicts
	fresh := &gamev1alpha1.GameServer{}
	if err := r.Get(ctx, types.NamespacedName{Name: gs.Name, Namespace: gs.Namespace}, fresh); err != nil {
		return fmt.Errorf("failed to re-fetch GameServer for ready status update: %w", err)
	}

	fresh.Status.Address = pod.Status.PodIP

	// Map ports from spec to status
	var statusPorts []gamev1alpha1.GameServerStatusPort
	for _, p := range fresh.Spec.Ports {
		statusPorts = append(statusPorts, gamev1alpha1.GameServerStatusPort{
			Name:     p.Name,
			Port:     p.ContainerPort,
			Protocol: p.Protocol,
		})
	}
	fresh.Status.Ports = statusPorts

	// Set Ready condition
	meta.SetStatusCondition(&fresh.Status.Conditions, metav1.Condition{
		Type:               gamev1alpha1.TypeReady,
		Status:             metav1.ConditionTrue,
		Reason:             "PodReady",
		Message:            "GameServer is running and ready",
		ObservedGeneration: fresh.Generation,
	})
	meta.SetStatusCondition(&fresh.Status.Conditions, metav1.Condition{
		Type:               gamev1alpha1.TypeProgressing,
		Status:             metav1.ConditionFalse,
		Reason:             "PodReady",
		Message:            "GameServer is running and ready",
		ObservedGeneration: fresh.Generation,
	})

	if err := r.Status().Update(ctx, fresh); err != nil {
		return fmt.Errorf("failed to update ready status: %w", err)
	}

	return nil
}

// refreshAndUpdateCondition re-fetches the GameServer and updates a single condition.
func (r *GameServerReconciler) refreshAndUpdateCondition(ctx context.Context, gs *gamev1alpha1.GameServer, condType string, status metav1.ConditionStatus, reason, message string) error {
	fresh := &gamev1alpha1.GameServer{}
	if err := r.Get(ctx, types.NamespacedName{Name: gs.Name, Namespace: gs.Namespace}, fresh); err != nil {
		return fmt.Errorf("failed to re-fetch GameServer for condition update: %w", err)
	}

	meta.SetStatusCondition(&fresh.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: fresh.Generation,
	})

	if err := r.Status().Update(ctx, fresh); err != nil {
		return fmt.Errorf("failed to update condition %s: %w", condType, err)
	}

	return nil
}

// isPodReady returns true if all containers in the Pod are ready.
func isPodReady(pod *corev1.Pod) bool {
	for _, cs := range pod.Status.ContainerStatuses {
		if !cs.Ready {
			return false
		}
	}
	// Also need at least one container status to exist
	return len(pod.Status.ContainerStatuses) > 0
}

// SetupWithManager sets up the controller with the Manager.
func (r *GameServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gamev1alpha1.GameServer{}).
		Owns(&corev1.Pod{}).
		WithEventFilter(predicate.Or(
			predicate.GenerationChangedPredicate{},
			predicate.AnnotationChangedPredicate{},
		)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 3,
		}).
		Named("gameserver").
		Complete(r)
}
