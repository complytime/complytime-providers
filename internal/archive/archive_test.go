// SPDX-License-Identifier: Apache-2.0

package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// tarEntry describes a single entry to include in a test archive.
type tarEntry struct {
	Name     string
	Body     []byte
	Typeflag byte
	Linkname string
}

// createTarGz builds a gzip-compressed tar archive in memory from
// the given entries.
func createTarGz(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	for _, e := range entries {
		hdr := &tar.Header{
			Name:     e.Name,
			Size:     int64(len(e.Body)),
			Typeflag: e.Typeflag,
			Mode:     0644,
			Linkname: e.Linkname,
		}
		if e.Typeflag == tar.TypeDir {
			hdr.Size = 0
		}
		require.NoError(t, tw.WriteHeader(hdr))
		if len(e.Body) > 0 {
			_, err := tw.Write(e.Body)
			require.NoError(t, err)
		}
	}

	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

// withLimits temporarily overrides the aggregate extraction limits
// for the duration of a test. Original values are restored via
// t.Cleanup.
func withLimits(
	t *testing.T, totalSize int64, fileCount int,
) {
	t.Helper()
	origSize := maxTotalExtractedSize
	origCount := maxExtractedFileCount
	maxTotalExtractedSize = totalSize
	maxExtractedFileCount = fileCount
	t.Cleanup(func() {
		maxTotalExtractedSize = origSize
		maxExtractedFileCount = origCount
	})
}

// writeTarGz is a convenience that writes archive bytes to a file
// and returns the path.
func writeTarGz(
	t *testing.T, dir, name string, data []byte,
) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, data, 0600))
	return p
}

// --- ResolveComplypackPath tests ---

func TestResolveComplypackPath_DirectoryPassthrough(
	t *testing.T,
) {
	dir := t.TempDir()
	policyDir := filepath.Join(dir, "policies")
	require.NoError(t, os.MkdirAll(policyDir, 0750))

	resolved, err := ResolveComplypackPath(policyDir)
	require.NoError(t, err)
	require.Equal(t, policyDir, resolved)
}

func TestResolveComplypackPath_TarGzExtraction(
	t *testing.T,
) {
	dir := t.TempDir()

	data := createTarGz(t, []tarEntry{
		{
			Name:     "policy.json",
			Body:     []byte(`{"id":"BP-1.01"}`),
			Typeflag: tar.TypeReg,
		},
	})
	archivePath := writeTarGz(t, dir, "content.tar.gz", data)

	resolved, err := ResolveComplypackPath(archivePath)
	require.NoError(t, err)

	expectedDir := filepath.Join(dir, "content")
	require.Equal(t, expectedDir, resolved)

	// Verify the extracted file exists with correct content.
	got, err := os.ReadFile(
		filepath.Join(expectedDir, "policy.json"),
	)
	require.NoError(t, err)
	require.Contains(t, string(got), "BP-1.01")
}

func TestResolveComplypackPath_IdempotentSkip(
	t *testing.T,
) {
	dir := t.TempDir()

	data := createTarGz(t, []tarEntry{
		{
			Name:     "policy.json",
			Body:     []byte(`{"id":"BP-1.01"}`),
			Typeflag: tar.TypeReg,
		},
	})
	archivePath := writeTarGz(t, dir, "content.tar.gz", data)

	// Pre-create the content directory with a marker file.
	contentDir := filepath.Join(dir, "content")
	require.NoError(t, os.MkdirAll(contentDir, 0750))
	markerPath := filepath.Join(contentDir, "marker.txt")
	require.NoError(t,
		os.WriteFile(markerPath, []byte("existing"), 0600),
	)

	resolved, err := ResolveComplypackPath(archivePath)
	require.NoError(t, err)
	require.Equal(t, contentDir, resolved)

	// The marker file should still exist (no re-extraction).
	got, err := os.ReadFile(markerPath)
	require.NoError(t, err)
	require.Equal(t, "existing", string(got))
}

func TestResolveComplypackPath_NonExistentPath(
	t *testing.T,
) {
	_, err := ResolveComplypackPath(
		"/nonexistent/path/content.tar.gz",
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "stat")
}

