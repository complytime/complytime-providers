// SPDX-License-Identifier: Apache-2.0

package targets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRepoURL_GitHub(t *testing.T) {
	platform, org, repo, err := ParseRepoURL("https://github.com/complytime/policies", "")
	require.NoError(t, err)
	assert.Equal(t, "github", platform)
	assert.Equal(t, "complytime", org)
	assert.Equal(t, "policies", repo)
}

func TestParseRepoURL_GitLab(t *testing.T) {
	platform, org, repo, err := ParseRepoURL("https://gitlab.com/group/subgroup/project", "")
	require.NoError(t, err)
	assert.Equal(t, "gitlab", platform)
	assert.Equal(t, "group/subgroup", org)
	assert.Equal(t, "project", repo)
}

func TestParseRepoURL_PlatformHint(t *testing.T) {
	platform, org, repo, err := ParseRepoURL("https://git.internal.io/team/repo", "gitlab")
	require.NoError(t, err)
	assert.Equal(t, "gitlab", platform)
	assert.Equal(t, "team", org)
	assert.Equal(t, "repo", repo)
}

func TestParseRepoURL_InvalidScheme(t *testing.T) {
	_, _, _, err := ParseRepoURL("http://github.com/org/repo", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HTTPS")
}

func TestParseRepoURL_SSHScheme(t *testing.T) {
	_, _, _, err := ParseRepoURL("ssh://git@github.com/org/repo", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HTTPS")
}

func TestParseRepoURL_EmptyURL(t *testing.T) {
	_, _, _, err := ParseRepoURL("", "")
	assert.Error(t, err)
}

func TestParseRepoURL_MissingPath(t *testing.T) {
	_, _, _, err := ParseRepoURL("https://github.com/", "")
	assert.Error(t, err)
}

func TestSanitizeRepoURL(t *testing.T) {
	result := SanitizeRepoURL("https://github.com/org/repo")
	assert.NotContains(t, result, "/")
	assert.NotContains(t, result, ".")
	assert.NotContains(t, result, ":")
}

func TestSanitizeRepoURL_VariousFormats(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"github", "https://github.com/org/repo"},
		{"gitlab", "https://gitlab.com/group/project"},
		{"custom", "https://git.internal.io/team/repo"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := SanitizeRepoURL(tc.input)
			assert.NotContains(t, result, "/")
			assert.NotContains(t, result, ".")
			assert.NotContains(t, result, ":")
			assert.NotEmpty(t, result)
			// Deterministic: same input always produces same output
			assert.Equal(t, result, SanitizeRepoURL(tc.input))
		})
	}
}

func TestRepoDisplayName(t *testing.T) {
	name := RepoDisplayName("https://github.com/complytime/policies")
	assert.Equal(t, "complytime/policies", name)
}

func TestRepoDisplayName_InvalidURL(t *testing.T) {
	name := RepoDisplayName("not-a-url")
	assert.Equal(t, "not-a-url", name)
}

func TestValidateInputPath_Valid(t *testing.T) {
	dir := t.TempDir()
	err := ValidateInputPath(dir)
	assert.NoError(t, err)
}

func TestValidateInputPath_ValidFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.yaml")
	require.NoError(t, os.WriteFile(f, []byte("test"), 0600))

	err := ValidateInputPath(f)
	assert.NoError(t, err)
}

func TestValidateInputPath_Traversal(t *testing.T) {
	err := ValidateInputPath("../../etc/passwd")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "traversal")
}

func TestValidateInputPath_NotExist(t *testing.T) {
	err := ValidateInputPath("/nonexistent/path/to/config")
	assert.Error(t, err)
}
