# Smoothcomp Extraction Audit

This report is the current reproducible extraction audit for the Smoothcomp ingestion adapter. It is based on the deterministic fixture corpus in `testdata/smoothcomp/audit`, not on live provider calls.

## Audit Scope

- pipelines audited: `smoothcomp.event_catalog`, `smoothcomp.event_participants`, `smoothcomp.event_detail`, `smoothcomp.athlete_profile_enrichment`, `smoothcomp.academy_catalog`
- total audit cases: `35`
- representative source coverage:
  - athlete profiles: `15`
  - event catalog pages: `5`
  - event detail pages: `5`
  - participant payloads: `5`
  - academy catalog/detail combinations: `5`

## Measured Summary

- exact matches: `235`
- partial matches: `0`
- hard mismatches: `6`
- source-not-visible or unsupported facts: `16`
- expected warnings: `11`
- missing warnings: `0`
- unexpected warnings: `0`

## What The Adapter Extracts Reliably Today

High-confidence fields from the audited corpus:

- `smoothcomp.event_catalog`
  - `event.name`
  - `event.source_url`
  - `event.city`
  - `event.country`
  - `event.country_code`
  - `event.status`
  - `event.starts_at` when present in the embedded payload
- `smoothcomp.event_detail`
  - `event.name`
  - `event.venue_name`
  - `event.country`
  - `event.organizer_name`
- `smoothcomp.athlete_profile_enrichment`
  - `person.full_name`
  - `person.country`
  - `person.country_code`
  - `person.age`
  - `person.belt_rank`
  - `person.organization_source_id`
  - `match.outcome`
  - `match.finish_method` when the provider shows it
  - `match.opponent_name` when the provider shows it
  - `match_summary.total_matches`
  - `match_summary.total_wins`
  - `match_summary.total_losses`
- `smoothcomp.event_participants`
  - `person.full_name`
  - `person.country`
  - `person.organization_source_id`
  - `organization.name`
  - `registration.actual_weight`
  - `registration.seed`
- `smoothcomp.academy_catalog`
  - `organization.name`
  - `organization.country_code`
  - `organization.attributes.athlete_count`

Warning behavior that is currently trustworthy in the audited corpus:

- `academy_detail_missing`
- `athlete_profile_events_unavailable`
- `event_info_panels_unavailable`
- `event_cms_parse_failed`
- `match_finish_method_missing`
- `match_opponent_hidden`
- `match_summary_inconsistent_with_profile`

## Match-Level Reliability

The adapter can now produce athlete-centric competitive records from Smoothcomp profile history with strong evidence for core result fields.

High-confidence match fields:

- `match.outcome`
- `match.finish_method`
- `match.opponent_name`
- `match.result_text`
- `match.event_name`
- `match_summary.total_matches`
- `match_summary.total_wins`
- `match_summary.total_losses`

Informational-only match fields for now:

- `match.opponent_source_id`
  Low sample count and depends on provider visibility.
- `match.score_text`
  Correct in the current corpus, but too lightly sampled to freeze as a strict import field.
- `match.round_label`
  Present in some payloads only.
- `match.weight_class`
  Reliable when visible, but current corpus is too small to mark it high-confidence.
- method-specific summary buckets such as `wins_by_submission`, `losses_by_points`, `losses_by_decision`
  These derive correctly in the current cases, but still need broader corpus coverage before contract freeze.

Not yet proven enough:

- event-centric match reconstruction from public results/bracket pages
- full cross-event canonical match identity beyond provider-visible IDs
- placement/bracket context across unsupported layouts

## Partial Or Informational-Only Fields

These fields are usable for display or operator context, but should not yet be treated as hard import keys without additional evidence:

- `smoothcomp.event_detail.event.description`
  Corpus result: `low confidence`
  Reason: body-visible description content is not consistently normalized unless it is present in JSON-LD.
- `smoothcomp.academy_catalog.organization.website_url`
  Corpus result: low sample count, no failures yet.
