// SPDX-License-Identifier: Apache-2.0

package server

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/complytime/complytime-providers/cmd/opa-provider/config"
	"github.com/complytime/complytime-providers/cmd/opa-provider/generate"
)

func TestScan_LocalWorkspaceAliasesFailClosed(t *testing.T) {
	for _, test := range []struct {
		name, cause string
		workspace   bool
	}{
		{"file alias", "resolves to workspace", true},
		{"directory alias", "resolves to workspace", true},
		{"nested directory alias", "resolves to workspace", true},
		{"indirect file alias", "resolves to workspace", true},
		{"symlinked workspace root", "resolves to workspace", true},
		{"directory cycle", "directory symlink cycle", false},
		{"symlink loop", "too many links", false},
		{"broken link", "no such file or directory", false},
		{"selected file alias", "is inside workspace", true},
		{"selected directory alias", "is inside workspace", true},
		{"selected broken link", "no such file or directory", false},
		{"selected symlink loop", "too many", false},
	} {
		scenario := test.name
		t.Run(scenario, func(t *testing.T) {
			base := t.TempDir()
			input := filepath.Join(base, "input")
			workspace := filepath.Join(base, "workspace")
			require.NoError(t, os.MkdirAll(input, 0750))
			require.NoError(t, os.MkdirAll(filepath.Join(workspace, "nested"), 0750))
			workspaceFile := filepath.Join(workspace, "nested", "file.json")
			require.NoError(t, os.WriteFile(workspaceFile, []byte("{}"), 0600))
			writeScanConfig(t, workspace)
			alias := filepath.Join(input, "alias")
			var destination string
			switch scenario {
			case "file alias", "selected file alias":
				destination = workspaceFile
			case "directory alias", "selected directory alias", "symlinked workspace root":
				destination = workspace
			case "nested directory alias":
				destination = filepath.Join(workspace, "nested")
			case "indirect file alias":
				destination = filepath.Join(base, "external")
				require.NoError(t, os.Mkdir(destination, 0750))
				require.NoError(t, os.Symlink(workspaceFile, filepath.Join(destination, "file.json")))
			case "directory cycle":
				destination = input
			case "symlink loop", "selected symlink loop":
				destination = alias
			case "broken link", "selected broken link":
				destination = filepath.Join(base, "missing")
			}
			require.NoError(t, os.Symlink(destination, alias))
			if scenario == "selected file alias" || scenario == "selected directory alias" ||
				scenario == "selected broken link" || scenario == "selected symlink loop" {
				input = alias
			}
			if scenario == "symlinked workspace root" {
				workspace = alias
			}
			runner := conftestRunner(conftestAllSuccessJSON)
			srv := newWorkspaceTestServer(t, ServerOptions{
				Runner: runner, WorkspaceDir: workspace,
				ToolChecker: func() ([]string, error) { return nil, nil },
			})
			target := localTarget(t, input, "")
			response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{target}))
			require.NoError(t, err)
			require.Len(t, response.Errors, 1)
			assert.Contains(t, response.Errors[0], target.TargetID)
			assert.Contains(t, response.Errors[0], test.cause)
			if test.workspace {
				assert.Contains(t, response.Errors[0], filepath.Join(base, "workspace"))
			}
			assert.Empty(t, response.Assessments)
			assert.Empty(t, runner.calls)
		})
	}
}

func TestWorkspaceBoundaries_BrokenAndLoopingSymlinks(t *testing.T) {
	base := t.TempDir()
	for _, test := range []struct {
		name, destination, cause string
	}{
		{"broken", filepath.Join(base, "missing"), "no such file or directory"},
		{"loop", filepath.Join(base, "loop"), "too many links"},
	} {
		t.Run(test.name, func(t *testing.T) {
			alias := filepath.Join(base, test.name)
			require.NoError(t, os.Symlink(test.destination, alias))
			_, err := workspaceBoundaries(nil, alias)
			require.Error(t, err)
			assert.Contains(t, err.Error(), alias)
			assert.Contains(t, err.Error(), "resolving workspace")
			assert.Contains(t, err.Error(), test.cause)
		})
	}
}

func TestInspectLocalInput_PropagatesTraversalError(t *testing.T) {
	input := t.TempDir()
	sentinel := errors.New("read failed")
	traversal := localTraversal{
		readDir: func(string) ([]os.DirEntry, error) { return nil, sentinel },
		lstat:   os.Lstat,
	}
	err := inspectInput(context.Background(), input, nil, "", "", traversal, nil, nil)
	require.ErrorIs(t, err, sentinel)
	assert.Contains(t, err.Error(), input)
	assert.Contains(t, err.Error(), "fix inaccessible or unsafe paths")
}

