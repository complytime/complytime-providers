// SPDX-License-Identifier: Apache-2.0

package server

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"

	"github.com/complytime/complytime-providers/cmd/opa-provider/config"
	"github.com/complytime/complytime-providers/internal/evidence"
)

func TestScan_RetainedOutputFailures(t *testing.T) {
	for _, scenario := range []struct {
		name         string
		rpc, warning bool
	}{
		{"allocate", false, false}, {"write", false, false}, {"close", false, false},
		{"digest", false, false}, {"parse", false, false}, {"tool", false, false},
		{"identity", true, false}, {"mode", true, false}, {"changed", true, false},
		{"owner", true, false}, {"parent", true, false},
		{"metadata-write", false, true}, {"metadata-close", false, true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			root := t.TempDir()
			writeScanConfig(t, root)
			input := filepath.Join(root, "input.json")
			require.NoError(t, os.WriteFile(input, []byte(`{}`), 0600))
			target := localTarget(t, input, "")
			target.Variables[evidence.WorkspaceVariable] = root
			request := &provider.ScanRequest{Targets: []provider.Target{target}}
			runner := conftestRunner(conftestAllSuccessJSON)
			var logs bytes.Buffer
			srv := New(ServerOptions{Runner: runner, ToolChecker: func() ([]string, error) { return nil, nil },
				Logger: hclog.New(&hclog.LoggerOptions{Output: &logs, Level: hclog.Warn})})
			prior, err := srv.Scan(context.Background(), request)
			require.NoError(t, err)
			require.Empty(t, prior.Errors)
			coordinate := prior.Assessments[0].Evidence[0].Source.Coordinate
			original, err := os.ReadFile(filepath.Join(root, coordinate))
			require.NoError(t, err)
			if scenario.name == "identity" {
				srv.opts.Identity = func() (evidence.Identity, error) { return evidence.Identity{}, errors.New("injected identity failure") }
			}
			if scenario.name == "parse" {
				runner.callFn = nil
				runner.response = []byte("invalid result")
			}
			if scenario.name == "tool" {
				runner.callFn = nil
				runner.err = errors.New("injected tool failure")
			}
			var token, directory string
			allocated := false
			srv.opts.Allocate = func(options evidence.StorageOptions, labels ...string) (*evidence.Evaluation, error) {
				if scenario.name == "allocate" {
					return nil, errors.New("injected allocation failure")
				}
				options.Operations.Write = func(file *os.File, data []byte) (int, error) {
					name := filepath.Base(file.Name())
					if scenario.name == "write" && name == evidence.ConftestFile ||
						scenario.name == "metadata-write" && name == evidence.MetadataFile {
						return 0, errors.New("injected write failure")
					}
					if scenario.name == "changed" && name == evidence.MetadataFile {
						require.NoError(t, os.WriteFile(filepath.Join(directory, evidence.ConftestFile), []byte("substituted"), 0600))
					}
					return file.Write(data)
				}
				options.Operations.Close = func(file *os.File) error {
					err := file.Close()
					name := filepath.Base(file.Name())
					if scenario.name == "close" && name == evidence.ConftestFile ||
						scenario.name == "metadata-close" && name == evidence.MetadataFile {
						return errors.Join(err, errors.New("injected close failure"))
					}
					return err
				}
				if scenario.name == "digest" {
					options.Operations.Read = func(*os.File) ([]byte, error) { return nil, errors.New("injected digest read failure") }
				}
				if scenario.name == "mode" {
					options.Operations.Chmod = func(*os.File, os.FileMode) error { return errors.New("injected mode failure") }
				}
				if scenario.name == "owner" {
					options.Operations.Stat = func(file *os.File) (os.FileInfo, error) {
						info, err := file.Stat()
						if err != nil || !info.IsDir() {
							return info, err
						}
						owner := *info.Sys().(*syscall.Stat_t)
						owner.Uid++
						return retainedOwnerInfo{FileInfo: info, owner: &owner}, nil
					}
				}
				if scenario.name == "parent" {
					options.Operations.OpenDirectory = func(parent *os.File, component string) (*os.File, error) {
						if allocated {
							return nil, fs.ErrPermission
						}
						return os.OpenInRoot(parent.Name(), component)
					}
				}
				evaluation, err := evidence.Allocate(options, labels...)
				if evaluation != nil {
					token, directory = evaluation.Token, evaluation.Directory
					allocated = true
				}
				return evaluation, err
			}
			response, err := srv.Scan(context.Background(), request)
			if scenario.rpc {
				require.Error(t, err)
				assert.Nil(t, response)
			} else {
				require.NoError(t, err)
				if scenario.warning {
					assert.Empty(t, response.Errors)
					require.Len(t, response.Assessments, 1)
					assert.Contains(t, logs.String(), "cannot save supporting evaluation metadata")
					assert.Contains(t, logs.String(), token)
					assertRetainedPrivate(t, filepath.Join(directory, evidence.MetadataFile), 0600)
				} else {
					require.Len(t, response.Errors, 1)
					assert.Contains(t, response.Errors[0], target.TargetID)
					assert.Empty(t, response.Assessments)
					assert.Empty(t, response.MappingReferences)
				}
			}
			stored, err := os.ReadFile(filepath.Join(root, coordinate))
			require.NoError(t, err)
			assert.Equal(t, original, stored)
		})
	}
}

