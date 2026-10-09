// SPDX-License-Identifier: Apache-2.0

// Package evidence provides shared evidence utilities for all
// provider plugins. It centralizes digest computation and evidence
// identity constants to avoid duplication across providers.
package evidence

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Evidence type constants used in provider.Evidence.Type.
const (
	TypeARF               = "ARF"
	TypeIntotoAttestation = "IntotoAttestation"
	TypeConftestResult    = "ConftestResult"
)

// MappingReference ID constants and prefixes used in
// provider.Evidence.Source.ReferenceID and
// provider.MappingReference.ID.
const (
	RefOpenSCAPARF    = "openscap-arf"
	RefPrefixAmpel    = "ampel-"
	RefPrefixSnappy   = "snappy-"
	RefPrefixConftest = "conftest-input-"
)

// Evidence ID constants and prefixes used in
// provider.Evidence.ID.
const (
	IDOpenSCAPARF    = "arf"
	IDPrefixAmpel    = "ampel-"
	IDPrefixSnappy   = "snappy-"
	IDPrefixConftest = "conftest-"
)

// FileDigest computes the SHA256 hash of the file at path and
// returns it in "sha256:<hex>" format. It returns an error if the
// path does not exist, is a directory, or cannot be read.
func FileDigest(path string) (string, error) {
	cleaned := filepath.Clean(path)
	info, err := os.Stat(cleaned)
	if err != nil {
		return "", fmt.Errorf("evidence digest: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("evidence digest: %s is a directory", path)
	}

	f, err := os.Open(cleaned)
	if err != nil {
		return "", fmt.Errorf("evidence digest: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("evidence digest: reading %s: %w", path, err)
	}

	return fmt.Sprintf("sha256:%x", h.Sum(nil)), nil
}
