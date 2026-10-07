# Pipeline dashboard — operator guide

> `evolve dashboard` serves a read-only, live web view of the loop. It shows what the loop does now,
> what is queued, what each cycle did, what went wrong, and the trend of the ship rate.
> Design: [ADR-0095](../architecture/adr/0095-pipeline-dashboard.md).

## Run it

```sh
cd ~/ai/claude/evolve-loop/runtime          # the plane that runs the loop
evolve dashboard                            # http://127.0.0.1:8090
evolve dashboard --addr 127.0.0.1:9000      # another port
evolve dashboard --project-root ../runtime  # from anywhere
evolve dashboard --snapshot | jq .trend     # JSON, no server (scripts, smoke checks)
```

It is safe beside a live loop. Every request is a read, it takes no lock, and it writes nothing under
`.evolve/`. To stop it, use Ctrl-C. It binds to loopback only and has no authentication.
If you tunnel it (`ssh -L 8090:127.0.0.1:8090 …`), remember that the page shows text that agents wrote.

## Reading the board

**Header tiles** — running / queued / pass / fail (listed cycles), the ship rate over the last
20 closed cycles, and the all-time ship rate. The SLO is ≥ 60 % (`internal/cyclehealth/outcome.go`).

**loop** — `RUNNING` means that the run lease (`.lease`) of the current cycle is fresh. The board never
uses a live PID as evidence. `PAUSED (brake)` means that `.evolve/loop-stop` exists. The card also shows the phase and the elapsed time.
It also shows the CLI · tier that dispatched the phase, the count of audit rounds, the lease heartbeat and the worktree.

**ship rate** — one bar for each closed cycle (green PASS, red FAIL, amber WARN), from oldest to newest.
Below the bars is the **repair-loop convergence** table: the cycles that needed N audit rounds, and how many of
those cycles shipped. If the 3-round row is a graveyard, the repair loop grinds and does not converge
(the research doc tells why).

**cycles** — newest first. The stepper has one square for each phase dispatch, in run order. (Put the pointer on a square
to see the verdict, the duration and the round.) Rounds = audit dispatches. "what went wrong" = the category and the
short fingerprint hash. Click a row to see the detail view.

**inbox** — the pending items in inbox-rank order, with the score of each item (kind, route), and the lifecycle counts.
The order comes from `evolve inbox rank` ([ADR-0121](../architecture/adr/0121-inbox-priority-is-a-computed-rank.md)).
Until 2026-10-06, the table sorted by weight and showed the weight.

**phase plan** — under the stepper of each cycle: `passed 3/5 required · +1 optional · build 23m · left: audit, ship`.
The stepper itself is the *plan* of the cycle, not only its history:

- A filled square is a phase that ran. Its colour is the last verdict of the phase.
- The square that pulses is the phase in progress.
- A hollow square is a mandatory phase that the cycle in progress did not reach yet.
- A dashed square is a mandatory phase that the cycle went past without a run.
- A faded square is a phase that a sealed cycle never reached.
- A ring marks a phase whose contract gate recorded `GATE_CONTRACT_VERIFIED` in the signal stream of the cycle.
- A round square is an optional phase that the cycle ran (for example, retro).

"Required" is the answer of the registry, read through `internal/config`:

- The required set is `config.mandatory_phases` (scout, triage, build, audit, ship — the set that the floor of the router never skips).
  It also contains the `conditional_mandatory` phases (tdd, plan-review, build-planner) that ran.
- If the registry is malformed, the board falls back to the compiled baseline and says so in the warnings.
- The overrides of the operator (`EVOLVE_MANDATORY_PHASES`, `EVOLVE_USE_PHASE_REGISTRY`, and the other overrides) change the set exactly as they change the floor of the loop.
- A conditional-mandatory phase appears only after it runs, because the board does not evaluate the rule.
  Thus `left:` names only the mandatory phases that are still ahead. The required count increases by one when such a phase runs.

For a sealed cycle, the record of what ran comes from `phase-timing.json`.
For a cycle in progress, it comes from the Signal Center stream of the cycle (`phase.outcome` events).
The loop flushes the timing file at closeout, so the stream is the live record.

