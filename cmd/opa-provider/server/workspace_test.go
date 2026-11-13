// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/complytime/complytime-providers/cmd/opa-provider/loader"
)

func TestWorkspaceBoundaries_ResolvedPaths(t *testing.T) {
	base := t.TempDir()
	hostBase := filepath.Join(base, "host [a]+")
	hostWorkspace := filepath.Join(hostBase, provider.WorkspaceDir)
	customWorkspace := filepath.Join(base, "custom (work)+")
	require.NoError(t, os.MkdirAll(hostWorkspace, 0750))
	require.NoError(t, os.MkdirAll(customWorkspace, 0750))
	workingDir, err := os.Getwd()
	require.NoError(t, err)
	relativeBase, err := filepath.Rel(workingDir, hostBase)
	require.NoError(t, err)
	relativeCustom, err := filepath.Rel(workingDir, customWorkspace)
	require.NoError(t, err)

	tests := []struct {
		name       string
		variables  map[string]string
		configured string
		want       []string
	}{
		{"different directories", map[string]string{loader.VarWorkspace: hostBase}, customWorkspace,
			[]string{hostWorkspace, customWorkspace}},
		{"relative directories", map[string]string{loader.VarWorkspace: relativeBase}, relativeCustom,
			[]string{hostWorkspace, customWorkspace}},
		{"same directory", map[string]string{loader.VarWorkspace: hostBase}, hostWorkspace,
			[]string{hostWorkspace}},
		{"missing variable uses configured", nil, customWorkspace, []string{customWorkspace}},
		{"empty variable uses configured", map[string]string{loader.VarWorkspace: ""}, customWorkspace,
			[]string{customWorkspace}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := workspaceBoundaries(test.variables, test.configured)
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestPathWithin_ComponentRelationships(t *testing.T) {
	base := filepath.Join(t.TempDir(), "host [a]+")
	workspace := filepath.Join(base, provider.WorkspaceDir)
	tests := []struct {
		name string
		root string
		path string
		want bool
	}{
		{"equal", workspace, workspace, true},
		{"descendant", workspace, filepath.Join(workspace, "opa", "result.json"), true},
		{"enclosing target", base, workspace, true},
		{"enclosing target is not inside workspace", workspace, base, false},
		{"similar sibling", workspace, filepath.Join(base, ".complytime-backup"), false},
		{"unrelated directory", workspace, filepath.Join(base, "input"), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := pathWithin(test.root, test.path)
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestWorkspaceBoundaries_UnresolvableWorkspace(t *testing.T) {
	_, err := workspaceBoundaries(nil, filepath.Join(t.TempDir(), "missing"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolving workspace")
}

func TestValidateLocalWorkspaceTarget_UnresolvableInputHasRemedy(t *testing.T) {
	workspace := t.TempDir()
	input := filepath.Join(t.TempDir(), "missing")
	err := validateLocalWorkspaceTarget(input, nil, workspace)
	require.Error(t, err)
	assert.Contains(t, err.Error(), input)
	assert.Contains(t, err.Error(), "check input path and filesystem access")
}

func TestValidateLocalWorkspaceTarget_RelativeAndAbsolute(t *testing.T) {
	base := t.TempDir()
	workspace := filepath.Join(base, provider.WorkspaceDir)
	input := filepath.Join(workspace, "config.json")
	sibling := filepath.Join(base, ".complytime-backup")
	require.NoError(t, os.MkdirAll(workspace, 0750))
	require.NoError(t, os.MkdirAll(sibling, 0750))
	require.NoError(t, os.WriteFile(input, []byte("{}"), 0600))
	workingDir, err := os.Getwd()
	require.NoError(t, err)
	relativeInput, err := filepath.Rel(workingDir, input)
	require.NoError(t, err)

	for _, path := range []string{input, relativeInput} {
		t.Run(path, func(t *testing.T) {
			err := validateLocalWorkspaceTarget(path, nil, workspace)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "copy intended input outside the workspace")
		})
	}
	assert.NoError(t, validateLocalWorkspaceTarget(base, nil, workspace))
	assert.NoError(t, validateLocalWorkspaceTarget(sibling, nil, workspace))
}

func TestWorkspaceBoundaries_SymlinkedRoot(t *testing.T) {
	base := t.TempDir()
	workspace := filepath.Join(base, "workspace")
	alias := filepath.Join(base, "alias")
	require.NoError(t, os.MkdirAll(workspace, 0750))
	require.NoError(t, os.Symlink(workspace, alias))
	boundaries, err := workspaceBoundaries(nil, alias)
	require.NoError(t, err)
	assert.Equal(t, []string{workspace}, boundaries)
}

func TestScan_ExplicitWorkspaceTargetsRejected(t *testing.T) {
	base := t.TempDir()
	hostBase := filepath.Join(base, "host")
	hostWorkspace := filepath.Join(hostBase, provider.WorkspaceDir)
	configuredWorkspace := filepath.Join(base, "custom-workspace")
	external := filepath.Join(base, "external.json")
	require.NoError(t, os.MkdirAll(filepath.Join(hostWorkspace, "nested"), 0750))
	require.NoError(t, os.MkdirAll(configuredWorkspace, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(hostWorkspace, "file.json"), []byte("{}"), 0600))
	require.NoError(t, os.WriteFile(external, []byte("{}"), 0600))
	writeScanConfig(t, hostBase)

	tests := []struct {
		name   string
		input  string
		isSafe bool
	}{
		{"host workspace", hostWorkspace, false},
		{"host workspace descendant", filepath.Join(hostWorkspace, "nested"), false},
		{"host workspace file", filepath.Join(hostWorkspace, "file.json"), false},
		{"configured workspace", configuredWorkspace, false},
		{"ordinary external file", external, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := conftestRunner(conftestAllSuccessJSON)
			srv := newWorkspaceTestServer(t, ServerOptions{
				Runner:       runner,
				ToolChecker:  func() ([]string, error) { return nil, nil },
				WorkspaceDir: configuredWorkspace,
			})
			target := localTarget(t, test.input, "")
			target.Variables[loader.VarWorkspace] = hostBase
			response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{target}))
			require.NoError(t, err)
			if test.isSafe {
				assert.Empty(t, response.Errors)
				require.Len(t, response.Assessments, 1)
				assert.Equal(t, "test-req", response.Assessments[0].RequirementID)
				require.Len(t, runner.calls, 1)
				assert.Equal(t, "test", runner.calls[0].args[0])
				return
			}
			require.Len(t, response.Errors, 1)
			assert.Contains(t, response.Errors[0], target.TargetID)
			assert.Contains(t, response.Errors[0], "copy intended input outside the workspace")
			assert.Empty(t, response.Assessments)
			assert.Empty(t, runner.calls)
		})
	}
}

func TestScan_WorkspaceRejectionPrecedesPolicyResolution(t *testing.T) {
	workspace := t.TempDir()
	runner := conftestRunner(conftestAllSuccessJSON)
	srv := newWorkspaceTestServer(t, ServerOptions{
		Runner:       runner,
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: workspace,
	})
	target := localTarget(t, workspace, "")
	response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{target}))
	require.NoError(t, err)
	require.Len(t, response.Errors, 1)
	assert.Contains(t, response.Errors[0], target.TargetID)
	assert.Contains(t, response.Errors[0], "copy intended input outside the workspace")
	assert.Empty(t, response.Assessments)
	assert.Empty(t, runner.calls)
}
