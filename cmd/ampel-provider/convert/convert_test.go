package convert

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"
)

func loadConfigurations(t *testing.T, path string) []provider.AssessmentConfiguration {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err, "reading fixture %s", path)
	var configs []provider.AssessmentConfiguration
	require.NoError(t, json.Unmarshal(data, &configs), "unmarshaling fixture %s", path)
	return configs
}

func loadExpectedBundle(t *testing.T, path string) *AmpelPolicyBundle {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err, "reading expected fixture %s", path)
	var expected AmpelPolicyBundle
	require.NoError(t, json.Unmarshal(data, &expected), "unmarshaling expected fixture %s", path)
	return &expected
}

// --- LoadGranularPolicies tests ---

func TestLoadGranularPolicies(t *testing.T) {
	policies, err := LoadGranularPolicies("testdata/policies")
	require.NoError(t, err)
	require.Len(t, policies, 5)

	expectedIDs := []string{
		"block-force-push",
		"minimum-approvals",
		"prevent-admin-bypass",
		"require-code-owner-review",
		"require-pull-request",
	}
	for _, id := range expectedIDs {
		p, ok := policies[id]
		require.True(t, ok, "expected policy %q to be loaded", id)
		require.Equal(t, id, p.ID)
		require.NotEmpty(t, p.Tenets, "policy %q should have tenets", id)
	}
}

func TestLoadGranularPolicies_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	policies, err := LoadGranularPolicies(dir)
	require.NoError(t, err)
	require.Empty(t, policies)
}

