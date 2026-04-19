# Smoothcomp Ingestion Adapter

This repository is an internal Smoothcomp ingestion adapter implemented in Go. It acts as an Anti-Corruption Layer between Smoothcomp and the future system of record, and owns only provider-facing ingestion concerns:

- external fetching
- raw snapshot capture
- HTML/JSON parsing
- technical normalization
- source-level change detection
- snapshot and publication deduplication
- publication decisioning
- ingestion job execution
- internal operational APIs

It is not a public scraper API and it is not the business-domain backend.

Ownership is intentionally split:

- Go owns source-level change detection, snapshot dedupe, normalized-result dedupe, publication dedupe, parser and normalization versioning, and deciding whether a new envelope should be published
- Nest.js owns contract validation on receipt, import-run lifecycle, idempotent application to the domain model, canonical persistence, multitenancy, security, and audit of imported business state

## Production Execution Model

The supported runtime is split into:

- `cmd/api`: internal control plane, health/readiness, enqueue endpoints, publication lookup
- `cmd/worker`: scheduler plus multi-worker-safe execution loop
- `cmd/server`: local convenience binary only; not the recommended production shape

The job lifecycle is:

1. API or scheduler enqueues a durable job in `ingestion_jobs`
2. A worker claims one available job with a lease
3. The worker renews the lease while processing
4. Raw provider responses are stored in `raw_snapshots`
5. Technical normalization is stored in `normalized_results`
6. Publication decision metadata is stored with the normalized result for every execution
7. Published importable output is stored in `published_results` only when Go decides a new effective publication is warranted
8. The job either completes, gets rescheduled for retry, or reaches a terminal state

## Architecture

Canonical layout:

```text
cmd/api
cmd/worker
cmd/server
internal/core
internal/application
internal/adapters/smoothcomp
internal/adapters/storage
internal/adapters/transport
internal/platform/config
internal/platform/bootstrap
migrations
testdata
```

Dependency direction points inward:

1. `internal/core`
   Contracts, job models, error taxonomy, repository and pipeline ports
2. `internal/application`
   Enqueue, worker lifecycle, retry policy, scheduler orchestration
3. `internal/adapters/*`
   Smoothcomp provider implementation, storage implementation, internal HTTP transport
4. `internal/platform/*`
   Config loading, runtime wiring, correlation helpers

## Persistence Model

Production persistence uses Postgres. SQLite remains available as a local development fallback only.

Operational tables:

- `ingestion_jobs`
  Durable queue record, lease state, retry schedule, error state, versions, counters
- `job_attempts`
  One row per execution attempt
- `job_state_transitions`
  Audit trail of lifecycle transitions
- `raw_snapshots`
  Append-only per job attempt with hash-based idempotency
- `normalized_results`
  One canonical normalized output per job, updated idempotently by `job_id`, including `scope_key`, `source_snapshot_hash`, `normalized_hash`, and publication decision audit fields
- `published_results`
  One published envelope per effective publication, including `scope_key`, `source_snapshot_hash`, `normalized_hash`, `envelope_hash`, publication decision fields, and supersession lineage
- `schedule_configs_v2`
  Internal scheduler configuration

## Locking and Leasing Strategy

Production worker claiming uses a lease model:

- Postgres claim path uses `SELECT ... FOR UPDATE SKIP LOCKED`
- a claimed job is marked `running`
- `claimed_by`, `claimed_at`, `lease_until`, and `last_heartbeat_at` are persisted
- a worker renews the lease periodically while processing
- a job becomes claimable again when:
  - it is `pending` and `next_retry_at <= now()`
  - or it is `running` but `lease_until < now()`

This gives:

- safe concurrent multi-worker claiming
- crash recovery for stuck jobs
- no concurrent double-processing of the same active lease

SQLite uses the same repository contract but is intended only for local development, not for multi-worker production.

## Retry Model

Retries are durable and explicit.

- `attempt_count` is incremented on every claim
- `max_attempts` is stored on the job
- retryable failures move the job back to `pending`
- `next_retry_at` is persisted using exponential backoff with cap
- non-retryable failures end in `failed`
- retryable failures that exhaust attempts end in `exhausted`
- failure category, code, message, and retryability are persisted on the job and attempt records

Current states:

- `pending`
- `running`
- `succeeded`
- `failed`
- `exhausted`

## Idempotency and Consistency

Consistency rules in the current implementation:

- snapshots are append-only per job attempt, with uniqueness on:
  - `(job_id, attempt_number, resource_type, resource_key, sha256)`
- normalized results are idempotent by `job_id`
- published results are idempotent by `job_id`
- latest effective publication is looked up by `(pipeline, scope_key)` ordered by `published_at DESC`
- normalized results store a canonical `normalized_hash`
- published results store a canonical `envelope_hash`
- every normalized execution stores `publication_decision`, `publication_reason`, and `change_classification`

Publication decisioning is explicit:

- `NO_CHANGE` -> `SKIP_NO_CHANGE`
- `CONTENT_CHANGED` -> `PUBLISH_CHANGED`
- `NORMALIZATION_CHANGED` -> `PUBLISH_CHANGED`
- `REPUBLISH_FORCED` -> `PUBLISH_FORCED`

The adapter computes:

- `source_snapshot_hash`
  Stable hash of the fetched provider snapshot set for a single scope
- `normalized_hash`
  Stable hash of normalized semantic content after removing execution-local volatility such as snapshot ids, job ids, correlation ids, and timestamps
- `envelope_hash`
  Stable hash of the published envelope after publication-lineage metadata is added, excluding volatile delivery metadata such as `published_at`

