// SPDX-License-Identifier: Apache-2.0

package generate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/complytime/complyctl/pkg/provider"
)

// ErrMappingNotFound indicates that the complytime-mapping.json file
// does not exist in the policy bundle directory.
var ErrMappingNotFound = errors.New("mapping file not found")

const (
	// MappingFileName is the name of the mapping file inside an OCI policy bundle.
	MappingFileName = "complytime-mapping.json"
)

// MappingFile represents the complytime-mapping.json file shipped in an OCI bundle.
type MappingFile struct {
	Version  string         `json:"version"`
	Mappings []MappingEntry `json:"mappings"`
}

// MappingEntry maps a Gemara requirement ID (from AssessmentPlan.RequirementId)
// to a Rego namespace. The ID field is the Rego package namespace and serves as
// the semantic, benchmark-agnostic identity (equivalent to AMPEL's granular
// policy id field). The RequirementID field holds the Gemara requirement-id
// (e.g., "CIS-K8S-5.2.6"), not the assessment plan's own id.
type MappingEntry struct {
	ID            string `json:"id"`
	RequirementID string `json:"requirement_id"`
}

// LoadMapping reads and validates a complytime-mapping.json file from the given
// bundle directory. Returns an error if the file is missing, malformed, or
// contains invalid entries.
func LoadMapping(bundleDir string) (*MappingFile, error) {
	path := filepath.Join(bundleDir, MappingFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s not found in %s", ErrMappingNotFound, MappingFileName, bundleDir)
		}
		return nil, fmt.Errorf("reading mapping file: %w", err)
	}

	var mapping MappingFile
	if err := json.Unmarshal(data, &mapping); err != nil {
		return nil, fmt.Errorf("parsing mapping file: %w", err)
	}

	if err := validateMapping(&mapping); err != nil {
		return nil, fmt.Errorf("validating mapping file: %w", err)
	}

	return &mapping, nil
}

// validateMapping checks that the mapping file has no empty or duplicate entries.
func validateMapping(m *MappingFile) error {
	seenIDs := make(map[string]bool, len(m.Mappings))
	seenReqIDs := make(map[string]bool, len(m.Mappings))
	for i, entry := range m.Mappings {
		if entry.ID == "" {
			return fmt.Errorf("mapping entry %d has empty id", i)
		}
		if entry.RequirementID == "" {
			return fmt.Errorf("mapping entry %d has empty requirement_id", i)
		}
		if seenIDs[entry.ID] {
			return fmt.Errorf("duplicate id %q in mapping", entry.ID)
		}
		if seenReqIDs[entry.RequirementID] {
			return fmt.Errorf("duplicate requirement_id %q in mapping", entry.RequirementID)
		}
		seenIDs[entry.ID] = true
		seenReqIDs[entry.RequirementID] = true
	}
	return nil
}

// MatchRequirements matches Gemara requirement IDs (cfg.RequirementID) against
// mapping entries using exact string equality (like AMPEL's MatchPolicies).
// Returns the matched ID list (Rego namespaces), a reverse mapping from Rego
// namespace to Gemara requirement ID for result-time resolution, and any
// warnings for unmatched requirements.
func MatchRequirements(
	configs []provider.AssessmentConfiguration,
	mapping *MappingFile,
) (ids []string, reverseMap map[string]string, warnings []string) {
	// Build lookup map: requirement_id -> id (Rego namespace)
	lookup := make(map[string]string, len(mapping.Mappings))
	for _, entry := range mapping.Mappings {
		lookup[entry.RequirementID] = entry.ID
	}

	seen := make(map[string]bool)
	reverseMap = make(map[string]string)

	for _, cfg := range configs {
		reqID := cfg.RequirementID
		if seen[reqID] {
			continue
		}
		seen[reqID] = true

		id, ok := lookup[reqID]
		if !ok {
			warnings = append(warnings,
				fmt.Sprintf("no mapping found for requirement %q", reqID))
			continue
		}
		ids = append(ids, id)
		reverseMap[id] = reqID
	}

	sort.Strings(ids)
	return ids, reverseMap, warnings
}
