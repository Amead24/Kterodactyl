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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	gamev1alpha1 "github.com/kterodactyl/kterodactyl/api/v1alpha1"
	"github.com/kterodactyl/kterodactyl/internal/util"
)

// DNSReconciler reconciles GameServer resources and creates per-server
// Service + HTTPRoute resources for DNS routing via ExternalDNS.
type DNSReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	Recorder          record.EventRecorder
	OperatorNamespace string
}

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes/status,verbs=get
// +kubebuilder:rbac:groups=game.kterodactyl.io,resources=gameservers,verbs=get;list;watch
// +kubebuilder:rbac:groups=game.kterodactyl.io,resources=gameservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile handles DNS-related resource management for GameServer resources.
// It creates ClusterIP Services and HTTPRoutes for Ready/Allocated GameServers,
// and cleans them up when servers leave those states.
func (r *DNSReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Fetch the GameServer CR
	gs := &gamev1alpha1.GameServer{}
	if err := r.Get(ctx, req.NamespacedName, gs); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 2. Load AdminConfig for BaseDomain and Gateway settings
	adminCfg, err := LoadAdminConfig(ctx, r.Client, r.operatorNs())
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to load admin config: %w", err)
	}
	if adminCfg.BaseDomain == "" {
		// DNS not configured; skip networking
		log.V(1).Info("BaseDomain not configured, skipping DNS reconciliation")
		return ctrl.Result{}, nil
	}

	// 3. If GameServer is being deleted, owner references handle cleanup automatically
	if !gs.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// 4. If GameServer is NOT Ready and NOT Allocated, clean up networking resources
	if gs.Status.State != gamev1alpha1.GameServerStateReady && gs.Status.State != gamev1alpha1.GameServerStateAllocated {
		if err := r.cleanupNetworking(ctx, gs); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to cleanup networking: %w", err)
		}
		return ctrl.Result{}, nil
	}

	// 5. Extract owner label
	owner := gs.Labels[util.LabelOwner]
	if owner == "" {
		log.Error(nil, "GameServer missing owner label, cannot create networking resources", "name", gs.Name)
		return ctrl.Result{}, nil
	}

	// 6. Build DNS name
	dnsName := util.GameServerDNSName(gs.Spec.GameType, owner, adminCfg.BaseDomain)

	// 7. Ensure Service exists
	if err := r.ensureService(ctx, gs); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure Service: %w", err)
	}

	// 8. Ensure HTTPRoute exists
	if err := r.ensureHTTPRoute(ctx, gs, dnsName, adminCfg); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure HTTPRoute: %w", err)
	}

	// 9. Update GameServer connection info in status
	if err := r.updateConnectionInfo(ctx, gs, dnsName); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update connection info: %w", err)
	}

	return ctrl.Result{}, nil
}

// ensureService creates or updates a ClusterIP Service for the GameServer.
func (r *DNSReconciler) ensureService(ctx context.Context, gs *gamev1alpha1.GameServer) error {
	log := logf.FromContext(ctx)
	owner := gs.Labels[util.LabelOwner]

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gs.Name,
			Namespace: gs.Namespace,
		},
	}

	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		// Set owner reference for automatic cleanup
		if err := ctrl.SetControllerReference(gs, svc, r.Scheme); err != nil {
			return fmt.Errorf("failed to set owner reference on Service: %w", err)
		}

		// Set labels
		if svc.Labels == nil {
			svc.Labels = make(map[string]string)
		}
		svc.Labels[util.LabelManagedBy] = util.ManagedByValue
		svc.Labels[util.LabelHTTPRouteOwner] = gs.Name

		// Set selector to match GameServer pods
		svc.Spec.Selector = map[string]string{
			util.LabelOwner: owner,
			util.LabelGame:  gs.Spec.GameType,
			util.LabelName:  util.AppNameValue,
		}

		// Set service type
		svc.Spec.Type = corev1.ServiceTypeClusterIP

		// Map ports from GameServer spec
		var servicePorts []corev1.ServicePort
		for _, p := range gs.Spec.Ports {
			servicePorts = append(servicePorts, corev1.ServicePort{
				Name:       p.Name,
				Port:       p.ContainerPort,
				TargetPort: intstr.FromInt32(p.ContainerPort),
				Protocol:   p.Protocol,
			})
		}
		svc.Spec.Ports = servicePorts

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create/update Service: %w", err)
	}

	log.Info("Service reconciled", "name", svc.Name, "result", result)
	return nil
}

