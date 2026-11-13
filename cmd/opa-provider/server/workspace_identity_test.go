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
	"github.com/complytime/complytime-providers/cmd/opa-provider/config"
	"github.com/complytime/complytime-providers/cmd/opa-provider/generate"
)

func TestScan_LocalHardLinksAndExternalFiles(t *testing.T) {
	base := t.TempDir()
	workspace := filepath.Join(base, "workspace")
	first := filepath.Join(base, "first")
	second := filepath.Join(base, "second")
	for _, path := range []string{workspace, first, second} {
		require.NoError(t, os.MkdirAll(path, 0750))
	}
	writeScanConfig(t, workspace)
	artifact := filepath.Join(workspace, "artifact.json")
	require.NoError(t, os.WriteFile(artifact, []byte(`{"same":true}`), 0600))
	link := filepath.Join(first, "hardlink.json")
	require.NoError(t, os.Link(artifact, link))
	alias := filepath.Join(second, "indirect.json")
	require.NoError(t, os.Symlink(link, alias))
	ordinary := filepath.Join(second, "ordinary.json")
	require.NoError(t, os.WriteFile(ordinary, []byte(`{"same":true}`), 0600))
	artifactInfo, err := os.Stat(artifact)
	require.NoError(t, err)
	linkInfo, err := os.Stat(link)
	require.NoError(t, err)
	ordinaryInfo, err := os.Stat(ordinary)
	require.NoError(t, err)
	require.True(t, os.SameFile(artifactInfo, linkInfo))
	require.False(t, os.SameFile(artifactInfo, ordinaryInfo))

	for _, test := range []struct {
		name, input string
		reject      bool
	}{
		{"explicit hard link", link, true},
		{"directory hard link", first, true},
		{"separate directory symlink to hard link", second, true},
		{"explicit symlink to hard link", alias, true},
		{"identical content but different identity", ordinary, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := conftestRunner(conftestAllSuccessJSON)
			srv := newWorkspaceTestServer(t, ServerOptions{
				Runner: runner, WorkspaceDir: workspace,
				ToolChecker: func() ([]string, error) { return nil, nil },
			})
			target := localTarget(t, test.input, "")
			response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{target}))
			require.NoError(t, err)
			if test.reject {
				require.Len(t, response.Errors, 1)
				assert.Contains(t, response.Errors[0], target.TargetID)
				assert.Contains(t, response.Errors[0], artifact)
				assert.Contains(t, response.Errors[0], "shares filesystem identity")
				assert.Contains(t, response.Errors[0], "copy intended input")
				assert.Empty(t, response.Assessments)
				assert.Empty(t, runner.calls)
			} else {
				assert.Empty(t, response.Errors)
				require.Len(t, response.Assessments, 1)
				assert.Equal(t, "test-req", response.Assessments[0].RequirementID)
				require.Len(t, runner.calls, 1)
			}
		})
	}
}

func TestScan_IgnoredHardLinkWithRealConftest(t *testing.T) {
	requireRealConftest(t)
	t.Chdir(t.TempDir())
	t.Setenv(conftestIgnoreEnv, `alias\.json$`)
	t.Setenv(conftestConfigFileEnv, "")
	input := t.TempDir()
	workspace := filepath.Join(input, "workspace")
	policy := t.TempDir()
	require.NoError(t, os.Mkdir(workspace, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(policy, "policy.rego"), []byte(
		"package main\ndeny contains \"blocked configuration\" if input.blocked == true\n",
	), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(input, "eligible.json"), []byte(`{"blocked":true}`), 0600))
	artifact := filepath.Join(workspace, "artifact.json")
	require.NoError(t, os.WriteFile(artifact, []byte(`{invalid`), 0600))
	require.NoError(t, os.Link(artifact, filepath.Join(input, "alias.json")))
	cfg := config.NewWorkspaceConfig(workspace)
	require.NoError(t, cfg.EnsureDirectories())
	require.NoError(t, generate.WriteScanConfig(
		cfg.GeneratedDirPath(), []string{"main"}, map[string]string{"main": "test-req"}, policy,
	))
	srv := newWorkspaceTestServer(t, ServerOptions{
		WorkspaceDir: workspace, ToolChecker: func() ([]string, error) { return nil, nil },
	})
	response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{
		localTarget(t, input, ""),
	}))
	require.NoError(t, err)
	assert.Empty(t, response.Errors)
	require.Len(t, response.Assessments, 1)
	require.Len(t, response.Assessments[0].Steps, 1)
	assert.Equal(t, provider.ResultFailed, response.Assessments[0].Steps[0].Result)
	assert.Contains(t, response.Assessments[0].Steps[0].Message, "blocked configuration")
}

