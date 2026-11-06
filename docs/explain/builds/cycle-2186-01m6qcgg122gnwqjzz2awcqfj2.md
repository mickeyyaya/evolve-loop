# Build Explanation — Cycle 2186

## Build Binding
- Cycle: 2186
- Base SHA: a5f971c2d806906c3af71e3deb3c0ba2393f8a67

## Summary
The command `evolve wave status` shows one named health state for each seat: working, blocked, stalled, zombie or idle. A pure classifier in the panestream package names the state from six facts. This cycle continues cycle 2172, which did not fail on a code defect. No artifact was missing there. The spine gate read a stale round 1 verdict (FAIL) from `handoff-auditor.json`, and the later audit rounds did not rewrite that file. The gate blocked the ship phase on that verdict.

## Rationale
A pane that waits for an answer read as busy-stagnant, and a dead session read as a live pane. The state table was in the specification, but no code built it. A pure function with a fact struct keeps the rules in one place, and a table test covers each branch. The earlier try kept its work in the worktree. This cycle verifies that work and archives the superseded files of cycle 2172. It also adds the acceptance predicates of cycle 2186.

## Changed Areas
- `go/internal/bridge/panestream/seathealth.go` — adds `SeatHealth`, `SeatFacts` and `ClassifySeat`, the single home of the state rules.
- `go/internal/bridge/panestream/seathealth_test.go` — table test of the classifier rules.
- `go/internal/bridge/panestream/seathealth_amplify_test.go` — extra cases that cover the remaining branches of the classifier.
- `go/cmd/evolve/cmd_bridge_sessions.go` — carries the tmux stderr in the list error and records which sockets lost their server, so one shared lister serves `bridge sessions` and `wave status`.
- `go/cmd/evolve/cmd_wave_status_seat.go` — builds the facts for each live-lease run from the pane-watch snapshots, the tmux `list-sessions` seam and the deliverable file.
- `go/cmd/evolve/cmd_wave_status.go` — adds the `seats` field to the status report and prints one `seat:` line per seat.
- `go/internal/panewatch/panewatch.go` — adds the field `StartedAt` to the snapshot, so a deliverable of an earlier round does not hide the seat of this round.
- `go/internal/panewatch/tracker.go` — sets `StartedAt` on the first observation.
- `go/cmd/evolve/cmd_wave_status_seat_test.go` — journey tests through the real `wave status` entry.
- `go/cmd/evolve/cmd_wave_status_seat_repair_test.go` — covers the earlier-round deliverable, the list-failure warning and the C locale against real tmux.
- `go/cmd/evolve/cmd_wave_status_seat_noserver_test.go` — covers the gone-server journey against real tmux and the single shared lister.
- `go/cmd/evolve/cmd_wave_status_seat_warn_test.go` — covers the warning path for an unreadable pane-watch file.
- `go/cmd/evolve/parallel_serial_tests_test.go` — registers the journey tests as serial because they swap tmux seams.
- `go/acs/cycle2186/predicates_test.go` — the 13 acceptance predicates of this cycle; they replace the archived package of cycle 2172.
- `.evolve/evals/agent-seat-health-states.md` — eval score caps of the task.
- `docs/architecture/regression-proof-and-supervision.md` — states the rule order, the stall window and the read-only guarantee of the view.
- `docs/architecture/packages/internal-bridge-panestream.md` — adds the seat health note.
- `docs/private/research/archived-2026-11-06/superseded-predicate-packages/cycle2172/predicates_test.go` — archive of the predicate package of cycle 2172, which this cycle replaces.
- `docs/private/research/archived-2026-11-06/unshipped-build-explanations/cycle-2172-01m6myqzpxav3m7z6xgm8bg7a8.md` — archive of the explanation of the unshipped cycle 2172.

## Design Decisions
A socket whose tmux server is gone lists no sessions, so the last session of a dead socket reads zombie. Any other list failure counts the session as alive, so it never reads zombie, and it prints a warning. A deliverable of this round outranks every other fact. The tmux listing uses a space as separator, because tmux escapes a tab in the C locale. The stall window is a constant of 10 minutes; no policy key exists for it yet. The view starts no server, seat or agent and sends no key.

## Verification
The ACS suite for cycle 2186 passes with red=0. The 13 predicates pass on the tree. With the zombie and blocked rules removed from `ClassifySeat`, 9 of the 13 fail on assertions, which proves red without the fix.

## Compatibility
`wave status --json` gains a `seats` array. Existing fields keep their names and meaning.

## Limitations
The stall window is not configurable. The `wave watch` path does not print seats.
