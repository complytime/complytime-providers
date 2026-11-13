// SPDX-License-Identifier: Apache-2.0

package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/antchfx/xmlquery"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/complytime/complytime-providers/cmd/openscap-provider/config"
	"github.com/complytime/complytime-providers/cmd/openscap-provider/xccdf"
	"github.com/complytime/complytime-providers/internal/evidence"
)

func TestMapResultStatus(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedResult provider.Result
		expectedError  error
	}{
		{"pass", "pass", provider.ResultPassed, nil},
		{"fixed", "fixed", provider.ResultPassed, nil},
		{"fail", "fail", provider.ResultFailed, nil},
		{"notapplicable", "notapplicable", provider.ResultSkipped, nil},
		{"error", "error", provider.ResultError, nil},
		{"unknown", "unknown", provider.ResultError, nil},
		{"invalid", "invalid", provider.ResultError, errors.New("couldn't match invalid")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mapResultStatus(tt.input)
			assert.Equal(t, tt.expectedResult, result)
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

const retainedARFFixture = `<root xmlns:ds="http://scap.nist.gov/schema/scap/source/1.2"
 xmlns:xccdf-1.2="http://checklists.nist.gov/xccdf/1.2">
 <target>observed-host</target><ds:component><xccdf-1.2:Benchmark>
 <xccdf-1.2:Rule id="rule_a"><xccdf-1.2:title>Rule A</xccdf-1.2:title>
 <xccdf-1.2:check system="http://oval.mitre.org/XMLSchema/oval-definitions-5">
 <xccdf-1.2:check-content-ref name="oval:ssg-rule_a:def:1"/>
 </xccdf-1.2:check></xccdf-1.2:Rule>
 <xccdf-1.2:Rule id="rule_b"><xccdf-1.2:title>Rule B</xccdf-1.2:title>
 <xccdf-1.2:check system="http://oval.mitre.org/XMLSchema/oval-definitions-5">
 <xccdf-1.2:check-content-ref name="oval:ssg-rule_b:def:1"/>
 </xccdf-1.2:check></xccdf-1.2:Rule></xccdf-1.2:Benchmark></ds:component>
 <rule-result idref="rule_a"><result>STATUS</result></rule-result>
 <rule-result idref="rule_b"><result>STATUS</result></rule-result></root>`

func setupOpenSCAPRetention(t *testing.T) (*ProviderServer, string, *provider.ScanRequest) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "workspace space % é")
	require.NoError(t, os.Mkdir(root, 0o700))
	cfg := config.WorkspaceConfig{Root: root}
	require.NoError(t, cfg.EnsureDirectories())
	require.NoError(t, os.WriteFile(cfg.PolicyPath(), []byte("<Tailoring/>"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "datastream.xml"), []byte("<Benchmark/>"), 0o600))
	fixture := filepath.Join(t.TempDir(), "fixture.xml")
	require.NoError(t, os.WriteFile(fixture,
		[]byte(strings.ReplaceAll(retainedARFFixture, "STATUS", "pass")), 0o600))
	toolDir := t.TempDir()
	// A real subprocess exercises the oscap argument boundary and waits for outputs.
	script := `#!/bin/sh
set -eu
pwd > "$TEST_OSCAP_CWD"
printf '%s\n' "$@" > "$TEST_OSCAP_ARGS"
arf=''; results=''; output=''
while [ "$#" -gt 0 ]; do
 case "$1" in
 --results-arf) arf="$2"; shift;;
 --results) results="$2"; shift;;
 --output) output="$2"; shift;;
 esac
 shift
done
if [ -n "$output" ]; then
 printf 'remediation\n' > "$output"
 exit 0
fi
case "$TEST_OSCAP_MODE" in
 missing-arf) rm "$arf";;
 empty-arf) ;;
 invalid-arf) printf '<broken' > "$arf";;
 no-target) printf '<root/>' > "$arf";;
 no-benchmark) printf '<root><target>host</target></root>' > "$arf";;
 symlink) rm "$arf"; ln -s "$TEST_OSCAP_FIXTURE" "$arf";;
 directory) rm "$arf"; mkdir "$arf";;
 *) cat "$TEST_OSCAP_FIXTURE" > "$arf";;
esac
case "$TEST_OSCAP_MODE" in
 missing-results) rm "$results";;
 empty-results) ;;
 invalid-results) printf '<broken' > "$results";;
 *) printf '<Benchmark><TestResult/></Benchmark>\n' > "$results";;
esac
printf 'done' > "$TEST_OSCAP_DONE"
exit "$TEST_OSCAP_STATUS"
`
	toolRoot, err := os.OpenRoot(toolDir)
	require.NoError(t, err)
	require.NoError(t, toolRoot.WriteFile("oscap", []byte(script), 0o755))
	require.NoError(t, toolRoot.Close())
	t.Setenv("PATH", toolDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_OSCAP_FIXTURE", fixture)
	t.Setenv("TEST_OSCAP_CWD", filepath.Join(t.TempDir(), "cwd"))
	t.Setenv("TEST_OSCAP_ARGS", filepath.Join(t.TempDir(), "args"))
	t.Setenv("TEST_OSCAP_DONE", filepath.Join(t.TempDir(), "done"))
	t.Setenv("TEST_OSCAP_MODE", "valid")
	t.Setenv("TEST_OSCAP_STATUS", "0")
	s := New()
	s.logger = hclog.NewNullLogger()
	return s, root, &provider.ScanRequest{Targets: []provider.Target{{TargetID: "requested-host",
		Variables: map[string]string{"workspace": root, "profile": "stig", "datastream": "datastream.xml"}}}}
}

func TestProviderServer_Scan_ThreeRetainedEvaluations(t *testing.T) {
	s, root, req := setupOpenSCAPRetention(t)
	invocation, err := os.Getwd()
	require.NoError(t, err)
	legacyARF, legacyResults, legacyPolicy := config.ARFPath, config.ResultsPath, config.PolicyPath
	legacy := map[string][]byte{}
	for _, name := range []string{config.ARFPath, config.ResultsPath} {
		path := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		legacy[path] = []byte("legacy " + name)
		require.NoError(t, os.WriteFile(path, legacy[path], 0o600))
	}
	start := time.Date(2026, 11, 10, 23, 59, 59, 0, time.UTC)
	clockCalls := 0
	s.allocate = func(options evidence.StorageOptions, labels ...string) (*evidence.Evaluation, error) {
		now := start
		options.Clock = func() time.Time { clockCalls++; return now }
		options.Operations.Read = func(file *os.File) ([]byte, error) {
			// The completed tool is read in the next day according to the injected clock.
			now = start.Add(2 * time.Second)
			return io.ReadAll(file)
		}
		options.Operations.Chmod = func(file *os.File, mode os.FileMode) error {
			// Finalization cannot begin while the subprocess still writes.
			if mode == evidence.FileMode {
				assert.FileExists(t, os.Getenv("TEST_OSCAP_DONE"))
			}
			return file.Chmod(mode)
		}
		return evidence.Allocate(options, labels...)
	}
	retained := map[string][]byte{}
	var sources []*provider.EvidenceSource
	var urls []string
	ids := map[string]bool{}
	for scanIndex, status := range []string{"pass", "fail", "fail"} {
		data := []byte(strings.ReplaceAll(retainedARFFixture, "STATUS", status))
		fixtureRoot, err := os.OpenRoot(filepath.Dir(os.Getenv("TEST_OSCAP_FIXTURE")))
		require.NoError(t, err)
		require.NoError(t, fixtureRoot.WriteFile("fixture.xml", data, 0o600))
		require.NoError(t, fixtureRoot.Close())
		if status == "fail" {
			t.Setenv("TEST_OSCAP_STATUS", "2")
		}
		resp, err := s.Scan(context.Background(), req)
		require.NoError(t, err)
		require.Len(t, resp.Assessments, 2)
		require.Len(t, resp.MappingReferences, 1, "supporting outputs are not evidence")
		item := resp.Assessments[0].Evidence[0]
		assert.Equal(t, item, resp.Assessments[1].Evidence[0])
		assert.Equal(t, start.Format(time.RFC3339), item.CollectedAt)
		assert.Equal(t, evidence.TypeARF, item.Type)
		token := strings.TrimPrefix(item.ID, "arf_")
		assert.Regexp(t, "^[0-9a-f]{32}$", token)
		assert.False(t, ids[item.ID])
		ids[item.ID] = true
		assert.Equal(t, "openscap-arf_"+token, item.Source.ReferenceID)
		ref := resp.MappingReferences[0]
		assert.Equal(t, item.Source.ReferenceID, ref.ID)
		parsed, err := url.Parse(ref.URL)
		require.NoError(t, err)
		assert.Equal(t, "file", parsed.Scheme)
		path := filepath.Join(root, item.Source.Coordinate)
		assert.Equal(t, path, parsed.Path)
		assert.Contains(t, ref.URL, "space%20%25%20%C3%A9")
		assert.Contains(t, item.Source.Coordinate, filepath.Join("requested-host", "stig",
			"evaluation-20261110T235959Z-"+token))
		assert.Equal(t, fmt.Sprintf("sha256:%x", sha256.Sum256(data)), item.Source.Digest)
		assert.Equal(t, data, mustReadOpenSCAPFile(t, path))
		retained[path] = data
		assert.Contains(t, string(mustReadOpenSCAPFile(t, os.Getenv("TEST_OSCAP_ARGS"))),
			"--results-arf\n"+path+"\n--tailoring-file\n"+filepath.Join(root, config.PolicyPath))
		assert.Equal(t, root+"\n", string(mustReadOpenSCAPFile(t, os.Getenv("TEST_OSCAP_CWD"))))
		directory := filepath.Dir(path)
		metadata := map[string]any{}
		require.NoError(t, json.Unmarshal(mustReadOpenSCAPFile(t,
			filepath.Join(directory, evidence.MetadataFile)), &metadata))
		assert.Equal(t, token, metadata["evaluation_id"])
		assert.Equal(t, start.Format(time.RFC3339), metadata["collected_at"])
		target := metadata["target"].(map[string]any)
		assert.Equal(t, "requested-host", target["target_id"])
		assert.Equal(t, "host:requested-host", target["target_key"])
		assert.Equal(t, "observed-host", target["observed_target"])
		configuration := metadata["configuration"].(map[string]any)
		assert.Equal(t, "datastream.xml", configuration["datastream_path"])
		assert.Equal(t, config.PolicyPath, configuration["tailoring_path"])
		assert.Equal(t, "stig", configuration["profile"])
		for _, name := range []string{"", evidence.ARFFile, evidence.XCCDFFile, evidence.MetadataFile} {
			info, err := os.Stat(filepath.Join(directory, name))
			require.NoError(t, err)
			mode := os.FileMode(evidence.FileMode)
			if name == "" {
				mode = evidence.DirectoryMode
			}
			assert.Equal(t, mode, info.Mode().Perm())
			owner := info.Sys().(*syscall.Stat_t)
			assert.Equal(t, os.Geteuid(), int(owner.Uid))
			assert.Equal(t, os.Getegid(), int(owner.Gid))
		}
		if scanIndex > 0 {
			assert.Equal(t, provider.ResultFailed, resp.Assessments[0].Steps[0].Result)
		}
		sources = append(sources, item.Source)
		urls = append(urls, ref.URL)
	}
	assert.Equal(t, 3, clockCalls)
	for path, expected := range retained {
		assert.Equal(t, expected, mustReadOpenSCAPFile(t, path))
	}
	for path, expected := range legacy {
		assert.Equal(t, expected, mustReadOpenSCAPFile(t, path))
	}
	assert.Equal(t, legacyARF, config.ARFPath)
	assert.Equal(t, legacyResults, config.ResultsPath)
	assert.Equal(t, legacyPolicy, config.PolicyPath)
	current, err := os.Getwd()
	require.NoError(t, err)
	assert.Equal(t, invocation, current)
	copyRoot := t.TempDir()
	require.NoError(t, os.CopyFS(copyRoot, os.DirFS(root)))
	for index, source := range sources {
		copied := filepath.Join(copyRoot, source.Coordinate)
		assert.Equal(t, source.Digest,
			fmt.Sprintf("sha256:%x", sha256.Sum256(mustReadOpenSCAPFile(t, copied))))
		assert.Equal(t, evidence.FileURL(filepath.Join(root, source.Coordinate)), urls[index])
		assert.NotEqual(t, evidence.FileURL(copied), urls[index])
	}
}

func mustReadOpenSCAPFile(t *testing.T, path string) []byte {
	t.Helper()
	root, err := os.OpenRoot(filepath.Dir(path))
	require.NoError(t, err)
	data, err := root.ReadFile(filepath.Base(path))
	require.NoError(t, root.Close())
	require.NoError(t, err)
	return data
}

func TestProviderServer_Scan_RetentionFailures(t *testing.T) {
	for _, failure := range []string{"allocation", "reserve-close", "missing-arf", "empty-arf", "invalid-arf",
		"no-target", "no-benchmark", "symlink", "directory", "missing-results", "empty-results", "invalid-results", "tool",
		"digest", "identity", "ownership", "mode", "changed-output"} {
		t.Run(failure, func(t *testing.T) {
			s, root, req := setupOpenSCAPRetention(t)
			legacyPath := filepath.Join(root, config.ARFPath)
			require.NoError(t, os.MkdirAll(filepath.Dir(legacyPath), 0o755))
			legacy := []byte(strings.ReplaceAll(retainedARFFixture, "STATUS", "pass"))
			require.NoError(t, os.WriteFile(legacyPath, legacy, 0o600))
			t.Setenv("TEST_OSCAP_MODE", failure)
			if failure == "tool" {
				t.Setenv("TEST_OSCAP_STATUS", "1")
			}
			if failure == "identity" {
				s.identity = func() (evidence.Identity, error) {
					return evidence.Identity{}, errors.New("injected identity failure")
				}
			}
			s.allocate = func(options evidence.StorageOptions, labels ...string) (*evidence.Evaluation, error) {
				switch failure {
				case "allocation":
					return nil, errors.New("injected allocation failure")
				case "reserve-close":
					options.Operations.Close = func(file *os.File) error {
						return errors.Join(file.Close(), errors.New("injected close failure"))
					}
				case "digest":
					options.Operations.Read = func(*os.File) ([]byte, error) {
						return nil, errors.New("injected digest failure")
					}
				case "ownership":
					options.Operations.Stat = func(file *os.File) (os.FileInfo, error) {
						info, err := file.Stat()
						if err == nil && info.Mode().IsRegular() {
							owner := *info.Sys().(*syscall.Stat_t)
							owner.Uid++
							return openSCAPFileInfo{FileInfo: info, owner: owner}, nil
						}
						return info, err
					}
				case "mode":
					options.Operations.Chmod = func(file *os.File, mode os.FileMode) error {
						if mode == evidence.FileMode {
							return errors.New("injected mode failure")
						}
						return file.Chmod(mode)
					}
				case "changed-output":
					reads := 0
					options.Operations.Read = func(file *os.File) ([]byte, error) {
						data, err := io.ReadAll(file)
						reads++
						if reads > 2 {
							data = append(data, '\n')
						}
						return data, err
					}
				}
				return evidence.Allocate(options, labels...)
			}
			resp, err := s.Scan(context.Background(), req)
			require.Error(t, err)
			assert.Nil(t, resp, "OpenSCAP errors must never publish a response")
			assert.Equal(t, legacy, mustReadOpenSCAPFile(t, legacyPath))
			assert.Equal(t, legacy, mustReadOpenSCAPFile(t, os.Getenv("TEST_OSCAP_FIXTURE")))
		})
	}
}

type openSCAPFileInfo struct {
	os.FileInfo
	owner syscall.Stat_t
}

func (info openSCAPFileInfo) Sys() any { return &info.owner }

func TestProviderServer_Scan_MetadataFailuresWarn(t *testing.T) {
	for _, failure := range []string{"write", "close"} {
		t.Run(failure, func(t *testing.T) {
			s, _, req := setupOpenSCAPRetention(t)
			var warnings bytes.Buffer
			s.logger = hclog.New(&hclog.LoggerOptions{Output: &warnings, Level: hclog.Warn})
			var directory, token string
			s.allocate = func(options evidence.StorageOptions, labels ...string) (*evidence.Evaluation, error) {
				options.Operations.Write = func(file *os.File, data []byte) (int, error) {
					if failure == "write" && filepath.Base(file.Name()) == evidence.MetadataFile {
						_, err := file.Write(data[:1])
						return 1, errors.Join(err, errors.New("metadata write"))
					}
					return file.Write(data)
				}
				options.Operations.Close = func(file *os.File) error {
					err := file.Close()
					if failure == "close" && filepath.Base(file.Name()) == evidence.MetadataFile {
						return errors.Join(err, errors.New("metadata close"))
					}
					return err
				}
				evaluation, err := evidence.Allocate(options, labels...)
				if evaluation != nil {
					directory, token = evaluation.Directory, evaluation.Token
				}
				return evaluation, err
			}
			resp, err := s.Scan(context.Background(), req)
			require.NoError(t, err)
			require.Len(t, resp.Assessments, 2)
			require.Len(t, resp.MappingReferences, 1)
			assert.Contains(t, warnings.String(), token)
			assert.Contains(t, warnings.String(), "cannot save supporting evaluation metadata")
			info, err := os.Stat(filepath.Join(directory, evidence.MetadataFile))
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(evidence.FileMode), info.Mode().Perm())
		})
	}
}

