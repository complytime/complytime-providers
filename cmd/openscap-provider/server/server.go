// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/antchfx/xmlquery"
	"github.com/hashicorp/go-hclog"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/complytime/complytime-providers/cmd/openscap-provider/config"
	"github.com/complytime/complytime-providers/cmd/openscap-provider/oscap"
	"github.com/complytime/complytime-providers/cmd/openscap-provider/xccdf"
	"github.com/complytime/complytime-providers/internal/evidence"
	"github.com/complytime/complytime-providers/internal/version"
)

var _ provider.Provider = (*ProviderServer)(nil)

// ProviderServer implements the OpenSCAP plugin with request-scoped artifact paths.
type ProviderServer struct {
	logger   hclog.Logger
	identity func() (evidence.Identity, error)
	allocate func(evidence.StorageOptions, ...string) (*evidence.Evaluation, error)
	// ruleToMatchID maps XCCDF rule short names to the original match
	// IDs from AssessmentConfiguration. Built during Generate so that
	// Scan can restore the original IDs on assessment results.
	ruleToMatchID map[string]string
}

// New constructs an OpenSCAP provider using OS execution identity and private storage.
func New() *ProviderServer {
	return &ProviderServer{logger: hclog.Default(), identity: evidence.ExecutionIdentity,
		allocate: evidence.Allocate}
}

func (s *ProviderServer) Describe(_ context.Context, _ *provider.DescribeRequest) (*provider.DescribeResponse, error) {
	return &provider.DescribeResponse{
		Healthy:                 true,
		Version:                 version.Version(),
		RequiredTargetVariables: []string{"profile"},
	}, nil
}

func (s *ProviderServer) Generate(ctx context.Context, req *provider.GenerateRequest) (*provider.GenerateResponse, error) {
	root, err := evidence.GenerateWorkspace(req)
	if err != nil {
		return nil, err
	}
	if err := generateArtifacts(ctx, req, config.WorkspaceConfig{Root: root}); err != nil {
		return &provider.GenerateResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}
	s.ruleToMatchID = buildRuleToMatchIDMap(req.Configuration)
	return &provider.GenerateResponse{Success: true}, nil
}

func generateArtifacts(ctx context.Context, req *provider.GenerateRequest, cfg config.WorkspaceConfig) error {
	if len(req.Configuration) == 0 {
		return fmt.Errorf("no assessment configurations provided")
	}

	profile, datastream, err := resolveProfileAndDatastream(req, cfg)
	if err != nil {
		return err
	}

	return executeGeneration(ctx, req.Configuration, datastream, profile, cfg)
}

func resolveProfileAndDatastream(req *provider.GenerateRequest, cfg config.WorkspaceConfig) (string, string, error) {
	vars := mergeVariables(req.GlobalVariables, req.TargetVariables)

	profile, err := config.SanitizeInput(vars["profile"])
	if err != nil {
		return "", "", fmt.Errorf("invalid profile: %w", err)
	}

	datastream, err := cfg.ResolveDatastream(vars["datastream"])
	if err != nil {
		return "", "", fmt.Errorf("datastream error: %w", err)
	}
	return profile, datastream, nil
}

func executeGeneration(ctx context.Context, configurations []provider.AssessmentConfiguration, datastream, profile string, cfg config.WorkspaceConfig) error {
	if err := cfg.EnsureDirectories(); err != nil {
		return fmt.Errorf("directory setup failed: %w", err)
	}

	if err := writeTailoringFile(configurations, datastream, profile, cfg.PolicyPath()); err != nil {
		return err
	}

	hclog.Default().Info("Generating remediation files")
	if err := oscap.OscapGenerateFix(ctx, cfg.ProviderPath(), profile, cfg.PolicyPath(), datastream, cfg.Root); err != nil {
		return fmt.Errorf("remediation generation failed: %w", err)
	}
	return nil
}

func writeTailoringFile(configurations []provider.AssessmentConfiguration, datastream, profile, policyPath string) error {
	hclog.Default().Info("Generating a tailoring file")
	tailoringXML, err := xccdf.PolicyToXML(configurations, datastream, profile)
	if err != nil {
		return fmt.Errorf("tailoring generation failed: %w", err)
	}

	dst, err := os.Create(policyPath)
	if err != nil {
		return fmt.Errorf("failed to create policy file: %w", err)
	}
	_, writeErr := dst.WriteString(tailoringXML)
	if err := errors.Join(writeErr, dst.Close()); err != nil {
		return fmt.Errorf("failed to save policy file: %w", err)
	}
	return nil
}

// attachARFEvidence adds a single ARF evidence entry to each assessment
// log, linking every assessment result to the OpenSCAP ARF source artifact.
func attachARFEvidence(assessments []provider.AssessmentLog, item provider.Evidence) {
	for i := range assessments {
		assessments[i].Evidence = []provider.Evidence{item}
	}
}

func buildAssessmentsFromARF(xmlnode *xmlquery.Node, ruleToMatchID map[string]string) ([]provider.AssessmentLog, error) {
	targetEl := xmlnode.SelectElement("//target")
	if targetEl == nil {
		return nil, errors.New("result has no 'target' attribute")
	}
	target := targetEl.InnerText()
	if xmlnode.SelectElement("//ds:component/xccdf-1.2:Benchmark") == nil {
		return nil, errors.New("result has no XCCDF Benchmark")
	}

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
	case "notapplicable":
		return provider.ResultSkipped, nil
	case "error", "unknown":
		return provider.ResultError, nil
	}
	return provider.ResultError, fmt.Errorf("couldn't match %s", resultText)
}
