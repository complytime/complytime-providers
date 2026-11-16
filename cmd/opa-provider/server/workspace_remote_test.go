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
	"github.com/complytime/complytime-providers/cmd/opa-provider/loader"
)

func TestScan_RemoteCloneAliases(t *testing.T) {
	for _, test := range []struct {
		name, cause string
		workspace   bool
		identity    bool
	}{
		{"workspace file symlink", "resolves to workspace", true, false},
		{"workspace directory symlink", "resolves to workspace", true, false},
		{"scan path alias", "resolves to workspace", true, false},
		{"scan path through alias", "resolves to workspace", true, false},
		{"external hardlink symlink", "shares filesystem identity", false, true},
		{"external directory hardlink", "shares filesystem identity", false, true},
		{"clone hardlink", "shares filesystem identity", false, true},
		{"directory cycle", "directory symlink cycle", false, false},
		{"symlink loop", "too many links", false, false},
		{"broken symlink", "no such file or directory", false, false},
		{"internal file symlink", "", false, false},
		{"internal directory symlink", "", false, false},
		{"ordinary clone", "", false, false},
	} {
		scenario := test.name
		t.Run(scenario, func(t *testing.T) {
			workspace := t.TempDir()
			writeScanConfig(t, workspace)
			clone := filepath.Join(workspace, "repos", "selected")
			require.NoError(t, os.MkdirAll(filepath.Join(clone, "nested"), 0750))
			cloneFile := filepath.Join(clone, "nested", "eligible.json")
			require.NoError(t, os.WriteFile(cloneFile, []byte(`{"blocked":true}`), 0600))
			outside := filepath.Join(workspace, "other")
			require.NoError(t, os.Mkdir(outside, 0750))
			workspaceFile := filepath.Join(outside, "artifact.json")
			require.NoError(t, os.WriteFile(workspaceFile, []byte(`{"blocked":true}`), 0600))
			alias := filepath.Join(clone, "alias")
			scanPath := ""
			switch scenario {
			case "workspace file symlink":
				require.NoError(t, os.Symlink(workspaceFile, alias))
			case "workspace directory symlink":
				require.NoError(t, os.Symlink(outside, alias))
			case "scan path alias":
				require.NoError(t, os.Symlink(workspaceFile, alias))
				scanPath = "alias"
			case "scan path through alias":
				require.NoError(t, os.Symlink(outside, alias))
				scanPath = filepath.Join("alias", "artifact.json")
			case "external hardlink symlink":
				external := filepath.Join(t.TempDir(), "external.json")
				require.NoError(t, os.Link(workspaceFile, external))
				require.NoError(t, os.Symlink(external, alias))
			case "external directory hardlink":
				external := t.TempDir()
				require.NoError(t, os.Link(workspaceFile, filepath.Join(external, "external.json")))
				require.NoError(t, os.Symlink(external, alias))
			case "clone hardlink":
				require.NoError(t, os.Remove(cloneFile))
				require.NoError(t, os.Link(workspaceFile, cloneFile))
			case "directory cycle":
				require.NoError(t, os.Symlink(clone, alias))
			case "symlink loop":
				require.NoError(t, os.Symlink(alias, alias))
			case "broken symlink":
				require.NoError(t, os.Symlink(filepath.Join(workspace, "missing"), alias))
			case "internal file symlink":
				require.NoError(t, os.Symlink(cloneFile, alias))
			case "internal directory symlink":
				require.NoError(t, os.Symlink(filepath.Join(clone, "nested"), alias))
			}
			input := filepath.Join(clone, scanPath)
			ldr := &mockLoader{loadFn: func(_ provider.Target, _ string) (string, error) {
				return input, nil
			}}
			runner := conftestRunner(`[{"filename":"eligible.json","namespace":"main",` +
				`"failures":[{"msg":"blocked", "metadata":{"query":"data.main.deny"}}]}]`)
			srv := newWorkspaceTestServer(t, ServerOptions{
				Runner: runner, Loader: ldr, WorkspaceDir: workspace,
				ToolChecker: func() ([]string, error) { return nil, nil },
			})
			target := remoteTarget("", "https://example.com/org/repo")
			target.Variables[loader.VarScanPath] = scanPath
			response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{target}))
			require.NoError(t, err)
			if test.cause == "" {
				assert.Empty(t, response.Errors)
				require.Len(t, response.Assessments, 1)
				assert.Equal(t, "test-req", response.Assessments[0].RequirementID)
				require.Len(t, response.Assessments[0].Steps, 1)
				assert.Equal(t, provider.ResultFailed, response.Assessments[0].Steps[0].Result)
				require.Len(t, runner.calls, 1)
				assert.Equal(t, input, runner.calls[0].args[1])
			} else {
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
				assert.Empty(t, runner.calls)
			}
		})
	}
}

func TestScan_RemoteCloneRejectsInjectedWorkspaceAlias(t *testing.T) {
	workspace := t.TempDir()
	clone := filepath.Join(workspace, "clone")
	require.NoError(t, os.Mkdir(clone, 0750))
	hostBase := t.TempDir()
	writeScanConfig(t, hostBase)
	hostWorkspace := filepath.Join(hostBase, provider.WorkspaceDir)
	require.NoError(t, os.MkdirAll(hostWorkspace, 0750))
	artifact := filepath.Join(hostWorkspace, "artifact.json")
	require.NoError(t, os.WriteFile(artifact, []byte("{}"), 0600))
	require.NoError(t, os.Link(artifact, filepath.Join(clone, "hardlink.json")))
	runner := conftestRunner(conftestAllSuccessJSON)
	srv := newWorkspaceTestServer(t, ServerOptions{
		Runner: runner, WorkspaceDir: workspace,
		Loader:      &mockLoader{loadFn: func(provider.Target, string) (string, error) { return clone, nil }},
		ToolChecker: func() ([]string, error) { return nil, nil },
	})
	target := remoteTarget("", "https://example.com/org/repo")
	target.Variables[loader.VarWorkspace] = hostBase
	response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{target}))
	require.NoError(t, err)
	require.Len(t, response.Errors, 1)
	assert.Contains(t, response.Errors[0], target.TargetID)
	assert.Contains(t, response.Errors[0], artifact)
	assert.Contains(t, response.Errors[0], "shares filesystem identity")
	assert.Empty(t, response.Assessments)
	assert.Empty(t, runner.calls)
}

