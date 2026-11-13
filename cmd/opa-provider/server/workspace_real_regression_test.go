// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/complytime/complytime-providers/cmd/opa-provider/config"
	"github.com/complytime/complytime-providers/cmd/opa-provider/generate"
	"github.com/complytime/complytime-providers/cmd/opa-provider/loader"
	"github.com/complytime/complytime-providers/cmd/opa-provider/scan"
)

type realConftestCall struct {
	args   []string
	output []byte
	stderr []byte
	err    error
}

// recordingRealRunner executes the real tool while retaining its structured
// output; remote clone URLs are redirected to a local test repository.
type recordingRealRunner struct {
	workingDirectory string
	gitSource        string
	clonePath        string
	postClone        func(string) error
	calls            []realConftestCall
}

func (runner *recordingRealRunner) Run(name string, args ...string) (scan.CommandOutput, error) {
	return runner.execute(nil, name, args)
}

func (runner *recordingRealRunner) RunWithEnv(env []string, name string, args ...string) (scan.CommandOutput, error) {
	return runner.execute(env, name, args)
}

func (runner *recordingRealRunner) execute(env []string, name string, args []string) (scan.CommandOutput, error) {
	actualArgs := append([]string(nil), args...)
	if name == "git" {
		if runner.gitSource == "" || len(actualArgs) != 7 || actualArgs[0] != "clone" {
			return scan.CommandOutput{}, fmt.Errorf("unexpected git invocation %q", actualArgs)
		}
		actualArgs[5] = runner.gitSource
		runner.clonePath = actualArgs[6]
	}
	var output scan.CommandOutput
	var err error
	if env == nil {
		output, err = (scan.ExecRunner{WorkingDirectory: runner.workingDirectory}).Run(name, actualArgs...)
	} else {
		output, err = (scan.ExecRunner{WorkingDirectory: runner.workingDirectory}).RunWithEnv(env, name, actualArgs...)
	}
	if name == "git" && err == nil && runner.postClone != nil {
		if err := runner.postClone(runner.clonePath); err != nil {
			return output, fmt.Errorf("preparing clone fixture: %w", err)
		}
	}
	if name == "conftest" && len(actualArgs) > 0 && actualArgs[0] == "test" {
		runner.calls = append(runner.calls, realConftestCall{
			args: actualArgs, output: output.Stdout, stderr: output.Stderr, err: err,
		})
	}
	return output, err
}

type realCheckResult struct {
	Filename  string `json:"filename"`
	Namespace string `json:"namespace"`
	Successes int    `json:"successes"`
	Failures  []struct {
		Message string `json:"msg"`
	} `json:"failures"`
	Warnings []json.RawMessage `json:"warnings"`
}

func isolateRealConftest(t *testing.T, base string) {
	t.Helper()
	t.Chdir(base)
	t.Setenv("HOME", base)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "xdg-config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(base, "xdg-data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(base, "xdg-cache"))
	for _, item := range os.Environ() {
		name, _, found := strings.Cut(item, "=")
		if found && strings.HasPrefix(name, "CONFTEST_") {
			t.Setenv(name, "")
		}
	}
	t.Setenv(conftestIgnoreEnv, "")
	t.Setenv(conftestConfigFileEnv, "")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
}

func realPolicyFixture(t *testing.T, workspace, policyDir string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(workspace, 0750))
	require.NoError(t, os.MkdirAll(policyDir, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(policyDir, "policy.rego"), []byte(
		"package main\ndeny contains input.marker if input.blocked == true\n"), 0600))
	cfg := config.NewWorkspaceConfig(workspace)
	require.NoError(t, cfg.EnsureDirectories())
	require.NoError(t, generate.WriteScanConfig(cfg.GeneratedDirPath(), []string{"main"},
		map[string]string{"main": "test-req"}, policyDir))
}

func decodeRealCall(t *testing.T, call realConftestCall) []realCheckResult {
	t.Helper()
	require.NoError(t, call.err, "%s", call.output)
	var records []realCheckResult
	require.NoError(t, json.Unmarshal(call.output, &records), "%s", call.output)
	return records
}