This means a repeated execution of the same job can safely overwrite the canonical normalized record for that job while preserving attempt history, and Go can avoid publishing a redundant envelope when the latest effective publication for the same scope has not materially changed.

## Internal API

Active internal endpoints:

- `GET /internal/v1/health/live`
- `GET /internal/v1/health/ready`
- `POST /internal/v1/jobs`
- `GET /internal/v1/jobs`
- `GET /internal/v1/jobs/{id}`
- `GET /internal/v1/publications/latest?pipeline=...`

`GET /internal/v1/publications/latest` now accepts optional scope filters:

- `country`
- `event_type`
- `event_id`
- `profile_id`

When scope filters are present, the lookup resolves the latest effective publication for that exact provider scope instead of the last publication for the whole pipeline.

The API requires an internal token unless `ALLOW_INSECURE_INTERNAL_AUTH=true` is explicitly set. CORS is not enabled by default.

## Configuration

Important environment variables:

```env
DATABASE_DRIVER=postgres
DATABASE_DSN=postgres://user:password@localhost:5432/smoothcomp_adapter?sslmode=disable
DATABASE_RUN_MIGRATIONS=true
DATABASE_MAX_OPEN_CONNS=10
DATABASE_MAX_IDLE_CONNS=5
DATABASE_CONN_MAX_LIFETIME_SEC=300

WORKER_POLL_INTERVAL_SEC=5
WORKER_LEASE_DURATION_SEC=60
WORKER_HEARTBEAT_INTERVAL_SEC=20
WORKER_MAX_ATTEMPTS=5
WORKER_BASE_RETRY_DELAY_SEC=15
WORKER_MAX_RETRY_DELAY_SEC=300

INTERNAL_AUTH_TOKEN=replace-me
```

Validation currently enforces:

- internal auth token unless insecure mode is explicitly enabled
- supported DB driver
- worker heartbeat lower than lease duration
- positive retry and connection-pool settings

## Migrations

Migrations live under:

- `migrations/postgres`
- `migrations/sqlite`

They are executed automatically on startup when `DATABASE_RUN_MIGRATIONS=true`.

Current migration flow:

1. open DB
2. ensure `schema_migrations`
3. run unapplied SQL files in lexical order
4. record applied versions

## Local Development

Local fallback mode:

```env
DATABASE_DRIVER=sqlite
DATABASE_DSN=./storage/adapter.db
ALLOW_INSECURE_INTERNAL_AUTH=true
```

Typical local commands:

```bash
go run ./cmd/api
go run ./cmd/worker
```

`cmd/server` still exists, but it should be treated as local convenience only.

## Supported Pipelines Today

- `smoothcomp.event_catalog`
- `smoothcomp.event_participants`
- `smoothcomp.event_detail`
- `smoothcomp.athlete_profile_enrichment`
- `smoothcomp.academy_catalog`

Fixture-based parser tests live under:

- `testdata/smoothcomp/events`
- `testdata/smoothcomp/participants`
- `testdata/smoothcomp/event_detail`
- `testdata/smoothcomp/athletes`
- `testdata/smoothcomp/academies`
- `testdata/smoothcomp/audit`

Storage lifecycle tests live in:

- `internal/adapters/storage/gormstore`

## Extraction Audit

This repository now includes a deterministic extraction audit runner for evidence-based verification of parser and normalization quality.

Artifacts:

- dataset: `testdata/smoothcomp/audit/dataset.json`
- raw audit fixtures: `testdata/smoothcomp/audit/fixtures`
- audit runner: `cmd/audit`
- current human-readable report: `docs/smoothcomp-extraction-audit.md`
- match extraction design note: `docs/smoothcomp-match-extraction.md`

Run it locally:

```bash
go run ./cmd/audit
go run ./cmd/audit -format json
```

The audit compares:

1. raw provider snapshots
2. normalized adapter output
3. manually curated truth assertions with mismatch classification

Mismatch classes currently used:

- `SOURCE_NOT_VISIBLE`
- `PARTIAL_SOURCE_DATA`
- `PARSER_DRIFT`
- `NORMALIZATION_BUG`
- `ID_RESOLUTION_BUG`
- `SUBDOMAIN_VARIANT`
- `EXPECTATION_WAS_WRONG`
- `UNSUPPORTED_VARIANT`

## Migration Status

The following legacy extraction capabilities have now been migrated into first-class provider pipelines:

- event catalog
- event participants
- event detail
- athlete profile enrichment
- academy catalog

The adapter also publishes athlete-centric match history and derived win/loss summaries from profile history where the provider visibly exposes them. Event-centric bracket/results reconstruction has not yet been frozen as a supported audited contract.

Legacy packages remain only as temporary reference for non-runtime helpers and for future cleanup:

- `internal/scraper`
- `internal/api`
- `internal/scheduler`
- `internal/config`

These packages are no longer the supported runtime path for day-to-day operation.

## Known Limitations After This Step

- Postgres is now wired, but production rollout still needs environment-specific operational packaging and deployment manifests
- metrics export is still hook-oriented/log-oriented; there is not yet a Prometheus/OpenTelemetry adapter
- the adapter now decides publication at source level, but delivery orchestration into Nest is still a separate next step
- provider coverage is broader now, but some Smoothcomp page variants may still require additional fixtures as new markup variations are discovered
- the extraction audit is deterministic and reproducible, but it is still based on a curated corpus rather than large-scale live production sampling
- athlete competitive records are currently strongest from profile-history sources; event-centric bracket/results pages still need archived fixture coverage before that path should be consumed downstream
- `cmd/server` still exists for convenience and should not be the default production topology
- concurrent jobs for the exact same scope can still race at publication time because there is not yet a dedicated per-scope publication lock or delivery outbox
