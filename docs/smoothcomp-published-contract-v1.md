# Smoothcomp Published Contract v1

## Contract Identity

- contract name: `Smoothcomp Published Envelope`
- emitted `contract_version`: `smoothcomp-published-envelope-v1`
- scope: frozen downstream contract for audited Smoothcomp normalized envelopes
- primary v1 product scope: athlete-centric identity, registrations, match outcomes, and match summaries

The JSON Schema companion for this contract lives at [smoothcomp-published-envelope-v1.schema.json](/home/camilo/smoothcomp-scraper/docs/schemas/smoothcomp-published-envelope-v1.schema.json).

## Versioning Policy

- `v1` compatibility is additive only.
- Allowed without a version bump:
  - adding new `OPTIONAL_CORE` fields
  - adding new `INFORMATIONAL_ONLY` fields
  - adding new warning codes
  - expanding source coverage or parser versions without changing existing field meaning
- Requires `v2`:
  - removing a field
  - changing the semantic meaning of an existing field
  - changing enum meanings such as `match.outcome`
  - promoting previously excluded event-centric semantics into product-grade required behavior

Consumers must key compatibility off `contract_version`, not `parser_version` or `normalization_version`.

## Ownership Boundary

- Go is the Smoothcomp anti-corruption adapter. It owns provider fetches, parsing, normalization, source-level change detection, dedupe, and deciding whether a new envelope should be published.
- Nest.js is the system of record. It owns contract validation on receipt, import-run lifecycle, idempotent application into canonical domain tables, multitenancy, security, and audit of imported business state.

In short:

- Go decides whether Smoothcomp changed enough to publish
- Nest decides how a received publication affects internal domain state

## Compatibility Expectations

- Consumers must tolerate missing `OPTIONAL_CORE` and `INFORMATIONAL_ONLY` fields.
- Consumers must tolerate additional warning codes in `warnings[]`.
- Consumers must ignore fields marked `EXCLUDED_FROM_V1`, even if the adapter emits internal values for experimentation later.
- Consumers must treat `warnings[]` as additive degradation context, not as a failure response.

## Publication Semantics

The frozen payload contract remains `smoothcomp-published-envelope-v1`, but the adapter now emits operational publication metadata additively through `metadata`.

Stable publication classifications are:

- `NO_CHANGE`
- `CONTENT_CHANGED`
- `NORMALIZATION_CHANGED`
- `REPUBLISH_FORCED`

Stable publication decisions are:

- `SKIP_NO_CHANGE`
- `PUBLISH_CHANGED`
- `PUBLISH_FORCED`

The adapter computes and persists, per provider scope:

- `source_snapshot_hash`
  Source-level revision fingerprint of the fetched provider snapshot set
- `normalized_hash`
  Canonical hash of normalized semantic content after removing execution-local volatility
- `envelope_hash`
  Canonical hash of the published envelope after publication-lineage metadata is added, excluding volatile delivery metadata such as `published_at`

When no effective change is detected, Go may skip publication entirely. A skipped run is still auditable in adapter storage, but no new downstream envelope is emitted.

## What Nest Should Trust

Nest can trust:

- `contract_version`
- `provider`
- `pipeline`
- `scope`
- the normalized records contained in the payload
- `metadata.scope_key`
- `metadata.source_snapshot_hash`
- `metadata.normalized_hash`
- `metadata.envelope_hash`
- `metadata.publication_decision`
- `metadata.publication_reason`
- `metadata.change_classification`
- `metadata.forced_republish`

Nest must not treat as domain truth:

- `parser_version`
- `normalization_version`
- raw snapshot lineage identifiers such as `raw_reference_ids`
- warning absence as proof that the provider exposed every possible field
- `publication_decision` as equivalent to domain apply or merge policy

Go guarantees that publication metadata explains why an envelope was or was not emitted for a provider scope. Go does not guarantee how the receiving system should mutate canonical domain records.

## Partial Visibility Semantics

- `REQUIRED_CORE`
  The field must be present for a record to be treated as product-grade within v1 scope.
- `OPTIONAL_CORE`
  The field is contract-safe for product use when present, but source visibility is not guaranteed on every payload.
- `INFORMATIONAL_ONLY`
  The field may be stored for lineage, operator review, or external display, but must not drive hard product semantics yet.
- `EXCLUDED_FROM_V1`
  The field or semantic surface is intentionally outside the frozen contract and must not be imported into Nest product logic.

Valid-but-partial means:

- all emitted records still satisfy their `REQUIRED_CORE` fields
- one or more `OPTIONAL_CORE` or `INFORMATIONAL_ONLY` fields are absent because the provider did not expose them
- the envelope may include warnings that explain missing context or degraded enrichment

