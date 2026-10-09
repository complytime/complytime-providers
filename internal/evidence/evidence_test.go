// SPDX-License-Identifier: Apache-2.0

package evidence

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileDigest_KnownContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := []byte("hello evidence")
	require.NoError(t, os.WriteFile(path, content, 0o600))

	digest, err := FileDigest(path)
	require.NoError(t, err)

	expected := fmt.Sprintf("sha256:%x", sha256.Sum256(content))
	assert.Equal(t, expected, digest)
	assert.Regexp(t, `^sha256:[a-f0-9]{64}$`, digest)
}

func TestFileDigest_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	require.NoError(t, os.WriteFile(path, []byte{}, 0o600))

	digest, err := FileDigest(path)
	require.NoError(t, err)

	expected := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte{}))
	assert.Equal(t, expected, digest)
}

func TestFileDigest_NonExistentPath(t *testing.T) {
	_, err := FileDigest("/nonexistent/path/file.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "evidence digest")
}

func TestFileDigest_DirectoryPath(t *testing.T) {
	dir := t.TempDir()
	_, err := FileDigest(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is a directory")
}

func TestHostRemark_BothAvailable(t *testing.T) {
	dir := t.TempDir()
	idFile := filepath.Join(dir, "machine-id")
	require.NoError(t, os.WriteFile(idFile, []byte("abc123def456\n"), 0o600))

	original := MachineIDPath
	MachineIDPath = idFile
	defer func() { MachineIDPath = original }()

	remark := HostRemark()
	assert.Contains(t, remark, "Collected on host")
	assert.Contains(t, remark, "(machine-id: abc123def456)")
	assert.NotContains(t, remark, "\n")
}

func TestHostRemark_NoMachineID(t *testing.T) {
	original := MachineIDPath
	MachineIDPath = "/nonexistent/machine-id"
	defer func() { MachineIDPath = original }()

	remark := HostRemark()
	assert.Contains(t, remark, "Collected on host")
	assert.NotContains(t, remark, "machine-id")
}

func TestHostRemark_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	idFile := filepath.Join(dir, "custom-machine-id")
	require.NoError(t, os.WriteFile(idFile, []byte("custom789\n"), 0o600))

	// Set MachineIDPath to nonexistent so only the env var works
	original := MachineIDPath
	MachineIDPath = "/nonexistent/machine-id"
	defer func() { MachineIDPath = original }()

	t.Setenv(EnvMachineIDFile, idFile)

	remark := HostRemark()
	assert.Contains(t, remark, "(machine-id: custom789)")
}

func TestHostRemark_MachineIDTrimmed(t *testing.T) {
	dir := t.TempDir()
	idFile := filepath.Join(dir, "machine-id")
	require.NoError(t, os.WriteFile(
		idFile, []byte("  abc123  \n\n"), 0o600,
	))

	original := MachineIDPath
	MachineIDPath = idFile
	defer func() { MachineIDPath = original }()

	remark := HostRemark()
	assert.Contains(t, remark, "(machine-id: abc123)")
	assert.NotContains(t, remark, " abc123 ")
}
