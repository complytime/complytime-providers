// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/antchfx/xmlquery"
	"github.com/hashicorp/go-hclog"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/complytime/complytime-providers/cmd/openscap-provider/config"
	"github.com/complytime/complytime-providers/cmd/openscap-provider/oscap"
	"github.com/complytime/complytime-providers/cmd/openscap-provider/scan"
	"github.com/complytime/complytime-providers/cmd/openscap-provider/xccdf"
	"github.com/complytime/complytime-providers/internal/version"
)

var _ provider.Provider = (*ProviderServer)(nil)

type ProviderServer struct {
	// ruleToMatchID maps XCCDF rule short names to the original match
	// IDs from AssessmentConfiguration. Built during Generate so that
	// Scan can restore the original IDs on assessment results.
	ruleToMatchID map[string]string
}

func New() *ProviderServer {
	return &ProviderServer{}
}

func (s *ProviderServer) Describe(_ context.Context, _ *provider.DescribeRequest) (*provider.DescribeResponse, error) {
	return &provider.DescribeResponse{
		Healthy:                 true,
		Version:                 version.Version(),
		RequiredTargetVariables: []string{"profile"},
	}, nil
}

func (s *ProviderServer) Generate(ctx context.Context, req *provider.GenerateRequest) (*provider.GenerateResponse, error) {
	if err := generateArtifacts(ctx, req); err != nil {
		return &provider.GenerateResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}
	s.ruleToMatchID = buildRuleToMatchIDMap(req.Configuration)
	return &provider.GenerateResponse{Success: true}, nil
}

func generateArtifacts(ctx context.Context, req *provider.GenerateRequest) error {
	if len(req.Configuration) == 0 {
		return fmt.Errorf("no assessment configurations provided")
	}

	profile, datastream, err := resolveProfileAndDatastream(req)
	if err != nil {
		return err
	}

	return executeGeneration(ctx, req.Configuration, datastream, profile)
}

func resolveProfileAndDatastream(req *provider.GenerateRequest) (string, string, error) {
	vars := mergeVariables(req.GlobalVariables, req.TargetVariables)

	profile, err := config.SanitizeInput(vars["profile"])
	if err != nil {
		return "", "", fmt.Errorf("invalid profile: %w", err)
	}

	datastream, err := config.ResolveDatastream(vars["datastream"])
	if err != nil {
		return "", "", fmt.Errorf("datastream error: %w", err)
	}
	return profile, datastream, nil
}

func executeGeneration(ctx context.Context, configurations []provider.AssessmentConfiguration, datastream, profile string) error {
	if err := config.EnsureDirectories(); err != nil {
		return fmt.Errorf("directory setup failed: %w", err)
	}

	if err := writeTailoringFile(configurations, datastream, profile); err != nil {
		return err
	}

	hclog.Default().Info("Generating remediation files")
	providerDir := filepath.Join(provider.WorkspaceDir, config.ProviderDir)
	if err := oscap.OscapGenerateFix(ctx, providerDir, profile, config.PolicyPath, datastream); err != nil {
		return fmt.Errorf("remediation generation failed: %w", err)
	}
	return nil
}

func writeTailoringFile(configurations []provider.AssessmentConfiguration, datastream, profile string) error {
	hclog.Default().Info("Generating a tailoring file")
	tailoringXML, err := xccdf.PolicyToXML(configurations, datastream, profile)
	if err != nil {
		return fmt.Errorf("tailoring generation failed: %w", err)
	}

	dst, err := os.Create(config.PolicyPath)
	if err != nil {
		return fmt.Errorf("failed to create policy file: %w", err)
	}
	defer dst.Close()
	if _, err := dst.WriteString(tailoringXML); err != nil {
		return fmt.Errorf("failed to write policy file: %w", err)
	}
	return nil
}

func (s *ProviderServer) Scan(ctx context.Context, req *provider.ScanRequest) (*provider.ScanResponse, error) {
	if len(req.Targets) == 0 {
		return nil, fmt.Errorf("no targets provided")
	}

	xmlnode, err := runScanAndParseARF(ctx, req.Targets[0].Variables)
	if err != nil {
		return nil, err
	}

	assessments, err := buildAssessmentsFromARF(xmlnode, s.ruleToMatchID)
	if err != nil {
		return nil, err
	}
	return &provider.ScanResponse{Assessments: assessments}, nil
}

func runScanAndParseARF(ctx context.Context, vars map[string]string) (*xmlquery.Node, error) {
	profile, err := config.SanitizeInput(vars["profile"])
	if err != nil {
		return nil, fmt.Errorf("invalid profile: %w", err)
	}

	datastream, err := config.ResolveDatastream(vars["datastream"])
	if err != nil {
		return nil, fmt.Errorf("datastream error: %w", err)
	}

	hclog.Default().Info("Running scan", "profile", profile, "datastream", datastream)
	_, err = scan.ScanSystem(ctx, datastream, profile)
	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}

	return xccdf.ParseARFFile(config.ARFPath)
}