func TestResolveComplypackPath_FailedExtractionCleansUp(
	t *testing.T,
) {
	// Use a small aggregate limit so we can trigger a failure
	// without writing large files.
	withLimits(t, 10, maxExtractedFileCount)

	dir := t.TempDir()

	// Archive with a file exceeding the 10-byte aggregate limit.
	badData := createTarGz(t, []tarEntry{
		{
			Name:     "big.json",
			Body:     bytes.Repeat([]byte("x"), 20),
			Typeflag: tar.TypeReg,
		},
	})
	archivePath := writeTarGz(
		t, dir, "content.tar.gz", badData,
	)

	_, err := ResolveComplypackPath(archivePath)
	require.Error(t, err)

	// The partial extraction directory must be cleaned up.
	contentDir := filepath.Join(dir, "content")
	_, statErr := os.Stat(contentDir)
	require.True(t, os.IsNotExist(statErr),
		"content dir should be removed after failed extraction",
	)

	// Retry phase: generous limits, scoped independently.
	t.Run("retry succeeds after cleanup", func(t *testing.T) {
		withLimits(t, 500<<20, maxExtractedFileCount)

		goodData := createTarGz(t, []tarEntry{
			{
				Name:     "ok.json",
				Body:     []byte(`{"ok":true}`),
				Typeflag: tar.TypeReg,
			},
		})
		require.NoError(t,
			os.WriteFile(archivePath, goodData, 0600),
		)

		resolved, err := ResolveComplypackPath(archivePath)
		require.NoError(t, err)
		require.Equal(t, contentDir, resolved)
	})
}

// --- ExtractTarGz tests ---

func TestExtractTarGz_PathTraversalRejected(t *testing.T) {
	dir := t.TempDir()

	data := createTarGz(t, []tarEntry{
		{
			Name:     "../escape.txt",
			Body:     []byte("malicious"),
			Typeflag: tar.TypeReg,
		},
	})
	archivePath := writeTarGz(t, dir, "bad.tar.gz", data)

	dst := filepath.Join(dir, "out")
	err := ExtractTarGz(archivePath, dst)
	require.Error(t, err)
	require.Contains(t, err.Error(), "path traversal")

	// The malicious file must not exist outside the destination.
	_, statErr := os.Stat(filepath.Join(dir, "escape.txt"))
	require.True(t, os.IsNotExist(statErr))
}

func TestExtractTarGz_AbsolutePathRejected(t *testing.T) {
	dir := t.TempDir()

	data := createTarGz(t, []tarEntry{
		{
			Name:     "/etc/passwd",
			Body:     []byte("root"),
			Typeflag: tar.TypeReg,
		},
	})
	archivePath := writeTarGz(t, dir, "abs.tar.gz", data)

	dst := filepath.Join(dir, "out")
	err := ExtractTarGz(archivePath, dst)
	require.Error(t, err)
	require.Contains(t, err.Error(), "path traversal")
}

func TestExtractTarGz_SymlinkRejected(t *testing.T) {
	dir := t.TempDir()

	data := createTarGz(t, []tarEntry{
		{
			Name:     "link.txt",
			Typeflag: tar.TypeSymlink,
			Linkname: "/etc/passwd",
		},
	})
	archivePath := writeTarGz(
		t, dir, "symlink.tar.gz", data,
	)

	dst := filepath.Join(dir, "out")
	err := ExtractTarGz(archivePath, dst)
	require.Error(t, err)
	require.Contains(t, err.Error(),
		"symlinks and hard links",
	)
}

func TestExtractTarGz_HardLinkRejected(t *testing.T) {
	dir := t.TempDir()

	data := createTarGz(t, []tarEntry{
		{
			Name:     "hardlink.txt",
			Typeflag: tar.TypeLink,
			Linkname: "target.txt",
		},
	})
	archivePath := writeTarGz(
		t, dir, "hardlink.tar.gz", data,
	)

	dst := filepath.Join(dir, "out")
	err := ExtractTarGz(archivePath, dst)
	require.Error(t, err)
	require.Contains(t, err.Error(),
		"symlinks and hard links",
	)
}

func TestExtractTarGz_OversizedFileRejected(t *testing.T) {
	dir := t.TempDir()

	// Build an archive with a file larger than the per-file
	// limit. We write the actual data so the LimitReader check
	// fires.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	bigSize := maxExtractedFileSize + 1
	hdr := &tar.Header{
		Name:     "huge.bin",
		Size:     int64(bigSize),
		Typeflag: tar.TypeReg,
		Mode:     0644,
	}
	require.NoError(t, tw.WriteHeader(hdr))

	chunk := make([]byte, 1<<20) // 1 MB chunks
	written := int64(0)
	for written < int64(bigSize) {
		n := int64(len(chunk))
		if written+n > int64(bigSize) {
			n = int64(bigSize) - written
		}
		_, err := tw.Write(chunk[:n])
		require.NoError(t, err)
		written += n
	}

	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())

	archivePath := writeTarGz(
		t, dir, "huge.tar.gz", buf.Bytes(),
	)

	dst := filepath.Join(dir, "out")
	err := ExtractTarGz(archivePath, dst)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds maximum size")
}

