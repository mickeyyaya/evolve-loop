# Build Explanation — Cycle 1695

## Build Binding
- Cycle: 1695
- Base SHA: 4a2103349a79bde1f8cbc2c96e84c975cc7f8f99

## Summary
This build delivers the ADR-0099 document deliverable for inbox item `india-operator-bundle-plan`. The
deliverable is a partnership-deal plan for Indian telecom operators (Jio, Airtel, Vi) to sell more Netflix
subscriptions with Netflix-supported integration. It holds three mechanistically distinct deal options,
an Options Compared recommendation with a runner-up and flip thresholds, and a sourced evidence file.
The build also tracks the eval file that the TDD phase authored for this item.

## Rationale
The acceptance criteria require at least two deal structures that differ in who bills, who bears
acquisition cost, and how entitlement is delivered. The three options each differ from the others on
all three axes. Option 1 is operator-billed, operator-funded, with operator-ID sign-on. Option 2 is
billed by Netflix through carrier billing, funded by a Netflix bounty, with a standard Netflix account.
Option 3 is billed once at the handset sale, funded by the OEM, with device preload. They are
therefore not the same lever at three sizes. The scout proposed zero-rated delivery. TRAI's 2016
discriminatory-tariffs regulation prohibits it, so the design replaces it with content-neutral embedded
Open Connect caching and names the constraint in every option.

## Changed Areas
- `solutions/india-operator-bundle-plan/assumptions-and-evidence.md` — adds 11 sourced evidence entries,
  covering Q1 FY27 operator results, TRAI data and regulation, Netflix India pricing and subscriber
  estimates, the existing Jio and Airtel bundles, and Open Connect. It also adds 13 labelled assumptions,
  each with a probe, so every number traces to a source or a named assumption.
- `solutions/india-operator-bundle-plan/options/1-operator-billed-hard-bundle.md` — the operator-billed
  hard bundle, with its mechanism triple, causal chain, month-24 uplift, commercials, technical
  commitments, top two risks with detection signals, and a term-sheet outline.
- `solutions/india-operator-bundle-plan/options/2-netflix-billed-carrier-billing.md` — adds the Netflix-merchant carrier-billing add-on with a paid-conversion bounty, because the mass prepaid base that Option 1's high-value plans never reach needs a deal where Netflix bills and funds acquisition.
- `solutions/india-operator-bundle-plan/options/3-handset-preload-oem-funded.md` — the OEM-funded 5G
  handset preload with a 12-month term. It excludes zero-rating explicitly under TRAI 2016.
- `solutions/india-operator-bundle-plan/recommendation.md` — the Options Compared matrix at one
  horizon (month 24), the recommendation (Option 1, with Option 2 as a cohort-disjoint complement),
  the runner-up (Option 3), and flip thresholds computed at the matrix crossovers.
- `.evolve/evals/india-operator-bundle-plan.md` — the eval the TDD phase authored for this item. The
  build tracks it unchanged, so its score caps ship with the deliverable.

## Design Decisions
All options are compared on one shared timing basis: every option starts in month 0 and is read at
month 24. Build lags and ramps are labelled assumptions, and Option 3's later sales start counts only
its time-shifted months. Revenue is valued on the lowest retail tier with an explicit dilution
identity. That identity gives each option a break-even incrementality, and it exposes Option 1's thin
revenue margin. The recommendation turns that margin into a repricing rule instead of hiding it. The
flip thresholds sit at the membership crossover between Option 1 and Option 3 (about 73% of Option 1's
base case), and the 37–40% band between that crossover and revenue break-even is named and justified.

## Verification
`evolve solution check india-operator-bundle-plan` prints OK. All 10 eval `score_cap` evidence commands
exit 0. They cover the mechanism triples, the quantified uplift, the technical commitments, the risks
with signals, the matrix, runner-up and flip, citation resolution, every number citing, sourced
evidence, and the topic check. The eval's `[code]` acceptance grep exits 0.

## Compatibility
No code, configuration or existing document changes. The new paths are additive, under `solutions/`,
`docs/explain/builds/` and `.evolve/evals/`.

## Limitations
Take-up, incrementality, wholesale rates, OEM volume and conversion are unsourced assumptions. Each one
names the probe that would measure it. Two Business Standard figures come from search excerpts,
because the pages refused a direct fetch. No option reaches a target the inbox item never set.
