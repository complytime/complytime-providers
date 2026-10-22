// SPDX-License-Identifier: Apache-2.0

// Package genindex manages target-to-generated-artifact indexes for providers.
package genindex

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	indexFilename = "generate-index.json"
	indexVersion  = 1
	keyLength     = 16
	fileMode      = 0o644
	directoryMode = 0o755
	urlKeyPrefix  = "url:"
	pathKeyPrefix = "path:"
)

var contentKeyPattern = regexp.MustCompile(`^[0-9a-f]{16}$`)

// GenerateIndex maps target keys to content-addressed generated artifact directories.
type GenerateIndex struct {
	Version int               `json:"version"`
	Targets map[string]string `json:"targets"`
}

// ContentKey returns the deterministic content key for requirement IDs and a policy source.
func ContentKey(requirementIDs []string, policySourceID string) string {
	sortedRequirementIDs := append([]string(nil), requirementIDs...)
	sort.Strings(sortedRequirementIDs)

	content := strings.Join(sortedRequirementIDs, "\n") + "\n" + policySourceID
	return hashPrefix(content)
}

// ReadIndex reads and validates the generate index stored in indexDir.
func ReadIndex(indexDir string) (GenerateIndex, error) {
	indexPath := filepath.Join(indexDir, indexFilename)
	contents, err := os.ReadFile(indexPath)
	if err != nil {
		return GenerateIndex{}, fmt.Errorf("read generate index: %w", err)
	}

	var index GenerateIndex
	if err := json.Unmarshal(contents, &index); err != nil {
		return GenerateIndex{}, fmt.Errorf("parse generate index: %w", err)
	}
	if index.Version != indexVersion {
		return GenerateIndex{}, fmt.Errorf(
			"unsupported generate index version %d", index.Version,
		)
	}
	if index.Targets == nil {
		return GenerateIndex{}, fmt.Errorf("generate index targets must be an object")
	}

	return index, nil
}

// WriteIndex validates and writes index to generate-index.json in indexDir.
func WriteIndex(indexDir string, index GenerateIndex) error {
	if index.Version != indexVersion {
		return fmt.Errorf("unsupported generate index version %d", index.Version)
	}
	if index.Targets == nil {
		return fmt.Errorf("generate index targets must be an object")
	}

	contents, err := json.Marshal(index)
	if err != nil {
		return fmt.Errorf("marshal generate index: %w", err)
	}
	if err := os.MkdirAll(indexDir, directoryMode); err != nil {
		return fmt.Errorf("create generate index directory: %w", err)
	}
	if err := os.WriteFile(
		filepath.Join(indexDir, indexFilename), contents, fileMode,
	); err != nil {
		return fmt.Errorf("write generate index: %w", err)
	}

	return nil
}

// UpdateIndex records contentKey for targetKey in the index stored in indexDir.
//
// Concurrent complyctl instances sharing a workspace may race during this
// read-modify-write cycle. File-level locking is a future consideration;
// complyctl currently generates sequentially.
func UpdateIndex(indexDir, targetKey, contentKey string) error {
	index, err := ReadIndex(indexDir)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("read generate index for update: %w", err)
		}
		index = GenerateIndex{
			Version: indexVersion,
			Targets: make(map[string]string),
		}
	}

	index.Targets[targetKey] = contentKey
	if err := WriteIndex(indexDir, index); err != nil {
		return fmt.Errorf("write updated generate index: %w", err)
	}

	return nil
}

// TargetKey returns a collision-resistant key for a target URL or input path.
func TargetKey(urlOrPath string) string {
	parsed, err := url.Parse(urlOrPath)
	if err == nil && parsed.Host != "" {
		parsed.Scheme = ""
		parsed.User = nil
		return hashPrefix(urlKeyPrefix + strings.TrimPrefix(parsed.String(), "//"))
	}

	return hashPrefix(pathKeyPrefix + filepath.Clean(urlOrPath))
}

// ValidateContentKey verifies that contentKey is safe to use within generatedDir.
func ValidateContentKey(contentKey, generatedDir string) error {
	if !contentKeyPattern.MatchString(contentKey) {
		return fmt.Errorf(
			"invalid content key %q: expected %d lowercase hexadecimal characters",
			contentKey, keyLength,
		)
	}

	absoluteGeneratedDir, err := filepath.Abs(generatedDir)
	if err != nil {
		return fmt.Errorf("resolve generated directory: %w", err)
	}
	absoluteContentDir, err := filepath.Abs(
		filepath.Join(absoluteGeneratedDir, contentKey),
	)
	if err != nil {
		return fmt.Errorf("resolve content key directory: %w", err)
	}
	relativePath, err := filepath.Rel(absoluteGeneratedDir, absoluteContentDir)
	if err != nil {
		return fmt.Errorf("check content key directory: %w", err)
	}
	if relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(os.PathSeparator)) ||
		filepath.IsAbs(relativePath) {
		return fmt.Errorf("content key %q resolves outside generated directory", contentKey)
	}

	return nil
}

// ResolveConfig resolves and reads a target's generated configuration, caching by content key.
func ResolveConfig[T any](
	targetKey string,
	index GenerateIndex,
	generatedDir string,
	cache map[string]*T,
	readConfig func(string) (*T, error),
) (*T, string, error) {
	contentKey, ok := index.Targets[targetKey]
	if !ok {
		return nil, "", fmt.Errorf("target is missing from generate index")
	}
	if err := ValidateContentKey(contentKey, generatedDir); err != nil {
		return nil, "", fmt.Errorf("validating content key: %w", err)
	}
	if config, ok := cache[contentKey]; ok {
		return config, contentKey, nil
	}

	config, err := readConfig(filepath.Join(generatedDir, contentKey))
	if err != nil {
		return nil, "", fmt.Errorf("reading generated scan config: %w", err)
	}
	cache[contentKey] = config
	return config, contentKey, nil
}

func hashPrefix(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])[:keyLength]
}
