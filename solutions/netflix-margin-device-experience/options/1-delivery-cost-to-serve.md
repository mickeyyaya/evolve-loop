# Option 1 — Cut delivery cost-to-serve by moving TV playback to AV1

**Mechanism: cost-to-serve (fewer bits per streamed hour).** This option changes no revenue line.
All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

Netflix raises AV1's share of streamed hours from ~30% (E5) to ~60% (A3). The ramp runs linearly
over months 1–24, starting in month 0 (A3). The change is on the living-room device fleet:

- AV1 hardware decode becomes a requirement in the partner-device certification program for new
  TVs and streaming sticks.
- AV1 becomes the default ladder on every capable TV. It stays opt-in on mobile.
- Film-grain synthesis is extended to the grain-heavy catalog. On those titles it cuts bitrate by
  66% (E5).

Members see no change except fewer rebuffers (E5). The cost falls on Netflix's delivery cost
center.

## Causal chain to operating margin

1. More hours stream in AV1, and each migrated hour uses one-third fewer bits (E5, A4).
2. Moving 30 percentage points of hours at a one-third saving cuts total bits delivered by ≈10%.
3. Fewer bits at peak means fewer Open Connect appliances to buy and refresh (E6) and less
   transit. About 50% of delivery cost scales this way (A2).
4. Lower "other cost of revenues" (E4) raises operating income one-for-one with revenue unchanged.

## Quantified expected effect

- End state (month 24 onward): delivery cost base ≈ $0.9B (A1) × 10% bits × 50% bit-variable
  share (A2) ≈ **$45M/yr** savings ≈ **+0.10pp** of operating margin at FY2025 revenue
  ($452M = 1pp; E1). The effect then stays flat. The plausible range is 0.05–0.2pp, depending
  mostly on A1.
- End of year 1: the AV1 share is only ≈45% (A3 linear ramp), so the bit cut is ≈5% and the
  run-rate is half the end state, ≈ $22M/yr ≈ **+0.05pp**. That is an upper bound, because A2
  lets cost follow bits over up to 24 months.
- **This option alone cannot reach +1pp.** Delivery would need to cost ≈$9B for a 10% bit cut to
  free $452M. That is larger than all of FY2024 other cost of revenues ($5.74B; E4).

## Cost and time-to-effect

- Cost: AV1 and film-grain encoding compute across the catalog, plus certification engineering.
  Encoding is a one-time cost per title and ladder, not a per-stream cost. Netflix already runs AV1
  at scale (E5), so this extends an existing pipeline.
- Time-to-effect: half the effect by month 12 and all of it by month 24 (A3 linear ramp). The
  binding constraint is device-fleet turnover, since only new TVs gain hardware decode.

## Top risks and early detection

1. **Delivery cost is smaller than A1.** Embedded appliances make ISPs carry power and space (E6),
   so the saving could be half of the estimate. *Detect:* the Open Connect cost-center ledger in
   the first quarter. If delivery is below $0.5B, the option is worth under 0.06pp.
2. **Migrated hours save less than one-third** (A4 is optimistic for older and lower-resolution
   devices). *Detect:* compare bits per hour for migrated cohorts against the E5 baseline after
   the first certification wave.

## Why it still matters

It is the only option here with high confidence in direction and near-zero member risk. Its 45%
rebuffer reduction (E5) also feeds Option 3's retention mechanism, so it is a no-regret complement
rather than a stand-alone path to +1pp.