Non-publishable for product-grade use means:

- a record needed for v1 product behavior is missing one of its `REQUIRED_CORE` fields
- the consumer is depending on an `EXCLUDED_FROM_V1` field
- the consumer is ignoring a warning whose semantics directly affect interpretation of a retained field

## Warning Contract

### Stable audited warning codes in v1

- `academy_detail_missing`
  Meaning: academy catalog row exists but no detail snapshot was captured.
  Consumer action: keep academy identity only; treat description, website, image, and stats as incomplete.
- `athlete_profile_events_unavailable`
  Meaning: athlete profile identity parsed, but the profile events feed was unavailable.
  Consumer action: keep profile identity; treat matches and match summaries as incomplete for that run.
- `event_info_panels_unavailable`
  Meaning: event detail HTML parsed, but optional info panels were unavailable.
  Consumer action: keep event record; venue and organizer enrichments may be incomplete.
- `event_cms_parse_failed`
  Meaning: event detail HTML parsed, but optional CMS blocks failed to decode.
  Consumer action: keep event record; treat CMS-derived attributes as absent.
- `match_finish_method_missing`
  Meaning: match visibility is real, but finish detail was not exposed by the provider.
  Consumer action: keep `match.outcome`, `match.result_text`, and `match.event_name`; leave `match.finish_method` null.
- `match_opponent_hidden`
  Meaning: match exists, but opponent identity was hidden or partial in the source.
  Consumer action: keep match result semantics; do not infer opponent identity.
- `match_summary_inconsistent_with_profile`
  Meaning: derived visible match totals disagree with visible profile counters.
  Consumer action: keep derived `match_summary` as the best audited summary and surface the warning for review.

### Open warning enum rule

Additional warning codes may appear inside `v1` without a contract-version bump. Consumers must:

- persist unknown warning codes as opaque strings
- avoid failing deserialization on unknown codes
- avoid turning unknown warnings into product-grade blocking errors automatically

Currently implemented but not yet frozen as stable warning semantics:

- `academy_detail_parse_failed`
- `academy_detail_unavailable`
- `athlete_profile_events_parse_failed`
- `event_cms_unavailable`
- `event_info_panels_parse_failed`
- `match_context_partial`

## Frozen v1 Scope

The frozen product scope for `v1` is athlete-centric:

- athlete identity from `smoothcomp.athlete_profile_enrichment`
- athlete-centric match outcomes from `smoothcomp.athlete_profile_enrichment`
- athlete-centric match summaries from `smoothcomp.athlete_profile_enrichment`
- participant and registration support data from `smoothcomp.event_participants`
- supporting academy identity references where a stable `organization_source_id` exists

Standalone event catalogs, event-detail pages, and academy catalogs remain supportive context in `v1`, not the primary product contract.

## Field Classification

### Envelope

- `contract_version`: `REQUIRED_CORE`
- `provider`: `REQUIRED_CORE`
- `pipeline`: `REQUIRED_CORE`
- `generated_at`: `REQUIRED_CORE`
- `scope`: `OPTIONAL_CORE`
- `warnings`: `OPTIONAL_CORE`
- `parser_version`: `INFORMATIONAL_ONLY`
- `normalization_version`: `INFORMATIONAL_ONLY`
- `correlation_id`: `INFORMATIONAL_ONLY`
- `metadata`: `INFORMATIONAL_ONLY`
  Current operational keys may include `scope_key`, `source_snapshot_hash`, `normalized_hash`, `envelope_hash`, `published_at`, `publication_decision`, `publication_reason`, `change_classification`, `forced_republish`, `supersedes_publication_id`, `source_profile_id`, `source_event_id`, and `import_request_id`

### `smoothcomp.athlete_profile_enrichment`

