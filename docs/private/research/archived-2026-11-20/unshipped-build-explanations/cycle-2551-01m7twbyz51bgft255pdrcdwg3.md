# Build Explanation — Cycle 2551

## Build Binding
- Cycle: 2551
- Base SHA: f25d49e4ae89c9d1c528d1c4fbc1687743635999

## Summary
The decision sections of `agents/evolve-orchestrator.md` now frame the persona as a proposer and assign verdict, skip, ship, release, and state actions to the Go kernel.

## Rationale
Cycle 2200 updated the frontmatter and opening prose. However, decision tables and workflow steps still instructed the persona to invoke `ship.sh`, `release-pipeline.sh`, `record-failure-to-state.sh`, `merge-lesson-into-state.sh`, and `cycle-state.sh advance`. Rewriting every decision section as propose-only aligns the full document with `core.Orchestrator` ownership.

## Changed Areas
- `agents/evolve-orchestrator.md` — rewrote the five decision sections (`EGPS Verdict-of-Record`, `PSMAS Phase-Skip`, `Trivial-Skip Logic`, `Verdict Decision Tree`, and `Closure-Mode Detection`) as proposals. Each section now states the Go kernel decision and drops imperative ship, release, failure-state, lesson-merge, and state-advance script calls.

## Design Decisions
Read-only queries (`cycle-state.sh get`) and subagent dispatch notes (`subagent-run.sh`) stay intact as kernel/read examples while every mutation or ship script moves to Go kernel descriptions. All compaction markers, `## Reference Index` tail bytes, and required regression anchors remain in place instead of deleting sections.

## Verification
`go test -v -tags acs -count=1 ./acs/cycle2551/...`, `go test -tags acs -count=1 ./acs/cycle2200/... ./acs/cycle421/...`, `go test -count=1 ./internal/prompts/... ./internal/testaudit/...`, and `evolve acs suite --cycle 2551` verify the persona rewrite and unit test.

## Compatibility
The `agents/evolve-orchestrator.md` section structure, frontmatter keys, `## Reference Index` compaction split (`>=2000` bytes saved), and byte budget (`<=20944` bytes) remain compatible with existing prompt and compaction gates.

## Limitations
`agents/evolve-orchestrator-reference.md` was not part of this task scope and stays untouched.
