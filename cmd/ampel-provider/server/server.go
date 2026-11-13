// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hashicorp/go-hclog"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/complytime/complytime-providers/cmd/ampel-provider/config"
	"github.com/complytime/complytime-providers/cmd/ampel-provider/convert"
	"github.com/complytime/complytime-providers/cmd/ampel-provider/generate"
	"github.com/complytime/complytime-providers/cmd/ampel-provider/results"
	"github.com/complytime/complytime-providers/cmd/ampel-provider/scan"
	"github.com/complytime/complytime-providers/cmd/ampel-provider/targets"
	"github.com/complytime/complytime-providers/cmd/ampel-provider/toolcheck"
	"github.com/complytime/complytime-providers/internal/archive"
	"github.com/complytime/complytime-providers/internal/evidence"
	"github.com/complytime/complytime-providers/internal/genindex"
	"github.com/complytime/complytime-providers/internal/version"
)

// ScanRunner is used by Scan to execute scan commands.
// It defaults to scan.ExecRunner{} and can be overridden for testing.
var ScanRunner scan.CommandRunner = scan.ExecRunner{}

// SkipToolCheck disables tool presence validation. Used in tests.
var SkipToolCheck bool

// safeBranchPattern matches valid git branch names.
var safeBranchPattern = regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)

var _ provider.Provider = (*ProviderServer)(nil)

// ProviderServer implements the provider.Provider interface for the AMPEL provider.
type ProviderServer struct {
	logger   hclog.Logger
	identity func() (evidence.Identity, error)
	allocate func(evidence.StorageOptions, ...string) (*evidence.Evaluation, error)
}

// New returns a new ProviderServer.
func New() *ProviderServer {
	return &ProviderServer{logger: hclog.Default(), identity: evidence.ExecutionIdentity, allocate: evidence.Allocate}
}

// Describe returns the provider metadata and health status.
func (s *ProviderServer) Describe(_ context.Context, _ *provider.DescribeRequest) (*provider.DescribeResponse, error) {
	return &provider.DescribeResponse{
		Healthy:                 true,
		Version:                 version.Version(),
		RequiredTargetVariables: []string{"url", "specs"},
	}, nil
}

// Generate matches requirement IDs from the assessment configurations against
// granular AMPEL policy files and merges the matched policies into a single
// bundle for scan.
func (s *ProviderServer) Generate(_ context.Context, req *provider.GenerateRequest) (*provider.GenerateResponse, error) {
	logger := s.logger
	root, err := evidence.GenerateWorkspace(req)
	if err != nil {
		return nil, err
	}
	cfg := config.NewWorkspaceConfig(root)

	if len(req.Configuration) == 0 {
		return &provider.GenerateResponse{
			Success:      false,
			ErrorMessage: "no assessment configurations provided",
		}, nil
	}

	if err := checkRequiredTools(logger); err != nil {
		return &provider.GenerateResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	if err := cfg.EnsureDirectories(); err != nil {
		return &provider.GenerateResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("directory setup failed: %v", err),
		}, nil
	}

	logger.Info("generating AMPEL policy")

	resolvedRequest := *req
	resolvedRequest.ComplypackContentPath = cfg.Path(req.ComplypackContentPath)
	sourceDir, resolveErr := resolvePolicyDir(logger, &resolvedRequest)
	if resolveErr != nil {
		return &provider.GenerateResponse{
			Success:      false,
			ErrorMessage: resolveErr.Error(),
		}, nil
	}
	sourceDir = cfg.Path(sourceDir)
	granular, err := convert.LoadGranularPolicies(sourceDir)
	if err != nil {
		return &provider.GenerateResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("loading granular policies: %v", err),
		}, nil
	}

	matched, warnings := convert.MatchPolicies(req.Configuration, granular)
	for _, w := range warnings {
		logger.Warn(w)
	}

	if len(matched) == 0 {
		logger.Info("no matching policies found, skipping policy generation")
		return &provider.GenerateResponse{Success: true}, nil
	}

	bundle := convert.MergeToBundle(matched)
	// Persist matched requirement IDs so Scan can synthesize passing
	// assessments for requirements with zero findings.
	requirementIDs := make([]string, len(matched))
	for i, p := range matched {
		requirementIDs[i] = p.ID
	}
	contentKey := genindex.ContentKey(requirementIDs, sourceDir)
	outputDir := cfg.Path(config.GeneratedDirForKey(contentKey))
	if err := os.MkdirAll(outputDir, genindex.ContentDirectoryMode); err != nil {
		return &provider.GenerateResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("creating generated artifact directory: %v", err),
		}, nil
	}
	if err := convert.WritePolicy(bundle, outputDir); err != nil {
		return &provider.GenerateResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("writing AMPEL policy bundle: %v", err),
		}, nil
	}
	if err := generate.WriteScanConfig(outputDir, requirementIDs); err != nil {
		return &provider.GenerateResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("writing scan config: %v", err),
		}, nil
	}
	if err := genindex.UpdateIndex(
		cfg.Path(config.GeneratedDirPath()), genindex.TargetKey(req.TargetVariables["url"]), contentKey,
	); err != nil {
		return &provider.GenerateResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("updating generate index: %v", err),
		}, nil
	}

	logger.Info("AMPEL policy bundle written", "path", outputDir, "policies", len(matched))
	return &provider.GenerateResponse{Success: true}, nil
}

