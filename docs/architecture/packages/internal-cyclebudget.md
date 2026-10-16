# internal/cyclebudget

## Purpose

`internal/cyclebudget` decides if a batch stops after a cycle. The loop runs until the planned backlog drains or the advisor judges the request complete. A safety cap bounds it. The operator no longer guesses `--max-cycles`. The package holds only the decision, and it is a stdlib-only leaf. The dispatcher supplies the inputs and acts on the verdict.

## Design

- **A three-stage dial** that follows the project's off, advisory, enforce axis, so it landed default-off. *Off* is byte-identical to the old behaviour: the dispatcher's `--max-cycles` loop governs and the budget never signals a stop. *Advisory* computes and reports the decision (shadow soak) but never stops the loop. *Enforce* lets completion and the cap drive the loop, and `--max-cycles` becomes the safety ceiling.
- **`ParseStage` fails safe**: an unknown or empty value is *Off*, today's behaviour.

## Invariants

- **Completion beats the cap** when both hold. Completion is the advisor's goal-complete signal or a drained backlog.
- **`cyclesRun` is 1-based**, because `Next` runs after a cycle. Completion can therefore fire only after the first planning cycle ran. This prevents a stop on an unplanned backlog.
- **`Decision.Reason`** is `goal_complete` or `cap`. `Advisory` is true only in the advisory stage when the loop can stop in the enforce stage. Pinned by `TestDecision_ZeroValueIsNoStop` and the stage tests.
