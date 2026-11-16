// SPDX-License-Identifier: Apache-2.0

package evidence

import (
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublicContext_MalformedSecretsNeverPersist(t *testing.T) {
	for _, value := range []string{
		"https://alice:123 456@example.com/repo",
		"https://alice:secret\tpassword@example.com/repo",
		"https://example.com/repo?token=first second",
		"label https://example.com/repo?token=first\nsecond",
	} {
		t.Run(value, func(t *testing.T) {
			options := storageOptions(t)
			evaluation := allocateTest(t, options, value)
			require.NoError(t, evaluation.Write(ConftestFile, []byte(value)))
			require.NoError(t, evaluation.WriteMetadata("opa", TargetContext{
				URL: value, TargetID: value, Branch: value,
			}, ConfigurationContext{SpecReference: value}))
			require.NoError(t, evaluation.Finalize())
			data, err := os.ReadFile(evaluation.Path(MetadataFile))
			require.NoError(t, err)
			var metadata struct {
				Target        TargetContext        `json:"target"`
				Configuration ConfigurationContext `json:"configuration"`
			}
			require.NoError(t, json.Unmarshal(data, &metadata))
			assert.Empty(t, metadata.Target.TargetID)
			assert.Empty(t, metadata.Target.URL)
			assert.Empty(t, metadata.Target.Branch)
			assert.Empty(t, metadata.Configuration.SpecReference)
			assert.Equal(t, "unknown", filepath.Base(filepath.Dir(evaluation.Directory)))
			raw, err := os.ReadFile(evaluation.Path(ConftestFile))
			require.NoError(t, err)
			assert.Equal(t, value, string(raw), "raw evidence must remain byte-for-byte unchanged")
		})
	}
	assert.Equal(t, "ordinary context", PublicContext("ordinary context"))
	assert.Equal(t, "https://example.com/repo",
		PublicContext("https://alice:password@example.com/repo?token=secret#fragment"))
}

func TestDisplayContext_SafeValuesComposeIntoReadableReferences(t *testing.T) {
	locator := url.URL{Scheme: "https", Host: "example.com", Path: "/org/repo",
		User: url.UserPassword("alice", "password"), RawQuery: "token=secret", Fragment: "fragment"}
	value := locator.String()
	assert.Equal(t, "url:example.com/org/repo", DisplayContext(value))
	assert.Equal(t, "host:url:example.com/org/repo", DisplayContext("host:"+value))
	assert.Equal(t, "builtin:github/check.yaml", DisplayContext("builtin:github/check.yaml"))
	assert.Empty(t, DisplayContext("https://alice:123 456@example.com"))
	evaluation := allocateTest(t, storageOptions(t))
	require.NoError(t, evaluation.Write(ConftestFile, []byte("[]")))
	_, err := evaluation.Read(ConftestFile)
	require.NoError(t, err)
	title := "Evaluation of " + DisplayContext(value)
	_, reference, err := evaluation.Reference(ConftestFile, "result_", "reference_", TypeConftestResult,
		title, "Target: "+DisplayContext(value), "host")
	require.NoError(t, err)
	assert.Equal(t, title, reference.Title)
	assert.Equal(t, "Target: url:example.com/org/repo", reference.Description)
}

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
