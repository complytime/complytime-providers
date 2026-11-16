// SPDX-License-Identifier: Apache-2.0

package evidence

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func storageOptions(t *testing.T) StorageOptions {
	t.Helper()
	root := t.TempDir()
	identity, err := ExecutionIdentity()
	require.NoError(t, err)
	return StorageOptions{WorkspaceRoot: root, ResultsRoot: filepath.Join(root, "results"),
		Identity: identity, Clock: func() time.Time { return time.Date(2026, 11, 10, 12, 1, 2, 0, time.UTC) }}
}

func allocateTest(t *testing.T, options StorageOptions, labels ...string) *Evaluation {
	t.Helper()
	evaluation, err := Allocate(options, labels...)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, evaluation.Close()) })
	return evaluation
}

func TestLabel_BoundedSafeComponents(t *testing.T) {
	for _, input := range []string{"", "a", strings.Repeat("a", 79), strings.Repeat("a", 80),
		strings.Repeat("a", 81), ".", "..", "/absolute/path", "a/../../b", "\x00\n\r", "release/1", "_", "é"} {
		t.Run(fmt.Sprintf("%q", input), func(t *testing.T) {
			label := Label(input)
			assert.Regexp(t, `^[A-Za-z0-9_-]{1,80}$`, label)
			if len(input) <= LabelLimit && regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(input) {
				assert.Equal(t, input, label)
			}
		})
	}
	assert.Equal(t, "unknown", Label(""))
	assert.Equal(t, strings.Repeat("a", LabelLimit), Label(strings.Repeat("a", 81)))
	assert.Equal(t, Label("release/1"), Label("release.1"))
	label := Label("https://user:password@example.com/repo?secret=token#private")
	assert.NotContains(t, label, "password")
	assert.NotContains(t, label, "token")
	assert.NotContains(t, label, "private")
}

func TestAllocate_ExclusiveCollisionsAndRollback(t *testing.T) {
	options := storageOptions(t)
	options.Random = bytes.NewReader(bytes.Repeat([]byte{1}, tokenBytes))
	first, err := Allocate(options, "repo", "release/1")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, first.Close()) })
	assert.Equal(t, strings.Repeat("01", tokenBytes), first.Token)
	assert.Equal(t, filepath.Join(options.ResultsRoot, "repo", "release-1",
		"evaluation-20261110T120102Z-"+first.Token), first.Directory)
	assert.Equal(t, options.Clock(), first.CollectionTime)
	info, err := os.Stat(first.Directory)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	assert.Equal(t, os.FileMode(DirectoryMode), info.Mode().Perm())
	entries, err := os.ReadDir(first.Directory)
	require.NoError(t, err)
	assert.Empty(t, entries)
	require.NoError(t, first.Write(ConftestFile, []byte("original")))
	// Force the first candidate to collide, then offer a different token.
	options.Random = bytes.NewReader(append(bytes.Repeat([]byte{1}, tokenBytes), bytes.Repeat([]byte{2}, tokenBytes)...))
	second := allocateTest(t, options, "repo", "release/1")
	assert.NotEqual(t, first.Directory, second.Directory)
	assert.Equal(t, strings.Repeat("02", tokenBytes), second.Token)
	original, err := os.ReadFile(first.Path(ConftestFile))
	require.NoError(t, err)
	assert.Equal(t, "original", string(original))
	options.Attempts = 2
	options.Random = bytes.NewReader(bytes.Repeat([]byte{1}, tokenBytes*2))
	failed, err := Allocate(options, "repo", "release/1")
	require.ErrorContains(t, err, "exhausted 2")
	assert.Nil(t, failed)
	options.Clock = func() time.Time { return first.CollectionTime.Add(-time.Hour) }
	options.Random = bytes.NewReader(bytes.Repeat([]byte{3}, tokenBytes))
	third := allocateTest(t, options, "repo", "release/1")
	assert.NotEqual(t, first.Directory, third.Directory)
	assert.True(t, third.CollectionTime.Before(first.CollectionTime))
}