func TestScan_OrdinaryAliasesRemainEvaluable(t *testing.T) {
	workspace := t.TempDir()
	writeScanConfig(t, workspace)
	input := t.TempDir()
	external := t.TempDir()
	file := filepath.Join(external, "file.json")
	require.NoError(t, os.WriteFile(file, []byte("{}"), 0600))
	require.NoError(t, os.Symlink(file, filepath.Join(input, "alias.json")))
	require.NoError(t, os.Symlink(external, filepath.Join(input, "directory")))
	runner := conftestRunner(conftestAllSuccessJSON)
	srv := newWorkspaceTestServer(t, ServerOptions{
		Runner: runner, WorkspaceDir: workspace,
		ToolChecker: func() ([]string, error) { return nil, nil },
	})
	for _, path := range []string{input, filepath.Join(input, "directory"), filepath.Join(input, "alias.json")} {
		t.Run(path, func(t *testing.T) {
			runner.calls = nil
			response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{
				localTarget(t, path, ""),
			}))
			require.NoError(t, err)
			assert.Empty(t, response.Errors)
			require.Len(t, response.Assessments, 1)
			assert.Equal(t, "test-req", response.Assessments[0].RequirementID)
			require.Len(t, runner.calls, 1)
			assert.Equal(t, path, runner.calls[0].args[1])
		})
	}
}

func TestScan_IgnoredWorkspaceTraversalFailureIsOperational(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv(conftestIgnoreEnv, "")
	t.Setenv(conftestConfigFileEnv, "")
	input := t.TempDir()
	workspace := filepath.Join(input, "workspace")
	require.NoError(t, os.Mkdir(workspace, 0750))
	writeScanConfig(t, workspace)
	runner := conftestRunner(conftestAllSuccessJSON)
	srv := newWorkspaceTestServer(t, ServerOptions{
		Runner: runner, WorkspaceDir: workspace,
		ToolChecker: func() ([]string, error) { return nil, nil },
	})
	srv.localTraversal.readDir = func(path string) ([]os.DirEntry, error) {
		if path == workspace {
			return nil, os.ErrPermission
		}
		return os.ReadDir(path)
	}
	target := localTarget(t, input, "")
	response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{target}))
	require.NoError(t, err)
	require.Len(t, response.Errors, 1)
	assert.Contains(t, response.Errors[0], target.TargetID)
	assert.Contains(t, response.Errors[0], workspace)
	assert.Contains(t, response.Errors[0], "permission denied")
	assert.Contains(t, response.Errors[0], "fix inaccessible or unsafe paths")
	assert.Empty(t, response.Assessments)
	assert.Empty(t, runner.calls)
}

func TestScan_BrokenLinkInsideIgnoredWorkspaceIsOperational(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv(conftestIgnoreEnv, "")
	t.Setenv(conftestConfigFileEnv, "")
	input := t.TempDir()
	workspace := filepath.Join(input, "workspace")
	require.NoError(t, os.Mkdir(workspace, 0750))
	writeScanConfig(t, workspace)
	broken := filepath.Join(workspace, "broken")
	require.NoError(t, os.Symlink(filepath.Join(input, "missing"), broken))
	runner := conftestRunner(conftestAllSuccessJSON)
	srv := newWorkspaceTestServer(t, ServerOptions{
		Runner: runner, WorkspaceDir: workspace,
		ToolChecker: func() ([]string, error) { return nil, nil },
	})
	target := localTarget(t, input, "")
	response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{target}))
	require.NoError(t, err)
	require.Len(t, response.Errors, 1)
	assert.Contains(t, response.Errors[0], target.TargetID)
	assert.Contains(t, response.Errors[0], broken)
	assert.Contains(t, response.Errors[0], "fix inaccessible or unsafe paths")
	assert.Empty(t, response.Assessments)
	assert.Empty(t, runner.calls)
}

