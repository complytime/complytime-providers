// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/complytime/complytime-providers/cmd/opa-provider/loader"
)

func TestScan_PreflightCancellationDoesNotProduceAssessments(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		remote bool
		stage  string
	}{
		{"local before preflight", false, "before"},
		{"local workspace traversal", false, "workspace"},
		{"local input traversal", false, "input"},
		{"remote before preflight", true, "before"},
		{"remote workspace traversal", true, "workspace"},
		{"remote input traversal", true, "input"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			workspace := t.TempDir()
			writeScanConfig(t, workspace)
			workspaceFile := filepath.Join(workspace, "artifact.json")
			require.NoError(t, os.WriteFile(workspaceFile, []byte("{}"), 0600))
			input := t.TempDir()
			if scenario.remote {
				input = filepath.Join(workspace, "selected-clone")
				require.NoError(t, os.Mkdir(input, 0750))
			}
			inputFile := filepath.Join(input, "input.json")
			require.NoError(t, os.WriteFile(inputFile, []byte("{}"), 0600))
			runner := conftestRunner(conftestAllSuccessJSON)
			srv := newWorkspaceTestServer(t, ServerOptions{
				Runner: runner, WorkspaceDir: workspace,
				Loader:      &mockLoader{loadFn: func(provider.Target, string) (string, error) { return input, nil }},
				ToolChecker: func() ([]string, error) { return nil, nil },
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			visited := false
			if scenario.stage == "before" {
				cancel()
			} else {
				cancelPath := workspaceFile
				if scenario.stage == "input" {
					cancelPath = inputFile
				}
				srv.localTraversal.lstat = func(path string) (os.FileInfo, error) {
					info, err := os.Lstat(path)
					if path == cancelPath {
						visited = true
						cancel()
					}
					return info, err
				}
			}
			target := localTarget(t, input, "")
			if scenario.remote {
				target = remoteTarget("", "https://example.com/org/repo")
			}
			response, err := srv.Scan(ctx, makeScanRequest(t, []provider.Target{target}))
			require.NoError(t, err)
			if scenario.stage != "before" {
				assert.True(t, visited, "cancellation must occur during the intended traversal")
			}
			require.Len(t, response.Errors, 1)
			assert.Contains(t, response.Errors[0], target.TargetID)
			assert.Contains(t, response.Errors[0], "context canceled")
			assert.Contains(t, response.Errors[0], "retry the scan")
			assert.NotContains(t, response.Errors[0], "fix inaccessible")
			assert.NotContains(t, response.Errors[0], "cannot safely inspect")
			assert.Empty(t, response.Assessments)
			assert.Empty(t, runner.calls)
		})
	}
}

func TestPreflight_CancellationDuringFinalDirectoryRead(t *testing.T) {
	for _, stage := range []string{"workspace", "input"} {
		t.Run(stage, func(t *testing.T) {
			dir := t.TempDir()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			traversal := localTraversal{
				lstat: os.Lstat,
				readDir: func(path string) ([]os.DirEntry, error) {
					entries, err := os.ReadDir(path)
					cancel()
					return entries, err
				},
			}
			if stage == "workspace" {
				identities, err := workspaceIdentities(ctx, []string{dir}, "", traversal, fileIdentityOf)
				require.ErrorIs(t, err, context.Canceled)
				assert.NotContains(t, err.Error(), "fix inaccessible")
				assert.Nil(t, identities)
			} else {
				err := inspectInput(ctx, dir, nil, "", "", traversal, nil, nil)
				require.ErrorIs(t, err, context.Canceled)
				assert.NotContains(t, err.Error(), "fix inaccessible")
			}
		})
	}
}

func TestPreflight_CancellationDuringFinalIdentity(t *testing.T) {
	for _, stage := range []string{"workspace", "input"} {
		t.Run(stage, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "file.json")
			require.NoError(t, os.WriteFile(file, []byte("{}"), 0600))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			identify := func(path string, info os.FileInfo) (fileIdentity, error) {
				identity, err := fileIdentityOf(path, info)
				cancel()
				return identity, err
			}
			traversal := localTraversal{lstat: os.Lstat, readDir: os.ReadDir}
			if stage == "workspace" {
				identities, err := workspaceIdentities(ctx, []string{file}, "", traversal, identify)
				require.ErrorIs(t, err, context.Canceled)
				assert.Nil(t, identities)
			} else {
				err := inspectInput(ctx, file, nil, "", "", traversal, map[fileIdentity]string{}, identify)
				require.ErrorIs(t, err, context.Canceled)
			}
		})
	}
}

