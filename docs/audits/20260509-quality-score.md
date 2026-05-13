# Quality Score Audit — 2026-05-09

## Summary

| Metric | Value |
|--------|-------|
| **Global Score** | 7.1 / 10 |
| **Grade** | C |
| **Date** | 2026-05-09 |
| **Scope** | Full codebase |
| **Go Version** | 1.25.0 (go.mod) / 1.26.2 (runtime) |
| **Tests** | 245 functions, 19/20 testable packages |
| **Linter Issues** | 7 |
| **Build** | Clean (3 binaries) |

## Scorecard

| # | Category | Score | Weight | Weighted | Issues |
|---|----------|-------|--------|----------|--------|
| 1 | Bugs & Correctness | 6/10 | 20% | 1.20 | 1 critical, 2 high |
| 2 | Security | 6/10 | 15% | 0.90 | 1 critical, 2 high, 3 medium |
| 3 | Dead Code & Hygiene | 8/10 | 10% | 0.80 | 7 formatting |
| 4 | Test Coverage & Quality | 8/10 | 20% | 1.60 | Minor flaky risks |
| 5 | Code Complexity & Consistency | 7/10 | 15% | 1.05 | 5 long fns, 3 duplication |
| 6 | Dependencies & Config | 7/10 | 10% | 0.70 | Version mismatch, no config docs |
| 7 | Build & Tooling | 8/10 | 10% | 0.80 | 7 linter warnings |
| | **GLOBAL** | **7.1/10** | | **7.05** | **Grade C** |

## Findings

### CRITICAL

| ID | Category | Location | Description |
|----|----------|----------|-------------|
| C1 | Bugs | `cmd/openscap-provider/xccdf/datastream.go:398` | Nil pointer dereference: `benchmarkDom` from `SelectElement` used without nil check |
| C2 | Security | `cmd/openscap-provider/xccdf/datastream.go:48` | XML parsing via `xmlquery.Parse` without explicit XXE protection |

### HIGH

| ID | Category | Location | Description |
|----|----------|----------|-------------|
| H1 | Bugs | `cmd/openscap-provider/config/config.go:108` | EOF check uses `err.Error() == "EOF"` instead of `errors.Is(err, io.EOF)` |
| H2 | Bugs | `cmd/opa-provider/server/server.go:50` | Error from `toolcheck.CheckTools()` discarded with `_` |
| H3 | Security | `cmd/ampel-provider/scan/scan.go:267` | Access token injected into env without format validation |
| H4 | Security | `cmd/openscap-provider/config/config.go:88` | `SanitizePath` uses `filepath.Clean` but no bounds check against workspace root |
| H5 | Complexity | `cmd/ampel-provider/server/server.go:118` | `Scan` function is 117 lines with high cyclomatic complexity |
| H6 | Complexity | ampel/server + opa/server | Duplicated `validateTargetVariables` — security-critical validation repeated |

### MEDIUM

| ID | Category | Location | Description |
|----|----------|----------|-------------|
| M1 | Security | `cmd/openscap-provider/xccdf/datastream.go:101` | XPath expression built with unescaped user input |
| M2 | Security | `cmd/ampel-provider/server/server.go:280` | Substring-based path traversal check (`Contains("..")`) is bypassable |
| M3 | Security | `cmd/openscap-provider/config/config_test.go:81` | Test uses `os.ModePerm` (0o777) for directory creation |
| M4 | Complexity | ampel/server + opa/server | Duplicated `splitCSV` utility across providers |
| M5 | Complexity | ampel/scan + opa/scan | Duplicated `buildTokenEnv` with inconsistent signatures |
| M6 | Build | Multiple files | 7 linter violations: 3 goimports, 3 gosec (false positive), 1 errcheck |

### LOW

| ID | Category | Location | Description |
|----|----------|----------|-------------|
| L1 | Complexity | Multiple providers | Inconsistent error message tone (imperative vs passive) |
| L2 | Complexity | Multiple providers | Inconsistent logger acquisition pattern (`hclog.Default()` vs stored variable) |
| L3 | Deps | `go.mod` | Go version mismatch: 1.25.0 in go.mod, 1.26.2 runtime |
| L4 | Deps | Project-wide | No user-facing configuration documentation |

## Category Details

### 1. Bugs & Correctness (6/10)

The nil pointer dereference at `datastream.go:398` is the most critical finding. `SelectElement` can return nil, and the result is used on the next line without a nil guard. The string-based EOF comparison at `config.go:108` is fragile — wrapped errors will not match. The discarded error at `server.go:50` in the OPA provider silently swallows tool availability failures.

### 2. Security (6/10)

The XXE concern at `datastream.go:48` is mitigated by Go's `encoding/xml` disabling external entities by default, but `antchfx/xmlquery` sits on top and the protection should be explicitly verified. Path traversal defense in `SanitizePath` lacks a bounds check — `filepath.Clean` normalizes but doesn't constrain. The `validateTargetVariables` substring check for `..` is a weak defense that can be bypassed.

### 3. Dead Code & Hygiene (8/10)

No dead code, no commented blocks, no stale TODOs. The only issues are 7 import ordering violations (goimports). This is a clean codebase.

### 4. Test Coverage & Quality (8/10)

Strong coverage: 245 tests across 19 packages. Table-driven tests in 11/20 files. Testify assertions are meaningful. Minor risks: `os.Chdir` in tests without parallel safety, some tests use basic `if err != nil` instead of `require`.

### 5. Code Complexity & Consistency (7/10)

The `Scan` function in ampel/server is 117 lines — the largest in the codebase. Three utility functions are duplicated between ampel and opa providers. The duplication of `validateTargetVariables` is particularly concerning because it's security-critical validation logic that could diverge.

### 6. Dependencies & Configuration (7/10)

Minimal dependency footprint (4 direct). No known critical vulnerabilities. Go version in go.mod should be updated. User-facing configuration documentation is absent.

### 7. Build & Tooling (8/10)

Build is clean. CI is comprehensive (6 workflows). 7 linter issues: 3 are gosec false positives (test fixture variable names matching credential patterns), 3 are goimports ordering, 1 is an unchecked error in a test cleanup.

## Trend

First audit — no trend data available.
