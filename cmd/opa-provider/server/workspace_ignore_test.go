// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/complytime/complytime-providers/cmd/opa-provider/config"
	"github.com/complytime/complytime-providers/cmd/opa-provider/generate"
	"github.com/complytime/complytime-providers/cmd/opa-provider/loader"
)

func TestWorkspaceIgnorePattern_WalkerPathsAndBoundaries(t *testing.T) {
	base := t.TempDir()
	t.Chdir(base)
	input := filepath.Join("input [a]+ space", "tree")
	workspace := filepath.Join(base, input, "custom (workspace)+")
	require.NoError(t, os.MkdirAll(workspace, 0750))
	for _, path := range []string{input, filepath.Join(base, input)} {
		t.Run(path, func(t *testing.T) {
			pattern, overlapping, err := workspaceIgnorePattern(path, []string{workspace})
			require.NoError(t, err)
			assert.Equal(t, []string{workspace}, overlapping)
			expression, err := regexp.Compile(pattern)
			require.NoError(t, err)
			walkerWorkspace := filepath.Join(path, "custom (workspace)+")
			assert.True(t, expression.MatchString(filepath.Join(walkerWorkspace, "opa", "result.json")))
			assert.True(t, expression.MatchString(filepath.Join(workspace, "opa", "result.json")))
			assert.False(t, expression.MatchString(filepath.Join(path, "custom (workspace)+-backup", "x.json")))
			assert.False(t, expression.MatchString(filepath.Join(path, "legitimate.json")))
		})
	}
	pattern, overlapping, err := workspaceIgnorePattern(base, []string{filepath.Join(t.TempDir(), "outside")})
	require.NoError(t, err)
	assert.Empty(t, pattern)
	assert.Empty(t, overlapping)
	secondWorkspace := filepath.Join(base, input, "other-workspace")
	require.NoError(t, os.MkdirAll(secondWorkspace, 0750))
	pattern, overlapping, err = workspaceIgnorePattern(input, []string{workspace, secondWorkspace})
	require.NoError(t, err)
	assert.Equal(t, []string{workspace, secondWorkspace}, overlapping)
	expression, err := regexp.Compile(pattern)
	require.NoError(t, err)
	assert.True(t, expression.MatchString(filepath.Join(input, "custom (workspace)+", "file.json")))
	assert.True(t, expression.MatchString(filepath.Join(input, "other-workspace", "file.json")))
	assert.False(t, expression.MatchString(filepath.Join(input, "other-workspace-backup", "file.json")))
}

func TestLocalWorkspaceIgnore_ReturnsOnlyOverlappingBoundaries(t *testing.T) {
	t.Setenv(conftestIgnoreEnv, "user-rule")
	input := t.TempDir()
	hostWorkspace := filepath.Join(input, provider.WorkspaceDir)
	require.NoError(t, os.Mkdir(hostWorkspace, 0750))
	externalWorkspace := t.TempDir()
	srv := newWorkspaceTestServer(t, ServerOptions{Runner: &mockRunner{}})
	pattern, environment, overlapping, err := srv.localWorkspaceIgnore(
		input, map[string]string{loader.VarWorkspace: input}, externalWorkspace,
	)
	require.NoError(t, err)
	assert.NotEmpty(t, environment)
	assert.Equal(t, []string{hostWorkspace}, overlapping)
	expression, err := regexp.Compile(pattern)
	require.NoError(t, err)
	assert.True(t, expression.MatchString(filepath.Join(hostWorkspace, "input.json")))
	assert.False(t, expression.MatchString(filepath.Join(externalWorkspace, "input.json")))
}

