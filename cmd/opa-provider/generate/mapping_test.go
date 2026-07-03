// SPDX-License-Identifier: Apache-2.0

package generate

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadMapping_Valid(t *testing.T) {
	dir := t.TempDir()
	data := `{
		"version": "1",
		"mappings": [
			{"id": "kubernetes.run_as_root", "requirement_id": "CIS-K8S-5.2.6"},
			{"id": "kubernetes.resource_limits", "requirement_id": "CIS-K8S-5.4.1"}
		]
	}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, MappingFileName), []byte(data), 0600))

	mapping, err := LoadMapping(dir)
	require.NoError(t, err)
	assert.Equal(t, "1", mapping.Version)
	assert.Len(t, mapping.Mappings, 2)
	assert.Equal(t, "kubernetes.run_as_root", mapping.Mappings[0].ID)
	assert.Equal(t, "CIS-K8S-5.2.6", mapping.Mappings[0].RequirementID)
}

func TestLoadMapping_MissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadMapping(dir)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrMappingNotFound),
		"missing file error should wrap ErrMappingNotFound")
	assert.Contains(t, err.Error(), MappingFileName)
}

func TestLoadMapping_MalformedFileIsNotMissingFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, MappingFileName), []byte("{invalid"), 0600))

	_, err := LoadMapping(dir)
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrMappingNotFound),
		"malformed file should not be reported as missing")
	assert.Contains(t, err.Error(), "parsing mapping file")
}

func TestLoadMapping_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, MappingFileName), []byte(""), 0600))

	_, err := LoadMapping(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing mapping file")
}

func TestLoadMapping_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, MappingFileName), []byte("{invalid"), 0600))

	_, err := LoadMapping(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing mapping file")
}

func TestLoadMapping_DuplicateID(t *testing.T) {
	dir := t.TempDir()
	data := `{
		"version": "1",
		"mappings": [
			{"id": "ns1", "requirement_id": "CIS-1"},
			{"id": "ns1", "requirement_id": "CIS-2"}
		]
	}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, MappingFileName), []byte(data), 0600))

	_, err := LoadMapping(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate id")
}

func TestLoadMapping_DuplicateRequirementID(t *testing.T) {
	dir := t.TempDir()
	data := `{
		"version": "1",
		"mappings": [
			{"id": "ns1", "requirement_id": "CIS-1"},
			{"id": "ns2", "requirement_id": "CIS-1"}
		]
	}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, MappingFileName), []byte(data), 0600))

	_, err := LoadMapping(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate requirement_id")
}

func TestLoadMapping_EmptyID(t *testing.T) {
	dir := t.TempDir()
	data := `{"version": "1", "mappings": [{"id": "", "requirement_id": "CIS-1"}]}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, MappingFileName), []byte(data), 0600))

	_, err := LoadMapping(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty id")
}

func TestLoadMapping_EmptyRequirementID(t *testing.T) {
	dir := t.TempDir()
	data := `{"version": "1", "mappings": [{"id": "ns1", "requirement_id": ""}]}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, MappingFileName), []byte(data), 0600))

	_, err := LoadMapping(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty requirement_id")
}

func TestMatchRequirements_AllMatched(t *testing.T) {
	mapping := &MappingFile{
		Version: "1",
		Mappings: []MappingEntry{
			{ID: "kubernetes.run_as_root", RequirementID: "CIS-1"},
			{ID: "kubernetes.resource_limits", RequirementID: "CIS-2"},
		},
	}
	configs := []provider.AssessmentConfiguration{
		{RequirementID: "CIS-1", PlanID: "ap-cis-1"},
		{RequirementID: "CIS-2", PlanID: "ap-cis-2"},
	}

	ids, reverseMap, warnings := MatchRequirements(configs, mapping)
	assert.Len(t, warnings, 0)
	assert.Equal(t, []string{"kubernetes.resource_limits", "kubernetes.run_as_root"}, ids)
	assert.Equal(t, "CIS-1", reverseMap["kubernetes.run_as_root"])
	assert.Equal(t, "CIS-2", reverseMap["kubernetes.resource_limits"])
}

func TestMatchRequirements_PartialMatch(t *testing.T) {
	mapping := &MappingFile{
		Version: "1",
		Mappings: []MappingEntry{
			{ID: "kubernetes.run_as_root", RequirementID: "CIS-1"},
		},
	}
	configs := []provider.AssessmentConfiguration{
		{RequirementID: "CIS-1", PlanID: "ap-cis-1"},
		{RequirementID: "CIS-MISSING", PlanID: "ap-cis-missing"},
	}

	ids, reverseMap, warnings := MatchRequirements(configs, mapping)
	assert.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "CIS-MISSING")
	assert.Equal(t, []string{"kubernetes.run_as_root"}, ids)
	assert.Equal(t, "CIS-1", reverseMap["kubernetes.run_as_root"])
}

func TestMatchRequirements_NoneMatched(t *testing.T) {
	mapping := &MappingFile{
		Version:  "1",
		Mappings: []MappingEntry{},
	}
	configs := []provider.AssessmentConfiguration{
		{RequirementID: "CIS-1", PlanID: "ap-cis-1"},
	}

	ids, _, warnings := MatchRequirements(configs, mapping)
	assert.Len(t, warnings, 1)
	assert.Empty(t, ids)
}

func TestMatchRequirements_DeduplicatesConfigs(t *testing.T) {
	mapping := &MappingFile{
		Version: "1",
		Mappings: []MappingEntry{
			{ID: "ns1", RequirementID: "CIS-1"},
		},
	}
	configs := []provider.AssessmentConfiguration{
		{RequirementID: "CIS-1", PlanID: "ap-cis-1"},
		{RequirementID: "CIS-1", PlanID: "ap-cis-1"},
	}

	ids, _, warnings := MatchRequirements(configs, mapping)
	assert.Len(t, warnings, 0)
	assert.Equal(t, []string{"ns1"}, ids)
}