func TestAllocate_ConcurrentFixedClock(t *testing.T) {
	options := storageOptions(t)
	const count = 24
	results := make(chan *Evaluation, count)
	errors := make(chan error, count)
	var workers sync.WaitGroup
	for range count {
		workers.Go(func() {
			evaluation, err := Allocate(options, "same")
			if err != nil {
				errors <- err
				return
			}
			results <- evaluation
		})
	}
	workers.Wait()
	close(results)
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	seen := make(map[string]bool)
	for evaluation := range results {
		assert.False(t, seen[evaluation.Directory])
		seen[evaluation.Directory] = true
		assert.Len(t, evaluation.Token, tokenBytes*2)
		require.NoError(t, evaluation.Close())
	}
	assert.Len(t, seen, count)
}

func TestAllocate_RejectsInvalidRootsAndSymlinkParents(t *testing.T) {
	options := storageOptions(t)
	outside := t.TempDir()
	require.NoError(t, os.Symlink(outside, options.ResultsRoot))
	_, err := Allocate(options, "repo")
	require.Error(t, err)
	entries, err := os.ReadDir(outside)
	require.NoError(t, err)
	assert.Empty(t, entries)
	for _, results := range []string{"relative", options.WorkspaceRoot, outside} {
		options.ResultsRoot = results
		_, err := Allocate(options, "repo")
		require.Error(t, err)
	}
}

func TestAllocate_OperationalFailuresAndUnsafeLabels(t *testing.T) {
	options := storageOptions(t)
	options.Random = bytes.NewReader(nil)
	_, err := Allocate(options)
	require.ErrorIs(t, err, io.EOF)
	options.Random = nil
	options.ResultsRoot = filepath.Join(options.WorkspaceRoot, "file", "results")
	require.NoError(t, os.WriteFile(filepath.Join(options.WorkspaceRoot, "file"), nil, FileMode))
	_, err = Allocate(options)
	require.ErrorContains(t, err, "evaluation context")
	options.ResultsRoot = filepath.Join(options.WorkspaceRoot, "results")
	for _, label := range []string{".", "..", "/absolute", "a/../../b", "\x00\r\n", ""} {
		evaluation := allocateTest(t, options, label)
		relative, err := filepath.Rel(options.ResultsRoot, evaluation.Directory)
		require.NoError(t, err)
		assert.True(t, filepath.IsLocal(relative))
		assert.Len(t, strings.Split(relative, string(filepath.Separator)), 2)
	}
}

func TestEnsureDirectories_RejectsSymlinksBeforeExternalWrites(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	err := EnsureDirectories(root, "provider/generated", "provider/results")
	require.NoError(t, err)
	require.DirExists(t, filepath.Join(root, "provider", "generated"))
	require.DirExists(t, filepath.Join(root, "provider", "results"))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "redirect")))
	err = EnsureDirectories(root, "redirect/generated")
	require.ErrorContains(t, err, "opening evaluation context")
	entries, err := os.ReadDir(outside)
	require.NoError(t, err)
	require.Empty(t, entries)
	for _, path := range []string{"../outside", outside} {
		err = EnsureDirectories(root, path)
		require.ErrorContains(t, err, "workspace-relative")
	}
}

func TestEnsureDirectories_PreservesExistingParentsAndContents(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "provider")
	require.NoError(t, os.Mkdir(parent, DirectoryMode))
	artifact := filepath.Join(parent, "existing.json")
	require.NoError(t, os.WriteFile(artifact, []byte("retained"), FileMode))
	before, err := os.Stat(parent)
	require.NoError(t, err)
	err = EnsureDirectories(root, "provider/generated", "provider/results")
	require.NoError(t, err)
	err = EnsureDirectories(root, "provider/generated", "provider/results")
	require.NoError(t, err)
	after, err := os.Stat(parent)
	require.NoError(t, err)
	assert.True(t, os.SameFile(before, after))
	assert.Equal(t, before.Mode(), after.Mode())
	data, err := os.ReadFile(artifact)
	require.NoError(t, err)
	assert.Equal(t, []byte("retained"), data)
	for _, name := range []string{"generated", "results"} {
		info, err := os.Stat(filepath.Join(parent, name))
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	}
	// Validate every requested path before creating any of them.
	err = EnsureDirectories(root, "uncreated", "../outside")
	require.ErrorContains(t, err, "workspace-relative")
	assert.NoDirExists(t, filepath.Join(root, "uncreated"))
	err = EnsureDirectories("relative", "provider")
	require.ErrorContains(t, err, "root must be absolute")
}

