## REMOVED Requirements

### Requirement: OpenSCAP provider declares export support
**Reason**: The upstream complyctl SDK removed the `SupportsExport` field from `DescribeResponse` and the `Exporter` interface entirely (complyctl PR #617, issue #606). The export subsystem was speculative infrastructure; export will be redesigned when the backend shape is known.
**Migration**: No action required. complyctl no longer calls Export on any provider. Remove any local tooling that depends on `SupportsExport: true` in the OpenSCAP provider's Describe response.

### Requirement: OpenSCAP provider implements Export RPC
**Reason**: The `provider.Exporter` interface, `ExportRequest`, `ExportResponse`, and `CollectorConfig` types no longer exist in the complyctl SDK. The Export RPC has been removed from the protobuf definitions.
**Migration**: No action required. complyctl no longer invokes the Export RPC. The ARF-to-GemaraEvidence conversion logic is deleted along with the ProofWatch integration. When export is redesigned upstream, a new implementation will be created from scratch.

### Requirement: OpenSCAP evidence carries correct Gemara attributes
**Reason**: With the Export RPC removed, there is no emission path for GemaraEvidence records. The Gemara attribute mapping logic is deleted.
**Migration**: No action required. Evidence attribute mappings will be redefined when the export mechanism is redesigned.
