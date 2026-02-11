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
	"testing"
)

func TestLoadFromDirectory(t *testing.T) {
	dir := t.TempDir()

	// Write two valid manifest files
	minecraft := `name: minecraft
displayName: "Minecraft Java Edition"
image: itzg/minecraft-server:latest
ports:
  - name: game
    containerPort: 25565
    protocol: TCP
parameters:
  EULA: "TRUE"
resources:
  requests:
    cpu: "500m"
    memory: "1Gi"
  limits:
    cpu: "2"
    memory: "4Gi"
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
resources:
  requests:
    cpu: "1"
    memory: "2Gi"
  limits:
    cpu: "2"
    memory: "4Gi"
`

	if err := os.WriteFile(filepath.Join(dir, "minecraft.yaml"), []byte(minecraft), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "valheim.yml"), []byte(valheim), 0644); err != nil {
		t.Fatal(err)
	}

	loader, err := LoadFromDirectory(dir)
	if err != nil {
		t.Fatalf("LoadFromDirectory() error = %v", err)
	}

	// Verify count via List
	list := loader.List()
	if len(list) != 2 {
		t.Fatalf("List() returned %d manifests, want 2", len(list))
	}

	// Verify List returns sorted order (minecraft before valheim)
	if list[0].Name != "minecraft" {
		t.Errorf("List()[0].Name = %q, want %q", list[0].Name, "minecraft")
	}
	if list[1].Name != "valheim" {
		t.Errorf("List()[1].Name = %q, want %q", list[1].Name, "valheim")
	}

	// Verify Get returns correct manifest
	mc, ok := loader.Get("minecraft")
	if !ok {
		t.Fatal("Get(\"minecraft\") returned false, want true")
	}
	if mc.DisplayName != "Minecraft Java Edition" {
		t.Errorf("Get(\"minecraft\").DisplayName = %q, want %q", mc.DisplayName, "Minecraft Java Edition")
	}
	if mc.Image != "itzg/minecraft-server:latest" {
		t.Errorf("Get(\"minecraft\").Image = %q, want %q", mc.Image, "itzg/minecraft-server:latest")
	}
	if len(mc.Ports) != 1 {
		t.Fatalf("Get(\"minecraft\").Ports has %d entries, want 1", len(mc.Ports))
	}
	if mc.Ports[0].ContainerPort != 25565 {
		t.Errorf("Get(\"minecraft\").Ports[0].ContainerPort = %d, want %d", mc.Ports[0].ContainerPort, 25565)
	}
	if mc.Parameters["EULA"] != "TRUE" {
		t.Errorf("Get(\"minecraft\").Parameters[\"EULA\"] = %q, want %q", mc.Parameters["EULA"], "TRUE")
	}

	// Verify valheim
	vh, ok := loader.Get("valheim")
	if !ok {
		t.Fatal("Get(\"valheim\") returned false, want true")
	}
	if len(vh.Ports) != 2 {
		t.Fatalf("Get(\"valheim\").Ports has %d entries, want 2", len(vh.Ports))
	}
}

func TestLoadFromDirectory_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	_, err := LoadFromDirectory(dir)
	if err == nil {
		t.Fatal("LoadFromDirectory() on empty dir should return error")
	}
}

func TestLoadFromDirectory_InvalidYAML(t *testing.T) {
	dir := t.TempDir()

	invalidYAML := `{{{invalid yaml content`
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte(invalidYAML), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFromDirectory(dir)
	if err == nil {
		t.Fatal("LoadFromDirectory() with invalid YAML should return error")
	}
}

func TestLoadFromDirectory_MissingName(t *testing.T) {
	dir := t.TempDir()

	noName := `displayName: "No Name Game"
image: example/game:latest
`
	if err := os.WriteFile(filepath.Join(dir, "noname.yaml"), []byte(noName), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFromDirectory(dir)
	if err == nil {
		t.Fatal("LoadFromDirectory() with missing name field should return error")
	}
}

func TestLoadFromDirectory_MissingImage(t *testing.T) {
	dir := t.TempDir()

	noImage := `name: noimage
displayName: "No Image Game"
`
	if err := os.WriteFile(filepath.Join(dir, "noimage.yaml"), []byte(noImage), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFromDirectory(dir)
	if err == nil {
		t.Fatal("LoadFromDirectory() with missing image field should return error")
	}
}

func TestGet_NotFound(t *testing.T) {
	dir := t.TempDir()

	valid := `name: testgame
displayName: "Test Game"
image: example/test:latest
`
	if err := os.WriteFile(filepath.Join(dir, "test.yaml"), []byte(valid), 0644); err != nil {
		t.Fatal(err)
	}

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
