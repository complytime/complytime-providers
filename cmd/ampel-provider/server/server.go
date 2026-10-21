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
type ProviderServer struct{}

// New returns a new ProviderServer.
func New() *ProviderServer {
	return &ProviderServer{}
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
	logger := hclog.Default()

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

	if err := config.EnsureDirectories(); err != nil {
		return &provider.GenerateResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("directory setup failed: %v", err),
		}, nil
	}

	logger.Info("generating AMPEL policy")

	sourceDir, resolveErr := resolvePolicyDir(logger, req)
	if resolveErr != nil {
		return &provider.GenerateResponse{
			Success:      false,
			ErrorMessage: resolveErr.Error(),
		}, nil
	}
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
	outputDir := config.GeneratedDirForKey(contentKey)
	if err := os.MkdirAll(outputDir, 0750); err != nil {
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
		config.GeneratedDirPath(), genindex.TargetKey(req.TargetVariables["url"]), contentKey,
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
	logger := hclog.Default()

	if len(req.Targets) == 0 {
		return nil, fmt.Errorf("no targets provided")
	}

	if err := checkRequiredTools(logger); err != nil {
		return nil, err
	}

	if err := config.EnsureDirectories(); err != nil {
		return nil, fmt.Errorf("directory setup failed: %w", err)
	}

	logger.Info("scanning target repositories")

	resultsDir := config.ResultsDirPath()
	specDir := config.SpecDirPath()

	if err := scan.WriteSpecFiles(specDir); err != nil {
		return nil, fmt.Errorf("writing spec files: %w", err)
	}

	baseScanCfg := scan.ScanConfig{
		OutputDir: resultsDir,
		SpecDir:   specDir,
	}
	index, indexErr := genindex.ReadIndex(config.GeneratedDirPath())
	if indexErr != nil && !errors.Is(indexErr, os.ErrNotExist) {
		return nil, fmt.Errorf("reading generate index for target %q: %w", req.Targets[0].TargetID, indexErr)
	}
	hasIndex := indexErr == nil
	var legacyScanConfig *generate.ScanConfig
	if !hasIndex {
		logger.Warn("generate index absent; using legacy scan configuration fallback")
		legacyScanConfig, indexErr = generate.ReadScanConfig(config.ScanConfigDirPath())
		if indexErr != nil {
			logger.Warn("scan config not found, passing assessments will not be synthesized", "error", indexErr)
		}
	}
	scanConfigCache := make(map[string]*generate.ScanConfig)

	var repoResults []*results.PerRepoResult
	hostRemark := evidence.HostRemark()

	for _, target := range req.Targets {
		repoURL := target.Variables["url"]
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
		if err := validateTargetVariables(repoURL, branches, specs, accessToken, target.TargetID); err != nil {
			return nil, err
		}
		targetScanConfig, policyPath, err := resolveTargetArtifacts(
			target, index, hasIndex, legacyScanConfig, scanConfigCache,
		)
		if err != nil {
			return nil, err
		}
		scanCfg := baseScanCfg
		scanCfg.PolicyPath = policyPath

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

		for _, branch := range branches {
			for _, specRef := range specs {
				specPath := scan.ResolveSpecPath(specRef, scanCfg.SpecDir)
				logger.Info("scanning repository", "url", repoURL, "branch", branch, "spec", specRef)

				rawResult, err := scan.ScanRepository(repo, branch, specPath, scanCfg, ScanRunner)
				if err != nil {
					logger.Error("scan failed", "repo", repoURL, "branch", branch, "spec", specRef, "error", err)
					errResult := &results.PerRepoResult{
						Repository: repoURL,
						Branch:     branch,
						Status:     "error",
						Error:      err.Error(),
					}
					repoResults = append(repoResults, errResult)
					if writeErr := results.WritePerRepoResult(errResult, resultsDir); writeErr != nil {
						logger.Error("failed to write error result", "error", writeErr)
					}
					continue
				}

				parsed, err := results.ParseAmpelOutput(rawResult.Output, repoURL, branch)
				if err != nil {
					logger.Error("failed to parse scan output", "repo", repoURL, "error", err)
					errResult := &results.PerRepoResult{
						Repository: repoURL,
						Branch:     branch,
						Status:     "error",
						Error:      err.Error(),
					}
					repoResults = append(repoResults, errResult)
					if writeErr := results.WritePerRepoResult(errResult, resultsDir); writeErr != nil {
						logger.Error("failed to write error result", "error", writeErr)
					}
					continue
				}
				if targetScanConfig != nil {
					parsed.RequirementIDs = targetScanConfig.RequirementIDs
				}

				// All-SKIP detection: when every finding skipped, no policy's
				// declared predicate types matched the attestation. This is a
				// misconfiguration, not a valid scan result (Decision 3).
				// Clear findings so ToScanResponse routes the result to
				// operational errors (status "error" + no findings).
				if parsed.AllFindingsSkipped() {
					parsed.Status = "error"
					parsed.Error = "all policies were skipped: " +
						"no policies matched the attestation predicate type"
					parsed.Findings = nil
				}

				// Thread attestation paths, spec label, and host remark from scan result
				parsed.AmpelAttestationPath = rawResult.AmpelAttestationPath
				parsed.SnappyAttestationPath = rawResult.SnappyAttestationPath
				parsed.SpecLabel = scan.SanitizeSpecName(specRef)
				parsed.Remarks = hostRemark

				// Pre-compute digests for evidence (keeps ToScanResponse pure)
				if rawResult.AmpelAttestationPath != "" {
					d, digestErr := evidence.FileDigest(rawResult.AmpelAttestationPath)
					if digestErr != nil {
						return nil, fmt.Errorf(
							"evidence digest failed for %s: %w",
							rawResult.AmpelAttestationPath, digestErr,
						)
					}
					parsed.AmpelDigest = d
				}
				if rawResult.SnappyAttestationPath != "" {
					d, digestErr := evidence.FileDigest(rawResult.SnappyAttestationPath)
					if digestErr != nil {
						return nil, fmt.Errorf(
							"evidence digest failed for %s: %w",
							rawResult.SnappyAttestationPath, digestErr,
						)
					}
					parsed.SnappyDigest = d
				}

				repoResults = append(repoResults, parsed)
				if writeErr := results.WritePerRepoResult(parsed, resultsDir); writeErr != nil {
					logger.Error("failed to write result", "error", writeErr)
				}
			}
		}
	}

	scanResponse := results.ToScanResponse(repoResults, nil)
	logger.Info("scan complete", "repositories_scanned", len(repoResults))
	return scanResponse, nil
}

