## ADDED Requirements

### Requirement: Generate resolves complypack content path with correct precedence
The ampel provider's `Generate()` RPC SHALL resolve the granular policy source directory using the following precedence order:
1. `req.ComplypackContentPath` (highest priority)
2. `req.GlobalVariables["ampel_policy_dir"]`
3. `config.GranularPolicyDirPath()` (default)

The resolved directory SHALL be passed to `convert.LoadGranularPolicies()` without modification to the convert package.

#### Scenario: Complypack content path is a directory
- **WHEN** `req.ComplypackContentPath` is set to a valid directory path containing granular policy JSON files
- **THEN** `Generate()` SHALL use that directory as the policy source, ignoring `ampel_policy_dir` and the default path

#### Scenario: Complypack content path is a tar.gz archive
- **WHEN** `req.ComplypackContentPath` is set to a path ending in a `.tar.gz` file
- **THEN** `Generate()` SHALL extract the archive to a sibling `content/` directory and use the extracted directory as the policy source

#### Scenario: Complypack content path takes precedence over ampel_policy_dir
- **WHEN** both `req.ComplypackContentPath` and `req.GlobalVariables["ampel_policy_dir"]` are set
- **THEN** `Generate()` SHALL use the complypack content path and ignore `ampel_policy_dir`

#### Scenario: Fallback to ampel_policy_dir when no complypack path
- **WHEN** `req.ComplypackContentPath` is empty and `req.GlobalVariables["ampel_policy_dir"]` is set
- **THEN** `Generate()` SHALL use the `ampel_policy_dir` value as the policy source (backward-compatible behavior)

#### Scenario: Fallback to default path when no overrides
- **WHEN** both `req.ComplypackContentPath` and `ampel_policy_dir` are empty
- **THEN** `Generate()` SHALL use `config.GranularPolicyDirPath()` as the policy source (backward-compatible behavior)

### Requirement: Secure tar.gz extraction with zip-slip protection
The tar.gz extraction function SHALL reject malicious archive entries that attempt path traversal, symlink attacks, or decompression bombs.

#### Scenario: Path traversal in tar entry is rejected
- **WHEN** a tar archive contains an entry with `../` or an absolute path in its name
- **THEN** extraction SHALL return an error and SHALL NOT write the entry to disk

#### Scenario: Symlinks in tar entry are rejected
- **WHEN** a tar archive contains a symlink or hard link entry
- **THEN** extraction SHALL return an error and SHALL NOT create the link

#### Scenario: Extracted file size is capped
- **WHEN** a tar archive contains a file exceeding 100 MB
- **THEN** extraction SHALL return an error after writing at most 100 MB

#### Scenario: Extracted files have restricted permissions
- **WHEN** a tar archive is successfully extracted
- **THEN** all extracted files SHALL have mode `0600` and directories SHALL have mode `0750`

### Requirement: Idempotent complypack extraction
When `ComplypackContentPath` points to a tar.gz archive, extraction SHALL be idempotent: if the sibling `content/` directory already exists from a prior extraction, it SHALL be reused without re-extracting.

#### Scenario: Previously extracted content is reused
- **WHEN** `ComplypackContentPath` points to a tar.gz archive and a `content/` directory already exists alongside the archive
- **THEN** `resolveComplypackPath()` SHALL return the existing `content/` directory without performing extraction

#### Scenario: Missing content directory triggers extraction
- **WHEN** `ComplypackContentPath` points to a tar.gz archive and no `content/` directory exists alongside it
- **THEN** `resolveComplypackPath()` SHALL extract the archive to create the `content/` directory

### Requirement: Generate returns error for invalid complypack path
When `ComplypackContentPath` is set but points to a non-existent path or an invalid archive, `Generate()` SHALL return a failure response with a descriptive error message.

#### Scenario: Non-existent complypack path
- **WHEN** `req.ComplypackContentPath` is set to a path that does not exist
- **THEN** `Generate()` SHALL return `Success: false` with an error message indicating the path was not found

#### Scenario: Corrupt tar.gz archive
- **WHEN** `req.ComplypackContentPath` points to a file that is not a valid gzip-compressed tar archive
- **THEN** `Generate()` SHALL return `Success: false` with an error message indicating extraction failed
