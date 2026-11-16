// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complytime-providers/cmd/ampel-provider/results"
	"github.com/complytime/complytime-providers/cmd/ampel-provider/scan"
	"github.com/complytime/complytime-providers/internal/evidence"
)

func TestScan_EmptyResultSetErrorCannotPublishSyntheticPass(t *testing.T) {
	for _, configured := range []bool{false, true} {
		t.Run(fmt.Sprintf("configured-requirements-%t", configured), func(t *testing.T) {
			server, root := setupRetentionServer(t)
			request := retentionRequest(root)
			request.Targets[0].Variables["branches"] = "main"
			request.Targets[0].Variables["specs"] = "builtin:github/branch-rules.yaml"
			ScanRunner = &mockScanRunner{snappyOutput: makeTestAttestation(),
				ampelOutput: []byte(`{"predicate":{"error":{}}}`)}
			var response *provider.ScanResponse
			if configured {
				var err error
				response, err = server.Scan(context.Background(), request)
				require.NoError(t, err)
			} else {
				// Exercise the per-evaluation adapter with no requirement IDs,
				// without a missing-index error masking the malformed tool output.
				policy := filepath.Join(t.TempDir(), "policy.json")
				require.NoError(t, os.WriteFile(policy, []byte("{}"), evidence.FileMode))
				identity, err := evidence.ExecutionIdentity()
				require.NoError(t, err)
				result, err := server.scanEvaluation(evaluationOptions{
					repository: scan.RepoTarget{URL: "https://github.com/myorg/repo1"},
					scanConfig: scan.ScanConfig{PolicyPath: policy}, runner: ScanRunner,
					storage: evidence.StorageOptions{WorkspaceRoot: root,
						ResultsRoot: filepath.Join(root, "results"), Identity: identity},
				}, "main", "builtin:github/branch-rules.yaml")
				require.NoError(t, err)
				require.Equal(t, "error", result.Status)
				response = results.ToScanResponse([]*results.PerRepoResult{result}, nil)
			}
			require.NotEmpty(t, response.Errors)
			require.Contains(t, response.Errors[0], "no result-set status or error")
			require.Empty(t, response.Assessments)
			require.Empty(t, response.MappingReferences)
		})
	}
}

func TestFinalizeEvaluation_PreservesReadableSanitizedReferences(t *testing.T) {
	root := t.TempDir()
	identity, err := evidence.ExecutionIdentity()
	require.NoError(t, err)
	evaluation, err := evidence.Allocate(evidence.StorageOptions{WorkspaceRoot: root,
		ResultsRoot: filepath.Join(root, "results"), Identity: identity})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, evaluation.Close()) })
	for _, name := range []string{evidence.AmpelFile, evidence.SnappyFile} {
		require.NoError(t, evaluation.Write(name, []byte("{}")))
	}
	locator := url.URL{Scheme: "https", Host: "example.com", Path: "/org/repo",
		User: url.UserPassword("alice", "password"), RawQuery: "token=secret"}
	parsed := &results.PerRepoResult{
		Repository: locator.String(), Branch: "main",
		SpecLabel: "https://spec.example.com/check.yaml?token=secret",
	}
	require.NoError(t, finalizeEvaluation(evaluation, parsed, "host"))
	require.Len(t, parsed.References, 2)
	for _, reference := range parsed.References {
		require.Contains(t, reference.Title, "for url:example.com/org/repo@main")
		require.Equal(t, "Retained in-toto attestation; collection spec: url:spec.example.com/check.yaml", reference.Description)
		for _, secret := range []string{"alice", "password", "secret"} {
			require.NotContains(t, reference.Title+reference.Description, secret)
		}
	}
}

func TestScan_ExecutionVerificationFailureStopsPublication(t *testing.T) {
	for _, phase := range []string{"write", "prepare"} {
		t.Run(phase, func(t *testing.T) {
			server, root := setupRetentionServer(t)
			ScanRunner = &mockScanRunner{snappyOutput: makeTestAttestation(),
				ampelOutput: makeAmpelResultAttestation()}
			moved := filepath.Join(t.TempDir(), "moved-results")
			allocations := 0
			server.allocate = func(options evidence.StorageOptions, labels ...string) (*evidence.Evaluation, error) {
				allocations++
				if phase == "write" {
					options.Operations.Write = func(file *os.File, data []byte) (int, error) {
						written, err := file.Write(data)
						if err != nil {
							return written, err
						}
						return written, os.Rename(options.ResultsRoot, moved)
					}
				} else {
					options.Operations.Read = func(file *os.File) ([]byte, error) {
						data, err := io.ReadAll(file)
						if err != nil {
							return data, err
						}
						return data, os.Rename(options.ResultsRoot, moved)
					}
				}
				return evidence.Allocate(options, labels...)
			}
			response, err := server.Scan(context.Background(), retentionRequest(root))
			var verification *evidence.VerificationError
			require.ErrorAs(t, err, &verification)
			require.Nil(t, response, "verification failures must suppress the whole RPC response")
			require.Equal(t, 1, allocations, "do not continue to later branches/specs")
			files, err := filepath.Glob(filepath.Join(moved, "*", "*", "*", "evaluation-*", evidence.SnappyFile))
			require.NoError(t, err)
			require.Len(t, files, 1)
			data, err := os.ReadFile(files[0])
			require.NoError(t, err)
			require.Equal(t, makeTestAttestation(), data)
			require.NoFileExists(t, filepath.Join(filepath.Dir(files[0]), evidence.AmpelFile))
		})
	}
}

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
