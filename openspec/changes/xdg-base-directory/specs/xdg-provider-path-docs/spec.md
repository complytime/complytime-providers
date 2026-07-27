## ADDED Requirements

### Requirement: Documentation references XDG-compliant provider paths

All provider documentation files MUST reference `~/.local/share/complytime/providers/` as the user-scoped provider installation directory instead of the legacy `~/.complytime/providers/` path. This applies to installation instructions, troubleshooting guides, and path examples.

#### Scenario: Provider installation instructions use XDG data path

- **GIVEN** the provider documentation has been updated per this spec
- **WHEN** a user reads any provider README for installation instructions
- **THEN** the documented provider binary path MUST be `~/.local/share/complytime/providers/`

#### Scenario: Provider troubleshooting references use XDG data path

- **GIVEN** the provider documentation has been updated per this spec
- **WHEN** a user reads troubleshooting or cleanup instructions in any provider README
- **THEN** all provider directory references MUST use `~/.local/share/complytime/providers/`

### Requirement: Documentation references XDG-compliant data paths

All documentation files that reference user-scoped persistent data paths (state files, provider binaries) MUST use `~/.local/share/complytime/` as the base data directory instead of `~/.complytime/`.

#### Scenario: State file path uses XDG data directory

- **GIVEN** the provider documentation has been updated per this spec
- **WHEN** a user reads documentation referencing `state.json` location
- **THEN** the documented path MUST be `~/.local/share/complytime/state.json`

### Requirement: Documentation references XDG-compliant cache paths

All documentation files that reference downloaded artifact paths (policies, complypacks) MUST use `~/.cache/complytime/` as the base cache directory instead of `~/.complytime/`.

#### Scenario: Policy and complypack paths use XDG cache directory

- **GIVEN** the provider documentation has been updated per this spec
- **WHEN** a user reads documentation referencing policy or complypack storage locations
- **THEN** the documented paths MUST use `~/.cache/complytime/policies/` and `~/.cache/complytime/complypacks/` respectively

### Requirement: Workspace-local paths remain unchanged

Documentation MUST continue to reference `.complytime/` (without home directory prefix) for workspace-local project-scoped paths. These paths are not subject to XDG Base Directory conventions.

#### Scenario: Workspace config paths are unmodified

- **GIVEN** a documentation file references the per-project workspace directory
- **WHEN** the path is workspace-local (e.g., `.complytime/scan/`, `.complytime/ampel/`, `.complytime/complytime.yaml`)
- **THEN** the path MUST remain as `.complytime/` with no XDG prefix

### Requirement: Consistent tilde expansion in openscap-provider

The `expandPath()` function in `cmd/openscap-provider/config/config.go` MUST use `os.UserHomeDir()` for tilde expansion instead of `user.Current().HomeDir`.

#### Scenario: Tilde expansion under normal user

- **GIVEN** `$HOME` is set to `/home/alice`
- **WHEN** `expandPath("~/foo")` is called
- **THEN** the result MUST be `/home/alice/foo`

#### Scenario: Tilde expansion under sudo

- **GIVEN** `sudo` is used and `$HOME` is `/home/alice` (not `/root`)
- **WHEN** `expandPath("~/foo")` is called
- **THEN** the result MUST be `/home/alice/foo` (following `$HOME`, not the effective user's passwd entry)

#### Scenario: Tilde expansion when home directory is unavailable

- **GIVEN** `$HOME` is unset and no home directory can be determined
- **WHEN** `expandPath("~/foo")` is called
- **THEN** the function MUST return an error with a message indicating the home directory could not be determined

### Requirement: Scripts reference XDG-compliant provider paths

The devcontainer setup script (`.devcontainer/scripts/post-create.sh`) MUST install provider binaries to `~/.local/share/complytime/providers/` instead of the legacy `~/.complytime/providers/` path.

#### Scenario: Devcontainer provider installation uses XDG data path

- **GIVEN** the devcontainer post-create script runs during environment setup
- **WHEN** the script copies provider binaries to the user-scoped provider directory
- **THEN** the target path MUST be `~/.local/share/complytime/providers/`
