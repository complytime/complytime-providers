// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeInput(t *testing.T) {
	tests := []struct {
		input       string
		expected    string
		expectError bool
	}{
		{"valid-input", "valid-input", false},
		{"another_valid.input", "another_valid.input", false},
		{"CAPS_and_numbers123", "CAPS_and_numbers123", false},
		{"mixed-123.UP_case", "mixed-123.UP_case", false},
		{"invalid/input", "", true},
		{"input with spaces", "", true},
		{"invalid@input", "", true},
		{"<invalid>", "", true},
		{";ls", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := SanitizeInput(tt.input)
			if (err != nil) != tt.expectError {
				t.Errorf("Expected error: %v, got: %v", tt.expectError, err)
			}
			if result != tt.expected {
				t.Errorf("Expected result: %s, got: %s", tt.expected, result)
			}
		})
	}
}

func TestSanitizePath(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err, "test requires $HOME to be set")

	tests := []struct {
		input       string
		expected    string
		expectError bool
	}{
		{"/foo/bar/../baz", "/foo/baz", false},
		{"./foo/bar", "foo/bar", false},
		{"foo/./bar", "foo/bar", false},
		{"foo/bar/..", "foo", false},
		{"/foo//bar", "/foo/bar", false},
		{"foo//bar//baz", "foo/bar/baz", false},
		{"foo/bar/../../baz", "baz", false},
		{"./../foo", "../foo", false},
		{"~/foo/bar", filepath.Join(homeDir, "foo", "bar"), false},
		{"~", homeDir, false},
		{"~weird", "~weird", false},
		{"", ".", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := SanitizePath(tt.input)
			if (err != nil) != tt.expectError {
				t.Errorf("Expected error: %v, got: %v", tt.expectError, err)
			}
			if result != tt.expected {
				t.Errorf("Expected result: %s, got: %s", tt.expected, result)
			}
		})
	}
}

func TestExpandPath_UsesHOMEEnvVar(t *testing.T) {
	t.Setenv("HOME", "/tmp/fakehome")

	result, err := expandPath("~/foo")
	require.NoError(t, err)
	require.Equal(t, "/tmp/fakehome/foo", result)
}

func TestExpandPath_BareTilde(t *testing.T) {
	t.Setenv("HOME", "/tmp/fakehome")

	result, err := expandPath("~")
	require.NoError(t, err)
	require.Equal(t, "/tmp/fakehome", result)
}

func TestExpandPath_NoTildePrefix(t *testing.T) {
	result, err := expandPath("/absolute/path")
	require.NoError(t, err)
	require.Equal(t, "/absolute/path", result)
}

func TestExpandPath_TildePrefixNotPath(t *testing.T) {
	result, err := expandPath("~weird")
	require.NoError(t, err)
	require.Equal(t, "~weird", result)
}

func TestExpandPath_ErrorWhenHomeUnavailable(t *testing.T) {
	t.Setenv("HOME", "")

	_, err := expandPath("~/foo")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to determine home directory")
}

func setupTestFiles() error {
	if err := os.MkdirAll("testdata", os.ModePerm); err != nil {
		return err
	}

	if err := os.WriteFile("testdata/valid.xml", []byte(`<root></root>`), 0600); err != nil {
		return err
	}
	if err := os.WriteFile("testdata/invalid.xml", []byte(`<root>`), 0600); err != nil {
		return err
	}
	return nil
}

func teardownTestFiles() {
	os.RemoveAll("testdata")
}

func TestIsXMLFile(t *testing.T) {
	if err := setupTestFiles(); err != nil {
		t.Fatalf("Failed to setup test files: %v", err)
	}
	defer teardownTestFiles()

	tests := []struct {
		name      string
		filePath  string
		want      bool
		expectErr bool
	}{
		{"Valid XML file", "testdata/valid.xml", true, false},
		{"Invalid XML file", "testdata/invalid.xml", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isXML, err := IsXMLFile(tt.filePath)
			if (err != nil) != tt.expectErr {
				t.Errorf("IsXMLFile(%s) error = %v, expectErr %v", tt.filePath, err, tt.expectErr)
				return
			}
			if isXML != tt.want {
				t.Errorf("IsXMLFile() = %v, want %v", isXML, tt.want)
			}
		})
	}
}

