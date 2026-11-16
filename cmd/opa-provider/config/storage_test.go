// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/complytime/complytime-providers/internal/evidence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureDirectories_PrivateWorkingData(t *testing.T) {
	root := t.TempDir()
	cfg := NewWorkspaceConfig(root)
	require.NoError(t, os.MkdirAll(cfg.WorkspaceDir, 0o755))
	require.NoError(t, os.Chmod(cfg.WorkspaceDir, 0o755))
	for range 2 {
		require.NoError(t, cfg.EnsureDirectories())
		for _, path := range []string{cfg.OpaDir(), cfg.ReposDirPath(), cfg.PolicyDirPath(), cfg.GeneratedDirPath()} {
			info, err := os.Stat(path)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o700), info.Mode().Perm(), path)
		}
	}
	info, err := os.Stat(cfg.WorkspaceDir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm(), "unrelated ancestor stays unchanged")
	require.NoError(t, os.Chmod(cfg.ReposDirPath(), 0o755))
	require.ErrorContains(t, cfg.EnsureDirectories(), "mode 0700")
	info, err = os.Stat(cfg.ReposDirPath())
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm(), "do not silently chmod existing directories")
}

func TestEnsureDirectories_RejectsWorkingDirectorySymlink(t *testing.T) {
	root := t.TempDir()
	cfg := NewWorkspaceConfig(root)
	require.NoError(t, os.MkdirAll(cfg.OpaDir(), 0o700))
	external := t.TempDir()
	require.NoError(t, os.Symlink(external, cfg.ReposDirPath()))
	require.Error(t, cfg.EnsureDirectories())
	entries, err := os.ReadDir(external)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestStorageOptions_ResolvedProviderRoots(t *testing.T) {
	root := t.TempDir()
	identity, err := evidence.ExecutionIdentity()
	require.NoError(t, err)
	cfg := NewWorkspaceConfig(root)
	options := cfg.StorageOptions(identity)
	assert.Equal(t, root, options.WorkspaceRoot)
	assert.Equal(t, identity, options.Identity)
	assert.Equal(t, filepath.Join(root, provider.WorkspaceDir, ProviderDir, ResultsDir), options.ResultsRoot)
	evaluation, err := evidence.Allocate(options, "target")
	require.NoError(t, err)
	assert.DirExists(t, evaluation.Directory)
	require.NoError(t, evaluation.Close())
	options.ResultsRoot = t.TempDir()
	_, err = evidence.Allocate(options, "target")
	require.ErrorContains(t, err, "beneath workspace")
}
