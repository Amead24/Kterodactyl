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

// GameServer lifecycle state constants.
const (
	GameServerStateCreating  GameServerState = "Creating"
	GameServerStateStarting  GameServerState = "Starting"
	GameServerStateReady     GameServerState = "Ready"
	GameServerStateAllocated GameServerState = "Allocated"
	GameServerStateShutdown  GameServerState = "Shutdown"
	GameServerStateError     GameServerState = "Error"
)

// ValidTransitions defines the allowed state transitions for a GameServer.
// Each key is a source state, and the value is a slice of valid target states.
var ValidTransitions = map[GameServerState][]GameServerState{
	GameServerStateCreating:  {GameServerStateStarting, GameServerStateError},
	GameServerStateStarting:  {GameServerStateReady, GameServerStateCreating, GameServerStateError, GameServerStateShutdown},
	GameServerStateReady:     {GameServerStateAllocated, GameServerStateError, GameServerStateShutdown},
	GameServerStateAllocated: {GameServerStateReady, GameServerStateError, GameServerStateShutdown},
	GameServerStateShutdown:  {}, // Terminal state: no transitions allowed
	GameServerStateError:     {GameServerStateShutdown}, // Can only shutdown from error
}

// IsValidTransition checks whether transitioning from one state to another is allowed.
func IsValidTransition(from, to GameServerState) bool {
	for _, valid := range ValidTransitions[from] {
		if valid == to {
			return true
		}
	}
	return false
}

// IsTerminal returns true if the given state is a terminal state (no further transitions).
func IsTerminal(state GameServerState) bool {
	return state == GameServerStateShutdown
}

// Status condition type constants for GameServer resources.
const (
	// TypeReady indicates the GameServer is fully operational.
	TypeReady = "Ready"

	// TypeProgressing indicates the GameServer is being created or updated.
	TypeProgressing = "Progressing"

	// TypeDegraded indicates the GameServer has failed to reach or maintain desired state.
	TypeDegraded = "Degraded"
)

// FinalizerName is the finalizer added to GameServer resources for cleanup.
const FinalizerName = "game.kterodactyl.io/finalizer"
