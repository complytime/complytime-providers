// SPDX-License-Identifier: Apache-2.0

package generate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteReadScanConfig_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	ids := []string{"BP-1.01", "BP-2.03"}

	err := WriteScanConfig(dir, ids)
	require.NoError(t, err)

	cfg, err := ReadScanConfig(dir)
	require.NoError(t, err)
	require.Equal(t, ids, cfg.RequirementIDs)
	require.NotEmpty(t, cfg.GeneratedAt)
}

func TestWriteReadScanConfig_EmptyIDs(t *testing.T) {
	dir := t.TempDir()

	err := WriteScanConfig(dir, []string{})
	require.NoError(t, err)

	cfg, err := ReadScanConfig(dir)
	require.NoError(t, err)
	require.Empty(t, cfg.RequirementIDs)
}

func TestReadScanConfig_MissingFile(t *testing.T) {
	dir := t.TempDir()

	_, err := ReadScanConfig(dir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "reading scan config")
}

func TestReadScanConfig_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ScanConfigFileName)
	require.NoError(t, os.WriteFile(path, []byte("{invalid"), 0600))

	_, err := ReadScanConfig(dir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parsing scan config")
}

func TestWriteReadScanConfig_NilIDs(t *testing.T) {
	dir := t.TempDir()

	err := WriteScanConfig(dir, nil)
	require.NoError(t, err)

	cfg, err := ReadScanConfig(dir)
	require.NoError(t, err)
	require.Nil(t, cfg.RequirementIDs)
}

func TestWriteScanConfig_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")

	err := WriteScanConfig(dir, []string{"BP-1.01"})
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(dir, ScanConfigFileName))
	require.NoError(t, err)
}
