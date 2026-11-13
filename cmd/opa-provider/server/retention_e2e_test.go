// SPDX-License-Identifier: Apache-2.0

package server

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These wire types deliberately read serialized host output, not provider structs.
type retainedLogEvidence struct {
	ID          string `json:"id" yaml:"id"`
	CollectedAt string `json:"collected-at" yaml:"collected-at"`
	Source      struct {
		ReferenceID string `json:"reference-id" yaml:"reference-id"`
		Coordinate  string `json:"coordinate" yaml:"coordinate"`
		Digest      string `json:"digest" yaml:"digest"`
	} `json:"source" yaml:"source"`
}

type retainedEvaluationLog struct {
	Metadata struct {
		References []struct {
			ID  string `json:"id" yaml:"id"`
			URL string `json:"url" yaml:"url"`
		} `json:"mapping-references" yaml:"mapping-references"`
	} `json:"metadata" yaml:"metadata"`
	Evaluations []struct {
		Assessments []struct {
			Evidence []retainedLogEvidence `json:"evidence" yaml:"evidence"`
		} `json:"assessment-logs" yaml:"assessment-logs"`
	} `json:"evaluations" yaml:"evaluations"`
}

func (f *complyctlFixture) seedRetainedComplypack(t *testing.T) {
	t.Helper()
	directory := filepath.Join(f.cache, "complytime", "complypacks", "opa", "1.0.0")
	require.NoError(t, os.MkdirAll(directory, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(directory, "config.json"),
		[]byte(`{"evaluator-id":"opa","version":"1.0.0"}`), 0600))
	file, err := os.Create(filepath.Join(directory, "content.tar.gz"))
	require.NoError(t, err)
	compressed := gzip.NewWriter(file)
	archive := tar.NewWriter(compressed)
	for name, content := range map[string]string{
		"policy.rego": "package main\ndeny contains input.marker if input.marker != \"\"\n",
		"complytime-mapping.json": `{"version":"1","mappings":[` +
			`{"id":"main","requirement_id":"test-req"}]}`,
	} {
		require.NoError(t, archive.WriteHeader(&tar.Header{
			Name: name, Mode: 0600, Size: int64(len(content)),
		}))
		_, err := archive.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, archive.Close())
	require.NoError(t, compressed.Close())
	require.NoError(t, file.Close())
}

func assertRetainedLog(
	t *testing.T, content []byte, root string,
) retainedLogEvidence {
	t.Helper()
	var log retainedEvaluationLog
	require.NoError(t, yaml.Unmarshal(content, &log))
	require.Len(t, log.Evaluations, 1)
	require.Len(t, log.Evaluations[0].Assessments, 1)
	require.Len(t, log.Evaluations[0].Assessments[0].Evidence, 1)
	evidence := log.Evaluations[0].Assessments[0].Evidence[0]
	assert.Regexp(t, `^conftest_[a-f0-9]{32}$`, evidence.ID)
	assert.Equal(t, "conftest-result_"+strings.TrimPrefix(evidence.ID, "conftest_"),
		evidence.Source.ReferenceID)
	assert.False(t, filepath.IsAbs(evidence.Source.Coordinate))
	assert.True(t, strings.HasPrefix(evidence.Source.Coordinate, ".complytime/opa/results/"))
	assert.Equal(t, "conftest.json", filepath.Base(evidence.Source.Coordinate))
	path := filepath.Join(root, evidence.Source.Coordinate)
	for _, item := range []struct {
		name string
		mode os.FileMode
	}{
		{filepath.Dir(path), 0700},
		{path, 0600},
		{filepath.Join(filepath.Dir(path), "metadata.json"), 0600},
	} {
		info, err := os.Stat(item.name)
		require.NoError(t, err)
		assert.Equal(t, item.mode, info.Mode().Perm())
		owner, ok := info.Sys().(*syscall.Stat_t)
		require.True(t, ok)
		assert.Equal(t, int64(os.Geteuid()), int64(owner.Uid))
		assert.Equal(t, int64(os.Getegid()), int64(owner.Gid))
	}
	bytes, err := os.ReadFile(path)
	require.NoError(t, err)
	sum := sha256.Sum256(bytes)
	assert.Equal(t, fmt.Sprintf("sha256:%x", sum), evidence.Source.Digest)
	collected, err := time.Parse(time.RFC3339, evidence.CollectedAt)
	require.NoError(t, err)
	assert.Contains(t, filepath.Base(filepath.Dir(path)), collected.UTC().Format("20060102T150405Z"))
	require.Len(t, log.Metadata.References, 1)
	reference := log.Metadata.References[0]
	assert.Equal(t, evidence.Source.ReferenceID, reference.ID)
	locator, err := url.Parse(reference.URL)
	require.NoError(t, err)
	assert.Equal(t, "file", locator.Scheme)
	assert.Empty(t, locator.Host)
	assert.Equal(t, path, locator.Path)
	assert.Equal(t, (&url.URL{Scheme: "file", Path: path}).String(), reference.URL)
	metadata, err := os.ReadFile(filepath.Join(filepath.Dir(path), "metadata.json"))
	require.NoError(t, err)
	var context struct {
		CollectedAt string `json:"collected_at"`
	}
	require.NoError(t, json.Unmarshal(metadata, &context))
	assert.Equal(t, evidence.CollectedAt, context.CollectedAt)
	return evidence
}

