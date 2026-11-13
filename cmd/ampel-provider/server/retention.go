// SPDX-License-Identifier: Apache-2.0

package server

import (
	"errors"

	"github.com/complytime/complytime-providers/cmd/ampel-provider/config"
	"github.com/complytime/complytime-providers/cmd/ampel-provider/results"
	"github.com/complytime/complytime-providers/cmd/ampel-provider/scan"
	"github.com/complytime/complytime-providers/internal/evidence"
)

type evaluationOptions struct {
	targetID      string
	repository    scan.RepoTarget
	scanConfig    scan.ScanConfig
	configuration evidence.ConfigurationContext
	storage       evidence.StorageOptions
	runner        scan.CommandRunner
	remarks       string
}

func (s *ProviderServer) scanEvaluation(
	options evaluationOptions, branch, specRef string,
) (result *results.PerRepoResult, err error) {
	repoURL := options.repository.URL
	s.logger.Info("scanning repository", "url", repoURL, "branch", branch, "spec", specRef)
	evaluation, err := s.allocate(options.storage,
		evidence.StripURLScheme(evidence.PublicContext(repoURL)), branch, specRef)
	if err != nil {
		var verification *evidence.VerificationError
		if errors.As(err, &verification) {
			return nil, err
		}
		return &results.PerRepoResult{
			Repository: repoURL, Branch: branch, Status: "error", Error: err.Error(),
		}, nil
	}
	// Handle lifetime belongs to one evaluation, including tool/parse failures.
	// A close failure must suppress the response rather than become a target error.
	defer func() {
		err = errors.Join(err, evaluation.Close())
		if err != nil {
			result = nil
		}
	}()
	options.scanConfig.Evaluation = evaluation
	options.configuration.SpecReference = specRef
	options.configuration.SpecPath = scan.ResolveSpecPath(specRef, options.scanConfig.SpecDir)
	result, err = s.executeEvaluation(options, branch)
	if err != nil {
		return &results.PerRepoResult{
			Repository: repoURL, Branch: branch, Status: "error",
			Error: publicOperationalError(err, options.repository.AccessToken),
		}, nil
	}
	if err := s.retainEvaluation(options, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *ProviderServer) executeEvaluation(
	options evaluationOptions, branch string,
) (*results.PerRepoResult, error) {
	raw, err := scan.ScanRepository(options.repository, branch, options.configuration.SpecPath,
		options.scanConfig, options.runner)
	if err != nil {
		s.logger.Error("scan failed", "repo", options.repository.URL, "branch", branch,
			"spec", options.configuration.SpecReference, "error", err)
		return nil, err
	}
	parsed, err := results.ParseAmpelOutput(raw.Output, options.repository.URL, branch)
	if err != nil {
		s.logger.Error("failed to parse scan output", "repo", options.repository.URL, "error", err)
		return nil, err
	}
	parsed.RequirementIDs = options.configuration.RequirementIDs
	// No matched predicate types is a misconfiguration, not a passing assessment.
	// Clear findings so ToScanResponse routes the result to operational errors.
	if parsed.AllFindingsSkipped() {
		parsed.Status = "error"
		parsed.Error = "all policies were skipped: " +
			"no policies matched the attestation predicate type"
		parsed.Findings = nil
	}
	parsed.ScannedAt = options.scanConfig.Evaluation.CollectionTime
	parsed.Repository = evidence.PublicContext(parsed.Repository)
	parsed.SpecLabel = evidence.PublicContext(options.configuration.SpecReference)
	return parsed, nil
}

func (s *ProviderServer) retainEvaluation(options evaluationOptions, parsed *results.PerRepoResult) error {
	evaluation := options.scanConfig.Evaluation
	if err := finalizeEvaluation(evaluation, parsed, options.remarks); err != nil {
		return err
	}
	evaluation.RecordMetadata(s.logger, config.ProviderDir, evidence.TargetContext{
		TargetKey: evidence.RemoteTargetKey(options.repository.URL), TargetID: options.targetID,
		URL: options.repository.URL, Branch: parsed.Branch,
	}, options.configuration)
	return evaluation.Finalize()
}
