// SPDX-License-Identifier: Apache-2.0

package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/antchfx/xmlquery"
	"github.com/complytime/complyctl/pkg/provider"

	"github.com/complytime/complytime-providers/cmd/openscap-provider/config"
	"github.com/complytime/complytime-providers/cmd/openscap-provider/scan"
	"github.com/complytime/complytime-providers/cmd/openscap-provider/xccdf"
	"github.com/complytime/complytime-providers/internal/evidence"
)

// Scan retains one executed system evaluation's ARF and supporting XCCDF output.
// All operational failures return an RPC error without publishing a response.
func (s *ProviderServer) Scan(ctx context.Context, req *provider.ScanRequest) (*provider.ScanResponse, error) {
	if len(req.Targets) == 0 {
		return nil, fmt.Errorf("no targets provided")
	}
	root, err := evidence.ScanWorkspace(req)
	if err != nil {
		return nil, err
	}
	identity, err := s.identity()
	if err != nil {
		return nil, fmt.Errorf("execution identity: %w", err)
	}
	cfg := config.WorkspaceConfig{Root: root}
	target := req.Targets[0]
	profile, err := config.SanitizeInput(target.Variables["profile"])
	if err != nil {
		return nil, fmt.Errorf("invalid profile: %w", err)
	}
	datastream, err := cfg.ResolveDatastream(target.Variables["datastream"])
	if err != nil {
		return nil, fmt.Errorf("datastream error: %w", err)
	}
	targetLabel := target.TargetID
	if targetLabel == "" {
		targetLabel = "system"
	}
	evaluation, err := s.allocate(config.StorageOptions(root, identity), targetLabel, profile)
	if err != nil {
		return nil, fmt.Errorf("allocating OpenSCAP evaluation: %w", err)
	}
	response, scanErr := s.scanEvaluation(ctx, cfg, evaluation, target, datastream, profile)
	if err := errors.Join(scanErr, evaluation.Close()); err != nil {
		return nil, fmt.Errorf("OpenSCAP evaluation: %w", err)
	}
	return response, nil
}

func (s *ProviderServer) scanEvaluation(ctx context.Context, cfg config.WorkspaceConfig,
	evaluation *evidence.Evaluation, target provider.Target, datastream, profile string,
) (*provider.ScanResponse, error) {
	arfPath, err := evaluation.Prepare(evidence.ARFFile)
	if err != nil {
		return nil, err
	}
	resultsPath, err := evaluation.Prepare(evidence.XCCDFFile)
	if err != nil {
		return nil, err
	}
	s.logger.Info("Running scan", "profile", profile, "datastream", datastream)
	_, err = scan.ScanSystem(ctx, scan.Options{Workspace: cfg.Root, PolicyPath: cfg.PolicyPath(),
		DatastreamPath: datastream, Profile: profile, ResultsPath: resultsPath, ARFPath: arfPath})
	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}
	node, err := readEvaluation(evaluation)
	if err != nil {
		return nil, err
	}
	assessments, err := buildAssessmentsFromARF(node, s.ruleToMatchID)
	if err != nil {
		return nil, err
	}
	item, reference, err := evaluation.Reference(evidence.ARFFile,
		evidence.IDOpenSCAPARF+"_", evidence.RefOpenSCAPARF+"_", evidence.TypeARF,
		"OpenSCAP ARF scan result", fmt.Sprintf("OpenSCAP evaluation of %s with profile %s",
			evidence.HostTargetKey(target.TargetID), profile), evidence.HostRemark())
	if err != nil {
		return nil, err
	}
	evaluation.RecordMetadata(s.logger, config.ProviderDir, evidence.TargetContext{
		TargetKey: evidence.HostTargetKey(target.TargetID), TargetID: target.TargetID,
		ObservedTarget: node.SelectElement("//target").InnerText(),
	}, evidence.ConfigurationContext{Profile: profile, DatastreamPath: datastream,
		TailoringPath: cfg.PolicyPath()})
	if err := evaluation.Finalize(); err != nil {
		return nil, err
	}
	attachARFEvidence(assessments, item)
	return &provider.ScanResponse{Assessments: assessments,
		MappingReferences: []provider.MappingReference{reference}}, nil
}

func readEvaluation(evaluation *evidence.Evaluation) (*xmlquery.Node, error) {
	arf, err := evaluation.Read(evidence.ARFFile)
	if err != nil {
		return nil, fmt.Errorf("reading retained ARF: %w", err)
	}
	node, err := xccdf.ParseARF(bytes.NewReader(arf.Data))
	if err != nil {
		return nil, err
	}
	results, err := evaluation.Read(evidence.XCCDFFile)
	if err != nil {
		return nil, fmt.Errorf("reading retained XCCDF results: %w", err)
	}
	if _, err := xccdf.ParseARF(bytes.NewReader(results.Data)); err != nil {
		return nil, fmt.Errorf("invalid retained XCCDF results: %w", err)
	}
	return node, nil
}
