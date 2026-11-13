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
	"github.com/complytime/complytime-providers/internal/evidence"
)

func TestScan_RemoteBranchRejectionDiscardsTargetAssessments(t *testing.T) {
	for _, scenario := range []string{
		"symlink", "hardlink", "directory cycle", "empty", "fully ignored", "missing evidence",
		"invalid output", "valid",
		"preflight cancellation",
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
						case "directory cycle":
							return clone, os.Symlink(".", filepath.Join(clone, "cycle"))
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
				srv := newWorkspaceTestServer(t, ServerOptions{Runner: runner, Loader: ldr, WorkspaceDir: workspace,
					ToolChecker: func() ([]string, error) { return nil, nil }})
				if scenario == "preflight cancellation" {
					cancelled := false
					srv.localTraversal.lstat = func(path string) (os.FileInfo, error) {
						if !cancelled && path == filepath.Join(workspace, "second") {
							cancelled = true
							return nil, context.Canceled
						}
						return os.Lstat(path)
					}
				}
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
				case "directory cycle":
					cause = "directory symlink cycle"
				case "invalid output":
					cause = "parsing conftest output"
				case "preflight cancellation":
					cause = "context canceled"
				}
				assert.Contains(t, response.Errors[0], cause)
				require.Len(t, assessment.Steps, 1)
				assert.Equal(t, validFile, assessment.Steps[0].Name)
				assert.Equal(t, provider.ResultPassed, assessment.Steps[0].Result)
				require.Len(t, assessment.Evidence, 1)
				coordinate := assessment.Evidence[0].Source.Coordinate
				assert.Equal(t, "conftest.json", filepath.Base(coordinate))
				stored, err := os.ReadFile(filepath.Join(workspace, coordinate))
				require.NoError(t, err)
				assert.Equal(t, conftestAllSuccessJSON, string(stored))
				assert.Len(t, response.MappingReferences, 1)
				// Complete and partial rejected-branch artifacts remain on disk;
				// directory contents must never expand the response's evidence set.
				retained, err := filepath.Glob(filepath.Join(workspace, provider.WorkspaceDir, "opa", "results",
					"example-com-org-repo", "*", "evaluation-*", "conftest.json"))
				require.NoError(t, err)
				if order == "main,second" {
					assert.NotEmpty(t, retained)
				}
			})
		}
	}
}

func TestScan_OrdinaryBranchFailuresRemainBranchScoped(t *testing.T) {
	for _, failure := range []string{"clone", "tool", "storage"} {
		for _, order := range []string{"main,second", "second,main"} {
			t.Run(failure+"/"+order, func(t *testing.T) {
				root := t.TempDir()
				writeScanConfig(t, root)
				ldr := &mockLoader{loadFn: func(target provider.Target, workDir string) (string, error) {
					branch := target.Variables[loader.VarBranch]
					if failure == "clone" && branch == "second" {
						return "", errors.New("injected clone failure")
					}
					path := filepath.Join(workDir, "repo", branch)
					require.NoError(t, os.MkdirAll(path, 0755))
					require.NoError(t, os.WriteFile(filepath.Join(path, "input.json"), []byte(`{}`), 0600))
					return path, nil
				}}
				runner := &mockRunner{callFn: func(_ string, args []string) ([]byte, error) {
					if failure == "tool" && filepath.Base(args[1]) == "second" {
						return nil, errors.New("injected tool failure")
					}
					return []byte(conftestAllSuccessJSON), nil
				}}
				srv := New(ServerOptions{Runner: runner, Loader: ldr,
					ToolChecker: func() ([]string, error) { return nil, nil }})
				if failure == "storage" {
					srv.opts.Allocate = func(options evidence.StorageOptions, labels ...string) (*evidence.Evaluation, error) {
						if labels[len(labels)-1] == "second" {
							options.Operations.Write = func(*os.File, []byte) (int, error) { return 0, errors.New("injected storage failure") }
						}
						return evidence.Allocate(options, labels...)
					}
				}
				target := remoteTarget("", "https://example.com/org/repo")
				target.Variables[loader.VarBranches] = order
				target.Variables[evidence.WorkspaceVariable] = root
				response, err := srv.Scan(context.Background(), &provider.ScanRequest{Targets: []provider.Target{target}})
				require.NoError(t, err)
				require.Len(t, response.Errors, 1)
				assert.Contains(t, response.Errors[0], "@second:")
				assert.Contains(t, response.Errors[0], "injected "+failure+" failure")
				require.Len(t, response.Assessments, 1)
				require.Len(t, response.Assessments[0].Evidence, 1)
				assert.Len(t, response.MappingReferences, 1)
				assert.Contains(t, response.Assessments[0].Evidence[0].Source.Coordinate, "/main/")
			})
		}
	}
}