// ensureHTTPRoute creates or updates an HTTPRoute for the GameServer.
func (r *DNSReconciler) ensureHTTPRoute(ctx context.Context, gs *gamev1alpha1.GameServer, hostname string, adminCfg *AdminConfig) error {
	log := logf.FromContext(ctx)

	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gs.Name,
			Namespace: gs.Namespace,
		},
	}

	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, route, func() error {
		// Set owner reference for automatic cleanup
		if err := ctrl.SetControllerReference(gs, route, r.Scheme); err != nil {
			return fmt.Errorf("failed to set owner reference on HTTPRoute: %w", err)
		}

		// Set annotations for ExternalDNS
		if route.Annotations == nil {
			route.Annotations = make(map[string]string)
		}
		route.Annotations[util.AnnotationExternalDNSTTL] = "60"

		// Set ParentRefs pointing to the Gateway
		gwNamespace := gatewayv1.Namespace(adminCfg.GatewayNamespace)
		sectionName := gatewayv1.SectionName("http")
		route.Spec.ParentRefs = []gatewayv1.ParentReference{
			{
				Name:        gatewayv1.ObjectName(adminCfg.GatewayName),
				Namespace:   &gwNamespace,
				SectionName: &sectionName,
			},
		}

		// Set hostnames
		route.Spec.Hostnames = []gatewayv1.Hostname{gatewayv1.Hostname(hostname)}

		// Determine the backend port
		var backendPort gatewayv1.PortNumber = 8080
		if len(gs.Spec.Ports) > 0 {
			backendPort = gatewayv1.PortNumber(gs.Spec.Ports[0].ContainerPort)
		}

		// Set rules with single backend ref pointing to the ClusterIP Service
		route.Spec.Rules = []gatewayv1.HTTPRouteRule{
			{
				BackendRefs: []gatewayv1.HTTPBackendRef{
					{
						BackendRef: gatewayv1.BackendRef{
							BackendObjectReference: gatewayv1.BackendObjectReference{
								Name: gatewayv1.ObjectName(gs.Name),
								Port: ptrTo(backendPort),
							},
						},
					},
				},
			},
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create/update HTTPRoute: %w", err)
	}

	log.Info("HTTPRoute reconciled", "name", route.Name, "hostname", hostname, "result", result)
	return nil
}

// updateConnectionInfo updates the GameServer status with DNS name and port information.
func (r *DNSReconciler) updateConnectionInfo(ctx context.Context, gs *gamev1alpha1.GameServer, dnsName string) error {
	// Re-fetch to avoid conflicts (established pattern)
	fresh := &gamev1alpha1.GameServer{}
	if err := r.Get(ctx, types.NamespacedName{Name: gs.Name, Namespace: gs.Namespace}, fresh); err != nil {
		return fmt.Errorf("failed to re-fetch GameServer for connection info update: %w", err)
	}

	// Only update if the address has changed
	if fresh.Status.Address == dnsName {
		return nil
	}

	fresh.Status.Address = dnsName

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

	if err := r.Status().Update(ctx, fresh); err != nil {
		return fmt.Errorf("failed to update connection info status: %w", err)
	}

	r.Recorder.Eventf(gs, corev1.EventTypeNormal, "DNSConfigured", "DNS name configured: %s", dnsName)
	return nil
}

// cleanupNetworking removes Service and HTTPRoute if they exist when a GameServer
// leaves Ready/Allocated state. Also clears status.Address and status.Ports.
func (r *DNSReconciler) cleanupNetworking(ctx context.Context, gs *gamev1alpha1.GameServer) error {
	log := logf.FromContext(ctx)

	// Delete Service if it exists
	svc := &corev1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Name: gs.Name, Namespace: gs.Namespace}, svc); err == nil {
		if err := r.Delete(ctx, svc); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to delete Service during cleanup: %w", err)
		}
		log.Info("Deleted Service during cleanup", "name", gs.Name)
	}

	// Delete HTTPRoute if it exists
	route := &gatewayv1.HTTPRoute{}
	if err := r.Get(ctx, types.NamespacedName{Name: gs.Name, Namespace: gs.Namespace}, route); err == nil {
		if err := r.Delete(ctx, route); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to delete HTTPRoute during cleanup: %w", err)
		}
		log.Info("Deleted HTTPRoute during cleanup", "name", gs.Name)
	}

	// Clear status.Address and status.Ports if they are set
	if gs.Status.Address != "" || len(gs.Status.Ports) > 0 {
		fresh := &gamev1alpha1.GameServer{}
		if err := r.Get(ctx, types.NamespacedName{Name: gs.Name, Namespace: gs.Namespace}, fresh); err != nil {
			return client.IgnoreNotFound(err)
		}
		if fresh.Status.Address != "" || len(fresh.Status.Ports) > 0 {
			fresh.Status.Address = ""
			fresh.Status.Ports = nil
			if err := r.Status().Update(ctx, fresh); err != nil {
				return fmt.Errorf("failed to clear connection info during cleanup: %w", err)
			}
			log.Info("Cleared connection info during cleanup", "name", gs.Name)
		}
	}

	return nil
}

// operatorNs returns the operator namespace, falling back to the default.
func (r *DNSReconciler) operatorNs() string {
	if r.OperatorNamespace != "" {
		return r.OperatorNamespace
	}
	return "kterodactyl-system"
}

// SetupWithManager sets up the DNS controller with the Manager.
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