func TestComplyctlScan_OfflineRetainedEvidence(t *testing.T) {
	for _, selection := range []string{"default", "environment", "flag"} {
		t.Run(selection, func(t *testing.T) {
			f := newComplyctlFixture(t)
			conftest := os.Getenv("CONFTEST_TEST_BINARY")
			if conftest == "" {
				t.Skip("a conftest binary is required")
			}
			require.NoError(t, os.Symlink(conftest, filepath.Join(f.realBin, "conftest")))
			f.seedRetainedComplypack(t)
			root, flag := f.work, ""
			environment := map[string]string{}
			if selection != "default" {
				root = filepath.Join(f.root, "selected space % café")
				f.seedWorkspace(t, root)
				if selection == "flag" {
					flag = root
					environment["COMPLYTIME_WORKSPACE"] = filepath.Join(f.root, "must-not-be-selected")
				} else {
					environment["COMPLYTIME_WORKSPACE"] = root
				}
			}
			input := filepath.Join(root, "input")
			require.NoError(t, os.MkdirAll(input, 0750))
			config := `policies:
  - url: example.test/test-policy:v1.0.0
    id: test-policy
targets:
  - id: retained-target
    policies: [test-policy]
    variables: {input_path: input}
`
			require.NoError(t, os.WriteFile(filepath.Join(root, ".complytime", "complytime.yaml"),
				[]byte(config), 0600))
			output, err := f.command(t, f.work, flag, f.realBin, environment,
				"generate", "--policy-id", "test-policy")
			require.NoError(t, err, "%s", output)
			index := filepath.Join(root, ".complytime", "opa", "generated", "generate-index.json")
			_, err = os.Stat(index)
			require.NoError(t, err)
			var history [][]byte
			var digests []string
			seen := map[string]bool{}
			for scan, marker := range []string{"first-output", "changed-output", "changed-output"} {
				require.NoError(t, os.WriteFile(filepath.Join(input, "input.json"),
					[]byte(fmt.Sprintf(`{"marker":%q}`, marker)), 0600))
				format := "json"
				if scan == 1 {
					format = "yaml"
				}
				output, err = f.command(t, f.work, flag, f.realBin, environment,
					"scan", "--policy-id", "test-policy", "--format", "oscal", "--log-format", format)
				require.NoError(t, err, "%s", output)
				logs, err := filepath.Glob(filepath.Join(root, ".complytime", "scan", "evaluation-log-*."+format))
				require.NoError(t, err)
				require.NotEmpty(t, logs)
				// The host can reuse second-resolution log names; preserve snapshots
				// before the next scan independently of provider artifact retention.
				var latest []byte
				for _, path := range logs {
					bytes, err := os.ReadFile(path)
					require.NoError(t, err)
					evidence := assertRetainedLog(t, bytes, root)
					if !seen[evidence.ID] {
						latest = bytes
						seen[evidence.ID] = true
					}
				}
				require.NotEmpty(t, latest, "each scan must retain a distinct artifact instance")
				evidence := assertRetainedLog(t, latest, root)
				bytes, err := os.ReadFile(filepath.Join(root, evidence.Source.Coordinate))
				require.NoError(t, err)
				assert.Contains(t, string(bytes), marker)
				digests = append(digests, evidence.Source.Digest)
				history = append(history, latest)
				for _, prior := range history {
					assertRetainedLog(t, prior, root)
				}
				oscal, err := filepath.Glob(filepath.Join(root, ".complytime", "scan", "assessment-results-*.json"))
				require.NoError(t, err)
				require.NotEmpty(t, oscal, "unchanged downstream OSCAL consumer must accept the log")
				assertRetainedOSCAL(t, oscal, latest, root)
				if module := os.Getenv("GEMARA_CUE_TEST_MODULE"); module != "" {
					validateRetainedSchema(t, f, scan, latest, format, module)
				}
			}
			assert.Len(t, seen, 3)
			assert.NotEqual(t, digests[0], digests[1])
			assert.Equal(t, digests[1], digests[2])
			if selection != "default" {
				for _, directory := range []string{"results", "generated"} {
					files, err := filepath.Glob(filepath.Join(f.work, ".complytime", "opa", directory, "*"))
					require.NoError(t, err)
					if directory == "generated" {
						assert.Equal(t, []string{filepath.Join(f.work, ".complytime", "opa", directory, "scan-config.json")}, files)
					} else {
						assert.Empty(t, files)
					}
				}
			}
		})
	}
}

func assertRetainedOSCAL(t *testing.T, paths []string, log []byte, root string) {
	t.Helper()
	evidence := assertRetainedLog(t, log, root)
	for _, path := range paths {
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		if !strings.Contains(string(content), evidence.Source.ReferenceID) {
			continue
		}
		assert.Contains(t, string(content), evidence.Source.Coordinate)
		assert.Contains(t, string(content), evidence.Source.Digest)
		locator := (&url.URL{Scheme: "file", Path: filepath.Join(root, evidence.Source.Coordinate)}).String()
		assert.Contains(t, string(content), locator)
		return
	}
	t.Fatal("OSCAL output lost the retained artifact reference")
}

func validateRetainedSchema(
	t *testing.T, f *complyctlFixture, scan int, content []byte, format, module string,
) {
	t.Helper()
	root, err := os.OpenRoot(f.root)
	require.NoError(t, err)
	name := fmt.Sprintf("schema-scan-%d.%s", scan, format)
	require.NoError(t, root.WriteFile(name, content, 0600))
	require.NoError(t, root.Close())
	path := filepath.Join(f.root, name)
	// Schema checking is separately opted in because cue may fetch the schema.
	// The operator explicitly selects the module; arguments never pass through a shell.
	output, err := exec.Command("cue", "vet", "-c", "-d", "#EvaluationLog", module, path).CombinedOutput() // #nosec G204 G702
	require.NoError(t, err, "%s", output)
}