func TestScan_OverlappingTargetIgnoreConfiguration(t *testing.T) {
	tests := []struct {
		name         string
		env          string
		unset        bool
		configName   string
		detectError  bool
		wantError    bool
		wantUserRule bool
	}{
		{"unset and no config", "", true, "", false, false, false},
		{"empty and no config", "", false, "", false, false, false},
		{"nonempty and no config", "user-file", false, "", false, false, true},
		{"unset and config", "", true, "conftest.toml", false, true, false},
		{"empty and config", "", false, "conftest.toml", false, true, false},
		{"nonempty and config", "user-file", false, "conftest.toml", false, false, true},
		{"unsupported config", "", true, "conftest.backup", false, false, false},
		{"detection failure", "", true, "", true, true, false},
		{"rule bypasses detection failure", "user-file", false, "", true, false, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workingDir := t.TempDir()
			t.Chdir(workingDir)
			t.Setenv(conftestConfigFileEnv, "")
			t.Setenv(conftestIgnoreEnv, test.env)
			if test.unset {
				require.NoError(t, os.Unsetenv(conftestIgnoreEnv))
			}
			beforeValue, beforeSet := os.LookupEnv(conftestIgnoreEnv)
			input := filepath.Join(t.TempDir(), "input [x]+")
			workspace := filepath.Join(input, "workspace (custom)+")
			require.NoError(t, os.MkdirAll(workspace, 0750))
			if test.configName != "" {
				require.NoError(t, os.WriteFile(filepath.Join(workspace, test.configName), nil, 0600))
			}
			writeScanConfig(t, workspace)
			runner := conftestRunner(conftestAllSuccessJSON)
			srv := newWorkspaceTestServer(t, ServerOptions{
				Runner:       runner,
				ToolChecker:  func() ([]string, error) { return nil, nil },
				WorkspaceDir: workspace,
			})
			if test.detectError {
				srv.detectConftestConfig = func(string) (string, error) {
					return "", errors.New("search /fake/conftest: injected failure")
				}
			}
			target := localTarget(t, input, "")
			response, err := srv.Scan(context.Background(), &provider.ScanRequest{
				Targets: []provider.Target{target},
			})
			require.NoError(t, err)
			afterValue, afterSet := os.LookupEnv(conftestIgnoreEnv)
			assert.Equal(t, beforeValue, afterValue)
			assert.Equal(t, beforeSet, afterSet)
			if test.wantError {
				require.Len(t, response.Errors, 1)
				assert.Contains(t, response.Errors[0], target.TargetID)
				assert.Contains(t, response.Errors[0], "CONFTEST_IGNORE")
				assert.Contains(t, response.Errors[0], "outside the workspace")
				assert.Contains(t, response.Errors[0], "^$")
				if test.detectError {
					assert.Contains(t, response.Errors[0], "/fake/conftest")
					assert.Contains(t, response.Errors[0], "injected failure")
				} else {
					assert.Contains(t, response.Errors[0], filepath.Join(workspace, test.configName))
				}
				assert.Empty(t, response.Assessments)
				assert.Empty(t, runner.calls)
				return
			}
			assert.Empty(t, response.Errors)
			require.Len(t, response.Assessments, 1)
			require.Len(t, runner.calls, 1)
			assert.Equal(t, os.Environ(), runner.calls[0].env)
			args := runner.calls[0].args
			var ignores []string
			for index, arg := range args {
				if arg == "--ignore" {
					require.Less(t, index+1, len(args))
					ignores = append(ignores, args[index+1])
				}
			}
			require.Len(t, ignores, 1)
			expression, err := regexp.Compile(ignores[0])
			require.NoError(t, err)
			assert.True(t, expression.MatchString(filepath.Join(workspace, "file.json")))
			assert.False(t, expression.MatchString(filepath.Join(input, "workspace (custom)+-backup", "file.json")))
			assert.Equal(t, test.wantUserRule, expression.MatchString(filepath.Join(input, "user-file.json")))
		})
	}
}

func TestEnvironmentValue_UsesLastMatchingEntry(t *testing.T) {
	environment := []string{"OTHER=x", "CONFTEST_IGNORE=old", "CONFTEST_IGNORE=new"}
	assert.Equal(t, "new", environmentValue(environment, conftestIgnoreEnv))
	assert.Empty(t, environmentValue(environment, conftestConfigFileEnv))
}

