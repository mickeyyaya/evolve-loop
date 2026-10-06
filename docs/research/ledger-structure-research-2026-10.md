# Ledger structure: how append-only, verifiable logs stay cheap to read (2026-10-06)

> **Purpose.** Evidence for the [ledger restructure plan](../plans/ledger-restructure-2026-10.md). The operator asked to make the audit ledger, `.evolve/ledger.jsonl`, *"more structured, efficient to load without loading all data into the context window"*. This record measures the ledger, surveys how established systems solve the same problems, and says what each answer means here. The plan decides; this record informs. Terms (carry, patch-id, epoch anchor, segment, checkpoint, plane, wave boundary) are defined in the plan's **Terms** block.
>
> **Trigger.** On 2026-10-06, `evolve ledger verify` reported the plane ledger broken at line 154453. A fleet-rebase carry record pointed at a proof diff inside a cycle worktree that had since been deleted. Because ship re-verifies the whole ledger before honouring a carry, every byte-identical carry was refused: six of six (cycles 1766–1810). Companion incident: `docs/incidents/2026-10-06-carry-refused-ledger-verify-f5.md`, landing with the carry fix.

## 1. The ledger today (measured on the runtime plane, 2026-10-06)

| Property | Value |
|---|---|
| File | `runtime/.evolve/ledger.jsonl`, plus `ledger.tip`, `ledger.lock`, `ledger-anchor.json`, `ledger-rebaseline.json` |
| Size | **47 MB, 159,823 lines**, starting 2026-05-07 |
| Shape | One flat, append-only JSONL file. Each line is chained to the previous line by hash. |
| Verbs | `evolve ledger verify`, `seal`, `tail`, `anchor`, `rebaseline` |
| Query or index | **None.** A reader that wants one cycle's records scans the whole file. |
| Direct readers | **27** non-test Go files reference the ledger path |
| Verify cost | `ledger.Verify` walks every line after the epoch anchor (seq 113890). The carry check at ship calls it every time. |
| Side files | Some record kinds (`composition-verdict`) name files on disk that Verify re-reads. If a file is missing, the chain counts as broken. |

**What the bytes are made of:**

| Record kind | Lines | Share of bytes |
|---|---:|---:|
| `phase_skipped` | 100,204 | **52.3%** (24.6 MB) |
| `agent_subprocess` | 13,279 | 15.2% |
| `routing_decision` | 15,053 | 13.1% |
| `phase` | 14,472 | 6.5% |
| `stop_review` | 5,750 | 3.4% |
| `advisor_prompt` / `advisor_response` | 2,019 each | 1.8% each |
| `plan_rejections` | 1,843 | 1.6% |
| everything else | — | about 4% |

**What these numbers point to** (from a design study that read the code and sampled the file):
1. **Noise.** Half the file records phases that did *not* run: one `phase_skipped` line per skipped phase per routing decision, about 14 per decision in recent waves (8.6 over the whole file), up to 74. These lines are envelope plus `source:"router"`. They say who skipped a phase, never why. No Go code reads them; only docs, one persona and some evals do.
2. **The envelope dominates, not payloads.** Records are 220–570 bytes. Prompts, responses and reports are *already* side files under `.evolve/runs/cycle-N/`, referenced by path plus sha256 (advisor prompts average about 40 KB). So moving payloads into a content-addressed store buys **durability, not ledger size**. Evidence decays today: 25–30% of artifacts referenced in the last 20k lines are already deleted by run-dir retention, and 21% of the surviving `agent_subprocess` artifacts no longer match their recorded sha, because they were overwritten in place.
3. **No random access.** No index or manifest exists. Ship runs 3–6 whole-file passes (each about 0.27 s, 130–200 MB RSS) to find the newest row for a run. `evolve ledger tail --n 1` loads the whole file and is lossy (it drops `method`, `patch_id`, `gate_results`). `skills/loop/phase2-discover.md:41` feeds the raw last 3 lines to an LLM, and in 56% of samples those lines are all `phase_skipped`.
4. **Verify is linear, racy and fragile.** Every check re-hashes the whole history, reads `ledger.tip` *after* walking (racing concurrent appends), and depends on side files that live in disposable places: the carry diffs in cycle worktrees.
5. **Segments exist but have never run.** `evolve ledger seal` already moves history byte-for-byte into gzipped `ledger-segments/` with a chained `segment_seal` line. It has never run on the plane, because plain `Verify` and most readers only understand the live file, so sealing today would hide records from them.

