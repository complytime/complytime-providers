// SPDX-License-Identifier: Apache-2.0

package evidence

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const generatedFileMode = 0o644

// ReplaceFile saves generated configuration beneath an absolute workspace root.
// Parents must exist and are opened without following child symlinks. A fresh
// inode replaces the leaf atomically, so existing symlink/hard-link referents are
// never truncated. Ancestor directories must remain under trusted control during
// the operation; a held descriptor pins identity, not pathname ancestry.
func ReplaceFile(root, path string, data []byte) (resultErr error) {
	if !filepath.IsAbs(root) || !filepath.IsLocal(path) || path == "." {
		return fmt.Errorf("generated output must be relative to an absolute workspace")
	}
	parent, err := os.Open(root)
	if err != nil {
		return fmt.Errorf("opening generated-output workspace: %w", err)
	}
	components := strings.Split(filepath.Clean(path), string(filepath.Separator))
	for _, component := range components[:len(components)-1] {
		next, openErr := openDirectory(parent, component)
		closeErr := parent.Close()
		if openErr != nil {
			return fmt.Errorf("opening generated-output parent: %w", errors.Join(openErr, closeErr))
		}
		if closeErr != nil {
			return errors.Join(closeErr, next.Close())
		}
		parent = next
	}
	defer func() { resultErr = errors.Join(resultErr, parent.Close()) }()
	var entropy [tokenBytes]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return fmt.Errorf("naming generated output: %w", err)
	}
	temporary := ".generated-" + hex.EncodeToString(entropy[:])
	file, err := openAt(parent, temporary, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL, generatedFileMode)
	if err != nil {
		return fmt.Errorf("creating generated output: %w", err)
	}
	parentFD, err := directoryDescriptor(parent)
	if err != nil {
		return errors.Join(err, file.Close())
	}
	published := false
	defer func() {
		if !published {
			resultErr = errors.Join(resultErr, syscall.Unlinkat(parentFD, temporary))
		}
	}()
	written, writeErr := file.Write(data)
	if writeErr == nil && written != len(data) {
		writeErr = io.ErrShortWrite
	}
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return fmt.Errorf("saving generated output: %w", err)
	}
	if err := syscall.Renameat(parentFD, temporary, parentFD, components[len(components)-1]); err != nil {
		return fmt.Errorf("replacing generated output: %w", err)
	}
	published = true
	return nil
}
