// SPDX-License-Identifier: Apache-2.0

package scan

import (
	"context"
	"fmt"
	"os"

	"github.com/complytime/complytime-providers/cmd/openscap-provider/config"
	"github.com/complytime/complytime-providers/cmd/openscap-provider/oscap"
	"github.com/complytime/complytime-providers/cmd/openscap-provider/xccdf"
)

func validateOpenSCAPFiles(policyPath, datastreamPath string) (map[string]string, error) {
	if _, err := os.Stat(policyPath); err != nil {
		return nil, err
	}

	isXML, err := config.IsXMLFile(policyPath)
	if err != nil || !isXML {
		return nil, err
	}

	return map[string]string{
		"datastream": datastreamPath,
		"policy":     policyPath,
		"results":    config.ResultsPath,
		"arf":        config.ARFPath,
	}, nil
}

// ScanSystem runs an OpenSCAP evaluation using the convention-based file
// paths and the provided datastream + profile.
func ScanSystem(ctx context.Context, datastreamPath, profile string) ([]byte, error) {
	openscapFiles, err := validateOpenSCAPFiles(config.PolicyPath, datastreamPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("absent openscap files: %w\n\nDid you run the generate command?", err)
		}
		return nil, fmt.Errorf("invalid openscap files: %w", err)
	}

	tailoringProfile := fmt.Sprintf("%s_%s", profile, xccdf.XCCDFTailoringSuffix)

	output, err := oscap.OscapScan(ctx, openscapFiles, tailoringProfile)
	if err != nil {
		return output, fmt.Errorf("failed during scan: %w", err)
	}

	return output, nil
}
