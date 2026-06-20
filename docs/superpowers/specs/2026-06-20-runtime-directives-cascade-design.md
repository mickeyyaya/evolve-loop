# Runtime Operator-Directives Cascade — Design Spec

**Date:** 2026-06-20
**Status:** design (awaiting review → implementation plan)
**Author:** main session (interactive) + operator

## Context

The flag-reduction campaign exposed a gap: the main (interactive) session has no way to
push a new standard, rule, or policy into a **running** evolve loop. Two reasons:

1. A loop's goal/standards are fixed at launch (`--goal-text` is a process arg) and its
   `policy.json` is read **once** at composition (`cmd_cycle.go:312`, baked into the
   orchestrator) — never re-read per cycle.
2. Loops run in **separate sibling worktrees** with separate project roots (e.g.
   `evolve-loop-flagreduce` ≠ main), so a per-project file can't reach them all, and
   `AGENTS.md` is branch-stale + read only by agent convention (unreliable).

This spec adds a **runtime operator-directives cascade**: two new layers — a
machine-**global** layer and a **per-loop** layer — that every loop re-reads at the
**start of each cycle** and that are **injected into every phase agent's prompt**. The
main session edits one Markdown file; all loops converge on the next cycle boundary,
without restart. It is the *delivery mechanism* for standards such as the
[Flag → Parameter Conversion Standard](../../../knowledge-base/research/flag-parameter-conversion-standard.md)
(the first concrete L0 directive).

## Goals / Non-goals

**Goals**
- Main session updates standards/rules across all running loops by editing a file.
- Each loop loads the directives at the **beginning of a cycle** (snapshotted for the
  whole cycle; never mid-cycle), so behavior is consistent within a cycle.
- Both a **global** (all loops) and a **per-loop** (one loop, by lane) layer.
- Directives reach agents reliably (programmatic injection, not read-convention).
- Every cycle records which directives version steered it (audit/reproducibility).