func TestEvaluation_ReadWriteAndPrivateFinalization(t *testing.T) {
	options := storageOptions(t)
	evaluation := allocateTest(t, options, "target")
	data := []byte("{\"result\": \"fail\"}\n")
	require.NoError(t, evaluation.Write(ConftestFile, data))
	require.Error(t, evaluation.Write(ConftestFile, []byte("replacement")))
	artifact, err := evaluation.Read(ConftestFile)
	require.NoError(t, err)
	assert.Equal(t, data, artifact.Data)
	assert.Equal(t, fmt.Sprintf("sha256:%x", sha256.Sum256(data)), artifact.Digest)
	assert.Equal(t, artifact.Path, filepath.Join(options.WorkspaceRoot, artifact.Coordinate))
	require.NoError(t, evaluation.Finalize())
	info, err := os.Stat(evaluation.Directory)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(DirectoryMode), info.Mode().Perm())
	info, err = os.Stat(artifact.Path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(FileMode), info.Mode().Perm())
	for _, name := range []string{"", ".", "..", "../outside", "/absolute"} {
		assert.Error(t, evaluation.Write(name, data))
		_, err := evaluation.Read(name)
		assert.Error(t, err)
	}
}

func TestEvaluation_RejectsSymlinkAndNonRegularOutputs(t *testing.T) {
	for _, kind := range []string{"symlink", "directory", "fifo"} {
		t.Run(kind, func(t *testing.T) {
			evaluation := allocateTest(t, storageOptions(t))
			outside := filepath.Join(t.TempDir(), "outside")
			require.NoError(t, os.WriteFile(outside, []byte("untouched"), FileMode))
			require.NoError(t, os.Chmod(outside, 0o644))
			if kind == "symlink" {
				require.NoError(t, os.Symlink(outside, evaluation.Path(ConftestFile)))
			}
			if kind == "directory" {
				require.NoError(t, os.Mkdir(evaluation.Path(ConftestFile), 0o755))
			}
			if kind == "fifo" {
				require.NoError(t, makeFIFO(evaluation.Path(ConftestFile)))
			}
			_, err := evaluation.Read(ConftestFile)
			require.Error(t, err)
			require.Error(t, evaluation.Finalize())
			data, err := os.ReadFile(outside)
			require.NoError(t, err)
			assert.Equal(t, "untouched", string(data))
			info, err := os.Stat(outside)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
		})
	}
}

func TestEvaluation_RejectsChangesAfterValidation(t *testing.T) {
	for _, change := range []string{"edit", "remove", "substitute", "parent-symlink"} {
		t.Run(change, func(t *testing.T) {
			evaluation := allocateTest(t, storageOptions(t), "target")
			require.NoError(t, evaluation.Write(ConftestFile, []byte("original")))
			_, err := evaluation.Read(ConftestFile)
			require.NoError(t, err)
			switch change {
			case "edit":
				require.NoError(t, os.WriteFile(evaluation.Path(ConftestFile), []byte("changed"), FileMode))
			case "remove":
				require.NoError(t, os.Remove(evaluation.Path(ConftestFile)))
			case "substitute":
				// Allocate the replacement while the original inode still exists;
				// unlink/recreate alone can recycle an inode on some filesystems.
				replacement := filepath.Join(filepath.Dir(evaluation.Directory), "replacement")
				require.NoError(t, os.WriteFile(replacement, []byte("original"), FileMode))
				originalInfo, err := os.Stat(evaluation.Path(ConftestFile))
				require.NoError(t, err)
				replacementInfo, err := os.Stat(replacement)
				require.NoError(t, err)
				require.False(t, os.SameFile(originalInfo, replacementInfo))
				require.NoError(t, os.Rename(replacement, evaluation.Path(ConftestFile)))
			case "parent-symlink":
				parent := filepath.Dir(evaluation.Directory)
				require.NoError(t, os.Rename(parent, parent+"-moved"))
				require.NoError(t, os.Symlink(parent+"-moved", parent))
			}
			require.Error(t, evaluation.Finalize())
		})
	}
}

