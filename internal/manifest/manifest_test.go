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

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeManifest creates a game subdirectory under dir and writes the given
// content as manifest.yaml inside it.
func writeManifest(t *testing.T, dir, gameName, content string) {
	t.Helper()
	gameDir := filepath.Join(dir, gameName)
	if err := os.MkdirAll(gameDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gameDir, "manifest.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadFromDirectory(t *testing.T) {
	dir := t.TempDir()

	minecraft := `name: minecraft
displayName: "Minecraft Java Edition"
image: itzg/minecraft-server:latest
ports:
  - name: game
    containerPort: 25565
    protocol: TCP
parameters:
  EULA: "TRUE"
  TYPE: "VANILLA"
resources:
  requests:
    cpu: "500m"
    memory: "1Gi"
  limits:
    cpu: "2"
    memory: "4Gi"
parameterSchema:
  type: object
  properties:
    TYPE:
      type: string
      title: "Server Type"
      description: "Minecraft server implementation"
      enum: ["VANILLA", "PAPER", "SPIGOT"]
      default: "VANILLA"
    MAX_PLAYERS:
      type: string
      title: "Max Players"
      description: "Maximum number of concurrent players"
      pattern: "^[1-9][0-9]*$"
      default: "20"
  required:
    - TYPE
`
	writeManifest(t, dir, "minecraft", minecraft)

	loader, err := LoadFromDirectory(dir)
	if err != nil {
		t.Fatalf("LoadFromDirectory() error = %v", err)
	}

	mc, ok := loader.Get("minecraft")
	if !ok {
		t.Fatal("Get(\"minecraft\") returned false, want true")
	}
	if mc.DisplayName != "Minecraft Java Edition" {
		t.Errorf("DisplayName = %q, want %q", mc.DisplayName, "Minecraft Java Edition")
	}
	if mc.Image != "itzg/minecraft-server:latest" {
		t.Errorf("Image = %q, want %q", mc.Image, "itzg/minecraft-server:latest")
	}
	if len(mc.Ports) != 1 {
		t.Fatalf("Ports has %d entries, want 1", len(mc.Ports))
	}
	if mc.Ports[0].ContainerPort != 25565 {
		t.Errorf("Ports[0].ContainerPort = %d, want 25565", mc.Ports[0].ContainerPort)
	}
	if mc.Parameters["EULA"] != "TRUE" {
		t.Errorf("Parameters[\"EULA\"] = %q, want %q", mc.Parameters["EULA"], "TRUE")
	}
	if mc.Parameters["TYPE"] != "VANILLA" {
		t.Errorf("Parameters[\"TYPE\"] = %q, want %q", mc.Parameters["TYPE"], "VANILLA")
	}
	if mc.ParameterSchema == nil {
		t.Fatal("ParameterSchema is nil, want non-nil")
	}
	props, ok := mc.ParameterSchema["properties"]
	if !ok {
		t.Fatal("ParameterSchema missing \"properties\" key")
	}
	propsMap, ok := props.(map[string]interface{})
	if !ok {
		t.Fatal("ParameterSchema[\"properties\"] is not a map")
	}
	if _, ok := propsMap["TYPE"]; !ok {
		t.Error("ParameterSchema properties missing \"TYPE\"")
	}
	if _, ok := propsMap["MAX_PLAYERS"]; !ok {
		t.Error("ParameterSchema properties missing \"MAX_PLAYERS\"")
	}
	if !mc.HasSchema() {
		t.Error("HasSchema() = false, want true")
	}
}

func TestLoadFromDirectory_MultipleGames(t *testing.T) {
	dir := t.TempDir()

	minecraft := `name: minecraft
displayName: "Minecraft Java Edition"
image: itzg/minecraft-server:latest
ports:
  - name: game
    containerPort: 25565
    protocol: TCP
parameters:
  EULA: "TRUE"
`
	valheim := `name: valheim
displayName: "Valheim Dedicated Server"
image: lloesche/valheim-server:latest
ports:
  - name: game
    containerPort: 2456
    protocol: UDP
  - name: query
    containerPort: 2457
    protocol: UDP
parameters:
  SERVER_NAME: "My Server"
`
	writeManifest(t, dir, "minecraft", minecraft)
	writeManifest(t, dir, "valheim", valheim)

	loader, err := LoadFromDirectory(dir)
	if err != nil {
		t.Fatalf("LoadFromDirectory() error = %v", err)
	}

	list := loader.List()
	if len(list) != 2 {
		t.Fatalf("List() returned %d manifests, want 2", len(list))
	}

	// Sorted alphabetically
	if list[0].Name != "minecraft" {
		t.Errorf("List()[0].Name = %q, want %q", list[0].Name, "minecraft")
	}
	if list[1].Name != "valheim" {
		t.Errorf("List()[1].Name = %q, want %q", list[1].Name, "valheim")
	}

	// Verify both are accessible via Get
	_, ok := loader.Get("minecraft")
	if !ok {
		t.Error("Get(\"minecraft\") returned false")
	}
	_, ok = loader.Get("valheim")
	if !ok {
		t.Error("Get(\"valheim\") returned false")
	}
}

func TestLoadFromDirectory_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	_, err := LoadFromDirectory(dir)
	if err == nil {
		t.Fatal("LoadFromDirectory() on empty dir should return error")
	}
	if !strings.Contains(err.Error(), "no valid game manifests found") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "no valid game manifests found")
	}
}

