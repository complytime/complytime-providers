// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"

	"github.com/complytime/complytime-providers/cmd/opa-provider/config"
	"github.com/complytime/complytime-providers/internal/evidence"
)

func TestScan_WorkspaceProcessIsolation(t *testing.T) {
	if os.Getenv("OPA_RETENTION_WORKSPACE_PROCESS") == "1" {
		runWorkspaceProcess(t)
		return
	}
	for _, spelling := range []string{"absent", "empty", "absolute", "relative", "symlink"} {
		for _, configuration := range []string{"invocation-only", "discovery", "explicit"} {
			t.Run(spelling+"/"+configuration, func(t *testing.T) {
				base := t.TempDir()
				invocation := filepath.Join(base, "invocation")
				workspace := filepath.Join(base, "workspace % café")
				for _, path := range []string{invocation, workspace} {
					require.NoError(t, os.Mkdir(path, 0755))
				}
				value := workspace
				switch spelling {
				case "absent", "empty":
					workspace, value = invocation, ""
				case "relative":
					value = filepath.Join("..", filepath.Base(workspace))
				case "symlink":
					value = filepath.Join(base, "alias")
					require.NoError(t, os.Symlink(workspace, value))
				}
				writePolicyMapping(t, filepath.Join(invocation, "policies"),
					`{"version":"1","mappings":[{"id":"wrong","requirement_id":"wrong"}]}`)
				writePolicyMapping(t, filepath.Join(workspace, "policies"),
					`{"version":"1","mappings":[{"id":"main","requirement_id":"REQ-1"}]}`)
				require.NoError(t, os.WriteFile(filepath.Join(invocation, "input.json"), []byte(`{"wrong":true}`), 0600))
				require.NoError(t, os.WriteFile(filepath.Join(workspace, "input.json"), []byte(`{"selected":true}`), 0600))
				if invocation != workspace {
					require.NoError(t, os.WriteFile(filepath.Join(invocation, "conftest.toml"), []byte(`ignore="wrong"`), 0600))
				}
				if configuration == "discovery" {
					require.NoError(t, os.WriteFile(filepath.Join(workspace, "conftest.toml"), []byte(`ignore="selected"`), 0600))
				}
				if configuration == "explicit" {
					for _, root := range []string{invocation, workspace} {
						require.NoError(t, os.MkdirAll(filepath.Join(root, "configs"), 0755))
						require.NoError(t, os.WriteFile(filepath.Join(root, "configs", "settings.toml"), nil, 0600))
					}
				}
				binaryDir := filepath.Join(base, "bin")
				oldArtifact := ""
				if invocation != workspace {
					oldArtifact = filepath.Join(invocation, provider.WorkspaceDir, "old-artifact.json")
					require.NoError(t, os.MkdirAll(filepath.Dir(oldArtifact), 0755))
					require.NoError(t, os.WriteFile(oldArtifact, []byte(`{"old":true}`), 0600))
					require.NoError(t, os.Link(oldArtifact, filepath.Join(workspace, "old-alias.json")))
				}
				require.NoError(t, os.Mkdir(binaryDir, 0755))
				script := "#!/bin/sh\nset -eu\n" +
					"{ pwd; printf '%s\\n' \"$@\"; } >> \"$OPA_RETENTION_TOOL_LOG\"\n" +
					"printf '%s\\n' '[{\"filename\":\"input.json\",\"namespace\":\"main\",\"successes\":1}]'\n" +
					"printf '%s\\n' 'diagnostic stderr' >&2\n"
				fakeConftest := filepath.Join(binaryDir, "conftest")
				require.NoError(t, os.WriteFile(fakeConftest, []byte(script), 0600))
				require.NoError(t, os.Chmod(fakeConftest, 0755))
				executable, err := os.Executable()
				require.NoError(t, err)
				command := exec.Command(executable, "-test.run=^TestScan_WorkspaceProcessIsolation$")
				command.Dir = invocation
				command.Env = append(os.Environ(), "OPA_RETENTION_WORKSPACE_PROCESS=1",
					"OPA_RETENTION_ROOT="+workspace, "OPA_RETENTION_VALUE="+value,
					"OPA_RETENTION_OLD_ARTIFACT="+oldArtifact,
					"OPA_RETENTION_SPELLING="+spelling, "OPA_RETENTION_CONFIGURATION="+configuration,
					"OPA_RETENTION_TOOL_LOG="+filepath.Join(workspace, "tool.log"),
					"PATH="+binaryDir+":"+os.Getenv("PATH"), "COMPLYTIME_WORKSPACE="+invocation,
					"CONFTEST_IGNORE=", "CONFTEST_CONFIG_FILE=", "SUDO_UID=12345", "SUDO_GID=12345", "USER=other")
				output, err := command.CombinedOutput()
				require.NoError(t, err, "%s", output)
				if invocation != workspace {
					entries, err := os.ReadDir(filepath.Dir(oldArtifact))
					require.NoError(t, err)
					require.Len(t, entries, 1, "do not create new provider artifacts beneath invocation")
					assert.Equal(t, "old-artifact.json", entries[0].Name())
					stored, err := os.ReadFile(oldArtifact)
					require.NoError(t, err)
					assert.Equal(t, `{"old":true}`, string(stored))
				}
			})
		}
	}
}