// Scan invokes the AMPEL toolchain to scan target repositories and returns
// standardized assessment results.
func (s *ProviderServer) Scan(_ context.Context, req *provider.ScanRequest) (*provider.ScanResponse, error) {
	logger := s.logger

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
	cfg := config.NewWorkspaceConfig(root)

	if err := checkRequiredTools(logger); err != nil {
		return nil, err
	}

	if err := cfg.EnsureDirectories(); err != nil {
		return nil, fmt.Errorf("directory setup failed: %w", err)
	}

	logger.Info("scanning target repositories")

	specDir := cfg.Path(config.SpecDirPath())
	runner := ScanRunner
	if execution, ok := runner.(scan.ExecRunner); ok {
		execution.WorkingDirectory = root
		runner = execution
	}

	if err := scan.WriteSpecFiles(specDir); err != nil {
		return nil, fmt.Errorf("writing spec files: %w", err)
	}

	baseScanCfg := scan.ScanConfig{
		SpecDir: root,
	}
	index, indexErr := genindex.ReadIndex(cfg.Path(config.GeneratedDirPath()))
	if indexErr != nil && !errors.Is(indexErr, os.ErrNotExist) {
		return nil, fmt.Errorf("reading generate index for target %q: %w", req.Targets[0].TargetID, indexErr)
	}
	hasIndex := indexErr == nil
	var legacyScanConfig *generate.ScanConfig
	if !hasIndex {
		logger.Warn("generate index absent; using legacy scan configuration fallback")
		legacyScanConfig, indexErr = generate.ReadScanConfig(cfg.Path(config.ScanConfigDirPath()))
		if indexErr != nil {
			logger.Warn("scan config not found, passing assessments will not be synthesized", "error", indexErr)
		}
	}
	scanConfigCache := make(map[string]*generate.ScanConfig)

	var repoResults []*results.PerRepoResult
	hostRemark := evidence.HostRemark()

	for _, target := range req.Targets {
		repoURL := evidence.PublicContext(target.Variables["url"])
		if repoURL == "" {
			return nil, fmt.Errorf("target %q: missing required variable 'url'", target.TargetID)
		}
		specsStr := target.Variables["specs"]
		if specsStr == "" {
			return nil, fmt.Errorf("target %q: missing required variable 'specs'", target.TargetID)
		}

		branchesStr := target.Variables["branches"]
		if branchesStr == "" {
			branchesStr = "main"
		}

		accessToken := target.Variables["access_token"]
		platformHint := target.Variables["platform"]

		branches := splitCSV(branchesStr)
		specs := splitCSV(specsStr)

		// Defense-in-depth: validate target variables on the provider side
		if err := validateTargetVariables(repoURL, branches, specs, accessToken, evidence.PublicContext(target.TargetID)); err != nil {
			return nil, err
		}
		targetScanConfig, policyPath, err := resolveTargetArtifacts(
			target, index, hasIndex, legacyScanConfig, scanConfigCache, cfg,
		)
		if err != nil {
			return nil, err
		}
		scanCfg := baseScanCfg
		scanCfg.PolicyPath = cfg.Path(policyPath)

		// Validate and detect platform
		platform, _, _, err := targets.ParseRepoURL(repoURL, platformHint)
		if err != nil {
			return nil, fmt.Errorf("target %q: %w", target.TargetID, err)
		}

		repo := scan.RepoTarget{
			URL:         repoURL,
			AccessToken: accessToken,
			Platform:    platform,
		}
		configuration := evidence.ConfigurationContext{PolicyPath: scanCfg.PolicyPath}
		if targetScanConfig != nil {
			configuration.RequirementIDs = targetScanConfig.RequirementIDs
		}
		if hasIndex {
			configuration.ContentKey = filepath.Base(filepath.Dir(scanCfg.PolicyPath))
		}
		options := evaluationOptions{
			targetID: target.TargetID, repository: repo, scanConfig: scanCfg,
			configuration: configuration, storage: cfg.StorageOptions(identity),
			runner: runner, remarks: hostRemark,
		}

		for _, branch := range branches {
			for _, specRef := range specs {
				parsed, err := s.scanEvaluation(options, branch, specRef)
				if err != nil {
					return nil, err
				}
				repoResults = append(repoResults, parsed)
			}
		}
	}

	scanResponse := results.ToScanResponse(repoResults, nil)
	logger.Info("scan complete", "repositories_scanned", len(repoResults))
	return scanResponse, nil
}