func TestLoadGranularPolicies_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{invalid"), 0600))

	_, err := LoadGranularPolicies(dir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parsing policy file")
}

func TestLoadGranularPolicies_EmptyPolicyID(t *testing.T) {
	dir := t.TempDir()
	data := `{"id": "", "meta": {"description": "test"}, "tenets": []}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "empty-id.json"), []byte(data), 0600))

	_, err := LoadGranularPolicies(dir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty id field")
}

func TestLoadGranularPolicies_SkipsOutputFile(t *testing.T) {
	dir := t.TempDir()

	// Write a valid granular policy
	p := AmpelPolicy{ID: "test-01", Meta: PolicyMeta{Description: "test"}, Tenets: []AmpelTenet{}}
	data, err := json.Marshal(p)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test-01.json"), data, 0600))

	// Write the output file (should be skipped)
	bundle := AmpelPolicyBundle{ID: "complytime-ampel-policy"}
	bdata, err := json.Marshal(bundle)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, PolicyFileName), bdata, 0600))

	policies, err := LoadGranularPolicies(dir)
	require.NoError(t, err)
	require.Len(t, policies, 1)
	require.Contains(t, policies, "test-01")
}

func TestLoadGranularPolicies_SkipsNonJSON(t *testing.T) {
	dir := t.TempDir()

	// Write a valid granular policy
	p := AmpelPolicy{ID: "test-01", Meta: PolicyMeta{Description: "test"}, Tenets: []AmpelTenet{}}
	data, err := json.Marshal(p)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test-01.json"), data, 0600))

	// Write a non-JSON file (should be skipped)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello"), 0600))

	policies, err := LoadGranularPolicies(dir)
	require.NoError(t, err)
	require.Len(t, policies, 1)
}

func TestLoadGranularPolicies_Subdirectory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir")
	require.NoError(t, os.MkdirAll(sub, 0o750))

	p := AmpelPolicy{
		ID:     "sub-policy-01",
		Meta:   PolicyMeta{Description: "in subdir"},
		Tenets: []AmpelTenet{{ID: "01", Code: "true"}},
	}
	data, err := json.Marshal(p)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		filepath.Join(sub, "sub-policy-01.json"), data, 0o600,
	))

	policies, err := LoadGranularPolicies(dir)
	require.NoError(t, err)
	require.Len(t, policies, 1)
	assert.Contains(t, policies, "sub-policy-01")
}

func TestLoadGranularPolicies_MixedFlatAndSubdirectory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "nested")
	require.NoError(t, os.MkdirAll(sub, 0o750))

	// Policy at root level.
	rootPolicy := AmpelPolicy{
		ID:     "root-policy",
		Meta:   PolicyMeta{Description: "at root"},
		Tenets: []AmpelTenet{{ID: "01", Code: "true"}},
	}
	rootData, err := json.Marshal(rootPolicy)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "root-policy.json"), rootData, 0o600,
	))

	// Policy in subdirectory.
	subPolicy := AmpelPolicy{
		ID:     "nested-policy",
		Meta:   PolicyMeta{Description: "in nested"},
		Tenets: []AmpelTenet{{ID: "01", Code: "true"}},
	}
	subData, err := json.Marshal(subPolicy)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		filepath.Join(sub, "nested-policy.json"), subData, 0o600,
	))

	policies, err := LoadGranularPolicies(dir)
	require.NoError(t, err)
	require.Len(t, policies, 2)
	assert.Contains(t, policies, "root-policy")
	assert.Contains(t, policies, "nested-policy")
}

func TestLoadGranularPolicies_PolicyFileNameSkipInSubdirectory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir")
	require.NoError(t, os.MkdirAll(sub, 0o750))

	// Place PolicyFileName in subdir — should be skipped.
	bundle := AmpelPolicyBundle{ID: "should-skip"}
	bData, err := json.Marshal(bundle)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		filepath.Join(sub, PolicyFileName), bData, 0o600,
	))

	// Place a valid policy alongside it.
	p := AmpelPolicy{
		ID:     "valid-policy",
		Meta:   PolicyMeta{Description: "valid"},
		Tenets: []AmpelTenet{{ID: "01", Code: "true"}},
	}
	pData, err := json.Marshal(p)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		filepath.Join(sub, "valid-policy.json"), pData, 0o600,
	))

	policies, err := LoadGranularPolicies(dir)
	require.NoError(t, err)
	require.Len(t, policies, 1)
	assert.Contains(t, policies, "valid-policy")
	assert.NotContains(t, policies, "should-skip")
}

func TestLoadGranularPolicies_NestedSubdirectories(t *testing.T) {
	dir := t.TempDir()
	level1 := filepath.Join(dir, "level1")
	level2 := filepath.Join(level1, "level2")
	require.NoError(t, os.MkdirAll(level2, 0o750))

	// Policy at level 1.
	p1 := AmpelPolicy{
		ID:     "level1-policy",
		Meta:   PolicyMeta{Description: "depth 1"},
		Tenets: []AmpelTenet{{ID: "01", Code: "true"}},
	}
	d1, err := json.Marshal(p1)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		filepath.Join(level1, "level1-policy.json"), d1, 0o600,
	))

	// Policy at level 2.
	p2 := AmpelPolicy{
		ID:     "level2-policy",
		Meta:   PolicyMeta{Description: "depth 2"},
		Tenets: []AmpelTenet{{ID: "01", Code: "true"}},
	}
	d2, err := json.Marshal(p2)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		filepath.Join(level2, "level2-policy.json"), d2, 0o600,
	))

	policies, err := LoadGranularPolicies(dir)
	require.NoError(t, err)
	require.Len(t, policies, 2)
	assert.Contains(t, policies, "level1-policy")
	assert.Contains(t, policies, "level2-policy")
}

func TestLoadGranularPolicies_NonJSONInSubdirectory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir")
	require.NoError(t, os.MkdirAll(sub, 0o750))

	// Valid JSON policy in subdir.
	p := AmpelPolicy{
		ID:     "json-policy",
		Meta:   PolicyMeta{Description: "valid"},
		Tenets: []AmpelTenet{{ID: "01", Code: "true"}},
	}
	pData, err := json.Marshal(p)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		filepath.Join(sub, "json-policy.json"), pData, 0o600,
	))

	// Non-JSON files in subdir — should be skipped.
	require.NoError(t, os.WriteFile(
		filepath.Join(sub, "readme.txt"), []byte("hello"), 0o600,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(sub, "notes.md"), []byte("# Notes"), 0o600,
	))

	policies, err := LoadGranularPolicies(dir)
	require.NoError(t, err)
	require.Len(t, policies, 1)
	assert.Contains(t, policies, "json-policy")
}

func TestLoadGranularPolicies_MalformedJSONInSubdirectory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir")
	require.NoError(t, os.MkdirAll(sub, 0o750))

	require.NoError(t, os.WriteFile(
		filepath.Join(sub, "bad.json"), []byte("{invalid json"), 0o600,
	))

	_, err := LoadGranularPolicies(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing policy file")
}

func TestLoadGranularPolicies_EmptyIDInSubdirectory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir")
	require.NoError(t, os.MkdirAll(sub, 0o750))

	data := `{"id": "", "meta": {"description": "empty"}, "tenets": []}`
	require.NoError(t, os.WriteFile(
		filepath.Join(sub, "empty-id.json"), []byte(data), 0o600,
	))

	_, err := LoadGranularPolicies(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty id field")
}

func TestLoadGranularPolicies_UnreadableFileInSubdirectory(t *testing.T) {
	// Skip when running as root because root can read any file.
	if os.Getuid() == 0 {
		t.Skip("skipping: test requires non-root user")
	}

	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir")
	require.NoError(t, os.MkdirAll(sub, 0o750))

	unreadable := filepath.Join(sub, "secret.json")
	require.NoError(t, os.WriteFile(unreadable, []byte(`{}`), 0o600))
	require.NoError(t, os.Chmod(unreadable, 0o000))

	_, err := LoadGranularPolicies(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading policy file")
}

func TestLoadGranularPolicies_DuplicatePolicyIDs(t *testing.T) {
	dir := t.TempDir()
	sub1 := filepath.Join(dir, "dir-a")
	sub2 := filepath.Join(dir, "dir-b")
	require.NoError(t, os.MkdirAll(sub1, 0o750))
	require.NoError(t, os.MkdirAll(sub2, 0o750))

	// Same ID in two different subdirectories.
	p := AmpelPolicy{
		ID:     "duplicate-id",
		Meta:   PolicyMeta{Description: "first"},
		Tenets: []AmpelTenet{{ID: "01", Code: "true"}},
	}
	pData, err := json.Marshal(p)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		filepath.Join(sub1, "dup.json"), pData, 0o600,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(sub2, "dup.json"), pData, 0o600,
	))

	_, err = LoadGranularPolicies(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate policy id")
	assert.Contains(t, err.Error(), "duplicate-id")
	// Error should mention both paths.
	assert.Contains(t, err.Error(), "dir-a")
	assert.Contains(t, err.Error(), "dir-b")
}

func TestLoadGranularPolicies_SymlinkToDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping: symlinks may not be supported on Windows")
	}

	dir := t.TempDir()

	// Target directory with a policy that should NOT be reached.
	target := filepath.Join(dir, "target")
	require.NoError(t, os.MkdirAll(target, 0o750))
	p := AmpelPolicy{
		ID:     "symlinked-policy",
		Meta:   PolicyMeta{Description: "behind symlink"},
		Tenets: []AmpelTenet{{ID: "01", Code: "true"}},
	}
	pData, err := json.Marshal(p)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		filepath.Join(target, "symlinked.json"), pData, 0o600,
	))

	// Create a symlink inside the walk root pointing to target.
	walkRoot := filepath.Join(dir, "root")
	require.NoError(t, os.MkdirAll(walkRoot, 0o750))
	linkPath := filepath.Join(walkRoot, "linked-dir")
	err = os.Symlink(target, linkPath)
	if err != nil {
		t.Skipf("skipping: cannot create symlink: %v", err)
	}

	policies, err := LoadGranularPolicies(walkRoot)
	require.NoError(t, err)
	// The symlink directory is not followed, so no policies loaded.
	assert.Empty(t, policies)
}

func TestLoadGranularPolicies_EmptySubdirectory(t *testing.T) {
	dir := t.TempDir()

	// Empty subdirectory.
	require.NoError(t, os.MkdirAll(
		filepath.Join(dir, "empty-sub"), 0o750,
	))

	// Populated subdirectory.
	populated := filepath.Join(dir, "populated-sub")
	require.NoError(t, os.MkdirAll(populated, 0o750))
	p := AmpelPolicy{
		ID:     "populated-policy",
		Meta:   PolicyMeta{Description: "has content"},
		Tenets: []AmpelTenet{{ID: "01", Code: "true"}},
	}
	pData, err := json.Marshal(p)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		filepath.Join(populated, "populated-policy.json"), pData, 0o600,
	))

	policies, err := LoadGranularPolicies(dir)
	require.NoError(t, err)
	require.Len(t, policies, 1)
	assert.Contains(t, policies, "populated-policy")
}

func TestLoadGranularPolicies_DuplicateIDsSameDirectory(t *testing.T) {
	dir := t.TempDir()

	p := AmpelPolicy{
		ID:     "same-id",
		Meta:   PolicyMeta{Description: "first"},
		Tenets: []AmpelTenet{{ID: "01", Code: "true"}},
	}
	data, err := json.Marshal(p)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "first.json"), data, 0o600,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "second.json"), data, 0o600,
	))

	_, err = LoadGranularPolicies(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate policy id")
	assert.Contains(t, err.Error(), "same-id")
}

func TestLoadGranularPolicies_SymlinkToFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping symlink test on Windows")
	}
	dir := t.TempDir()

	// Create a real policy file outside the walk root.
	outside := filepath.Join(dir, "outside")
	require.NoError(t, os.MkdirAll(outside, 0o750))
	p := AmpelPolicy{
		ID:     "linked-policy",
		Meta:   PolicyMeta{Description: "symlinked file"},
		Tenets: []AmpelTenet{{ID: "01", Code: "true"}},
	}
	data, err := json.Marshal(p)
	require.NoError(t, err)
	realFile := filepath.Join(outside, "real.json")
	require.NoError(t, os.WriteFile(realFile, data, 0o600))

	// Create the walk root with a symlink to the file.
	walkRoot := filepath.Join(dir, "root")
	require.NoError(t, os.MkdirAll(walkRoot, 0o750))
	linkPath := filepath.Join(walkRoot, "linked.json")
	err = os.Symlink(realFile, linkPath)
	if err != nil {
		t.Skipf("skipping: cannot create symlink: %v", err)
	}

	policies, err := LoadGranularPolicies(walkRoot)
	require.NoError(t, err)
	// The symlinked file should be skipped.
	assert.Empty(t, policies)
}

// --- MatchPolicies tests ---

func TestMatchPolicies(t *testing.T) {
	granular, err := LoadGranularPolicies("testdata/policies")
	require.NoError(t, err)

	input := loadConfigurations(t, "testdata/assessment-plan-full.json")
	matched, warnings := MatchPolicies(input, granular)

	require.Len(t, matched, 5)
	require.Empty(t, warnings)

	// Verify sorted order
	for i := 1; i < len(matched); i++ {
		require.True(t, matched[i-1].ID < matched[i].ID,
			"expected sorted order, got %s before %s", matched[i-1].ID, matched[i].ID)
	}
}

func TestMatchPolicies_Subset(t *testing.T) {
	granular, err := LoadGranularPolicies("testdata/policies")
	require.NoError(t, err)

	input := loadConfigurations(t, "testdata/assessment-plan-subset.json")
	matched, warnings := MatchPolicies(input, granular)

	require.Len(t, matched, 2)
	require.Empty(t, warnings)
	require.Equal(t, "block-force-push", matched[0].ID)
	require.Equal(t, "require-pull-request", matched[1].ID)
}

func TestMatchPolicies_UnmatchedRule(t *testing.T) {
	granular, err := LoadGranularPolicies("testdata/policies")
	require.NoError(t, err)

	input := []provider.AssessmentConfiguration{
		{PlanID: "ap-require-pull-request", RequirementID: "require-pull-request"},
		{PlanID: "ap-nonexistent-rule", RequirementID: "nonexistent-rule"},
	}

	matched, warnings := MatchPolicies(input, granular)
	require.Len(t, matched, 1)
	require.Equal(t, "require-pull-request", matched[0].ID)
	require.Len(t, warnings, 1)
	require.Contains(t, warnings[0], "nonexistent-rule")
}

func TestMatchPolicies_AllUnmatched(t *testing.T) {
	granular, err := LoadGranularPolicies("testdata/policies")
	require.NoError(t, err)

	input := []provider.AssessmentConfiguration{
		{PlanID: "ap-no-such-rule-1", RequirementID: "no-such-rule-1"},
		{PlanID: "ap-no-such-rule-2", RequirementID: "no-such-rule-2"},
	}

	matched, warnings := MatchPolicies(input, granular)
	require.Empty(t, matched)
	require.Len(t, warnings, 2)
}

func TestMatchPolicies_EmptyInput(t *testing.T) {
	granular, err := LoadGranularPolicies("testdata/policies")
	require.NoError(t, err)

	matched, warnings := MatchPolicies([]provider.AssessmentConfiguration{}, granular)
	require.Empty(t, matched)
	require.Empty(t, warnings)
}

func TestMatchPolicies_DuplicateRequirements(t *testing.T) {
	granular, err := LoadGranularPolicies("testdata/policies")
	require.NoError(t, err)

	input := []provider.AssessmentConfiguration{
		{PlanID: "ap-require-pull-request", RequirementID: "require-pull-request"},
		{PlanID: "ap-require-pull-request", RequirementID: "require-pull-request"},
	}

	matched, warnings := MatchPolicies(input, granular)
	require.Len(t, matched, 1, "duplicate requirements should be deduplicated")
	require.Empty(t, warnings)
}

// --- MergeToBundle tests ---

func TestMergeToBundle(t *testing.T) {
	policies := []*AmpelPolicy{
		{ID: "block-force-push", Meta: PolicyMeta{Description: "Force push"}, Tenets: []AmpelTenet{{ID: "01"}}},
		{ID: "require-pull-request", Meta: PolicyMeta{Description: "PR required"}, Tenets: []AmpelTenet{{ID: "01"}}},
	}

	bundle := MergeToBundle(policies)
	require.Equal(t, "complytime-ampel-policy", bundle.ID)
	require.Len(t, bundle.Meta.Frameworks, 1)
	require.Equal(t, "ComplyTime-AMPEL-Policy", bundle.Meta.Frameworks[0].ID)
	require.Len(t, bundle.Policies, 2)
	require.Equal(t, "block-force-push", bundle.Policies[0].ID)
	require.Equal(t, "require-pull-request", bundle.Policies[1].ID)
}

func TestMergeToBundle_Empty(t *testing.T) {
	bundle := MergeToBundle(nil)
	require.Equal(t, "complytime-ampel-policy", bundle.ID)
	require.Empty(t, bundle.Policies)
}

// --- End-to-end: load, match, merge, compare to expected ---

func TestEndToEnd_FullPlan(t *testing.T) {
	granular, err := LoadGranularPolicies("testdata/policies")
	require.NoError(t, err)

	input := loadConfigurations(t, "testdata/assessment-plan-full.json")
	matched, warnings := MatchPolicies(input, granular)
	require.Empty(t, warnings)
	require.Len(t, matched, 5)

	bundle := MergeToBundle(matched)
	expected := loadExpectedBundle(t, "testdata/ampel-bundle-expected-full.json")

	require.Equal(t, expected.ID, bundle.ID)
	require.Equal(t, expected.Meta, bundle.Meta)
	require.Len(t, bundle.Policies, len(expected.Policies))
	for i, ep := range expected.Policies {
		require.Equal(t, ep.ID, bundle.Policies[i].ID, "policy %d ID mismatch", i)
		require.Equal(t, ep.Meta, bundle.Policies[i].Meta, "policy %d meta mismatch", i)
		require.Equal(t, len(ep.Tenets), len(bundle.Policies[i].Tenets), "policy %d tenet count mismatch", i)
		for j, et := range ep.Tenets {
			require.Equal(t, et.ID, bundle.Policies[i].Tenets[j].ID, "policy %d tenet %d ID mismatch", i, j)
			require.Equal(t, et.Code, bundle.Policies[i].Tenets[j].Code, "policy %d tenet %d Code mismatch", i, j)
		}
	}
}

func TestEndToEnd_SubsetPlan(t *testing.T) {
	granular, err := LoadGranularPolicies("testdata/policies")
	require.NoError(t, err)

	input := loadConfigurations(t, "testdata/assessment-plan-subset.json")
	matched, warnings := MatchPolicies(input, granular)
	require.Empty(t, warnings)
	require.Len(t, matched, 2)

	bundle := MergeToBundle(matched)
	expected := loadExpectedBundle(t, "testdata/ampel-bundle-expected-subset.json")

	require.Equal(t, expected.ID, bundle.ID)
	require.Len(t, bundle.Policies, len(expected.Policies))
	for i, ep := range expected.Policies {
		require.Equal(t, ep.ID, bundle.Policies[i].ID, "policy %d ID mismatch", i)
	}
}

// --- WritePolicy tests ---

func TestWritePolicy(t *testing.T) {
	t.Run("writes bundle file", func(t *testing.T) {
		dir := t.TempDir()
		bundle := &AmpelPolicyBundle{
			ID: "test-bundle",
			Meta: BundleMeta{
				Frameworks: []Framework{{ID: "test", Name: "Test"}},
			},
			Policies: []*AmpelPolicy{
				{
					ID:   "policy-1",
					Meta: PolicyMeta{Description: "Test policy"},
					Tenets: []AmpelTenet{
						{ID: "01", Code: "true", Predicates: PredicateSpec{Types: []string{"type"}}},
					},
				},
			},
		}
		err := WritePolicy(bundle, dir)
		require.NoError(t, err)

		path := filepath.Join(dir, PolicyFileName)
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Contains(t, string(data), "test-bundle")
		require.Contains(t, string(data), "policy-1")
	})

	t.Run("nil bundle writes nothing", func(t *testing.T) {
		dir := t.TempDir()
		err := WritePolicy(nil, dir)
		require.NoError(t, err)

		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Empty(t, entries)
	})

	t.Run("empty policies writes nothing", func(t *testing.T) {
		dir := t.TempDir()
		bundle := &AmpelPolicyBundle{ID: "empty", Policies: nil}
		err := WritePolicy(bundle, dir)
		require.NoError(t, err)

		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Empty(t, entries)
	})

	t.Run("creates directory if missing", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "subdir", "nested")
		bundle := &AmpelPolicyBundle{
			ID: "test",
			Policies: []*AmpelPolicy{
				{ID: "p1", Tenets: []AmpelTenet{{ID: "01", Code: "true", Predicates: PredicateSpec{Types: []string{"type"}}}}},
			},
		}
		err := WritePolicy(bundle, dir)
		require.NoError(t, err)

		_, err = os.Stat(filepath.Join(dir, PolicyFileName))
		require.NoError(t, err)
	})

	t.Run("overwrites existing file", func(t *testing.T) {
		dir := t.TempDir()
		b1 := &AmpelPolicyBundle{
			ID: "first",
			Policies: []*AmpelPolicy{
				{ID: "p1", Tenets: []AmpelTenet{{ID: "01", Code: "v1", Predicates: PredicateSpec{Types: []string{"type"}}}}},
			},
		}
		b2 := &AmpelPolicyBundle{
			ID: "second",
			Policies: []*AmpelPolicy{
				{ID: "p2", Tenets: []AmpelTenet{{ID: "01", Code: "v2", Predicates: PredicateSpec{Types: []string{"type"}}}}},
			},
		}

		require.NoError(t, WritePolicy(b1, dir))
		require.NoError(t, WritePolicy(b2, dir))

		data, err := os.ReadFile(filepath.Join(dir, PolicyFileName))
		require.NoError(t, err)
		require.Contains(t, string(data), "second")
		require.NotContains(t, string(data), "first")
	})
}
