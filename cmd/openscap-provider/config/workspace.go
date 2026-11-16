// SPDX-License-Identifier: Apache-2.0

package config

import (
	"path/filepath"

	"github.com/complytime/complyctl/pkg/provider"

	"github.com/complytime/complytime-providers/internal/evidence"
)

// WorkspaceConfig supplies explicit Generate/Scan paths without changing globals.
// Root must be the workspace resolved once for the current RPC.
type WorkspaceConfig struct{ Root string }

// Path resolves relative configuration against the workspace, leaving absolute
// external inputs unchanged. Empty inputs remain available for auto-detection.
func (cfg WorkspaceConfig) Path(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(cfg.Root, path)
}

// ResolveDatastream validates an explicit path or discovers a matching datastream.
// Relative paths, including discovery configuration, use the resolved workspace.
// Missing or invalid inputs return an error without changing the process directory.
func (cfg WorkspaceConfig) ResolveDatastream(path string) (string, error) {
	if path == "" {
		return findMatchingDatastream(cfg)
	}
	expanded, err := SanitizePath(path)
	if err != nil {
		return "", err
	}
	return ResolveDatastream(cfg.Path(expanded))
}

// ProviderPath returns the absolute provider artifact directory.
func (cfg WorkspaceConfig) ProviderPath() string {
	return filepath.Join(cfg.Root, provider.WorkspaceDir, ProviderDir)
}

// PolicyPath returns the Generate-time tailoring location for this workspace.
func (cfg WorkspaceConfig) PolicyPath() string {
	return filepath.Join(cfg.ProviderPath(), PolicyDir, TailoringFile)
}

// EnsureDirectories creates confined Generate directories without following child
// symlinks or altering existing directories. Retained Scan outputs are allocated separately.
func (cfg WorkspaceConfig) EnsureDirectories() error {
	return evidence.EnsureDirectories(cfg.Root,
		filepath.Join(provider.WorkspaceDir, ProviderDir, PolicyDir),
		filepath.Join(provider.WorkspaceDir, ProviderDir, RemediationDir))
}
