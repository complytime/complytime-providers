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
	"github.com/complytime/complytime-providers/internal/evidence"
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

func TestProcessRemoteBranches_RejectionPreservesArtifactWriteErrors(t *testing.T) {
	workspace := t.TempDir()
	cfg := config.NewWorkspaceConfig(workspace)
	require.NoError(t, cfg.EnsureDirectories())
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
	srv := newWorkspaceTestServer(t, ServerOptions{Runner: runner, Loader: ldr, WorkspaceDir: workspace})
	srv.opts.Allocate = func(options evidence.StorageOptions, labels ...string) (*evidence.Evaluation, error) {
		if labels[len(labels)-1] == "main" {
			options.Operations.Write = func(*os.File, []byte) (int, error) {
				return 0, errors.New("injected artifact write failure")
			}
		}
		return evidence.Allocate(options, labels...)
	}
	target := remoteTarget("", "https://example.com/org/repo")
	branchResults, err := srv.processRemoteBranches(context.Background(), hclog.NewNullLogger(),
		target, []string{"main", "second"}, cfg.PolicyDirPath(), cfg,
		&generate.ScanConfig{IDs: []string{"main"}})
	require.NoError(t, err)
	require.Len(t, branchResults, 2)
	assert.Equal(t, "error", branchResults[0].Status)
	assert.Equal(t, "main", branchResults[0].Branch)
	assert.Contains(t, branchResults[0].Error, "injected artifact write failure")
	assert.Equal(t, "second", branchResults[1].Branch)
	assert.Contains(t, branchResults[1].Error, "no eligible input")
	response := results.ToScanResponse(branchResults, nil)
	assert.Empty(t, response.Assessments)
	assert.Empty(t, response.MappingReferences)
}
