# Build Explanation — Cycle 1692

## Build Binding
- Cycle: 1692
- Base SHA: cb625ba622db7a8bbcfe6b098e1993c33922f919

## Summary
This build delivers the ADR-0099 document for inbox item `netflix-margin-device-experience`. It
contains three strategy options, each moving a different mechanism (delivery cost-to-serve,
device-partner economics, and churn through playback quality). It also contains a recommendation
built from an Options Compared matrix, and an assumptions-and-evidence file that every number cites.

## Rationale
The inbox acceptance asks for at least two mechanistically distinct options, each with a quantified
effect resting on a named assumption, plus a recommendation that names the runner-up and the
evidence that would flip it. Three options cover the mechanisms the inbox record lists without
repeating one lever at different sizes. Netflix does not disclose delivery cost, partner payments,
device mix or churn by device. The document therefore anchors every calculation on sourced
public figures (FY2025 shareholder letter, FY2024 10-K and annual report, Netflix TechBlog, Antenna
churn data, Krishnan & Sitaraman IMC 2012). It labels each gap as an explicit `A<n>` assumption and
names a probe that would replace it.

## Changed Areas
- `solutions/netflix-margin-device-experience/assumptions-and-evidence.md` — adds the baseline arithmetic (1pp ≈ $452M at FY2025 revenue), one timing basis shared by all options, 14 labelled assumptions (A13 is Option 3's 6-month build lag and A14 is Option 2's 3-month attribution lag; A5 and A6 name their probes), and 10 sourced evidence entries. Every number in the options and the recommendation cites one of these entries, so the audit can check that each source supports the magnitude and direction.
- `solutions/netflix-margin-device-experience/options/1-delivery-cost-to-serve.md` — the cost-to-serve option: move TV playback to AV1, worth ≈ +0.05pp at year 1 and ≈ +0.10pp from month 24 on its labelled linear ramp (A3). It states plainly that this option cannot reach the target alone. It is included because it is the only cost-side mechanism, so the comparison does not rest on revenue levers alone.
- `solutions/netflix-margin-device-experience/options/2-device-partner-economics.md` — the device-partner option: reprice CE and operator deals to pay per verified retained member, worth ≈ +0.15pp at year 1, ≈ +0.34pp at year 3 and ≈ +0.39pp at full run-rate in year 4, after the A14 attribution lag. Its runner-up rationale now credits the quarter-long A5 and A6 probes. It is included because it changes who pays, which is a mechanism distinct from delivery cost and churn, and it is the runner-up the recommendation must name.
- `solutions/netflix-margin-device-experience/options/3-playback-quality-retention.md` — the churn option: fix playback on the low-end TV long tail, worth ≈ +0.12pp at year 1 and ≈ +0.50pp at year 3 after the 6-month A13 build lag. It includes a horizon table so the compounding arithmetic can be audited. It is included because churn is the largest lever the device experience controls, and it is the recommended option.
- `solutions/netflix-margin-device-experience/recommendation.md` — the Options Compared matrix on one timing-basis row, the recommendation (Option 3 plus Option 1 as a complement, chosen on year 3), the runner-up (Option 2), flip thresholds at the matrix crossover (≈0.10pp churn cut, ≈17% cohort), and a statement that +1pp is not reached within three years: the parallel three-option portfolio gives ≈0.94pp by year 3 and ≈1.12pp by year 4.
- `.evolve/evals/netflix-margin-device-experience.md` — commits the TDD phase's eval file, which it left untracked in the worktree. Its score caps are the graders this deliverable is judged by, so it must ship with the deliverable to be replayable.
- `.evolve/inbox/2026-09-10T00-10-00Z-netflix-margin-device-experience.json` — removed from the open inbox by the ship phase's claim-consume step. The item is done by this cycle, so it must leave the open queue and not be dispatched again. The deliverable does not depend on this record.
- `.evolve/inbox/consumed/2026-09-10T00-10-00Z-netflix-margin-device-experience.json` — the same inbox record, moved to `consumed/` by ship with its lifecycle fields updated. It is the audit trail that this cycle claimed and delivered the item. It entered the base-bound diff when the cycle commit was rebased onto the new fleet base.

## Design Decisions
All margin effects use one baseline (FY2025, E1) and one revenue-lever formula (contribution
margin A10). This keeps the options comparable in the matrix. For Option 3, retained members decay
at the base churn rate (A12), which is conservative, rather than being modelled as a steady-state
member-base uplift. The steady-state model overstates the effect over any horizon an operator
would plan for. Every option starts in month 0, and each option's lag and ramp is a labelled
assumption (A3, A7 with A14, A13). The matrix therefore compares run-rates at the same horizon.
Audit round 1 found that an earlier draft compared un-lagged Option 3 figures with renewal-timed
Option 2 figures. The recommendation is made on the year-3 run-rate. It says openly that Option 2
leads in year 1 and is quicker to size. Its flip thresholds sit at the crossover the matrix
implies.

## Verification
`evolve solution check netflix-margin-device-experience` exits 0. All twelve score-cap evidence
commands in `.evolve/evals/netflix-margin-device-experience.md` exit 0. Caps 1–5 check the
contract, quantified options, citation resolution, runner-up and flip, and topic. Caps 6–12 check
the timing-basis row, the labelled time-to-effect, that year 1 is below year 3, that portfolio sums
match the matrix, the flip crossover, the absence of untestable-runner-up claims, and the
time-to-know probes.

## Compatibility
The change is documents only. It changes no code, configuration or protected control-plane path.

## Limitations
The magnitude of every option rests on an unsourced assumption (A1, A5, A9 and the A13 build lag
are the most important), because Netflix does not disclose those figures. The Antenna churn figure is a US
estimate that the document applies globally.