func buildAssessmentsFromARF(xmlnode *xmlquery.Node, ruleToMatchID map[string]string) ([]provider.AssessmentLog, error) {
	targetEl := xmlnode.SelectElement("//target")
	if targetEl == nil {
		return nil, errors.New("result has no 'target' attribute")
	}
	target := targetEl.InnerText()

	ruleTable := xccdf.NewRuleHashTable(xmlnode)
	results := xmlnode.SelectElements("//rule-result")

	var assessments []provider.AssessmentLog
	for i := range results {
		assessment, skip, err := assessmentFromRuleResult(results[i], ruleTable, target, ruleToMatchID)
		if err != nil {
			return nil, err
		}
		if !skip {
			assessments = append(assessments, assessment)
		}
	}
	return assessments, nil
}

func assessmentFromRuleResult(result *xmlquery.Node, ruleTable map[string]*xmlquery.Node, target string, ruleToMatchID map[string]string) (provider.AssessmentLog, bool, error) {
	ruleIDRef, rule, resultText, skip := resolveRuleResult(result, ruleTable)
	if skip {
		return provider.AssessmentLog{}, true, nil
	}

	return buildAssessmentLog(rule, result, ruleIDRef, resultText, target, ruleToMatchID)
}

func resolveRuleResult(result *xmlquery.Node, ruleTable map[string]*xmlquery.Node) (string, *xmlquery.Node, string, bool) {
	resultEl := result.SelectElement("result")
	if resultEl == nil {
		return "", nil, "", true
	}
	resultText := resultEl.InnerText()
	if xccdf.IsSkippableResult(resultText) {
		return "", nil, "", true
	}

	ruleIDRef := result.SelectAttr("idref")
	rule, ok := ruleTable[ruleIDRef]
	if !ok {
		return "", nil, "", true
	}
	return ruleIDRef, rule, resultText, false
}

func buildAssessmentLog(rule, result *xmlquery.Node, ruleIDRef, resultText, target string, ruleToMatchID map[string]string) (provider.AssessmentLog, bool, error) {
	ovalRefEl := xccdf.FindOVALCheckContentRef(rule)
	if ovalRefEl == nil {
		return provider.AssessmentLog{}, true, nil
	}

	xccdfShortName, err := xccdf.ParseCheck(ovalRefEl)
	if err != nil {
		return provider.AssessmentLog{}, false, err
	}

	mappedResult, err := mapResultStatus(resultText)
	if err != nil {
		return provider.AssessmentLog{}, false, err
	}

	// Restore the original match ID from the assessment configuration.
	// If no mapping exists (e.g. Scan called without prior Generate),
	// fall back to the XCCDF short name.
	requirementID := xccdfShortName
	if matchID, ok := ruleToMatchID[xccdfShortName]; ok {
		requirementID = matchID
	}

	return provider.AssessmentLog{
		RequirementID: requirementID,
		Steps: []provider.Step{
			{
				Name:    ruleIDRef,
				Result:  mappedResult,
				Message: xccdf.RuleResultMessage(rule, result, resultText),
			},
		},
		Message:    fmt.Sprintf("Host %s evaluated", target),
		Confidence: provider.ConfidenceLevelHigh,
	}, false, nil
}

// buildRuleToMatchIDMap creates a mapping from XCCDF rule short names
// (used as RequirementID in AssessmentConfiguration) to the original
// match IDs that complyctl sent. This lets Scan restore the original
// IDs on assessment results instead of returning XCCDF short names.
func buildRuleToMatchIDMap(configs []provider.AssessmentConfiguration) map[string]string {
	m := make(map[string]string, len(configs))
	for _, cfg := range configs {
		m[cfg.RequirementID] = cfg.MatchID()
	}
	return m
}

// mergeVariables combines global and target variable maps into a single
// config map. Target variables override global ones for the same key.
func mergeVariables(global, target map[string]string) map[string]string {
	merged := make(map[string]string, len(global)+len(target))
	for k, v := range global {
		merged[k] = v
	}
	for k, v := range target {
		merged[k] = v
	}
	return merged
}

func mapResultStatus(resultText string) (provider.Result, error) {
	switch resultText {
	case "pass", "fixed":
		return provider.ResultPassed, nil
	case "fail":
		return provider.ResultFailed, nil
	case "error", "unknown":
		return provider.ResultError, nil
	}
	return provider.ResultError, fmt.Errorf("couldn't match %s", resultText)
}