func TestRemoteCloneRoot_ValidatesLoaderPath(t *testing.T) {
	root := t.TempDir()
	got, err := remoteCloneRoot(filepath.Join(root, "a", "b"), filepath.Join("a", "b"))
	require.NoError(t, err)
	assert.Equal(t, root, got)
	got, err = remoteCloneRoot(root, "")
	require.NoError(t, err)
	assert.Equal(t, root, got)
	_, err = remoteCloneRoot(filepath.Join(root, "a", "b"), "different")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scan_path to a subdirectory of the cloned repository")
	assert.Contains(t, err.Error(), "matches the cloned input path")
	got, err = remoteCloneRoot(filepath.Join(root, "absolute"), filepath.Join(string(filepath.Separator), "absolute"))
	require.NoError(t, err)
	assert.Equal(t, root, got)
	_, err = remoteCloneRoot(root, filepath.Join("..", "escape"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scan_path to a subdirectory of the cloned repository")
	assert.Contains(t, err.Error(), "matches the cloned input path")
}

func TestScan_RemoteCloneRootMismatchHasRemedy(t *testing.T) {
	workspace := t.TempDir()
	writeScanConfig(t, workspace)
	input := t.TempDir()
	runner := conftestRunner(conftestAllSuccessJSON)
	srv := newWorkspaceTestServer(t, ServerOptions{Runner: runner, WorkspaceDir: workspace,
		Loader:      &mockLoader{loadFn: func(provider.Target, string) (string, error) { return input, nil }},
		ToolChecker: func() ([]string, error) { return nil, nil }})
	target := remoteTarget("", "https://example.com/org/repo")
	target.Variables[loader.VarScanPath] = "configs"
	response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{target}))
	require.NoError(t, err)
	require.Len(t, response.Errors, 1)
	assert.Contains(t, response.Errors[0], target.TargetID)
	assert.Contains(t, response.Errors[0], "cannot establish remote clone root")
	assert.Contains(t, response.Errors[0], "scan_path to a subdirectory of the cloned repository")
	assert.Contains(t, response.Errors[0], "matches the cloned input path")
	assert.Empty(t, response.Assessments)
	assert.Empty(t, runner.calls)
}

func TestScan_RemoteWorkspaceInspectionErrors(t *testing.T) {
	for _, test := range []struct{ name, cause string }{
		{"workspace traversal", "reading workspace directory"},
		{"workspace identity", "identifying workspace file"},
		{"clone identity", "identifying input file"},
	} {
		scenario := test.name
		t.Run(scenario, func(t *testing.T) {
			workspace := t.TempDir()
			writeScanConfig(t, workspace)
			clone := filepath.Join(workspace, "clone")
			require.NoError(t, os.Mkdir(clone, 0750))
			workspaceFile := filepath.Join(workspace, "workspace-file.json")
			cloneFile := filepath.Join(clone, "clone-file.json")
			require.NoError(t, os.WriteFile(workspaceFile, []byte("{}"), 0600))
			require.NoError(t, os.WriteFile(cloneFile, []byte("{}"), 0600))
			runner := conftestRunner(conftestAllSuccessJSON)
			srv := newWorkspaceTestServer(t, ServerOptions{
				Runner: runner, WorkspaceDir: workspace,
				Loader: &mockLoader{loadFn: func(provider.Target, string) (string, error) {
					return clone, nil
				}},
				ToolChecker: func() ([]string, error) { return nil, nil },
			})
			injected := errors.New("injected remote inspection failure")
			if scenario == "workspace traversal" {
				srv.localTraversal.readDir = func(path string) ([]os.DirEntry, error) {
					if path == workspace {
						return nil, injected
					}
					return os.ReadDir(path)
				}
			} else {
				failedPath := workspaceFile
				if scenario == "clone identity" {
					failedPath = cloneFile
				}
				srv.identifyFile = func(path string, info os.FileInfo) (fileIdentity, error) {
					if path == failedPath {
						return fileIdentity{}, injected
					}
					return fileIdentityOf(path, info)
				}
			}
			target := remoteTarget("", "https://example.com/org/repo")
			response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{target}))
			require.NoError(t, err)
			require.Len(t, response.Errors, 1)
			assert.Contains(t, response.Errors[0], target.TargetID)
			assert.Contains(t, response.Errors[0], "injected remote inspection failure")
			assert.Contains(t, response.Errors[0], test.cause)
			switch scenario {
			case "workspace traversal":
				assert.Contains(t, response.Errors[0], workspace)
			case "workspace identity":
				assert.Contains(t, response.Errors[0], workspaceFile)
			case "clone identity":
				assert.Contains(t, response.Errors[0], cloneFile)
			}
			assert.Contains(t, response.Errors[0], "fix inaccessible or unsafe paths")
			assert.Empty(t, response.Assessments)
			assert.Empty(t, runner.calls)
		})
	}
}
