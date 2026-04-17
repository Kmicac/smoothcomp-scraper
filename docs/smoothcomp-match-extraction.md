# Smoothcomp Match Extraction Design

This document describes the current match-level extraction design for the Smoothcomp ingestion adapter and the evidence behind it.

## Source Discovery Summary

The provider exposes match/result information through several public-facing surfaces, but they are not equally reliable for deterministic ingestion.

### Source Families Investigated

1. athlete profile event history
   - Public provider shape: `/en/profile/{id}/events`
   - Current adapter source: JSON payload already used by `smoothcomp.athlete_profile_enrichment`
   - Typical contents: event identity, registrations, opponent name, win/loss outcome, result text, result method, score text, limited bracket context
   - Stability: strongest currently-audited source
   - Nature: athlete-centric

2. athlete profile HTML summary
   - Public provider shape: `/en/profile/{id}`
   - Typical contents: profile identity, visible stats, summary win/loss counters, upcoming events
   - Stability: good as a reconciliation source, not detailed enough for full match history
   - Nature: athlete-centric

3. event results, brackets, finished matches, match pages
   - Public provider family indicated by Smoothcomp public help and page navigation patterns
   - Typical contents: richer event-centric result context, bracket placement, fight result pages, match detail pages
   - Stability: useful candidates, but not yet proven enough in the archived deterministic corpus
   - Nature: event-centric

Smoothcomp documentation confirms that athlete profiles expose history and results when events have published results, which matches the current primary source choice:
- athlete profile help: https://support.smoothcomp.com/article/398-athlete-profile
- bracket information help: https://support.smoothcomp.com/article/386-bracket-information
- results/top lists help: https://support.smoothcomp.com/article/387-how-to-view-results

## Current Design Choice

The adapter now supports athlete-centric competitive records using a two-source strategy:

1. primary source
   - profile events JSON
   - used for match-level records and method extraction

2. reconciliation source
   - profile HTML summary counters
   - used to emit a warning when visible profile summary totals disagree with derived match totals

This design is deliberate:

- it avoids freezing a brittle event-centric result parser before enough evidence exists
- it produces useful downstream competitive records now
- it keeps provider-specific irregularities inside `internal/adapters/smoothcomp`

## Normalized Match Contract

The provider-neutral contract now includes:

- `matches[]`
  - `source_id`
  - `source_url`
  - `event_source_id`
  - `event_name`
  - `athlete_source_id`
  - `opponent_source_id`
  - `opponent_name`
  - `opponent_country`
  - `division`
  - `age_category`
  - `rank`
  - `weight_class`
  - `outcome`
  - `finish_method`
  - `result_text`
  - `score_text`
  - `bracket_label`
  - `round_label`
  - `placement`
  - `confidence`
  - `starts_at`
  - `raw_reference_ids`
  - `attributes`

- `match_summaries[]`
  - `athlete_source_id`
  - `scope`
  - `confidence`
  - `total_matches`
  - `total_wins`
  - `total_losses`
  - `wins_by_submission`
  - `wins_by_points`
  - `wins_by_decision`
  - `wins_by_dq`
  - `losses_by_submission`
  - `losses_by_points`
  - `losses_by_decision`
  - `losses_by_dq`
  - `raw_reference_ids`
  - `attributes`

## Outcome and Method Normalization

Outcome normalization currently maps provider signals into stable adapter values such as:

- `win`
- `loss`
- `walkover`
- `unknown`

Finish-method normalization currently canonicalizes visible provider text into:

- `submission`
- `points`
- `decision`
- `dq`
- `walkover`
- `unknown`

If the provider exposes the match but not the finish detail, the adapter does not invent precision. It emits a `match_finish_method_missing` warning instead.

## Warning Model

Current match-related warnings:

- `match_finish_method_missing`
  Match is visible, but method detail is absent.
- `match_opponent_hidden`
  Match is visible, but opponent identity is absent or partial.
- `match_context_partial`
  Match exists, but bracket/category context is incomplete.
- `match_summary_inconsistent_with_profile`
  Derived totals from visible matches do not agree with visible summary counters in profile HTML.

These warnings are part of the published normalized envelope and are audited.

## What Is Ready Today

Ready now:

- athlete-centric match history from profile events JSON
- total win/loss derivation from visible match history
- visible finish method extraction when the provider includes it
- warning-based degradation when source detail is partial

Not ready to freeze yet:

- event-centric reconstruction from public bracket/results pages
- strong guarantees for bracket placement and round semantics across multiple unsupported layouts
- aggressive canonicalization of opponent identity when the provider hides IDs

## Next Step

The next evidence-driven increment for match data should be:

1. archive representative bracket/results/match-page fixtures
2. implement an event-centric secondary source parser behind the same normalized contract
3. merge athlete-centric and event-centric records only after deterministic audit proves the merge semantics