func assertRealAssessment(t *testing.T, response *provider.ScanResponse, result provider.Result, marker string) {
	t.Helper()
	assert.Empty(t, response.Errors)
	require.Len(t, response.Assessments, 1)
	assert.Equal(t, "test-req", response.Assessments[0].RequirementID)
	require.NotEmpty(t, response.Assessments[0].Steps)
	for _, step := range response.Assessments[0].Steps {
		assert.Equal(t, result, step.Result)
		assert.Contains(t, step.Message, marker)
		assert.NotContains(t, step.Message, "workspace-deny")
	}
}

func TestScan_RealConftestWorkspaceIsolation(t *testing.T) {
	requireRealConftest(t)
	base := t.TempDir()
	isolateRealConftest(t, base)
	t.Setenv(conftestIgnoreEnv, `user-ignored\.json$`)
	input := filepath.Join(base, "input [x]+ space")
	workspace := filepath.Join(input, "workspace (reserved)+")
	policy := filepath.Join(base, "policy")
	realPolicyFixture(t, workspace, policy)
	write := func(path, content string) {
		t.Helper()
		require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	}
	externalFail := filepath.Join(input, "external-fail.json")
	externalPass := filepath.Join(input, "external-pass.json")
	workspaceFail := filepath.Join(workspace, "workspace-fail.json")
	workspacePass := filepath.Join(workspace, "workspace-pass.json")
	workspaceMalformed := filepath.Join(workspace, "workspace-malformed.json")
	write(externalFail, `{"blocked":true,"marker":"external-deny"}`)
	write(externalPass, `{"blocked":false,"marker":"external-pass"}`)
	write(filepath.Join(input, "user-ignored.json"), `{invalid`)
	write(workspaceFail, `{"blocked":true,"marker":"workspace-deny"}`)
	write(workspacePass, `{"blocked":false,"marker":"workspace-pass"}`)
	write(workspaceMalformed, `{invalid`)

	// Verify the workspace fixtures would really change output or fail parsing.
	control := scan.ExecRunner{}
	for _, test := range []struct {
		path    string
		success int
		failure string
	}{
		{workspaceFail, 0, "workspace-deny"},
		{workspacePass, 1, ""},
	} {
		raw, err := scan.EvalPolicyWithNamespacesAndIgnore(test.path, policy, []string{"main"}, control, "", nil)
		require.NoError(t, err)
		records := decodeRealCall(t, realConftestCall{output: raw.Stdout})
		require.Len(t, records, 1)
		assert.Equal(t, test.path, records[0].Filename)
		assert.Equal(t, test.success, records[0].Successes)
		if test.failure != "" {
			require.Len(t, records[0].Failures, 1)
			assert.Equal(t, test.failure, records[0].Failures[0].Message)
		} else {
			assert.Empty(t, records[0].Failures)
		}
	}
	_, err := scan.EvalPolicyWithNamespacesAndIgnore(workspaceMalformed, policy, []string{"main"}, control, "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), workspaceMalformed)

	runner := &recordingRealRunner{}
	srv := newWorkspaceTestServer(t, ServerOptions{Runner: runner, WorkspaceDir: workspace,
		ToolChecker: func() ([]string, error) { return nil, nil }})
	relativeInput, err := filepath.Rel(workspace, input)
	require.NoError(t, err)
	for index, path := range []string{relativeInput, input, input} {
		t.Run(fmt.Sprintf("scan-%d", index+1), func(t *testing.T) {
			runner.calls = nil
			response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{
				localTarget(t, path, ""),
			}))
			require.NoError(t, err)
			assertRealAssessment(t, response, provider.ResultFailed, "external-deny")
			require.Len(t, response.Assessments[0].Steps, 1)
			require.Len(t, runner.calls, 1)
			records := decodeRealCall(t, runner.calls[0])
			require.Len(t, records, 2, "%s", runner.calls[0].output)
			seen := map[string]realCheckResult{}
			for _, record := range records {
				absolute, absErr := filepath.Abs(record.Filename)
				require.NoError(t, absErr)
				seen[absolute] = record
				assert.Equal(t, "main", record.Namespace)
				assert.Empty(t, record.Warnings)
			}
			require.Contains(t, seen, externalFail)
			require.Contains(t, seen, externalPass)
			assert.Equal(t, 0, seen[externalFail].Successes)
			require.Len(t, seen[externalFail].Failures, 1)
			assert.Equal(t, "external-deny", seen[externalFail].Failures[0].Message)
			assert.Equal(t, 1, seen[externalPass].Successes)
			assert.Empty(t, seen[externalPass].Failures)
			write(filepath.Join(workspace, fmt.Sprintf("new-artifact-%d.json", index)),
				`{"blocked":true,"marker":"workspace-deny"}`)
		})
	}

	t.Run("clean-file", func(t *testing.T) {
		runner.calls = nil
		response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{
			localTarget(t, externalPass, ""),
		}))
		require.NoError(t, err)
		assertRealAssessment(t, response, provider.ResultPassed, "all checks passed")
		require.Len(t, runner.calls, 1)
		records := decodeRealCall(t, runner.calls[0])
		require.Len(t, records, 1)
		assert.Equal(t, externalPass, records[0].Filename)
		assert.Equal(t, "main", records[0].Namespace)
		assert.Equal(t, 1, records[0].Successes)
		assert.Empty(t, records[0].Failures)
		assert.Empty(t, records[0].Warnings)
	})

	for _, scenario := range []string{"empty", "fully ignored"} {
		t.Run(scenario, func(t *testing.T) {
			dir := filepath.Join(base, strings.ReplaceAll(scenario, " ", "-"))
			require.NoError(t, os.Mkdir(dir, 0750))
			if scenario == "fully ignored" {
				write(filepath.Join(dir, "ignored.json"), `{"blocked":false}`)
				t.Setenv(conftestIgnoreEnv, ".*")
			}
			runner.calls = nil
			response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{
				localTarget(t, dir, ""),
			}))
			require.NoError(t, err)
			require.Len(t, response.Errors, 1)
			assert.Contains(t, response.Errors[0], "no eligible input")
			assert.Contains(t, response.Errors[0], "add a supported input or adjust ignore settings")
			assert.Contains(t, response.Errors[0], "no files found")
			assert.Empty(t, response.Assessments)
			require.Len(t, runner.calls, 1)
			require.Error(t, runner.calls[0].err)
			assert.Contains(t, string(runner.calls[0].stderr), "no files found")
		})
	}
}

