// SPDX-License-Identifier: Apache-2.0
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/hashicorp/go-hclog"

	"github.com/complytime/complytime-providers/internal/evidence"
)

const (
	// ProviderDir is the subdirectory name for ampel artifacts within the workspace.
	ProviderDir = "ampel"
	// DefaultGranularPolicyDir is the default directory name for granular AMPEL policy source files.
	DefaultGranularPolicyDir = "granular-policies"
	// GeneratedPolicyDir is the workspace subdirectory for generated policy artifacts.
	GeneratedPolicyDir = "policy"
	// GeneratedDir is the workspace subdirectory for target-scoped generation artifacts.
	GeneratedDir = "generated"
	// DefaultResultsDir is the default directory name for per-repository result files.
	DefaultResultsDir = "results"
)

// Config holds the provider configuration with hardcoded workspace-relative paths.
type Config struct {
	WorkspaceRoot string
}

// NewConfig returns a new Config.
func NewConfig() *Config {
	return &Config{}
}

// NewWorkspaceConfig binds provider paths to the per-RPC resolved workspace.
func NewWorkspaceConfig(root string) *Config { return &Config{WorkspaceRoot: root} }

// Path resolves provider configuration paths beneath the selected workspace root.
func (c *Config) Path(path string) string { return evidence.WorkspacePath(c.WorkspaceRoot, path) }

// StorageOptions supplies explicit provider roots and OS identity to the allocator.
// The caller resolves WorkspaceRoot once per RPC; no root selection occurs here.
func (c *Config) StorageOptions(identity evidence.Identity) evidence.StorageOptions {
	return evidence.StorageOptions{WorkspaceRoot: c.WorkspaceRoot,
		ResultsRoot: c.Path(ResultsDirPath()), Identity: identity}
}

// ampelDir returns the path to the ampel subdirectory within the workspace.
func ampelDir() string {
	return filepath.Join(provider.WorkspaceDir, ProviderDir)
}

// EnsureDirectories creates the workspace directory structure required by the provider.
func EnsureDirectories() error {
	return NewConfig().EnsureDirectories()
}

// EnsureDirectories creates generated/configuration directories beneath the
// selected root. Retained evaluation storage uses the shared constrained allocator.
func (c *Config) EnsureDirectories() error {
	directories := []string{
		ampelDir(),
		GranularPolicyDirPath(),
		GeneratedPolicyDirPath(),
		GeneratedDirPath(),
		ResultsDirPath(),
		SpecDirPath(),
	}
	if c.WorkspaceRoot != "" {
		return evidence.EnsureDirectories(c.WorkspaceRoot, directories...)
	}
	for _, dir := range directories {
		dir = c.Path(dir)
		if err := os.MkdirAll(dir, 0750); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
		hclog.Default().Debug("ensured directory", "path", dir)
	}
	return nil
}

// GranularPolicyDirPath returns the default path for granular policy source files.
func GranularPolicyDirPath() string {
	return filepath.Join(ampelDir(), DefaultGranularPolicyDir)
}

// ResultsDirPath returns the path for the per-repository result files.
func ResultsDirPath() string {
	return filepath.Join(ampelDir(), DefaultResultsDir)
}

// GeneratedPolicyDirPath returns the workspace path for generated policy output.
func GeneratedPolicyDirPath() string {
	return filepath.Join(ampelDir(), GeneratedPolicyDir)
}

// GeneratedDirPath returns the path for target-scoped generation artifacts.
func GeneratedDirPath() string {
	return filepath.Join(ampelDir(), GeneratedDir)
}

// GeneratedDirForKey returns the path for artifacts associated with contentKey.
func GeneratedDirForKey(contentKey string) string {
	return filepath.Join(GeneratedDirPath(), contentKey)
}

// ScanConfigDirPath returns the directory path where scan-config.json is stored.
// This is the same as the generated policy directory, keeping all Generate
// artifacts together.
func ScanConfigDirPath() string {
	return GeneratedPolicyDirPath()
}

// SpecDirPath returns the path for the embedded spec files directory.
func SpecDirPath() string {
	return filepath.Join(ampelDir(), "specs")
}
