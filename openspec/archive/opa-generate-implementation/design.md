## Architecture

Follow the AMPEL provider's declarative mapping pattern. The mapping between Gemara RequirementIDs and Rego namespaces is declared in a sidecar JSON file shipped inside the OCI policy bundle, owned by the policy author.

```
                complyctl generate --policy-id k8s-policy
                              │
                    complyctl routes by executor.id: "opa"
                              │
                              ▼
                    OPA Provider Generate RPC
                    ┌─────────────────────────────┐
                    │ 1. Validate req.Configuration│
                    │ 2. Check tools (conftest,git)│
                    │ 3. Ensure directories        │
                    │ 4. Pull OCI bundle            │
                    │ 5. Read complytime-mapping   │
                    │    .json from bundle dir      │
                    │ 6. Match RequirementIDs       │
                    │    (exact map lookup)         │
                    │ 7. Write scan-config.json     │
                    └──────────────┬──────────────┘
                                   │
                                   ▼
                    .complytime/opa/generated/
                      scan-config.json
```

## Mapping File Format

Policy bundle authors include `complytime-mapping.json` in their OCI bundle:

```json
{
  "version": "1",
  "mappings": [
    {
      "id": "kubernetes.run_as_root",
      "requirement_id": "CIS-K8S-5.2.6"
    },
    {
      "id": "kubernetes.resource_limits",
      "requirement_id": "CIS-K8S-5.4.1"
    }
  ]
}
```

- `id`: The Rego package namespace that identifies this policy. Serves as the semantic, benchmark-agnostic identity (equivalent to AMPEL's granular policy `id` field). Also used as the conftest `--namespace` filter value.
- `requirement_id`: Must match the Gemara assessment plan's `RequirementID` exactly (like AMPEL's direct map lookup).
- `version`: Schema version for forward compatibility.

### Why This Format

The AMPEL provider established the precedent: each granular policy file declares its own `"id"` field as the primary identity, and `MatchPolicies` does exact map lookup (`granular[reqID]`). The OPA mapping file follows the same structural principle -- the `id` field is the semantic identity of the policy, and the `requirement_id` field maps it to a Gemara assessment plan entry.

The Rego namespace is the natural choice for `id` because policy authors already choose descriptive package names (`kubernetes.run_as_root`, `docker.network_encryption`). These serve the same readability purpose as AMPEL's semantic slugs (`require-pull-request`, `block-force-push`). Using the namespace as `id` avoids introducing a third identifier and keeps the mapping file's primary key self-documenting.

The OpenSCAP approach (prefix concatenation with XCCDF rule naming) is not suitable because Rego has no ecosystem-wide naming standard like ComplianceAsCode/SSG provides for XCCDF.

### Design Decision: Rename `namespace` to `id`

**Context**: The original mapping entry schema used `requirement_id` + `namespace` fields. PR #32 (ampel policy ID refactoring) moved AMPEL's granular policy IDs from benchmark-coupled `BP-X.YY` format to semantic, benchmark-agnostic slugs. This established that `id` is the primary identity field across complytime providers, and that identity should be semantic rather than benchmark-coupled.

**Decision**: Use `id` instead of `namespace` as the field name for the Rego package identifier in mapping entries. The `id` field doubles as the conftest `--namespace` value since the Rego package name *is* the semantic identity.

**Rationale**:
- Consistent with AMPEL's `id`-first pattern across providers
- The Rego namespace is already semantic and human-readable
- No new identifier needed -- `id` *is* the namespace
- Policy authors see `id` as the primary field when reading the mapping file, matching the experience of reading AMPEL granular policy files
- The reverse mapping in scan-config.json keys on `id` values, making result resolution self-documenting

**Impact**: Field rename only. No change to match logic, validation, scan config format, or fallback behavior. The `id` value is passed directly to `conftest --namespace` flags.

## Generation Artifact Format

Generate writes `.complytime/opa/generated/scan-config.json`:

```json
{
  "ids": ["kubernetes.resource_limits", "kubernetes.run_as_root"],
  "reverse_mapping": {
    "kubernetes.run_as_root": "CIS-K8S-5.2.6",
    "kubernetes.resource_limits": "CIS-K8S-5.4.1"
  },
  "bundle_dir": ".complytime/opa/policy/ghcr.io_myorg_opa-policies_v1.0",
  "generated_at": "2026-05-29T14:30:00Z"
}
```

- `ids`: List of matched mapping entry IDs (Rego namespaces) to pass as `conftest --namespace` flags. `null` means use `--all-namespaces` (fallback).
- `reverse_mapping`: Maps IDs (Rego-derived, from `deriveIDFromQuery`) back to Gemara RequirementIDs. `null` means use `deriveIDFromQuery` as-is (fallback).
- `bundle_dir`: Path to the pulled bundle for Scan to reference.
- `generated_at`: ISO 8601 timestamp for freshness tracking.

## Generate Flow