func TestEnsureDirectory(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		path        string
		expectError bool
	}{
		{filepath.Join(tempDir, "absent_dir"), false},
		{filepath.Join(tempDir, "existing_dir"), false},
		{tempDir + "/invalid\000dir", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if tt.path == filepath.Join(tempDir, "existing_dir") {
				if err := os.MkdirAll(tt.path, 0750); err != nil {
					t.Fatalf("Failed to create directory: %v", err)
				}
			}

			err := ensureDirectory(tt.path)
			if (err != nil) != tt.expectError {
				t.Errorf("Expected error: %v, got: %v", tt.expectError, err)
			}

			if !tt.expectError {
				if _, err := os.Stat(tt.path); os.IsNotExist(err) {
					t.Errorf("Expected directory to be created: %s", tt.path)
				}
			}
		})
	}
}

func TestEnsureDirectories(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() { _ = os.Chdir(originalDir) })

	require.NoError(t, EnsureDirectories())

	expectedDirs := []string{
		filepath.Join(".complytime", ProviderDir),
		filepath.Join(".complytime", ProviderDir, PolicyDir),
		filepath.Join(".complytime", ProviderDir, ResultsDir),
		filepath.Join(".complytime", ProviderDir, RemediationDir),
	}
	for _, dir := range expectedDirs {
		_, statErr := os.Stat(dir)
		require.NoError(t, statErr, "Expected directory to be created: %s", dir)
	}
}

func TestResolveDatastream(t *testing.T) {
	tempDir := t.TempDir()
	validDS := filepath.Join(tempDir, "ds.xml")
	require.NoError(t, os.WriteFile(validDS, []byte(`<root></root>`), 0600))

	t.Run("explicit valid path", func(t *testing.T) {
		result, err := ResolveDatastream(validDS)
		require.NoError(t, err)
		require.Equal(t, validDS, result)
	})

	t.Run("nonexistent path", func(t *testing.T) {
		_, err := ResolveDatastream(filepath.Join(tempDir, "missing.xml"))
		require.Error(t, err)
	})
}

func TestNormalizeCPE_AlreadyCPE22(t *testing.T) {
	cpe := normalizeCPE("cpe:/o:redhat:enterprise_linux:9")
	assert.Equal(t, "cpe:/o:redhat:enterprise_linux:9", cpe)
}

func TestNormalizeCPE_CPE23ToURI(t *testing.T) {
	cpe := normalizeCPE("cpe:2.3:o:suse:sles:15:sp5:*:*:*:*:*:*")
	assert.Equal(t, "cpe:/o:suse:sles:15:sp5", cpe)
}

func TestNormalizeCPE_CPE23AllWildcards(t *testing.T) {
	cpe := normalizeCPE("cpe:2.3:o:centos:centos:10:*:*:*:*:*:*:*")
	assert.Equal(t, "cpe:/o:centos:centos:10", cpe)
}

func TestNormalizeCPE_EmptyString(t *testing.T) {
	cpe := normalizeCPE("")
	assert.Equal(t, "", cpe)
}

func TestCPEMatches_ExactMatch(t *testing.T) {
	assert.True(t, cpeMatches(
		"cpe:/o:centos:centos:10",
		"cpe:/o:centos:centos:10",
	))
}

func TestCPEMatches_SystemHasMoreComponents(t *testing.T) {
	assert.True(t, cpeMatches(
		"cpe:/o:suse:sles:15:sp5",
		"cpe:/o:suse:sles:15",
	))
}

func TestCPEMatches_DatastreamHasMoreComponents(t *testing.T) {
	assert.True(t, cpeMatches(
		"cpe:/o:suse:sles:15",
		"cpe:/o:suse:sles:15:sp5",
	))
}

func TestCPEMatches_DifferentProduct(t *testing.T) {
	assert.False(t, cpeMatches(
		"cpe:/o:centos:centos:10",
		"cpe:/o:redhat:enterprise_linux:10",
	))
}