func TestScan_IdentityAndWorkspaceTraversalErrors(t *testing.T) {
	for _, scenario := range []string{"workspace identity", "target identity", "workspace traversal"} {
		t.Run(scenario, func(t *testing.T) {
			workspace := t.TempDir()
			writeScanConfig(t, workspace)
			workspaceFile := filepath.Join(workspace, "workspace-file.json")
			input := filepath.Join(t.TempDir(), "target-file.json")
			require.NoError(t, os.WriteFile(workspaceFile, []byte("{}"), 0600))
			require.NoError(t, os.WriteFile(input, []byte("{}"), 0600))
			runner := conftestRunner(conftestAllSuccessJSON)
			srv := newWorkspaceTestServer(t, ServerOptions{
				Runner: runner, WorkspaceDir: workspace,
				ToolChecker: func() ([]string, error) { return nil, nil },
			})
			injected := errors.New("injected failure")
			if scenario == "workspace traversal" {
				srv.localTraversal.readDir = func(path string) ([]os.DirEntry, error) {
					if path == workspace {
						return nil, injected
					}
					return os.ReadDir(path)
				}
			} else {
				name := filepath.Base(workspaceFile)
				if scenario == "target identity" {
					name = filepath.Base(input)
				}
				srv.identifyFile = func(path string, info os.FileInfo) (fileIdentity, error) {
					if info.Name() == name {
						return fileIdentity{}, injected
					}
					return fileIdentityOf(path, info)
				}
			}
			target := localTarget(t, input, "")
			response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{target}))
			require.NoError(t, err)
			require.Len(t, response.Errors, 1)
			assert.Contains(t, response.Errors[0], target.TargetID)
			assert.Contains(t, response.Errors[0], "injected failure")
			assert.Contains(t, response.Errors[0], "fix inaccessible or unsafe paths")
			assert.Empty(t, response.Assessments)
			assert.Empty(t, runner.calls)
		})
	}
}

func TestWorkspaceIdentities_OmitClonePathsNotIdentities(t *testing.T) {
	workspace := t.TempDir()
	clone := filepath.Join(workspace, "clone")
	require.NoError(t, os.Mkdir(clone, 0750))
	cloneFile := filepath.Join(clone, "clone.json")
	require.NoError(t, os.WriteFile(cloneFile, []byte("{}"), 0600))
	outsideClone := filepath.Join(workspace, "other.json")
	require.NoError(t, os.Link(cloneFile, outsideClone))
	traversal := localTraversal{readDir: os.ReadDir, lstat: os.Lstat}
	identities, err := workspaceIdentities(context.Background(), []string{workspace}, clone, traversal, fileIdentityOf)
	require.NoError(t, err)
	info, err := os.Stat(cloneFile)
	require.NoError(t, err)
	identity, err := fileIdentityOf(cloneFile, info)
	require.NoError(t, err)
	assert.Equal(t, outsideClone, identities[identity])
	require.NoError(t, os.Remove(outsideClone))
	identities, err = workspaceIdentities(context.Background(), []string{workspace}, clone, traversal, fileIdentityOf)
	require.NoError(t, err)
	assert.NotContains(t, identities, identity)
}

func TestWorkspaceIdentities_DirectoryCycleRetainsRegularFiles(t *testing.T) {
	workspace := t.TempDir()
	link := filepath.Join(workspace, "link")
	require.NoError(t, os.Symlink(workspace, link))
	file := filepath.Join(workspace, "z-artifact.json")
	require.NoError(t, os.WriteFile(file, []byte(`{}`), 0600))
	identities, err := workspaceIdentities(context.Background(), []string{workspace}, "",
		localTraversal{readDir: os.ReadDir, lstat: os.Lstat}, fileIdentityOf)
	require.NoError(t, err)
	info, err := os.Stat(file)
	require.NoError(t, err)
	identity, err := fileIdentityOf(file, info)
	require.NoError(t, err)
	assert.Equal(t, map[fileIdentity]string{identity: file}, identities)
}

