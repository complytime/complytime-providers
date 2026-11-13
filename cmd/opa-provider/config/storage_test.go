// SPDX-License-Identifier: Apache-2.0

package config

import (
	"path/filepath"
	"testing"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/complytime/complytime-providers/internal/evidence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageOptions_ResolvedProviderRoots(t *testing.T) {
	root := t.TempDir()
	identity, err := evidence.ExecutionIdentity()
	require.NoError(t, err)
	cfg := NewWorkspaceConfig(root)
	options := cfg.StorageOptions(identity)
	assert.Equal(t, root, options.WorkspaceRoot)
	assert.Equal(t, identity, options.Identity)
	assert.Equal(t, filepath.Join(root, provider.WorkspaceDir, ProviderDir, ResultsDir), options.ResultsRoot)
	evaluation, err := evidence.Allocate(options, "target")
	require.NoError(t, err)
	assert.DirExists(t, evaluation.Directory)
	require.NoError(t, evaluation.Close())
	options.ResultsRoot = t.TempDir()
	_, err = evidence.Allocate(options, "target")
	require.ErrorContains(t, err, "beneath workspace")
}
