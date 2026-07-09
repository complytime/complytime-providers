## ADDED Requirements

### Requirement: LoadGranularPolicies walks subdirectories
The `LoadGranularPolicies` function SHALL recursively walk all subdirectories under the given root directory to find policy JSON files. The implementation MUST use `filepath.WalkDir` (not `filepath.Walk`) to ensure correct symlink handling.

#### Scenario: Policies in subdirectories are loaded
- **GIVEN** a directory containing a `branch-protection/` subdirectory with valid JSON policy files
- **WHEN** `LoadGranularPolicies` is called with the parent directory
- **THEN** all policy files from the subdirectory are loaded into the result map

#### Scenario: Policies in the root directory are still loaded
- **GIVEN** a directory containing JSON policy files directly (no subdirectories)
- **WHEN** `LoadGranularPolicies` is called with that directory
- **THEN** all policy files are loaded, preserving backward compatibility with flat directory layouts

#### Scenario: Nested subdirectories are walked
- **GIVEN** a directory containing `level1/level2/` subdirectories with JSON policy files at each level
- **WHEN** `LoadGranularPolicies` is called with the root directory
- **THEN** policy files at any depth are loaded

#### Scenario: Empty subdirectory is silently skipped
- **GIVEN** a directory containing an empty subdirectory alongside other subdirectories with policy files
- **WHEN** `LoadGranularPolicies` is called
- **THEN** no error is returned and policies from other subdirectories are loaded normally

### Requirement: Existing filter rules apply at all depths
The existing filters (skip non-JSON files, skip `PolicyFileName`, require non-empty `id`) SHALL apply to files at any directory depth.

#### Scenario: Non-JSON files in subdirectories are skipped
- **GIVEN** a subdirectory containing `.txt` and `.yaml` files alongside JSON policy files
- **WHEN** `LoadGranularPolicies` is called
- **THEN** only the JSON files are loaded

#### Scenario: PolicyFileName is skipped in subdirectories
- **GIVEN** a subdirectory containing a file named `complytime-ampel-policy.json` alongside other valid policy files
- **WHEN** `LoadGranularPolicies` is called
- **THEN** the `complytime-ampel-policy.json` file is skipped and not included in the result map

#### Scenario: Empty ID in subdirectory file produces error
- **GIVEN** a subdirectory containing a JSON policy file with an empty `id` field
- **WHEN** `LoadGranularPolicies` is called
- **THEN** an error is returned identifying the file path

### Requirement: Duplicate policy IDs produce an error
When two or more files at any depth contain the same `id` field value, `LoadGranularPolicies` SHALL return an error identifying both file paths and the duplicate ID.

#### Scenario: Duplicate IDs across subdirectories
- **GIVEN** two JSON policy files in different subdirectories with the same `id` value
- **WHEN** `LoadGranularPolicies` is called
- **THEN** an error is returned naming both file paths and the duplicate ID

#### Scenario: Duplicate IDs in same directory
- **GIVEN** two JSON policy files in the same directory with the same `id` value
- **WHEN** `LoadGranularPolicies` is called
- **THEN** an error is returned naming both file paths and the duplicate ID

### Requirement: Symlink safety
The `LoadGranularPolicies` function SHALL NOT follow symbolic links to directories. Symbolic links to files SHALL be skipped without error.

#### Scenario: Symlink to directory is not followed
- **GIVEN** a policy directory containing a symbolic link pointing to another directory
- **WHEN** `LoadGranularPolicies` is called
- **THEN** the symlink target directory is not walked and its contents are not loaded

#### Scenario: Symlink to file is skipped
- **GIVEN** a policy directory containing a symbolic link pointing to a JSON file
- **WHEN** `LoadGranularPolicies` is called
- **THEN** the symlink is skipped without error

### Requirement: Errors halt the walk
Any file read or parse error encountered during the directory walk SHALL halt processing and return the error.

#### Scenario: Malformed JSON in subdirectory halts loading
- **GIVEN** a subdirectory containing a malformed JSON file (invalid syntax)
- **WHEN** `LoadGranularPolicies` is called
- **THEN** an error is returned and partial results are not returned

#### Scenario: Unreadable file halts loading
- **GIVEN** a file in a subdirectory with permissions set to prevent reading
- **WHEN** `LoadGranularPolicies` is called
- **THEN** an error is returned identifying the unreadable file
