// SPDX-License-Identifier: Apache-2.0

// Package evidence provides shared evidence utilities for all
// provider plugins. It centralizes digest computation and evidence
// identity constants to avoid duplication across providers.
package evidence

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Evidence type constants used in provider.Evidence.Type.
const (
	TypeARF               = "ARF"
	TypeIntotoAttestation = "IntotoAttestation"
	TypeConftestResult    = "ConftestResult"
)

// MappingReference ID constants and prefixes used in
// provider.Evidence.Source.ReferenceID and
// provider.MappingReference.ID. Prefixes use an underscore
// trailer to separate the type from the JoinForID suffix,
// keeping underscores as structural field separators.
const (
	RefOpenSCAPARF    = "openscap-arf"
	RefPrefixAmpel    = "ampel_"
	RefPrefixSnappy   = "snappy_"
	RefPrefixConftest = "conftest-input_"
)

// EnvMachineIDFile is the environment variable that overrides the
// default machine-id file path. When set, HostRemark reads the
// machine-id from this path instead of the default /etc/machine-id.
const EnvMachineIDFile = "COMPLYTIME_MACHINE_ID_FILE"

// defaultMachineIDPath is the default path to the machine-id file.
// Use the COMPLYTIME_MACHINE_ID_FILE environment variable to
// override at runtime or in tests via t.Setenv.
const defaultMachineIDPath = "/etc/machine-id"

// Evidence ID constants and prefixes used in
// provider.Evidence.ID. Prefixes use an underscore trailer
// to match the MappingReference prefix convention.
const (
	IDOpenSCAPARF    = "arf"
	IDPrefixAmpel    = "ampel_"
	IDPrefixSnappy   = "snappy_"
	IDPrefixConftest = "conftest_"
)

// FileDigest computes the SHA256 hash of the file at path and
// returns it in "sha256:<hex>" format. It returns an error if the
// path does not exist, is not a regular file, or cannot be read.
// The function opens the file first and validates the file type via
// Fstat on the descriptor, avoiding a TOCTOU window between
// validation and reading.
func FileDigest(path string) (string, error) {
	cleaned := filepath.Clean(path)
	f, err := os.Open(cleaned)
	if err != nil {
		return "", fmt.Errorf("evidence digest: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("evidence digest: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf(
			"evidence digest: %s is not a regular file", path,
		)
	}

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("evidence digest: reading %s: %w", path, err)
	}

	return fmt.Sprintf("sha256:%x", h.Sum(nil)), nil
}

var (
	hostRemarkOnce sync.Once
	cachedRemark   string
)

// HostRemark returns a human-readable string identifying the host
// where evidence was collected. It combines the system hostname with
// the machine-id from /etc/machine-id (or the path specified by the
// COMPLYTIME_MACHINE_ID_FILE environment variable). The result is
// suitable for use as EvidenceSource.Remarks.
//
// The result is computed once and cached for the process lifetime,
// since hostname and machine-id do not change between Scan calls.
//
// Fallback behavior:
//   - Both available: Collected on host "name" (machine-id: abc123)
//   - Hostname only:  Collected on host "name"
//   - Machine-id only: Collected on host with machine-id: abc123
//   - Neither:        Collected on unknown host
func HostRemark() string {
	hostRemarkOnce.Do(func() {
		cachedRemark = computeHostRemark()
	})
	return cachedRemark
}

// computeHostRemark builds the uncached host remark. Tests call it
// directly so each case can vary COMPLYTIME_MACHINE_ID_FILE.
func computeHostRemark() string {
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

// maxMachineIDSize is the upper bound for the machine-id file.
// A standard /etc/machine-id is 33 bytes (32 hex + newline). The
// generous limit prevents accidental reads of arbitrarily large files
// when COMPLYTIME_MACHINE_ID_FILE is misconfigured.
const maxMachineIDSize = 1024

// readMachineID reads the machine-id file and returns its content
// trimmed of whitespace. Returns an empty string if the file cannot
// be read, exceeds maxMachineIDSize, or does not contain a valid
// hex string.
func readMachineID() string {
	path := os.Getenv(EnvMachineIDFile)
	if path == "" {
		path = defaultMachineIDPath
	}
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return ""
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxMachineIDSize+1))
	if err != nil || len(data) > maxMachineIDSize {
		return ""
	}
	trimmed := strings.TrimSpace(string(data))
	if !isHexString(trimmed) {
		return ""
	}
	return trimmed
}

// SanitizeForID encodes a string into a human-readable, ID-safe
// form that prioritizes readability over full collision resistance.
// Alphanumeric characters and hyphens pass through unchanged.
// Forward slashes are encoded as "--" to preserve path structure
// visibility. All other characters (dots, underscores, colons,
// spaces, etc.) are replaced with a single hyphen. Underscores
// never appear in the output, reserving them as unambiguous
// component separators in JoinForID.
//
// Known limitation: the encoding is lossy. Consecutive
// non-alphanumeric characters (e.g., "--", "..", "-.") all
// produce "--" in the output, colliding with the "/" encoding.
// For example, "a/b", "a--b", and "a..b" all encode to "a--b".
// In practice, JoinForID's multi-field composition mitigates
// this: all fields (repo, branch, spec) must collide
// simultaneously, and git hosting platforms do not allow "/" in
// repository names, limiting the collision surface to local OPA
// paths with adversarial directory naming.
func SanitizeForID(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-':
			b.WriteRune(r)
		case r == '/':
			b.WriteString("--")
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}

// JoinForID encodes multiple identity components into a single
// human-readable ID string. Each component is sanitized
// individually via SanitizeForID, then joined with an underscore
// separator. Because SanitizeForID never produces underscores,
// the separator is unambiguous. This guarantees that ("a/b", "c")
// and ("a", "b/c") always produce distinct IDs ("a--b_c" vs
// "a_b--c").
func JoinForID(components ...string) string {
	sanitized := make([]string, len(components))
	for i, c := range components {
		sanitized[i] = SanitizeForID(c)
	}
	return strings.Join(sanitized, "_")
}

// StripURLScheme removes the URL scheme prefix (e.g., "https://")
// from a URL string. Used to produce a scheme-independent identity
// for evidence IDs while preserving the host, path, and all other
// URL components.
func StripURLScheme(u string) string {
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(u, prefix) {
			return u[len(prefix):]
		}
	}
	return u
}

// FileURL resolves a file path to a proper file:// URL. Relative
// paths are resolved to absolute paths. The result conforms to
// RFC 8089: file:///absolute/path. Special characters in the path
// are percent-encoded.
func FileURL(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	u := &url.URL{Scheme: "file", Path: filepath.ToSlash(absPath)}
	return u.String()
}

// isHexString reports whether s is a non-empty string of hexadecimal
// characters (0-9, a-f, A-F).
func isHexString(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}
