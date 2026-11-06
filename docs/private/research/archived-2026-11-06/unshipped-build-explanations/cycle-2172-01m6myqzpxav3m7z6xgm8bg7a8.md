# Build Explanation — Cycle 2172

## Build Binding
- Cycle: 2172
- Base SHA: ab2c75c24cb048247fd9af3fc6e54192c167b742

## Summary
The command `evolve wave status` now shows one named health state for each seat: working, blocked, stalled, zombie or idle. A pure classifier in the panestream package names the state from six facts.

## Rationale
A pane that waits for an answer read as busy-stagnant, and a dead session read as a live pane. The state table was already in the specification, but no code built it. A pure function with a fact struct keeps the rules in one place and lets a table test cover each branch.

## Changed Areas
- `go/internal/bridge/panestream/seathealth.go` — adds `SeatHealth`, `SeatFacts` and `ClassifySeat`, the single home of the state rules.
- `go/cmd/evolve/cmd_bridge_sessions.go` — carries the tmux stderr in the list error and records which sockets lost their server.
- `go/cmd/evolve/cmd_wave_status_seat.go` — builds the facts for each live-lease run from the pane-watch snapshots, the tmux `list-sessions` seam and the deliverable file.
- `go/cmd/evolve/cmd_wave_status.go` — adds the `seats` field to the status report and prints one `seat:` line per seat.
- `go/internal/panewatch/panewatch.go` — adds the field `StartedAt` to the snapshot, so a deliverable of an earlier round does not hide the seat of this round.
- `go/internal/panewatch/tracker.go` — sets `StartedAt` on the first observation.
- `go/cmd/evolve/cmd_wave_status_seat_repair_test.go` — covers the earlier-round deliverable, the list-failure warning and the C locale against real tmux.
- `go/cmd/evolve/cmd_wave_status_seat_warn_test.go` — covers the warning path for an unreadable pane-watch file.
- `go/cmd/evolve/cmd_wave_status_seat_noserver_test.go` — covers the gone-server journey against real tmux and the single shared lister.
- `go/cmd/evolve/cmd_wave_status_seat_test.go` — journey tests through the real `wave status` entry.
- `go/internal/bridge/panestream/seathealth_test.go` — table test of the classifier.
- `go/cmd/evolve/parallel_serial_tests_test.go` — registers the journey tests as serial because they swap tmux seams.
- `go/acs/cycle2172/predicates_test.go` — acceptance predicates of the cycle.
- `.evolve/evals/agent-seat-health-states.md` — eval score caps of the task.
- `docs/architecture/regression-proof-and-supervision.md` — states the rule order, the stall window and the read-only guarantee.
- `docs/architecture/packages/internal-bridge-panestream.md` — adds the seat health note.

## Design Decisions
A socket whose tmux server is gone lists no sessions, so the last session of a dead socket reads zombie. Any other list failure counts the session as alive, so it never reads zombie. The seam error now carries the tmux stderr, and `socketActivity` is the one shared listing used by both `bridge sessions` and `wave status`. The view starts no server, seat or agent and sends no key.

A deliverable of this round outranks every other fact. The tmux listing uses a space as separator, because tmux escapes a tab in the C locale. A list failure that is not a gone server prints a warning. The stall window is a constant of 10 minutes; no policy key exists for it yet.

## Verification
Journey tests, classifier tests and the ACS suite for cycle 2172 pass. Every function in the classifier file and in the new status file reports 100.0% coverage.

## Compatibility
`wave status --json` gains a `seats` array. Existing fields keep their names and meaning.

## Limitations
The stall window is not configurable. The `wave watch` path does not print seats.
