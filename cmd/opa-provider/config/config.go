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

// EnsureDirectories creates private working directories with mode 0700 and
// ordinary retained-result context parents. Existing working leaves must already
// be private; unrelated ancestors retain their modes and ownership.
func (c *Config) EnsureDirectories() error {
	root := c.WorkspaceRoot
	if root == "" {
		var err error
		root, err = filepath.Abs(c.WorkspaceDir)
		if err != nil {
			return fmt.Errorf("resolving provider workspace: %w", err)
		}
		if err := os.MkdirAll(root, evidence.DirectoryMode); err != nil {
			return fmt.Errorf("creating provider workspace: %w", err)
		}
	}
	directories := []string{
		c.OpaDir(),
		c.PolicyDirPath(),
		c.ReposDirPath(),
		c.ResultsDirPath(),
		c.GeneratedDirPath(),
	}
	for _, dir := range directories {
		absolute, err := filepath.Abs(dir)
		if err != nil {
			return fmt.Errorf("resolving provider directory: %w", err)
		}
		relative, err := filepath.Rel(root, absolute)
		if err != nil {
			return fmt.Errorf("resolving provider directory: %w", err)
		}
		if dir == c.ResultsDirPath() {
			err = evidence.EnsureDirectories(root, relative)
		} else {
			err = evidence.EnsurePrivateDirectory(root, relative)
		}
		if err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}
	return nil
}
