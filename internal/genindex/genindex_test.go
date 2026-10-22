// SPDX-License-Identifier: Apache-2.0

package genindex

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContentKey_DeterministicAndSourceSensitive(t *testing.T) {
	requirementIDs := []string{"requirement-b", "requirement-a"}
	key := ContentKey(requirementIDs, "/policy/source")

	expectedDigest := sha256.Sum256([]byte("requirement-a\nrequirement-b\n/policy/source"))
	expectedKey := hex.EncodeToString(expectedDigest[:])[:keyLength]
	assert.Equal(t, expectedKey, key)
	assert.Equal(
		t, key, ContentKey([]string{"requirement-a", "requirement-b"}, "/policy/source"),
	)
	assert.Equal(t, []string{"requirement-b", "requirement-a"}, requirementIDs)
	assert.NotEqual(
		t, key, ContentKey([]string{"requirement-a", "requirement-c"}, "/policy/source"),
	)
	assert.NotEqual(
		t, key, ContentKey([]string{"requirement-a", "requirement-b"}, "/other/source"),
	)
}

func TestIndex_CreateReadAndUpdate(t *testing.T) {
	indexDir := filepath.Join(t.TempDir(), "generated")
	initialIndex := GenerateIndex{
		Version: indexVersion,
		Targets: map[string]string{"target-one": "0123456789abcdef"},
	}

	require.NoError(t, WriteIndex(indexDir, initialIndex))

	contents, err := os.ReadFile(filepath.Join(indexDir, indexFilename))
	require.NoError(t, err)
	assert.NotContains(t, string(contents), "\n")

	index, err := ReadIndex(indexDir)
	require.NoError(t, err)
	assert.Equal(t, initialIndex, index)

	require.NoError(t, UpdateIndex(indexDir, "target-one", "fedcba9876543210"))
	updatedIndex, err := ReadIndex(indexDir)
	require.NoError(t, err)
	assert.Equal(t, "fedcba9876543210", updatedIndex.Targets["target-one"])
}

func TestUpdateIndex_CreatesNewIndex(t *testing.T) {
	indexDir := filepath.Join(t.TempDir(), "generated")

	require.NoError(t, UpdateIndex(indexDir, "target-one", "0123456789abcdef"))

	index, err := ReadIndex(indexDir)
	require.NoError(t, err)
	assert.Equal(t, indexVersion, index.Version)
	assert.Equal(
		t, map[string]string{"target-one": "0123456789abcdef"}, index.Targets,
	)
}

func TestReadIndex_MalformedOrUnsupported(t *testing.T) {
	tests := []struct {
		name     string
		contents string
		contains string
	}{
		{
			name:     "malformed JSON",
			contents: `{`,
			contains: "parse generate index",
		},
		{
			name:     "unsupported version",
			contents: `{"version":2,"targets":{}}`,
			contains: "unsupported generate index version 2",
		},
		{
			name:     "missing targets",
			contents: `{"version":1}`,
			contains: "targets must be an object",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			indexDir := t.TempDir()
			require.NoError(t, os.WriteFile(
				filepath.Join(indexDir, indexFilename), []byte(tc.contents), fileMode,
			))

			_, err := ReadIndex(indexDir)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.contains)
		})
	}
}

func TestTargetKey_URLAndInputPath(t *testing.T) {
	urlKey := TargetKey("https://github.com/org/repo")
	assert.Equal(t, urlKey, TargetKey("http://user:token@github.com/org/repo"))
	assert.NotEqual(t, urlKey, TargetKey("https://github.com/org/other-repo"))

	inputPathKey := TargetKey("./fixtures/../github.com/org/repo")
	assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{16}$`), inputPathKey)
	assert.Equal(t, inputPathKey, TargetKey("github.com/org/repo"))
	assert.NotEqual(t, inputPathKey, TargetKey("https://github.com/org/repo"))
}

func TestValidateContentKey_ValidAndInvalid(t *testing.T) {
	generatedDir := t.TempDir()
	validKey := "0123456789abcdef"
	require.NoError(t, ValidateContentKey(validKey, generatedDir))

	tests := []struct {
		name       string
		contentKey string
	}{
		{name: "path traversal", contentKey: "../etc"},
		{name: "invalid characters", contentKey: "0123456789abcdeg"},
		{name: "empty", contentKey: ""},
		{name: "wrong length", contentKey: "0123456789abcde"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateContentKey(tc.contentKey, generatedDir)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid content key")
		})
	}
}

func TestResolveConfig_ReadsAndCachesByContentKey(t *testing.T) {
	generatedDir := t.TempDir()
	contentKey := "0123456789abcdef"
	index := GenerateIndex{Version: indexVersion, Targets: map[string]string{"target": contentKey}}
	cache := make(map[string]*string)
	readCalls := 0

	readConfig := func(path string) (*string, error) {
		readCalls++
		assert.Equal(t, filepath.Join(generatedDir, contentKey), path)
		config := "config"
		return &config, nil
	}

	first, firstKey, err := ResolveConfig("target", index, generatedDir, cache, readConfig)
	require.NoError(t, err)
	second, secondKey, err := ResolveConfig("target", index, generatedDir, cache, readConfig)
	require.NoError(t, err)

	assert.Equal(t, contentKey, firstKey)
	assert.Equal(t, contentKey, secondKey)
	assert.Equal(t, "config", *first)
	assert.Same(t, first, second)
	assert.Equal(t, 1, readCalls)
}

func TestResolveConfig_MissingTargetAndReadError(t *testing.T) {
	generatedDir := t.TempDir()
	index := GenerateIndex{Version: indexVersion, Targets: map[string]string{}}
	_, _, err := ResolveConfig[string]("missing", index, generatedDir, nil, nil)
	require.ErrorContains(t, err, "target is missing from generate index")

	index.Targets["target"] = "0123456789abcdef"
	_, _, err = ResolveConfig("target", index, generatedDir, make(map[string]*string), func(string) (*string, error) {
		return nil, errors.New("read failed")
	})
	require.ErrorContains(t, err, "reading generated scan config: read failed")
}
