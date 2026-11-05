// SPDX-License-Identifier: Apache-2.0

package server

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindConftestConfig_Discovery(t *testing.T) {
	for _, name := range []string{
		"conftest.json", "conftest.toml", "conftest.yaml", "conftest.yml",
		"conftest.properties", "conftest.props", "conftest.prop", "conftest.hcl",
		"conftest.tfvars", "conftest.dotenv", "conftest.env", "conftest.ini",
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, name)
			require.NoError(t, os.WriteFile(path, nil, 0600))
			got, err := findConftestConfig(dir, "", os.Stat)
			require.NoError(t, err)
			assert.Equal(t, path, got)
		})
	}
	t.Run("unsupported extension ignored", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "conftest.backup"), nil, 0600))
		got, err := findConftestConfig(dir, "", os.Stat)
		require.NoError(t, err)
		assert.Empty(t, got)
	})
	t.Run("discovery order", func(t *testing.T) {
		dir := t.TempDir()
		for _, name := range []string{"conftest.json", "conftest.toml"} {
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), nil, 0600))
		}
		got, err := findConftestConfig(dir, "", os.Stat)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(dir, "conftest.json"), got)
	})
	t.Run("named directory is not a config", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(dir, "conftest.json"), 0750))
		got, err := findConftestConfig(dir, "", os.Stat)
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestFindConftestConfig_ExplicitPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.toml")
	require.NoError(t, os.WriteFile(path, nil, 0600))
	for _, input := range []string{"custom.toml", path} {
		t.Run(input, func(t *testing.T) {
			got, err := findConftestConfig(dir, input, os.Stat)
			require.NoError(t, err)
			assert.Equal(t, path, got)
		})
	}
	_, err := findConftestConfig(dir, "missing.toml", os.Stat)
	require.Error(t, err)
	assert.Contains(t, err.Error(), filepath.Join(dir, "missing.toml"))
	_, err = findConftestConfig(dir, ".", os.Stat)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "directory")
}

func TestFindConftestConfig_StatFailure(t *testing.T) {
	dir := t.TempDir()
	wantErr := errors.New("injected stat failure")
	_, err := findConftestConfig(dir, "", func(string) (os.FileInfo, error) {
		return nil, wantErr
	})
	require.ErrorIs(t, err, wantErr)
	assert.Contains(t, err.Error(), filepath.Join(dir, "conftest.json"))
}