func TestProviderServer_WorkspaceRPCIsolated(t *testing.T) {
	if root := os.Getenv("TEST_OPENSCAP_CHILD_ROOT"); root != "" {
		invocation, err := os.Getwd()
		require.NoError(t, err)
		s := New()
		workspaceValue := os.Getenv("TEST_OPENSCAP_WORKSPACE_VALUE")
		variables := map[string]string{"workspace": workspaceValue, "profile": "stig",
			"datastream": os.Getenv("TEST_OPENSCAP_DATASTREAM_REF"), "owner_uid": "123", "owner_gid": "456"}
		generated, err := s.Generate(context.Background(), &provider.GenerateRequest{
			GlobalVariables: variables,
			Configuration:   []provider.AssessmentConfiguration{{RequirementID: "rule_a", PlanID: "plan-a"}},
		})
		require.NoError(t, err)
		if expected := os.Getenv("TEST_OPENSCAP_DISCOVERY_ERROR"); expected != "" {
			require.False(t, generated.Success)
			require.Contains(t, generated.ErrorMessage, expected)
			response, err := s.Scan(context.Background(), &provider.ScanRequest{
				Targets: []provider.Target{{TargetID: "requested-host", Variables: variables}},
			})
			require.ErrorContains(t, err, expected)
			require.Nil(t, response)
			assert.NoFileExists(t, os.Getenv("TEST_OSCAP_ARGS"))
			assert.NoDirExists(t, filepath.Join(invocation, provider.WorkspaceDir))
			return
		}
		require.True(t, generated.Success, generated.ErrorMessage)
		policy := string(mustReadOpenSCAPFile(t, filepath.Join(root, config.PolicyPath)))
		assert.Contains(t, policy, os.Getenv("TEST_OPENSCAP_DATASTREAM_PATH"))
		assert.Contains(t, policy, "Workspace profile")
		resp, err := s.Scan(context.Background(), &provider.ScanRequest{Targets: []provider.Target{{
			TargetID: "requested-host", Variables: variables}}})
		require.NoError(t, err)
		require.Len(t, resp.Assessments, 2)
		assert.Equal(t, "plan-a", resp.Assessments[0].RequirementID)
		assert.FileExists(t, filepath.Join(root, resp.Assessments[0].Evidence[0].Source.Coordinate))
		assert.Equal(t, evidence.FileURL(filepath.Join(root,
			resp.Assessments[0].Evidence[0].Source.Coordinate)), resp.MappingReferences[0].URL)
		assert.NoDirExists(t, filepath.Join(invocation, provider.WorkspaceDir))
		current, err := os.Getwd()
		require.NoError(t, err)
		assert.Equal(t, invocation, current)
		assert.Equal(t, root+"\n", string(mustReadOpenSCAPFile(t, os.Getenv("TEST_OSCAP_CWD"))))
		assert.Contains(t, string(mustReadOpenSCAPFile(t, os.Getenv("TEST_OSCAP_ARGS"))),
			os.Getenv("TEST_OPENSCAP_DATASTREAM_PATH")+"\n")
		for _, name := range []string{"remediation-script.sh", "remediation-playbook.yml", "remediation-blueprint.toml"} {
			assert.Equal(t, "remediation\n", string(mustReadOpenSCAPFile(t,
				filepath.Join(root, provider.WorkspaceDir, config.ProviderDir, config.RemediationDir, name))))
		}
		return
	}
	for _, form := range []string{"absolute", "relative", "symlink", "discovery-workspace-only",
		"discovery-conflict", "discovery-missing-workspace", "discovery-missing-system-info",
		"discovery-absolute-external"} {
		t.Run(form, func(t *testing.T) {
			_, root, _ := setupOpenSCAPRetention(t)
			invocation := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(invocation, "datastream.xml"), []byte("invalid"), 0o600))
			datastream := strings.ReplaceAll(retainedARFFixture, "STATUS", "pass")
			datastream = strings.ReplaceAll(datastream, "<xccdf-1.2:Rule ", `<xccdf-1.2:Rule selected="true" `)
			datastream = strings.ReplaceAll(datastream, "</xccdf-1.2:title>",
				"</xccdf-1.2:title><xccdf-1.2:description>Rule description</xccdf-1.2:description>")
			datastream = strings.ReplaceAll(datastream, `id="rule_a"`, `id="xccdf_org.ssgproject.content_rule_rule_a"`)
			datastream = strings.ReplaceAll(datastream, "<xccdf-1.2:Benchmark>", `<xccdf-1.2:Benchmark>
 <xccdf-1.2:Profile id="xccdf_org.ssgproject.content_profile_stig">
 <xccdf-1.2:title>Workspace profile</xccdf-1.2:title>
 </xccdf-1.2:Profile>`)
			require.NoError(t, os.WriteFile(filepath.Join(root, "datastream.xml"), []byte(datastream), 0o600))
			t.Setenv("TEST_OPENSCAP_DATASTREAM_REF", "datastream.xml")
			t.Setenv("TEST_OPENSCAP_DATASTREAM_PATH", filepath.Join(root, "datastream.xml"))
			t.Setenv("TEST_OPENSCAP_DISCOVERY_ERROR", "")
			if strings.HasPrefix(form, "discovery-") {
				const workspaceCPE = "cpe:/o:test:workspace:1"
				const invocationCPE = "cpe:/o:test:invocation:1"
				t.Setenv("TEST_OPENSCAP_DATASTREAM_REF", "")
				t.Setenv(config.DatastreamsDirEnvVar, "content")
				t.Setenv(config.SystemInfoFileEnvVar, "os-release")
				content := filepath.Join(root, "content")
				if form == "discovery-absolute-external" {
					content = t.TempDir()
					t.Setenv(config.DatastreamsDirEnvVar, content)
					t.Setenv(config.SystemInfoFileEnvVar, filepath.Join(root, "os-release"))
				}
				path := filepath.Join(content, "ssg-workspace-ds.xml")
				t.Setenv("TEST_OPENSCAP_DATASTREAM_PATH", path)
				datastream = strings.Replace(datastream, "<target>",
					`<cpe-item name="`+workspaceCPE+`"/><target>`, 1)
				if form != "discovery-missing-workspace" {
					require.NoError(t, os.MkdirAll(content, 0o700))
					require.NoError(t, os.WriteFile(path, []byte(datastream), 0o600))
				} else {
					t.Setenv("TEST_OPENSCAP_DISCOVERY_ERROR", content)
				}
				if form != "discovery-missing-system-info" {
					require.NoError(t, os.WriteFile(filepath.Join(root, "os-release"),
						[]byte(`CPE_NAME="`+workspaceCPE+`"`), 0o600))
				} else {
					t.Setenv("TEST_OPENSCAP_DISCOVERY_ERROR", filepath.Join(root, "os-release"))
				}
				if form != "discovery-workspace-only" {
					require.NoError(t, os.Mkdir(filepath.Join(invocation, "content"), 0o700))
					require.NoError(t, os.WriteFile(filepath.Join(invocation, "content", "ssg-invocation-ds.xml"),
						[]byte(strings.ReplaceAll(datastream, workspaceCPE, invocationCPE)), 0o600))
					require.NoError(t, os.WriteFile(filepath.Join(invocation, "os-release"),
						[]byte(`CPE_NAME="`+invocationCPE+`"`), 0o600))
				}
			}
			value := root
			switch form {
			case "relative":
				var err error
				value, err = filepath.Rel(invocation, root)
				require.NoError(t, err)
			case "symlink":
				value = filepath.Join(t.TempDir(), "linked-root")
				require.NoError(t, os.Symlink(root, value))
			}
			t.Setenv("TEST_OPENSCAP_CHILD_ROOT", root)
			t.Setenv("TEST_OPENSCAP_WORKSPACE_VALUE", value)
			t.Setenv("COMPLYTIME_WORKSPACE", invocation)
			t.Setenv("SUDO_UID", "123")
			t.Setenv("SUDO_GID", "456")
			t.Setenv("USER", "not-the-execution-account")
			binary, err := os.Executable()
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, binary, "-test.run=^TestProviderServer_WorkspaceRPCIsolated$")
			command.Dir = invocation
			output, err := command.CombinedOutput()
			require.NoError(t, err, "%s", output)
			assert.NoDirExists(t, filepath.Join(invocation, provider.WorkspaceDir))
		})
	}
}

