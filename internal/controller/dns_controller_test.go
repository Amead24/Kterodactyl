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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	gamev1alpha1 "github.com/kterodactyl/kterodactyl/api/v1alpha1"
	"github.com/kterodactyl/kterodactyl/internal/util"
)

var _ = Describe("DNS Controller", func() {

	const (
		timeout  = 10 * time.Second
		interval = 250 * time.Millisecond

		testBaseDomain  = "test.example.com"
		testGatewayName = "test-gateway"
		testGatewayNs   = "test-gateway-ns"
	)

	// testCounter generates unique namespace names across tests.
	var testCounter int

	// createDNSTestNamespace creates a unique namespace for test isolation.
	createDNSTestNamespace := func(prefix string) string {
		testCounter++
		name := fmt.Sprintf("%s-%d", prefix, testCounter)
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					util.LabelManagedByKterodactyl: util.ManagedByValue,
				},
			},
		}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: name}, ns)
		if errors.IsNotFound(err) {
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())
		}
		return name
	}

	// createAdminConfigMap creates the admin ConfigMap with DNS settings in the operator namespace.
	createAdminConfigMap := func(baseDomain string) {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kterodactyl-admin-config",
				Namespace: testOperatorNamespace,
			},
			Data: map[string]string{
				"baseDomain":       baseDomain,
				"gatewayName":      testGatewayName,
				"gatewayNamespace": testGatewayNs,
			},
		}
		// Delete existing ConfigMap if present
		existing := &corev1.ConfigMap{}
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      "kterodactyl-admin-config",
			Namespace: testOperatorNamespace,
		}, existing)
		if err == nil {
			Expect(k8sClient.Delete(ctx, existing)).To(Succeed())
			// Wait for deletion
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "kterodactyl-admin-config",
					Namespace: testOperatorNamespace,
				}, &corev1.ConfigMap{})
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		}
		Expect(k8sClient.Create(ctx, cm)).To(Succeed())
	}

	// deleteAdminConfigMap removes the admin ConfigMap from the operator namespace.
	deleteAdminConfigMap := func() {
		cm := &corev1.ConfigMap{}
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      "kterodactyl-admin-config",
			Namespace: testOperatorNamespace,
		}, cm)
		if err == nil {
			_ = k8sClient.Delete(ctx, cm)
		}
	}

	// newDNSTestGameServer creates a GameServer with owner label and game ports.
	newDNSTestGameServer := func(name, namespace, owner, gameType string) *gamev1alpha1.GameServer {
		return &gamev1alpha1.GameServer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels: map[string]string{
					util.LabelOwner: owner,
					util.LabelGame:  gameType,
				},
			},
			Spec: gamev1alpha1.GameServerSpec{
				GameType: gameType,
				Image:    "itzg/minecraft-server:latest",
				Ports: []gamev1alpha1.GameServerPort{
					{
						Name:          "game",
						ContainerPort: 25565,
						Protocol:      corev1.ProtocolTCP,
					},
				},
			},
		}
	}

	// patchGameServerState manually patches a GameServer status to the given state.
	// envtest has no kubelet, so GameServers will not naturally reach Ready state.
	patchGameServerState := func(name, namespace string, state gamev1alpha1.GameServerState) {
		Eventually(func() error {
			gs := &gamev1alpha1.GameServer{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, gs); err != nil {
				return err
			}
			gs.Status.State = state
			return k8sClient.Status().Update(ctx, gs)
		}, timeout, interval).Should(Succeed())
	}

	// deleteGameServer deletes a GameServer, ignoring NotFound errors.
	deleteGameServer := func(name, namespace string) {
		gs := &gamev1alpha1.GameServer{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, gs)
		if err == nil {
			_ = k8sClient.Delete(ctx, gs)
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, gs)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		}
	}

	Context("Test Case 1: Service creation when GameServer reaches Ready state", func() {
		var (
			testNs   string
			gsName   = "dns-svc-test"
			owner    = "dnsuser1"
			gameType = "minecraft"
		)

		BeforeEach(func() {
			testNs = createDNSTestNamespace("dns-test-ns")
			createAdminConfigMap(testBaseDomain)
		})

		AfterEach(func() {
			deleteGameServer(gsName, testNs)
			deleteAdminConfigMap()
		})

		It("should create Service when GameServer reaches Ready state", func() {
			By("creating a GameServer CR")
			gs := newDNSTestGameServer(gsName, testNs, owner, gameType)
			Expect(k8sClient.Create(ctx, gs)).To(Succeed())

			By("waiting for GameServer to reach Starting state")
			Eventually(func() gamev1alpha1.GameServerState {
				gsLookup := &gamev1alpha1.GameServer{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: gsName, Namespace: testNs}, gsLookup)
				if err != nil {
					return ""
				}
				return gsLookup.Status.State
			}, timeout, interval).Should(Equal(gamev1alpha1.GameServerStateStarting))

			By("manually patching GameServer status to Ready")
			patchGameServerState(gsName, testNs, gamev1alpha1.GameServerStateReady)

			By("verifying a Service is created with correct properties")
			svc := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: gsName, Namespace: testNs}, svc)
			}, timeout, interval).Should(Succeed())

			// Verify Service selector labels
			Expect(svc.Spec.Selector).To(HaveKeyWithValue(util.LabelOwner, owner))
			Expect(svc.Spec.Selector).To(HaveKeyWithValue(util.LabelGame, gameType))
			Expect(svc.Spec.Selector).To(HaveKeyWithValue(util.LabelName, util.AppNameValue))

			// Verify Service ports match GameServer spec
			Expect(svc.Spec.Ports).To(HaveLen(1))
			Expect(svc.Spec.Ports[0].Name).To(Equal("game"))
			Expect(svc.Spec.Ports[0].Port).To(Equal(int32(25565)))
			Expect(svc.Spec.Ports[0].Protocol).To(Equal(corev1.ProtocolTCP))

			// Verify Service type
			Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
		})
	})

	Context("Test Case 2: HTTPRoute creation when GameServer reaches Ready state", func() {
		var (
			testNs   string
			gsName   = "dns-route-test"
			owner    = "dnsuser2"
			gameType = "minecraft"
		)

		BeforeEach(func() {
			testNs = createDNSTestNamespace("dns-test-ns")
			createAdminConfigMap(testBaseDomain)
		})

		AfterEach(func() {
			deleteGameServer(gsName, testNs)
			deleteAdminConfigMap()
		})

		It("should create HTTPRoute when GameServer reaches Ready state", func() {
			By("creating a GameServer CR")
			gs := newDNSTestGameServer(gsName, testNs, owner, gameType)
			Expect(k8sClient.Create(ctx, gs)).To(Succeed())

			By("waiting for GameServer to reach Starting state")
			Eventually(func() gamev1alpha1.GameServerState {
				gsLookup := &gamev1alpha1.GameServer{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: gsName, Namespace: testNs}, gsLookup)
				if err != nil {
					return ""
				}
				return gsLookup.Status.State
			}, timeout, interval).Should(Equal(gamev1alpha1.GameServerStateStarting))

			By("manually patching GameServer status to Ready")
			patchGameServerState(gsName, testNs, gamev1alpha1.GameServerStateReady)

			By("verifying an HTTPRoute is created with correct properties")
			route := &gatewayv1.HTTPRoute{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: gsName, Namespace: testNs}, route)
			}, timeout, interval).Should(Succeed())

			// Verify hostname matches expected pattern: gameType.owner.baseDomain
			expectedHostname := fmt.Sprintf("%s.%s.%s", gameType, owner, testBaseDomain)
			Expect(route.Spec.Hostnames).To(HaveLen(1))
			Expect(string(route.Spec.Hostnames[0])).To(Equal(expectedHostname))

			// Verify parentRef points to configured gateway
			Expect(route.Spec.ParentRefs).To(HaveLen(1))
			Expect(string(route.Spec.ParentRefs[0].Name)).To(Equal(testGatewayName))
			Expect(route.Spec.ParentRefs[0].Namespace).NotTo(BeNil())
			Expect(string(*route.Spec.ParentRefs[0].Namespace)).To(Equal(testGatewayNs))

			// Verify backendRef points to the Service name
			Expect(route.Spec.Rules).To(HaveLen(1))
			Expect(route.Spec.Rules[0].BackendRefs).To(HaveLen(1))
			Expect(string(route.Spec.Rules[0].BackendRefs[0].Name)).To(Equal(gsName))
		})
	})

	Context("Test Case 3: GameServer status.address updated with DNS name", func() {
		var (
			testNs   string
			gsName   = "dns-addr-test"
			owner    = "dnsuser3"
			gameType = "minecraft"
		)

		BeforeEach(func() {
			testNs = createDNSTestNamespace("dns-test-ns")
			createAdminConfigMap(testBaseDomain)
		})

		AfterEach(func() {
			deleteGameServer(gsName, testNs)
			deleteAdminConfigMap()
		})

		It("should update GameServer status.address with DNS name", func() {
			By("creating a GameServer CR")
			gs := newDNSTestGameServer(gsName, testNs, owner, gameType)
			Expect(k8sClient.Create(ctx, gs)).To(Succeed())

			By("waiting for GameServer to reach Starting state")
			Eventually(func() gamev1alpha1.GameServerState {
				gsLookup := &gamev1alpha1.GameServer{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: gsName, Namespace: testNs}, gsLookup)
				if err != nil {
					return ""
				}
				return gsLookup.Status.State
			}, timeout, interval).Should(Equal(gamev1alpha1.GameServerStateStarting))

			By("manually patching GameServer status to Ready")
			patchGameServerState(gsName, testNs, gamev1alpha1.GameServerStateReady)

			By("verifying GameServer status.address contains the DNS name")
			expectedAddress := fmt.Sprintf("%s.%s.%s", gameType, owner, testBaseDomain)
			Eventually(func() string {
				gsLookup := &gamev1alpha1.GameServer{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: gsName, Namespace: testNs}, gsLookup)
				if err != nil {
					return ""
				}
				return gsLookup.Status.Address
			}, timeout, interval).Should(Equal(expectedAddress))
		})
	})

	Context("Test Case 4: Cleanup when GameServer transitions to Shutdown", func() {
		var (
			testNs   string
			gsName   = "dns-cleanup-test"
			owner    = "dnsuser4"
			gameType = "minecraft"
		)

		BeforeEach(func() {
			testNs = createDNSTestNamespace("dns-test-ns")
			createAdminConfigMap(testBaseDomain)
		})

		AfterEach(func() {
			deleteGameServer(gsName, testNs)
			deleteAdminConfigMap()
		})

		It("should clean up Service and HTTPRoute when GameServer transitions to Shutdown", func() {
			By("creating a GameServer CR")
			gs := newDNSTestGameServer(gsName, testNs, owner, gameType)
			Expect(k8sClient.Create(ctx, gs)).To(Succeed())

			By("waiting for GameServer to reach Starting state")
			Eventually(func() gamev1alpha1.GameServerState {
				gsLookup := &gamev1alpha1.GameServer{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: gsName, Namespace: testNs}, gsLookup)
				if err != nil {
					return ""
				}
				return gsLookup.Status.State
			}, timeout, interval).Should(Equal(gamev1alpha1.GameServerStateStarting))

			By("manually patching GameServer status to Ready")
			patchGameServerState(gsName, testNs, gamev1alpha1.GameServerStateReady)

			By("waiting for Service to appear")
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: gsName, Namespace: testNs}, &corev1.Service{})
			}, timeout, interval).Should(Succeed())

			By("waiting for HTTPRoute to appear")
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: gsName, Namespace: testNs}, &gatewayv1.HTTPRoute{})
			}, timeout, interval).Should(Succeed())

			By("manually patching GameServer status to Shutdown")
			patchGameServerState(gsName, testNs, gamev1alpha1.GameServerStateShutdown)

			By("verifying Service is deleted")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: gsName, Namespace: testNs}, &corev1.Service{})
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			By("verifying HTTPRoute is deleted")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: gsName, Namespace: testNs}, &gatewayv1.HTTPRoute{})
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("Test Case 5: No networking resources when baseDomain is empty", func() {
		var (
			testNs   string
			gsName   = "dns-nodomain-test"
			owner    = "dnsuser5"
			gameType = "minecraft"
		)

		BeforeEach(func() {
			testNs = createDNSTestNamespace("dns-test-ns")
			// Create admin ConfigMap with empty baseDomain
			createAdminConfigMap("")
		})

		AfterEach(func() {
			deleteGameServer(gsName, testNs)
			deleteAdminConfigMap()
		})

		It("should not create networking resources when baseDomain is empty", func() {
			By("creating a GameServer CR")
			gs := newDNSTestGameServer(gsName, testNs, owner, gameType)
			Expect(k8sClient.Create(ctx, gs)).To(Succeed())

			By("waiting for GameServer to reach Starting state")
			Eventually(func() gamev1alpha1.GameServerState {
				gsLookup := &gamev1alpha1.GameServer{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: gsName, Namespace: testNs}, gsLookup)
				if err != nil {
					return ""
				}
				return gsLookup.Status.State
			}, timeout, interval).Should(Equal(gamev1alpha1.GameServerStateStarting))

			By("manually patching GameServer status to Ready")
			patchGameServerState(gsName, testNs, gamev1alpha1.GameServerStateReady)

			By("verifying no Service is created (consistently for 2 seconds)")
			Consistently(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: gsName, Namespace: testNs}, &corev1.Service{})
				return errors.IsNotFound(err)
			}, 2*time.Second, interval).Should(BeTrue())
		})
	})
})