- `smoothcomp.academy_catalog.organization.image_url`
  Corpus result: low sample count, no failures yet.
- `smoothcomp.athlete_profile_enrichment.person.birth_year`
  Corpus result: low sample count, no failures yet.
- `smoothcomp.athlete_profile_enrichment.match.score_text`
  Corpus result: correct in the current corpus, but low sample count.
- `smoothcomp.athlete_profile_enrichment.match_summary.wins_by_submission`
  Corpus result: correct in the current corpus, but low sample count.
- `smoothcomp.event_participants.person.profile_url`
  Corpus result: derived cleanly from `user_id`, but only lightly sampled in this audit.

## Do Not Consume Yet

These fields should not yet be treated as trustworthy for downstream import semantics:

- `smoothcomp.event_participants.person.age`
  Result: `0/4` exact matches, classified as `NORMALIZATION_BUG`
  Reason: the source age is visible and captured only in `person.attributes.age`; it is not normalized into the canonical `person.age` field.
- `smoothcomp.academy_catalog.organization.country`
  Result: parser drift on the audited location-visible academy case.
  Reason: country text can be visible in academy detail pages but is not currently normalized.
- event-centric match/bracket reconstruction fields not sourced from athlete profile history
  Reason: identified as useful source families, but not yet covered by deterministic archived fixtures and reliability assertions.

## Confirmed Mismatches

The current hard mismatches are concentrated in three areas:

1. `event_participants -> person.age`
   Four audited cases fail because age remains in `attributes.age` instead of the canonical `age` field.
2. `event_detail -> description`
   One audited event exposes a body description that the parser currently ignores.
3. `academy_catalog -> organization.country`
   One audited academy detail page visibly includes location text that is not normalized.

There are no hard mismatches in the current match-history corpus. Match-related gaps are currently warning-based partial visibility cases rather than silent misclassification.

## Downstream Consumption Recommendation

Safe to consume now:

- event identity and listing fields from `event_catalog`: `name`, `source_url`, `city`, `country`, `country_code`, `status`
- event core fields from `event_detail`: `name`, `venue_name`, `country`, `organizer_name`
- athlete core identity fields from `athlete_profile_enrichment`: `full_name`, `country`, `country_code`, `age`, `belt_rank`, `organization_source_id`
- athlete-centric competitive record fields from `athlete_profile_enrichment`: `match.outcome`, `match.finish_method`, `match.result_text`, `match.opponent_name`, `match.event_name`, `match_summary.total_matches`, `match_summary.total_wins`, `match_summary.total_losses`
- participant identity and registration fields except canonical age: `full_name`, `country`, `organization_source_id`, `registration.division`, `registration.rank`, `registration.weight_class`, `registration.actual_weight`, `registration.seed`
- academy identity fields: `name`, `country_code`

Consume as informational only:

- event descriptions
- academy website/social/image metadata
- athlete match `score_text`, `round_label`, `weight_class`, `opponent_source_id`
- athlete method-specific aggregate buckets such as `wins_by_submission` until the corpus grows further
- participant `profile_url`

Do not import into the main backend yet:

- `event_participants.person.age`
- `academy_catalog.organization.country`
- any event-centric match reconstruction fields not yet proven through archived fixture audit

## Remaining Risks

- this is a curated fixture audit, not yet a large-scale live production sampling run
- more source variants are still possible across Smoothcomp subdomains and page layouts
- some match-level fields have high exactness but low sample volume; those remain provisional until the corpus grows
- public results/bracket pages are promising secondary sources, but they are not yet part of the deterministic audited path

## Next Recommended Step

1. fix the confirmed normalization bug for `event_participants.person.age`
2. extend `event_detail` parsing to read visible body descriptions when JSON-LD is absent
3. extend academy detail normalization to capture visible country/location text
4. add archived bracket/results-page fixtures to verify event-centric match reconstruction before freezing that part of the contract
