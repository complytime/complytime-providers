// SPDX-License-Identifier: Apache-2.0

package evidence

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveWorkspace_RootAndLegacyFallback(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(root, link))
	resolved, err := ResolveWorkspace(link)
	require.NoError(t, err)
	assert.Equal(t, root, resolved)
	cwd, err := os.Getwd()
	require.NoError(t, err)
	fallback, err := ResolveWorkspace("")
	require.NoError(t, err)
	assert.Equal(t, cwd, fallback)
	relative, err := filepath.Rel(cwd, root)
	require.NoError(t, err)
	resolved, err = ResolveWorkspace(relative)
	require.NoError(t, err)
	assert.Equal(t, root, resolved)
	_, err = ResolveWorkspace(filepath.Join(root, "absent"))
	require.Error(t, err)
	file := filepath.Join(root, "file")
	require.NoError(t, os.WriteFile(file, nil, FileMode))
	_, err = ResolveWorkspace(file)
	require.ErrorContains(t, err, "directory")
	assert.Equal(t, filepath.Join(root, "input"), WorkspacePath(root, "input"))
	assert.Equal(t, "/external", WorkspacePath(root, "/external"))
	assert.Empty(t, WorkspacePath(root, ""))
}

func TestWorkspace_RequestConsistencyAndEnvironmentIndependence(t *testing.T) {
	root, other := t.TempDir(), t.TempDir()
	t.Setenv("COMPLYTIME_WORKSPACE", other)
	global := map[string]string{WorkspaceVariable: root}
	resolved, err := GenerateWorkspace(&provider.GenerateRequest{GlobalVariables: global})
	require.NoError(t, err)
	assert.Equal(t, root, resolved)
	_, err = GenerateWorkspace(&provider.GenerateRequest{GlobalVariables: global,
		TargetVariables: map[string]string{WorkspaceVariable: other}})
	require.ErrorContains(t, err, "conflicts")
	request := &provider.ScanRequest{Targets: []provider.Target{
		{TargetID: "a", Variables: global}, {TargetID: "b", Variables: global},
	}}
	resolved, err = ScanWorkspace(request)
	require.NoError(t, err)
	assert.Equal(t, root, resolved)
	request.Targets[1].Variables = map[string]string{WorkspaceVariable: other}
	_, err = ScanWorkspace(request)
	require.ErrorContains(t, err, "conflicting")
}
