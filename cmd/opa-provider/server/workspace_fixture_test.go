// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"testing"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complytime-providers/cmd/opa-provider/config"
	"github.com/complytime/complytime-providers/internal/evidence"
)

// workspaceTestServer makes pre-existing artifact-directory fixtures explicit RPC
// workspace fixtures. Production never infers an output root from ServerOptions.
type workspaceTestServer struct{ *ProviderServer }

func newWorkspaceTestServer(t *testing.T, opts ServerOptions) *workspaceTestServer {
	t.Helper()
	if opts.WorkspaceDir == "" {
		opts.WorkspaceDir = t.TempDir()
	}
	srv := New(opts).inWorkspace(opts.WorkspaceDir)
	identity, err := srv.opts.Identity()
	require.NoError(t, err)
	srv.storage = config.NewWorkspaceConfig(opts.WorkspaceDir).StorageOptions(identity)
	return &workspaceTestServer{srv}
}

func (s *workspaceTestServer) Scan(ctx context.Context, req *provider.ScanRequest) (*provider.ScanResponse, error) {
	copy := *req
	copy.Targets = append([]provider.Target(nil), req.Targets...)
	for index := range copy.Targets {
		target := &copy.Targets[index]
		target.Variables = copyVars(target.Variables)
		if target.Variables[evidence.WorkspaceVariable] == "" {
			target.Variables[evidence.WorkspaceVariable] = s.opts.WorkspaceDir
		}
	}
	if runner, ok := s.opts.Runner.(*recordingRealRunner); ok {
		root, err := evidence.ScanWorkspace(&copy)
		if err != nil {
			return nil, err
		}
		runner.workingDirectory = root
	}
	return s.ProviderServer.Scan(ctx, &copy)
}

func (s *workspaceTestServer) Generate(ctx context.Context, req *provider.GenerateRequest) (*provider.GenerateResponse, error) {
	copy := *req
	copy.GlobalVariables = copyVars(req.GlobalVariables)
	if copy.GlobalVariables[evidence.WorkspaceVariable] == "" {
		copy.GlobalVariables[evidence.WorkspaceVariable] = s.opts.WorkspaceDir
	}
	return s.ProviderServer.Generate(ctx, &copy)
}
