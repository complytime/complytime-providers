// SPDX-License-Identifier: Apache-2.0

package evidence

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func trackArtifactHandles(evaluation *Evaluation) *[]*os.File {
	var files []*os.File
	evaluation.operations.Stat = func(file *os.File) (os.FileInfo, error) {
		info, err := file.Stat()
		if err == nil && info.Mode().IsRegular() {
			files = append(files, file)
		}
		return info, err
	}
	return &files
}

func liveArtifactHandles(files []*os.File) []*os.File {
	var live []*os.File
	seen := make(map[*os.File]bool)
	for _, file := range files {
		if _, err := file.Stat(); err == nil && !seen[file] {
			live = append(live, file)
			seen[file] = true
		}
	}
	return live
}

func TestEvaluation_ReadPinsIdentityUntilClose(t *testing.T) {
	evaluation, err := Allocate(storageOptions(t), "target")
	require.NoError(t, err)
	closed := false
	t.Cleanup(func() {
		if !closed {
			require.NoError(t, evaluation.Close())
		}
	})
	require.NoError(t, evaluation.Write(ConftestFile, []byte("original")))
	files := trackArtifactHandles(evaluation)
	original, err := evaluation.Read(ConftestFile)
	require.NoError(t, err)
	held := liveArtifactHandles(*files)
	require.Len(t, held, 1, "validation must pin the original inode until evaluation close")
	for range 3 {
		repeated, err := evaluation.Read(ConftestFile)
		require.NoError(t, err)
		assert.Equal(t, original.Data, repeated.Data)
		assert.Equal(t, original.Digest, repeated.Digest)
		assert.Equal(t, held, liveArtifactHandles(*files), "revalidation must not leak handles")
	}
	require.NoError(t, evaluation.Finalize())
	assert.Equal(t, held, liveArtifactHandles(*files), "finalization must keep the identity pinned")
	require.NoError(t, os.Remove(evaluation.Path(ConftestFile)))
	require.NoError(t, os.WriteFile(evaluation.Path(ConftestFile), original.Data, FileMode))
	replacement, err := os.Stat(evaluation.Path(ConftestFile))
	require.NoError(t, err)
	assert.False(t, os.SameFile(original.info, replacement), "a live inode cannot be reused")
	_, err = evaluation.Read(ConftestFile)
	require.ErrorContains(t, err, "changed after validation")
	assert.Equal(t, held, liveArtifactHandles(*files), "rejected reads must keep the original pin")
	var verification *VerificationError
	require.ErrorAs(t, evaluation.Finalize(), &verification)
	require.NoError(t, evaluation.Close())
	closed = true
	assert.Empty(t, liveArtifactHandles(*files))
	_, err = evaluation.dir.Stat()
	assert.ErrorIs(t, err, os.ErrClosed)
	stored, err := os.ReadFile(evaluation.Path(ConftestFile))
	require.NoError(t, err)
	assert.Equal(t, original.Data, stored, "cleanup must never remove retained output")
}

func TestEvaluation_ReadFailuresReleaseIdentityHandles(t *testing.T) {
	for _, failure := range []string{"read", "pin-open", "pin-stat", "pin-replaced", "close"} {
		t.Run(failure, func(t *testing.T) {
			evaluation := allocateTest(t, storageOptions(t))
			require.NoError(t, evaluation.Write(ConftestFile, []byte("original")))
			files := trackArtifactHandles(evaluation)
			injected := errors.New("injected read lifecycle failure")
			switch failure {
			case "read":
				evaluation.operations.Read = func(*os.File) ([]byte, error) { return nil, injected }
			case "pin-open", "pin-replaced":
				evaluation.operations.Read = func(file *os.File) ([]byte, error) {
					data, err := io.ReadAll(file)
					if err != nil {
						return nil, err
					}
					if failure == "pin-open" {
						return data, os.Remove(evaluation.Path(ConftestFile))
					}
					replacement := filepath.Join(filepath.Dir(evaluation.Directory), "replacement")
					if err := os.WriteFile(replacement, data, FileMode); err != nil {
						return nil, err
					}
					return data, os.Rename(replacement, evaluation.Path(ConftestFile))
				}
			case "pin-stat":
				stat := evaluation.operations.Stat
				calls := 0
				evaluation.operations.Stat = func(file *os.File) (os.FileInfo, error) {
					info, err := stat(file)
					calls++
					if calls == 2 {
						return nil, injected
					}
					return info, err
				}
			case "close":
				evaluation.operations.Close = func(file *os.File) error {
					return errors.Join(injected, file.Close())
				}
			}
			artifact, err := evaluation.Read(ConftestFile)
			require.Error(t, err)
			assert.Nil(t, artifact)
			assert.Empty(t, evaluation.validated)
			assert.Empty(t, liveArtifactHandles(*files), "failed reads must release every opened handle")
			if failure == "read" || failure == "pin-stat" || failure == "close" {
				assert.ErrorIs(t, err, injected)
			}
		})
	}
}

func TestEvaluation_CloseReleasesAllHandlesAfterFailure(t *testing.T) {
	evaluation, err := Allocate(storageOptions(t))
	require.NoError(t, err)
	closed := false
	t.Cleanup(func() {
		if !closed {
			require.NoError(t, evaluation.Close())
		}
	})
	files := trackArtifactHandles(evaluation)
	for _, name := range []string{ConftestFile, MetadataFile} {
		require.NoError(t, evaluation.Write(name, []byte("{}")))
		_, err := evaluation.Read(name)
		require.NoError(t, err)
	}
	held := liveArtifactHandles(*files)
	require.Len(t, held, 2)
	require.NoError(t, held[0].Close())
	err = evaluation.Close()
	closed = true
	assert.ErrorIs(t, err, os.ErrClosed)
	assert.Empty(t, liveArtifactHandles(*files), "one close failure must not leak other pins")
	_, err = evaluation.dir.Stat()
	assert.ErrorIs(t, err, os.ErrClosed)
}