func TestLoadFromDirectory_NoManifestInSubdir(t *testing.T) {
	dir := t.TempDir()

	// Create a subdirectory but no manifest.yaml inside
	if err := os.MkdirAll(filepath.Join(dir, "emptygame"), 0755); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFromDirectory(dir)
	if err == nil {
		t.Fatal("LoadFromDirectory() should return error when no manifests found")
	}
	if !strings.Contains(err.Error(), "no valid game manifests found") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "no valid game manifests found")
	}
}

func TestLoadFromDirectory_InvalidYAML(t *testing.T) {
	dir := t.TempDir()

	writeManifest(t, dir, "badgame", `{{{invalid yaml content`)

	_, err := LoadFromDirectory(dir)
	if err == nil {
		t.Fatal("LoadFromDirectory() with invalid YAML should return error")
	}
	if !strings.Contains(err.Error(), "failed to parse") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "failed to parse")
	}
}

func TestLoadFromDirectory_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		content string
		errMsg  string
	}{
		{
			name: "missing name",
			content: `displayName: "No Name Game"
image: example/game:latest
`,
			errMsg: "name field is required",
		},
		{
			name: "missing image",
			content: `name: noimage
displayName: "No Image Game"
`,
			errMsg: "image field is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeManifest(t, dir, "testgame", tt.content)

			_, err := LoadFromDirectory(dir)
			if err == nil {
				t.Fatal("LoadFromDirectory() should return error")
			}
			if !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestLoadFromDirectory_NoSchema(t *testing.T) {
	dir := t.TempDir()

	noSchema := `name: simplgame
displayName: "Simple Game"
image: example/simple:latest
ports:
  - name: game
    containerPort: 27015
    protocol: UDP
parameters:
  MAP: "de_dust2"
`
	writeManifest(t, dir, "simplgame", noSchema)

	loader, err := LoadFromDirectory(dir)
	if err != nil {
		t.Fatalf("LoadFromDirectory() error = %v", err)
	}

	m, ok := loader.Get("simplgame")
	if !ok {
		t.Fatal("Get(\"simplgame\") returned false")
	}
	if m.ParameterSchema != nil {
		t.Errorf("ParameterSchema = %v, want nil", m.ParameterSchema)
	}
	if m.HasSchema() {
		t.Error("HasSchema() = true, want false")
	}
	// ValidateParameters with no schema should return nil (no validation)
	if err := m.ValidateParameters(map[string]string{"anything": "goes"}); err != nil {
		t.Errorf("ValidateParameters() error = %v, want nil", err)
	}
}

func TestValidateParameters_Valid(t *testing.T) {
	dir := t.TempDir()

	manifest := `name: testgame
displayName: "Test Game"
image: example/test:latest
parameterSchema:
  type: object
  properties:
    DIFFICULTY:
      type: string
      enum: ["easy", "normal", "hard"]
    MAX_PLAYERS:
      type: string
      pattern: "^[1-9][0-9]*$"
`
	writeManifest(t, dir, "testgame", manifest)

	loader, err := LoadFromDirectory(dir)
	if err != nil {
		t.Fatalf("LoadFromDirectory() error = %v", err)
	}

	m, _ := loader.Get("testgame")
	err = m.ValidateParameters(map[string]string{
		"DIFFICULTY":  "normal",
		"MAX_PLAYERS": "20",
	})
	if err != nil {
		t.Errorf("ValidateParameters() error = %v, want nil", err)
	}
}

func TestValidateParameters_Invalid(t *testing.T) {
	dir := t.TempDir()

	manifest := `name: testgame
displayName: "Test Game"
image: example/test:latest
parameterSchema:
  type: object
  properties:
    DIFFICULTY:
      type: string
      enum: ["easy", "normal", "hard"]
`
	writeManifest(t, dir, "testgame", manifest)

	loader, err := LoadFromDirectory(dir)
	if err != nil {
		t.Fatalf("LoadFromDirectory() error = %v", err)
	}

	m, _ := loader.Get("testgame")
	err = m.ValidateParameters(map[string]string{
		"DIFFICULTY": "nightmare",
	})
	if err == nil {
		t.Fatal("ValidateParameters() should return error for invalid enum value")
	}
	if !strings.Contains(err.Error(), "parameter validation failed") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "parameter validation failed")
	}
}

