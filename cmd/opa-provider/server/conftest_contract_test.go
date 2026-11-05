// SPDX-License-Identifier: Apache-2.0

package server

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequireRealConftest_VersionIsDiagnosticOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture requires a POSIX shell")
	}
	for _, version := range []string{"0.71.1", "dev", "unknown", "unavailable"} {
		reached := false
		t.Run(version, func(t *testing.T) {
			bin := t.TempDir()
			path := filepath.Join(bin, "conftest")
			script := "#!/bin/sh\nprintf 'Conftest: %s\\n' \"$TEST_CONFTEST_VERSION\"\n"
			if version == "unavailable" {
				script += "exit 1\n"
			}
			// Only the owner executes this isolated test fixture.
			require.NoError(t, os.WriteFile(path, []byte(script), 0700)) // #nosec G306
			t.Setenv("PATH", bin)
			t.Setenv("TEST_CONFTEST_VERSION", version)
			assert.Equal(t, path, requireRealConftest(t))
			reached = true
		})
		require.True(t, reached, "an installed executable must not skip on version metadata")
	}
}

func realContractFixture(t *testing.T) (string, string, string) {
	t.Helper()
	binary := requireRealConftest(t)
	base := t.TempDir()
	isolateRealConftest(t, base)
	input := filepath.Join(base, "input")
	policy := filepath.Join(base, "policy")
	require.NoError(t, os.Mkdir(input, 0750))
	require.NoError(t, os.Mkdir(policy, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(policy, "policy.rego"), []byte(
		"package main\ndeny contains \"blocked\" if input.blocked == true\n"), 0600))
	for _, name := range []string{
		"eligible.json", "file-ignored.json", "env-ignored.json", "flag-ignored.json",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(input, name), []byte(`{"blocked":false}`), 0600))
	}
	return binary, input, policy
}

func realContractCommand(binary, input, policy string, extra ...string) ([]byte, error) {
	args := []string{
		"test", input, "--policy", policy, "--namespace", "main", "--output", "json", "--no-fail",
	}
	args = append(args, extra...)
	return exec.Command(binary, args...).CombinedOutput() // #nosec G204 -- hermetic local fixtures
}

func assertContractFiles(t *testing.T, output []byte, want []string) {
	t.Helper()
	records := decodeRealCall(t, realConftestCall{output: output})
	var filenames []string
	for _, record := range records {
		filenames = append(filenames, filepath.Base(record.Filename))
		assert.Equal(t, "main", record.Namespace)
		assert.Equal(t, 1, record.Successes)
		assert.Empty(t, record.Failures)
		assert.Empty(t, record.Warnings)
	}
	assert.ElementsMatch(t, want, filenames)
}

func TestScan_RealConftestIgnorePrecedenceContract(t *testing.T) {
	for _, source := range []string{"config", "environment", "flag", "explicit file"} {
		t.Run(source, func(t *testing.T) {
			binary, input, policy := realContractFixture(t)
			require.NoError(t, os.WriteFile("conftest.toml", []byte(`ignore = 'file-ignored.json$'`), 0600))
			want := []string{"eligible.json", "env-ignored.json", "flag-ignored.json"}
			var extra []string
			if source != "config" {
				t.Setenv(conftestIgnoreEnv, `env-ignored\.json$`)
				want = []string{"eligible.json", "file-ignored.json", "flag-ignored.json"}
			}
			if source == "flag" || source == "explicit file" {
				extra = []string{"--ignore", `flag-ignored\.json$`}
				want = []string{"eligible.json", "file-ignored.json", "env-ignored.json"}
			}
			if source == "explicit file" {
				input = filepath.Join(input, "flag-ignored.json")
				want = []string{"flag-ignored.json"}
			}
			output, err := realContractCommand(binary, input, policy, extra...)
			if source == "explicit file" && err != nil {
				// Releases differ on whether explicit files honor ignore. Either
				// evaluating only that file or rejecting empty input is safe; the
				// provider must not replace directory traversal with a file list.
				assert.Contains(t, string(output), conftestNoFilesFound)
				return
			}
			require.NoError(t, err, "%s", output)
			assertContractFiles(t, output, want)
		})
	}
}

func TestScan_RealConftestConfigDiscoveryContract(t *testing.T) {
	for _, name := range []string{
		"conftest.json", "conftest.toml", "conftest.yaml", "conftest.yml",
		"conftest.properties", "conftest.props", "conftest.prop", "conftest.hcl",
		"conftest.tfvars", "conftest.dotenv", "conftest.env", "conftest.ini",
		"conftest.backup",
	} {
		t.Run(name, func(t *testing.T) {
			binary, input, policy := realContractFixture(t)
			content := "ignore=file-ignored.json$\n"
			switch filepath.Ext(name) {
			case ".json":
				content = `{"ignore":"file-ignored.json$"}`
			case ".toml", ".hcl", ".tfvars":
				content = `ignore = "file-ignored.json$"`
			case ".yaml", ".yml":
				content = `ignore: 'file-ignored.json$'`
			}
			require.NoError(t, os.WriteFile(name, []byte(content), 0600))
			detected, err := detectConftestConfig("")
			require.NoError(t, err)
			output, runErr := realContractCommand(binary, input, policy)
			if name == "conftest.backup" {
				assert.Empty(t, detected)
				require.NoError(t, runErr, "%s", output)
				assertContractFiles(t, output, []string{
					"eligible.json", "file-ignored.json", "env-ignored.json", "flag-ignored.json"})
				return
			}
			assert.Equal(t, name, filepath.Base(detected))
			if runErr != nil {
				// Discovery can find formats without a registered decoder. They
				// still apply: conftest must fail reading configuration, not scan it.
				assert.Contains(t, string(output), "config")
				return
			}
			assertContractFiles(t, output, []string{
				"eligible.json", "env-ignored.json", "flag-ignored.json"})
		})
	}
}

func TestScan_RealConftestExplicitConfigAndDiscoveryOrderContract(t *testing.T) {
	for _, explicit := range []string{"", "relative", "absolute"} {
		t.Run(fmt.Sprintf("explicit=%s", explicit), func(t *testing.T) {
			binary, input, policy := realContractFixture(t)
			require.NoError(t, os.WriteFile("conftest.json", []byte(
				`{"ignore":"file-ignored.json$"}`), 0600))
			require.NoError(t, os.WriteFile("conftest.toml", []byte(
				`ignore = 'env-ignored.json$'`), 0600))
			path := ""
			want := []string{"eligible.json", "env-ignored.json", "flag-ignored.json"}
			if explicit != "" {
				path = "custom.toml"
				require.NoError(t, os.WriteFile(path, []byte(`ignore = 'flag-ignored.json$'`), 0600))
				if explicit == "absolute" {
					var err error
					path, err = filepath.Abs(path)
					require.NoError(t, err)
				}
				t.Setenv(conftestConfigFileEnv, path)
				want = []string{"eligible.json", "file-ignored.json", "env-ignored.json"}
			}
			detected, err := detectConftestConfig(path)
			require.NoError(t, err)
			if explicit == "" {
				assert.Equal(t, "conftest.json", filepath.Base(detected))
			} else {
				assert.Equal(t, "custom.toml", filepath.Base(detected))
			}
			output, err := realContractCommand(binary, input, policy)
			require.NoError(t, err, "%s", output)
			assertContractFiles(t, output, want)
		})
	}
}
