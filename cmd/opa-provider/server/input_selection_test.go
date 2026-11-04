// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/complytime/complytime-providers/cmd/opa-provider/config"
	"github.com/complytime/complytime-providers/cmd/opa-provider/generate"
	"github.com/complytime/complytime-providers/cmd/opa-provider/loader"
	"github.com/complytime/complytime-providers/cmd/opa-provider/results"
)

func TestInputSelectionError_PreservesClassificationAndCause(t *testing.T) {
	cause := errors.New("missing input")
	err := &inputSelectionError{err: cause}
	assert.Equal(t, cause.Error(), err.Error())
	wrapped := fmt.Errorf("evaluation: %w", err)
	var selectionErr *inputSelectionError
	require.ErrorAs(t, wrapped, &selectionErr)
	assert.Same(t, err, selectionErr)
	assert.ErrorIs(t, wrapped, cause)
	assert.False(t, errors.As(cause, &selectionErr))
}

func TestRejectTargetResults_PreservesEarlierOperationalErrors(t *testing.T) {
	prior := targetError("target", "", "first", errors.New("clone failed"))
	passed := &results.PerTargetResult{Status: "scanned", Target: "target", Branch: "main"}
	rejection := targetError("target", "", "second", errors.New("unsafe input"))
	previous := []*results.PerTargetResult{prior, passed}
	assert.Equal(t, []*results.PerTargetResult{prior, rejection},
		rejectTargetResults(previous, rejection))
	assert.Equal(t, []*results.PerTargetResult{prior, passed}, previous)
}

func TestProcessRemoteBranches_RejectionPreservesWriteErrors(t *testing.T) {
	workspace := t.TempDir()
	cfg := config.NewConfig(workspace)
	require.NoError(t, cfg.EnsureDirectories())
	// A regular file at the results directory deterministically blocks writes.
	require.NoError(t, os.Remove(cfg.ResultsDirPath()))
	require.NoError(t, os.WriteFile(cfg.ResultsDirPath(), []byte("blocked"), 0600))
	ldr := &mockLoader{loadFn: func(target provider.Target, _ string) (string, error) {
		clone := filepath.Join(workspace, target.Variables[loader.VarBranch])
		require.NoError(t, os.Mkdir(clone, 0750))
		return clone, nil
	}}
	runner := &mockRunner{callFn: func(_ string, args []string) ([]byte, error) {
		if filepath.Base(args[1]) == "second" {
			return []byte(`[]`), nil
		}
		return []byte(conftestAllSuccessJSON), nil
	}}
	srv := New(ServerOptions{Runner: runner, Loader: ldr, WorkspaceDir: workspace})
	target := remoteTarget("", "https://example.com/org/repo")
	branchResults, err := srv.processRemoteBranches(context.Background(), hclog.NewNullLogger(),
		target, []string{"main", "second"}, cfg.PolicyDirPath(), cfg,
		&generate.ScanConfig{IDs: []string{"main"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating results directory")
	assert.Contains(t, err.Error(), cfg.ResultsDirPath())
	require.Len(t, branchResults, 1)
	assert.Equal(t, "error", branchResults[0].Status)
	assert.Equal(t, "second", branchResults[0].Branch)
	assert.Contains(t, branchResults[0].Error, "no eligible input")
}