- `person.source_id`: `REQUIRED_CORE`
- `person.full_name`: `REQUIRED_CORE`
- `person.country`: `OPTIONAL_CORE`
- `person.country_code`: `OPTIONAL_CORE`
- `person.age`: `OPTIONAL_CORE`
- `person.belt_rank`: `OPTIONAL_CORE`
- `person.organization_source_id`: `OPTIONAL_CORE`
- `organization.source_id`: `OPTIONAL_CORE`
- `organization.name`: `OPTIONAL_CORE`
- `organization.kind`: `INFORMATIONAL_ONLY`
- `person.birth_year`: `INFORMATIONAL_ONLY`
- `person.profile_url`: `INFORMATIONAL_ONLY`
- `person.image_url`: `INFORMATIONAL_ONLY`
- `person.attributes.*`: `INFORMATIONAL_ONLY`
- `events[].name`: `INFORMATIONAL_ONLY`
- `events[].status`: `INFORMATIONAL_ONLY`
- `match.source_id`: `REQUIRED_CORE`
- `match.athlete_source_id`: `REQUIRED_CORE`
- `match.outcome`: `REQUIRED_CORE`
- `match.result_text`: `REQUIRED_CORE`
- `match.event_name`: `REQUIRED_CORE`
- `match.finish_method`: `OPTIONAL_CORE`
- `match.opponent_name`: `OPTIONAL_CORE`
- `match.opponent_source_id`: `INFORMATIONAL_ONLY`
- `match.event_source_id`: `INFORMATIONAL_ONLY`
- `match.division`: `INFORMATIONAL_ONLY`
- `match.age_category`: `INFORMATIONAL_ONLY`
- `match.rank`: `INFORMATIONAL_ONLY`
- `match.weight_class`: `INFORMATIONAL_ONLY`
- `match.score_text`: `INFORMATIONAL_ONLY`
- `match.round_label`: `INFORMATIONAL_ONLY`
- `match.source_url`: `INFORMATIONAL_ONLY`
- `match.starts_at`: `INFORMATIONAL_ONLY`
- `match.confidence`: `INFORMATIONAL_ONLY`
- `match.attributes.*`: `INFORMATIONAL_ONLY`
- `match.bracket_label`: `EXCLUDED_FROM_V1`
- `match.placement`: `EXCLUDED_FROM_V1`
- `match_summary.athlete_source_id`: `REQUIRED_CORE`
- `match_summary.total_matches`: `REQUIRED_CORE`
- `match_summary.total_wins`: `REQUIRED_CORE`
- `match_summary.total_losses`: `REQUIRED_CORE`
- `match_summary.confidence`: `INFORMATIONAL_ONLY`
- `match_summary.scope`: `INFORMATIONAL_ONLY`
- `match_summary.wins_by_submission`: `INFORMATIONAL_ONLY`
- `match_summary.wins_by_points`: `INFORMATIONAL_ONLY`
- `match_summary.wins_by_decision`: `INFORMATIONAL_ONLY`
- `match_summary.wins_by_dq`: `INFORMATIONAL_ONLY`
- `match_summary.losses_by_submission`: `INFORMATIONAL_ONLY`
- `match_summary.losses_by_points`: `INFORMATIONAL_ONLY`
- `match_summary.losses_by_decision`: `INFORMATIONAL_ONLY`
- `match_summary.losses_by_dq`: `INFORMATIONAL_ONLY`
- `match_summary.attributes.*`: `INFORMATIONAL_ONLY`

### `smoothcomp.event_participants`

- `event.source_id`: `OPTIONAL_CORE`
- `event.name`: `INFORMATIONAL_ONLY`
- `person.source_id`: `REQUIRED_CORE`
- `person.full_name`: `REQUIRED_CORE`
- `person.country`: `OPTIONAL_CORE`
- `person.country_code`: `INFORMATIONAL_ONLY`
- `person.age`: `OPTIONAL_CORE`
- `person.organization_source_id`: `OPTIONAL_CORE`
- `person.profile_url`: `INFORMATIONAL_ONLY`
- `person.attributes.age`: `INFORMATIONAL_ONLY`
- `organization.source_id`: `OPTIONAL_CORE`
- `organization.name`: `OPTIONAL_CORE`
- `organization.country_code`: `INFORMATIONAL_ONLY`
- `registration.source_id`: `REQUIRED_CORE`
- `registration.event_source_id`: `REQUIRED_CORE`
- `registration.person_source_id`: `REQUIRED_CORE`
- `registration.organization_source_id`: `OPTIONAL_CORE`
- `registration.seed`: `OPTIONAL_CORE`
- `registration.actual_weight`: `OPTIONAL_CORE`
- `registration.division`: `INFORMATIONAL_ONLY`
- `registration.age_category`: `INFORMATIONAL_ONLY`
- `registration.rank`: `INFORMATIONAL_ONLY`
- `registration.weight_class`: `INFORMATIONAL_ONLY`

### `smoothcomp.event_catalog`

- `events[]` as a standalone feed: `INFORMATIONAL_ONLY`
- `event.name`: `INFORMATIONAL_ONLY`
- `event.source_url`: `INFORMATIONAL_ONLY`
- `event.city`: `INFORMATIONAL_ONLY`
- `event.country`: `INFORMATIONAL_ONLY`
- `event.country_code`: `INFORMATIONAL_ONLY`
- `event.status`: `INFORMATIONAL_ONLY`
- `event.starts_at`: `INFORMATIONAL_ONLY`

### `smoothcomp.event_detail`

