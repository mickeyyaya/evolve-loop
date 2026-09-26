# Option 3 — Fix playback on the low-end TV long tail to cut churn

**Mechanism: churn reduction through playback quality (revenue retained at high contribution
margin).** This option changes neither delivery cost nor partner payments. All numbers cite
`assumptions-and-evidence.md`.

## What changes, for whom

The target is the ~25% of members whose primary screen is a low-end or legacy TV device (A8):
older smart-TV platforms and low-memory sticks, where startup is slow, rebuffers are frequent
and the app crashes. Netflix gives that device class its own quality program:

- a lighter app runtime and startup prefetch, aimed at getting startup under 2s (E10);
- per-device bitrate ladders, with AV1 wherever decode allows (E5 shows 45% fewer rebuffers on
  AV1 sessions);
- a crash and hang burn-down;
- a device-class quality-of-experience scorecard reviewed every week.

## Causal chain to operating margin

1. Startup over 2s and rebuffering measurably cut viewing. Each extra second of startup adds 5.8%
   to abandonment, and a 1% rebuffer ratio costs 5% of play time (E10). Worse sessions mean less
   engagement.
2. Less engagement shows up as cancellations. Fixing the cohort cuts its gross churn from 1.8% (E9)
   by 0.15pp/month, or 0.0375pp/month across the whole base (A8, A9).
3. That is ≈122K fewer cancellations per month on 325M members (E3), from month 7 onward after
   the 6-month build (A13). Retained members accumulate, and they then leave at the 1.8% base
   rate (A12).
4. Each retained member is worth ≈$144/yr (A11). About 85% of it drops to operating income because
   content cost is fixed in the short run (A10). Margin rises because revenue grows faster than
   cost.

## Quantified expected effect

At FY2025 revenue and margin (E1), Δmargin = (OI + ΔR·0.85) ÷ (R + ΔR) − 29.5%. Nothing is
credited before month 7 (A13):

| Horizon | Months of fix live (A13) | Retained members | Revenue run-rate ΔR | Operating margin |
|---|---|---|---|---|
| End of year 1 | 6 | ≈0.70M | ≈$101M | **≈ +0.12pp** |
| End of year 2 | 18 | ≈1.89M | ≈$272M | ≈ +0.33pp |
| End of year 3 | 30 | ≈2.84M | ≈$410M | **≈ +0.50pp** |
| End of year 4 | 42 | ≈3.61M | ≈$520M | ≈ +0.63pp |

For comparison, the same model with no build lag gives +0.23pp at year 1 and +0.57pp at year 3.
Each month of build lag costs ≈0.013pp of the year-3 effect.

The effect keeps compounding, but **it does not reach +1pp alone within 3 years** on these
assumptions. It would need about twice the A9 churn reduction.

## Cost and time-to-effect

- Cost: a dedicated device-platform engineering team, plus per-device ladder encoding. The ladder
  work shares the pipeline with Option 1.
- Time-to-effect: the fix is live from month 7 after a 6-month build (A13). The churn signal is
  then measurable in a **90-day A/B holdout** on the device cohort (A9), so the first read comes
  around month 9. The margin effect builds over years 1–4 and is still compounding at year 4.

## Top risks and early detection

1. **Churn elasticity is far smaller than A9.** Netflix's churn is already the lowest in the
   industry (E9), and quality may not be why low-end-device members cancel. *Detect:* the 90-day
   cohort holdout. If the cohort churn cut is under ≈0.10pp/month (two-thirds of A9), Option 2
   becomes the larger year-3 lever (recommendation, flip section).
2. **The low-end cohort is smaller than 25%** (A8), because the fleet renews faster than assumed.
   *Detect:* device-class telemetry in week 1. The effect scales down proportionally. Below ≈17%
   of members, Option 2 becomes the larger year-3 lever.
