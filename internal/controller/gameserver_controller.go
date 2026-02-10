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
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
)

// GameServerReconciler reconciles a GameServer object.
type GameServerReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=game.kterodactyl.io,resources=gameservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=game.kterodactyl.io,resources=gameservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=game.kterodactyl.io,resources=gameservers/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=resourcequotas,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch
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

// reconcileCreating handles the Creating state: validates labels, creates/updates Pod, transitions to Starting.
func (r *GameServerReconciler) reconcileCreating(ctx context.Context, gs *gamev1alpha1.GameServer) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Reconciling Creating state", "name", gs.Name)

	// Extract owner from label
	owner := gs.Labels[util.LabelOwner]
	if owner == "" {
		log.Error(nil, "Missing owner label", "label", util.LabelOwner)
		return r.transitionState(ctx, gs, gamev1alpha1.GameServerStateError, "MissingOwnerLabel", "GameServer is missing required owner label")
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
