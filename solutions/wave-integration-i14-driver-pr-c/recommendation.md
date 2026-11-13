# Recommendation — I14 PR (c) enforce actor

Citations: [assumptions-and-evidence.md](assumptions-and-evidence.md).

## Options Compared

| Criterion | 1 — one card, ship-side guard | 2 — three cards | 3 — code first |
|---|---|---|---|
| Decides all seven section 9 inputs (E1) | yes, in one document | yes, in three | no: decided in review |
| Follows the item: card and review before code (A4) | yes | yes | no |
| One contract for `delivered` and landing (E3, E4) | yes | no: repeated or linked | yes |
| Resume check cannot be skipped (A3) | yes: one point in ship | yes | depends on the diff |
| Fits this cycle without touching driver code (A5) | yes | yes | no |
| Cost | one document cycle | three document cycles | one larger code cycle |

## Recommendation

**Winner: Option 1.** It follows the item (A4). It keeps one contract for the `delivered` state and the landing (E3, E4). It changes no driver code (A5). The card is `docs/architecture/integration-driver-enforce.md`. Section 9 of the parent spec links to it.

**Runner-up: Option 2.** Use it if the review shows that the card is too large to review at once. The first card needed three review rounds (E7).

**Evidence that can flip the choice:**

- Review names a conflict between the `delivered` state and the landing contract. Then Option 2 wins on review size.
- The operator rules that no card round is needed. Then Option 3 is valid. A4 and A5 state the opposite today.
- The checkpoint has no lane identity to read. Then the ship-side guard of Option 1 changes to a refusal only (E6). Option 1 stays the choice.

**Not sourced:** no edit time or review time was measured for any option. The checkpoint format is read from the parent spec and is not re-run (E6).
