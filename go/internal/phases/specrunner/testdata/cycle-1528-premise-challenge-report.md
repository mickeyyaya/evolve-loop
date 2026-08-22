<!-- challenge-token: 59185fc485424870 -->

# Premise Challenge — cycle-1528

Gate position: Scout → Triage → **[Premise Challenge]** → build. Read-only; no source modified.

## Stated Premise

### Goal (restated verbatim)

Cycle `goal` (from cycle context / `goal_hash: 805f6ced…`):

> "Optimize per-agent token usage across all phase agents (Scout, Builder, Auditor, orchestrator, and supporting agents): trim verbose agent prompts, cut redundant context/artifact injection, and tighten report sizes so the pipeline is more stable (fewer context-limit and quota failures) and faster per cycle. Preserve every phase-integrity guarantee and gate behavior."

Lane-committed work (`triage-report.md` top_n, the ONLY item this lane may build):

> "surface-transient-api-error-under-artifact-timeout: add apiErrorLine classifier so a genuine transient upstream API-error line wins over the generic artifact-timeout summary in exit-81 cause selection — priority=H, files=go/internal/bridge/engine.go;go/internal/bridge/engine_artifact_timeout_apierror_test.go"

Scout's framing of the problem it solves (`scout-report.md#Key-Findings`, verbatim):

> "Net effect: when a driver's actual root cause of hanging until the artifact-wait deadline was a transient upstream API failure, the operator/forensic trail (`<agent>-launch-error.txt`, the wrapped error message, downstream `error_message` fields) shows only the generic 'waited Ns / extends consumed' summary — the specific, actionable API-error signal is silently dropped… operators cannot distinguish 'genuinely slow agent' from 'API was erroring the whole time' from the recorded cause."

### Success criteria (verbatim, from the materialized eval)

From `.evolve/runs/cycle-1528/.evolve/evals/surface-transient-api-error-under-artifact-timeout.md` — three `[code]` criteria:

1. **C1** — "When a launch's stderr contains an upstream-API-error diagnostic line (matching the new `apiErrorPattern` classifier, e.g. `[bridge] ERROR: API overloaded (529)`) followed later by the driver's `artifact-timeout: ...` summary line, and the launch exits 81 (ExitArtifactTimeout), the wrapped error message returned by `(*Engine).Launch` must contain the API-error line's text, not just the generic 'waited Ns' summary."
2. **C2** — "When stderr has NO api-error-shaped line, only the driver's generic `artifact-timeout: ...` summary, the wrapped message must still equal the pre-existing behavior (the summary line)."
3. **C3** — "When stderr contains an unrelated `[bridge] WARN: sandbox chatter` line before an artifact-timeout summary … the api-error classifier must NOT match it, and the summary line must still win."

### Stated assumptions (scout `## Hypotheses`, verbatim)

- **H1** — "Introducing a narrow API-error line classifier that runs BEFORE the `artifactTimeoutSummary` override, and letting an API-error match take precedence over the generic timeout summary, restores visibility without reintroducing the WARN-chatter regression the override was built to fix."
- **H2** — "The safest signal to key off is the same vocabulary used elsewhere in the codebase for classifying transient/quota driver failures (grep hits: 'overloaded', 'rate limit', '429', '529', 'quota') rather than inventing new terms."

### Implicit assumptions — NEVER STATED, flagged here

