# Smoothcomp Ingestion Adapter

This repository is being refactored from a mixed scraper API into an internal Smoothcomp ingestion adapter. The service remains in Go and is now structured as an Anti-Corruption Layer that owns provider fetching, raw snapshot capture, parsing, technical normalization, ingestion job execution, and internal operational APIs.

## Target Architecture

Canonical layout:

```text
cmd/api
cmd/worker
cmd/server                # local-dev compatibility binary
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

1. `internal/core`: stable contracts, job model, errors, repository ports
2. `internal/application`: enqueue/worker/scheduler/operations orchestration
3. `internal/adapters/*`: Smoothcomp provider integration, GORM storage, internal HTTP transport
4. `internal/platform/*`: config loading, runtime wiring, correlation context

## Active Internal API

The new active service path exposes only internal operational endpoints:

- `GET /internal/v1/health/live`
- `GET /internal/v1/health/ready`
- `POST /internal/v1/jobs`
- `GET /internal/v1/jobs`
- `GET /internal/v1/jobs/{id}`
- `GET /internal/v1/publications/latest?pipeline=...`

The API requires an internal token unless `ALLOW_INSECURE_INTERNAL_AUTH=true` is explicitly set. CORS is no longer enabled by default.

## First Implemented Pipelines

1. `smoothcomp.event_catalog`
   Fetches Smoothcomp event listing HTML, stores the raw snapshot, parses it, normalizes it into a stable contract, and publishes the importable result.
2. `smoothcomp.event_participants`
   Fetches Smoothcomp event participant JSON, stores the raw snapshot, parses it, normalizes organizations/people/registrations, and publishes the importable result.

Each run now separates:

- raw external data: `raw_snapshots`
- normalized technical data: `normalized_results`
- published/importable contract output: `published_results`

## Storage

Repository abstractions are now in place for jobs, snapshots, normalized results, published results, and schedules.

- `sqlite` is isolated as the current local-dev backing store.
- `postgres` is recognized in config but intentionally not wired in this offline refactor pass; the adapter boundaries now make that swap localized to the storage layer plus SQL migrations.

## Scheduler and Worker Model

- `cmd/api`: internal control plane only
- `cmd/worker`: polling worker + scheduler
- `cmd/server`: compatibility binary that runs both in one process for local development

The API enqueues pending jobs; it no longer starts scraping in request goroutines.

## Parser Test Strategy

Fixture-based parser tests now live under:

- `testdata/smoothcomp/events`
- `testdata/smoothcomp/participants`

Current tests cover event catalog HTML parsing and event participant JSON normalization. Extend this fixture strategy before migrating academy scraping, event detail extraction, and athlete profile enrichment into the new pipelines.

## Migration Notes

Legacy packages remain in the repository for extraction reference:

- `internal/scraper`
- `internal/api`
- `internal/scheduler`
- `internal/config`

They are no longer the target architecture. New work should extend the new adapter path, not the legacy mixed packages.

Current migration map:

- legacy `internal/api` trigger handlers -> `internal/adapters/transport/httpapi`
- legacy `internal/scheduler` cron execution -> `internal/application/scheduler`
- legacy `internal/scraper` orchestration -> `internal/application/ingestion` + `internal/adapters/smoothcomp`
- legacy global DB access in `internal/config` -> `internal/adapters/storage/gormstore`
- legacy mixed persistence models -> raw snapshots + normalized results + published results repositories
