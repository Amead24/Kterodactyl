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

// Networking-related annotation keys.
const (
	// AnnotationDNSName stores the computed DNS name for a GameServer.
	// Format: game.username.baseDomain (e.g., minecraft.alice.example.com)
	AnnotationDNSName = "kterodactyl.io/dns-name"

	// AnnotationExternalDNSTTL is the ExternalDNS TTL annotation.
	// Used to set the TTL for DNS records managed by ExternalDNS.
	AnnotationExternalDNSTTL = "external-dns.alpha.kubernetes.io/ttl"
)

// Networking-related label keys.
const (
	// LabelHTTPRouteOwner links an HTTPRoute back to the GameServer that owns it.
	// Value is the GameServer name (namespace/name).
	LabelHTTPRouteOwner = "kterodactyl.io/gameserver"
)

// GameServerDNSName constructs the DNS name for a GameServer.
// Format: game.username.baseDomain (e.g., minecraft.alice.example.com)
func GameServerDNSName(gameType, owner, baseDomain string) string {
	return fmt.Sprintf("%s.%s.%s", gameType, owner, baseDomain)
}