func TestScan_RealConftestLocalAliases(t *testing.T) {
	requireRealConftest(t)
	for _, test := range []struct {
		name, cause string
		identity    bool
	}{
		{"workspace file symlink", "resolves to workspace", false},
		{"workspace directory symlink", "resolves to workspace", false},
		{"hard link", "shares filesystem identity", true},
		{"explicit hard link", "shares filesystem identity", true},
		{"symlinked workspace root", "", false},
	} {
		scenario := test.name
		t.Run(scenario, func(t *testing.T) {
			base := t.TempDir()
			isolateRealConftest(t, base)
			input := filepath.Join(base, "input")
			workspace := filepath.Join(base, "workspace")
			require.NoError(t, os.Mkdir(input, 0750))
			if scenario == "symlinked workspace root" {
				actual := filepath.Join(input, "workspace-real")
				require.NoError(t, os.Mkdir(actual, 0750))
				require.NoError(t, os.Symlink(actual, workspace))
			}
			policy := filepath.Join(base, "policy")
			realPolicyFixture(t, workspace, policy)
			workspaceFile := filepath.Join(workspace, "artifact.json")
			require.NoError(t, os.WriteFile(workspaceFile,
				[]byte(`{"blocked":true,"marker":"workspace-deny"}`), 0600))
			eligible := filepath.Join(input, "eligible.json")
			require.NoError(t, os.WriteFile(eligible,
				[]byte(`{"blocked":true,"marker":"external-deny"}`), 0600))
			alias := filepath.Join(input, "alias.json")
			switch scenario {
			case "workspace file symlink":
				require.NoError(t, os.Symlink(workspaceFile, alias))
			case "workspace directory symlink":
				require.NoError(t, os.Symlink(workspace, alias))
			case "hard link", "explicit hard link":
				require.NoError(t, os.Link(workspaceFile, alias))
				artifactInfo, err := os.Stat(workspaceFile)
				require.NoError(t, err)
				aliasInfo, err := os.Stat(alias)
				require.NoError(t, err)
				require.True(t, os.SameFile(artifactInfo, aliasInfo))
			}
			runner := &recordingRealRunner{}
			srv := newWorkspaceTestServer(t, ServerOptions{Runner: runner, WorkspaceDir: workspace,
				ToolChecker: func() ([]string, error) { return nil, nil }})
			selected := input
			if scenario == "explicit hard link" {
				selected = alias
			}
			target := localTarget(t, selected, "")
			response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{target}))
			require.NoError(t, err)
			if test.cause == "" {
				assertRealAssessment(t, response, provider.ResultFailed, "external-deny")
				require.Len(t, runner.calls, 1)
				records := decodeRealCall(t, runner.calls[0])
				require.Len(t, records, 1)
				assert.Equal(t, eligible, records[0].Filename)
				require.Len(t, records[0].Failures, 1)
				assert.Equal(t, "external-deny", records[0].Failures[0].Message)
			} else {
				require.Len(t, response.Errors, 1)
				assert.Contains(t, response.Errors[0], target.TargetID)
				assert.Contains(t, response.Errors[0], test.cause)
				if test.identity {
					assert.Contains(t, response.Errors[0], workspaceFile)
				} else {
					assert.Contains(t, response.Errors[0], workspace)
				}
				assert.Empty(t, response.Assessments)
				assert.Empty(t, runner.calls, "unsafe alias must be rejected before conftest evaluation")
			}

			// A separate ordinary target still reaches the real tool and produces
			// a finding, so rejection cannot pass merely by disabling evaluation.
			runner.calls = nil
			response, err = srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{
				localTarget(t, eligible, ""),
			}))
			require.NoError(t, err)
			assertRealAssessment(t, response, provider.ResultFailed, "external-deny")
			require.Len(t, runner.calls, 1)
			records := decodeRealCall(t, runner.calls[0])
			require.Len(t, records, 1)
			assert.Equal(t, eligible, records[0].Filename)
			require.Len(t, records[0].Failures, 1)
			assert.Equal(t, "external-deny", records[0].Failures[0].Message)
		})
	}
}