func resolveTargetArtifacts(
	target provider.Target,
	index genindex.GenerateIndex,
	hasIndex bool,
	legacyScanConfig *generate.ScanConfig,
	cache map[string]*generate.ScanConfig,
) (*generate.ScanConfig, string, error) {
	if !hasIndex {
		return legacyScanConfig, filepath.Join(config.GeneratedPolicyDirPath(), convert.PolicyFileName), nil
	}

	contentKey, ok := index.Targets[genindex.TargetKey(target.Variables["url"])]
	if !ok {
		return nil, "", fmt.Errorf("target %q is missing from generate index", target.TargetID)
	}
	if err := genindex.ValidateContentKey(contentKey, config.GeneratedDirPath()); err != nil {
		return nil, "", fmt.Errorf("validating content key for target %q: %w", target.TargetID, err)
	}
	if scanConfig, ok := cache[contentKey]; ok {
		return scanConfig, filepath.Join(config.GeneratedDirForKey(contentKey), convert.PolicyFileName), nil
	}

	scanConfig, err := generate.ReadScanConfig(config.GeneratedDirForKey(contentKey))
	if err != nil {
		return nil, "", fmt.Errorf("reading generated scan config for target %q: %w", target.TargetID, err)
	}
	cache[contentKey] = scanConfig
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
