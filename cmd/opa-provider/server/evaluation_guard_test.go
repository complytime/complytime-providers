// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/complytime/complytime-providers/cmd/opa-provider/scan"
)

func TestScan_VersionMetadataDoesNotGateLocalOrRemoteEvaluation(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, remote := range []bool{false, true} {
		for _, test := range []struct {
			name, output string
			err          error
		}{
			{"other version", "Conftest: 0.69.0\n", nil},
			{"source build", "Conftest: dev\n", nil},
			{"unknown version", "unknown\n", nil},
			{"unavailable version", "failure output", errors.New("version failed")},
		} {
			t.Run(test.name+map[bool]string{true: " remote", false: " local"}[remote], func(t *testing.T) {
				workspace := t.TempDir()
				writeScanConfig(t, workspace)
				input := t.TempDir()
				file := filepath.Join(input, "input.json")
				require.NoError(t, os.WriteFile(file, []byte("{}"), 0600))
				runner := &mockRunner{callFn: func(name string, args []string) ([]byte, error) {
					if len(args) == 1 && args[0] == "--version" {
						return []byte(test.output), test.err
					}
					require.Equal(t, "conftest", name)
					require.Equal(t, "test", args[0])
					return []byte(conftestAllSuccessJSON), nil
				}}
				srv := newWorkspaceTestServer(t, ServerOptions{
					Runner: runner, WorkspaceDir: workspace,
					Loader:      &mockLoader{loadFn: func(provider.Target, string) (string, error) { return file, nil }},
					ToolChecker: func() ([]string, error) { return nil, nil },
				})
				target := localTarget(t, file, "")
				if remote {
					target = remoteTarget("", "https://example.com/org/repo")
				}
				response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{target}))
				require.NoError(t, err)
				assert.Empty(t, response.Errors)
				require.Len(t, response.Assessments, 1)
				assert.Equal(t, "test-req", response.Assessments[0].RequirementID)
				assert.Equal(t, provider.ResultPassed, response.Assessments[0].Steps[0].Result)
				require.Len(t, runner.calls, 1)
				assert.Equal(t, "test", runner.calls[0].args[0])
			})
		}
	}
}

func TestScan_SeparatedConftestStreams(t *testing.T) {
	for _, scenario := range []struct {
		name, stdout, stderr string
		executionError       bool
		wantError            string
	}{
		{"valid stdout with warnings", conftestAllSuccessJSON, "diagnostic warning", false, ""},
		{"noncompliant stdout", conftestHappyJSON, "diagnostic warning", false, ""},
		{"invalid stdout with valid JSON stderr", "not json", conftestAllSuccessJSON, false, "parsing conftest output"},
		{"empty directory on stderr", "", conftestNoFilesFound + "\n", true, "no eligible input"},
		{"sentinel on stdout is not a diagnostic", conftestNoFilesFound, "rego parse error", true, "evaluating policies"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			workspace := t.TempDir()
			writeScanConfig(t, workspace)
			input := t.TempDir()
			runner := &mockRunner{outputFn: func(name string, args []string) (scan.CommandOutput, error) {
				assert.Equal(t, "conftest", name)
				assert.Contains(t, args, "--no-fail")
				var err error
				if scenario.executionError {
					err = errors.New("tool failed")
				}
				return scan.CommandOutput{Stdout: []byte(scenario.stdout), Stderr: []byte(scenario.stderr)}, err
			}}
			srv := newWorkspaceTestServer(t, ServerOptions{Runner: runner, WorkspaceDir: workspace,
				ToolChecker: func() ([]string, error) { return nil, nil }})
			target := localTarget(t, input, "")
			response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{target}))
			require.NoError(t, err)
			if scenario.wantError != "" {
				require.Len(t, response.Errors, 1)
				assert.Contains(t, response.Errors[0], scenario.wantError)
				assert.Empty(t, response.Assessments)
				assert.Empty(t, response.MappingReferences)
				if scenario.name == "sentinel on stdout is not a diagnostic" {
					assert.NotContains(t, response.Errors[0], "no eligible input")
				}
				return
			}
			assert.Empty(t, response.Errors)
			require.NotEmpty(t, response.Assessments)
			if scenario.name == "noncompliant stdout" {
				assert.Equal(t, provider.ResultFailed, response.Assessments[0].Steps[0].Result)
			}
		})
	}
}

