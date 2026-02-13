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

// Backup lifecycle state constants.
const (
	BackupStatePending    BackupState = "Pending"
	BackupStateInProgress BackupState = "InProgress"
	BackupStateCompleted  BackupState = "Completed"
	BackupStateFailed     BackupState = "Failed"
)

// BackupValidTransitions defines the allowed state transitions for a Backup.
// Each key is a source state, and the value is a slice of valid target states.
var BackupValidTransitions = map[BackupState][]BackupState{
	BackupStatePending:    {BackupStateInProgress, BackupStateFailed},
	BackupStateInProgress: {BackupStateCompleted, BackupStateFailed},
}

// IsBackupValidTransition checks whether transitioning from one state to another is allowed.
func IsBackupValidTransition(from, to BackupState) bool {
	for _, valid := range BackupValidTransitions[from] {
		if valid == to {
			return true
		}
	}
	return false
}

// IsBackupTerminal returns true if the given state is a terminal state (no further transitions).
func IsBackupTerminal(state BackupState) bool {
	return state == BackupStateCompleted || state == BackupStateFailed
}

// Status condition type constants for Backup resources.
const (
	// BackupConditionReady indicates the Backup has completed successfully.
	BackupConditionReady = "Ready"
)