type retainedOwnerInfo struct {
	os.FileInfo
	owner *syscall.Stat_t
}

func (info retainedOwnerInfo) Sys() any { return info.owner }

func TestScan_RepeatedContextsKeepDistinctArtifactInstances(t *testing.T) {
	root := t.TempDir()
	writeScanConfig(t, root)
	input := filepath.Join(root, "input.json")
	require.NoError(t, os.WriteFile(input, []byte(`{}`), 0600))
	first := localTarget(t, input, "")
	first.TargetID = "colliding/label"
	first.Variables[evidence.WorkspaceVariable] = root
	second := first
	second.TargetID = "colliding-label"
	srv := New(ServerOptions{Runner: conftestRunner(conftestAllSuccessJSON),
		ToolChecker: func() ([]string, error) { return nil, nil }})
	response, err := srv.Scan(context.Background(), &provider.ScanRequest{Targets: []provider.Target{first, second, first}})
	require.NoError(t, err)
	require.Empty(t, response.Errors)
	require.Len(t, response.Assessments, 1)
	assert.Len(t, response.MappingReferences, 3)
	coordinates := map[string]bool{}
	for _, item := range response.Assessments[0].Evidence {
		coordinates[item.Source.Coordinate] = true
		assert.Contains(t, item.Source.Coordinate, "colliding-label")
	}
	assert.Len(t, coordinates, 3)
	entries, err := os.ReadDir(config.NewWorkspaceConfig(root).ResultsDirPath())
	require.NoError(t, err)
	require.Len(t, entries, 1, "lossy labels may share only context parents")
	assert.False(t, strings.Contains(entries[0].Name(), "/"))
}

