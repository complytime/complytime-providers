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
