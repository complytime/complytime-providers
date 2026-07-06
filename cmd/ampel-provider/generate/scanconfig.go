// SPDX-License-Identifier: Apache-2.0

package generate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// ScanConfigFileName is the name of the generation artifact consumed by Scan.
	ScanConfigFileName = "scan-config.json"
)

// ScanConfig is the generation artifact written by Generate and read by Scan.
// It records the Gemara requirement IDs matched during Generate so that Scan
// can synthesize passing assessments for requirements with zero findings.
type ScanConfig struct {
	RequirementIDs []string `json:"requirement_ids"`
	GeneratedAt    string   `json:"generated_at"`
}

// WriteScanConfig writes a scan-config.json to the given directory.
func WriteScanConfig(dir string, requirementIDs []string) error {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("creating generated directory: %w", err)
	}

	cfg := ScanConfig{
		RequirementIDs: requirementIDs,
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling scan config: %w", err)
	}

	path := filepath.Join(dir, ScanConfigFileName)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing scan config: %w", err)
	}

	return nil
}

// ReadScanConfig reads a scan-config.json from the given directory. Returns
// an error if the file is missing or malformed.
func ReadScanConfig(dir string) (*ScanConfig, error) {
	path := filepath.Join(dir, ScanConfigFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading scan config: %w", err)
	}

	var cfg ScanConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing scan config: %w", err)
	}

	return &cfg, nil
}