func TestCPEMatches_DifferentVersion(t *testing.T) {
	assert.False(t, cpeMatches(
		"cpe:/o:redhat:enterprise_linux:7",
		"cpe:/o:redhat:enterprise_linux:7.9:GA:server",
	))
}

func TestCPEMatches_CrossFormat_CPE23SystemVs22Datastream(t *testing.T) {
	assert.True(t, cpeMatches(
		"cpe:2.3:o:centos:centos:10:*:*:*:*:*:*:*",
		"cpe:/o:centos:centos:10",
	))
}

func TestCPEMatches_CaseInsensitive(t *testing.T) {
	assert.True(t, cpeMatches(
		"cpe:/o:RedHat:Enterprise_Linux:9",
		"cpe:/o:redhat:enterprise_linux:9",
	))
}

func TestExtractCPEFromOsRelease_CentOSStream(t *testing.T) {
	osRelease := `NAME="CentOS Stream"
VERSION="10"
ID=centos
ID_LIKE="rhel fedora"
VERSION_ID="10"
CPE_NAME="cpe:/o:centos:centos:10"
`
	cpe, err := extractCPEFromOsRelease(strings.NewReader(osRelease))
	require.NoError(t, err)
	assert.Equal(t, "cpe:/o:centos:centos:10", cpe)
}

func TestExtractCPEFromOsRelease_RHEL(t *testing.T) {
	osRelease := `NAME="Red Hat Enterprise Linux"
VERSION="9.4 (Plow)"
ID="rhel"
VERSION_ID="9.4"
CPE_NAME="cpe:/o:redhat:enterprise_linux:9"
`
	cpe, err := extractCPEFromOsRelease(strings.NewReader(osRelease))
	require.NoError(t, err)
	assert.Equal(t, "cpe:/o:redhat:enterprise_linux:9", cpe)
}

func TestExtractCPEFromOsRelease_MissingCPE(t *testing.T) {
	osRelease := `NAME="Test Distro"
ID=test
VERSION_ID="1.0"
`
	_, err := extractCPEFromOsRelease(strings.NewReader(osRelease))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CPE_NAME not found")
}

func TestExtractCPEFromOsRelease_EmptyCPE(t *testing.T) {
	osRelease := `CPE_NAME=""
`
	_, err := extractCPEFromOsRelease(strings.NewReader(osRelease))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CPE_NAME field is empty")
}

func TestExtractCPEFromOsRelease_SingleQuoted(t *testing.T) {
	osRelease := `NAME="CentOS Stream"
CPE_NAME='cpe:/o:centos:centos:10'
`
	cpe, err := extractCPEFromOsRelease(strings.NewReader(osRelease))
	require.NoError(t, err)
	assert.Equal(t, "cpe:/o:centos:centos:10", cpe)
}

func TestExtractCPEFromOsRelease_Unquoted(t *testing.T) {
	osRelease := `CPE_NAME=cpe:/o:centos:centos:10
`
	cpe, err := extractCPEFromOsRelease(strings.NewReader(osRelease))
	require.NoError(t, err)
	assert.Equal(t, "cpe:/o:centos:centos:10", cpe)
}

func TestExtractCPEFromOsRelease_ReaderError(t *testing.T) {
	_, err := extractCPEFromOsRelease(iotest.ErrReader(fmt.Errorf("disk failure")))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading os-release data")
	assert.ErrorContains(t, err, "disk failure")
}

func TestExtractCPEFromDatastream(t *testing.T) {
	datastreamXML := `<?xml version="1.0" encoding="UTF-8"?>
<data-stream-collection xmlns="http://scap.nist.gov/schema/scap/source/1.2">
  <component id="comp1">
    <cpe-list xmlns="http://cpe.mitre.org/dictionary/2.0">
      <cpe-item name="cpe:/o:centos:centos:10">
        <title>CentOS Stream 10</title>
      </cpe-item>
      <cpe-item name="cpe:/o:redhat:enterprise_linux:10">
        <title>Red Hat Enterprise Linux 10</title>
      </cpe-item>
    </cpe-list>
  </component>
</data-stream-collection>`

	fsys := fstest.MapFS{
		"ssg-cs10-ds.xml": &fstest.MapFile{Data: []byte(datastreamXML)},
	}

	cpes, err := extractCPEFromDatastream(fsys, "ssg-cs10-ds.xml")
	require.NoError(t, err)
	assert.Len(t, cpes, 2)
	assert.Contains(t, cpes, "cpe:/o:centos:centos:10")
	assert.Contains(t, cpes, "cpe:/o:redhat:enterprise_linux:10")
}

