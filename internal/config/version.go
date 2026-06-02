package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

)

// ConfigVersion represents a saved configuration snapshot.
type ConfigVersion struct {
	Version   int       `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	Path      string    `json:"path"`
}

// SaveVersioned saves the config and creates a backup in a versions directory.
// Returns the version number.
func SaveVersioned(cfg *Config, path string, comment string) (int, error) {
	// Create versions directory alongside the config file
	versionsDir := path + ".versions"
	if err := os.MkdirAll(versionsDir, 0755); err != nil {
		return 0, fmt.Errorf("create versions dir: %w", err)
	}

	// Determine next version number
	nextVer := nextVersion(versionsDir)

	// Save the versioned backup
	versionPath := fmt.Sprintf("%s/v%04d_%s.yaml", versionsDir, nextVer,
		time.Now().Format("20060102_150405"))
	if err := Save(cfg, versionPath); err != nil {
		return 0, fmt.Errorf("save version: %w", err)
	}

	// Save version metadata
	meta := ConfigVersion{
		Version:   nextVer,
		Timestamp: time.Now(),
		Path:      versionPath,
	}
	metaPath := fmt.Sprintf("%s/v%04d.meta.json", versionsDir, nextVer)
	metaData, _ := json.MarshalIndent(meta, "", "  ")
	os.WriteFile(metaPath, metaData, 0644)

	// Also save current config
	if err := Save(cfg, path); err != nil {
		return nextVer, fmt.Errorf("save current: %w", err)
	}

	return nextVer, nil
}

// ListVersions returns all saved config versions, newest first.
func ListVersions(basePath string) ([]ConfigVersion, error) {
	versionsDir := basePath + ".versions"
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var versions []ConfigVersion
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".meta.json") {
			data, err := os.ReadFile(versionsDir + "/" + e.Name())
			if err != nil {
				continue
			}
			var v ConfigVersion
			if json.Unmarshal(data, &v) == nil {
				versions = append(versions, v)
			}
		}
	}

	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Version > versions[j].Version
	})
	return versions, nil
}

// Rollback restores config to a specific version.
func Rollback(basePath string, version int) (*Config, error) {
	versionsDir := basePath + ".versions"
	versionPath := fmt.Sprintf("%s/v%04d.yaml", versionsDir, version)

	// Find the actual file (timestamps may vary)
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		return nil, fmt.Errorf("read versions dir: %w", err)
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, fmt.Sprintf("v%04d_", version)) && strings.HasSuffix(name, ".yaml") {
			versionPath = versionsDir + "/" + name
			break
		}
	}

	cfg, err := Load(versionPath)
	if err != nil {
		return nil, fmt.Errorf("load version %d: %w", version, err)
	}

	// Also save as current
	if err := Save(cfg, basePath); err != nil {
		return cfg, fmt.Errorf("save rolled-back config: %w", err)
	}

	return cfg, nil
}

func nextVersion(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 1
	}
	maxVer := 0
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "v") && strings.Contains(name, ".meta.json") {
			var v int
			if _, err := fmt.Sscanf(name, "v%04d.meta.json", &v); err == nil && v > maxVer {
				maxVer = v
			}
		}
	}
	return maxVer + 1
}