func finalizeEvaluation(evaluation *evidence.Evaluation, parsed *results.PerRepoResult, remarks string) error {
	for _, artifact := range []struct{ name, prefix, title string }{
		{evidence.AmpelFile, evidence.IDPrefixAmpel, "Ampel policy attestation"},
		{evidence.SnappyFile, evidence.IDPrefixSnappy, "Snappy observation"},
	} {
		// Re-read after tool completion to detect loss of an earlier required artifact;
		// this preserves Ampel's existing RPC error channel for digest failures.
		if _, err := evaluation.Read(artifact.name); err != nil {
			return fmt.Errorf("evidence digest failed: %w", err)
		}
		item, reference, err := evaluation.Reference(artifact.name, artifact.prefix, artifact.prefix,
			evidence.TypeIntotoAttestation, artifact.title+" for "+parsed.Repository+"@"+parsed.Branch,
			"Retained in-toto attestation; collection spec: "+parsed.SpecLabel, remarks)
		if err != nil {
			return err
		}
		parsed.RetainedEvidence = append(parsed.RetainedEvidence, item)
		parsed.References = append(parsed.References, reference)
	}
	return nil
}

func publicOperationalError(err error, accessToken string) string {
	message := err.Error()
	if accessToken != "" {
		message = strings.ReplaceAll(message, accessToken, "[redacted]")
	}
	return evidence.PublicContext(message)
}

func resolveTargetArtifacts(
	target provider.Target,
	index genindex.GenerateIndex,
	hasIndex bool,
	legacyScanConfig *generate.ScanConfig,
	cache map[string]*generate.ScanConfig,
	cfg *config.Config,
) (*generate.ScanConfig, string, error) {
	if !hasIndex {
		return legacyScanConfig, filepath.Join(config.GeneratedPolicyDirPath(), convert.PolicyFileName), nil
	}

	scanConfig, contentKey, err := genindex.ResolveConfig(
		genindex.TargetKey(target.Variables["url"]), index, cfg.Path(config.GeneratedDirPath()), cache,
		generate.ReadScanConfig,
	)
	if err != nil {
		return nil, "", fmt.Errorf("resolving generated config for target %q: %w", target.TargetID, err)
	}
	return scanConfig, filepath.Join(config.GeneratedDirForKey(contentKey), convert.PolicyFileName), nil
}

// splitCSV splits a comma-separated string into trimmed, non-empty parts.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// validateTargetVariables performs defense-in-depth validation of target
// variable values received from the CLI. This catches issues even if the
// CLI validation was bypassed.
func validateTargetVariables(repoURL string, branches, specs []string, accessToken, targetID string) error {
	prefix := fmt.Sprintf("target %q", targetID)

	// URL: must be HTTPS, valid structure
	parsed, err := url.Parse(repoURL)
	if err != nil {
		return fmt.Errorf("%s: invalid url %q: %w", prefix, repoURL, err)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("%s: url %q must use HTTPS scheme", prefix, repoURL)
	}

	// Branches: safe characters, no path traversal
	for _, branch := range branches {
		if !safeBranchPattern.MatchString(branch) {
			return fmt.Errorf("%s: branch name contains invalid characters: %q", prefix, branch)
		}
		if strings.Contains(branch, "..") {
			return fmt.Errorf("%s: branch name contains path traversal: %q", prefix, branch)
		}
	}

	// Specs: non-empty, no path traversal
	for _, spec := range specs {
		if spec == "" {
			return fmt.Errorf("%s: spec cannot be empty", prefix)
		}
		if strings.Contains(spec, "..") {
			return fmt.Errorf("%s: spec contains path traversal: %q", prefix, spec)
		}
	}

	// AccessToken: reject newlines and null bytes
	if accessToken != "" {
		if strings.ContainsAny(accessToken, "\n\r\x00") {
			return fmt.Errorf("%s: access_token contains invalid characters (newline or null byte)", prefix)
		}
	}

	return nil
}

// resolvePolicyDir determines the directory containing granular policy
// files for Generate. It checks ComplypackContentPath first (delivered by
// complyctl from a cached complypack), then the ampel_policy_dir global
// variable, and finally falls back to the default path.
func resolvePolicyDir(logger hclog.Logger, req *provider.GenerateRequest) (string, error) {
	if req.ComplypackContentPath != "" {
		logger.Info("using complypack content path for generate",
			"complypack_content_path", req.ComplypackContentPath)
		resolved, err := archive.ResolveComplypackPath(req.ComplypackContentPath)
		if err != nil {
			return "", fmt.Errorf("resolving complypack content path: %w", err)
		}
		return resolved, nil
	}

	if customDir, ok := req.GlobalVariables["ampel_policy_dir"]; ok && customDir != "" {
		return customDir, nil
	}

	return config.GranularPolicyDirPath(), nil
}

// checkRequiredTools validates that all required AMPEL tools are on PATH
// and that ampel meets the minimum version for --allow-skip support.
func checkRequiredTools(logger hclog.Logger) error {
	if SkipToolCheck {
		return nil
	}
	missing, err := toolcheck.CheckTools()
	if err != nil {
		return fmt.Errorf("checking required tools: %w", err)
	}
	if len(missing) > 0 {
		logger.Warn("required tools missing", "tools", missing)
		return toolcheck.FormatMissingToolsError(missing)
	}
	// Verify ampel version supports --allow-skip.
	if err := toolcheck.CheckVersion(toolcheck.MinAmpelVersion); err != nil {
		return err
	}
	return nil
}
