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
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"gopkg.in/yaml.v3"

	gamev1alpha1 "github.com/kterodactyl/kterodactyl/api/v1alpha1"
)

// GameManifest defines a game type template loaded from a YAML file.
// It specifies the container image, ports, default parameters, and resource requirements
// for creating GameServer instances of this game type.
type GameManifest struct {
	// Name is the unique identifier for the game type (must match filename stem).
	Name string `yaml:"name"`

	// DisplayName is the human-readable name shown in the UI.
	DisplayName string `yaml:"displayName"`

	// Image is the container image to run for this game type.
	Image string `yaml:"image"`

	// Ports defines the network ports exposed by the game server.
	Ports []gamev1alpha1.GameServerPort `yaml:"ports"`

	// Parameters holds default game-specific configuration key-value pairs.
	Parameters map[string]string `yaml:"parameters"`

	// Resources defines CPU/memory requests and limits for the game server container.
	Resources corev1.ResourceRequirements `yaml:"-"`
}

// rawGameManifest is an intermediate type for YAML unmarshaling that handles
// resource.Quantity fields as strings (since resource.Quantity only implements
// JSON Unmarshaler, not yaml.v3 Unmarshaler) and ports with explicit yaml tags
// (since GameServerPort only has json tags).
type rawGameManifest struct {
	Name        string            `yaml:"name"`
	DisplayName string            `yaml:"displayName"`
	Image       string            `yaml:"image"`
	Ports       []rawPort         `yaml:"ports"`
	Parameters  map[string]string `yaml:"parameters"`
	Resources   rawResources      `yaml:"resources"`
}

// rawPort mirrors gamev1alpha1.GameServerPort with yaml tags for YAML parsing.
type rawPort struct {
	Name          string        `yaml:"name"`
	ContainerPort int32         `yaml:"containerPort"`
	Protocol      corev1.Protocol `yaml:"protocol"`
}

// rawResources mirrors corev1.ResourceRequirements with string values for YAML parsing.
type rawResources struct {
	Requests map[string]string `yaml:"requests"`
	Limits   map[string]string `yaml:"limits"`
}

// parseResourceList converts a map of string quantities to a corev1.ResourceList.
func parseResourceList(raw map[string]string) (corev1.ResourceList, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	result := make(corev1.ResourceList, len(raw))
	for k, v := range raw {
		q, err := resource.ParseQuantity(v)
		if err != nil {
			return nil, fmt.Errorf("invalid resource quantity for %s=%q: %w", k, v, err)
		}
		result[corev1.ResourceName(k)] = q
	}
	return result, nil
}

// Loader holds loaded game manifests and provides access to them by name.
type Loader struct {
	manifests map[string]*GameManifest
}

// LoadFromDirectory reads all .yaml and .yml files from the given directory,
// parses them into GameManifest structs, validates required fields, and returns
// a Loader with all manifests accessible by name.
//
// Returns an error if the directory does not exist, contains no valid manifests,
// or any manifest is missing required fields (Name, Image).
func LoadFromDirectory(dir string) (*Loader, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest directory %s: %w", dir, err)
	}

	manifests := make(map[string]*GameManifest)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read manifest file %s: %w", filePath, err)
		}

		var raw rawGameManifest
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("failed to parse manifest file %s: %w", filePath, err)
		}

		// Validate required fields
		if raw.Name == "" {
			return nil, fmt.Errorf("manifest file %s: name field is required", filePath)
		}
		if raw.Image == "" {
			return nil, fmt.Errorf("manifest file %s: image field is required", filePath)
		}

		// Parse resource quantities from strings
		requests, err := parseResourceList(raw.Resources.Requests)
		if err != nil {
			return nil, fmt.Errorf("manifest file %s: invalid requests: %w", filePath, err)
		}
		limits, err := parseResourceList(raw.Resources.Limits)
		if err != nil {
			return nil, fmt.Errorf("manifest file %s: invalid limits: %w", filePath, err)
		}

		// Convert raw ports to GameServerPort types
		ports := make([]gamev1alpha1.GameServerPort, len(raw.Ports))
		for i, rp := range raw.Ports {
			ports[i] = gamev1alpha1.GameServerPort{
				Name:          rp.Name,
				ContainerPort: rp.ContainerPort,
				Protocol:      rp.Protocol,
			}
		}

		m := &GameManifest{
			Name:        raw.Name,
			DisplayName: raw.DisplayName,
			Image:       raw.Image,
			Ports:       ports,
			Parameters:  raw.Parameters,
			Resources: corev1.ResourceRequirements{
				Requests: requests,
				Limits:   limits,
			},
		}

		manifests[m.Name] = m
	}

	if len(manifests) == 0 {
		return nil, fmt.Errorf("no valid game manifests found in %s", dir)
	}

	return &Loader{manifests: manifests}, nil
}

// Get returns the GameManifest with the given name and a boolean indicating whether it was found.
func (l *Loader) Get(name string) (*GameManifest, bool) {
	m, ok := l.manifests[name]
	return m, ok
}

// List returns all loaded game manifests sorted alphabetically by name.
func (l *Loader) List() []*GameManifest {
	result := make([]*GameManifest, 0, len(l.manifests))
	for _, m := range l.manifests {
		result = append(result, m)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}
