<!-- sdd-spec-version: 1.1 -->

## Objective

Define and validate an enterprise-grade architecture for Photonic and darktable that supports canonical vault-hosted RAW assets, safe roving/offline editing, and multiple computers without sharing or mutating darktable's live SQLite database. Produce evidence sufficient to choose among integration-only, upstream darktable enhancement, or a maintained fork.

## Why This Matters

The desired workflow is import once, verify durable vault storage, edit from one or more computers, and remove disposable local copies safely. Darktable already supports local copies and XMP sidecars, while Photonic provides processing orchestration, but neither currently provides a transactional vault ingest authority or supported concurrent multi-computer catalog.

## Repository & System Clarity

- Repo: `jmh-devel/photonic`
- System / subdomain: Photonic photo, astrophotography, panorama, and timelapse processing; darktable desktop integration; JMH NFS-backed photography vault
- API spec / OpenAPI doc: N/A; this ticket defines research contracts before implementation
- Related upstream: `darktable-org/darktable`

## Existing System Audit

- Checked `docs/DARKTABLE_INTEGRATION.md`: describes filesystem and read-only darktable database correlation, but overstates portions of the implemented API/workflow.
- Checked `docs/COMPREHENSIVE_PHOTO_LIBRARY_VISION.md`: proposes library management, ingest, metadata, and processing concepts; transactional vault semantics remain unspecified.
- Checked Photonic implementation: darktable CLI orchestration, astro/timelapse/panorama pipelines, read-only SQLite inspection, and basic filesystem monitoring exist; `scan` and robust recursive ingest are incomplete.
- Checked darktable 5.4.1 on Yoda and official documentation: local copies, XMP sidecars, and `resync local copy` support roving work; darktable does not watch directories, does not provide native database sharing, and does not provide conflict resolution for multiple computers.
- Checked current storage/catalog topology: JMH is mounted at `/mnt/jmh/data`; relevant roots include `Photography`, `Darktable`, and `yoda-photography-staging`; the active catalog substantially references `/data/Photography`, including suspicious duplicated path segments.
- Do not create a custom sync plugin, write directly to darktable schemas, or choose MariaDB/PostgreSQL before auditing upstream persistence boundaries and offline/conflict semantics.

## Scope

- Repos: `jmh-devel/photonic`; read-only research against `darktable-org/darktable`
- Paths allowed: `docs/`, `tickets/`, backlog documents; a future approved implementation may add dedicated ingest/catalog adapter packages
- Paths forbidden: direct writes to user `library.db`/`data.db`, destructive photo moves/deletes, credentials, production vault mutation
- Research tracks:
  1. Photonic transactional vault ingest and manifest authority.
  2. Supported darktable local-copy/XMP integration.
  3. Multi-computer catalog semantics and conflict model.
  4. Feasibility of a storage abstraction and server-backed catalog (PostgreSQL/MariaDB or another fit).
  5. Upstream contribution versus maintained fork/plugin boundary.

## API / Data Contracts

- Proposed ingest states: `discovered | copying | vault_verified | backup_verified | imported | cleanup_eligible | cleaned | failed`
- Canonical asset identity: content digest plus immutable asset ID; filesystem path is a location, not identity.
- Asset locations: one canonical vault object, zero or more verified replicas/caches, and explicit availability state.
- Edit authority: XMP sidecars remain portable edit metadata unless research proves a compatible migration path.
- Concurrency outcomes: `accepted | rejected_stale_revision | conflict_requires_resolution`.
- Cleanup rule: source deletion is prohibited until vault checksum, required backup policy, and import/catalog acknowledgment are durably recorded.
- Database contract: no direct Photonic writes to darktable-owned tables.

## Implementation Constraints

- Preserve darktable's offline usability and non-destructive editing model.
- Treat SQLite replacement as a source-level architecture investigation, not a configuration change.
- Evaluate PostgreSQL and MariaDB against transactions, locking, schema migration, latency, offline operation, and upstream portability; do not assume SQL dialect compatibility.
- Keep RAW binaries out of Git/Git LFS.
- Use stable mount paths across clients or introduce a tested logical asset-location abstraction.
- Any destructive cleanup requires dry-run output, audit evidence, and explicit operator enablement.
- Auth: N/A during research; future remote catalog service requires authenticated clients and scoped change authority.
- Timezone / locale: UTC for machine events; preserve capture timezone metadata separately.
- DB / ORM: migration strategy required before prototype; no production catalog migration in this ticket.
- K8s / deployment: N/A.
- Idempotency: scans and ingest retries must be safe to repeat.
- Rollback path: retain existing local darktable databases, XMP files, and canonical originals throughout prototypes.

## Testing Strategy

- Build an upstream source map of SQLite calls, prepared statements, transactions, migrations, and assumptions.
- Prototype only against disposable catalogs and copied sample assets.
- Simulate NFS interruption during copy, checksum, import acknowledgment, XMP resync, and cleanup.
- Exercise two clients editing the same and different assets; record deterministic conflict behavior.
- Benchmark local SQLite versus a server-backed prototype under LAN latency and disconnected operation.
- Verify darktable local-copy behavior using documented `copy locally` and `resync local copy` workflows.
- Reconcile roots with a resumable read-only inventory before any migration proposal.

## Related Issues & Blockers

- Tracking issue: `jmh-devel/photonic#2`
- Depends on: complete read-only inventory of `/data/Photography`, `/mnt/jmh/data/Photography`, `/mnt/jmh/data/yoda-photography-staging`, film-roll paths, XMP coverage, duplicates, and one-sided assets.
- Blocked by: no trustworthy completed full-root inventory; the previous scan received SIGTERM.
- Serialize-with job: any photo-vault migration or cleanup job.

## Acceptance Criteria

- [ ] `docs/DARKTABLE_ENTERPRISE_VAULT_RESEARCH.md` records validated current behavior, rejected claims, and architecture options.
- [ ] A source-level darktable persistence map identifies coupling that must change for a server database.
- [ ] A decision record compares integration-only, plugin/upstream enhancement, and maintained-fork options.
- [ ] A disposable proof of concept demonstrates multi-client behavior without touching production catalogs or originals.
- [ ] A vault ingest design includes checksums, durable states, retry/resume, backup policy, and cleanup auditability.
- [ ] Risks and an upstream engagement plan are documented before implementation is authorized.

## Observability Proof

- Research artifacts include command transcripts, benchmark outputs, schema/source references, and failure-injection results.
- A future ingest prototype emits an append-only event for every state transition and cleanup decision.

## Execution Mode

- Mode: read-only research and documentation
- Sandbox: workspace-write for documentation and disposable prototypes only
- Approval: required before production catalog, vault, or source cleanup changes

## SDD Spec Version

- `sdd-spec-version: 1.1`
- Bootstrap anchor: repository `README.md` and this ticket
- Repo catalog anchor: `jmh-devel/photonic`

