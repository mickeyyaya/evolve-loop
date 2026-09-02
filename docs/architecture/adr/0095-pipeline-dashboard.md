# ADR-0095 — `evolve dashboard`: a read-only, stdlib-only live view of the pipeline

- **Status:** Accepted (2026-09-02).
- **Driving evidence:** the 2026-09-02 ship-rate investigation
  ([research](../../research/ship-rate-harness-reliability-2026-09-02.md)). The batch SLO
  "SHIPPED ≥ 60 %" existed only as a comment in `internal/cyclehealth/outcome.go`; nothing
  computed it. Measured from the committed dossiers it was **19.6 %** over cycles 1560–1605
  with an **0 / 11** streak, and ship probability fell from 100 % after one audit round to
  0 % after four — a repair loop that was grinding, invisibly. Operators were reading
  `recent-outcomes.md` and `audit-report.md` by hand across ~100 files per cycle.
- **Design spec:** [2026-09-02-ship-rate-harness-and-pipeline-dashboard-design.md](../../superpowers/specs/2026-09-02-ship-rate-harness-and-pipeline-dashboard-design.md).
- **Pattern research:** [pipeline-dashboard-patterns-2026-09-02.md](../../research/pipeline-dashboard-patterns-2026-09-02.md) (~45 sources: Langfuse, Phoenix, OpenHands, SWE-agent inspector, Airflow, Buildkite, Sentry, Dozzle, ntfy).
- **Related:** [ADR-0055](0055-cycle-dossier.md) (the committed dossier the trend is computed from), [ADR-0072](0072-system-failure-policy.md) (the failure vocabulary the panel renders), [ADR-0092](0092-audit-repair-loop.md) / [ADR-0093](0093-retry-envelope-and-terminal-retro.md) (the repair rounds the round history shows).

## Problem

A human supervising the loop must answer five questions quickly: is it alive and what is it
doing; what is queued; what did each cycle do; if it failed, exactly why and is that new or
recurring; and is the ship rate moving. Every answer is on disk, but spread across
`cycle-state.json`, the run lease, ~100 artifacts per run workspace, 1,840 committed dossier
files, and the inbox tree. The scripted surfaces (`cycle timing`, `soak-report`, `ledger
tail`) each answer one question for one cycle and none of them updates live.

## Decision

Add `evolve dashboard` (`go/internal/dashboard`, `go/cmd/evolve/cmd_dashboard.go`): a local
web page served by the evolve binary itself.

1. **Read-only, by construction.** Every handler is GET; nothing under the project root is
   written; the loop's `flock` sidecars are never opened. The reader relies on the writers'
   atomic-rename discipline (a torn read is impossible; a rename in flight shows the old file)
   and treats an unparsable artifact as a warning on the page, never a failure.
2. **Stdlib only.** `net/http`, `html`-free static shell, `embed`, `encoding/json`. The module's
   `go.mod` is vendored with two dependencies; a dashboard is not a reason to add a third.
   Consequences: no fsnotify (change detection is a mtime/size fingerprint poll, 2 s default),
   no markdown renderer (artifacts are shown as escaped text with heading hints), no chart
   library (the verdict strip and timeline are plain DOM).
3. **Beliefs are imported, never re-declared.** Cycle workspace paths come from
   `core.RunWorkspacePath`, the cycle-state path from `core.ResolveCycleStatePath`, liveness
   from `runlease.Fresh` (a live PID is never proof), inbox items from `inboxbatch.LoadDir`,
   dossiers from `dossier.ParseJSON`, timing from `phasetiming.Read`, the audit report's name
   from `phasecontract.ArtifactFilename`, and its round archives from
   `phasecontract.RoundArchiveFilename` — which the writer `core.retireSupersededAuditArtifacts`
   now also derives from. Where a name had no single home, this change gave it one and moved
   the existing writer onto it: `paths.LoopStopPath` (the `loop-stop` brake, shared with
   `cmd_loop_chain.go`), `paths.EvolveDirOf` (the `.evolve` spelling, from which `Layout`
   derives), `dossier.CyclesDir` (`knowledge-base/cycles`, shared with the dossier producer and
   the chronicle seed), `bridge.LLMCallsLogFilename` (the dispatch ledger, shared with the
   engine that writes it and the `tokens` / `models live` readers). Phase order is taken from
   the observed timing log rather than a fourth copy of the canonical list. The SYSTEM level
   that colours a cycle "halted" is `policy.LevelSystem`, not a literal.
4. **Ship rate from the committed dossiers, not a new ledger.** `knowledge-base/cycles/cycle-N.json`
   already carries `final_verdict`, `commit_sha` and `failure.fingerprint` durably and survives
   the run-workspace GC. The page computes last-20 / last-50 / all-time rates, a per-cycle
   verdict strip, and Sentry-style fingerprint groups (count, first/last seen, **regressed** when
   an identity returns after a later shipped cycle). Audit-round convergence is bounded to the
   workspaces still on disk because dossiers do not record rounds.
5. **The failure panel is a triage order, not a log.** For a FAIL cycle: category · level ·
   action (`failure-decision.json`); fingerprint + recurrence; legitimacy · layer · root cause
   (`disposition.json`); deterministic gate reasons (`audit-fail-reason.json`); the final
   round's auditor findings parsed from `### H1 (HIGH) — …` headings; the immutable repair-round
   history diffed by normalised title (`audit-report.round<N>.md` archives); the salvage pointer.
