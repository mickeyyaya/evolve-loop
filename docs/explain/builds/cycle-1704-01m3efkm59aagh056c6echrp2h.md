# Build Explanation — Cycle 1704

## Build Binding
- Cycle: 1704
- Base SHA: 2198b2d3bbe6ba0e861fb8d496b3d322a341fb16

## Summary
This cycle re-checks the two technique families that the 2026-07-05 token-optimization survey parked as
"track, don't build". It delivers a research dossier with one BUILD/DEFER verdict per family, and an
ADR-0099 solution package that compares three watch strategies. Both families stay DEFER. Each verdict
names the observable triggers that would flip it, so the next re-check has a concrete test. No verdict is
BUILD, so no inbox record is emitted. No source code changed.

## Rationale
The ollama-lane verdict rests on measured telemetry, not on the survey's assumption that a self-hosted
lane exists. Of 1,239 dispatches in cycles 1600–1705, one went to ollama, and it failed before launch
with 0 tokens. Ollama also ships no KV import/export, and its persistence PR is open and deprioritized.
Any KV technique would multiply against zero traffic. The latent-channel verdict rests on a fresh
literature and vendor sweep. The newer work is white-box, and causal audits show its gains are
conditional. Vendor "compaction" is same-vendor and either text or opaque, and no fleet CLI imports
another model's latent state. The document cycle's kernel floor needs a `solutions/<id>/` package, so the
watch strategies are laid out as options and linked to the dossier. That keeps the two documents one
deliverable.

## Changed Areas
- `docs/research/token-optimization-2026/frontier-watch-2026-09-26.md` — new dossier. It holds the
  measured lane baseline with the recorded `llm-calls.ndjson` query that produces it, the ollama
  serving-surface check with derived token and latency estimates,
  the latent literature and vendor re-survey, a DEFER verdict per family, and the flip triggers.
- `docs/research/token-optimization-2026/README.md` — indexes the dossier as a companion file and dates
  the "Track, don't build" line with the re-check result, so the package points to its own follow-up.
- `solutions/token-frontier-watch/assumptions-and-evidence.md` — numbered assumptions (A1–A6) and
  evidence (E1–E14), plus the derived arithmetic that every option cites.
- `solutions/token-frontier-watch/options/1-defer-on-observable-triggers.md` — the recommended strategy:
  DEFER with trigger-gated quarterly re-checks.
- `solutions/token-frontier-watch/options/2-build-ollama-kv-spike-now.md` — the BUILD-now alternative,
  with its costs and zero realized effect.
- `solutions/token-frontier-watch/options/3-retire-watch-and-redirect.md` — the retire alternative and its
  flip-detection gap.
- `solutions/token-frontier-watch/recommendation.md` — the options comparison matrix and the
  recommendation, with per-family verdicts that match the dossier and a link to it.
- `.evolve/evals/token-frontier-watch.md` — the TDD-authored eval (eight score caps). It is committed
  unmodified so the caps that grade this deliverable ship with it.
- `docs/explain/builds/cycle-1704-01m3efkm59aagh056c6echrp2h.md` — this explanation document.
- `.evolve/inbox/2026-07-06T02-20-00Z-token-frontier-watch.json` — removed from the open inbox. The
  ship step claimed the item this cycle delivers, so it no longer waits for triage.
- `.evolve/inbox/consumed/2026-07-06T02-20-00Z-token-frontier-watch.json` — the same record, moved here
  by the ship step. It is stamped `consumed` (`via: ship`), so the watch item is closed and not re-queued.
  Its acceptance criteria are unchanged.

## Design Decisions
The dossier lives in `docs/research/token-optimization-2026/`, not the inbox's `knowledge-base/` path.
The package moved there on 2026-08-05, and `knowledge-base/` is now a runtime write surface. The ollama
estimates are shown twice: the realized value at the measured share (0) and a counterfactual that routes
triage to ollama once per cycle. The counterfactual is interpolated between the two rows of a published
prefill measurement (ollama PR #17953) that bracket the median triage-prompt size. Its per-token cost
grows with prompt length, so the largest row's rate would overstate a shorter prompt. It shows the
ceiling is small, not just that today's value is zero. Three options with different mechanisms were compared (watch, build, retire), not three sizes of one lever.
The audited commit was replayed onto a newer main whose only addition is another cycle's
`knowledge-base/cycles/` closeout. This document is therefore bound to that new base. No deliverable
byte changed in the replay.

## Verification
All eight score caps in `.evolve/evals/token-frontier-watch.md` exit 0, including the `evolve solution
check` engine, the dossier location and verdict parse, citation resolution, the quantified ollama
estimates, the BUILD⇒record rule, the recommendation-to-dossier link, the size-matched latency
counterfactual and the recorded lane-share query. The cycle's docs-tree-only
check (AC3) reports every changed path inside the docs tree.

## Compatibility
The change is documentation and research only. No code, flag, profile, schema or runtime behavior
changed.

## Limitations
The literature and vendor checks read abstracts and documentation. They did not exercise the APIs or
reproduce any result. Three estimate inputs are unmeasured assumptions: prefix reuse across ollama REPL
sessions, linear prefill scaling from 3B to 8B, and linear interpolation in prompt length between the
two PR #17953 rows (5,181 and 9,932 tokens) that bracket the ≈7,464-token triage prompt. None of them
can change the verdict, because the lane carries no traffic.
