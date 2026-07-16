// SPDX-License-Identifier: Apache-2.0

package archive

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// maxExtractedFileSize is the maximum size allowed for a single file
// extracted from a complypack archive. Complypack content consists of
// policy files (JSON) which are small; this limit guards against
// decompression bombs.
const maxExtractedFileSize = 100 << 20 // 100 MB

// maxTotalExtractedSize is the aggregate byte limit across all files
// in a single extraction. Declared as a var so that tests in the same
// package can temporarily override it without writing hundreds of MB.
var maxTotalExtractedSize int64 = 500 << 20 // 500 MB

// maxExtractedFileCount is the maximum number of regular files
// allowed in a single extraction. Directories do not count.
// Declared as a var for the same testability reason as above.
var maxExtractedFileCount = 10_000

// ResolveComplypackPath returns a directory containing the complypack
// content. If contentPath is already a directory it is returned as-is.
// If it is a tar.gz archive it is extracted to a sibling "content"
// directory (idempotent: skips extraction when the directory exists).
func ResolveComplypackPath(contentPath string) (string, error) {
	info, err := os.Stat(contentPath)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", contentPath, err)
	}

	if info.IsDir() {
		return contentPath, nil
	}

	// Archive file — extract next to the archive.
	extractDir := filepath.Join(
		filepath.Dir(contentPath), "content",
	)

	// Idempotent: if a previous run already extracted, reuse it.
	if fi, statErr := os.Stat(extractDir); statErr == nil &&
		fi.IsDir() {
		return extractDir, nil
	}

	if err := ExtractTarGz(contentPath, extractDir); err != nil {
		// Clean up partial extraction so the idempotent check
		// does not reuse incomplete content on a subsequent
		// attempt.
		_ = os.RemoveAll(extractDir)
		return "", fmt.Errorf(
			"extracting complypack archive: %w", err,
		)
	}
	return extractDir, nil
}

// ExtractTarGz extracts a gzip-compressed tar archive into dst.
// It creates dst and all necessary parent directories. Symlinks,
// hard links, and entries with path traversal components are
// rejected. Individual files are capped at maxExtractedFileSize
// bytes, aggregate extraction is capped at maxTotalExtractedSize
// bytes and maxExtractedFileCount regular files.
func ExtractTarGz(archive, dst string) error {
	f, err := os.Open(archive)
	if err != nil {
		return fmt.Errorf("opening archive: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("creating gzip reader: %w", err)
	}
	defer gz.Close()

	if err := os.MkdirAll(dst, 0750); err != nil {
		return fmt.Errorf(
			"creating destination directory: %w", err,
		)
	}

	tr := tar.NewReader(gz)

	var totalBytes int64
	var fileCount int

	for {
		hdr, readErr := tr.Next()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf(
				"reading tar entry: %w", readErr,
			)
		}

		// Reject path traversal.
		clean := filepath.Clean(hdr.Name)
		if strings.HasPrefix(clean, "..") ||
			filepath.IsAbs(clean) {
			return fmt.Errorf(
				"tar entry %q contains path traversal",
				hdr.Name,
			)
		}

		// Skip the root directory entry produced by standard
		// tar tools. The destination directory is already
		// created by os.MkdirAll above.
		if clean == "." {
			continue
		}

		target := filepath.Join(dst, clean)

		// Zip-slip guard: verify the resolved path is
		// within dst.
		if !strings.HasPrefix(
			target,
			filepath.Clean(dst)+string(os.PathSeparator),
		) {
			return fmt.Errorf(
				"tar entry %q resolves outside destination",
				hdr.Name,
			)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if mkErr := os.MkdirAll(target, 0750); mkErr != nil {
				return fmt.Errorf(
					"creating directory %s: %w",
					target, mkErr,
				)
			}
		case tar.TypeReg:
			// File count check precedes byte count check
			// for deterministic precedence when both limits
			// are exceeded simultaneously.
			if fileCount >= maxExtractedFileCount {
				return fmt.Errorf(
					"tar entry %q: file count limit "+
						"of %d exceeded",
					hdr.Name, maxExtractedFileCount,
				)
			}

			if mkErr := os.MkdirAll(
				filepath.Dir(target), 0750,
			); mkErr != nil {
				return fmt.Errorf(
					"creating parent directory for %s: %w",
					target, mkErr,
				)
			}

			n, writeErr := writeFileFromTar(target, tr)
			if writeErr != nil {
				return writeErr
			}

			fileCount++
			totalBytes += n

			if totalBytes > maxTotalExtractedSize {
				return fmt.Errorf(
					"tar entry %q: total extracted size "+
						"%d exceeds limit of %d bytes",
					hdr.Name, totalBytes,
					maxTotalExtractedSize,
				)
			}
		case tar.TypeSymlink, tar.TypeLink:
			return fmt.Errorf(
				"tar entry %q: symlinks and hard links "+
					"are not permitted", hdr.Name,
			)
		default:
			// Skip metadata-only entries (e.g., pax headers).
			continue
		}
	}
	return nil
}

// writeFileFromTar writes a single file from a tar reader and returns
// the number of bytes written. Files are created with mode 0600
// (owner read/write only) regardless of the archive header to enforce
// the project's permission model. The write is capped at
// maxExtractedFileSize to guard against decompression bombs.
func writeFileFromTar(
	path string, r io.Reader,
) (int64, error) {
	out, err := os.OpenFile(
		path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600,
	)
	if err != nil {
		return 0, fmt.Errorf("creating file %s: %w", path, err)
	}
	defer out.Close()

	limited := io.LimitReader(r, maxExtractedFileSize+1)
	n, copyErr := io.Copy(out, limited)
	if copyErr != nil {
		return 0, fmt.Errorf(
			"writing file %s: %w", path, copyErr,
		)
	}
	if n > maxExtractedFileSize {
		return 0, fmt.Errorf(
			"file %s exceeds maximum size of %d bytes",
			path, maxExtractedFileSize,
		)
	}
	return n, nil
}
