// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/complytime/complyctl/pkg/provider"
)

const (
	// PolicyFileName is the output filename for the merged AMPEL policy bundle.
	PolicyFileName = "complytime-ampel-policy.json"
)

// LoadGranularPolicies recursively reads all .json files from dir
// (skipping PolicyFileName and symbolic links) and returns a map
// keyed by each policy's ID field. Duplicate policy IDs across
// files produce an error.
func LoadGranularPolicies(dir string) (map[string]*AmpelPolicy, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("opening policy directory %q: %w", dir, err)
	}
	defer root.Close()

	policies := make(map[string]*AmpelPolicy)
	// idPaths tracks which file first defined each policy ID
	// so duplicate-ID errors can reference both paths.
	idPaths := make(map[string]string)

	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if skipEntry(d) {
			return nil
		}
		return loadPolicy(root, dir, path, policies, idPaths)
	})
	if walkErr != nil {
		return nil, walkErr
	}

	return policies, nil
}

// skipEntry returns true for directory entries, symbolic links,
// non-JSON files, and the merged policy bundle output file.
func skipEntry(d fs.DirEntry) bool {
	if d.IsDir() {
		return true
	}
	if d.Type()&fs.ModeSymlink != 0 {
		return true
	}
	if filepath.Ext(d.Name()) != ".json" {
		return true
	}
	return d.Name() == PolicyFileName
}

// loadPolicy reads a single policy JSON file through the root-scoped
// handle, parses it, checks for duplicates, and adds it to the map.
func loadPolicy(
	root *os.Root,
	dir, path string,
	policies map[string]*AmpelPolicy,
	idPaths map[string]string,
) error {
	relPath, relErr := filepath.Rel(dir, path)
	if relErr != nil {
		return fmt.Errorf("computing relative path for %q: %w", path, relErr)
	}

	data, readErr := readRootFile(root, relPath, path)
	if readErr != nil {
		return readErr
	}

	var p AmpelPolicy
	if unmarshalErr := json.Unmarshal(data, &p); unmarshalErr != nil {
		return fmt.Errorf("parsing policy file %q: %w", path, unmarshalErr)
	}
	if p.ID == "" {
		return fmt.Errorf("policy file %q has empty id field", path)
	}

	if existingPath, exists := idPaths[p.ID]; exists {
		return fmt.Errorf(
			"duplicate policy id %q found in %q and %q",
			p.ID, existingPath, path,
		)
	}
	idPaths[p.ID] = path
	policies[p.ID] = &p

	return nil
}

// readRootFile reads a file using the root-scoped API to prevent
// symlink TOCTOU traversal.
func readRootFile(root *os.Root, relPath, absPath string) ([]byte, error) {
	f, openErr := root.Open(relPath)
	if openErr != nil {
		return nil, fmt.Errorf("reading policy file %q: %w", absPath, openErr)
	}
	defer f.Close()

	data, readErr := io.ReadAll(f)
	if readErr != nil {
		return nil, fmt.Errorf("reading policy file %q: %w", absPath, readErr)
	}
	return data, nil
}

// MatchPolicies looks up each Gemara requirement ID (config.RequirementID) from
// the assessment configurations in the granular policy map. It returns the
// matched policies and warning strings for unmatched requirements.
func MatchPolicies(configs []provider.AssessmentConfiguration, granular map[string]*AmpelPolicy) ([]*AmpelPolicy, []string) {
	var matched []*AmpelPolicy
	var warnings []string
	seen := make(map[string]bool)

	for _, config := range configs {
		reqID := config.RequirementID
		if seen[reqID] {
			continue
		}
		seen[reqID] = true

		p, ok := granular[reqID]
		if !ok {
			warnings = append(warnings, fmt.Sprintf("no granular policy found for requirement %q", reqID))
			continue
		}
		matched = append(matched, p)
	}

	// Sort by policy ID for deterministic output.
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].ID < matched[j].ID
	})

	return matched, warnings
}

// MergeToBundle wraps matched policies into a top-level AmpelPolicyBundle.
func MergeToBundle(policies []*AmpelPolicy) *AmpelPolicyBundle {
	return &AmpelPolicyBundle{
		ID: "complytime-ampel-policy",
		Meta: BundleMeta{
			Frameworks: []Framework{
				{
					ID:   "ComplyTime-AMPEL-Policy",
					Name: "ComplyTime AMPEL Policy",
				},
			},
		},
		Policies: policies,
	}
}

// WritePolicy marshals an AmpelPolicyBundle to JSON and writes it to the given directory.
// If bundle is nil or has no policies, no file is written and nil is returned.
func WritePolicy(bundle *AmpelPolicyBundle, dir string) error {
	if bundle == nil || len(bundle.Policies) == 0 {
		return nil
	}

	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("creating policy directory %q: %w", dir, err)
	}

	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling AMPEL policy bundle: %w", err)
	}

	path := filepath.Join(dir, PolicyFileName)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing policy file: %w", err)
	}

	return nil
}
