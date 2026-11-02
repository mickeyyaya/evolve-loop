# Build Explanation — Cycle 2101

## Build Binding
- Cycle: 2101
- Base SHA: 737352d1bd7eb54da1ee46c0a0e00a600540729e

## Summary
A pending auto-respond send now clears against the same tail window that fired it. The human-input sent-keys line now names the fired rule.

## Rationale
The decide path matched only the last lines of the pane for a tail-scoped rule. The resolve path matched the whole pane. A dialog that scrolled out of the tail stayed pending, so the outcome evidence under-reported prompt_cleared. The pending record now stores the rule tail, so both paths use one window.

## Changed Areas
- `go/internal/bridge/autorespond.go` — the pending record stores the rule tail; resolvePending matches the same tail window; the human path passes the fired rule name.
- `go/internal/bridge/human_input.go` — humanSendKeysCSV takes the rule name and prints it as rule=<name>.

## Design Decisions
The tail is a field of the pending record, so no second lookup of the rule is needed at resolve time. The human path reuses firedRuleName, the same source as the auto path.

## Verification
Eleven named bridge tests cover tail scroll-out, tail boundaries 1, 2, 3 and 5, the untailed rule, and both attribution lines. The ACS suite for cycle 2101 reports red=0.

## Compatibility
A rule without tail_lines keeps whole-pane matching. The auto-respond line is unchanged.

## Limitations
The plan-dialog drift alarm and the self-matching manifest stay out of scope for this cycle.
