// SPDX-License-Identifier: Apache-2.0

package evidence

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileDigest_KnownContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := []byte("hello evidence")
	require.NoError(t, os.WriteFile(path, content, 0o600))

	digest, err := FileDigest(path)
	require.NoError(t, err)

	expected := fmt.Sprintf("sha256:%x", sha256.Sum256(content))
	assert.Equal(t, expected, digest)
	assert.Regexp(t, `^sha256:[a-f0-9]{64}$`, digest)
}

func TestFileDigest_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	require.NoError(t, os.WriteFile(path, []byte{}, 0o600))

	digest, err := FileDigest(path)
	require.NoError(t, err)

	expected := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte{}))
	assert.Equal(t, expected, digest)
}

func TestFileDigest_NonExistentPath(t *testing.T) {
	_, err := FileDigest("/nonexistent/path/file.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "evidence digest")
}

func TestFileDigest_DirectoryPath(t *testing.T) {
	dir := t.TempDir()
	_, err := FileDigest(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is not a regular file")
}

func TestFileDigest_SymlinkToDirectory(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "link-to-dir")
	require.NoError(t, os.Symlink(dir, link))

	_, err := FileDigest(link)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is not a regular file")
}

func TestFileDigest_UnreadableFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unreadable.txt")
	require.NoError(t, os.WriteFile(path, []byte("secret"), 0o600))
	require.NoError(t, os.Chmod(path, 0o000))
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	_, err := FileDigest(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "evidence digest")
}

func TestHostRemark_BothAvailable(t *testing.T) {
	dir := t.TempDir()
	idFile := filepath.Join(dir, "machine-id")
	require.NoError(t, os.WriteFile(idFile, []byte("abc123def456\n"), 0o600))

	t.Setenv(EnvMachineIDFile, idFile)

	remark := HostRemark()
	assert.Contains(t, remark, "Collected on host")
	assert.Contains(t, remark, "(machine-id: abc123def456)")
	assert.NotContains(t, remark, "\n")
}

func TestHostRemark_NoMachineID(t *testing.T) {
	t.Setenv(EnvMachineIDFile, "/nonexistent/machine-id")

	remark := HostRemark()
	assert.Contains(t, remark, "Collected on host")
	assert.NotContains(t, remark, "machine-id")
}

func TestHostRemark_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	idFile := filepath.Join(dir, "custom-machine-id")
	require.NoError(t, os.WriteFile(idFile, []byte("caf3babe00112233\n"), 0o600))

	t.Setenv(EnvMachineIDFile, idFile)

	remark := HostRemark()
	assert.Contains(t, remark, "(machine-id: caf3babe00112233)")
}

func TestHostRemark_MachineIDTrimmed(t *testing.T) {
	dir := t.TempDir()
	idFile := filepath.Join(dir, "machine-id")
	require.NoError(t, os.WriteFile(
		idFile, []byte("  abc123  \n\n"), 0o600,
	))

	t.Setenv(EnvMachineIDFile, idFile)

	remark := HostRemark()
	assert.Contains(t, remark, "(machine-id: abc123)")
	assert.NotContains(t, remark, " abc123 ")
}

func TestHostRemark_OversizedMachineIDIgnored(t *testing.T) {
	dir := t.TempDir()
	idFile := filepath.Join(dir, "machine-id")
	// Write a file larger than maxMachineIDSize (1KB)
	largeContent := make([]byte, maxMachineIDSize+1)
	for i := range largeContent {
		largeContent[i] = 'a'
	}
	require.NoError(t, os.WriteFile(idFile, largeContent, 0o600))

	t.Setenv(EnvMachineIDFile, idFile)

	remark := HostRemark()
	assert.NotContains(t, remark, "machine-id",
		"oversized machine-id files should be silently ignored")
}

func TestHostRemark_NonHexMachineIDIgnored(t *testing.T) {
	dir := t.TempDir()
	idFile := filepath.Join(dir, "machine-id")
	require.NoError(t, os.WriteFile(
		idFile, []byte("not-a-hex-string!\n"), 0o600,
	))

	t.Setenv(EnvMachineIDFile, idFile)

	remark := HostRemark()
	assert.NotContains(t, remark, "machine-id",
		"non-hex machine-id content should be silently ignored")
}

func TestSanitizeForID_SlashDistinction(t *testing.T) {
	// Forward slashes encode as "--", keeping them distinct from
	// single hyphens. This prevents collisions between path-like
	// and hyphenated inputs.
	pairs := [][2]string{
		{"a/b", "a-b"},
		{"org/repo", "org-repo"},
	}
	for _, pair := range pairs {
		a := SanitizeForID(pair[0])
		b := SanitizeForID(pair[1])
		assert.NotEqual(t, a, b,
			"SanitizeForID(%q) == SanitizeForID(%q) == %q",
			pair[0], pair[1], a)
	}
}

func TestSanitizeForID_NonSlashSpecialCharsCollapse(t *testing.T) {
	// Non-slash special characters (dots, colons, spaces, etc.)
	// all map to "-", so they intentionally collide with each
	// other and with literal hyphens. This is a deliberate
	// readability trade-off: these collisions are not realistic
	// for evidence identity inputs (repo URLs, branch names,
	// spec paths).
	groups := [][]string{
		{"a.b", "a:b", "a b", "a-b"},
	}
	for _, group := range groups {
		first := SanitizeForID(group[0])
		for _, s := range group[1:] {
			assert.Equal(t, first, SanitizeForID(s),
				"non-slash special chars should all map to hyphen")
		}
	}
}

