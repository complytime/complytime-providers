// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
)

// Config holds the provider configuration with workspace-relative paths.
type Config struct {
	WorkspaceDir string
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
	}
	for _, dir := range directories {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}
	return nil
}
