// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/complytime/complytime-providers/internal/evidence"
)

const (
	// ProviderDir is the subdirectory name for OPA artifacts within the workspace.
	ProviderDir = "opa"
	// PolicyDir is the subdirectory for downloaded OPA policy bundles.
	PolicyDir = "policy"
	// ReposDir is the subdirectory for cloned git repositories.
	ReposDir = "repos"
	// ResultsDir is the subdirectory for per-target result files.
	ResultsDir = "results"
	// GeneratedDir is the subdirectory for generation artifacts (scan-config.json).
	GeneratedDir = "generated"
)

// Config holds the provider configuration with workspace-relative paths.
type Config struct {
	WorkspaceDir  string
	WorkspaceRoot string
}

// NewWorkspaceConfig derives provider paths from one resolved coordinate root.
// The configured artifact boundary for input isolation remains a server concern.
func NewWorkspaceConfig(root string) *Config {
	return &Config{WorkspaceDir: filepath.Join(root, provider.WorkspaceDir), WorkspaceRoot: root}
}

// StorageOptions supplies resolved roots and execution identity without selecting
// workspace precedence or importing provider orchestration into shared helpers.
func (c *Config) StorageOptions(identity evidence.Identity) evidence.StorageOptions {
	return evidence.StorageOptions{WorkspaceRoot: c.WorkspaceRoot,
		ResultsRoot: c.ResultsDirPath(), Identity: identity}
}

// NewConfig returns a new Config rooted at the given workspace directory.
func NewConfig(workspaceDir string) *Config {
	return &Config{WorkspaceDir: workspaceDir}
}

// OpaDir returns the path to the opa subdirectory within the workspace.
func (c *Config) OpaDir() string {
	return filepath.Join(c.WorkspaceDir, ProviderDir)
}

// PolicyDirPath returns the path for downloaded OPA policy bundles.
func (c *Config) PolicyDirPath() string {
	return filepath.Join(c.OpaDir(), PolicyDir)
}

// ReposDirPath returns the path for cloned git repositories.
func (c *Config) ReposDirPath() string {
	return filepath.Join(c.OpaDir(), ReposDir)
}

// ResultsDirPath returns the path for per-target result files.
func (c *Config) ResultsDirPath() string {
	return filepath.Join(c.OpaDir(), ResultsDir)
}

// GeneratedDirPath returns the path for generation artifacts.
func (c *Config) GeneratedDirPath() string {
	return filepath.Join(c.OpaDir(), GeneratedDir)
}

// GeneratedDirForKey returns the path for artifacts associated with contentKey.
func (c *Config) GeneratedDirForKey(contentKey string) string {
	return filepath.Join(c.GeneratedDirPath(), contentKey)
}

// PolicyDirForBundle returns a bundle-specific policy directory path.
func (c *Config) PolicyDirForBundle(bundleRef string) string {
	sanitized := bundleRef
	sanitized = strings.TrimPrefix(sanitized, "oci://")
	sanitized = strings.NewReplacer("/", "_", ":", "_").Replace(sanitized)
	return filepath.Join(c.OpaDir(), PolicyDir, sanitized)
}

// EnsureDirectories creates all workspace subdirectories with mode 0750.
func (c *Config) EnsureDirectories() error {
	directories := []string{
		c.OpaDir(),
		c.PolicyDirPath(),
		c.ReposDirPath(),
		c.ResultsDirPath(),
		c.GeneratedDirPath(),
	}
	for _, dir := range directories {
		if c.WorkspaceRoot != "" {
			relative, err := filepath.Rel(c.WorkspaceRoot, dir)
			if err != nil {
				return fmt.Errorf("resolving provider directory: %w", err)
			}
			if err := evidence.EnsureDirectories(c.WorkspaceRoot, relative); err != nil {
				return fmt.Errorf("creating directory %s: %w", dir, err)
			}
			continue
		}
		if err := os.MkdirAll(dir, 0750); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}
	return nil
}