func TestPreflight_CancellationErrorsHaveNoFilesystemRemedy(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(fmt.Sprintf("deadline=%t", deadline), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			expected := context.Canceled
			if deadline {
				cancel()
				ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
				expected = context.DeadlineExceeded
			} else {
				cancel()
			}
			defer cancel()
			dir := t.TempDir()
			traversal := localTraversal{lstat: os.Lstat, readDir: os.ReadDir}
			_, err := workspaceIdentities(ctx, []string{dir}, "", traversal, fileIdentityOf)
			require.ErrorIs(t, err, expected)
			assert.NotContains(t, err.Error(), "fix inaccessible")
			assert.NotContains(t, err.Error(), "cannot inspect workspace")
			err = inspectInput(ctx, dir, nil, "", "", traversal, nil, nil)
			require.ErrorIs(t, err, expected)
			assert.NotContains(t, err.Error(), "fix inaccessible")
			assert.NotContains(t, err.Error(), "cannot safely inspect input")
		})
	}
}

func TestScan_CancellationAfterPreflightDoesNotEvaluate(t *testing.T) {
	workspace := t.TempDir()
	input := filepath.Join(t.TempDir(), "input.json")
	require.NoError(t, os.WriteFile(input, []byte("{}"), 0600))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner := &mockRunner{callFn: func(name string, args []string) ([]byte, error) {
		require.Equal(t, "conftest", name)
		require.Equal(t, "pull", args[0])
		cancel()
		return []byte("ok"), nil
	}}
	srv := newWorkspaceTestServer(t, ServerOptions{
		Runner: runner, WorkspaceDir: workspace,
		ToolChecker: func() ([]string, error) { return nil, nil },
	})
	target := localTarget(t, input, "ghcr.io/org/bundle:dev")
	response, err := srv.Scan(ctx, makeScanRequest(t, []provider.Target{target}))
	require.NoError(t, err)
	require.Len(t, response.Errors, 1)
	assert.Contains(t, response.Errors[0], target.TargetID)
	assert.Contains(t, response.Errors[0], "context canceled")
	assert.Empty(t, response.Assessments)
	require.Len(t, runner.calls, 1)
	assert.Equal(t, "pull", runner.calls[0].args[0])
}

func TestScan_RemoteBranchCancellationDiscardsEarlierAssessments(t *testing.T) {
	workspace := t.TempDir()
	writeScanConfig(t, workspace)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner := conftestRunner(conftestAllSuccessJSON)
	ldr := &mockLoader{loadFn: func(target provider.Target, _ string) (string, error) {
		clone := filepath.Join(workspace, target.Variables[loader.VarBranch])
		require.NoError(t, os.MkdirAll(clone, 0750))
		require.NoError(t, os.WriteFile(filepath.Join(clone, "input.json"), []byte("{}"), 0600))
		return clone, nil
	}}
	srv := newWorkspaceTestServer(t, ServerOptions{
		Runner: runner, Loader: ldr, WorkspaceDir: workspace,
		ToolChecker: func() ([]string, error) { return nil, nil },
	})
	cancelPath := filepath.Join(workspace, "second", "input.json")
	cancelledDuringSecondBranch := false
	srv.localTraversal.lstat = func(path string) (os.FileInfo, error) {
		info, err := os.Lstat(path)
		if path == cancelPath {
			cancelledDuringSecondBranch = true
			cancel()
		}
		return info, err
	}
	target := remoteTarget("", "https://example.com/org/repo")
	target.Variables[loader.VarBranches] = "main,second"
	response, err := srv.Scan(ctx, makeScanRequest(t, []provider.Target{target}))
	require.NoError(t, err)
	assert.True(t, cancelledDuringSecondBranch)
	require.Len(t, response.Errors, 1)
	assert.Contains(t, response.Errors[0], "context canceled")
	assert.Contains(t, response.Errors[0], target.TargetID)
	assert.Empty(t, response.Assessments)
	require.Len(t, runner.calls, 1)
	assert.Equal(t, "test", runner.calls[0].args[0])
}

func TestScan_ExpiredDeadlineIsOperational(t *testing.T) {
	workspace := t.TempDir()
	writeScanConfig(t, workspace)
	input := t.TempDir()
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	runner := conftestRunner(conftestAllSuccessJSON)
	srv := newWorkspaceTestServer(t, ServerOptions{
		Runner: runner, WorkspaceDir: workspace,
		ToolChecker: func() ([]string, error) { return nil, nil },
	})
	target := localTarget(t, input, "")
	response, err := srv.Scan(ctx, makeScanRequest(t, []provider.Target{target}))
	require.NoError(t, err)
	require.Len(t, response.Errors, 1)
	assert.Contains(t, response.Errors[0], target.TargetID)
	assert.Contains(t, response.Errors[0], "context deadline exceeded")
	assert.Contains(t, response.Errors[0], "retry the scan")
	assert.Empty(t, response.Assessments)
	assert.Empty(t, runner.calls)
}
