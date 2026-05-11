// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := NewConfig(dir)
	assert.Equal(t, dir, cfg.WorkspaceDir)
}

func TestOpaDir(t *testing.T) {
	dir := t.TempDir()
	cfg := NewConfig(dir)
	assert.Equal(t, filepath.Join(dir, "opa"), cfg.OpaDir())
}

func TestPolicyDirPath(t *testing.T) {
	dir := t.TempDir()
	cfg := NewConfig(dir)
	assert.Equal(t, filepath.Join(dir, "opa", "policy"), cfg.PolicyDirPath())
}

func TestReposDirPath(t *testing.T) {
	dir := t.TempDir()
	cfg := NewConfig(dir)
	assert.Equal(t, filepath.Join(dir, "opa", "repos"), cfg.ReposDirPath())
}

func TestResultsDirPath(t *testing.T) {
	dir := t.TempDir()
	cfg := NewConfig(dir)
	assert.Equal(t, filepath.Join(dir, "opa", "results"), cfg.ResultsDirPath())
}

func TestEnsureDirectories(t *testing.T) {
	dir := t.TempDir()
	cfg := NewConfig(dir)

	err := cfg.EnsureDirectories()
	require.NoError(t, err)

	for _, subdir := range []string{
		cfg.OpaDir(),
		cfg.PolicyDirPath(),
		cfg.ReposDirPath(),
		cfg.ResultsDirPath(),
	} {
		info, err := os.Stat(subdir)
		require.NoError(t, err, "directory should exist: %s", subdir)
		assert.True(t, info.IsDir())
		assert.Equal(t, os.FileMode(0750), info.Mode().Perm())
	}
}

func TestEnsureDirectories_AlreadyExist(t *testing.T) {
	dir := t.TempDir()
	cfg := NewConfig(dir)

	err := cfg.EnsureDirectories()
	require.NoError(t, err)

	err = cfg.EnsureDirectories()
	assert.NoError(t, err)
}