func TestScan_ConftestTraversalFailureIsOperational(t *testing.T) {
	workspace := t.TempDir()
	writeScanConfig(t, workspace)
	input := t.TempDir()
	runner := &mockRunner{callFn: func(_ string, _ []string) ([]byte, error) {
		return []byte("walk path: permission denied"), errors.New("exit status 1")
	}}
	srv := newWorkspaceTestServer(t, ServerOptions{
		Runner: runner, WorkspaceDir: workspace,
		ToolChecker: func() ([]string, error) { return nil, nil },
	})
	target := localTarget(t, input, "")
	response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{target}))
	require.NoError(t, err)
	require.Len(t, response.Errors, 1)
	assert.Contains(t, response.Errors[0], target.TargetID)
	assert.Contains(t, response.Errors[0], "walk path: permission denied")
	assert.Contains(t, response.Errors[0], "check input paths, filesystem access")
	assert.Empty(t, response.Assessments)
	require.Len(t, runner.calls, 1)
}

func TestProcessLocalInput_LogsExcludedBoundary(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv(conftestIgnoreEnv, "")
	t.Setenv(conftestConfigFileEnv, "")
	input := t.TempDir()
	workspace := filepath.Join(input, "workspace")
	require.NoError(t, os.Mkdir(workspace, 0750))
	cfg := config.NewWorkspaceConfig(workspace)
	require.NoError(t, cfg.EnsureDirectories())
	var logOutput bytes.Buffer
	logger := hclog.New(&hclog.LoggerOptions{Output: &logOutput, Level: hclog.Info})
	runner := conftestRunner(conftestAllSuccessJSON)
	srv := newWorkspaceTestServer(t, ServerOptions{Runner: runner, WorkspaceDir: workspace})
	result, err := srv.processLocalInput(context.Background(), logger, localTarget(t, input, ""), cfg, nil,
		&generate.ScanConfig{IDs: []string{"main"}, PolicyDir: t.TempDir()})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.NotEqual(t, "error", result[0].Status)
	assert.Contains(t, logOutput.String(), "excluding workspace from local input")
	assert.Contains(t, logOutput.String(), workspace)
}

func TestScan_WorkspaceAliasWithRealConftest(t *testing.T) {
	requireRealConftest(t)
	t.Chdir(t.TempDir())
	t.Setenv(conftestIgnoreEnv, "")
	t.Setenv(conftestConfigFileEnv, "")
	workspace := t.TempDir()
	input := t.TempDir()
	policy := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(policy, "policy.rego"), []byte(
		"package main\ndeny contains \"blocked configuration\" if input.blocked == true\n",
	), 0600))
	file := filepath.Join(input, "eligible.json")
	require.NoError(t, os.WriteFile(file, []byte(`{"blocked":true}`), 0600))
	workspaceFile := filepath.Join(workspace, "artifact.json")
	require.NoError(t, os.WriteFile(workspaceFile, []byte(`{invalid`), 0600))
	require.NoError(t, os.Symlink(workspaceFile, filepath.Join(input, "alias.json")))
	cfg := config.NewWorkspaceConfig(workspace)
	require.NoError(t, cfg.EnsureDirectories())
	require.NoError(t, generate.WriteScanConfig(
		cfg.GeneratedDirPath(), []string{"main"}, map[string]string{"main": "test-req"}, policy,
	))
	srv := newWorkspaceTestServer(t, ServerOptions{
		WorkspaceDir: workspace, ToolChecker: func() ([]string, error) { return nil, nil },
	})
	target := localTarget(t, input, "")
	response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{target}))
	require.NoError(t, err)
	require.Len(t, response.Errors, 1)
	assert.Contains(t, response.Errors[0], "alias.json")
	assert.Contains(t, response.Errors[0], "resolves to workspace")
	assert.Contains(t, response.Errors[0], workspace)
	assert.Empty(t, response.Assessments)

	response, err = srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{
		localTarget(t, file, ""),
	}))
	require.NoError(t, err)
	assert.Empty(t, response.Errors)
	require.Len(t, response.Assessments, 1)
	assert.Equal(t, provider.ResultFailed, response.Assessments[0].Steps[0].Result)
	assert.Contains(t, response.Assessments[0].Steps[0].Message, "blocked configuration")
}

func TestInspectLocalInput_SafeLinksAndIgnoredWorkspace(t *testing.T) {
	base := t.TempDir()
	input := filepath.Join(base, "input")
	workspace := filepath.Join(input, "workspace")
	external := filepath.Join(base, "external")
	require.NoError(t, os.MkdirAll(workspace, 0750))
	require.NoError(t, os.MkdirAll(external, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "artifact.json"), []byte("{}"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(external, "file.json"), []byte("{}"), 0600))
	require.NoError(t, os.Symlink(external, filepath.Join(input, "external-alias")))
	require.NoError(t, os.Symlink(filepath.Join(external, "file.json"), filepath.Join(workspace, "ignored")))
	pattern, _, err := workspaceIgnorePattern(input, []string{workspace})
	require.NoError(t, err)
	traversal := localTraversal{readDir: os.ReadDir, lstat: os.Lstat}
	assert.NoError(t, inspectInput(context.Background(), input, []string{workspace}, "", pattern, traversal, nil, nil))
	assert.ErrorContains(t, inspectInput(context.Background(), input, []string{workspace}, "", "", traversal, nil, nil), "workspace")
}