**phase × cycle** — an Airflow-style grid. The rows are the phases, in first-seen order. The columns are the
last 24 cycles that have workspaces. The cell colour is the last verdict of the phase. `rN` shows that the phase ran
N times. Click a column header to see that cycle.

**failure classes** — Sentry-style groups, by failure fingerprint, from the committed dossiers.
Each group shows the count and the first/last cycle. It also shows one of these marks:
`REGRESSED` (the identity came back after a later shipped cycle),
`recurring`, or `new`. Click a group to open the last cycle that has it.

## Reading a cycle (detail view)

**phase plan** — the same sequence as a table. The columns are:

- the phase (with its optional / conditional mark);
- the status (`pass` / `warn` / `fail` / `incomplete` for a run without a recorded verdict / `ongoing` / `pending` / `unreached` / `skipped`);
- the gate mark;
- the wall clock;
- the rounds.

Below the table, the view compares the proposal of the advisor with what ran. It shows:

- the phases proposed to run that did not run (or did not run yet, on a cycle in progress);
- the phases proposed to skip;
- the phases proposed to skip that ran anyway (the mandatory floor overrode the proposal).

Sources: `phase-timing.json` or the signal stream for what ran, `signals.ndjson` for the gate marks, and `phase-replan.json` (else `phase-plan.json`) for the proposal.
If a newer proposal is torn, the view reports it. The view never replaces it with the older file.

Top to bottom is the triage order:

1. **Header** — the state pill, the shipped SHA, the tokens, the goal, the committed task slugs and the wall-clock window.
   It also shows which sources fed the view (run workspace, committed dossier, or both).
2. **what went wrong** (FAIL cycles only) —
   - pills: `category · level · action/fix_type` from `failure-decision.json`;
   - fingerprint · `seen N× · first #A · last #B` · `REGRESSED`;
   - legitimacy · layer · root-cause summary from `disposition.json`. The view highlights `false-rejection`,
     because that is a pipeline defect, not a task defect;
   - deterministic gate reasons (`audit-fail-reason.json`). These have priority over any narrative;
   - auditor findings of the final round (`### H1 (HIGH) — …`), highest severity first;
   - repair-round history: `r1 FAIL (7) → r2 FAIL (3: 5 resolved, 0 new, 3 carried) → …`.
     A carried finding is a finding that the builder got and did not fix;
   - salvage pointer (the preserved worktree and the base SHA).
3. **phase timeline** — one lane for each dispatch. Bar = wall clock. Label = `cli model`.
4. **artifacts** — every readable file in the run workspace, reports first.
   Click a file to see it as escaped text (2 MiB cap). The prompts are there too.
   `build-prompt.txt` is the exact text that the builder got.

## What it deliberately does not do

- It does not write anything (no brake toggle, no inbox edits). For these, use `evolve loop-stop` and `evolve inbox …`.
  With `evolve loop-stop`, the loop in progress finishes its wave and exits. `--release` lifts the brake.
- It does not render markdown or load a chart library. It uses only escaped text and plain DOM.
- It does not replace `evolve cycle timing`, `evolve soak-report` and `evolve ledger tail`. It renders the
  same files. Those commands stay the scripted surfaces.

## Troubleshooting

| Symptom | Meaning |
|---|---|
| loop card shows `NO CYCLE` while a cycle dir exists | `.evolve/cycle-state.json` is absent (the kernel removes it on a clean stop). The board shows the `run.json` of the newest run as `incomplete`. |
| a cycle shows `incomplete · paused (brake)` | The cycle has no dossier yet and the lease is stale. `evolve loop --resume` continues it. |
| "N unreadable artifacts" in the footer | A file was absent or half-written at read time. The list names the files. The message clears on the next change. |
| no findings in the failure panel | The auditor used a heading shape that the parser does not recognise. The raw `audit-report.md` is one click away in artifacts. |
| `reconnecting…` in the header | The SSE stream dropped. The browser reconnects automatically. Meanwhile, a 60 s safety poll keeps the page current. |
| `421 Misdirected Request` | The `Host` of the request was not a loopback name (`127.0.0.1`, `localhost`, `::1`) and not the address that the server bound. This is the DNS-rebinding guard. Open the page at `http://127.0.0.1:<port>/`, or bind the name that you want with `--addr host:port`. |
