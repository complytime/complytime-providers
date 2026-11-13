// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/complytime/complytime-providers/internal/evidence"
)

func TestScan_EvaluationErrorsPreserveLaterSpecEvidence(t *testing.T) {
	for _, failure := range []string{"allocation", "tool", "parse"} {
		t.Run(failure, func(t *testing.T) {
			server, root := setupRetentionServer(t)
			request := retentionRequest(root)
			request.Targets[0].Variables["branches"] = "main"
			request.Targets[0].Variables["specs"] = "team/first.yaml,team/second.yaml"
			request.Targets[0].Variables["access_token"] = "test-access-token"
			runner := &mockScanRunner{snappyOutput: makeTestAttestation()}
			ScanRunner = runner
			allocations := 0
			var successfulDirectory string
			var evaluations []*evidence.Evaluation
			server.allocate = func(options evidence.StorageOptions, labels ...string) (*evidence.Evaluation, error) {
				allocations++
				runner.err = nil
				runner.ampelOutput = makeAmpelResultAttestation()
				if allocations == 1 {
					switch failure {
					case "allocation":
						return nil, syscall.ENOSPC
					case "tool":
						runner.err = fmt.Errorf("tool rejected test-access-token")
					case "parse":
						runner.ampelOutput = []byte("{malformed")
					}
				}
				evaluation, err := evidence.Allocate(options, labels...)
				if err == nil {
					evaluations = append(evaluations, evaluation)
					if allocations == 2 {
						successfulDirectory = evaluation.Directory
					}
				}
				return evaluation, err
			}
			response, err := server.Scan(context.Background(), request)
			require.NoError(t, err)
			require.Equal(t, 2, allocations)
			require.Len(t, response.Errors, 1)
			require.NotContains(t, response.Errors[0], "test-access-token")
			require.Len(t, response.Assessments, 2)
			require.Len(t, response.MappingReferences, 2)
			for _, assessment := range response.Assessments {
				require.Len(t, assessment.Evidence, 2)
				for _, item := range assessment.Evidence {
					path := filepath.Join(root, item.Source.Coordinate)
					require.Equal(t, successfulDirectory, filepath.Dir(path))
					digest, err := evidence.FileDigest(path)
					require.NoError(t, err)
					require.Equal(t, digest, item.Source.Digest)
				}
			}
			data, err := os.ReadFile(filepath.Join(successfulDirectory, evidence.MetadataFile))
			require.NoError(t, err)
			var metadata struct {
				Configuration evidence.ConfigurationContext `json:"configuration"`
			}
			require.NoError(t, json.Unmarshal(data, &metadata))
			require.Equal(t, "team/second.yaml", metadata.Configuration.SpecReference)
			require.ElementsMatch(t, []string{"BP-1.01", "BP-3.01"}, metadata.Configuration.RequirementIDs)
			for _, evaluation := range evaluations {
				path, err := evaluation.Prepare("after-scan.json")
				require.Error(t, err, "evaluation handles must close on success and operational failure")
				require.Empty(t, path)
				require.NoFileExists(t, evaluation.Path("after-scan.json"))
			}
			assertLegacyAttestations(t, root)
		})
	}
}

func TestScan_EvaluationVerificationDigestAndCloseFailuresStopPublication(t *testing.T) {
	for _, failure := range []string{"allocation-verification", "digest", "finalization", "close"} {
		t.Run(failure, func(t *testing.T) {
			server, root := setupRetentionServer(t)
			request := retentionRequest(root)
			ScanRunner = &mockScanRunner{
				snappyOutput: makeTestAttestation(), ampelOutput: makeAmpelResultAttestation(),
			}
			allocations := 0
			server.allocate = func(options evidence.StorageOptions, labels ...string) (*evidence.Evaluation, error) {
				allocations++
				if failure == "allocation-verification" {
					return nil, &evidence.VerificationError{Err: syscall.EACCES}
				}
				reads := 0
				options.Operations.Read = func(file *os.File) ([]byte, error) {
					if file.Name() == evidence.SnappyFile {
						reads++
					}
					if failure == "digest" && reads == 2 {
						return nil, io.ErrUnexpectedEOF
					}
					return io.ReadAll(file)
				}
				options.Operations.Chmod = func(file *os.File, mode os.FileMode) error {
					if failure == "finalization" && file.Name() == evidence.AmpelFile {
						return syscall.EPERM
					}
					return file.Chmod(mode)
				}
				evaluation, err := evidence.Allocate(options, labels...)
				if err == nil && failure == "close" {
					require.NoError(t, evaluation.Close())
				}
				return evaluation, err
			}
			response, err := server.Scan(context.Background(), request)
			require.Error(t, err)
			require.Nil(t, response)
			require.Equal(t, 1, allocations, "critical failures must stop before the next spec/branch")
			switch failure {
			case "allocation-verification":
				require.ErrorIs(t, err, syscall.EACCES)
			case "digest":
				require.ErrorIs(t, err, io.ErrUnexpectedEOF)
			case "finalization":
				require.ErrorIs(t, err, syscall.EPERM)
			case "close":
				require.ErrorIs(t, err, os.ErrClosed)
			}
			assertLegacyAttestations(t, root)
		})
	}
}
