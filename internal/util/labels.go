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

package util

import "fmt"

// Standard Kubernetes label keys used by kterodactyl.
const (
	// LabelManagedBy is the standard Kubernetes label indicating the tool managing the resource.
	// Value should be "kterodactyl" for all resources managed by this operator.
	LabelManagedBy = "app.kubernetes.io/managed-by"

	// LabelName is the standard Kubernetes label for the application name.
	// Value should be "gameserver" for GameServer-related resources.
	LabelName = "app.kubernetes.io/name"
)

// Kterodactyl-specific label keys.
const (
	// LabelOwner identifies the user who owns a GameServer resource.
	LabelOwner = "kterodactyl.io/owner"

	// LabelGame identifies the game type of a GameServer resource.
	LabelGame = "kterodactyl.io/game"

	// LabelUser identifies the user associated with a namespace.
	LabelUser = "kterodactyl.io/user"

	// LabelManagedByKterodactyl indicates a namespace is managed by the kterodactyl operator.
	// Value should be "kterodactyl".
	LabelManagedByKterodactyl = "kterodactyl.io/managed-by"

	// LabelBackupGameServer identifies which GameServer a Backup belongs to.
	LabelBackupGameServer = "kterodactyl.io/backup-gameserver"
)

// Kterodactyl-specific annotation keys.
const (
	// AnnotationAllocated marks a GameServer as allocated (for Phase 1 kubectl-based allocation).
	AnnotationAllocated = "kterodactyl.io/allocated"

	// AnnotationModPath stores the container directory where mod files are stored.
	// Set by the API handler at GameServer creation time from the game manifest's modPath field.
	// Empty or absent means the game does not support mods.
	AnnotationModPath = "kterodactyl.io/mod-path"

	// AnnotationBackupPath stores the container directory to back up.
	// Set by the API handler at GameServer creation time from the game manifest's backupPath field.
	// Defaults to "/data" if not specified.
	AnnotationBackupPath = "kterodactyl.io/backup-path"

	// AnnotationBackupSchedule stores a cron expression for scheduled backups.
	// Set by admin via API on the GameServer.
	AnnotationBackupSchedule = "kterodactyl.io/backup-schedule"

	// AnnotationBackupRetention stores the max number of backups to retain per server.
	AnnotationBackupRetention = "kterodactyl.io/backup-retention"

	// AnnotationLastBackupTime stores the timestamp of the last scheduled backup.
	AnnotationLastBackupTime = "kterodactyl.io/last-backup-time"
)

// Standard label values.
const (
	// ManagedByValue is the value for LabelManagedBy and LabelManagedByKterodactyl.
	ManagedByValue = "kterodactyl"

	// AppNameValue is the value for LabelName on GameServer resources.
	AppNameValue = "gameserver"
)

// UserNamespace returns the namespace name for a given username.
// Format: "user-<username>"
func UserNamespace(username string) string {
	return fmt.Sprintf("user-%s", username)
}

// GameServerLabels returns the standard label set for a GameServer resource.
func GameServerLabels(owner, gameType string) map[string]string {
	return map[string]string{
		LabelManagedBy: ManagedByValue,
		LabelName:      AppNameValue,
		LabelOwner:     owner,
		LabelGame:      gameType,
	}
}
