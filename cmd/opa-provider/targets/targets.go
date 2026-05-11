// SPDX-License-Identifier: Apache-2.0

package targets

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// ParseRepoURL extracts the hosting platform, organization, and repository
// name from a repository URL. The URL must use HTTPS.
//
// If platformHint is non-empty, it overrides hostname-based platform detection.
func ParseRepoURL(repoURL, platformHint string) (platform, org, repo string, err error) {
	if repoURL == "" {
		return "", "", "", fmt.Errorf("repository URL is empty")
	}

	parsed, err := url.Parse(repoURL)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid URL %q: %w", repoURL, err)
	}

	if parsed.Scheme != "https" {
		return "", "", "", fmt.Errorf("URL %q must use HTTPS scheme", repoURL)
	}

	path := strings.Trim(parsed.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] == "" {
		return "", "", "", fmt.Errorf("URL %q must contain org/repo path", repoURL)
	}

	if platformHint != "" {
		platform = platformHint
	} else {
		host := strings.ToLower(parsed.Hostname())
		if strings.Contains(host, "github.com") {
			platform = "github"
		} else if strings.Contains(host, "gitlab.com") {
			platform = "gitlab"
		} else {
			return "", "", "", fmt.Errorf(
				"URL %q: unknown host (set 'platform' variable for self-hosted instances)", repoURL,
			)
		}
	}

	if platform == "gitlab" && len(parts) > 2 {
		org = strings.Join(parts[:len(parts)-1], "/")
		repo = parts[len(parts)-1]
	} else {
		org = parts[0]
		repo = parts[1]
	}

	return platform, org, repo, nil
}

// SanitizeRepoURL converts a repository URL into a filesystem-safe name
// by stripping the scheme and replacing special characters with hyphens.
func SanitizeRepoURL(repoURL string) string {
	name := repoURL
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(name, prefix) {
			name = name[len(prefix):]
			break
		}
	}
	var result []rune
	for _, r := range name {
		if r == '/' || r == '.' || r == ':' {
			result = append(result, '-')
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

// RepoDisplayName extracts the "org/repo" portion from a repository URL.
func RepoDisplayName(repoURL string) string {
	_, org, repo, err := ParseRepoURL(repoURL, "")
	if err != nil {
		return repoURL
	}
	return org + "/" + repo
}

// ValidateInputPath checks that a local input path exists and does not
// contain directory traversal sequences.
func ValidateInputPath(inputPath string) error {
	if strings.Contains(inputPath, "..") {
		return fmt.Errorf("input path %q contains directory traversal", inputPath)
	}
	if _, err := os.Stat(inputPath); err != nil {
		return fmt.Errorf("input path %q does not exist: %w", inputPath, err)
	}
	return nil
}