**Non-goals (v1)**
- Structured/keyed rules with machine-enforced conflict resolution (prose only in v1).
- A directive that can weaken or disable a binary gate (impossible by construction).
- A `directive → enforceable gate` bridge (deferred; see Future).
- An `EVOLVE_*` feature flag to toggle the system (forbidden by standing rule — control
  is config-as-code: a file's presence/absence is the switch).

## Architecture — the layered cascade

Extends the existing static cascade with two **runtime** layers (re-read per cycle):

```
L0  Global operator directives   ~/.claude/evolve/directives.md      NEW · machine-global · ALL loops
L1  Per-loop directives          ~/.claude/evolve/loops/<lane>.md    NEW · one loop (runscope lane)
L2  Project standards            <worktree>/AGENTS.md                existing (static, branch-scoped)
L3  Role                         agents/<role>.md + profile          existing
L4  Task                         goal-text + cycle context           existing
```

**Precedence:** most-specific-wins for non-safety guidance (L1 can refine L0 for one
loop), but a narrower layer may only **restrict/append**, never **remove**, a broader
directive (the shared-values "restrict-not-remove" invariant = the existing integrity
floor). In v1 (prose), this is realized by **labeled concatenation** + a precedence note,
not keyed merge: agents see `## Operator Directives` with a `### Global` block then a
`### This loop (<lane>)` block, plus a line stating global safety directives are
authoritative.

## Components & interfaces

A new leaf package `internal/directives` — **pure, environment-agnostic, public-API**
(held to the Flag→Parameter Conversion Standard itself: black-box tests, no `os.Getenv`,
100% API coverage, enrolled in the env-agnostic guard).

```go
package directives

// Layer is one resolved directives source.
type Layer struct { Name string; Path string; Body string } // Body "" = absent

// Set is the merged, cycle-snapshotted directives.
type Set struct {
    Global  Layer
    PerLoop Layer
    Merged  string // the rendered "## Operator Directives" block ("" = none)
    Version string // sha256 of Merged (stamped to the ledger); "" when Merged==""
}

// Load reads the two explicit paths (DI — caller resolves them; Load touches no env),
// renders the merged block, and computes Version. Missing/unreadable file ⇒ that Layer
// is absent (fail-open); never returns a blocking error.
func Load(globalPath, perLoopPath string) Set

// Resolve is the composition-root helper that derives the paths from home dir + lane.
// Kept separate from Load so Load stays pure/testable.
func Resolve(homeDir, lane string) (globalPath, perLoopPath string)
```

- **`Load` is pure** (paths in → `Set` out, fail-open). All env/home/lane resolution is
  done by the caller (`Resolve` + composition root), so `Load` is unit-testable with
  temp files and zero environment — and the package passes the env-agnostic guard.
- **Rendering** wraps each present layer in a labeled section and prepends a fixed
  precedence/authority preamble (so the LLM knows global safety directives win and that
  these are guidance, not gate overrides).

## Lifecycle / data flow

```
main session  ──edits──▶  ~/.claude/evolve/directives.md   (and/or loops/<lane>.md)
                                   │
   ┌───────────────────────────────┴──────────── per loop, every cycle ─────────────┐
   │ planCycle (cyclerun.go) — cycle START, alongside catalog-refresh:               │
   │   1. lane   := runscope lane (already known to the loop)                        │
   │   2. paths  := directives.Resolve(home, lane)                                   │
   │   3. set    := directives.Load(paths...)         (fail-open, WARN on read error)│
   │   4. snapshot `set` into the cyclePlan (immutable for the whole cycle)          │
   │   5. ledger.append({kind:"operator_directives", cycle, version:set.Version})    │
   └───────────────────────────────┬──────────────────────────────────────────────-┘
                                    │  set.Merged threaded through cyclePlan → each phase
                                    ▼
   bridge dispatch (bridge.go): prepend set.Merged as the "## Operator Directives"
   block ABOVE the existing "## Rules"/agent body — via the existing injection path
   (template-injected once per cycle, reused for every phase agent → no N× re-read).
```

- **Load point:** `internal/core/cyclerun.go:planCycle` (the per-cycle preamble where
  catalog-refresh already runs the identical best-effort/WARN/fail-open pattern).
- **Snapshot:** loaded once per cycle into `cyclePlan`; mid-cycle file edits apply on the
  **next** cycle (the "beginning, not the middle" guarantee).
- **Injection:** threaded into the phase dispatch request and prepended in `bridge.go`
  adjacent to the existing `injectRulesPrefix` seam (no new dispatch path).

## Versioning & audit

`Set.Version = sha256(Merged)`. Each cycle appends an `operator_directives` record
(`{cycle, version, global_present, perloop_present}`) to the existing **tamper-evident
ledger**. This gives: (a) exact reproducibility ("which directives steered cycle N"),
(b) drift visibility (version changes over a batch), (c) tamper-evidence via the existing
hash chain. No new state store — reuses the ledger.

## Integrity floor / safety (critical)

Directives are **guidance prose only**, prepended to agent prompts. They **cannot** alter
the binary trust boundary:

- The hard gates — EGPS `red_count==0`, ship-gate attestation, role/phase kernel gates,
  `policy.json` `ClampPlanToFloorWith` floor — are **code**, computed from sandbox exit
  codes, and ignore prose. A directive saying "skip audit" or "disable the floor" is
  **inert**.
- Therefore directives can only *add* restriction in practice, never remove a safety
  guarantee — satisfying "restrict-not-remove" structurally, not by trust.
- The directives block is clearly labeled as operator guidance so agents weigh it
  correctly against their role contract and the gates.

## Authoring

- **Primary:** edit the Markdown directly (like `AGENTS.md`).
- **Ergonomic CLI:** `evolve directives show|edit|set|validate [--lane <lane>]` —
  `show` renders the merged block + version a loop would see; `validate` checks the file
  is readable/UTF-8 and warns on a likely gate-override phrasing; `set`/`edit` write
  atomically (temp+rename) so a loop never reads a half-written file.

## Error handling

Fail-open everywhere (a directives problem must never stop a loop):
- Missing file → that layer absent. Both absent → no injection (byte-identical to today).
- Unreadable/again-invalid UTF-8 → WARN to the cycle log, treat layer as absent.
- Atomic writes (temp+rename) prevent torn reads under concurrent loops.
- No locking needed for reads (read-only, last-write-wins file semantics; the snapshot
  per cycle bounds inconsistency to "you get cycle-start contents").

## Testing strategy

Held to the standard this system distributes (dogfood):
- `internal/directives` is **black-box** (`package directives_test`), **env-free**
  (temp files only, no `t.Setenv`/`os.Getenv`), table-driven over the matrix: both absent
  → empty Set + empty Version; global-only; per-loop-only; both present → labeled merge +
  stable Version; unreadable path → fail-open; identical content → identical Version
  (determinism); atomic-write race (concurrent read sees old or new, never torn).
- Enroll `internal/directives` in `paramPackages` (env-agnostic guard) + apicover.
- A `planCycle` test asserting: directives loaded once per cycle, snapshot immutable
  mid-cycle, ledger record written with the version, fail-open on bad file.
- A `bridge` test asserting the merged block is prepended for a representative phase and
  is **absent** when the Set is empty (no behavior change when unused).

## Integration points (files)

- `internal/directives/` (new package + tests).
- `internal/core/cyclerun.go` (`planCycle`): resolve lane → `Load` → snapshot → ledger.
- `internal/core/*` cyclePlan/request struct: carry `directives.Set` to phases.
- `internal/adapters/bridge/bridge.go`: prepend `Set.Merged` near `injectRulesPrefix`.
- `internal/runscope`: reuse lane resolution (no change).
- `internal/paths` or composition root: `Resolve(home, lane)` + `os.UserHomeDir` at the
  boundary only (keeps `directives.Load` pure/env-agnostic).
- `cmd/evolve/cmd_directives.go` (new): `evolve directives` CLI.
- ledger: new record kind `operator_directives` (additive).

## Risks & mitigations

| Risk | Mitigation |
|---|---|
| Prose directive misread as gate override | Labeled "guidance"; gates are code + inert to prose; `validate` warns on override phrasing |
| Directives bloat agent prompts (N agents × size) | Template-inject once per cycle; keep directives concise; it's one shared block |
| A bad global file breaks every loop | Fail-open (absent layer) + atomic writes + `validate`; never blocks a cycle |
| Per-loop lane drift (wrong file targets wrong loop) | Lane is the loop's existing runscope identity; `show --lane` previews exactly what a loop sees |
| Scope creep into a parallel policy system | v1 is prose-only; structured/keyed rules + gate bridge explicitly deferred |

## Future (deferred, out of v1)

- **Directive → gate bridge:** promote specific structured directives (e.g. "env-agnostic
  guard required for conversions") into auditor-checked acceptance criteria. Chosen
  enforcement posture for now is guidance + audit only.
- **Keyed-rule merge + conflict escalation** (the full shared-values model) if prose
  proves insufficient.
- **L2 (`AGENTS.md`) programmatic injection** to retire the read-convention entirely.

## Verification (when implemented)

```
cd go && go test ./internal/directives/... ./internal/core/... ./internal/adapters/bridge/...
gofmt -s + go vet ; apicover -enforce exit 0 ; directives package at 100% API coverage
# end-to-end: write ~/.claude/evolve/directives.md, run one cycle, confirm the ledger
# operator_directives record + the block in the dispatched prompt artifact; empty file ⇒
# byte-identical-to-today dispatch.
```
