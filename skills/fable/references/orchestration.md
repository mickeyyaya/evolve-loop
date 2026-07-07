# Parallelism & Delegation

> Load when: a task has independent parts, needs research/review at scale, or involves long-running processes. Serial-by-default wastes wall-clock and attention; undisciplined parallelism wastes tokens. This is the discipline between.

## Parallelize by data dependency, nothing else

- Two operations with no data dependency (neither consumes the other's output) go in ONE message/batch: multiple tool calls, multiple subagents, multiple background commands.
- A conceptual sequence ("first explore, then design") is NOT a data dependency if the design brief doesn't need the exploration result — but usually it does; be honest about which.
- Long-running commands (test suites, CI watches, batch processes) run in the background while you do the next independent thing. Never foreground-sleep waiting for them; never poll on a timer what will notify you on completion.

## The subagent contract (RIGID)

Delegation quality = brief quality. Every brief contains:

1. **Exact scope** — which directories/questions/failure; what is explicitly OUT of scope.
2. **The questions to answer**, numbered, each answerable with evidence.
3. **Required evidence format** — "file:line for every claim"; "quote the exact error"; "state VERIFIED vs INFERRED".
4. **Output contract** — where to write (exact path) and the shape (sections, verdict enum, summary length). A researcher writes a file and returns a 10-line summary; a reviewer returns APPROVE/REQUEST_CHANGES + itemized findings.
5. **Negative constraints** — read-only agents are TOLD they are read-only; reviewers are told "do not edit, report only"; explorers are told what a sibling agent already covers so they don't duplicate.

Once delegated, don't redo the work yourself — wait for the result, then verify spot-checks if it's load-bearing.

## Fan-out patterns (choose by task shape)

- **Disjoint exploration** (unknown codebase area): 2-3 explorers, each with one premise-cluster or subsystem; briefs reference each other's scope to prevent overlap.
- **Perspective pair** (research): split by SOURCE TYPE not topic — e.g., one agent on academic papers, one on OSS implementations of the same question; they can't converge on the same easy findings.
- **Review pair** (before commit): one correctness reviewer (language-specific) + one simplification reviewer, in parallel; both must return before the gate. Give the simplifier the context that detail is intentional when it is, or it will (correctly) flag your intended verbosity.
- **Adversarial single** (design): one reviewer with the attack brief (see design-and-review.md) beats three agreeable ones.

## Monitoring and boundaries

- Background tasks re-invoke you on completion — the notification IS the schedule. Plan work in boundary-sized chunks: launch, do independent work, handle the boundary when it arrives.
- A process the harness cannot track (detached, external) gets an explicit watcher (`while kill -0 PID; do sleep 30; done` as a tracked background task) so its exit still reaches you. Never launch-and-forget anything whose exit you must act on.
- At every boundary run the same routine, every time (assess → clean → rebuild → verify → relaunch). Boundary routines are checklists, not judgment calls: on FIRST encounter, derive the checklist from what the boundary actually required and write it down (task tracker, memory, or runbook doc); every later boundary executes that list mechanically, and improvements are edits to the list, never ad-hoc deviations.

## While waiting

Waiting time is prep time: stage the next phase's inputs, draft the report skeleton, gather the evidence the pending result will be judged against. The one forbidden move: starting work that races the thing you're waiting for (writing to files a running process owns, mutating state a pending merge will touch). If the pending work owns the tree, your work goes to a scratch area until the boundary.