func TestScan_RejectedCloneCyclePreservesIndependentTargets(t *testing.T) {
	workspace := t.TempDir()
	writeScanConfig(t, workspace)
	localFile := filepath.Join(t.TempDir(), "input.json")
	require.NoError(t, os.WriteFile(localFile, []byte(`{}`), 0600))
	rejectedClone := filepath.Join(workspace, "rejected-clone")
	validClone := filepath.Join(workspace, "valid-clone")
	runner := conftestRunner(conftestAllSuccessJSON)
	ldr := &mockLoader{loadFn: func(target provider.Target, _ string) (string, error) {
		if path := target.Variables[loader.VarInputPath]; path != "" {
			return path, nil
		}
		if target.TargetID == "rejected-target" {
			require.NoError(t, os.Mkdir(rejectedClone, 0750))
			require.NoError(t, os.Symlink(".", filepath.Join(rejectedClone, "cycle")))
			return rejectedClone, nil
		}
		require.NoError(t, os.MkdirAll(validClone, 0750))
		require.NoError(t, os.WriteFile(filepath.Join(validClone, "input.json"), []byte(`{}`), 0600))
		return validClone, nil
	}}
	srv := newWorkspaceTestServer(t, ServerOptions{Runner: runner, Loader: ldr, WorkspaceDir: workspace,
		ToolChecker: func() ([]string, error) { return nil, nil }})
	rejected := remoteTarget("", "https://example.com/org/rejected")
	rejected.TargetID = "rejected-target"
	local := localTarget(t, localFile, "")
	local.TargetID = "valid-local"
	remote := remoteTarget("", "https://example.com/org/valid")
	remote.TargetID = "valid-remote"

	for _, repeated := range []bool{false, true} {
		targets := []provider.Target{local, remote}
		if !repeated {
			targets = append([]provider.Target{rejected}, targets...)
		}
		response, err := srv.Scan(context.Background(), makeScanRequest(t, targets))
		require.NoError(t, err)
		if repeated {
			assert.Empty(t, response.Errors)
		} else {
			require.Len(t, response.Errors, 1)
			assert.Contains(t, response.Errors[0], "rejected-target@main:")
			assert.Contains(t, response.Errors[0], "directory symlink cycle")
		}
		require.Len(t, response.Assessments, 1)
		assessment := response.Assessments[0]
		assert.Equal(t, "test-req", assessment.RequirementID)
		assert.ElementsMatch(t, []provider.Step{
			{Name: localFile, Result: provider.ResultPassed, Message: "all checks passed"},
			{Name: "https://example.com/org/valid@main", Result: provider.ResultPassed,
				Message: "all checks passed"},
		}, assessment.Steps)
		var coordinates []string
		for _, evidence := range assessment.Evidence {
			require.NotNil(t, evidence.Source)
			coordinates = append(coordinates, evidence.Source.Coordinate)
		}
		require.Len(t, coordinates, 2)
		assert.NotEqual(t, coordinates[0], coordinates[1])
		for _, coordinate := range coordinates {
			assert.Equal(t, "conftest.json", filepath.Base(coordinate))
			stored, err := os.ReadFile(filepath.Join(workspace, coordinate))
			require.NoError(t, err)
			assert.Equal(t, conftestAllSuccessJSON, string(stored))
		}
		assert.Len(t, response.MappingReferences, 2)
		link, err := os.Readlink(filepath.Join(rejectedClone, "cycle"))
		require.NoError(t, err, "rejected clones must remain on disk")
		assert.Equal(t, ".", link)
	}
	require.Len(t, runner.calls, 4)
	for _, call := range runner.calls {
		assert.Equal(t, "conftest", call.name)
		require.GreaterOrEqual(t, len(call.args), 2)
		assert.Equal(t, "test", call.args[0])
		assert.Contains(t, []string{localFile, validClone}, call.args[1])
	}
}
