// SPDX-License-Identifier: Apache-2.0
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-hclog"
)

const (
	// workspaceDir is the workspace-local directory used by complyctl for all artifacts.
	// Matches complyctl's internal/complytime.WorkspaceDir constant.
	workspaceDir = ".complytime"

	// ProviderDir is the subdirectory name for ampel artifacts within the workspace.
	ProviderDir = "ampel"
	// DefaultGranularPolicyDir is the default directory name for granular AMPEL policy source files.
	DefaultGranularPolicyDir = "granular-policies"
	// GeneratedPolicyDir is the workspace subdirectory for generated policy artifacts.
	GeneratedPolicyDir = "policy"
	// DefaultResultsDir is the default directory name for per-repository result files.
	DefaultResultsDir = "results"
)

// Config holds the provider configuration with hardcoded workspace-relative paths.
type Config struct{}

// NewConfig returns a new Config.
func NewConfig() *Config {
	return &Config{}
}

// WorkspaceDir returns the workspace-local directory used by complyctl for all artifacts.
func WorkspaceDir() string {
	return workspaceDir
}

// ampelDir returns the path to the ampel subdirectory within the workspace.
func ampelDir() string {
	return filepath.Join(workspaceDir, ProviderDir)
}

// EnsureDirectories creates the workspace directory structure required by the provider.
func EnsureDirectories() error {
	directories := []string{
		ampelDir(),
		GranularPolicyDirPath(),
		GeneratedPolicyDirPath(),
		ResultsDirPath(),
		SpecDirPath(),
	}
	for _, dir := range directories {
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

// SpecDirPath returns the path for the embedded spec files directory.
func SpecDirPath() string {
	return filepath.Join(ampelDir(), "specs")
}