func TestExtractTarGz_CorruptArchive(t *testing.T) {
	dir := t.TempDir()

	archivePath := filepath.Join(dir, "corrupt.tar.gz")
	require.NoError(t,
		os.WriteFile(
			archivePath, []byte("not a gzip file"), 0600,
		),
	)

	dst := filepath.Join(dir, "out")
	err := ExtractTarGz(archivePath, dst)
	require.Error(t, err)
	require.Contains(t, err.Error(), "gzip reader")
}

func TestExtractTarGz_DotDirectoryEntry(t *testing.T) {
	dir := t.TempDir()

	data := createTarGz(t, []tarEntry{
		{Name: "./", Typeflag: tar.TypeDir},
		{
			Name:     "policy.json",
			Body:     []byte(`{"id":"BP-1.01"}`),
			Typeflag: tar.TypeReg,
		},
	})
	archivePath := writeTarGz(t, dir, "dot.tar.gz", data)

	dst := filepath.Join(dir, "out")
	require.NoError(t, ExtractTarGz(archivePath, dst))

	got, err := os.ReadFile(
		filepath.Join(dst, "policy.json"),
	)
	require.NoError(t, err)
	require.Contains(t, string(got), "BP-1.01")
}

func TestExtractTarGz_FilePermissions(t *testing.T) {
	dir := t.TempDir()

	data := createTarGz(t, []tarEntry{
		{Name: "subdir/", Typeflag: tar.TypeDir},
		{
			Name:     "subdir/policy.json",
			Body:     []byte(`{}`),
			Typeflag: tar.TypeReg,
		},
	})
	archivePath := writeTarGz(t, dir, "perms.tar.gz", data)

	dst := filepath.Join(dir, "out")
	require.NoError(t, ExtractTarGz(archivePath, dst))

	fi, err := os.Stat(filepath.Join(dst, "subdir"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0750), fi.Mode().Perm())

	fi, err = os.Stat(
		filepath.Join(dst, "subdir", "policy.json"),
	)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), fi.Mode().Perm())
}

// --- Aggregate size limit tests ---

func TestExtractTarGz_AggregateSizeExceeded(t *testing.T) {
	// Use a small aggregate limit: 500 bytes total.
	withLimits(t, 500, maxExtractedFileCount)

	dir := t.TempDir()

	// 6 files of ~100 bytes each = 600 bytes > 500 limit.
	fileBody := bytes.Repeat([]byte("A"), 100)
	entries := make([]tarEntry, 6)
	for i := range entries {
		entries[i] = tarEntry{
			Name:     fmt.Sprintf("file%d.bin", i),
			Body:     fileBody,
			Typeflag: tar.TypeReg,
		}
	}
	data := createTarGz(t, entries)
	archivePath := writeTarGz(
		t, dir, "agg.tar.gz", data,
	)

	dst := filepath.Join(dir, "out")
	err := ExtractTarGz(archivePath, dst)
	require.Error(t, err)
	require.Contains(t, err.Error(), "total extracted size")
	require.Contains(t, err.Error(), "500")
}

func TestExtractTarGz_AggregateSizeAtExactLimit(
	t *testing.T,
) {
	// Aggregate limit = 200 bytes; write exactly 200 bytes
	// across two files.
	withLimits(t, 200, maxExtractedFileCount)

	dir := t.TempDir()

	data := createTarGz(t, []tarEntry{
		{
			Name:     "a.bin",
			Body:     bytes.Repeat([]byte("X"), 100),
			Typeflag: tar.TypeReg,
		},
		{
			Name:     "b.bin",
			Body:     bytes.Repeat([]byte("Y"), 100),
			Typeflag: tar.TypeReg,
		},
	})
	archivePath := writeTarGz(
		t, dir, "exact.tar.gz", data,
	)

	dst := filepath.Join(dir, "out")
	require.NoError(t, ExtractTarGz(archivePath, dst))
}