func TestSanitizeForID_KnownLossyCollisions(t *testing.T) {
	// Documents known collision classes where the lossy encoding
	// produces identical output for distinct inputs. These are
	// accepted trade-offs for readability. In practice, JoinForID
	// multi-field composition (repo + branch + spec) mitigates
	// real-world impact: all fields must collide simultaneously,
	// and git hosting platforms do not allow "/" in repo names.
	//
	// Consecutive non-alphanumeric characters produce "--",
	// colliding with the "/" encoding:
	collisionGroup := []string{
		"a/b",  // slash encodes to "--"
		"a--b", // two hyphens pass through
		"a..b", // two dots each encode to "-"
		"a-.b", // hyphen passes through, dot encodes to "-"
		"a.-b", // dot encodes to "-", hyphen passes through
	}
	expected := "a--b"
	for _, input := range collisionGroup {
		assert.Equal(t, expected, SanitizeForID(input),
			"known lossy collision: SanitizeForID(%q)", input)
	}
}

func TestSanitizeForID_Passthrough(t *testing.T) {
	// Alphanumeric and hyphens pass through unchanged.
	input := "abc-DEF-123"
	assert.Equal(t, input, SanitizeForID(input))
}

func TestSanitizeForID_SpecialChars(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/", "--"},
		{".", "-"},
		{":", "-"},
		{" ", "-"},
		{"_", "-"},
		{"a/b.c:d", "a--b-c-d"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, SanitizeForID(tc.input))
		})
	}
}

func TestSanitizeForID_NoUnderscoresInOutput(t *testing.T) {
	// Underscores must never appear in SanitizeForID output so
	// they remain unambiguous as JoinForID component separators.
	inputs := []string{
		"a_b", "hello_world", "my_repo",
		"github.com/org/my_repo",
	}
	for _, input := range inputs {
		result := SanitizeForID(input)
		assert.NotContains(t, result, "_",
			"SanitizeForID(%q) = %q must not contain underscores",
			input, result)
	}
}

func TestFileURL_AbsolutePath(t *testing.T) {
	got := FileURL("/opt/results/arf.xml")
	parsed, err := url.Parse(got)
	require.NoError(t, err)
	assert.Equal(t, "file", parsed.Scheme)
	assert.Empty(t, parsed.Host, "file URL must have empty host")
	assert.Equal(t, "/opt/results/arf.xml", parsed.Path)
}

func TestFileURL_RelativePath(t *testing.T) {
	got := FileURL(".complytime/results/arf.xml")
	parsed, err := url.Parse(got)
	require.NoError(t, err)
	assert.Equal(t, "file", parsed.Scheme)
	assert.Empty(t, parsed.Host,
		"relative paths must not produce a host component")

	if runtime.GOOS != "windows" {
		assert.True(t, strings.HasPrefix(parsed.Path, "/"),
			"resolved path must be absolute: %s", parsed.Path)
	}
	assert.True(t, strings.HasSuffix(parsed.Path,
		".complytime/results/arf.xml"),
		"resolved path must end with the original relative path")
}

func TestFileURL_SpacesAndSpecialChars(t *testing.T) {
	got := FileURL("/path/with spaces/file#1.xml")
	parsed, err := url.Parse(got)
	require.NoError(t, err)
	assert.Empty(t, parsed.Host)
	assert.Equal(t, "/path/with spaces/file#1.xml", parsed.Path,
		"url.Parse should decode percent-encoded path back correctly")
}

func TestJoinForID_CollisionResistance(t *testing.T) {
	// ("a/b", "c") and ("a", "b/c") must produce distinct IDs.
	// With the underscore separator: "a--b_c" vs "a_b--c".
	id1 := JoinForID("a/b", "c")
	id2 := JoinForID("a", "b/c")
	assert.Equal(t, "a--b_c", id1)
	assert.Equal(t, "a_b--c", id2)
	assert.NotEqual(t, id1, id2,
		"JoinForID must distinguish components with slashes")

	// Same components produce same result
	assert.Equal(t, JoinForID("x", "y"), JoinForID("x", "y"))

	// Single component equals SanitizeForID
	assert.Equal(t, SanitizeForID("hello"), JoinForID("hello"))
}

func TestJoinForID_UnderscoreSeparator(t *testing.T) {
	// Components are joined with underscore separators.
	assert.Equal(t, "a_b_c", JoinForID("a", "b", "c"))
	assert.Equal(t,
		"github-com--org--repo_main_branch-rules",
		JoinForID("github.com/org/repo", "main", "branch-rules"),
	)
}

func TestStripURLScheme(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://github.com/org/repo", "github.com/org/repo"},
		{"http://gitlab.com/group/proj", "gitlab.com/group/proj"},
		{"github.com/org/repo", "github.com/org/repo"},
		{"/local/path", "/local/path"},
		{"", ""},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, StripURLScheme(tc.input),
			"input: %s", tc.input)
	}
}

func TestIsHexString(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"abc123def456", true},
		{"ABCDEF012345", true},
		{"0123456789abcdef", true},
		{"", false},
		{"not-hex", false},
		{"abc 123", false},
		{"abc\n123", false},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.want, isHexString(tc.input))
		})
	}
}
