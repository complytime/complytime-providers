// SPDX-License-Identifier: Apache-2.0

package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func retainedConfigFixture(t *testing.T, input string) (*complyctlFixture, string) {
	t.Helper()
	f := newComplyctlFixture(t)
	conftest := os.Getenv("CONFTEST_TEST_BINARY")
	if conftest == "" {
		t.Skip("a conftest binary is required")
	}
	require.NoError(t, os.Symlink(conftest, filepath.Join(f.realBin, "conftest")))
	f.seedRetainedComplypack(t)
	workspace := filepath.Join(f.root, "selected workspace")
	f.seedWorkspace(t, workspace)
	configuration := fmt.Sprintf(`policies:
  - url: example.test/test-policy:v1.0.0
    id: test-policy
targets:
  - id: config-target
    policies: [test-policy]
    variables: {input_path: %q}
`, input)
	require.NoError(t, os.WriteFile(filepath.Join(workspace, ".complytime", "complytime.yaml"),
		[]byte(configuration), 0600))
	output, err := f.command(t, f.work, workspace, f.realBin, nil,
		"generate", "--policy-id", "test-policy")
	require.NoError(t, err, "%s", output)
	// Either discovery or an explicit relative path resolved against invocation
	// CWD would encounter invalid configuration instead of the workspace fixture.
	for _, base := range []string{f.work, workspace} {
		require.NoError(t, os.MkdirAll(filepath.Join(base, "configs"), 0750))
	}
	for _, name := range []string{"conftest.toml", filepath.Join("configs", "settings.toml")} {
		require.NoError(t, os.WriteFile(filepath.Join(f.work, name), []byte("= invalid TOML"), 0600))
	}
	return f, workspace
}

func assertRetainedConfigInput(t *testing.T, workspace string, expected []string) {
	t.Helper()
	pattern := filepath.Join(workspace, ".complytime", "scan", "evaluation-log-*.json")
	logs, err := filepath.Glob(pattern)
	require.NoError(t, err)
	var evidenceLogs [][]byte
	for _, path := range logs {
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		var log retainedEvaluationLog
		require.NoError(t, json.Unmarshal(content, &log))
		if len(log.Metadata.References) != 0 {
			evidenceLogs = append(evidenceLogs, content)
		} else {
			assertNoRetainedConfigEvidence(t, content)
		}
	}
	require.Len(t, evidenceLogs, 1, "exactly one scan must publish retained evidence")
	evidence := assertRetainedLog(t, evidenceLogs[0], workspace)
	content, err := os.ReadFile(filepath.Join(workspace, evidence.Source.Coordinate))
	require.NoError(t, err)
	var records []struct {
		Filename string `json:"filename"`
		Failures []struct {
			Message string `json:"msg"`
		} `json:"failures"`
	}
	require.NoError(t, json.Unmarshal(content, &records))
	var names []string
	for _, record := range records {
		name := filepath.Base(record.Filename)
		names = append(names, name)
		require.Len(t, record.Failures, 1)
		assert.Equal(t, "workspace-"+name, record.Failures[0].Message)
	}
	sort.Strings(names)
	assert.Equal(t, expected, names)
}

func assertNoRetainedConfigEvidence(t *testing.T, content []byte) {
	t.Helper()
	var log retainedEvaluationLog
	require.NoError(t, json.Unmarshal(content, &log))
	assert.Empty(t, log.Metadata.References)
	for _, evaluation := range log.Evaluations {
		for _, assessment := range evaluation.Assessments {
			assert.Empty(t, assessment.Evidence)
		}
	}
}

func assertNoInvocationRetention(t *testing.T, f *complyctlFixture) {
	t.Helper()
	assert.NoDirExists(t, filepath.Join(f.work, ".complytime", "opa", "results"))
	index := filepath.Join(f.work, ".complytime", "opa", "generated", "generate-index.json")
	assert.NoFileExists(t, index)
	assert.NoDirExists(t, filepath.Join(f.work, ".complytime", "scan"))
	content, err := os.ReadFile(filepath.Join(f.work, "conftest.toml"))
	require.NoError(t, err)
	assert.Equal(t, "= invalid TOML", string(content))
}