func createRealGitRepository(t *testing.T, base string) string {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required to create the local repository fixture")
	}
	repository := filepath.Join(base, "source-repository")
	require.NoError(t, os.Mkdir(repository, 0750))
	command := exec.Command(git, "init", "--initial-branch=main", repository)
	output, err := command.CombinedOutput()
	require.NoError(t, err, "%s", output)
	return repository
}

func commitRealGitRepository(t *testing.T, repository string) {
	t.Helper()
	git, err := exec.LookPath("git")
	require.NoError(t, err)
	for _, args := range [][]string{
		{"-C", repository, "add", "--all"},
		{"-C", repository, "-c", "user.name=Test", "-c", "user.email=test@example.com",
			"-c", "commit.gpgsign=false", "commit", "-m", "fixture"},
	} {
		// The executable and arguments come only from this hermetic fixture.
		output, err := exec.Command(git, args...).CombinedOutput() // #nosec G204
		require.NoError(t, err, "%s", output)
	}
}

func TestScan_RealConftestRemoteCloneAliases(t *testing.T) {
	requireRealConftest(t)
	for _, test := range []struct {
		name, cause string
		workspace   bool
		identity    bool
	}{
		{"workspace file symlink", "resolves to workspace", true, false},
		{"workspace directory symlink", "resolves to workspace", true, false},
		{"selected scan alias", "resolves to workspace", true, false},
		{"external hardlink symlink", "shares filesystem identity", false, true},
		{"clone hardlink", "shares filesystem identity", false, true},
		{"symlink cycle", "directory symlink cycle", false, false},
		{"internal file symlink", "", false, false},
		{"internal directory symlink", "", false, false},
		{"ordinary clone", "", false, false},
	} {
		scenario := test.name
		t.Run(scenario, func(t *testing.T) {
			base := t.TempDir()
			isolateRealConftest(t, base)
			workspace := filepath.Join(base, "workspace")
			policy := filepath.Join(base, "policy")
			realPolicyFixture(t, workspace, policy)
			workspaceDir := filepath.Join(workspace, "other-provider")
			require.NoError(t, os.Mkdir(workspaceDir, 0750))
			workspaceFile := filepath.Join(workspaceDir, "artifact.json")
			require.NoError(t, os.WriteFile(workspaceFile,
				[]byte(`{"blocked":true,"marker":"workspace-deny"}`), 0600))
			repository := createRealGitRepository(t, base)
			nested := filepath.Join(repository, "nested")
			require.NoError(t, os.Mkdir(nested, 0750))
			require.NoError(t, os.WriteFile(filepath.Join(nested, "eligible.json"),
				[]byte(`{"blocked":true,"marker":"remote-deny"}`), 0600))
			alias := filepath.Join(repository, "alias.json")
			scanPath := ""
			runner := &recordingRealRunner{gitSource: repository}
			switch scenario {
			case "workspace file symlink":
				require.NoError(t, os.Symlink(workspaceFile, alias))
			case "workspace directory symlink":
				require.NoError(t, os.Symlink(workspaceDir, alias))
			case "selected scan alias":
				require.NoError(t, os.Symlink(workspaceFile, alias))
				scanPath = "alias.json"
			case "external hardlink symlink":
				external := filepath.Join(base, "external-hardlink.json")
				require.NoError(t, os.Link(workspaceFile, external))
				require.NoError(t, os.Symlink(external, alias))
			case "clone hardlink":
				runner.postClone = func(clone string) error {
					path := filepath.Join(clone, "nested", "eligible.json")
					if err := os.Remove(path); err != nil {
						return err
					}
					return os.Link(workspaceFile, path)
				}
			case "symlink cycle":
				require.NoError(t, os.Symlink(".", alias))
			case "internal file symlink":
				require.NoError(t, os.Symlink(filepath.Join("nested", "eligible.json"), alias))
			case "internal directory symlink":
				require.NoError(t, os.Symlink("nested", alias))
			}
			commitRealGitRepository(t, repository)
			srv := newWorkspaceTestServer(t, ServerOptions{Runner: runner, WorkspaceDir: workspace,
				ToolChecker: func() ([]string, error) { return nil, nil }})
			target := remoteTarget("", "https://example.com/org/repo")
			target.Variables[loader.VarScanPath] = scanPath
			response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{target}))
			require.NoError(t, err)
			require.NotEmpty(t, runner.clonePath, "git must clone the hermetic repository")
			_, err = os.Stat(runner.clonePath)
			require.NoError(t, err)
			if test.cause != "" {
				require.Len(t, response.Errors, 1)
				assert.Contains(t, response.Errors[0], target.TargetID)
				assert.Contains(t, response.Errors[0], test.cause)
				if test.workspace {
					assert.Contains(t, response.Errors[0], workspace)
				}
				if test.identity {
					assert.Contains(t, response.Errors[0], workspaceFile)
				}
				assert.Empty(t, response.Assessments)
				assert.Empty(t, runner.calls, "unsafe clone must be rejected before conftest evaluation")
				return
			}
			assertRealAssessment(t, response, provider.ResultFailed, "remote-deny")
			require.Len(t, runner.calls, 1)
			records := decodeRealCall(t, runner.calls[0])
			require.NotEmpty(t, records)
			for _, record := range records {
				assert.Equal(t, filepath.Join(runner.clonePath, "nested", "eligible.json"), record.Filename)
				assert.Equal(t, "main", record.Namespace)
				assert.Equal(t, 0, record.Successes)
				require.Len(t, record.Failures, 1)
				assert.Equal(t, "remote-deny", record.Failures[0].Message)
				assert.Empty(t, record.Warnings)
			}
		})
	}
}