func TestExtractCPEFromDatastream_NoCPEItems(t *testing.T) {
	datastreamXML := `<?xml version="1.0" encoding="UTF-8"?>
<data-stream-collection xmlns="http://scap.nist.gov/schema/scap/source/1.2">
  <component id="comp1">
    <title>No CPE here</title>
  </component>
</data-stream-collection>`

	fsys := fstest.MapFS{
		"test-ds.xml": &fstest.MapFile{Data: []byte(datastreamXML)},
	}

	_, err := extractCPEFromDatastream(fsys, "test-ds.xml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no CPE entries found")
}

func TestExtractCPEFromDatastream_InvalidXML(t *testing.T) {
	fsys := fstest.MapFS{
		"bad-ds.xml": &fstest.MapFile{Data: []byte("<invalid>")},
	}

	_, err := extractCPEFromDatastream(fsys, "bad-ds.xml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse XML")
}

func TestExtractCPEFromDatastream_EmptyNameAttributes(t *testing.T) {
	datastreamXML := `<?xml version="1.0" encoding="UTF-8"?>
<data-stream-collection xmlns="http://scap.nist.gov/schema/scap/source/1.2">
  <component id="comp1">
    <cpe-list xmlns="http://cpe.mitre.org/dictionary/2.0">
      <cpe-item name="">
        <title>Empty CPE</title>
      </cpe-item>
    </cpe-list>
  </component>
</data-stream-collection>`

	fsys := fstest.MapFS{
		"ssg-test-ds.xml": &fstest.MapFile{Data: []byte(datastreamXML)},
	}

	_, err := extractCPEFromDatastream(fsys, "ssg-test-ds.xml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no valid CPE entries")
}

func TestMatchDatastreamByCPE_CentOSStream(t *testing.T) {
	centosXML := `<?xml version="1.0" encoding="UTF-8"?>
<data-stream-collection xmlns="http://scap.nist.gov/schema/scap/source/1.2">
  <component>
    <cpe-list xmlns="http://cpe.mitre.org/dictionary/2.0">
      <cpe-item name="cpe:/o:centos:centos:10"><title>CentOS 10</title></cpe-item>
    </cpe-list>
  </component>
</data-stream-collection>`

	rhelXML := `<?xml version="1.0" encoding="UTF-8"?>
<data-stream-collection xmlns="http://scap.nist.gov/schema/scap/source/1.2">
  <component>
    <cpe-list xmlns="http://cpe.mitre.org/dictionary/2.0">
      <cpe-item name="cpe:/o:redhat:enterprise_linux:10"><title>RHEL 10</title></cpe-item>
    </cpe-list>
  </component>
</data-stream-collection>`

	fsys := fstest.MapFS{
		"ssg-cs10-ds.xml":   &fstest.MapFile{Data: []byte(centosXML)},
		"ssg-rhel10-ds.xml": &fstest.MapFile{Data: []byte(rhelXML)},
	}

	got, err := matchDatastreamByCPE(fsys, "cpe:/o:centos:centos:10", "/test/ssg")
	require.NoError(t, err)
	assert.Equal(t, "ssg-cs10-ds.xml", got)
}

