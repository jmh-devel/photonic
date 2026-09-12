# Darktable Enterprise Vault and Multi-Computer Research

## Status

Preserved discovery notes and research direction. This document is not an implementation decision and does not authorize migration or deletion of photographs.

## Validated Findings

- Darktable 5.4.1 is installed on Yoda with Lua and OpenCL support.
- Darktable's supported **local copies** workflow can cache selected vault originals for disconnected work. On reconnection, **resync local copy** writes edit metadata back and removes the managed cache copy.
- XMP sidecars are the supported portable representation of edit history and metadata outside the local catalog.
- Darktable does not watch the filesystem for new assets.
- Darktable does not provide a supported shared catalog, multi-computer database synchronization, or conflict-resolution mechanism.
- A live darktable SQLite catalog should not be placed on NFS or synchronized bidirectionally.
- Photonic already has useful boundaries: CLI processing, darktable CLI invocation, astro/timelapse/panorama orchestration, filesystem events, and read-only catalog inspection.
- Photonic lacks a production-grade transactional ingest state machine, checksum-gated cleanup, durable retry/resume, and authoritative asset manifest.

## Claims Rejected from the Prior Assessment

No evidence supports the described native darktable database synchronization, Git LFS integration, remote history manager, vault synchronization plugin, Photonic batch-import plugin, or the sample `PhotonicImport`, `PhotonicProcessor`, and `DarktablePhotonicSync` APIs. Those names are proposals or inventions, not installed capabilities.

Git LFS is not an appropriate catalog or vault synchronization mechanism for RAW archives. It does not solve concurrent metadata changes, database locking, backup policy, cache lifecycle, or offline reconciliation.

## Current Topology and Audit Gap

- JMH NFS roots observed: `/mnt/jmh/data/Photography`, `/mnt/jmh/data/Darktable`, and `/mnt/jmh/data/yoda-photography-staging`.
- The active darktable catalog substantially references `/data/Photography`, with some duplicated-looking `/Photography/Photography/` paths.
- A prior full-root scan was terminated by SIGTERM and produced no trustworthy complete inventory.

No local source may be expunged until a resumable reconciliation proves which originals and XMP files exist on each side, identifies duplicates by digest, and confirms independent backup coverage.

## Recommended Near-Term Boundary

### Photonic owns ingest and processing orchestration

Photonic should discover camera/staging assets, plan canonical destinations, copy into a temporary vault location, verify full-file checksums, atomically promote assets, record a durable manifest, request supported darktable import, and gate source cleanup on explicit policy.

Suggested lifecycle:

`discovered -> copying -> vault_verified -> backup_verified -> imported -> cleanup_eligible -> cleaned`

Failures remain durable and retryable. A disappearing NFS mount must never turn a copy failure into source deletion.

### Darktable owns creative edits and its catalog

Keep `library.db` and `data.db` local. Keep originals and XMP sidecars in canonical vault paths. Use native local copies for travel. Photonic may read darktable databases and invoke supported CLI/import surfaces, but should not mutate private database tables.

### JMH owns canonical assets

Adopt a stable namespace such as `Photography/YYYY/MM/DD/<shoot>/`. The physical vault must have snapshots or another independent backup before automated source cleanup is eligible.

## Enterprise Darktable Exploration

The larger idea is valid: darktable could benefit from an enterprise mode with multiple authenticated clients, vault-backed source assets, stronger catalog availability, and explicit concurrency semantics. Replacing SQLite with MariaDB or PostgreSQL is not merely swapping a connection string.

Research must answer:

1. How deeply are SQLite syntax, pragmas, row IDs, attached databases, migrations, locking, and transaction assumptions embedded?
2. Which state is authoritative: central catalog, XMP, or a revisioned merge of both?
3. How do offline clients reserve work, detect stale revisions, and reconcile conflicts?
4. How are absolute paths replaced by logical asset IDs and location records?
5. What happens when the vault or database is unavailable but cached RAWs remain editable?
6. Can this be delivered through supported extension points, or does it require upstream core changes?
7. Can an upstreamable persistence abstraction avoid maintaining a permanent darktable fork?

## Architecture Options to Compare

### A. Photonic coordination, unchanged darktable core

- Separate local darktable catalogs per workstation.
- Canonical RAW/XMP vault and stable mount paths.
- Photonic manifest, ingest, cache policy, and conflict warnings.
- Lowest maintenance and fastest path; collaboration remains constrained.

### B. Photonic catalog service plus darktable adapter

- Photonic holds enterprise asset identity, location, workflow, and audit state in a server database.
- Darktable remains a local editor/catalog; adapters import/export XMP and collections through supported surfaces.
- Provides enterprise orchestration without immediately rewriting darktable persistence.

### C. Upstream darktable storage abstraction

- Introduce a catalog/persistence interface and test alternate backend.
- Potential PostgreSQL/MariaDB support, logical asset locations, revision checks, and migrations.
- Highest upstream value but large scope, compatibility burden, and long review horizon.

### D. Maintained enterprise fork

- Enables deeper server-backed and multi-user behavior.
- Carries ongoing merge, release, packaging, security, and schema obligations.
- Pursue only if upstream rejects required primitives and commercial value justifies permanent ownership.

## Initial Recommendation

Build option **B** incrementally while researching **C** upstream. This gives Photonic a defensible enterprise coordination role without coupling safety-critical ingest to an invasive darktable fork. Keep option **A** as the immediate usable workflow. Treat option **D** as a last resort.

PostgreSQL is likely the first server backend to prototype because of mature concurrency, transactional DDL, replication, JSON, and extension capabilities, but the choice must follow a source-level compatibility audit. MariaDB remains a candidate, not the default conclusion.

## Required Follow-Up

1. Complete the resumable read-only root/catalog/XMP inventory.
2. Map darktable persistence code and extension boundaries against the current upstream version.
3. Specify Photonic's immutable asset ID, location, revision, ingest-state, and cleanup-event contracts.
4. Build disposable failure-injection tests for NFS loss and duplicate ingest.
5. Prototype two-client revision/conflict behavior against copied sample catalogs.
6. Open an upstream design discussion before maintaining core changes.

## Sources

- Darktable manual: local copies, sidecars, importing, and multiple-computer guidance: <https://docs.darktable.org/usermanual/development/en/>
- Darktable upstream source: <https://github.com/darktable-org/darktable>
- Photonic integration notes: `docs/DARKTABLE_INTEGRATION.md`
- Photonic library vision: `docs/COMPREHENSIVE_PHOTO_LIBRARY_VISION.md`