func TestScan_ExplicitConfigPathsRejected(t *testing.T) {
	for _, absolute := range []bool{false, true} {
		t.Run(fmt.Sprintf("absolute=%t", absolute), func(t *testing.T) {
			workingDir := t.TempDir()
			t.Chdir(workingDir)
			t.Setenv(conftestIgnoreEnv, "")
			path := filepath.Join(workingDir, "configs", "settings.toml")
			require.NoError(t, os.MkdirAll(filepath.Dir(path), 0750))
			require.NoError(t, os.WriteFile(path, nil, 0600))
			setting := filepath.Join("configs", "settings.toml")
			if absolute {
				setting = path
			}
			t.Setenv(conftestConfigFileEnv, setting)
			input := t.TempDir()
			workspace := filepath.Join(input, "workspace")
			require.NoError(t, os.MkdirAll(workspace, 0750))
			if !absolute {
				path = filepath.Join(workspace, setting)
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0750))
				require.NoError(t, os.WriteFile(path, nil, 0600))
			}
			writeScanConfig(t, workspace)
			runner := conftestRunner(conftestAllSuccessJSON)
			srv := newWorkspaceTestServer(t, ServerOptions{
				Runner: runner, WorkspaceDir: workspace,
				ToolChecker: func() ([]string, error) { return nil, nil },
			})
			response, err := srv.Scan(context.Background(), &provider.ScanRequest{
				Targets: []provider.Target{localTarget(t, input, "")},
			})
			require.NoError(t, err)
			require.Len(t, response.Errors, 1)
			assert.Contains(t, response.Errors[0], path)
			assert.Contains(t, response.Errors[0], "CONFTEST_IGNORE")
			assert.Empty(t, response.Assessments)
			assert.Empty(t, runner.calls)
		})
	}
}

func TestScan_NonOverlappingInputsDoNotAddIgnore(t *testing.T) {
	workingDir := t.TempDir()
	t.Chdir(workingDir)
	t.Setenv(conftestIgnoreEnv, "")
	t.Setenv(conftestConfigFileEnv, "")
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "conftest.toml"), nil, 0600))
	workspace := t.TempDir()
	writeScanConfig(t, workspace)
	directory := t.TempDir()
	file := filepath.Join(t.TempDir(), "file.json")
	require.NoError(t, os.WriteFile(file, []byte("{}"), 0600))
	for _, input := range []string{directory, file} {
		t.Run(input, func(t *testing.T) {
			runner := conftestRunner(conftestAllSuccessJSON)
			srv := newWorkspaceTestServer(t, ServerOptions{
				Runner: runner, WorkspaceDir: workspace,
				ToolChecker: func() ([]string, error) { return nil, nil },
			})
			srv.detectConftestConfig = func(string) (string, error) {
				return "", errors.New("detection must not run")
			}
			response, err := srv.Scan(context.Background(), &provider.ScanRequest{
				Targets: []provider.Target{localTarget(t, input, "")},
			})
			require.NoError(t, err)
			assert.Empty(t, response.Errors)
			require.Len(t, response.Assessments, 1)
			require.Len(t, runner.calls, 1)
			assert.NotContains(t, runner.calls[0].args, "--ignore")
		})
	}
}

