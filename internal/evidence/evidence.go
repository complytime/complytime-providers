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
	"strings"
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

// EnvMachineIDFile is the environment variable that overrides the
// default machine-id file path. When set, HostRemark reads the
// machine-id from this path instead of MachineIDPath.
const EnvMachineIDFile = "COMPLYTIME_MACHINE_ID_FILE"

// MachineIDPath is the default path to the machine-id file.
// Override this in tests to avoid reading the real system file.
var MachineIDPath = "/etc/machine-id"

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

// HostRemark returns a human-readable string identifying the host
// where evidence was collected. It combines the system hostname with
// the machine-id from /etc/machine-id (or the path specified by the
// COMPLYTIME_MACHINE_ID_FILE environment variable). The result is
// suitable for use as EvidenceSource.Remarks.
//
// Fallback behavior:
//   - Both available: Collected on host "name" (machine-id: abc123)
//   - Hostname only:  Collected on host "name"
//   - Machine-id only: Collected on host with machine-id: abc123
//   - Neither:        Collected on unknown host
func HostRemark() string {
	hostname, hostErr := os.Hostname()
	machineID := readMachineID()

	switch {
	case hostErr == nil && machineID != "":
		return fmt.Sprintf(
			"Collected on host %q (machine-id: %s)", hostname, machineID,
		)
	case hostErr == nil:
		return fmt.Sprintf("Collected on host %q", hostname)
	case machineID != "":
		return fmt.Sprintf(
			"Collected on host with machine-id: %s", machineID,
		)
	default:
		return "Collected on unknown host"
	}
}

// readMachineID reads the machine-id file and returns its content
// trimmed of whitespace. Returns an empty string if the file cannot
// be read.
func readMachineID() string {
	path := os.Getenv(EnvMachineIDFile)
	if path == "" {
		path = MachineIDPath
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
