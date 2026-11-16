// SPDX-License-Identifier: Apache-2.0

package evidence

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplaceFile_ReplacesLinksWithoutTouchingReferents(t *testing.T) {
	for _, kind := range []string{"absent", "regular", "symlink", "hardlink"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			require.NoError(t, EnsureDirectories(root, "policy"))
			external := filepath.Join(t.TempDir(), "external.xml")
			require.NoError(t, os.WriteFile(external, []byte("external"), FileMode))
			require.NoError(t, os.Chmod(external, 0o640))
			path := filepath.Join(root, "policy", "tailoring.xml")
			switch kind {
			case "regular":
				require.NoError(t, os.WriteFile(path, []byte("old"), FileMode))
			case "symlink":
				require.NoError(t, os.Symlink(external, path))
			case "hardlink":
				require.NoError(t, os.Link(external, path))
			}
			for _, data := range []string{"first generation", "legitimate rerun"} {
				require.NoError(t, ReplaceFile(root, filepath.Join("policy", "tailoring.xml"), []byte(data)))
				stored, err := os.ReadFile(path)
				require.NoError(t, err)
				assert.Equal(t, data, string(stored))
			}
			stored, err := os.ReadFile(external)
			require.NoError(t, err)
			assert.Equal(t, "external", string(stored))
			info, err := os.Stat(external)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
			entries, err := os.ReadDir(filepath.Dir(path))
			require.NoError(t, err)
			require.Len(t, entries, 1)
			assert.Equal(t, "tailoring.xml", entries[0].Name())
		})
	}
}

func TestReplaceFile_RejectsUnsafeParentsAndCleansFailedReplacement(t *testing.T) {
	root, external := t.TempDir(), t.TempDir()
	require.NoError(t, os.Symlink(external, filepath.Join(root, "alias")))
	for _, path := range []string{"../escape", "/absolute", "alias/tailoring.xml", "missing/file"} {
		require.Error(t, ReplaceFile(root, path, []byte("unsafe")))
	}
	entries, err := os.ReadDir(external)
	require.NoError(t, err)
	assert.Empty(t, entries)
	require.NoError(t, os.Mkdir(filepath.Join(root, "directory"), DirectoryMode))
	require.Error(t, ReplaceFile(root, "directory", []byte("not a directory")))
	entries, err = os.ReadDir(root)
	require.NoError(t, err)
	require.Len(t, entries, 2, "failed replacement must remove its temporary file")
}
