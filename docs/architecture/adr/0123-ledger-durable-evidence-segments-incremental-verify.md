# ADR-0123: The audit ledger keeps durable evidence, one reader, sealed segments and an incremental verify

- **Status:** Proposed (2026-10-06). Approved as a plan by the operator in plan mode. Each decision moves to Accepted as its component lands; see the [plan](../../plans/ledger-restructure-2026-10.md) §4.
- **Amends:**
  - [ADR-0048](0048-work-conservation-fast-reland-resilient-ship.md): the epoch anchor stays, and routine verify no longer re-walks the anchored prefix.
  - [ADR-0081](0081-boot-base-divergence-halt-and-in-band-ledger-seal.md): in-band seals stay, and segment seals gain a manifest.
  - [ADR-0105](0105-identity-preserving-fleet-rebase.md): its finding F5 is resolved, and the B3 carry evidence becomes durable.
- **Evidence:** [ledger structure research](../../research/ledger-structure-research-2026-10.md), and the 2026-10-06 carry incident (`docs/incidents/2026-10-06-carry-refused-ledger-verify-f5.md`, landing with C1–C5).
- **Terms:** carry, patch-id, epoch anchor, segment and checkpoint are defined in the [plan's Terms block](../../plans/ledger-restructure-2026-10.md).

## Context

The loop's audit ledger, `.evolve/ledger.jsonl`, is one append-only, hash-chained JSONL file: 47 MB and about 160k lines by 2026-10-06. Four problems surfaced together.

1. **Noise.** `phase_skipped` is 52% of the bytes. It is one envelope-only line per skipped phase per routing decision, carries no reason, and no Go code reads it (docs, a persona and evals do).
2. **Fragile evidence.** Some records commit to side files that live in disposable places. The fleet-rebase carry wrote its proof diffs into the cycle worktree, and the worktree was deleted. `ledger.Verify` re-derives each carry's patch-id from those files and treats a missing file as a broken chain. From the first deletion on, the whole-ledger Verify that ship runs before honouring a carry failed. **Every carry was refused (6 of 6)** and re-audited, silently. ADR-0105 had named this hazard (F5) and deferred it.
3. **No access path.** There is no index or manifest. Ship makes 3–6 whole-file passes to find one run's newest row. `ledger tail` loads everything. Agents are pointed at raw lines.
4. **Linear, racy verify.** Every carry check re-hashes all history, and it reads the tip after walking the file.

`evolve ledger seal` already exists: it moves history byte-for-byte into gzipped segments behind a chained `segment_seal` line. It has never run, because the readers and plain Verify only see the live file.

Established systems solve these problems with known patterns (research §3–§9):
- checkpoints over sealed history, instead of re-walking a chain (Certificate Transparency, Trillian, sumdb);
- segments with small sidecar summaries (Kafka);
- commitments to the hash of an artifact held in content-addressed storage;
- projections for reads, never rewriting history (event sourcing);
- a stable, hierarchical record envelope (OpenTelemetry GenAI spans);
- bounded, projected access for LLM agents.

## Decision

1. **History is never rewritten** (all decisions). Existing bytes stay as they are. Every change applies going forward, or as an additive, chained record.
2. **Integrity-bearing evidence is content-addressed and durable** (D2, D6).
   - Artifacts that a gate or Verify re-reads (composition diffs, audit reports, contracts and verdict artifacts) are stored write-once at `.evolve/ledger-artifacts/sha256/<2>/<62>`. gc never prunes that path.
   - Records name them by sha256, and Verify resolves a sha from the store before any recorded path.
   - A missing committed artifact is still a chain break: fail closed.
   - Prompts and responses keep normal run retention.
3. **The six broken carry records are repaired from git, not rebaselined** (D3). `evolve ledger evidence restore` regenerates each diff from the objects its record names. It accepts a diff only if it reproduces the recorded patch-id, and then stores it. Strict verification therefore keeps covering everything after the epoch anchor.
4. **Skipped phases are recorded once per decision, with their reasons** (D1). A `routing_decision` record carries `skipped_phases: [{phase, reason}]`, and no new `phase_skipped` lines are written.
5. **One scanner reads the ledger.** It streams forward or backward in chunks, prefilters on raw bytes and exits early, then reads sealed segments after the live file. Every reader migrates onto it.
6. **Agents read the ledger only through bounded verbs:**
   - `evolve ledger query`, with filters, projected fields and a limit;
   - `evolve ledger cycle N --summary`, one screen;
   - a lossless `tail`.

   Skills and personas never name `ledger.jsonl`.
7. **History is sealed at wave boundaries into segments with an in-chain manifest** (D4).
   - Each `segment_seal` record carries a per-segment summary and `manifest_sha256`.
   - The manifest file is a projection that can be rebuilt.
   - Sealing happens only at a wave boundary, and only once every reader can read segments.
8. **Routine verify is incremental; deep verify is the audit** (D5).
   - Per ship: a strict walk of the live tail, the manifest chain, and a hash of each segment's bytes.
   - At each boundary: `verify --deep` keeps the full re-walk and the patch-id re-derivation.
   - The tip is read *before* the chain it vouches for, under the ledger lock.
9. **JSONL stays the canonical format** (D7). Indexes, manifests and summaries are derived artifacts, never the source of truth.

## Alternatives considered

- **Rebaseline seal over the six broken records**, as with the earlier line-78729 break. It needs no new code, but strict verification would restart after the seal and the carries' history would stop being checked. Rejected, because the evidence can be proven back.
- **Move every artifact into the store,** prompts and responses included. Every prompt and response would be kept forever, about 110 MB a year. Deferred: the store is integrity-scoped first, and its scope is config if it ever widens.
- **Compact existing history** by rewriting `phase_skipped` lines. That would break append-only immutability. Rejected.
- **A full Merkle tree with tiles** (RFC 9162, tlog-tiles). This is the strongest model, with O(lg N) inclusion proofs for outside verifiers. It is larger than the problem, since the ledger has no outside verifier and runs at about 10⁵ records per quarter. Deferred; segment checkpoints capture most of the benefit.
- **A hierarchical record envelope** (`wave`, `phase`, `attempt`, `span_id`, after the OpenTelemetry GenAI spans). Deferred (plan D8): the query verbs' filters cover today's needs; revisit if export to tracing tools is wanted.
- **A database (SQLite or DuckDB) as the store of record.** Rejected as the source of truth, because it would lose the append-only text log, its hash chain and its tooling. It remains welcome as a *derived* index.

## Consequences

- **Carries work.** A byte-identical fleet-rebase carry can be re-proven at ship no matter when its worktree was cleaned up. The re-audit it exists to avoid stops happening.
- **The forward stream shrinks** by about 71% in lines and 56% in bytes. History compresses to about 10 MB of gzipped segments, and the live file holds about one wave.
- **Ship's verify falls** from about 0.3 s and 130 MB to about 25 ms. Its ledger reads fall from several whole-file passes to kilobytes.
- **Agents can be pointed at ledger history** without exhausting their context.
- **New surface to maintain:** a content store, a scanner, query verbs, and segment manifests. Each is small, tested and documented, and the derived pieces can be rebuilt from the canonical log.
- **The grep-by-kind contract for `phase_skipped` ends** for new records. Its readers move to `skipped_phases`: the scheduling doc, `internal-core.md`, two knowledge docs, the orchestrator persona's resume rule, and the evals `ledger-skip-source*`, `user-phase-pipeline-hardening` and `plan-time-dispatchability-clamp`.
