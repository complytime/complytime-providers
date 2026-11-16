// SPDX-License-Identifier: Apache-2.0

package evidence

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluation_RejectsHardLinksWithoutExternalSideEffects(t *testing.T) {
	for _, operation := range []string{"read", "finalize-supporting", "finalize-after-read"} {
		t.Run(operation, func(t *testing.T) {
			evaluation := allocateTest(t, storageOptions(t))
			external := filepath.Join(t.TempDir(), "external.json")
			data := []byte("external bytes must not change")
			if operation == "finalize-after-read" {
				require.NoError(t, evaluation.Write(ConftestFile, data))
				_, err := evaluation.Read(ConftestFile)
				require.NoError(t, err)
				require.NoError(t, os.Link(evaluation.Path(ConftestFile), external))
			} else {
				require.NoError(t, os.WriteFile(external, data, FileMode))
				require.NoError(t, os.Link(external, evaluation.Path(ConftestFile)))
			}
			require.NoError(t, os.Chmod(external, 0o640))
			reads := 0
			read := evaluation.operations.Read
			evaluation.operations.Read = func(file *os.File) ([]byte, error) {
				reads++
				return read(file)
			}
			if operation == "read" {
				artifact, err := evaluation.Read(ConftestFile)
				require.ErrorContains(t, err, "hard link")
				assert.Nil(t, artifact)
			} else {
				require.ErrorContains(t, evaluation.Finalize(), "hard link")
			}
			assert.Zero(t, reads, "reject before reading even previously validated outputs")
			stored, err := os.ReadFile(external)
			require.NoError(t, err)
			assert.Equal(t, data, stored)
			info, err := os.Stat(external)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
		})
	}
	// An ordinary single-link artifact remains valid.
	evaluation := allocateTest(t, storageOptions(t))
	require.NoError(t, evaluation.Write(ConftestFile, []byte("[]")))
	artifact, err := evaluation.Read(ConftestFile)
	require.NoError(t, err)
	assert.Equal(t, "[]", string(artifact.Data))
	require.NoError(t, evaluation.Finalize())
}

func TestEvaluation_RelocationBetweenAllocationAndWrite(t *testing.T) {
	for _, operation := range []string{"write", "prepare-and-tool"} {
		t.Run(operation, func(t *testing.T) {
			options := storageOptions(t)
			evaluation := allocateTest(t, options, "context")
			relocated := filepath.Join(t.TempDir(), "moved-results")
			start, done := make(chan struct{}), make(chan error, 1)
			go func() {
				<-start
				done <- os.Rename(options.ResultsRoot, relocated)
			}()
			close(start)
			require.NoError(t, <-done)
			relative, err := filepath.Rel(options.ResultsRoot, evaluation.Directory)
			require.NoError(t, err)
			movedFile := filepath.Join(relocated, relative, ConftestFile)
			if operation == "write" {
				require.Error(t, evaluation.Write(ConftestFile, []byte("held-descriptor bytes")))
			} else {
				path, err := evaluation.Prepare(ConftestFile)
				require.Error(t, err)
				assert.Empty(t, path)
			}
			assert.NoFileExists(t, movedFile, "already-relocated allocations must not create output")
			assert.NoFileExists(t, evaluation.Path(ConftestFile))
			require.Error(t, evaluation.Finalize(), "relocation must suppress publication")
		})
	}
}

func TestEvaluation_RelocationDuringWriteDetectsButCannotUndoBytes(t *testing.T) {
	options := storageOptions(t)
	relocated := filepath.Join(t.TempDir(), "moved-results")
	start, done := make(chan struct{}), make(chan error, 1)
	go func() {
		<-start
		done <- os.Rename(options.ResultsRoot, relocated)
	}()
	options.Operations.Write = func(file *os.File, data []byte) (int, error) {
		close(start)
		if err := <-done; err != nil {
			return 0, err
		}
		return file.Write(data)
	}
	evaluation := allocateTest(t, options, "context")
	require.Error(t, evaluation.Write(ConftestFile, []byte("relocated bytes")))
	relative, err := filepath.Rel(options.ResultsRoot, evaluation.Directory)
	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join(relocated, relative, ConftestFile))
	require.NoError(t, err)
	assert.Equal(t, "relocated bytes", string(data), "locator checks cannot atomically prevent ancestor renames")
	assert.NoFileExists(t, evaluation.Path(ConftestFile))
	require.Error(t, evaluation.Finalize())
}

func TestEvaluation_ToolPathReopenAfterRelocationUsesReplacementTree(t *testing.T) {
	options := storageOptions(t)
	evaluation := allocateTest(t, options, "context")
	path, err := evaluation.Prepare(ConftestFile)
	require.NoError(t, err)
	relocated := filepath.Join(t.TempDir(), "moved-results")
	require.NoError(t, os.Rename(options.ResultsRoot, relocated))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), DirectoryMode))
	require.NoError(t, os.WriteFile(path, []byte("tool bytes"), FileMode))
	relative, err := filepath.Rel(options.ResultsRoot, evaluation.Directory)
	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join(relocated, relative, ConftestFile))
	require.NoError(t, err)
	assert.Empty(t, data, "tool pathname writes do not use the original held descriptor")
	data, err = os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "tool bytes", string(data))
	require.Error(t, evaluation.Finalize())
}
