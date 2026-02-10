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
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	gamev1alpha1 "github.com/kterodactyl/kterodactyl/api/v1alpha1"
	"github.com/kterodactyl/kterodactyl/internal/util"
)

var _ = Describe("GameServer Controller", func() {

	const (
		timeout  = 10 * time.Second
		interval = 250 * time.Millisecond
	)

	// createTestNamespace creates a namespace for test isolation and returns a cleanup function.
	createTestNamespace := func(name string) {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: name}, ns)
		if errors.IsNotFound(err) {
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())
		}
	}

	// newGameServer creates a GameServer CR with the given parameters.
	newGameServer := func(name, namespace, gameType, image string, labels map[string]string) *gamev1alpha1.GameServer {
		return &gamev1alpha1.GameServer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    labels,
			},
			Spec: gamev1alpha1.GameServerSpec{
				GameType: gameType,
				Image:    image,
			},
		}
	}

	// deleteGameServer deletes a GameServer CR, ignoring NotFound errors.
	deleteGameServer := func(name, namespace string) {
		gs := &gamev1alpha1.GameServer{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, gs)
		if err == nil {
			_ = k8sClient.Delete(ctx, gs)
			// Wait for the GameServer to be fully gone
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, gs)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		}
	}

	Context("Test Case 1: Pod creation from GameServer CR", func() {
		const (
			testNs   = "test-ns-1"
			gsName   = "mc-pod-test"
			owner    = "testuser1"
			gameType = "minecraft"
			image    = "itzg/minecraft-server:latest"
		)

		BeforeEach(func() {
			createTestNamespace(testNs)
		})

		AfterEach(func() {
			deleteGameServer(gsName, testNs)
		})

		It("should create a Pod when GameServer is created", func() {
			By("creating a GameServer CR")
			gs := newGameServer(gsName, testNs, gameType, image, map[string]string{
				util.LabelOwner: owner,
			})
			Expect(k8sClient.Create(ctx, gs)).To(Succeed())

			By("verifying a Pod is created with correct properties")
			pod := &corev1.Pod{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: gsName, Namespace: testNs}, pod)
			}, timeout, interval).Should(Succeed())

			// Verify container name and image
			Expect(pod.Spec.Containers).To(HaveLen(1))
			Expect(pod.Spec.Containers[0].Name).To(Equal("gameserver"))
			Expect(pod.Spec.Containers[0].Image).To(Equal(image))

			// Verify owner reference
			Expect(pod.OwnerReferences).To(HaveLen(1))
			Expect(pod.OwnerReferences[0].Name).To(Equal(gsName))
			Expect(pod.OwnerReferences[0].Kind).To(Equal("GameServer"))

			// Verify labels match GameServerLabels
			expectedLabels := util.GameServerLabels(owner, gameType)
			for k, v := range expectedLabels {
				Expect(pod.Labels).To(HaveKeyWithValue(k, v))
			}

			// Verify RestartPolicy
			Expect(pod.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyNever))
		})
	})

	Context("Test Case 2: State transitions from Creating to Starting", func() {
		const (
			testNs   = "test-ns-2"
			gsName   = "mc-state-test"
			owner    = "testuser2"
			gameType = "minecraft"
			image    = "itzg/minecraft-server:latest"
		)

		BeforeEach(func() {
			createTestNamespace(testNs)
		})

		AfterEach(func() {
			deleteGameServer(gsName, testNs)
		})

		It("should transition through Creating to Starting state", func() {
			By("creating a GameServer CR")
			gs := newGameServer(gsName, testNs, gameType, image, map[string]string{
				util.LabelOwner: owner,
			})
			Expect(k8sClient.Create(ctx, gs)).To(Succeed())

			By("verifying state transitions to Creating then Starting")
			// Note: the initial state is "" which transitions to Creating, then the
			// reconcileCreating handler creates the Pod and transitions to Starting.
			// In envtest, Pods never actually become Running (no kubelet/scheduler),
			// so we cannot test Starting -> Ready. This is documented and expected.
			gsLookup := &gamev1alpha1.GameServer{}
			Eventually(func() gamev1alpha1.GameServerState {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: gsName, Namespace: testNs}, gsLookup)
				if err != nil {
					return ""
				}
				return gsLookup.Status.State
			}, timeout, interval).Should(Equal(gamev1alpha1.GameServerStateStarting))
		})
	})

	Context("Test Case 3: Deletion with finalizer cleanup", func() {
		const (
			testNs   = "test-ns-3"
			gsName   = "mc-delete-test"
			owner    = "testuser3"
			gameType = "minecraft"
			image    = "itzg/minecraft-server:latest"
		)

		BeforeEach(func() {
			createTestNamespace(testNs)
		})

		It("should handle deletion with finalizer cleanup", func() {
			By("creating a GameServer CR")
			gs := newGameServer(gsName, testNs, gameType, image, map[string]string{
				util.LabelOwner: owner,
			})
			Expect(k8sClient.Create(ctx, gs)).To(Succeed())

			By("waiting for finalizer to be added")
			gsLookup := &gamev1alpha1.GameServer{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: gsName, Namespace: testNs}, gsLookup)
				if err != nil {
					return false
				}
				for _, f := range gsLookup.Finalizers {
					if f == gamev1alpha1.FinalizerName {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("waiting for Pod to be created")
			pod := &corev1.Pod{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: gsName, Namespace: testNs}, pod)
			}, timeout, interval).Should(Succeed())

			By("deleting the GameServer CR")
			Expect(k8sClient.Delete(ctx, gsLookup)).To(Succeed())

			By("verifying GameServer CR is gone")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: gsName, Namespace: testNs}, &gamev1alpha1.GameServer{})
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			By("verifying Pod is gone")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: gsName, Namespace: testNs}, &corev1.Pod{})
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("Test Case 4: Error state when owner label is missing", func() {
		const (
			testNs   = "test-ns-4"
			gsName   = "mc-no-owner-test"
			gameType = "minecraft"
			image    = "itzg/minecraft-server:latest"
		)

		BeforeEach(func() {
			createTestNamespace(testNs)
		})

		AfterEach(func() {
			deleteGameServer(gsName, testNs)
		})

		It("should set Error state when owner label is missing", func() {
			By("creating a GameServer CR WITHOUT the owner label")
			gs := newGameServer(gsName, testNs, gameType, image, nil)
			Expect(k8sClient.Create(ctx, gs)).To(Succeed())

			By("verifying state becomes Error")
			gsLookup := &gamev1alpha1.GameServer{}
			Eventually(func() gamev1alpha1.GameServerState {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: gsName, Namespace: testNs}, gsLookup)
				if err != nil {
					return ""
				}
				return gsLookup.Status.State
			}, timeout, interval).Should(Equal(gamev1alpha1.GameServerStateError))
		})
	})

	Context("Test Case 5: User namespace with ResourceQuota", func() {
		const (
			testNs   = "test-ns-5"
			gsName   = "mc-quota-test"
			owner    = "quotauser"
			gameType = "minecraft"
			image    = "itzg/minecraft-server:latest"
		)

		var userNsName string

		BeforeEach(func() {
			createTestNamespace(testNs)
			userNsName = fmt.Sprintf("user-%s", owner)
		})

		AfterEach(func() {
			deleteGameServer(gsName, testNs)
		})

		It("should create user namespace with ResourceQuota", func() {
			By("creating a GameServer CR with owner label")
			gs := newGameServer(gsName, testNs, gameType, image, map[string]string{
				util.LabelOwner: owner,
			})
			Expect(k8sClient.Create(ctx, gs)).To(Succeed())

			By("verifying user namespace exists with correct labels")
			ns := &corev1.Namespace{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: userNsName}, ns)
			}, timeout, interval).Should(Succeed())
			Expect(ns.Labels).To(HaveKeyWithValue(util.LabelManagedByKterodactyl, util.ManagedByValue))

			By("verifying ResourceQuota exists in user namespace")
			quota := &corev1.ResourceQuota{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "user-quota",
					Namespace: userNsName,
				}, quota)
			}, timeout, interval).Should(Succeed())

			// Verify hard limits include pods
			Expect(quota.Spec.Hard).To(HaveKey(corev1.ResourcePods))
		})
	})

	Context("Test Case 6: NetworkPolicy in user namespace", func() {
		const (
			testNs   = "test-ns-6"
			gsName   = "mc-netpol-test"
			owner    = "netpoluser"
			gameType = "minecraft"
			image    = "itzg/minecraft-server:latest"
		)

		var userNsName string

		BeforeEach(func() {
			createTestNamespace(testNs)
			userNsName = fmt.Sprintf("user-%s", owner)
		})

		AfterEach(func() {
			deleteGameServer(gsName, testNs)
		})

		It("should create NetworkPolicy in user namespace", func() {
			By("creating a GameServer CR with owner label")
			gs := newGameServer(gsName, testNs, gameType, image, map[string]string{
				util.LabelOwner: owner,
			})
			Expect(k8sClient.Create(ctx, gs)).To(Succeed())

			By("verifying NetworkPolicy exists in user namespace")
			np := &networkingv1.NetworkPolicy{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "deny-cross-namespace",
					Namespace: userNsName,
				}, np)
			}, timeout, interval).Should(Succeed())

			// Verify policy types include both Ingress and Egress
			Expect(np.Spec.PolicyTypes).To(ContainElements(
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			))
		})
	})

	Context("Test Case 7: Admin ConfigMap values for quota", func() {
		const (
			testNs   = "test-ns-7"
			gsName   = "mc-config-test"
			owner    = "configuser"
			gameType = "minecraft"
			image    = "itzg/minecraft-server:latest"
		)

		var userNsName string

		BeforeEach(func() {
			createTestNamespace(testNs)
			userNsName = fmt.Sprintf("user-%s", owner)
		})

		AfterEach(func() {
			// Clean up ConfigMap
			cm := &corev1.ConfigMap{}
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "kterodactyl-admin-config",
				Namespace: testOperatorNamespace,
			}, cm)
			if err == nil {
				_ = k8sClient.Delete(ctx, cm)
			}
			deleteGameServer(gsName, testNs)
		})

		It("should use admin ConfigMap values for quota when available", func() {
			By("creating the admin ConfigMap with custom values")
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kterodactyl-admin-config",
					Namespace: testOperatorNamespace,
				},
				Data: map[string]string{
					"maxServersPerUser": "2",
					"quotaCPURequests":  "2",
				},
			}
			Expect(k8sClient.Create(ctx, cm)).To(Succeed())

			By("creating a GameServer CR with owner label")
			gs := newGameServer(gsName, testNs, gameType, image, map[string]string{
				util.LabelOwner: owner,
			})
			Expect(k8sClient.Create(ctx, gs)).To(Succeed())

			By("verifying ResourceQuota uses the custom CPU request value")
			quota := &corev1.ResourceQuota{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "user-quota",
					Namespace: userNsName,
				}, quota)
				if err != nil {
					return false
				}
				cpuReq, ok := quota.Spec.Hard[corev1.ResourceRequestsCPU]
				if !ok {
					return false
				}
				// Custom value "2" should be reflected
				return cpuReq.String() == "2"
			}, timeout, interval).Should(BeTrue())
		})
	})

})