## 2. Questions this research answers

1. How do tamper-evident logs prove integrity *without* re-reading everything? (§3)
2. How do high-volume append-only logs give random access? (§4)
3. Where should large payloads live in a verifiable log? (§5)
4. How do event-sourced systems keep reads cheap over a long history? (§6)
5. What structure makes agent activity easy to filter? (§7)
6. How should an LLM agent read a log too big for its context? (§8)
7. How do we keep ad-hoc analysis possible without new infrastructure? (§9)

## 3. Verifiable logs: checkpoints instead of re-walking the chain

**How it works.** Certificate Transparency ([RFC 6962](https://datatracker.ietf.org/doc/html/rfc6962), [RFC 9162](https://www.rfc-editor.org/info/rfc9162/)), Google's Trillian ([verifiable data structures](https://github.com/google/trillian/blob/master/docs/VerifiableDataStructures-Latest.md)) and Go's checksum database ([Russ Cox, *Transparent Logs for Skeptical Clients*](https://research.swtch.com/tlog)) store entries in a **Merkle tree**, not a linear hash chain.
- **Checkpoints.** A *checkpoint* (signed tree head) records the log's size and root hash. It commits to every entry so far.
- **Inclusion proofs** show that one entry is in the log in O(lg N) hashes.
- **Consistency proofs** show that a newer checkpoint extends an older one. That is the append-only guarantee, again in O(lg N).
- **Cost.** By Cox's figures, a proof against a log of 100 million records needs about three hash *tiles*, about 24 kB.
- **Tiles.** In [C2SP tlog-tiles](https://github.com/C2SP/C2SP/blob/main/tlog-tiles.md), hashes are stored as fixed-height, immutable tiles. With height 8 each tile is about 8 kB, and tiles can be cached forever.
- **Who verifies what.** Ordinary clients verify only what they touch. A full audit (download and replay everything) is reserved for a few dedicated auditors ([transparency.dev](https://transparency.dev/articles/logs-a-verifiable-transport-layer/)).

**What we take:**
- **The checkpoint idea.** Seal history into segments and record each segment's head hash as a checkpoint. Routine verification (for example the carry check at ship) then covers only the *live tail* plus the checkpoint chain, not all 47 MB. A full re-walk remains available as an explicit audit (`verify --deep` already exists).
- **The audit/verify split.** Routine checks are cheap. The exhaustive audit runs rarely, on purpose.

**What we leave, for now.**
- A full Merkle tree with tiles. A per-segment checkpoint chain captures most of the gain at our scale, which is about 10⁵ records per quarter, not 10⁸.
- The Merkle tree stays a recorded upgrade path if the ledger ever has to prove inclusion to an outside party.

## 4. Segmented logs with sidecar indexes (Kafka)

**How it works.** Kafka stores each partition as a series of **segments**:
- **Segments.** A segment rolls over at `segment.bytes` and is named by its first offset.
- **Sidecar indexes.** Each segment has two small sidecars. The `.index` is a *sparse* map from offset to byte position, one entry per ~4 KB of log by default. The `.timeindex` maps timestamp to offset.
- **Lookup.** A read binary-searches the sparse index and scans a few kilobytes.
- **Retention.** Retention and deletion work per segment.
- Sources: [Conduktor: Kafka log retention and segments](https://conduktor.io/blog/understanding-kafka-s-internal-storage-and-log-retention), [Kafka storage internals](https://rohithsankepally.github.io/Kafka-Storage-Internals/).

**What we take:**
- Name segments by their first sequence number, and seal them (immutable) at a size or wave boundary.
- Give each sealed segment a small sidecar index keyed by **cycle** and **kind**, plus its time range, built once at seal time.
- `evolve ledger query --cycle 1810` then reads only the matching segment ranges, kilobytes instead of 47 MB.
- The live (unsealed) tail stays small enough to scan.

## 5. Large payloads: commit to a hash, store the blob by content

**How it works.**
- **Hash, not blob.** Transparency logs keep leaves small. A leaf commits to the *hash* of a large artifact, and the artifact lives elsewhere. Trillian's binary-transparency example logs "a hash of the binary blob to reduce leaf size" ([Trillian transparent logging guide](https://github.com/google/trillian/blob/master/docs/TransparentLogging.md)).
- **Two hashes.** Trillian separates the **Merkle hash** (what is committed) from an optional **identity hash** (what counts as a duplicate).
- **Retrieval.** Content addressing, as in git objects, makes retrieval by hash location-independent.

**What we take:**
- **A blob store.** Most records already commit to side files by sha256; composition verdicts commit only by `patch_id` and a path. The gain is durability, not size. The artifacts a gate or Verify re-reads (composition diffs, audit reports, contracts, verdict artifacts; plan D2) are stored write-once at `.evolve/ledger-artifacts/sha256/<2>/<62>`, which gc never prunes, and records address them by sha256.
- **A root-cause fix.** This removes the root cause of the 2026-10-06 carry break, and of the slow decay of evidence under run-dir retention. Verify fetches a blob *by hash*, so where a worktree or run dir lived no longer matters.

## 6. Event sourcing: projections and snapshots, and no rewritten history

**How it works.**
- **Projections.** Event-sourced systems never replay the full stream on a read. They keep *projections*, materialized read models updated incrementally as events arrive.
- **Snapshots.** They take *snapshots* (state at sequence N) and replay only later events. Typical guidance is a snapshot every few hundred events, treated as a disposable accelerator. Sources: [Azure: event sourcing pattern](https://docs.microsoft.com/en-us/azure/architecture/patterns/event-sourcing), [snapshot strategies](https://www.nilus.be/blog/event_sourcing_snapshot_strategies_in_event_stores/).
- **Compaction is not free.** Replacing many old events with fewer coarse ones changes the stream, so it is no longer append-only and immutable ([planetgeek: prevent long event streams](https://www.planetgeek.ch/2026/05/05/event-sourcing-you-better-prevent-long-event-streams/)).

**What we take:**
- **Existing history is never rewritten.** The current file is sealed as legacy segments as-is, with its hash chain intact.
- **Coarser records apply going forward only,** where fine grain carries no information. For example, the `routing_decision` record carries `skipped_phases: [{phase, reason}]` instead of one `phase_skipped` line per phase.
- **Per-cycle summaries are projections,** derived and rebuildable from the ledger and never authoritative.

## 7. A hierarchy for agent activity (OpenTelemetry GenAI conventions)

**How it works.** The OpenTelemetry GenAI semantic conventions model agent work as a span tree:
- an `invoke_workflow` or `invoke_agent` span at the root;
- `chat` (model call) and `execute_tool` spans beneath it;
- common attributes under `gen_ai.*`: model, token usage, agent name, tool name, conversation id.

Sources: [OpenTelemetry GenAI semantic conventions (spec)](https://opentelemetry.io/docs/specs/semconv/gen-ai/), [OTel GenAI agent spans](https://www.matthewswong.com/en/blog/opentelemetry-genai-agent-spans-tracing/), [OTel GenAI implementation guide](https://hidekazu-konishi.com/entry/opentelemetry_genai_semantic_conventions_guide.html).

**What we take:**
- **A stable envelope on every record:** `seq, ts, kind, wave, cycle, phase, attempt, agent, span_id, parent_span_id`, plus a typed `body`.
- **Filtering without parsing.** Readers, human or LLM, filter by hierarchy (wave → cycle → phase → attempt → agent) without parsing bodies.
- **An export path.** It aligns with OTel, so we can export to standard tracing tools later if we want to.

## 8. LLM agents and logs bigger than their context

**How it works.**
- **Four moves.** Context engineering is framed as *Write, Select, Compress, Isolate*: never paste a large artifact into the context; keep it outside, insert a compact reference, and retrieve slices on demand ([LangChain: context engineering for agents](https://www.langchain.com/blog/context-engineering-for-agents)).
- **For logs:** chunk, filter and summarize first, then reason over the summaries ([Splunk: LLMs for log file analysis](https://www.splunk.com/en_us/blog/learn/log-file-analysis-llms.html)).

**What we take:**
- **Query verbs** with projections and hard limits: `evolve ledger query --cycle N --kind K --since T --fields a,b --limit 50 --json`.
- **A summary view.** A one-screen `evolve ledger cycle N --summary` built from the per-cycle projection.
- **A rule for agents.** Personas and skills are told to use these verbs and never to read `ledger.jsonl` directly.

## 9. Ad-hoc analysis without new infrastructure (DuckDB)

**How it works.** DuckDB reads newline-delimited JSON directly, in parallel, and infers columns. It also reads partitioned directory layouts as a single table ([DuckDB JSON](https://duckdb.org/2023/03/03/json), [JSON format settings](https://duckdb.org/docs/current/data/json/format_settings)).

**What we take:**
- **JSONL stays canonical.** It is human-readable, append-friendly and works with existing tools.
- **Segments are queryable as-is** by DuckDB for one-off analysis.
- **Derived indexes only.** Any SQLite or DuckDB index is a derived, rebuildable artifact, never the source of truth.

## 10. The options at a glance

| Option | Solves | Integrity impact | Migration | Effort |
|---|---|---|---|---|
| **A. Sealed segments + checkpoint chain** (§3, §4) | verify cost, retention | stronger: segment heads are recorded | seal the current file as legacy segments; no rewrite | medium |
| **B. Sidecar index per segment** (§4) | random access by cycle, kind and time | none (derived) | build at seal; backfill legacy once | small–medium |
| **C. Content-addressed artifacts** (§5) | durability: the F5 side-file break, evidence decay | stronger: hash-addressed | forward-only, plus a one-time restore of the six broken carry diffs (plan D3) | medium |
| **D. Coarser `phase_skipped`** (§6) | half the file's bytes | none if every reason is preserved | forward-only | small |
| **E. Record envelope** (§7) | filtering by hierarchy | none | forward-only, additive fields | small–medium: **deferred** (plan D8) |
| **F. Agent query verbs + cycle summaries** (§8) | context-window fit | none (read-only) | none | medium |
| G. Full Merkle tree + tiles (§3) | O(lg N) inclusion proofs for outsiders | strongest | larger redesign | large: **deferred** |

## 11. Conclusions handed to the plan

1. **Keep the chain; stop re-walking it.** Routine checks verify only the live tail plus the segment checkpoints, and the full walk becomes an explicit audit.
2. **Seal, never rewrite.** Existing history becomes legacy segments byte-for-byte, and every change applies forward.
3. **Keep the log small and its evidence durable:** the skipped-phase noise collapsed (D), and committed artifacts kept by hash in a store nothing prunes (C).
4. **Give agents bounded, projected access** (F). A full hierarchical record envelope (E) is deferred (plan D8): the query verbs' filters cover today's needs.
5. **Defer the full Merkle-tile design (G)** until an outside party needs inclusion proofs.

The plan ([`docs/plans/ledger-restructure-2026-10.md`](../plans/ledger-restructure-2026-10.md)) turns these conclusions into decisions, components and a rollout order.
