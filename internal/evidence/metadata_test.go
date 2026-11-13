// SPDX-License-Identifier: Apache-2.0

package evidence

import (
	"errors"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type warningRecorder struct{ entries [][]any }

func (logger *warningRecorder) Warn(message string, args ...any) {
	logger.entries = append(logger.entries, append([]any{message}, args...))
}

func TestRecordMetadata_WriteAndCloseFailuresWarnWithoutBlockingEvidence(t *testing.T) {
	for _, failure := range []string{"write", "short-write", "close"} {
		t.Run(failure, func(t *testing.T) {
			evaluation := allocateTest(t, storageOptions(t))
			require.NoError(t, evaluation.Write(ConftestFile, []byte("[]\n")))
			artifact, err := evaluation.Read(ConftestFile)
			require.NoError(t, err)
			injected := errors.New("injected metadata storage failure")
			switch failure {
			case "write":
				evaluation.operations.Write = func(file *os.File, _ []byte) (int, error) {
					n, err := file.Write([]byte("partial"))
					return n, errors.Join(injected, err)
				}
			case "short-write":
				evaluation.operations.Write = func(_ *os.File, _ []byte) (int, error) { return 0, nil }
			case "close":
				evaluation.operations.Close = func(file *os.File) error { return errors.Join(injected, file.Close()) }
			}
			logger := &warningRecorder{}
			evaluation.RecordMetadata(logger, "opa", TargetContext{TargetKey: "host:system"}, ConfigurationContext{})
			require.Len(t, logger.entries, 1)
			assert.Contains(t, logger.entries[0], evaluation.Token)
			require.NoError(t, evaluation.Finalize())
			stored, err := os.ReadFile(artifact.Path)
			require.NoError(t, err)
			assert.Equal(t, artifact.Data, stored)
			info, err := os.Stat(evaluation.Path(MetadataFile))
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(FileMode), info.Mode().Perm())
		})
	}
}

func TestEvaluation_RequiredArtifactWriteCloseFailures(t *testing.T) {
	evaluation := allocateTest(t, storageOptions(t))
	evaluation.operations.Write = func(_ *os.File, _ []byte) (int, error) { return 0, io.ErrShortWrite }
	writeErr := evaluation.Write(ConftestFile, []byte("[]"))
	require.ErrorIs(t, writeErr, io.ErrShortWrite)
	// Fresh evaluations, not retries into partially written artifacts, are required.
	require.Error(t, evaluation.Write(ConftestFile, []byte("[]")))
	other := allocateTest(t, storageOptions(t))
	other.operations.Close = func(file *os.File) error { return errors.Join(io.ErrClosedPipe, file.Close()) }
	require.ErrorIs(t, other.Write(ConftestFile, []byte("[]")), io.ErrClosedPipe)
}
