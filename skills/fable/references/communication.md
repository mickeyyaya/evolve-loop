# Writing Like the Report Matters

> Load when: composing any user-facing message — findings, status, completion reports, failure reports.

The reader is a teammate who stepped away, not a log parser.

## Lead with the outcome (expands SKILL.md §4)

The first sentence answers the question the user would ask if they said "just give me the TLDR."

- BAD: "I started by looking at the CI configuration, which has two workflows…"
- GOOD: "**Main is green.** The go workflow completed success in 7m6s on both platforms — the first green run since the red streak began."

Everything after the first sentence is supporting detail for readers who want it. If the user reads only your first paragraph, they must not be misled about the state of the world.

## Quantify everything load-bearing

Numbers are claims; adjectives are vibes.

| Vague | Quantified |
|---|---|
| "lots of retries" | "20 fallback double-dispatches, 40 quota-exhaustion events per 8-cycle batch" |
| "the gate has been broken a while" | "dead since cycle ~215; current cycle 568" |
| "the slow platform" | "ubuntu 7m3s vs macOS 4m30s" (label which side is which — a bare "A vs B" leaves the direction ambiguous) |

(Two more pairs in SKILL.md §4.)

## Structure rules

- **Tables for enumerable facts** (statuses, inventories, before/after); **prose for causality and reasoning**. Never bury the causal chain inside table cells — the table shows *what*, the surrounding prose explains *why it matters and what happens next*.
- **Every code claim carries file:line.** "The gate fail-opens" is an opinion; "the gate fail-opens at `ciparity.go:279`" is a finding someone can check in one click.
- **Complete sentences; technical terms spelled out.** No fragment chains (`checked X → dead → rerouted`), no codenames the reader didn't coin. Readability beats brevity: the way to be short is to *select* fewer things, not to compress the grammar of the things you keep.
- **Bold the load-bearing phrase** in a long paragraph — the reader scanning for the verdict should hit it.

## The failure-report shape (RIGID)

When you broke something, the first sentence has four parts — outcome, mechanism, ownership, cost:

> "Wave 3 went 0/2 [outcome] — both lanes died on the tree-diff guard [mechanism] because of MY doc landings [ownership]; ~2 cycles' tokens wasted [cost]."

Then, in order: root cause (why the action interacted with the system that way), the lesson as a *written rule* (where you persisted it), and the prevention (what makes recurrence structurally impossible). Never passive voice ("an issue occurred"), never burying your own fault under someone else's bug, never reporting it later than the discovery turn.

Corrections to your own earlier claims outrank new work: if you discover a previous message was wrong (a command ran in the wrong directory, a number was misread), correct the record explicitly — "correcting my earlier claim: X was actually Y because Z" — before proceeding.

## Status cadence while working

- Before the first tool call of a task: one sentence on the approach ("CI is red again — pulling the run history and the exact failure first").
- At each load-bearing discovery or direction change: one line ("Found it: ubuntu-only, and the failure is in the gate step, not the tests — switching to environment diffing").
- Never narrate mechanics ("now I will run grep"); narrate *meaning* ("the writer turns out to be an LLM artifact, which explains the extinction").

## The final message (RIGID)

Everything the user needs from the turn must be in the last message — mid-turn notes may never be displayed. Restate key mid-turn findings; don't reference them ("as I found above" is a broken link in some UIs — repeat the finding in one clause instead). End-state check: no promises of work you could do now; open decisions stated as questions WITH your recommendation; everything else already handled.

## Calibration

- Slightly tighter for experts, more explanatory for newcomers — but never switch to fragments for anyone.
- Long outputs (>~300 lines of findings) go to a file; the message carries a 5-line summary plus the path.
- Match the response size to the question: a yes/no question gets an answer sentence plus context, not a sectioned report.
