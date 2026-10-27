# internal/fixability

## Purpose

`internal/fixability` sorts each audit defect into a lane finding or a handoff finding. It stops a repair round when a handoff finding blocks the lane.

## Design

- **Classification.** `Classify` reads the defect strings, the red paths, and the open console items. It checks each defect for console items and fence paths.
- **Handoff findings.** A finding becomes a handoff finding when it names an open console item or cites a protected surface.
- **Blockers.** A handoff finding blocks when its severity is HIGH or CRITICAL. A MEDIUM finding blocks when the audit proves it red.
- **Stop decision.** When blockers exist, `Classify` sets `Stop` to true, sets `Reason` to `handoff_blocker`, and attaches the salvage branch.
- **Isolation.** The package does not start or direct any process. It remains a pure classifier.

## Invariants

- A finding on an unprotected path with no console item remains a lane finding.
- A handoff finding at MEDIUM without red proof does not stop the cycle.
- A stop decision carries the open console item of each blocking finding.