func TestMatchDatastreamByCPE_PrefixMatch(t *testing.T) {
	slesXML := `<?xml version="1.0" encoding="UTF-8"?>
<data-stream-collection xmlns="http://scap.nist.gov/schema/scap/source/1.2">
  <component>
    <cpe-list xmlns="http://cpe.mitre.org/dictionary/2.0">
      <cpe-item name="cpe:/o:suse:sles:15"><title>SLES 15</title></cpe-item>
    </cpe-list>
  </component>
</data-stream-collection>`

	fsys := fstest.MapFS{
		"ssg-sle15-ds.xml": &fstest.MapFile{Data: []byte(slesXML)},
	}

	got, err := matchDatastreamByCPE(fsys, "cpe:/o:suse:sles:15:sp5", "/test/ssg")
	require.NoError(t, err)
	assert.Equal(t, "ssg-sle15-ds.xml", got)
}

func TestMatchDatastreamByCPE_CrossFormatMatch(t *testing.T) {
	centosXML := `<?xml version="1.0" encoding="UTF-8"?>
<data-stream-collection xmlns="http://scap.nist.gov/schema/scap/source/1.2">
  <component>
    <cpe-list xmlns="http://cpe.mitre.org/dictionary/2.0">
      <cpe-item name="cpe:/o:centos:centos:10"><title>CentOS 10</title></cpe-item>
    </cpe-list>
  </component>
</data-stream-collection>`

	fsys := fstest.MapFS{
		"ssg-cs10-ds.xml": &fstest.MapFile{Data: []byte(centosXML)},
	}

	got, err := matchDatastreamByCPE(
		fsys,
		"cpe:2.3:o:centos:centos:10:*:*:*:*:*:*:*",
		"/test/ssg",
	)
	require.NoError(t, err)
	assert.Equal(t, "ssg-cs10-ds.xml", got)
}

func TestMatchDatastreamByCPE_NoMatch(t *testing.T) {
	rhelXML := `<?xml version="1.0" encoding="UTF-8"?>
<data-stream-collection xmlns="http://scap.nist.gov/schema/scap/source/1.2">
  <component>
    <cpe-list xmlns="http://cpe.mitre.org/dictionary/2.0">
      <cpe-item name="cpe:/o:redhat:enterprise_linux:10"><title>RHEL 10</title></cpe-item>
    </cpe-list>
  </component>
</data-stream-collection>`

	fsys := fstest.MapFS{
		"ssg-rhel10-ds.xml": &fstest.MapFile{Data: []byte(rhelXML)},
	}

	_, err := matchDatastreamByCPE(fsys, "cpe:/o:centos:centos:10", "/test/ssg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no datastream matches system CPE")
	assert.Contains(t, err.Error(), "cpe:/o:centos:centos:10")
	assert.Contains(t, err.Error(), "ssg-rhel10-ds.xml")
}

func TestMatchDatastreamByCPE_NoValidDatastreams(t *testing.T) {
	fsys := fstest.MapFS{
		"README.md":      &fstest.MapFile{Data: []byte("not a datastream")},
		"invalid-ds.xml": &fstest.MapFile{Data: []byte("<invalid>")},
	}

	_, err := matchDatastreamByCPE(fsys, "cpe:/o:test:test:1", "/test/ssg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no valid SSG datastreams found")
}

func TestMatchDatastreamByCPE_AllParseFailures(t *testing.T) {
	fsys := fstest.MapFS{
		"ssg-rhel10-ds.xml": &fstest.MapFile{Data: []byte("not xml at all")},
		"ssg-cs10-ds.xml":   &fstest.MapFile{Data: []byte("{json, not xml}")},
	}

	_, err := matchDatastreamByCPE(fsys, "cpe:/o:test:test:1", "/test/ssg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse")
	assert.Contains(t, err.Error(), "2 files")
}

// failFS is an fs.FS that always returns an error on ReadDir.
type failFS struct{}

func (failFS) Open(string) (fs.File, error) {
	return nil, fmt.Errorf("open not supported")
}

func (failFS) ReadDir(string) ([]fs.DirEntry, error) {
	return nil, fmt.Errorf("permission denied")
}

func TestMatchDatastreamByCPE_ReadDirError(t *testing.T) {
	_, err := matchDatastreamByCPE(failFS{}, "cpe:/o:test:test:1", "/test/ssg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading datastream directory")
	assert.ErrorContains(t, err, "permission denied")
}
