# Build Explanation — Cycle 2361

## Build Binding
- Cycle: 2361
- Base SHA: 722ecf0d20a8e61948b4a9bdc817ac90d4672b4d

## Summary
This build adds the design card of the `EnforceActor` and the solution record that compares three ways to deliver it. It changes no code.

## Rationale
The inbox item asks for a card and a review before any code. A card is the smallest change that decides the seven open inputs of the parent spec. Code first touches protected driver files.

## Changed Areas
- `docs/architecture/integration-driver-enforce.md` — adds the card. It defines `EnforceActor` and decides F2, F3, F11, F12 and Findings 3, 4, 5. It states the test journey.
- `docs/architecture/integration-driver.md` — links the card from section 9, so the open inputs point to their decisions.
- `solutions/wave-integration-i14-driver-pr-c/assumptions-and-evidence.md` — lists the assumptions and evidence that the options cite.
- `solutions/wave-integration-i14-driver-pr-c/options/1-card-with-ship-side-guard.md` — states the chosen option.
- `solutions/wave-integration-i14-driver-pr-c/options/2-split-into-three-cards.md` — states the runner-up.
- `solutions/wave-integration-i14-driver-pr-c/options/3-code-first-spike.md` — states the rejected option.
- `solutions/wave-integration-i14-driver-pr-c/recommendation.md` — compares the options and names the winner.

## Design Decisions
The resume check sits in the ship step, where all paths meet. A new resume path cannot skip it. The `delivered` state is inbox data, not a flag.

## Verification
`evolve solution check` reports OK. `evolve docs ste-lint --strict` reports 0 findings on the card. The eval graders grep the card for each acceptance term.

## Compatibility
No code changes. Shadow behavior stays as it is.

## Limitations
The checkpoint format is read from the parent spec and is not re-run. The card does not measure the cost of `BatchTest` at enforce.
