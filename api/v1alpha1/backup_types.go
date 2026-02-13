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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BackupState describes the lifecycle state of a Backup.
// +kubebuilder:validation:Enum=Pending;InProgress;Completed;Failed
type BackupState string

// BackupSpec defines the desired state of Backup.
type BackupSpec struct {
	// GameServerName references the GameServer to back up.
	// +kubebuilder:validation:MinLength=1
	GameServerName string `json:"gameServerName"`

	// BackupPaths lists container paths to include in the backup.
	// If empty, uses the backupPath annotation from the GameServer.
	// +optional
	BackupPaths []string `json:"backupPaths,omitempty"`
}

// BackupStatus defines the observed state of Backup.
type BackupStatus struct {
	// State is the current backup lifecycle state.
	// +kubebuilder:validation:Enum=Pending;InProgress;Completed;Failed
	State BackupState `json:"state,omitempty"`

	// S3Key is the object key in the S3 bucket.
	S3Key string `json:"s3Key,omitempty"`

	// S3Bucket is the S3 bucket name.
	S3Bucket string `json:"s3Bucket,omitempty"`

	// Size is the backup size in bytes.
	Size int64 `json:"size,omitempty"`

	// StartedAt is when the backup started.
	StartedAt *metav1.Time `json:"startedAt,omitempty"`

	// CompletedAt is when the backup completed.
	CompletedAt *metav1.Time `json:"completedAt,omitempty"`

	// Message provides human-readable status details.
	Message string `json:"message,omitempty"`

	// Conditions represent the latest observations.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="GameServer",type=string,JSONPath=`.spec.gameServerName`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Size",type=integer,JSONPath=`.status.size`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:resource:shortName=bk

// Backup is the Schema for the backups API.
type Backup struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Backup.
	// +required
	Spec BackupSpec `json:"spec"`

	// status defines the observed state of Backup.
	// +optional
	Status BackupStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// BackupList contains a list of Backup.
type BackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Backup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Backup{}, &BackupList{})
}