func TestWorkspaceIdentities_ExternalSymlinksAreNotCollected(t *testing.T) {
	workspace := t.TempDir()
	external := t.TempDir()
	externalFile := filepath.Join(external, "ordinary.json")
	require.NoError(t, os.WriteFile(externalFile, []byte("{}"), 0600))
	workspaceFile := filepath.Join(workspace, "artifact.json")
	require.NoError(t, os.WriteFile(workspaceFile, []byte("{}"), 0600))
	oldClone := filepath.Join(workspace, "old-clone")
	require.NoError(t, os.Mkdir(oldClone, 0750))
	for name, destination := range map[string]string{
		"outside-dir": external, "outside-file": externalFile, "filesystem-root": string(filepath.Separator),
	} {
		require.NoError(t, os.Symlink(destination, filepath.Join(oldClone, name)))
	}
	traversal := localTraversal{lstat: os.Lstat, readDir: func(path string) ([]os.DirEntry, error) {
		assert.Contains(t, []string{workspace, oldClone}, path, "external directories must never be traversed")
		return os.ReadDir(path)
	}}
	identities, err := workspaceIdentities(context.Background(), []string{workspace}, "", traversal, fileIdentityOf)
	require.NoError(t, err)
	info, err := os.Stat(externalFile)
	require.NoError(t, err)
	identity, err := fileIdentityOf(externalFile, info)
	require.NoError(t, err)
	assert.NotContains(t, identities, identity)
	assert.Len(t, identities, 1)
	writeScanConfig(t, workspace)
	runner := conftestRunner(conftestAllSuccessJSON)
	srv := newWorkspaceTestServer(t, ServerOptions{Runner: runner, WorkspaceDir: workspace,
		ToolChecker: func() ([]string, error) { return nil, nil }})
	response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{
		localTarget(t, externalFile, ""),
	}))
	require.NoError(t, err)
	assert.Empty(t, response.Errors)
	require.Len(t, response.Assessments, 1)
	assert.Equal(t, provider.ResultPassed, response.Assessments[0].Steps[0].Result)
}

func TestWorkspaceIdentities_TraversesLinksToOtherBoundary(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	file := filepath.Join(second, "artifact.json")
	require.NoError(t, os.WriteFile(file, []byte("{}"), 0600))
	link := filepath.Join(first, "other-workspace")
	require.NoError(t, os.Symlink(second, link))
	identities, err := workspaceIdentities(context.Background(), []string{first, second}, "",
		localTraversal{readDir: os.ReadDir, lstat: os.Lstat}, fileIdentityOf)
	require.NoError(t, err)
	info, err := os.Stat(file)
	require.NoError(t, err)
	id, err := fileIdentityOf(file, info)
	require.NoError(t, err)
	assert.Contains(t, identities, id)
}

