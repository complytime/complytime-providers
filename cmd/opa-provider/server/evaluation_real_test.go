// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/complytime/complytime-providers/cmd/opa-provider/config"
	"github.com/complytime/complytime-providers/cmd/opa-provider/generate"
	"github.com/complytime/complytime-providers/cmd/opa-provider/loader"
)

func TestScan_RealConftestRequiresEvaluatedInput(t *testing.T) {
	requireRealConftest(t)
	for _, scenario := range []string{
		"empty", "workspace only", "fully ignored", "remote empty clone", "remote empty scan path",
		"evaluated", "evaluated with zero checks",
	} {
		t.Run(scenario, func(t *testing.T) {
			base := t.TempDir()
			t.Chdir(base)
			t.Setenv("HOME", base)
			t.Setenv(conftestConfigFileEnv, "")
			t.Setenv(conftestIgnoreEnv, "")
			input := filepath.Join(base, "input")
			workspace := filepath.Join(base, "workspace")
			if scenario == "workspace only" {
				workspace = filepath.Join(input, "workspace")
			}
			require.NoError(t, os.MkdirAll(input, 0750))
			policy := filepath.Join(base, "policy")
			require.NoError(t, os.Mkdir(policy, 0750))
			require.NoError(t, os.WriteFile(filepath.Join(policy, "policy.rego"), []byte(
				"package main\ndeny contains \"blocked\" if input.blocked == true\n",
			), 0600))
			if scenario == "evaluated with zero checks" {
				require.NoError(t, os.WriteFile(filepath.Join(policy, "policy.rego"), []byte(
					"package main\nfoo := true\n"), 0600))
			}
			cfg := config.NewWorkspaceConfig(workspace)
			require.NoError(t, os.MkdirAll(workspace, 0750))
			require.NoError(t, cfg.EnsureDirectories())
			require.NoError(t, generate.WriteScanConfig(cfg.GeneratedDirPath(), []string{"main"},
				map[string]string{"main": "test-req"}, policy))
			if scenario == "fully ignored" || strings.HasPrefix(scenario, "evaluated") {
				require.NoError(t, os.WriteFile(filepath.Join(input, "input.json"), []byte(`{"blocked":false}`), 0600))
			}
			if scenario == "fully ignored" {
				t.Setenv(conftestIgnoreEnv, ".*")
			}
			target := localTarget(t, input, "")
			opts := ServerOptions{WorkspaceDir: workspace, ToolChecker: func() ([]string, error) { return nil, nil }}
			if strings.HasPrefix(scenario, "remote ") {
				clone := filepath.Join(workspace, "clone")
				require.NoError(t, os.Mkdir(clone, 0750))
				scanPath := ""
				if scenario == "remote empty scan path" {
					scanPath = "empty"
					require.NoError(t, os.Mkdir(filepath.Join(clone, scanPath), 0750))
				}
				opts.Loader = &mockLoader{loadFn: func(provider.Target, string) (string, error) {
					return filepath.Join(clone, scanPath), nil
				}}
				target = remoteTarget("", "https://example.com/org/repo")
				target.Variables[loader.VarScanPath] = scanPath
			}
			srv := newWorkspaceTestServer(t, opts)
			response, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{target}))
			require.NoError(t, err)
			if strings.HasPrefix(scenario, "evaluated") {
				assert.Empty(t, response.Errors)
				require.Len(t, response.Assessments, 1)
				assert.Equal(t, "test-req", response.Assessments[0].RequirementID)
				assert.Equal(t, provider.ResultPassed, response.Assessments[0].Steps[0].Result)
				return
			}
			require.Len(t, response.Errors, 1)
			assert.Contains(t, response.Errors[0], target.TargetID)
			assert.Contains(t, response.Errors[0], "no eligible input")
			assert.Contains(t, response.Errors[0], "add a supported input or adjust ignore settings")
			assert.Contains(t, response.Errors[0], "no files found")
			assert.Empty(t, response.Assessments)
		})
	}
}
