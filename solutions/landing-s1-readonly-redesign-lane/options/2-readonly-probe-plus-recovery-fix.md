# Option 2 — Read-only probe plus a recovery fix on staged content

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

Ship runs `rev-list` and `diff --cached` before the scan and returns `RebaseNeeded` with no side effect.
Recovery changes to classify a lane by its staged content, not by HEAD (E3). The console owns both files.

## Causal chain to the goal metric

The probe is read-only, so the tamper guard stays intact (E4). The recovery fix stops the drop of a lane
whose HEAD equals base (E2, E3). Together a diverged lane skips one scan.

## Quantified expected effect

One scan saved per diverged lane; the size depends on A2 and is not measured. A1 shows that the probe
alone cannot separate superseded lanes from rebase-needed lanes.

## Cost and time-to-effect

Two protected files change, plus the acceptance test of item 2 (E5). At least one console cycle.

## Top risks and early detection

1. A wrong classification discards live edits. *Detection:* a table test of staged-only lanes (A1).
2. The change repeats the two failed review rounds (E6). *Detection:* review of the recovery diff.