func TestScan_DirectoryRequiresEvidenceOfEvaluation(t *testing.T) {
	for _, test := range []struct {
		name, output string
		valid        bool
		remote       bool
	}{
		{"empty array", `[]`, false, false},
		{"remote empty array", `[]`, false, true},
		{"null", `null`, false, false},
		{"empty object", `[{}]`, false, false},
		{"missing namespace", `[{"filename":"config.json","successes":3}]`, false, false},
		{"missing filename", `[{"namespace":"main","successes":3}]`, false, false},
		{"whitespace filename", `[{"filename":" ","namespace":"main","successes":3}]`, false, false},
		{"evaluated with no findings", `[{"filename":"config.json","namespace":"main","successes":0}]`, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			workspace := t.TempDir()
			writeScanConfig(t, workspace)
			input := t.TempDir()
			runner := conftestRunner(test.output)
			opts := ServerOptions{
				Runner: runner, WorkspaceDir: workspace,
				ToolChecker: func() ([]string, error) { return nil, nil },
			}
			target := localTarget(t, input, "")
			if test.remote {
				opts.Loader = &mockLoader{loadFn: func(provider.Target, string) (string, error) { return input, nil }}
				target = remoteTarget("", "https://example.com/org/repo")
			}
			srv := newWorkspaceTestServer(t, opts)
			response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{target}))
			require.NoError(t, err)
			if test.valid {
				assert.Empty(t, response.Errors)
				require.Len(t, response.Assessments, 1)
				assert.Equal(t, "test-req", response.Assessments[0].RequirementID)
				assert.Equal(t, provider.ResultPassed, response.Assessments[0].Steps[0].Result)
			} else {
				require.Len(t, response.Errors, 1)
				assert.Contains(t, response.Errors[0], target.TargetID)
				assert.Contains(t, response.Errors[0], "no eligible input")
				assert.Contains(t, response.Errors[0], "adjust ignore settings")
				assert.Empty(t, response.Assessments)
			}
			require.Len(t, runner.calls, 1)
		})
	}
}

func TestScan_RejectedWorkspaceTargetPreservesValidTarget(t *testing.T) {
	workspace := t.TempDir()
	writeScanConfig(t, workspace)
	unsafeFile := filepath.Join(workspace, "artifact.json")
	validFile := filepath.Join(t.TempDir(), "valid.json")
	require.NoError(t, os.WriteFile(unsafeFile, []byte("{}"), 0600))
	require.NoError(t, os.WriteFile(validFile, []byte("{}"), 0600))
	runner := conftestRunner(conftestAllSuccessJSON)
	srv := newWorkspaceTestServer(t, ServerOptions{Runner: runner, WorkspaceDir: workspace,
		ToolChecker: func() ([]string, error) { return nil, nil }})
	rejected := localTarget(t, unsafeFile, "")
	rejected.TargetID = "rejected-target"
	valid := localTarget(t, validFile, "")
	valid.TargetID = "valid-target"
	response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{rejected, valid}))
	require.NoError(t, err)
	require.Len(t, response.Errors, 1)
	assert.Contains(t, response.Errors[0], rejected.TargetID)
	assert.Contains(t, response.Errors[0], "is inside workspace")
	assert.NotContains(t, response.Errors[0], valid.TargetID)
	require.Len(t, response.Assessments, 1)
	assert.Equal(t, "test-req", response.Assessments[0].RequirementID)
	require.Len(t, response.Assessments[0].Steps, 1)
	assert.Equal(t, validFile, response.Assessments[0].Steps[0].Name)
	assert.Equal(t, provider.ResultPassed, response.Assessments[0].Steps[0].Result)
	require.Len(t, runner.calls, 1)
	assert.Equal(t, "conftest", runner.calls[0].name)
	assert.Equal(t, "test", runner.calls[0].args[0])
	assert.Equal(t, validFile, runner.calls[0].args[1])
}

func TestScan_InvalidDirectoryOutputIsOperational(t *testing.T) {
	for _, output := range []string{"", "not json"} {
		t.Run(output, func(t *testing.T) {
			workspace := t.TempDir()
			writeScanConfig(t, workspace)
			input := t.TempDir()
			runner := conftestRunner(output)
			srv := newWorkspaceTestServer(t, ServerOptions{
				Runner: runner, WorkspaceDir: workspace,
				ToolChecker: func() ([]string, error) { return nil, nil },
			})
			target := localTarget(t, input, "")
			response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{target}))
			require.NoError(t, err)
			require.Len(t, response.Errors, 1)
			assert.Contains(t, response.Errors[0], target.TargetID)
			assert.NotContains(t, response.Errors[0], "no eligible input")
			assert.Contains(t, response.Errors[0], "check conftest output")
			assert.Empty(t, response.Assessments)
		})
	}
}