func TestProviderServer_WorkspaceFailuresBeforeStorage(t *testing.T) {
	for _, scenario := range []string{"scan-conflict", "generate-conflict", "scan-invalid", "generate-invalid"} {
		t.Run(scenario, func(t *testing.T) {
			s := New()
			s.identity = func() (evidence.Identity, error) {
				t.Fatal("workspace validation must precede identity lookup/storage")
				return evidence.Identity{}, nil
			}
			root, other := t.TempDir(), t.TempDir()
			if strings.HasSuffix(scenario, "invalid") {
				root = filepath.Join(root, "missing")
			}
			if strings.HasPrefix(scenario, "scan") {
				req := &provider.ScanRequest{Targets: []provider.Target{{Variables: map[string]string{"workspace": root}}}}
				if scenario == "scan-conflict" {
					req.Targets = append(req.Targets, provider.Target{Variables: map[string]string{"workspace": other}})
				}
				resp, err := s.Scan(context.Background(), req)
				require.Error(t, err)
				assert.Nil(t, resp)
			} else {
				resp, err := s.Generate(context.Background(), &provider.GenerateRequest{
					GlobalVariables: map[string]string{"workspace": root},
					TargetVariables: map[string]string{"workspace": other},
				})
				require.Error(t, err)
				assert.Nil(t, resp)
			}
			assert.NoDirExists(t, filepath.Join(other, provider.WorkspaceDir))
		})
	}
}

