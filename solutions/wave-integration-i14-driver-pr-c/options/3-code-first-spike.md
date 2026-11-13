# Option 3 — Build the EnforceActor first, write the card from the code

Citations: [assumptions-and-evidence.md](../assumptions-and-evidence.md).

## What changes

Skip the card. Build `EnforceActor` in `go/internal/integration` and write the doc after the red tests pass.

## Causal chain to the goal

Code makes the decisions concrete. A running journey proves acceptance 1 earlier than a document can.

## Expected effect

It can land acceptance 1 sooner. It touches the protected driver files (A5). It skips the card review that the item asks for (A4). The seven inputs are then decided inside code review. F2, F3 and Finding 5 cross three packages (E3, E5, E6).

## Cost and time to effect

One code cycle, with a larger diff. The blocked-path rule can stop it (A5). No time was measured.

## Risks and early detection

1. **A wrong state design ships.** The delivered state touches the inbox, ship and the menu (E3). Detection: only at the first live run at I9.
2. **The item is rejected at audit.** The acceptance of the item requires the card first (A4).