func TestInspectLocalInput_ExplicitlyIgnoredAlias(t *testing.T) {
	input := t.TempDir()
	workspace := t.TempDir()
	file := filepath.Join(workspace, "file.json")
	require.NoError(t, os.WriteFile(file, []byte("{}"), 0600))
	alias := filepath.Join(input, "alias.json")
	require.NoError(t, os.Symlink(file, alias))
	traversal := localTraversal{readDir: os.ReadDir, lstat: os.Lstat}
	assert.ErrorContains(t, inspectInput(context.Background(), input, []string{workspace}, "", "", traversal, nil, nil), alias)
	assert.NoError(t, inspectInput(context.Background(), input, []string{workspace}, "",
		"^"+regexp.QuoteMeta(alias)+"$", traversal, nil, nil))
}

func TestScan_TraversalErrorsAreOperational(t *testing.T) {
	for _, test := range []struct{ operation, cause string }{
		{"read directory", "reading workspace directory"},
		{"stat entry", "accessing"},
	} {
		operation := test.operation
		t.Run(operation, func(t *testing.T) {
			workspace := t.TempDir()
			writeScanConfig(t, workspace)
			input := t.TempDir()
			file := filepath.Join(input, "file.json")
			require.NoError(t, os.WriteFile(file, []byte("{}"), 0600))
			runner := conftestRunner(conftestAllSuccessJSON)
			srv := newWorkspaceTestServer(t, ServerOptions{
				Runner: runner, WorkspaceDir: workspace,
				ToolChecker: func() ([]string, error) { return nil, nil },
			})
			injected := errors.New("injected permission failure")
			if operation == "read directory" {
				srv.localTraversal.readDir = func(string) ([]os.DirEntry, error) { return nil, injected }
			} else {
				srv.localTraversal.lstat = func(path string) (os.FileInfo, error) {
					if path == file {
						return nil, injected
					}
					return os.Lstat(path)
				}
			}
			target := localTarget(t, input, "")
			response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{target}))
			require.NoError(t, err)
			require.Len(t, response.Errors, 1)
			assert.Contains(t, response.Errors[0], target.TargetID)
			assert.Contains(t, response.Errors[0], "injected permission failure")
			assert.Contains(t, response.Errors[0], test.cause)
			if operation == "read directory" {
				assert.Contains(t, response.Errors[0], workspace)
			} else {
				assert.Contains(t, response.Errors[0], file)
			}
			assert.Contains(t, response.Errors[0], "fix inaccessible or unsafe paths")
			assert.Empty(t, response.Assessments)
			assert.Empty(t, runner.calls)
		})
	}
}

func TestScan_UnreadableDirectoryIsOperationalError(t *testing.T) {
	workspace := t.TempDir()
	writeScanConfig(t, workspace)
	input := t.TempDir()
	blocked := filepath.Join(input, "unreadable")
	require.NoError(t, os.Mkdir(blocked, 0700))
	require.NoError(t, os.Chmod(blocked, 0000))
	t.Cleanup(func() { assert.NoError(t, os.Chmod(blocked, 0700)) })
	if _, err := os.ReadDir(blocked); !errors.Is(err, os.ErrPermission) {
		t.Skipf("filesystem does not enforce fixture read permissions: %v", err)
	}
	runner := conftestRunner(conftestAllSuccessJSON)
	srv := newWorkspaceTestServer(t, ServerOptions{
		Runner: runner, WorkspaceDir: workspace,
		ToolChecker: func() ([]string, error) { return nil, nil },
	})
	target := localTarget(t, input, "")
	response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{target}))
	require.NoError(t, err)
	require.Len(t, response.Errors, 1)
	assert.Contains(t, response.Errors[0], blocked)
	assert.Contains(t, response.Errors[0], "permission denied")
	assert.Contains(t, response.Errors[0], "fix inaccessible or unsafe paths")
	assert.Empty(t, response.Assessments)
	assert.Empty(t, runner.calls)
}