func TestParseCheck(t *testing.T) {
	tests := []struct {
		name           string
		xmlContent     string
		expectedResult string
		expectedError  error
	}{
		{
			name:           "Valid/ExpectedFormat",
			xmlContent:     `<check-content-ref name="oval:ssg-audit_perm_change_success:def:1"/>`,
			expectedResult: "audit_perm_change_success",
		},
		{
			name:           "Invalid/UnexpectedFormat",
			xmlContent:     `<check-content-ref name="ovalssg-audit_perm_change_success:def:1"/>`,
			expectedResult: "",
			expectedError:  errors.New("check id \"ovalssg-audit_perm_change_success:def:1\" is in unexpected format"),
		},
		{
			name:           "Invalid/NoNameAttribute",
			xmlContent:     `<check-content-ref/>`,
			expectedResult: "",
			expectedError:  errors.New("check-content-ref node has no 'name' attribute"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := xmlquery.Parse(strings.NewReader(tt.xmlContent))
			assert.NoError(t, err)
			check, err := xccdf.ParseCheck(node.SelectElement("check-content-ref"))
			assert.Equal(t, tt.expectedResult, check)
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestProviderServer_Describe(t *testing.T) {
	s := New()
	resp, err := s.Describe(context.Background(), &provider.DescribeRequest{})
	require.NoError(t, err)
	assert.True(t, resp.Healthy)
	assert.Equal(t, "0.0.0-unknown", resp.Version)
	assert.Contains(t, resp.RequiredTargetVariables, "profile")
}

func TestProviderServer_Generate_NoConfig(t *testing.T) {
	s := New()
	resp, err := s.Generate(context.Background(), &provider.GenerateRequest{})
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.ErrorMessage, "no assessment configurations")
}

func TestProviderServer_Scan_NoTargets(t *testing.T) {
	s := New()
	_, err := s.Scan(context.Background(), &provider.ScanRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no targets")
}

func TestParseARFFile_Missing(t *testing.T) {
	_, err := xccdf.ParseARFFile("/nonexistent/arf.xml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open ARF")
}

func TestParseARFFile_InvalidXML(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "arf.xml")
	require.NoError(t, os.WriteFile(tmp, []byte("not xml <<<<"), 0600))
	_, err := xccdf.ParseARFFile(tmp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse ARF")
}

func TestParseARFFile_Valid(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "arf.xml")
	require.NoError(t, os.WriteFile(tmp, []byte("<root><target>host</target></root>"), 0600))
	node, err := xccdf.ParseARFFile(tmp)
	require.NoError(t, err)
	assert.NotNil(t, node)
}

func TestBuildAssessmentsFromARF_NoTarget(t *testing.T) {
	xml := `<root><ds:component xmlns:ds="http://scap.nist.gov/schema/scap/source/1.2">
		<xccdf-1.2:Benchmark xmlns:xccdf-1.2="http://checklists.nist.gov/xccdf/1.2"></xccdf-1.2:Benchmark>
		</ds:component></root>`
	node, err := xmlquery.Parse(strings.NewReader(xml))
	require.NoError(t, err)
	_, err = buildAssessmentsFromARF(node, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no 'target' attribute")
}

func TestBuildAssessmentsFromARF_NoResults(t *testing.T) {
	xml := `<root>
		<target>host1</target>
		<ds:component xmlns:ds="http://scap.nist.gov/schema/scap/source/1.2">
		<xccdf-1.2:Benchmark xmlns:xccdf-1.2="http://checklists.nist.gov/xccdf/1.2"></xccdf-1.2:Benchmark>
		</ds:component></root>`
	node, err := xmlquery.Parse(strings.NewReader(xml))
	require.NoError(t, err)
	assessments, err := buildAssessmentsFromARF(node, nil)
	require.NoError(t, err)
	assert.Empty(t, assessments)
}

func TestBuildAssessmentsFromARF_NotApplicable(t *testing.T) {
	// A rule-result with "notapplicable" must produce a ResultSkipped
	// assessment, not be silently dropped.
	xml := `<root xmlns:ds="http://scap.nist.gov/schema/scap/source/1.2"
	              xmlns:xccdf-1.2="http://checklists.nist.gov/xccdf/1.2">
		<target>host1</target>
		<ds:component>
			<xccdf-1.2:Benchmark>
				<xccdf-1.2:Rule id="xccdf_org.ssgproject.content_rule_test_na">
					<xccdf-1.2:title>Test NA Rule</xccdf-1.2:title>
					<xccdf-1.2:check system="http://oval.mitre.org/XMLSchema/oval-definitions-5">
						<xccdf-1.2:check-content-ref name="oval:ssg-test_na:def:1"/>
					</xccdf-1.2:check>
				</xccdf-1.2:Rule>
			</xccdf-1.2:Benchmark>
		</ds:component>
		<rule-result idref="xccdf_org.ssgproject.content_rule_test_na">
			<result>notapplicable</result>
		</rule-result>
	</root>`
	node, err := xmlquery.Parse(strings.NewReader(xml))
	require.NoError(t, err)
	assessments, err := buildAssessmentsFromARF(node, nil)
	require.NoError(t, err)
	require.Len(t, assessments, 1, "notapplicable must produce an assessment, not be dropped")
	assert.Equal(t, "test_na", assessments[0].RequirementID)
	require.Len(t, assessments[0].Steps, 1)
	assert.Equal(t, provider.ResultSkipped, assessments[0].Steps[0].Result)
}

func TestBuildAssessmentsFromARF_NotSelected(t *testing.T) {
	// A rule-result with "notselected" must still be silently dropped.
	xml := `<root xmlns:ds="http://scap.nist.gov/schema/scap/source/1.2"
	              xmlns:xccdf-1.2="http://checklists.nist.gov/xccdf/1.2">
		<target>host1</target>
		<ds:component>
			<xccdf-1.2:Benchmark>
				<xccdf-1.2:Rule id="xccdf_org.ssgproject.content_rule_test_ns">
					<xccdf-1.2:title>Test NS Rule</xccdf-1.2:title>
					<xccdf-1.2:check system="http://oval.mitre.org/XMLSchema/oval-definitions-5">
						<xccdf-1.2:check-content-ref name="oval:ssg-test_ns:def:1"/>
					</xccdf-1.2:check>
				</xccdf-1.2:Rule>
			</xccdf-1.2:Benchmark>
		</ds:component>
		<rule-result idref="xccdf_org.ssgproject.content_rule_test_ns">
			<result>notselected</result>
		</rule-result>
	</root>`
	node, err := xmlquery.Parse(strings.NewReader(xml))
	require.NoError(t, err)
	assessments, err := buildAssessmentsFromARF(node, nil)
	require.NoError(t, err)
	assert.Empty(t, assessments, "notselected must still be skipped")
}

func TestFindOVALCheckContentRef_NoChecks(t *testing.T) {
	node, err := xmlquery.Parse(strings.NewReader("<rule></rule>"))
	require.NoError(t, err)
	ref := xccdf.FindOVALCheckContentRef(node.SelectElement("rule"))
	assert.Nil(t, ref)
}

func TestMergeVariables(t *testing.T) {
	global := map[string]string{"a": "1", "b": "2"}
	target := map[string]string{"b": "override", "c": "3"}
	merged := mergeVariables(global, target)
	assert.Equal(t, "1", merged["a"])
	assert.Equal(t, "override", merged["b"])
	assert.Equal(t, "3", merged["c"])
}

func TestBuildAssessmentLog_UsesMatchIDMapping(t *testing.T) {
	// Rule XML with an OVAL check-content-ref that ParseCheck extracts
	// short name "sshd_disable_root_login" from. Uses the xccdf-1.2
	// namespace expected by FindOVALCheckContentRef.
	ruleXML := `<rule xmlns:xccdf-1.2="http://checklists.nist.gov/xccdf/1.2">
		<xccdf-1.2:title>Disable Root Login</xccdf-1.2:title>
		<xccdf-1.2:check system="http://oval.mitre.org/XMLSchema/oval-definitions-5">
			<xccdf-1.2:check-content-ref name="oval:ssg-sshd_disable_root_login:def:1"/>
		</xccdf-1.2:check>
	</rule>`
	resultXML := `<rule-result><result>pass</result><message>ok</message></rule-result>`

	ruleNode, err := xmlquery.Parse(strings.NewReader(ruleXML))
	require.NoError(t, err)
	resultNode, err := xmlquery.Parse(strings.NewReader(resultXML))
	require.NoError(t, err)

	rule := ruleNode.SelectElement("rule")
	result := resultNode.SelectElement("rule-result")
	ruleIDRef := "xccdf_org.ssgproject.content_rule_sshd_disable_root_login"

	t.Run("WithMapping", func(t *testing.T) {
		// Map XCCDF short name → original match ID (e.g. a plan ID)
		ruleToMatchID := map[string]string{
			"sshd_disable_root_login": "plan-123-ssh-root",
		}
		assessment, skip, err := buildAssessmentLog(rule, result, ruleIDRef, "pass", "host1", ruleToMatchID)
		require.NoError(t, err)
		assert.False(t, skip)
		assert.Equal(t, "plan-123-ssh-root", assessment.RequirementID,
			"should use the mapped match ID, not the XCCDF short name")
	})

	t.Run("WithoutMapping", func(t *testing.T) {
		// No mapping — falls back to XCCDF short name
		assessment, skip, err := buildAssessmentLog(rule, result, ruleIDRef, "pass", "host1", nil)
		require.NoError(t, err)
		assert.False(t, skip)
		assert.Equal(t, "sshd_disable_root_login", assessment.RequirementID,
			"should fall back to XCCDF short name when no mapping exists")
	})

	t.Run("MappingMiss", func(t *testing.T) {
		// Mapping exists but doesn't contain this rule
		ruleToMatchID := map[string]string{
			"some_other_rule": "plan-456",
		}
		assessment, skip, err := buildAssessmentLog(rule, result, ruleIDRef, "pass", "host1", ruleToMatchID)
		require.NoError(t, err)
		assert.False(t, skip)
		assert.Equal(t, "sshd_disable_root_login", assessment.RequirementID,
			"should fall back to XCCDF short name when rule not in mapping")
	})
}

func TestBuildRuleToMatchIDMap(t *testing.T) {
	configs := []provider.AssessmentConfiguration{
		{PlanID: "plan-123", RequirementID: "sshd_disable_root_login"},
		{PlanID: "", RequirementID: "audit_perm_change_success"},
		{PlanID: "plan-456", RequirementID: "enable_fips_mode"},
	}

	m := buildRuleToMatchIDMap(configs)

	// PlanID present → MatchID() returns PlanID
	assert.Equal(t, "plan-123", m["sshd_disable_root_login"])
	// PlanID empty → MatchID() returns RequirementID (identity mapping)
	assert.Equal(t, "audit_perm_change_success", m["audit_perm_change_success"])
	// PlanID present → MatchID() returns PlanID
	assert.Equal(t, "plan-456", m["enable_fips_mode"])
}

func TestBuildRuleToMatchIDMap_Empty(t *testing.T) {
	m := buildRuleToMatchIDMap(nil)
	assert.NotNil(t, m)
	assert.Empty(t, m)
}

func TestAttachARFEvidence(t *testing.T) {
	// Build assessments from a known ARF XML fragment with two
	// passing rules to verify evidence is attached to every entry.
	arfXML := `<root xmlns:ds="http://scap.nist.gov/schema/scap/source/1.2"
	                  xmlns:xccdf-1.2="http://checklists.nist.gov/xccdf/1.2">
		<target>host1</target>
		<ds:component>
			<xccdf-1.2:Benchmark>
				<xccdf-1.2:Rule id="xccdf_org.ssgproject.content_rule_rule_a">
					<xccdf-1.2:title>Rule A</xccdf-1.2:title>
					<xccdf-1.2:check system="http://oval.mitre.org/XMLSchema/oval-definitions-5">
						<xccdf-1.2:check-content-ref name="oval:ssg-rule_a:def:1"/>
					</xccdf-1.2:check>
				</xccdf-1.2:Rule>
				<xccdf-1.2:Rule id="xccdf_org.ssgproject.content_rule_rule_b">
					<xccdf-1.2:title>Rule B</xccdf-1.2:title>
					<xccdf-1.2:check system="http://oval.mitre.org/XMLSchema/oval-definitions-5">
						<xccdf-1.2:check-content-ref name="oval:ssg-rule_b:def:1"/>
					</xccdf-1.2:check>
				</xccdf-1.2:Rule>
			</xccdf-1.2:Benchmark>
		</ds:component>
		<rule-result idref="xccdf_org.ssgproject.content_rule_rule_a">
			<result>pass</result>
		</rule-result>
		<rule-result idref="xccdf_org.ssgproject.content_rule_rule_b">
			<result>fail</result>
		</rule-result>
	</root>`

	node, err := xmlquery.Parse(strings.NewReader(arfXML))
	require.NoError(t, err)
	assessments, err := buildAssessmentsFromARF(node, nil)
	require.NoError(t, err)
	require.Len(t, assessments, 2, "expected two assessments")

	collectedAt := "2026-10-09T12:00:00Z"
	digest := "sha256:abc123def456"
	remarks := "Collected on host \"testhost\" (machine-id: abc123)"
	item := provider.Evidence{ID: "arf_token", Type: evidence.TypeARF, CollectedAt: collectedAt,
		Source: &provider.EvidenceSource{ReferenceID: "openscap-arf_token",
			Coordinate: ".complytime/openscap/results/host/profile/evaluation-time-token/arf.xml",
			Digest:     digest, Remarks: remarks}}
	attachARFEvidence(assessments, item)

	for i, a := range assessments {
		require.Len(t, a.Evidence, 1,
			"assessment %d must have exactly one evidence entry", i)
		ev := a.Evidence[0]
		assert.Equal(t, item.ID, ev.ID)
		assert.Equal(t, evidence.TypeARF, ev.Type)
		assert.Equal(t, collectedAt, ev.CollectedAt)
		require.NotNil(t, ev.Source,
			"assessment %d evidence must have a Source", i)
		assert.Equal(t, item.Source.ReferenceID,
			ev.Source.ReferenceID)
		assert.Equal(t, item.Source.Coordinate, ev.Source.Coordinate)
		assert.Equal(t, digest, ev.Source.Digest)
		assert.Equal(t, remarks, ev.Source.Remarks)
	}
}

func TestAttachARFEvidence_Empty(t *testing.T) {
	// Attaching evidence to an empty slice must not panic.
	var assessments []provider.AssessmentLog
	attachARFEvidence(assessments, provider.Evidence{})
	assert.Empty(t, assessments)
}

func TestRetainedARF_MappingReference(t *testing.T) {
	root := t.TempDir()
	identity, err := evidence.ExecutionIdentity()
	require.NoError(t, err)
	evaluation, err := evidence.Allocate(evidence.StorageOptions{WorkspaceRoot: root,
		ResultsRoot: filepath.Join(root, ".complytime", "openscap", "results"), Identity: identity}, "host", "profile")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, evaluation.Close()) })
	require.NoError(t, evaluation.Write(evidence.ARFFile, []byte("<root/>")))
	_, err = evaluation.Read(evidence.ARFFile)
	require.NoError(t, err)
	item, ref, err := evaluation.Reference(evidence.ARFFile, "arf_", "openscap-arf_",
		evidence.TypeARF, "OpenSCAP ARF scan result", "ARF result", "host")
	require.NoError(t, err)
	assert.Equal(t, "openscap-arf_"+evaluation.Token, ref.ID)
	assert.Equal(t, ref.ID, item.Source.ReferenceID)
	assert.Equal(t, "OpenSCAP ARF scan result", ref.Title)
	assert.NotEmpty(t, ref.Description)
	assert.True(t, strings.HasPrefix(ref.URL, "file:///"),
		"URL must be a proper file:// URL with absolute path: %s", ref.URL)
	assert.Equal(t, evidence.FileURL(evaluation.Path(evidence.ARFFile)), ref.URL)
}

func TestRuleResultMessage(t *testing.T) {
	tests := []struct {
		name       string
		ruleXML    string
		resultXML  string
		resultText string
		contains   string
	}{
		{
			name:       "TitleAndMessage",
			ruleXML:    `<rule xmlns:xccdf-1.2="http://checklists.nist.gov/xccdf/1.2"><xccdf-1.2:title>My Rule</xccdf-1.2:title></rule>`,
			resultXML:  `<rule-result><message>check failed</message></rule-result>`,
			resultText: "fail",
			contains:   "My Rule",
		},
		{
			name:       "NoTitleNoMessage",
			ruleXML:    `<rule></rule>`,
			resultXML:  `<rule-result></rule-result>`,
			resultText: "pass",
			contains:   "openscap rule-result is pass",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ruleNode, err := xmlquery.Parse(strings.NewReader(tt.ruleXML))
			require.NoError(t, err)
			resultNode, err := xmlquery.Parse(strings.NewReader(tt.resultXML))
			require.NoError(t, err)
			msg := xccdf.RuleResultMessage(ruleNode.SelectElement("rule"), resultNode.SelectElement("rule-result"), tt.resultText)
			assert.Contains(t, msg, tt.contains)
		})
	}
}