func runWorkspaceProcess(t *testing.T) {
	root := os.Getenv("OPA_RETENTION_ROOT")
	workspace, err := os.OpenRoot(root)
	require.NoError(t, err)
	defer func() { require.NoError(t, workspace.Close()) }()
	value := os.Getenv("OPA_RETENTION_VALUE")
	before, err := os.Getwd()
	require.NoError(t, err)
	variables := map[string]string{}
	if os.Getenv("OPA_RETENTION_SPELLING") != "absent" {
		variables[evidence.WorkspaceVariable] = value
	}
	srv := New(ServerOptions{ToolChecker: func() ([]string, error) { return nil, nil }})
	generated, err := srv.Generate(context.Background(), &provider.GenerateRequest{
		GlobalVariables: variables, TargetVariables: map[string]string{"input_path": "."},
		ComplypackContentPath: "policies", Configuration: []provider.AssessmentConfiguration{{RequirementID: "REQ-1"}},
	})
	require.NoError(t, err)
	require.True(t, generated.Success, generated.ErrorMessage)
	target := localTarget(t, ".", "")
	for key, value := range variables {
		target.Variables[key] = value
	}
	request := &provider.ScanRequest{Targets: []provider.Target{target}}
	configuration := os.Getenv("OPA_RETENTION_CONFIGURATION")
	if configuration == "explicit" {
		t.Setenv(conftestConfigFileEnv, "configs/settings.toml")
	}
	if configuration != "invocation-only" {
		response, err := srv.Scan(context.Background(), request)
		require.NoError(t, err)
		require.Len(t, response.Errors, 1)
		assert.Contains(t, response.Errors[0], root)
		assert.Contains(t, response.Errors[0], "CONFTEST_IGNORE")
		assert.Empty(t, response.Assessments)
		assert.NoFileExists(t, os.Getenv("OPA_RETENTION_TOOL_LOG"))
		t.Setenv(conftestIgnoreEnv, "^$")
	}
	if os.Getenv("OPA_RETENTION_OLD_ARTIFACT") != "" {
		response, err := srv.Scan(context.Background(), request)
		require.NoError(t, err)
		require.Len(t, response.Errors, 1)
		assert.Contains(t, response.Errors[0], "shares filesystem identity")
		assert.Empty(t, response.Assessments)
		assert.Empty(t, response.MappingReferences)
		require.NoError(t, workspace.Remove("old-alias.json"))
	}
	response, err := srv.Scan(context.Background(), request)
	require.NoError(t, err)
	require.Empty(t, response.Errors)
	require.Len(t, response.Assessments, 1)
	assert.Equal(t, "REQ-1", response.Assessments[0].RequirementID)
	item := response.Assessments[0].Evidence[0]
	assert.FileExists(t, filepath.Join(root, item.Source.Coordinate))
	assertRetainedPrivate(t, filepath.Join(root, item.Source.Coordinate), 0600)
	log, err := workspace.ReadFile("tool.log")
	require.NoError(t, err)
	lines := strings.Split(string(log), "\n")
	assert.Equal(t, root, lines[0], "tool working directory must be the selected workspace")
	assert.Equal(t, root, lines[2], "relative local input must resolve against the selected workspace")
	assert.Contains(t, string(log), "--ignore")
	if configuration != "invocation-only" {
		assert.Contains(t, string(log), "(?:^$)|(?:")
	}
	after, err := os.Getwd()
	require.NoError(t, err)
	assert.Equal(t, before, after, "RPCs must never change process CWD")
	assert.DirExists(t, config.NewWorkspaceConfig(root).GeneratedDirPath())
}

func TestScan_WorkspaceAndIdentityValidationPrecedeStorage(t *testing.T) {
	for _, scenario := range []string{"invalid-scan", "conflicting-scan", "invalid-generate", "conflicting-generate"} {
		t.Run(scenario, func(t *testing.T) {
			root, other := t.TempDir(), t.TempDir()
			srv := New(ServerOptions{ToolChecker: func() ([]string, error) { t.Fatal("workspace validation must precede tools"); return nil, nil }})
			target := localTarget(t, "input.json", "")
			target.Variables[evidence.WorkspaceVariable] = root
			if strings.Contains(scenario, "generate") {
				value := other
				if scenario == "invalid-generate" {
					value = filepath.Join(root, "missing")
				}
				response, err := srv.Generate(context.Background(), &provider.GenerateRequest{
					GlobalVariables: map[string]string{evidence.WorkspaceVariable: value}, TargetVariables: target.Variables})
				require.Error(t, err)
				assert.Nil(t, response)
			} else {
				second := localTarget(t, "input.json", "")
				second.Variables[evidence.WorkspaceVariable] = other
				if scenario == "invalid-scan" {
					second.Variables[evidence.WorkspaceVariable] = filepath.Join(root, "missing")
				}
				response, err := srv.Scan(context.Background(), &provider.ScanRequest{Targets: []provider.Target{target, second}})
				require.Error(t, err)
				assert.Nil(t, response)
			}
			assert.NoDirExists(t, filepath.Join(root, provider.WorkspaceDir))
			assert.NoDirExists(t, filepath.Join(other, provider.WorkspaceDir))
		})
	}
}