func TestEvaluation_RejectsUnexpectedOwner(t *testing.T) {
	options := storageOptions(t)
	options.Identity.UID++
	_, err := Allocate(options)
	require.ErrorContains(t, err, "ownership")
}

func TestEvaluation_MetadataAndInstanceReferences(t *testing.T) {
	options := storageOptions(t)
	// Exercise URL escaping without changing process CWD.
	options.ResultsRoot = filepath.Join(options.WorkspaceRoot, "space % é", "results")
	evaluation := allocateTest(t, options, "repo")
	require.NoError(t, evaluation.Write(ConftestFile, []byte("[]\n")))
	_, err := evaluation.Read(ConftestFile)
	require.NoError(t, err)
	item, reference, err := evaluation.Reference(ConftestFile, IDPrefixConftest, "conftest-result_",
		TypeConftestResult, "Conftest results", "Stored result", "host")
	require.NoError(t, err)
	assert.Equal(t, IDPrefixConftest+evaluation.Token, item.ID)
	assert.Equal(t, item.Source.ReferenceID, reference.ID)
	repeated, repeatedReference, err := evaluation.Reference(ConftestFile, IDPrefixConftest, "conftest-result_",
		TypeConftestResult, "Conftest results", "Stored result", "host")
	require.NoError(t, err)
	assert.Equal(t, item, repeated)
	assert.Equal(t, reference, repeatedReference)
	assert.NotEqual(t, item.ID, reference.ID)
	parsed, err := url.Parse(reference.URL)
	require.NoError(t, err)
	assert.Equal(t, "file", parsed.Scheme)
	assert.Equal(t, filepath.Join(options.WorkspaceRoot, item.Source.Coordinate), parsed.Path)
	assert.Contains(t, reference.URL, "%20%25%20%C3%A9")
	_, _, err = evaluation.Reference("missing", "", "", "", "", "", "")
	require.Error(t, err)
	fixtureURL := url.URL{Scheme: "https", Host: "example.com", Path: "/a b",
		User: url.UserPassword("user", "secret"), RawQuery: "token=private", Fragment: "fragment"}
	inputURL := fixtureURL.String()
	require.NoError(t, evaluation.WriteMetadata("opa", TargetContext{
		TargetKey: RemoteTargetKey(inputURL), URL: inputURL, TargetID: strings.Repeat("a", 100), Branch: "release/1",
	}, ConfigurationContext{PolicyDirectory: filepath.Join(options.WorkspaceRoot, "policy"), DatastreamPath: "/external/source.xml"}))
	metadata, err := evaluation.Read(MetadataFile)
	require.NoError(t, err)
	var envelope map[string]any
	require.NoError(t, json.Unmarshal(metadata.Data, &envelope))
	assert.Len(t, envelope, 6)
	assert.Equal(t, float64(1), envelope["schema_version"])
	assert.Equal(t, evaluation.Token, envelope["evaluation_id"])
	assert.Equal(t, item.CollectedAt, envelope["collected_at"])
	target := envelope["target"].(map[string]any)
	assert.Equal(t, "url:example.com/a%20b", target["target_key"])
	assert.Equal(t, strings.Repeat("a", 100), target["target_id"])
	configuration := envelope["configuration"].(map[string]any)
	assert.Equal(t, "policy", configuration["policy_directory"])
	assert.Equal(t, "/external/source.xml", configuration["datastream_path"])
	for _, secret := range []string{"secret", "private", "fragment", "digest", "environment", "variables"} {
		assert.NotContains(t, string(metadata.Data), secret)
	}
	// Best-effort failure does not prevent validation/finalization of required output.
	require.Error(t, evaluation.WriteMetadata("opa", TargetContext{}, ConfigurationContext{}))
	require.NoError(t, evaluation.Finalize())
}
