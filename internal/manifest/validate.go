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

package manifest

import "fmt"

// ValidateParameters validates user-supplied parameters against the
// manifest's compiled JSON Schema. Returns nil if validation passes
// or if the manifest has no schema defined.
func (m *GameManifest) ValidateParameters(params map[string]string) error {
	if m.compiledSchema == nil {
		return nil
	}

	// Convert string params to interface{} map for schema validation.
	// Parameters are always strings (environment variables), which matches
	// the JSON Schema type: string definitions in the manifest.
	instance := make(map[string]interface{}, len(params))
	for k, v := range params {
		instance[k] = v
	}

	if err := m.compiledSchema.Validate(instance); err != nil {
		return fmt.Errorf("parameter validation failed: %w", err)
	}
	return nil
}

// HasSchema reports whether this manifest has a compiled JSON Schema
// for parameter validation.
func (m *GameManifest) HasSchema() bool {
	return m.compiledSchema != nil
}