func TestComplyctlScan_OfflineRetainedConfigSelection(t *testing.T) {
	for _, scenario := range []struct {
		name        string
		environment map[string]string
		ignored     string
	}{
		{"discovery", nil, "file-ignored.json"},
		{"relative-explicit", map[string]string{
			"CONFTEST_CONFIG_FILE": "configs/settings.toml",
		}, "explicit-ignored.json"},
		{"environment-precedence", map[string]string{
			"CONFTEST_CONFIG_FILE": "configs/settings.toml", "CONFTEST_IGNORE": "env-ignored.json$",
		}, "env-ignored.json"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			f, workspace := retainedConfigFixture(t, "input")
			for _, config := range []struct{ name, content string }{
				{"conftest.toml", `ignore = 'file-ignored.json$'`},
				{filepath.Join("configs", "settings.toml"), `ignore = 'explicit-ignored.json$'`},
			} {
				require.NoError(t, os.WriteFile(filepath.Join(workspace, config.name),
					[]byte(config.content), 0600))
			}
			var expected []string
			for _, base := range []string{f.work, workspace} {
				require.NoError(t, os.MkdirAll(filepath.Join(base, "input"), 0750))
				for _, name := range []string{
					"eligible.json", "file-ignored.json", "explicit-ignored.json", "env-ignored.json",
				} {
					content := "{invalid input"
					if base == workspace && name != scenario.ignored {
						content = fmt.Sprintf(`{"marker":%q}`, "workspace-"+name)
						expected = append(expected, name)
					}
					require.NoError(t, os.WriteFile(filepath.Join(base, "input", name), []byte(content), 0600))
				}
			}
			sort.Strings(expected)
			output, err := f.command(t, f.work, workspace, f.realBin, scenario.environment,
				"scan", "--policy-id", "test-policy", "--log-format", "json")
			require.NoError(t, err, "%s", output)
			assertRetainedConfigInput(t, workspace, expected)
			assertNoInvocationRetention(t, f)
		})
	}
}

func TestComplyctlScan_OfflineRetainedConfigFailClosed(t *testing.T) {
	for _, scenario := range []struct {
		name        string
		environment map[string]string
		diagnostic  string
	}{
		{"discovery-ambiguous", map[string]string{}, "may define an ignore rule"},
		{"relative-explicit-ambiguous", map[string]string{
			"CONFTEST_CONFIG_FILE": "configs/settings.toml",
		}, "may define an ignore rule"},
		{"missing-relative-config", map[string]string{
			"CONFTEST_CONFIG_FILE": "configs/missing.toml",
		}, "cannot determine conftest configuration"},
		{"invalid-ignore", map[string]string{"CONFTEST_IGNORE": "["}, "invalid CONFTEST_IGNORE"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			f, workspace := retainedConfigFixture(t, ".")
			for _, name := range []string{"conftest.toml", filepath.Join("configs", "settings.toml")} {
				require.NoError(t, os.WriteFile(filepath.Join(workspace, name),
					[]byte(`ignore = 'ignored.json$'`), 0600))
			}
			require.NoError(t, os.WriteFile(filepath.Join(workspace, "eligible.json"),
				[]byte(`{"marker":"workspace-eligible.json"}`), 0600))
			ignored := filepath.Join(workspace, "ignored.json")
			require.NoError(t, os.WriteFile(ignored, []byte("{invalid"), 0600))
			output, err := f.command(t, f.work, workspace, f.realBin, scenario.environment,
				"scan", "--policy-id", "test-policy", "--log-format", "json")
			require.Error(t, err, "%s", output)
			assert.Contains(t, output, "config-target:")
			assert.Contains(t, output, scenario.diagnostic)
			results, err := filepath.Glob(filepath.Join(workspace, ".complytime", "opa", "results", "*"))
			require.NoError(t, err)
			assert.Empty(t, results, "configuration rejection must precede evaluation allocation")
			logs, err := filepath.Glob(filepath.Join(workspace, ".complytime", "scan", "evaluation-log-*"))
			require.NoError(t, err)
			require.Len(t, logs, 1)
			content, err := os.ReadFile(logs[0])
			require.NoError(t, err)
			// The unchanged host serializes operational failures too. Its log
			// must contain no evidence or references from the rejected target.
			assertNoRetainedConfigEvidence(t, content)

			// Apply the documented remedy in the same fixture. Explicitly account
			// for the file's rule and exclude configuration inputs; provider-added
			// workspace exclusion must still protect generated and retained files.
			environment := map[string]string{
				"CONFTEST_CONFIG_FILE": "configs/settings.toml",
				"CONFTEST_IGNORE":      `ignored.json$|conftest\.toml$|configs/`,
			}
			output, err = f.command(t, f.work, workspace, f.realBin, environment,
				"scan", "--policy-id", "test-policy", "--log-format", "json")
			require.NoError(t, err, "%s", output)
			assertRetainedConfigInput(t, workspace, []string{"eligible.json"})
			assertNoInvocationRetention(t, f)
		})
	}
}
