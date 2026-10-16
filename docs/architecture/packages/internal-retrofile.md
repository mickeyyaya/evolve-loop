# internal/retrofile

## Purpose

`internal/retrofile` closes the loop from learning to action. It lifts the machine-readable `preventive_actions` block of a retro report into weighted `.evolve/inbox` todos. A diagnosed recurrence then becomes queued work. It is no longer a lesson that is written and forgotten.

## Design

- **A leaf package that uses only the standard library.** It has no loop or policy imports. The caller wires it at the same deterministic post-cycle seam as the `FAILED_UNEXPLAINED` classifier (`fileUnexplainedOutcomeDefect` in `cmd/evolve/cmd_loop_outcome.go`).
- **The caller supplies the default weight and the clock.** The weight comes from policy and never from a Go literal. `now` stamps `injected_at`. The package is pure and its output is byte-stable.
- **The report format.** `ParsePreventiveActions` reads the JSON array in the fenced `json` block. The block is under the heading `## Recommended preventive actions` (`preventiveHeading`). The retro agent doc (`agents/evolve-retrospective.md`) documents the matching format.
- **`PreventiveAction.ID` is the stable inbox slug.** It is the same across cycles, so a recurrence deduplicates and does not spam. A positive `WeightHint` overrides the default weight. It is the lever of recurrence escalation.
- **`Recurrence` is the count that the retro attached.** It is advisory. It appears in the evidence of the filed item.
- **`FileActions` writes `auto-retro-<cycle>-<slug>.json` for each action** into `inboxDir`. The weight is the `WeightHint` when positive, else `defaultWeight`. It returns the paths that it wrote.
- **`existingIDs` scans the whole inbox.** It reads the `id` of every `*.json` item under `inboxDir`. That covers open items at the top level and consumed items under `processed/**`. An action that is queued or shipped is skipped wherever its item lives.

## Invariants

- **No recommendations is a no-op, not an error.** A report with no block returns `(nil, nil)`. A heading without a fence (prose-only actions) does the same. `TestParsePreventiveActions_AbsentReturnsNil` and `TestParsePreventiveActions_HeadingButNoFence` pin this.
- **A malformed block is an error that the caller sees.** An unterminated fence or invalid JSON is never dropped silently. `TestParsePreventiveActions_UnterminatedFence` and `TestParsePreventiveActions_MalformedJSON` pin this.
- **A recurrence does not file again a fix that is queued or done.** An action whose `ID` is an open item, or a consumed item under `processed/**`, is skipped. The same action from two consecutive FAIL cycles files once. A duplicate `ID` inside one batch is skipped too. The `TestFileActions_Dedup*` tests pin this.
- **An action with an empty `ID` is never filed.** It has no dedup key and no valid slug. `TestFileActions_EmptyIDSkipped` pins this.
- **A file that is not an inbox item does not stop dedup.** An object without `id`, a JSON array or garbage under the inbox is ignored. A new action is still filed. `TestFileActions_NonItemJSONIgnoredDuringDedup` pins this.
- **A failed inbox directory is a real error.** A non-directory at `inboxDir` makes `FileActions` return the error. Best-effort callers can ignore it. The function never claims success. `TestFileActions_MkdirFailsWhenInboxIsFile` pins this.
