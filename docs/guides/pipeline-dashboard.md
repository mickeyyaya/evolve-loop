# Pipeline dashboard — operator guide

> `evolve dashboard` serves a read-only, live web view of the loop: what it is doing, what is
> queued, what each cycle did, what went wrong, and how the ship rate is trending.
> Design: [ADR-0095](../architecture/adr/0095-pipeline-dashboard.md).

## Run it

```sh
cd ~/ai/claude/evolve-loop/runtime          # the plane that runs the loop
evolve dashboard                            # http://127.0.0.1:8090
evolve dashboard --addr 127.0.0.1:9000      # another port
evolve dashboard --project-root ../runtime  # from anywhere
evolve dashboard --snapshot | jq .trend     # JSON, no server (scripts, smoke checks)
```

It is safe beside a live loop: every request is a read, no lock is taken, nothing under
`.evolve/` is written. Stop it with Ctrl-C. It binds loopback only and has no authentication;
if you tunnel it (`ssh -L 8090:127.0.0.1:8090 …`) remember the page renders agent-authored
text.

## Reading the board

**Header tiles** — running / queued / pass / fail (listed cycles), ship rate over the last
20 closed cycles, and all-time. The SLO is ≥ 60 % (`internal/cyclehealth/outcome.go`).

**loop** — `RUNNING` means the current cycle's run lease (`.lease`) is fresh; a live PID is
never used as evidence. `PAUSED (brake)` means `.evolve/loop-stop` exists. Phase, elapsed,
the CLI · tier that dispatched it, audit-round count, lease heartbeat, worktree.

**ship rate** — one bar per closed cycle (green PASS, red FAIL, amber WARN), oldest → newest,
and the **repair-loop convergence** table: cycles that needed N audit rounds and how many of
those shipped. If the 3-round row is a graveyard, the repair loop is grinding rather than
converging (see the research doc for why).

**cycles** — newest first. The stepper is one square per phase dispatch in run order (hover
for verdict, duration, round). Rounds = audit dispatches. "what went wrong" = category and the
short fingerprint hash. Click a row for the detail view.

**inbox** — pending items by weight (kind, route), plus lifecycle counts.

**phase × cycle** — Airflow-style grid: rows are phases in first-seen order, columns the
last 24 cycles with workspaces, cell colour the phase's last verdict, `rN` when the phase ran
N times. Click a column header for that cycle.

**failure classes** — Sentry-style groups by failure fingerprint from the committed dossiers:
count, first/last cycle, `REGRESSED` when the identity came back after a later shipped cycle,
`recurring`, or `new`. Click to open the last cycle carrying it.

## Reading a cycle (detail view)

Top to bottom is the triage order:

1. **Header** — state pill, shipped SHA, tokens, goal, committed task slugs, wall-clock window,
   which sources fed the view (run workspace, committed dossier, or both).
2. **what went wrong** (FAIL cycles only) —
   - pills: `category · level · action/fix_type` from `failure-decision.json`;
   - fingerprint · `seen N× · first #A · last #B` · `REGRESSED`;
   - legitimacy · layer · root-cause summary from `disposition.json` (`false-rejection` is
     highlighted — that is a pipeline defect, not a task defect);
   - deterministic gate reasons (`audit-fail-reason.json`) — these outrank any narrative;
   - auditor findings of the final round (`### H1 (HIGH) — …`), highest severity first;
   - repair-round history: `r1 FAIL (7) → r2 FAIL (3: 5 resolved, 0 new, 3 carried) → …` — a
     carried finding is one the builder was told about and did not fix;
   - salvage pointer (the preserved worktree and base SHA).
3. **phase timeline** — one lane per dispatch, bar = wall clock, label = `cli model`.
4. **artifacts** — every readable file in the run workspace, reports first; click to view as
   escaped text (2 MiB cap). Prompts are there too: `build-prompt.txt` is what the builder was
   actually told.

## What it deliberately does not do

- Write anything (no brake toggle, no inbox edits) — use `touch .evolve/loop-stop` and
  `evolve inbox …`.
- Render markdown or load a chart library — escaped text and plain DOM only.
- Replace `evolve cycle timing`, `evolve soak-report`, `evolve ledger tail` — it renders the
  same files; those remain the scripted surfaces.

## Troubleshooting

| Symptom | Meaning |
|---|---|
| loop card shows `NO CYCLE` while a cycle dir exists | `.evolve/cycle-state.json` is absent (the kernel removes it on a clean stop); the newest run's `run.json` is shown as `incomplete`. |
| a cycle shows `incomplete · paused (brake)` | no dossier yet and the lease is stale; `evolve loop --resume` picks it up. |
| "N unreadable artifacts" in the footer | a file was absent or half-written at read time; the list names them; it clears on the next change. |
| no findings in the failure panel | the auditor used a heading shape the parser does not recognise; the raw `audit-report.md` is one click away in artifacts. |
| `reconnecting…` in the header | the SSE stream dropped; the browser reconnects automatically and a 60 s safety poll keeps the page current meanwhile. |
| `421 Misdirected Request` | the request's `Host` was neither a loopback name (`127.0.0.1`, `localhost`, `::1`) nor the address the server bound. This is the DNS-rebinding guard; open the page by `http://127.0.0.1:<port>/`, or bind the name you want with `--addr host:port`. |