func TestExtractTarGz_AggregateSizeExceededByOne(
	t *testing.T,
) {
	// Aggregate limit = 200 bytes; write 201 bytes total.
	withLimits(t, 200, maxExtractedFileCount)

	dir := t.TempDir()

	data := createTarGz(t, []tarEntry{
		{
			Name:     "a.bin",
			Body:     bytes.Repeat([]byte("X"), 100),
			Typeflag: tar.TypeReg,
		},
		{
			Name:     "b.bin",
			Body:     bytes.Repeat([]byte("Y"), 101),
			Typeflag: tar.TypeReg,
		},
	})
	archivePath := writeTarGz(
		t, dir, "over1.tar.gz", data,
	)

	dst := filepath.Join(dir, "out")
	err := ExtractTarGz(archivePath, dst)
	require.Error(t, err)
	require.Contains(t, err.Error(), "total extracted size")
	require.Contains(t, err.Error(), "200")
}

// --- File count limit tests ---

func TestExtractTarGz_FileCountExceeded(t *testing.T) {
	// Override to a small limit for fast testing.
	withLimits(t, maxTotalExtractedSize, 5)

	dir := t.TempDir()

	// 10 zero-length files: well over the limit of 5.
	entries := make([]tarEntry, 10)
	for i := range entries {
		entries[i] = tarEntry{
			Name:     fmt.Sprintf("f%d.txt", i),
			Body:     nil,
			Typeflag: tar.TypeReg,
		}
	}
	data := createTarGz(t, entries)
	archivePath := writeTarGz(
		t, dir, "count.tar.gz", data,
	)

	dst := filepath.Join(dir, "out")
	err := ExtractTarGz(archivePath, dst)
	require.Error(t, err)
	require.Contains(t, err.Error(), "file count")
	require.Contains(t, err.Error(), "5")
}

func TestExtractTarGz_FileCountAtExactLimit(t *testing.T) {
	withLimits(t, maxTotalExtractedSize, 5)

	dir := t.TempDir()

	// Exactly 5 zero-length files: at the limit, should succeed.
	entries := make([]tarEntry, 5)
	for i := range entries {
		entries[i] = tarEntry{
			Name:     fmt.Sprintf("f%d.txt", i),
			Body:     nil,
			Typeflag: tar.TypeReg,
		}
	}
	data := createTarGz(t, entries)
	archivePath := writeTarGz(
		t, dir, "exact_count.tar.gz", data,
	)

	dst := filepath.Join(dir, "out")
	require.NoError(t, ExtractTarGz(archivePath, dst))
}

func TestExtractTarGz_FileCountExceededByOne(t *testing.T) {
	withLimits(t, maxTotalExtractedSize, 5)

	dir := t.TempDir()

	// 6 zero-length files: the 6th file (index 5) triggers the
	// error because fileCount == 5 == maxExtractedFileCount
	// before extraction.
	entries := make([]tarEntry, 6)
	for i := range entries {
		entries[i] = tarEntry{
			Name:     fmt.Sprintf("f%d.txt", i),
			Body:     nil,
			Typeflag: tar.TypeReg,
		}
	}
	data := createTarGz(t, entries)
	archivePath := writeTarGz(
		t, dir, "over1_count.tar.gz", data,
	)

	dst := filepath.Join(dir, "out")
	err := ExtractTarGz(archivePath, dst)
	require.Error(t, err)
	require.Contains(t, err.Error(), "file count")
	require.Contains(t, err.Error(), "5")
}

// --- Both limits exceeded: file count takes precedence ---

func TestExtractTarGz_BothLimitsExceeded(t *testing.T) {
	// Set both limits very low: 3 files, 90 bytes total.
	// The first 3 files consume 30 bytes each (90 total = byte
	// limit, which is allowed). The 4th file triggers the file
	// count check (3 >= 3) before extraction, so the byte limit
	// is never evaluated for that entry.
	withLimits(t, 90, 3)

	dir := t.TempDir()

	entries := make([]tarEntry, 4)
	for i := range entries {
		entries[i] = tarEntry{
			Name:     fmt.Sprintf("f%d.bin", i),
			Body:     bytes.Repeat([]byte("Z"), 30),
			Typeflag: tar.TypeReg,
		}
	}
	data := createTarGz(t, entries)
	archivePath := writeTarGz(
		t, dir, "both.tar.gz", data,
	)

	dst := filepath.Join(dir, "out")
	err := ExtractTarGz(archivePath, dst)
	require.Error(t, err)
	// File count error must be returned, not bytes error.
	require.Contains(t, err.Error(), "file count")
	require.NotContains(t, err.Error(),
		"total extracted size",
	)
}
