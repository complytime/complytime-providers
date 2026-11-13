// SPDX-License-Identifier: Apache-2.0

package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These opt-in tests use prebuilt binaries and an isolated network namespace.
// Set COMPLYCTL_TEST_BINARY and OPA_PROVIDER_TEST_BINARY to run them.
type complyctlFixture struct {
	root, home, data, cache, work, policy, fakeBin, realBin string
	complyctl, bubblewrap                                   string
}

func newComplyctlFixture(t *testing.T) *complyctlFixture {
	t.Helper()
	complyctl := os.Getenv("COMPLYCTL_TEST_BINARY")
	providerBinary := os.Getenv("OPA_PROVIDER_TEST_BINARY")
	if complyctl == "" || providerBinary == "" || runtime.GOOS != "linux" {
		t.Skip("offline complyctl and OPA provider binaries on Linux are required")
	}
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		t.Skip("bubblewrap network isolation is required")
	}
	f := &complyctlFixture{root: t.TempDir(), complyctl: complyctl, bubblewrap: bwrap}
	f.home = filepath.Join(f.root, "home")
	f.data = filepath.Join(f.root, "data")
	f.cache = filepath.Join(f.root, "cache")
	f.work = filepath.Join(f.root, "work")
	f.policy = filepath.Join(f.root, "policy")
	f.fakeBin = filepath.Join(f.root, "fakebin")
	f.realBin = filepath.Join(f.root, "realbin")
	for _, dir := range []string{f.home, f.work, f.policy, f.fakeBin, f.realBin,
		filepath.Join(f.data, "complytime", "providers")} {
		require.NoError(t, os.MkdirAll(dir, 0750))
	}
	require.NoError(t, os.Symlink(providerBinary,
		filepath.Join(f.data, "complytime", "providers", "complyctl-provider-opa")))
	f.seedPolicyCache(t)
	f.seedWorkspace(t, f.work)
	require.NoError(t, os.WriteFile(filepath.Join(f.policy, "policy.rego"), []byte(
		"package main\ndeny contains input.marker if input.marker != \"\"\n"), 0600))
	fake := `#!/bin/sh
case "$1" in
  --version) printf 'Conftest: %s\nOPA: 1.15.2\n' "${FAKE_VERSION}";;
  test) case "$2" in *empty*) echo 'Error: running test: parse files: no files found' >&2; exit 1;; esac
        printf '[{"filename":"input.json","namespace":"main","successes":1}]\n';;
  *) exit 1;;
esac
`
	// The isolated test executable must be runnable by its owner.
	require.NoError(t, os.WriteFile(filepath.Join(f.fakeBin, "conftest"), []byte(fake), 0700)) // #nosec G306
	return f
}

func (f *complyctlFixture) seedPolicyCache(t *testing.T) {
	t.Helper()
	store := filepath.Join(f.cache, "complytime", "policies", "test-policy")
	blobs := filepath.Join(store, "blobs", "sha256")
	require.NoError(t, os.MkdirAll(blobs, 0750))
	put := func(content []byte, mediaType string) map[string]any {
		t.Helper()
		sum := sha256.Sum256(content)
		digest := hex.EncodeToString(sum[:])
		require.NoError(t, os.WriteFile(filepath.Join(blobs, digest), content, 0600))
		return map[string]any{"mediaType": mediaType, "digest": "sha256:" + digest, "size": len(content)}
	}
	policy := []byte(`title: Test OPA Policy
metadata:
  id: test-policy
  version: "1.0.0"
adherence:
  evaluation-methods:
    - type: Behavioral
      executor: {id: opa, name: opa, type: Software}
  assessment-plans:
    - id: test-plan
      requirement-id: test-req
      frequency: continuous
      evaluation-methods:
        - type: Behavioral
          executor: {id: opa, name: opa, type: Software}
`)
	catalog := []byte("id: test-policy\ntitle: Test Catalog\ncontrols:\n  - id: test-req\n    title: Test requirement\n")
	config := put([]byte(`{}`), "application/vnd.oci.empty.v1+json")
	layers := []map[string]any{put(catalog, "application/vnd.gemara.catalog.v1+yaml"),
		put(policy, "application/vnd.gemara.policy.v1+yaml")}
	manifest, err := json.Marshal(map[string]any{"schemaVersion": 2,
		"mediaType": "application/vnd.oci.image.manifest.v1+json", "config": config, "layers": layers})
	require.NoError(t, err)
	descriptor := put(manifest, "application/vnd.oci.image.manifest.v1+json")
	descriptor["annotations"] = map[string]string{"org.opencontainers.image.ref.name": "v1.0.0"}
	index, err := json.Marshal(map[string]any{"schemaVersion": 2, "manifests": []any{descriptor}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(store, "index.json"), index, 0600))
	require.NoError(t, os.WriteFile(filepath.Join(store, "oci-layout"), []byte(`{"imageLayoutVersion":"1.0.0"}`), 0600))
}

func (f *complyctlFixture) seedWorkspace(t *testing.T, base string) {
	t.Helper()
	ws := filepath.Join(base, ".complytime")
	for _, dir := range []string{filepath.Join(ws, "generation"), filepath.Join(ws, "opa", "generated")} {
		require.NoError(t, os.MkdirAll(dir, 0750))
	}
	state := `{"policy_id":"test-policy","policy_digest":"","evaluator_ids":["opa"]}`
	require.NoError(t, os.WriteFile(filepath.Join(ws, "generation", "test-policy.json"), []byte(state), 0600))
	scanConfig, err := json.Marshal(map[string]any{"ids": []string{"main"},
		"reverse_mapping": map[string]string{"main": "test-req"}, "bundle_dir": f.policy})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(ws, "opa", "generated", "scan-config.json"), scanConfig, 0600))
}