func TestWorkspaceIdentities_MissingEntriesAndTraversalFailures(t *testing.T) {
	workspace := t.TempDir()
	child := filepath.Join(workspace, "directory")
	require.NoError(t, os.Mkdir(child, 0750))
	for _, test := range []struct {
		name       string
		path       string
		err        error
		wantReject bool
	}{
		{"missing child", child, os.ErrNotExist, false},
		{"unreadable child", child, os.ErrPermission, true},
		{"missing boundary", workspace, os.ErrNotExist, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			traversal := localTraversal{lstat: os.Lstat, readDir: func(path string) ([]os.DirEntry, error) {
				if path == test.path {
					return nil, test.err
				}
				return os.ReadDir(path)
			}}
			_, err := workspaceIdentities(context.Background(), []string{workspace}, "", traversal, fileIdentityOf)
			if test.wantReject {
				require.ErrorIs(t, err, test.err)
				assert.Contains(t, err.Error(), test.path)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWorkspaceIdentities_LinkTargetStatErrors(t *testing.T) {
	workspace := t.TempDir()
	target := filepath.Join(workspace, "z-file.json")
	require.NoError(t, os.WriteFile(target, []byte("{}"), 0600))
	link := filepath.Join(workspace, "a-link.json")
	require.NoError(t, os.Symlink(target, link))
	for _, test := range []struct {
		name       string
		err        error
		wantReject bool
	}{
		{"missing target", os.ErrNotExist, false},
		{"inaccessible target", os.ErrPermission, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			linkChecked := false
			traversal := localTraversal{readDir: os.ReadDir, lstat: func(path string) (os.FileInfo, error) {
				if path == link {
					linkChecked = true
				}
				if path == target && linkChecked {
					linkChecked = false
					return nil, test.err
				}
				return os.Lstat(path)
			}}
			identities, err := workspaceIdentities(context.Background(), []string{workspace}, "", traversal, fileIdentityOf)
			if test.wantReject {
				require.ErrorIs(t, err, test.err)
				assert.Contains(t, err.Error(), link)
			} else {
				require.NoError(t, err)
				assert.Len(t, identities, 1)
			}
		})
	}
}

func TestWorkspaceIdentities_DanglingLinksDoNotBlockOtherTargets(t *testing.T) {
	workspace := t.TempDir()
	writeScanConfig(t, workspace)
	oldClone := filepath.Join(workspace, "old-clone")
	require.NoError(t, os.Mkdir(oldClone, 0750))
	require.NoError(t, os.Symlink(filepath.Join(workspace, "missing"), filepath.Join(oldClone, "dangling")))
	input := filepath.Join(t.TempDir(), "input.json")
	require.NoError(t, os.WriteFile(input, []byte("{}"), 0600))
	runner := conftestRunner(conftestAllSuccessJSON)
	srv := newWorkspaceTestServer(t, ServerOptions{Runner: runner, WorkspaceDir: workspace,
		ToolChecker: func() ([]string, error) { return nil, nil }})
	response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{
		localTarget(t, input, ""),
	}))
	require.NoError(t, err)
	assert.Empty(t, response.Errors)
	require.Len(t, response.Assessments, 1)
	assert.Equal(t, provider.ResultPassed, response.Assessments[0].Steps[0].Result)
}

func TestWorkspaceIdentities_VisitsSymlinkDiamondOnce(t *testing.T) {
	workspace := t.TempDir()
	dir := filepath.Join(workspace, "data")
	require.NoError(t, os.Mkdir(dir, 0750))
	file := filepath.Join(dir, "artifact.json")
	require.NoError(t, os.WriteFile(file, []byte("{}"), 0600))
	for _, name := range []string{"link-a", "link-b"} {
		require.NoError(t, os.Symlink(dir, filepath.Join(workspace, name)))
	}
	count := 0
	traversal := localTraversal{lstat: os.Lstat, readDir: func(path string) ([]os.DirEntry, error) {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return nil, err
		}
		if resolved == dir {
			count++
		}
		return os.ReadDir(path)
	}}
	identities, err := workspaceIdentities(context.Background(), []string{workspace}, "", traversal, fileIdentityOf)
	require.NoError(t, err)
	assert.Len(t, identities, 1)
	assert.Equal(t, 1, count)
}

func TestWorkspaceIdentities_AliasRetainsIdentityOutsideSelectedClone(t *testing.T) {
	workspace := t.TempDir()
	parent := filepath.Join(workspace, "a-parent")
	clone := filepath.Join(parent, "clone")
	require.NoError(t, os.MkdirAll(clone, 0750))
	file := filepath.Join(clone, "file.json")
	require.NoError(t, os.WriteFile(file, []byte("{}"), 0600))
	alias := filepath.Join(workspace, "z-alias")
	require.NoError(t, os.Symlink(parent, alias))
	identities, err := workspaceIdentities(context.Background(), []string{workspace}, clone,
		localTraversal{readDir: os.ReadDir, lstat: os.Lstat}, fileIdentityOf)
	require.NoError(t, err)
	info, err := os.Stat(file)
	require.NoError(t, err)
	id, err := fileIdentityOf(file, info)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(alias, "clone", "file.json"), identities[id])
}

func TestWorkspaceIdentities_CycleRetainsIdentityOutsideSelectedClone(t *testing.T) {
	workspace := t.TempDir()
	clone := filepath.Join(workspace, "clone")
	require.NoError(t, os.Mkdir(clone, 0750))
	file := filepath.Join(clone, "file.json")
	require.NoError(t, os.WriteFile(file, []byte(`{}`), 0600))
	alias := filepath.Join(workspace, "a-cycle")
	require.NoError(t, os.Symlink(workspace, alias))
	traversal := localTraversal{readDir: os.ReadDir, lstat: os.Lstat}
	identities, err := workspaceIdentities(context.Background(), []string{workspace}, clone,
		traversal, fileIdentityOf)
	require.NoError(t, err)
	info, err := os.Stat(file)
	require.NoError(t, err)
	identity, err := fileIdentityOf(file, info)
	require.NoError(t, err)
	assert.Equal(t, map[fileIdentity]string{
		identity: filepath.Join(alias, "clone", "file.json"),
	}, identities)
	err = inspectInput(context.Background(), clone, []string{workspace}, clone, "",
		traversal, identities, fileIdentityOf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "shares filesystem identity")
	assert.Contains(t, err.Error(), file)
}