6. **Everything rendered is hostile text, and loopback is not a boundary.** The static shell
   ships a CSP (`script-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action
   'none'`); JSON is escaped; the client builds the DOM through `textContent` only; artifacts
   are served as `text/plain; nosniff` through an allowlist on the file name,
   `reportdoc.OpenRegularNoFollow` (symlink-safe, identity-checked), and a 2 MiB cap. The
   server binds `127.0.0.1` by default and has no authentication — hence no write endpoint of
   any kind, including a brake toggle. Because a DNS-rebinding page an operator visits would
   become same-origin with `127.0.0.1:8090`, every request's `Host` must be a loopback name or
   the bound address; anything else is `421 Misdirected Request`.
7. **One SSE stream per page.** `/events` pushes a tiny `event: snapshot` with a sequence id
   when the fingerprint moves, plus `: ping` keep-alives; the client re-fetches `/api/snapshot`.
   `Server.WriteTimeout` stays zero on purpose (a global write deadline kills streams). The
   serve context is every request's `BaseContext`, so cancelling it ends open streams and
   `Shutdown` completes instead of timing out behind them.
8. **Read failures are never silent.** Absence is the ordinary sparse-workspace shape and stays
   quiet; a present-but-unreadable or torn artifact (a half-written `disposition.json`, an
   unreadable `knowledge-base/cycles`, an unlistable `.evolve/runs`) lands in `Warnings` with the
   file and the error. The alternative — a headline "ship rate 0 % over 0 cycles" for a repo
   with 1,800 dossiers, or a halted cycle rendered "incomplete" — is the proxy-as-verdict class
   the repo already has an incident ledger for.

## Alternatives considered

- **fsnotify.** Rejected: a new dependency, and the writers rename atomically — file watches are
  orphaned by exactly that pattern; directory-mtime polling is what the repo already does
  (`cmd_bridge_watch.go`).
- **A durable outcome ledger written at cycle closeout.** Rejected as unnecessary: the dossier
  already is that ledger. A second writer of verdict state would be the single-source violation
  the operating policy forbids.
- **Server-side HTML templates per page.** Rejected: JSON + one static shell keeps the
  escaping story to one rule (textContent) and lets `--snapshot` reuse the same model for
  scripts.
- **A brake toggle / inbox editing.** Rejected: an unauthenticated write surface on a page that
  renders agent-authored text is a prompt-injection-to-operator-action path. Operators keep
  `touch .evolve/loop-stop`.
- **Markdown rendering.** Rejected for now: readability gain is real, XSS surface is real,
  and the escaped-text view with heading hints is enough to read an audit report.

## Consequences

- Operators get the ship-rate SLO as a live number for the first time, and the repair-loop
  round histogram makes the "grind" visible per batch.
- `go/.apicover-enforce` gains `./internal/dashboard` (34/34 exports named and exercised);
  `internal/paths` gains `LoopStopFile`/`LoopStopPath` (6/6).
- The dashboard is a **consumer** of artifact shapes owned elsewhere; a schema change to
  `failure-decision.json`, `disposition.json`, the auditor's heading format, or `llm-calls.ndjson`
  degrades a panel to blank (and a `Warnings` entry), never to a wrong claim.
- The auditor's issue grammar (`### H1 (HIGH) — title`) and the prose verdict grammar are
  single-homed in `internal/reportdoc` (`Findings`, `Verdict`, `FindingKey`). The verdict
  regexes are the audit gate's own — moved verbatim out of `phases/audit`, which now calls
  `reportdoc.Verdict` for its below-enforce prose fallback (the machine-readable sentinel,
  `phasecontract.ParseVerdictSentinel`, stays authoritative and is tried first by the gate and
  the dashboard alike). So what the gate scores, what the operator is shown, and what the
  repair-brief seed (proposal R2) tells the rebuilding agent are one definition. Both
  functions scan visible lines only, like every other `reportdoc` extractor: a fenced template,
  an indented example or an HTML comment can neither declare a verdict nor raise a phantom
  finding — for the gate as well as the page. The one deliberate behaviour change this
  introduces on the gate's fallback path: a verdict line indented four spaces or more is now
  code, not a declaration.
- `reportdoc.Findings` accepts every issue layout that is live — the four observed in recorded
  reports (paren heading, dash heading with or without a severity token, table rows with
  optional emphasis) and the reference template's own fenced `tsv` block directly under
  `## Issues` (`agents/evolve-auditor-reference.md`), which is the one fence that is the table
  rather than an example. Outside an `## Issues` section a heading must carry an explicit
  severity token to count, so a PASS-shaped or differently organised report never grows
  phantom findings. Cross-round matching uses the lead clause of the title or the id, and
  consumes each previous finding at most once.
- Round archives are read by parsing their indices (`phasecontract.ParseRoundArchive`), never
  by probing 1, 2, 3 in sequence: the writer advances the dispatch counter even when a dead
  dispatch left nothing to archive, so indices are not contiguous and a positional walk would
  show a self-contradicting history.
- Not a replacement for the scripted surfaces or the dossier: it renders them.