- `events[]` as a standalone feed: `INFORMATIONAL_ONLY`
- `event.name`: `INFORMATIONAL_ONLY`
- `event.description`: `INFORMATIONAL_ONLY`
- `event.venue_name`: `INFORMATIONAL_ONLY`
- `event.country`: `INFORMATIONAL_ONLY`
- `event.organizer_name`: `INFORMATIONAL_ONLY`
- `event.city`: `INFORMATIONAL_ONLY`
- `event.image_url`: `INFORMATIONAL_ONLY`
- `event.source_id`: `INFORMATIONAL_ONLY`
- `event.source_url`: `INFORMATIONAL_ONLY`
- `event.attributes.*`: `INFORMATIONAL_ONLY`

### `smoothcomp.academy_catalog`

- `organizations[]` as a standalone feed: `INFORMATIONAL_ONLY`
- `organization.source_id`: `INFORMATIONAL_ONLY`
- `organization.name`: `INFORMATIONAL_ONLY`
- `organization.country_code`: `INFORMATIONAL_ONLY`
- `organization.country`: `INFORMATIONAL_ONLY`
- `organization.website_url`: `INFORMATIONAL_ONLY`
- `organization.image_url`: `INFORMATIONAL_ONLY`
- `organization.description`: `INFORMATIONAL_ONLY`
- `organization.attributes.athlete_count`: `INFORMATIONAL_ONLY`
- `organization.attributes.total_wins`: `INFORMATIONAL_ONLY`
- `organization.attributes.instagram_url`: `INFORMATIONAL_ONLY`

## Out Of Scope For v1

- event-centric bracket reconstruction
- full public results/brackets/match-page semantics
- placement and round guarantees across unsupported layouts
- cross-source canonical event identity merge logic
- cross-source canonical opponent identity merge logic
- unaudited provider surfaces and speculative enrichments

## Product-Visible Persistence Guidance

Safe to persist as product-grade data now:

- athlete `person.source_id`, `person.full_name`
- athlete optional core fields when present: `country`, `country_code`, `age`, `belt_rank`, `organization_source_id`
- match `source_id`, `athlete_source_id`, `outcome`, `result_text`, `event_name`
- match optional core fields when present: `finish_method`, `opponent_name`
- match summary `athlete_source_id`, `total_matches`, `total_wins`, `total_losses`
- participant `person.source_id`, `person.full_name`, `person.age`, `person.organization_source_id`
- registration `source_id`, `event_source_id`, `person_source_id`, plus optional `seed` and `actual_weight`

Store as external or operational context only:

- standalone event catalog/detail payloads
- standalone academy catalog payloads
- birth years, images, profile URLs, academy websites, academy descriptions
- low-sample match metadata such as `score_text`, `round_label`, `weight_class`
- method-specific `match_summary` buckets

Do not import yet:

- bracket placement or event-centric result semantics
- any excluded field listed above
- any downstream logic that assumes warnings can be ignored

## Valid Payload Examples

### Athlete-centric, complete enough for product use

```json
{
  "contract_version": "smoothcomp-published-envelope-v1",
  "provider": "smoothcomp",
  "pipeline": "smoothcomp.athlete_profile_enrichment",
  "generated_at": "2026-04-17T15:35:14Z",
  "scope": {
    "profile_id": "7788"
  },
  "people": [
    {
      "source_id": "7788",
      "full_name": "Maria Silva",
      "country": "Brazil",
      "country_code": "BR",
      "age": 27,
      "belt_rank": "Brown belt",
      "organization_source_id": "academy_name:alliance-sao-paulo"
    }
  ],
  "matches": [
    {
      "source_id": "9001",
      "athlete_source_id": "7788",
      "event_name": "South American Open 2026",
      "outcome": "win",
      "finish_method": "submission",
      "result_text": "Won by submission",
      "opponent_name": "Ana Costa"
    }
  ],
  "match_summaries": [
    {
      "athlete_source_id": "7788",
      "total_matches": 12,
      "total_wins": 9,
      "total_losses": 3
    }
  ]
}
```

### Valid-but-partial with warnings

```json
{
  "contract_version": "smoothcomp-published-envelope-v1",
  "provider": "smoothcomp",
  "pipeline": "smoothcomp.athlete_profile_enrichment",
  "generated_at": "2026-04-17T15:35:14Z",
  "scope": {
    "profile_id": "8805"
  },
  "people": [
    {
      "source_id": "8805",
      "full_name": "Leo Andrade",
      "country": "Brazil"
    }
  ],
  "warnings": [
    {
      "code": "athlete_profile_events_unavailable",
      "subject_type": "person",
      "subject_id": "8805"
    }
  ]
}
```