func TestValidateParameters_RequiredMissing(t *testing.T) {
	dir := t.TempDir()

	manifest := `name: testgame
displayName: "Test Game"
image: example/test:latest
parameterSchema:
  type: object
  properties:
    EULA:
      type: string
      const: "TRUE"
    TYPE:
      type: string
  required:
    - EULA
`
	writeManifest(t, dir, "testgame", manifest)

	loader, err := LoadFromDirectory(dir)
	if err != nil {
		t.Fatalf("LoadFromDirectory() error = %v", err)
	}

	m, _ := loader.Get("testgame")
	// Pass an empty map -- EULA is required but missing
	err = m.ValidateParameters(map[string]string{})
	if err == nil {
		t.Fatal("ValidateParameters() should return error when required field is missing")
	}
	if !strings.Contains(err.Error(), "parameter validation failed") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "parameter validation failed")
	}
}

func TestGet_NotFound(t *testing.T) {
	dir := t.TempDir()

	writeManifest(t, dir, "testgame", `name: testgame
displayName: "Test Game"
image: example/test:latest
`)

	loader, err := LoadFromDirectory(dir)
	if err != nil {
		t.Fatalf("LoadFromDirectory() error = %v", err)
	}

	_, ok := loader.Get("nonexistent")
	if ok {
		t.Error("Get(\"nonexistent\") returned true, want false")
	}
}

func TestLoadFromDirectory_NonexistentDir(t *testing.T) {
	_, err := LoadFromDirectory("/tmp/definitely-does-not-exist-" + t.Name())
	if err == nil {
		t.Fatal("LoadFromDirectory() on nonexistent dir should return error")
	}
}

func TestLoadFromDirectory_ManifestYml(t *testing.T) {
	dir := t.TempDir()

	// Use .yml extension instead of .yaml
	gameDir := filepath.Join(dir, "ymlgame")
	if err := os.MkdirAll(gameDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `name: ymlgame
displayName: "YML Game"
image: example/yml:latest
`
	if err := os.WriteFile(filepath.Join(gameDir, "manifest.yml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	loader, err := LoadFromDirectory(dir)
	if err != nil {
		t.Fatalf("LoadFromDirectory() error = %v", err)
	}

	m, ok := loader.Get("ymlgame")
	if !ok {
		t.Fatal("Get(\"ymlgame\") returned false, want true")
	}
	if m.DisplayName != "YML Game" {
		t.Errorf("DisplayName = %q, want %q", m.DisplayName, "YML Game")
	}
}

func TestLoadFromDirectory_SkipsNonDirEntries(t *testing.T) {
	dir := t.TempDir()

	// Write a valid game in a subdirectory
	writeManifest(t, dir, "realgame", `name: realgame
displayName: "Real Game"
image: example/real:latest
`)

	// Write a flat file at the top level (should be ignored)
	if err := os.WriteFile(filepath.Join(dir, "stray.yaml"), []byte("name: stray\nimage: x\n"), 0644); err != nil {
		t.Fatal(err)
	}

	loader, err := LoadFromDirectory(dir)
	if err != nil {
		t.Fatalf("LoadFromDirectory() error = %v", err)
	}

	list := loader.List()
	if len(list) != 1 {
		t.Fatalf("List() returned %d manifests, want 1", len(list))
	}
	if list[0].Name != "realgame" {
		t.Errorf("List()[0].Name = %q, want %q", list[0].Name, "realgame")
	}

	// The flat file "stray" should not be loaded
	_, ok := loader.Get("stray")
	if ok {
		t.Error("Get(\"stray\") returned true -- flat files should be ignored")
	}
}