func TestScan_RemoteRepositoryCloneRemainsEvaluable(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for the hermetic repository fixture")
	}
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	repository := t.TempDir()
	for _, args := range [][]string{
		{"init", "--initial-branch=main", repository},
		{"-C", repository, "add", "config.json"},
		{"-C", repository, "-c", "user.name=Test", "-c", "user.email=test@example.com",
			"commit", "-m", "fixture"},
	} {
		if args[0] == "-C" && args[2] == "add" {
			require.NoError(t, os.WriteFile(
				filepath.Join(repository, "config.json"), []byte(`{"blocked":true}`), 0600,
			))
		}
		output, err := exec.Command("git", args...).CombinedOutput()
		require.NoError(t, err, "%s", output)
	}
	workspace := t.TempDir()
	writeScanConfig(t, workspace)
	runner := &mockRunner{callFn: func(name string, args []string) ([]byte, error) {
		if name == "git" {
			require.Len(t, args, 7)
			cloneArgs := append([]string(nil), args...)
			cloneArgs[5] = repository
			return exec.Command("git", cloneArgs...).CombinedOutput()
		}
		require.Equal(t, "conftest", name)
		require.Equal(t, "test", args[0])
		assert.NotContains(t, args, "--ignore")
		data, err := os.ReadFile(filepath.Join(args[1], "config.json"))
		require.NoError(t, err)
		require.JSONEq(t, `{"blocked":true}`, string(data))
		return []byte(`[{
			"filename":"config.json", "namespace":"main", "failures":[{
				"msg":"blocked configuration", "metadata":{"query":"data.main.deny"}
			}]
		}]`), nil
	}}
	srv := newWorkspaceTestServer(t, ServerOptions{
		Runner: runner, WorkspaceDir: workspace,
		ToolChecker: func() ([]string, error) { return nil, nil },
	})
	srv.detectConftestConfig = func(string) (string, error) {
		return "", errors.New("remote targets must not check local exclusion config")
	}
	response, err := srv.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{remoteTarget("", "https://github.com/org/repo")},
	})
	require.NoError(t, err)
	assert.Empty(t, response.Errors)
	require.Len(t, response.Assessments, 1)
	assert.Equal(t, "test-req", response.Assessments[0].RequirementID)
	assert.Equal(t, provider.ResultFailed, response.Assessments[0].Steps[0].Result)
	require.Len(t, runner.calls, 2)
}

func TestScan_WorkspaceIgnoreWithConftest(t *testing.T) {
	requireRealConftest(t)
	workingDir := t.TempDir()
	t.Chdir(workingDir)
	t.Setenv("HOME", workingDir)
	t.Setenv("XDG_DATA_HOME", filepath.Join(workingDir, "xdg"))
	t.Setenv(conftestIgnoreEnv, `user-ignored\.json$`)
	t.Setenv(conftestConfigFileEnv, "")
	require.NoError(t, os.WriteFile(
		filepath.Join(workingDir, "conftest.toml"), []byte(`ignore = "does-not-apply"`), 0600,
	))
	input := filepath.Join(workingDir, "input [x]+ space")
	workspace := filepath.Join(input, "workspace (custom)+")
	policyDir := filepath.Join(workingDir, "policy")
	require.NoError(t, os.MkdirAll(workspace, 0750))
	require.NoError(t, os.MkdirAll(policyDir, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(policyDir, "policy.rego"), []byte(
		"package main\ndeny contains \"blocked configuration\" if input.blocked == true\n",
	), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(input, "eligible.json"), []byte(`{"blocked":true}`), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(input, "user-ignored.json"), []byte(`{invalid`), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "artifact.json"), []byte(`{invalid`), 0600))
	cfg := config.NewWorkspaceConfig(workspace)
	require.NoError(t, cfg.EnsureDirectories())
	require.NoError(t, generate.WriteScanConfig(
		cfg.GeneratedDirPath(), []string{"main"}, map[string]string{"main": "test-req"}, policyDir,
	))
	relativeInput, err := filepath.Rel(workspace, input)
	require.NoError(t, err)
	for _, path := range []string{relativeInput, input} {
		t.Run(path, func(t *testing.T) {
			srv := newWorkspaceTestServer(t, ServerOptions{
				WorkspaceDir: workspace,
				ToolChecker:  func() ([]string, error) { return nil, nil },
			})
			response, err := srv.Scan(context.Background(), &provider.ScanRequest{
				Targets: []provider.Target{localTarget(t, path, "")},
			})
			require.NoError(t, err)
			assert.Empty(t, response.Errors)
			require.Len(t, response.Assessments, 1)
			assert.Equal(t, "test-req", response.Assessments[0].RequirementID)
			require.Len(t, response.Assessments[0].Steps, 1)
			assert.Equal(t, provider.ResultFailed, response.Assessments[0].Steps[0].Result)
			assert.Contains(t, response.Assessments[0].Steps[0].Message, "blocked configuration")
		})
	}
}