```go
func (s *ProviderServer) Generate(ctx context.Context, req *provider.GenerateRequest) (*provider.GenerateResponse, error) {
    // 1. Validate
    if len(req.Configuration) == 0 {
        return &provider.GenerateResponse{
            Success: false, ErrorMessage: "no assessment configurations provided",
        }, nil
    }

    // 2. Check tools
    missing, err := s.opts.ToolChecker()
    // return error if tools missing

    // 3. Ensure directories (including new GeneratedDirPath)
    cfg := config.NewConfig(s.opts.WorkspaceDir)
    cfg.EnsureDirectories()

    // 4. Pull bundle
    vars := mergeVariables(req.GlobalVariables, req.TargetVariables)
    bundleRef := vars["opa_bundle_ref"]
    if bundleRef == "" {
        return &provider.GenerateResponse{
            Success: false, ErrorMessage: "opa_bundle_ref variable is required",
        }, nil
    }
    policyDir := cfg.PolicyDirForBundle(bundleRef)
    scan.PullBundle(bundleRef, policyDir, s.opts.Runner)

    // 5. Read mapping file
    mapping, err := generate.LoadMapping(policyDir)
    if err != nil {
        logger.Warn("no complytime-mapping.json found, skipping requirement filtering")
        generate.WriteScanConfig(cfg.GeneratedDirPath(), nil, nil, policyDir)
        return &provider.GenerateResponse{Success: true}, nil
    }

    // 6. Match RequirementIDs to mapping IDs (exact lookup, like AMPEL)
    ids, reverseMap, warnings := generate.MatchRequirements(req.Configuration, mapping)
    for _, w := range warnings {
        logger.Warn(w)
    }

    // 7. Write scan-config.json (ids are used as conftest --namespace values)
    generate.WriteScanConfig(cfg.GeneratedDirPath(), ids, reverseMap, policyDir)

    return &provider.GenerateResponse{Success: true}, nil
}
```

## Scan Changes

Scan reads `scan-config.json` before evaluating. The change is localized to `processTarget`:

```go
// In processTarget, after bundle resolution:
scanCfg, err := generate.ReadScanConfig(cfg.GeneratedDirPath())
if err == nil && scanCfg.IDs != nil {
    // IDs are Rego namespaces, passed directly to conftest --namespace
    raw, err = scan.EvalPolicyWithNamespaces(inputPath, policyDir, scanCfg.IDs, s.opts.Runner)
} else {
    raw, err = scan.EvalPolicy(inputPath, policyDir, s.opts.Runner)
}
```

New function in `scan/scan.go`:

```go
func constructConftestTestCommandWithNamespaces(inputPath, policyDir string, namespaces []string) (string, []string) {
    args := []string{"test", inputPath, "--policy", policyDir, "--output", "json", "--no-fail"}
    for _, ns := range namespaces {
        args = append(args, "--namespace", ns)
    }
    return "conftest", args
}
```

## Results Changes

`ToScanResponse` accepts an optional reverse mapping:

```go
func ToScanResponse(targetResults []*PerTargetResult, reverseMap map[string]string) *provider.ScanResponse {
    // ... existing grouping logic ...
    for _, f := range tr.Findings {
        reqID := f.RequirementID
        if reverseMap != nil {
            if gemaraID, ok := reverseMap[reqID]; ok {
                reqID = gemaraID
            }
        }
        // group by reqID ...
    }
}
```

The reverse mapping is passed from `processTarget` through `evalAndParse` to `ToScanResponse`. When the scan config is absent or has no mapping, `nil` is passed and behavior is identical to today.

## Fallback Behavior

| Bundle has mapping? | Generate writes | Scan uses | Result IDs |
|---------------------|----------------|-----------|------------|
| Yes | `ids: [...]`, `reverse_mapping: {...}` | `--namespace id1 --namespace id2` | Gemara IDs |
| No | `ids: null`, `reverse_mapping: null` | `--all-namespaces` | Rego-derived |

Both modes are first-class. The fallback is permanent, not deprecated.

## Workspace Layout After Generate

```
.complytime/opa/
  policy/
    ghcr.io_myorg_opa-policies_v1.0/
      kubernetes/run_as_root.rego
      kubernetes/resource_limits.rego
      docker/network_encryption.rego
      complytime-mapping.json          ← read by Generate
  generated/                           ← NEW directory
    scan-config.json                   ← written by Generate, read by Scan
  repos/
  results/
```

## Dependencies

- No new Go module dependencies.
- No changes to `complyctl` or the plugin protobuf interface.
- `conftest --namespace` flag is stable and documented.

## Testing Strategy

- Unit tests per package (generate, scan, results, config, server).
- Integration-level test: Generate with mapping → Scan with namespace filtering → verify Gemara IDs in response.
- Fallback test: Generate without mapping → Scan with `--all-namespaces` → verify identical behavior to current implementation.
- All tests use dependency injection via existing interfaces (`CommandRunner`, `ToolChecker`, `DataLoader`).
