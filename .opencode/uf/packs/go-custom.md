---
pack_id: go-custom
language: Go
version: 1.0.0
---
<!-- scaffolded by uf v0.21.0 -->

# Custom Rules: Go

Project-specific Go conventions that extend the canonical
Go convention pack. Rules in this file are loaded alongside
`go.md` by Cobalt-Crush (during implementation) and
all Divisor persona agents (during review).

Use the `CR-NNN` prefix for all custom rules. Use `[MUST]`,
`[SHOULD]`, or `[MAY]` severity indicators per RFC 2119.

## Custom Rules

- **CR-001** [MUST] Override TC-001/TC-002: use
  `github.com/stretchr/testify` (assert/require) for test
  assertions. All 14 existing test files use testify. Do NOT
  flag testify usage as a violation.
- **CR-002** [MUST] Override CS-008: use
  `github.com/hashicorp/go-hclog` for logging (provided by
  the go-plugin dependency). Do NOT use charmbracelet/log.
- **CR-003** [MUST] Override CS-009: this project does not use
  cobra. Providers are gRPC subprocess plugins managed by
  `hashicorp/go-plugin` via `provider.Serve()`. There is no
  CLI command routing layer.
- **CR-004** [MUST] Override AP-001/AP-002: providers use the
  `server.New()` + `provider.Serve()` pattern, not
  Options/Result/Run(). The plugin framework owns the
  entrypoint lifecycle.
- **CR-005** [MUST] Override AP-004: providers have no static
  assets to embed. Do NOT require `embed.FS` usage.
- **CR-006** [MUST] Scoped override of Go pack SC-005:
  - OPA working directories containing private clones MUST
    use `0o700`.
  - Retained evaluation directories MUST use `0o700`.
  - Retained evidence and supporting files MUST use `0o600`.
  - Outside these scopes, SC-005 remains unchanged:
    `0o644` for regular files and `0o755` for executable
    scripts and directories.
  - This override MUST NOT waive path validation, ownership,
    secret protection, testing, or security and review
    requirements.

## SC-005 Maintainer Decision

The maintainer decision is to preserve private clones and
retained evaluation evidence through the narrowly scoped
permission modes in CR-006, rather than broaden access to
satisfy SC-005's generic defaults.

- **Authorizing maintainer:** marcusburghardt
- **Decision date:** 2026-11-16
- **Durable authorization:** [Issue #242](https://github.com/complytime/complytime-providers/issues/242)
- **Affected implementation:**
  [PR #238](https://github.com/complytime/complytime-providers/pull/238).

The authorization applies only to CR-006's listed scopes.
All other convention rules, security requirements, review
gates, and merge requirements remain unchanged.