func TestScan_RetainedHistorySafety(t *testing.T) {
	for _, scenario := range []string{"local-symlink", "local-hardlink", "remote-symlink", "remote-hardlink", "private-history"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			writeScanConfig(t, root)
			input := filepath.Join(root, "input.json")
			require.NoError(t, os.WriteFile(input, []byte(`{}`), 0600))
			target := localTarget(t, input, "")
			target.Variables[evidence.WorkspaceVariable] = root
			runner := conftestRunner(conftestAllSuccessJSON)
			srv := New(ServerOptions{Runner: runner, ToolChecker: func() ([]string, error) { return nil, nil }})
			prior, err := srv.Scan(context.Background(), &provider.ScanRequest{Targets: []provider.Target{target}})
			require.NoError(t, err)
			require.Empty(t, prior.Errors)
			artifact := filepath.Join(root, prior.Assessments[0].Evidence[0].Source.Coordinate)
			original, err := os.ReadFile(artifact)
			require.NoError(t, err)
			history := filepath.Dir(artifact)
			if scenario == "private-history" {
				srv.localTraversal.readDir = func(path string) ([]os.DirEntry, error) {
					if path == history {
						return nil, fs.ErrPermission
					}
					return os.ReadDir(path)
				}
			} else {
				alias := filepath.Join(root, "alias.json")
				if strings.HasPrefix(scenario, "remote") {
					clone := filepath.Join(config.NewWorkspaceConfig(root).ReposDirPath(), "selected")
					require.NoError(t, os.MkdirAll(clone, 0755))
					alias = filepath.Join(clone, "alias.json")
					srv.opts.Loader = &mockLoader{loadFn: func(provider.Target, string) (string, error) { return clone, nil }}
					srv.defaultLoader = false
					target = remoteTarget("", "https://example.com/org/repo")
					target.Variables[evidence.WorkspaceVariable] = root
				} else {
					target.Variables["input_path"] = alias
				}
				if strings.HasSuffix(scenario, "symlink") {
					require.NoError(t, os.Symlink(artifact, alias))
				} else {
					require.NoError(t, os.Link(artifact, alias))
				}
			}
			runner.calls = nil
			response, err := srv.Scan(context.Background(), &provider.ScanRequest{Targets: []provider.Target{target}})
			require.NoError(t, err)
			require.Len(t, response.Errors, 1)
			assert.Contains(t, response.Errors[0], target.TargetID)
			if scenario == "private-history" {
				assert.Contains(t, response.Errors[0], "fix inaccessible or unsafe paths")
				assert.Contains(t, response.Errors[0], history)
			}
			assert.Empty(t, response.Assessments)
			assert.Empty(t, response.MappingReferences)
			assert.Empty(t, runner.calls)
			stored, err := os.ReadFile(artifact)
			require.NoError(t, err)
			assert.Equal(t, original, stored)
			assertRetainedPrivate(t, history, 0700)
			assertRetainedPrivate(t, artifact, 0600)
		})
	}
}

func TestScan_RemoteScanPathsKeepSeparateEvaluations(t *testing.T) {
	root := t.TempDir()
	writeScanConfig(t, root)
	ldr := &mockLoader{loadFn: func(target provider.Target, workDir string) (string, error) {
		clone := filepath.Join(workDir, "repo", target.Variables["branch"])
		require.NoError(t, os.RemoveAll(clone))
		path := filepath.Join(clone, target.Variables["scan_path"])
		require.NoError(t, os.MkdirAll(path, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(path, "input.json"), []byte(`{}`), 0600))
		return path, nil
	}}
	srv := New(ServerOptions{Runner: conftestRunner(conftestAllSuccessJSON), Loader: ldr,
		ToolChecker: func() ([]string, error) { return nil, nil }})
	var targets []provider.Target
	for _, path := range []string{"one", "two", "one"} {
		target := remoteTarget("", "https://example.com/org/repo")
		target.Variables[evidence.WorkspaceVariable] = root
		target.Variables["scan_path"] = path
		targets = append(targets, target)
	}
	response, err := srv.Scan(context.Background(), &provider.ScanRequest{Targets: targets})
	require.NoError(t, err)
	require.Empty(t, response.Errors)
	require.Len(t, response.Assessments, 1)
	assert.Len(t, response.Assessments[0].Evidence, 3)
	assert.Len(t, response.MappingReferences, 3)
	seen := map[string]bool{}
	paths := map[string]int{}
	for _, item := range response.Assessments[0].Evidence {
		assert.False(t, seen[item.ID])
		seen[item.ID] = true
		metadata, err := os.ReadFile(filepath.Join(root, filepath.Dir(item.Source.Coordinate), evidence.MetadataFile))
		require.NoError(t, err)
		for _, path := range []string{"one", "two"} {
			if strings.Contains(string(metadata), `"scan_path": "`+path+`"`) {
				paths[path]++
			}
		}
	}
	assert.Equal(t, map[string]int{"one": 2, "two": 1}, paths)
}