func TestScan_ConftestExecutionFailureDoesNotClaimEmptyInput(t *testing.T) {
	for _, diagnostic := range []string{
		"permission denied", "rego parse error", "rego error: parse files: no files found",
	} {
		t.Run(diagnostic, func(t *testing.T) {
			workspace := t.TempDir()
			writeScanConfig(t, workspace)
			input := t.TempDir()
			runner := &mockRunner{callFn: func(string, []string) ([]byte, error) {
				return []byte(diagnostic), errors.New("exit status 1")
			}}
			srv := newWorkspaceTestServer(t, ServerOptions{Runner: runner, WorkspaceDir: workspace,
				ToolChecker: func() ([]string, error) { return nil, nil }})
			target := localTarget(t, input, "")
			response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{target}))
			require.NoError(t, err)
			require.Len(t, response.Errors, 1)
			assert.Contains(t, response.Errors[0], target.TargetID)
			assert.Contains(t, response.Errors[0], diagnostic)
			assert.Contains(t, response.Errors[0], "check conftest output and policy configuration")
			assert.NotContains(t, response.Errors[0], "no eligible input")
			assert.NotContains(t, response.Errors[0], "adjust ignore settings")
			assert.Empty(t, response.Assessments)
		})
	}
}

func TestScan_NoFilesSentinelRequiresDirectoryInput(t *testing.T) {
	workspace := t.TempDir()
	writeScanConfig(t, workspace)
	input := filepath.Join(t.TempDir(), "input.json")
	require.NoError(t, os.WriteFile(input, []byte("{}"), 0600))
	runner := &mockRunner{callFn: func(string, []string) ([]byte, error) {
		return []byte(conftestNoFilesFound + "\n"), errors.New("exit status 1")
	}}
	srv := newWorkspaceTestServer(t, ServerOptions{Runner: runner, WorkspaceDir: workspace,
		ToolChecker: func() ([]string, error) { return nil, nil }})
	target := localTarget(t, input, "")
	response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{target}))
	require.NoError(t, err)
	require.Len(t, response.Errors, 1)
	assert.Contains(t, response.Errors[0], target.TargetID)
	assert.Contains(t, response.Errors[0], conftestNoFilesFound)
	assert.NotContains(t, response.Errors[0], "no eligible input")
	assert.Empty(t, response.Assessments)
}

func TestScan_NoFilesSentinelOnDirectoryReportsRemedy(t *testing.T) {
	workspace := t.TempDir()
	writeScanConfig(t, workspace)
	input := t.TempDir()
	runner := &mockRunner{callFn: func(string, []string) ([]byte, error) {
		return []byte(conftestNoFilesFound + "\n"), errors.New("exit status 1")
	}}
	srv := newWorkspaceTestServer(t, ServerOptions{Runner: runner, WorkspaceDir: workspace,
		ToolChecker: func() ([]string, error) { return nil, nil }})
	target := localTarget(t, input, "")
	response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{target}))
	require.NoError(t, err)
	require.Len(t, response.Errors, 1)
	assert.Contains(t, response.Errors[0], target.TargetID)
	assert.Contains(t, response.Errors[0], "no eligible input")
	assert.Contains(t, response.Errors[0], "add a supported input or adjust ignore settings")
	assert.Contains(t, response.Errors[0], conftestNoFilesFound)
	assert.Empty(t, response.Assessments)
}

func TestScan_EmptyTargetDoesNotSuppressOtherAssessments(t *testing.T) {
	workspace := t.TempDir()
	writeScanConfig(t, workspace)
	empty := t.TempDir()
	valid := t.TempDir()
	runner := &mockRunner{callFn: func(_ string, args []string) ([]byte, error) {
		if args[1] == empty {
			return []byte(`[]`), nil
		}
		return []byte(conftestAllSuccessJSON), nil
	}}
	srv := newWorkspaceTestServer(t, ServerOptions{
		Runner: runner, WorkspaceDir: workspace,
		ToolChecker: func() ([]string, error) { return nil, nil },
	})
	emptyTarget := localTarget(t, empty, "")
	emptyTarget.TargetID = "empty"
	validTarget := localTarget(t, valid, "")
	validTarget.TargetID = "valid"
	response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{emptyTarget, validTarget}))
	require.NoError(t, err)
	require.Len(t, response.Errors, 1)
	assert.Contains(t, response.Errors[0], "empty")
	assert.Contains(t, response.Errors[0], "no eligible input")
	require.Len(t, response.Assessments, 1)
	require.Len(t, response.Assessments[0].Steps, 1)
	assert.Equal(t, valid, response.Assessments[0].Steps[0].Name)
	assert.Equal(t, provider.ResultPassed, response.Assessments[0].Steps[0].Result)
}
