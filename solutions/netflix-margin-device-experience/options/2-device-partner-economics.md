# Option 2 — Reprice device-partner deals to pay per verified retained member

**Mechanism: device-partner economics (lower distribution payments per member acquired).** This
option changes neither streaming cost nor churn. All numbers cite
`assumptions-and-evidence.md`.

## What changes, for whom

Netflix pays marketing partners: TV and device makers (CE manufacturers), pay-TV operators, mobile
operators and ISPs. The payments buy the Netflix remote button, pre-installation, home-row placement
and bundle positioning, and they are booked in marketing expense (E7). Today many of these payments
are fixed or per-device.

At each renewal, Netflix moves the partner to a single rate card:

1. Pay per member who is verified active and retained. Attribution uses the device's own sign-up
   and activation telemetry, such as sign-ups through the button or the pre-installed app.
2. Stop paying for placements where a holdout test shows no measured incremental lift.

Members see nothing. The change is for partners' commercial teams.

## Causal chain to operating margin

1. Partner payments ≈ $0.7B/yr. This is ≈60% of the ≈$1.14B of FY2024 marketing that is not
   advertising (E8, A5).
2. Paying only for incremental, retained members removes spend on placements with no lift. That
   cuts partner payments by 25% at renewal (A6).
3. Marketing expense falls with revenue unchanged, so operating income rises one-for-one.

## Quantified expected effect

- Full run-rate savings: $0.7B (A5) × 25% (A6) ≈ **$175M/yr ≈ +0.39pp** at FY2025 revenue (E1).
- Timing: a 3-month attribution build comes first (A14). Renewals follow A7, and the contracts
  renewing in months 1–3 keep current terms until after month 36 (A14). The repriced share is
  37.5% / 62.5% / 87.5% / 100% at the end of years 1 / 2 / 3 / 4 (A7, A14). That gives
  ≈ $66M ≈ **+0.15pp** (year 1), ≈ $109M ≈ +0.24pp (year 2), ≈ $153M ≈ **+0.34pp** (year 3), and
  the full +0.39pp in year 4.
- Each quarter's delay in starting forfeits another 12.5% of contract value until month 36 (A14),
  which is ≈ 0.05pp of the year-3 effect.
- **This option alone cannot reach +1pp.** It would need a 65% cut to the whole assumed partner pool.

## Cost and time-to-effect

- Cost: an attribution and measurement build (device-level holdouts per partner) plus commercial
  negotiation effort. No member-facing engineering.
- Time-to-effect: first savings from month 4, after the 3-month attribution build (A14). The rest
  follows contract renewal dates through month 36 (A7), and the full run-rate arrives in year 4
  (A14).

## Top risks and early detection

1. **Partners retaliate with worse placement.** Examples are losing the remote button or being
   moved off the home row. Gross adds fall and the revenue loss exceeds the savings. *Detect:* a
   sign-up-through-partner KPI for each partner, compared against the holdout, in the first renewal
   quarter. Pause if attributed gross adds fall more than 5% relative.
2. **The partner pool is smaller than A5.** Netflix has not disclosed the split since the 10-K
   wording in E7. *Detect:* the internal marketing ledger in the first quarter (A5 probe). If the
   pool is under $0.4B, the option is worth under 0.23pp even at full run-rate.

## Why it is the runner-up

It needs no change in member behavior. Its magnitude unknowns are cheap and quick to probe: A5 is
a ledger read and A6 is a 90-day one-partner holdout, both about one quarter. It also has the
largest year-1 effect of the three. It ranks second for three reasons:

- Its year-3 effect (+0.34pp) is smaller than Option 3's.
- The repriced contracts are multi-year, so it is hard to reverse.
- Its main risk, partner retaliation (risk 1), is a partner reaction that neither probe measures.
  That reaction first shows at the repriced renewals from month 4 (A14), and it lands on the same
  device-experience surface the goal is meant to improve.
