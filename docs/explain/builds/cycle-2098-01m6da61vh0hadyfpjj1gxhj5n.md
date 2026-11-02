# Build Explanation — Cycle 2098

## Build Binding
- Cycle: 2098
- Base SHA: 09ecdad80fa018342d02b57fa8693879e9d4ca59

## Summary
Each advisor prompt now names its deliverable behind the `Artifact path: ` anchor. The token scanner can then attribute the advisor transcript to its dispatch and record a measured source.

## Rationale
The scanner already accepts the `Artifact path: ` anchor in the first user text. The advisor prompts named the file only in a write sentence, so no anchor matched. One shared helper adds the anchor once to every seat, which is the smallest change.

## Changed Areas
- `go/internal/core/advisor/prompt.go` — adds `writeArtifactAnchor` and calls it in the routing, plan and persona prompts, once each.
- `docs/architecture/packages/internal-tokenusage.md` — records the agy cache write finding: no field holds it.
- `.evolve/evals/claude-advisor-rows-measured.md` — eval graders for this task.
- `go/acs/cycle2098/predicates_test.go` — predicates of the TDD phase.
- `go/internal/core/advisor/testdata/capture-prompt-plan.golden.txt` — golden prompt now has the anchor line.
- `go/internal/core/advisor/testdata/capture-prompt-replan.golden.txt` — same change for the replan seat.
- `go/internal/core/advisor/testdata/capture-span-plan.golden.json` — prompt hash follows the new prompt.
- `go/internal/core/advisor/testdata/capture-span-replan.golden.json` — prompt hash follows the new prompt.
- `go/internal/core/advisor/testdata/launch-plan.golden.json` — launch prompt has the anchor.
- `go/internal/core/advisor/testdata/launch-proposal.golden.json` — launch prompt has the anchor.
- `go/internal/core/advisor/testdata/launch-replan.golden.json` — launch prompt has the anchor.
- `go/internal/core/advisor/testdata/prompt-plan-legacy.golden.txt` — golden prompt has the anchor.
- `go/internal/core/advisor/testdata/prompt-plan-persona-recon.golden.txt` — golden prompt has the anchor.
- `go/internal/core/advisor/testdata/prompt-plan-persona.golden.txt` — golden prompt has the anchor.
- `go/internal/core/advisor/testdata/prompt-plan-replan.golden.txt` — golden prompt has the anchor.
- `go/internal/core/advisor/testdata/prompt-routing-audit-fail.golden.txt` — golden prompt has the anchor.
- `go/internal/core/advisor/testdata/prompt-routing-happy.golden.txt` — golden prompt has the anchor.
- `go/internal/core/advisor/testdata/prompt-routing-retro.golden.txt` — golden prompt has the anchor.

## Design Decisions
The anchor is the exact string the scanner already matches, so the scanner is unchanged. Attribution still needs the artifact path of the launch, so a transcript of another artifact stays unattributed.

## Verification
Predicates TestC2098_001 to 005 pass. The advisor and tokenusage packages pass. The ACS suite reports red=0.

## Compatibility
The prompt text gains one line. Golden files were regenerated for this reason only.

## Limitations
The share of partial claude-tmux rows on the next wave is measured after the wave runs, not in this cycle.
