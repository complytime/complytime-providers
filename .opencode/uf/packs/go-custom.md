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