func (f *complyctlFixture) scan(t *testing.T, cwd, workspace, binDir, fakeVersion string) (string, error) {
	t.Helper()
	return f.command(t, cwd, workspace, binDir, map[string]string{"FAKE_VERSION": fakeVersion},
		"scan", "--policy-id", "test-policy")
}

func (f *complyctlFixture) command(
	t *testing.T, cwd, workspace, binDir string, environment map[string]string, command ...string,
) (string, error) {
	t.Helper()
	args := []string{"--unshare-net", "--bind", "/", "/", "--dev-bind", "/dev", "/dev", "--proc", "/proc",
		"--chdir", cwd, "--setenv", "HOME", f.home, "--setenv", "XDG_DATA_HOME", f.data,
		"--setenv", "XDG_CACHE_HOME", f.cache, "--setenv", "PATH", binDir + ":/usr/bin:/bin",
		"--setenv", "COMPLYTIME_WORKSPACE", "", "--setenv", "CONFTEST_IGNORE", "",
		"--setenv", "CONFTEST_CONFIG_FILE", ""}
	for name, value := range environment {
		args = append(args, "--setenv", name, value)
	}
	args = append(args, "--", f.complyctl)
	args = append(args, command...)
	if workspace != "" {
		args = append(args, "--workspace", workspace)
	}
	// This opt-in test executes explicitly supplied local binaries without a shell.
	output, err := exec.Command(f.bubblewrap, args...).CombinedOutput() // #nosec G204
	return string(output), err
}

func TestComplyctlScan_OfflineOperationalErrors(t *testing.T) {
	f := newComplyctlFixture(t)
	input := filepath.Join(f.work, "input")
	empty := filepath.Join(f.work, "empty")
	require.NoError(t, os.MkdirAll(input, 0750))
	require.NoError(t, os.MkdirAll(empty, 0750))
	inputFile := filepath.Join(input, "input.json")
	require.NoError(t, os.WriteFile(inputFile, []byte(`{}`), 0600))
	workspace := filepath.Join(f.work, ".complytime")
	artifact := filepath.Join(workspace, "artifact.json")
	require.NoError(t, os.WriteFile(artifact, []byte(`{}`), 0600))
	require.NoError(t, os.Symlink(artifact, filepath.Join(input, "alias.json")))
	config := fmt.Sprintf(`policies:
  - url: example.test/test-policy:v1.0.0
    id: test-policy
targets:
  - id: workspace-target
    policies: [test-policy]
    variables: {input_path: %q}
  - id: alias-target
    policies: [test-policy]
    variables: {input_path: %q}
  - id: empty-target
    policies: [test-policy]
    variables: {input_path: %q}
  - id: valid-target
    policies: [test-policy]
    variables: {input_path: %q}
`, workspace, input, empty, inputFile)
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "complytime.yaml"), []byte(config), 0600))
	for _, version := range []string{"0.69.0", "dev", ""} {
		t.Run("version="+version, func(t *testing.T) {
			output, err := f.scan(t, f.work, "", f.fakeBin, version)
			require.Error(t, err)
			for _, expected := range []string{"workspace-target:", "alias-target:", "empty-target:",
				"copy intended input outside the workspace", "no eligible input", "no files found",
				"add a supported input or adjust ignore settings",
				"valid-target  test-req", "1 requirements: 1 passed"} {
				assert.Contains(t, output, expected)
			}
			assert.NotContains(t, output, "unverified conftest version")
		})
	}
}

func TestComplyctlScan_OfflineDistinctWorkspaceBoundaries(t *testing.T) {
	f := newComplyctlFixture(t)
	conftest := os.Getenv("CONFTEST_TEST_BINARY")
	if conftest == "" {
		t.Skip("a conftest binary is required")
	}
	require.NoError(t, os.Symlink(conftest, filepath.Join(f.realBin, "conftest")))
	enclosing := filepath.Join(f.root, "enclosing")
	host := filepath.Join(enclosing, "host")
	cwd := filepath.Join(enclosing, "runner")
	f.seedWorkspace(t, host)
	f.seedWorkspace(t, cwd)
	require.NoError(t, os.WriteFile(filepath.Join(enclosing, "eligible.json"),
		[]byte(`{"marker":"eligible-only"}`), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(host, ".complytime", "artifact.json"),
		[]byte(`{"marker":"workspace-host"}`), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(cwd, ".complytime", "artifact.json"), []byte(`{invalid`), 0600))
	config := fmt.Sprintf(`policies:
  - url: example.test/test-policy:v1.0.0
    id: test-policy
targets:
  - id: boundaries-target
    policies: [test-policy]
    variables: {input_path: %q}
`, enclosing)
	require.NoError(t, os.WriteFile(filepath.Join(host, ".complytime", "complytime.yaml"), []byte(config), 0600))
	output, err := f.scan(t, cwd, host, f.realBin, "")
	require.NoError(t, err, "%s", output)
	assert.Contains(t, output, "boundaries-target  test-req")
	assert.Contains(t, output, "eligible-only")
	assert.NotContains(t, output, "workspace-host")
}
