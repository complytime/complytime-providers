// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/complytime/complytime-providers/cmd/opa-provider/loader"
)

func TestScan_RemoteBranchRejectionDiscardsTargetAssessments(t *testing.T) {
	for _, scenario := range []string{
		"symlink", "hardlink", "empty", "fully ignored", "missing evidence", "invalid output", "valid",
	} {
		for _, order := range []string{"main,second", "second,main"} {
			t.Run(scenario+"/"+order, func(t *testing.T) {
				workspace := t.TempDir()
				writeScanConfig(t, workspace)
				artifact := filepath.Join(workspace, "artifact.json")
				require.NoError(t, os.WriteFile(artifact, []byte(`{}`), 0600))
				validFile := filepath.Join(t.TempDir(), "valid.json")
				require.NoError(t, os.WriteFile(validFile, []byte(`{}`), 0600))
				ldr := &mockLoader{loadFn: func(target provider.Target, _ string) (string, error) {
					if path := target.Variables[loader.VarInputPath]; path != "" {
						return path, nil
					}
					branch := target.Variables[loader.VarBranch]
					clone := filepath.Join(workspace, branch)
					require.NoError(t, os.MkdirAll(clone, 0750))
					input := filepath.Join(clone, "input.json")
					if branch == "second" {
						switch scenario {
						case "symlink":
							return clone, os.Symlink(artifact, input)
						case "hardlink":
							return clone, os.Link(artifact, input)
						case "empty":
							return clone, nil
						}
					}
					return clone, os.WriteFile(input, []byte(`{}`), 0600)
				}}
				runner := &mockRunner{callFn: func(_ string, args []string) ([]byte, error) {
					if args[1] == filepath.Join(workspace, "second") {
						switch scenario {
						case "empty", "fully ignored":
							return []byte(conftestNoFilesFound), errors.New("exit status 1")
						case "missing evidence":
							return []byte(`[{}]`), nil
						case "invalid output":
							return []byte(`invalid`), nil
						}
					}
					return []byte(conftestAllSuccessJSON), nil
				}}
				srv := New(ServerOptions{Runner: runner, Loader: ldr, WorkspaceDir: workspace,
					ToolChecker: func() ([]string, error) { return nil, nil }})
				target := remoteTarget("", "https://example.com/org/repo")
				target.TargetID = "multi-branch"
				target.Variables[loader.VarBranches] = order
				valid := localTarget(t, validFile, "")
				valid.TargetID = "other-target"
				response, err := srv.Scan(context.Background(),
					makeScanRequest(t, []provider.Target{target, valid}))
				require.NoError(t, err)
				require.Len(t, response.Assessments, 1)
				assessment := response.Assessments[0]
				assert.Equal(t, "test-req", assessment.RequirementID)
				if scenario == "valid" {
					assert.Empty(t, response.Errors)
					require.Len(t, assessment.Steps, 3)
					assert.Len(t, response.MappingReferences, 3)
					for _, branch := range strings.Split(order, ",") {
						assert.Contains(t, assessment.Steps, provider.Step{
							Name:   "https://example.com/org/repo@" + branch,
							Result: provider.ResultPassed, Message: "all checks passed",
						})
					}
					return
				}
				require.Len(t, response.Errors, 1)
				assert.Contains(t, response.Errors[0], "multi-branch@second:")
				cause := "no eligible input"
				switch scenario {
				case "symlink":
					cause = "resolves to workspace"
				case "hardlink":
					cause = "shares filesystem identity"
				case "invalid output":
					cause = "parsing conftest output"
				}
				assert.Contains(t, response.Errors[0], cause)
				require.Len(t, assessment.Steps, 1)
				assert.Equal(t, validFile, assessment.Steps[0].Name)
				assert.Equal(t, provider.ResultPassed, assessment.Steps[0].Result)
				require.Len(t, assessment.Evidence, 1)
				assert.Equal(t, validFile, assessment.Evidence[0].Source.Coordinate)
				assert.Len(t, response.MappingReferences, 1)
			})
		}
	}
}