- **IA-1 (fatal candidate)** — that the stderr buffer `Engine.Launch` inspects (`stderrBuf`, engine.go:507-509) *can contain* a real upstream API-error line on an exit-81 death. Never asserted, never probed; C1 asserts it only over a hand-written fixture.
- **IA-2** — that a plain case-insensitive substring scan for `{overloaded, rate limit, 429, 529, quota}` (scout's "Implementation sketch") is safe against the other lines already present on the exit-81 stderr path.
- **IA-3** — that the diagnostic gap is real end-to-end, i.e. no *other* pipeline surface already tells the operator "the API was erroring."
- **IA-4** — that the cycle `goal` (token-usage trimming) and the lane's committed item are reconcilable; scout asserts the lane-scope contract overrides the goal text, citing `lane-scope.json`.

## Falsification Attempts

### Attack 1 — Goal-is-wrong: the diagnostic capability already exists — **CRITICAL**

The premise is that an operator "cannot distinguish 'genuinely slow agent' from 'API was erroring the whole time'." That capability is already built, single-sourced, and wired:

- On the exit-81 path the tmux driver captures the pane and writes it to the phase logs — raw → `cfg.StderrLog`, ANSI-stripped → `cfg.StdoutLog` (`go/internal/bridge/driver_tmux_repl.go:900-902`). The provider's 429/529/"overloaded" text renders *in the pane*, so it lands in those files.
- `channel.Producer.Run` builds one normalizer over exactly `<phase>-stdout.log` / `<phase>-stderr.log` (`go/internal/bridge/channel/producer.go:71-92`).
- The normalizer's `classifyPlain` scans those plaintext lines — its comment names this case explicitly: *"Plaintext infra markers (CLI error lines, tmux scrollback) surface as infra_failure"* (`go/internal/phasestream/classify.go:377-388`) — emitting `kind==infra_failure` with marker `api_529` / `api_429` / `rate_limit` (`go/internal/phasestream/marker.go:18-25`).
- `cycleclassify.Classify` Pass 2 consumes those typed events and returns `ClassInfrastructure`, documented as catching *"the API 529 / sandbox EPERM signal"* (`go/internal/cycleclassify/classify.go:105-155`, regex at :87).

So the answer to "was the API erroring the whole time?" is already produced for an exit-81 cycle, as a typed event and a cycle classification. What is genuinely missing is narrower than stated: one *string field* (the wrapped error's `cause`) does not repeat it. The goal as written ("operators cannot distinguish…") is falsified; the honest goal is "make the exit-81 cause line echo a signal the pipeline already classifies elsewhere," which is cosmetic, not the H-priority forensic gap triage committed to.

### Attack 2 — Fatal unstated assumption IA-1 is FALSE: no API-error line can reach that buffer — **CRITICAL**

`Engine.Launch` selects the cause from `stderrBuf`, and `stderrBuf` is filled solely by the driver's `deps.Stderr` writer:

- `code := callEngine.LaunchArgs(ctx, args, req.Env, io.Discard, &stderrBuf)` (`go/internal/bridge/engine.go:507-509`); the exit-81 branch reads `artifactTimeoutSummary(stderrBuf.String())` (engine.go:557-561).
- **Exit 81 has exactly ONE production emitter**: `return ExitArtifactTimeout, nil` at `go/internal/bridge/driver_tmux_repl.go:885`. `grep -rn "return ExitArtifactTimeout" internal/bridge/*.go | grep -v _test` → that single hit. (engine.go:790's "e.g. a non-tmux driver returning 81" is stale — no such driver exists.)
- Every `deps.Stderr` write in that driver is a **bridge-authored note** carrying the `pfx` prefix — session/model/workdir banners, boot and prompt-delivery notes, stop-review verdicts, the workspace file listing, the drift alarm, the marker line (enumerated: driver_tmux_repl.go:121,124,150,152,158,160,213,236,253,256,308,319,324,329,353,365,378,412,419,559,602,604,609,628,637,694,697,700,747,750,802,828,849,850,852,869,880,902,906,911,924). **None writes pane content to `deps.Stderr`** — the pane goes to the log *files* (:901-902), a different sink.
- The stop-review `Reason` strings interpolated into the marker line (`reason=%q`, :880-883) are templated bridge prose with no pane text (`go/internal/bridge/stopreview.go:186-207`).

Consequence: the input C1 requires — a `[bridge] ERROR: API overloaded (529)` line inside the launch stderr on an exit-81 death — **cannot be produced by any production path**. The change would go green on its fixtures and be a permanent no-op in production. This is the fatal assumption, shown false.

### Attack 3 — IA-2 false: the classifier's only real matches on this path are false positives — **CRITICAL (active regression)**

Of the lines that *are* on the exit-81 stderr path, scout's proposed vocabulary matches the wrong ones:

- **Drift alarm** — `warnExhaustionRegexDrift` fires *only on an already-failed exit-81 teardown* and prints "POSSIBLE EXHAUSTION-REGEX DRIFT: the teardown pane matches a broad **quota**-wall heuristic but %s's controls.usage.exhausted_regex did not…" (`go/internal/bridge/exhaustion_drift.go:39-41`), emitted at driver_tmux_repl.go:869 — immediately *before* the marker line at :880. It contains `quota`, so it matches scout's vocabulary and, under H1's precedence rule, **replaces** the timeout summary as the recorded cause. Its own text says "diagnostic only, this exit-81 verdict is unchanged" — promoting it to *the cause* is precisely the failure mode engine.go:551-556 was built to prevent.
- **Workspace file listing** — printed on this path at driver_tmux_repl.go:849-852 as `"%s (%d bytes)"` per entry (`go/internal/bridge/driver_common.go:388`). A plain substring scan for `429`/`529` (scout's sketch: "scans stderr lines for a small fixed vocabulary … case-insensitively") matches any file sized e.g. 429, 1529, 5290 bytes. The existing single-owner vocabulary avoids exactly this with word boundaries — `\b529\b`, `\b429\b` (`go/internal/phasestream/marker.go:20-21`) — a detail the sketch drops.
- C3 guards only against `[bridge] WARN: sandbox chatter`. Neither false-positive vector above is covered by any criterion, so the eval can be fully green while the change actively degrades exit-81 forensics.

### Attack 4 — Simpler-approach-exists: a third copy of a single-owner vocabulary — **CRITICAL**

`go/internal/phasestream/marker.go:5-8` states the invariant outright: *"infraMarker is the system-wide infrastructure-failure vocabulary. Defined ONCE here (ADR-0020): the normalizer owns detection, and cycleclassify consumes the typed `kind==infra_failure` events rather than re-scanning raw text."* `cycleclassify`'s package doc repeats it: *"one owner of the infra-marker vocabulary"* (`go/internal/cycleclassify/classify.go:9-11`).

Scout's sketch adds a **third, hand-rolled, boundary-less copy** of that vocabulary inside `internal/bridge`. That violates the ADR-0020 single-owner contract and the standing rule `feedback_never_duplicate_centralize_via_design_patterns` (single-source-with-projection). The strictly simpler paths, in order of preference:

1. **Do nothing in `engine.go`** — the signal is already emitted (`api_529` infra_failure) and already classified (`ClassInfrastructure`). Zero files changed. If the complaint is discoverability, the cheap fix is documentation/report surfacing, not a new classifier.
2. If a bridge-level echo is still wanted, **export and reuse** `phasestream`'s `detectInfraMarker` (currently unexported, `marker.go:29`) as the one owner and call it from the bridge — one exported symbol instead of a duplicated vocabulary, and it inherits the word-boundary safety.

Either is materially smaller than "new helper + new vocabulary + new test file," and both avoid the regression in Attack 3.

### Attack 5 — Criteria falsifiability (per-criterion)

| Criterion | Falsifying observation | Falsifiable? |
|---|---|---|
| C1 | Feed a fixture stderr with an API-error line + `artifact-timeout: …` through `Engine.Launch` at exit 81; assert `strings.Contains(err.Error(), "529")` — if false, C1 is refuted. | Yes |
| C2 | Fixture with only the summary; if the returned message ≠ `artifactTimeoutSummary` output, C2 is refuted. | Yes |
| C3 | Fixture with `[bridge] WARN: sandbox chatter` + summary; if the message contains "sandbox chatter", C3 is refuted. | Yes |

All three are falsifiable as written, and each carries an explicit anti-gaming clause. **`premise.unfalsifiable_count = 0`** — the gate does not fire on this axis. The defect is not unfalsifiability: it is that all three are falsifiable *only against synthetic fixtures*, and the outcome they stand in for (IA-1: production visibility restored) is falsifiable and **already falsified** by Attack 2. No criterion observes a real bridge stderr, so the suite can be 3/3 green with the production defect untouched.

### Attack 6 — Process findings (non-blocking, recorded)

- **Eval materialized to the wrong path — HIGH.** Scout's Decision Trace asserts `"evals-materialized": true` and cites `.evolve/evals/surface-transient-api-error-under-artifact-timeout.md`. The file actually exists only at `.evolve/runs/cycle-1528/.evolve/evals/surface-transient-api-error-under-artifact-timeout.md` — a nested `.evolve/` under the run workspace (`find . -name "*surface-transient*"` → that single hit; the canonical `.evolve/evals/` does not contain it). The canonical location is projectRoot-relative (`go/internal/core/cycle_outcome.go:13-23`: "SELECTED slugs' evals to projectRoot/.evolve/evals/<slug>.md in the MAIN [tree]"), so with `eval_gate=enforce` this eval is not where the gate looks. Known class: relative paths resolved against the workspace NEST.
- **Goal ↔ lane divergence — MEDIUM (IA-4).** The cycle `goal` is token-usage trimming; the lane builds an unrelated bridge diagnostic fix. Scout discloses this and cites the lane-scope contract; `lane-scope.json` does pin `{"todo_ids":["transient-api-error-invisible-inside-artifact-timeout"]}` under this `goal_hash`, so the divergence is contract-sanctioned, not a scout error. Recorded because the resulting ship is attributed to a token-optimization `goal_hash`.
- **Verified-correct scout claims (credit where due).** engine.go:539-583 cause-selection flow, engine.go:786-799 `artifactTimeoutSummary` + its documented WARN-chatter rationale, `artifactTimeoutMarker` at stopreview.go:101, and `orchestrator_transient_test.go:55` ("exit 81 (ArtifactTimeout) should not be classified as transient bridge failure sentinel") all read exactly as scout described. The code reading is sound; the *premise* built on top of it is not.

### Severity roll-up

| # | Finding | Severity |
|---|---|---|
| 1 | Goal-is-wrong — the API-vs-slow distinction already exists (phasestream `api_529` → cycleclassify `ClassInfrastructure`) | CRITICAL |
| 2 | Fatal assumption IA-1 false — no API-error line can reach `stderrBuf`; exit 81 has one emitter and it writes only bridge notes there | CRITICAL |
| 3 | IA-2 false — drift alarm ("quota") and file-size listing ("429"/"529" substrings) are the only matches, both false positives that would replace the summary | CRITICAL |
| 4 | Simpler approach — reuse the ADR-0020 single-owner vocabulary, or change nothing; scout's sketch adds a third boundary-less copy | CRITICAL |
| 5 | Eval materialized to a nested `.evolve/` path, not projectRoot `.evolve/evals/` | HIGH |
| 6 | Cycle `goal` and lane-committed item are disjoint (contract-sanctioned, disclosed) | MEDIUM |

## Verdict

**FAIL (BLOCK).** The cycle must not proceed as framed.

**Blocking reason:** `premise.severity_max == CRITICAL`. The plan's load-bearing unstated assumption — that a transient upstream API-error line can appear in the stderr buffer `Engine.Launch` inspects on an exit-81 death — is false. Exit 81 has exactly one production emitter (`driver_tmux_repl.go:885`), and every `deps.Stderr` write on that path is a bridge-authored `pfx`-prefixed note; the provider's 429/529 text renders in the pane, which is written to the `cfg.StderrLog` / `cfg.StdoutLog` **files** (`driver_tmux_repl.go:900-902`), never into that buffer. The proposed `apiErrorLine` classifier would therefore never fire on a real API error while passing all three fixture-based criteria. The only lines it *would* match on that path are the exit-81 drift alarm (contains "quota", `exhaustion_drift.go:39-41`, emitted at :869 just before the marker at :880) and workspace file sizes (`"%s (%d bytes)"`, `driver_common.go:388`) under the sketch's boundary-less substring scan — both of which would displace the self-describing timeout summary, regressing the very fix at engine.go:551-561.

**Minimal reframe that would clear this gate** — pick one:

1. **Drop the item (recommended).** The capability is already shipped: pane text → `<phase>-stdout.log` → `phasestream.classifyPlain` → `kind==infra_failure`/`api_529` → `cycleclassify` Pass 2 → `ClassInfrastructure`. Return the todo-id to the backlog marked "already-satisfied downstream," citing `phasestream/classify.go:377-388`, `phasestream/marker.go:18-25`, `cycleclassify/classify.go:105-155`.
2. **Re-scope to the real sink, with the real vocabulary.** If the exit-81 *cause string* must echo the signal, source it from the captured pane (`cfg.StderrLog` / `lastGoodPane`), not from `stderrBuf`, and detect it by exporting and reusing `phasestream`'s `detectInfraMarker` — never a third local vocabulary. Then add the two missing negative criteria this eval lacks: the drift-alarm line must not win, and a `(1529 bytes)` listing line must not match. Re-run this gate against the re-scoped premise.
3. **Fix the eval path regardless.** Move the eval to projectRoot `.evolve/evals/surface-transient-api-error-under-artifact-timeout.md` (`git add -f` past `.gitignore`) before any ship, or `eval_gate=enforce` is judging an absent file.

**Emitted signals**

```json
{
  "premise.severity_max": "CRITICAL",
  "premise.unfalsifiable_count": 0
}
```

- `premise.severity_max` = **CRITICAL** (findings 1-4)
- `premise.unfalsifiable_count` = **0** (all three stated criteria carry an explicit falsifying observation; the block comes from the severity axis, not this one)

<!-- evolve-verdict: {"phase":"premise-challenge","verdict":"FAIL","schema_version":1} -->
