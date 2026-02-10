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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GameServerState describes the lifecycle state of a GameServer.
// +kubebuilder:validation:Enum=Creating;Starting;Ready;Allocated;Shutdown;Error
type GameServerState string

// GameServerSpec defines the desired state of GameServer.
type GameServerSpec struct {
	// GameType references the game definition (e.g., "minecraft", "valheim").
	// Must be a valid DNS label: lowercase alphanumeric with optional hyphens.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`
	GameType string `json:"gameType"`

	// Image is the container image to run for the game server.
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// Resources defines CPU/memory requests and limits for the game server container.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Ports defines the ports exposed by the game server.
	// +optional
	Ports []GameServerPort `json:"ports,omitempty"`

	// Parameters holds game-specific configuration key-value pairs.
	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`
}

// GameServerPort defines a port exposed by the game server.
type GameServerPort struct {
	// Name is a descriptive identifier for the port.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// ContainerPort is the port number on the container.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	ContainerPort int32 `json:"containerPort"`

	// Protocol is the network protocol (TCP or UDP).
	// +kubebuilder:validation:Enum=TCP;UDP
	// +kubebuilder:default=TCP
	Protocol corev1.Protocol `json:"protocol,omitempty"`
}

// GameServerStatus defines the observed state of GameServer.
type GameServerStatus struct {
	// State is the current lifecycle state of the game server.
	// +kubebuilder:validation:Enum=Creating;Starting;Ready;Allocated;Shutdown;Error
	// +optional
	State GameServerState `json:"state,omitempty"`

	// Address is the connection address for the game server.
	// +optional
	Address string `json:"address,omitempty"`

	// Ports lists the allocated external ports for the game server.
	// +optional
	Ports []GameServerStatusPort `json:"ports,omitempty"`

	// Conditions represent the latest observations of the GameServer's state.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// GameServerStatusPort describes an allocated port on the game server.
type GameServerStatusPort struct {
	// Name is the descriptive identifier for this port.
	Name string `json:"name"`

	// Port is the allocated external port number.
	Port int32 `json:"port"`

	// Protocol is the network protocol for this port.
	// +optional
	Protocol corev1.Protocol `json:"protocol,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Game",type=string,JSONPath=`.spec.gameType`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Address",type=string,JSONPath=`.status.address`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:resource:shortName=gs

// GameServer is the Schema for the gameservers API.
type GameServer struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of GameServer.
	// +required
	Spec GameServerSpec `json:"spec"`

	// status defines the observed state of GameServer.
	// +optional
	Status GameServerStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// GameServerList contains a list of GameServer.
type GameServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []GameServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GameServer{}, &GameServerList{})
}
