# Recommendation — Netflix × Indian telecom operators: bundle plan and partnership deal

Goal: design a deal with Indian telecom operators (Jio, Airtel and Vi) so that they sell more Netflix
subscriptions, with Netflix supporting their technical integration. The three big operators serve
≈1,103M mobile subscribers (E1, E2, E3), and Netflix has more than 16M paying members in India (E6).
All numbers cite `assumptions-and-evidence.md`.

The three options each change a different deal mechanism:

- **Option 1** is operator-billed, operator-funded, with entitlement through operator-ID sign-on.
- **Option 2** is Netflix-billed through carrier billing and Netflix-funded by a bounty, with a standard
  Netflix account.
- **Option 3** is billed once at the handset sale, OEM-funded, with entitlement through device preload.

**Timing basis.** All three options start in month 0 and are compared at **month 24**. Each option's
build lag and ramp is a labelled assumption, shown in the first matrix row. Option 3 is not a later
wave. Its 6-month build means it sells for only 18 of the 24 months, and the matrix counts only
those months.

## Options Compared

| Criterion (all rows at month 24) | Option 1 — Operator-billed hard bundle | Option 2 — Netflix-billed carrier billing + bounty | Option 3 — OEM-funded handset preload |
|---|---|---|---|
| Timing basis: start, lag, ramp (A2, A6, A9, A12) | Month 0; 12-week build; linear ramp months 3–24 | Month 0; 12-week build; linear ramp months 3–24 | Month 0; 6-month build; sales months 7–24 |
| Who bills / who bears acquisition cost / entitlement delivery | Operator / operator / operator-ID sign-on | Netflix via carrier billing / Netflix bounty / standard Netflix account | Operator retail, once / handset OEM / device preload |
| Partner memberships active (A2, A6, A9, A11) | 6.6M | 1.8M | 3.45M (3.0M in term + 0.45M converted) |
| **Incremental paid Netflix memberships** (A1, A2, A3, A5, A6, A7, A9, A11) | **3.3M** | **1.1M** | **2.4M** |
| Uplift on the India base (E6 plus the row above) | +21% | +7% | +15% |
| Net Netflix revenue run-rate (A3, A4, A7, A8, A10, A11, E7) | +₹114 crore/yr | +₹101 crore/yr | +₹111 crore/yr |
| Break-even incrementality for net revenue ≥ 0 (A4, A8, A10) | 40% (assumed 50%, A3) | ≈29% (assumed 60%, A7) | ≈52% (assumed 70%, A11) |
| Who pays the acquisition cost, per year (A3, A4, A8, A9, A10) | Netflix price concession ≈₹238 crore (dilution) | Netflix fees and bounty ≈₹64 crore | OEM ≈₹215 crore; Netflix concession ≈₹96 crore |
| Published Indian precedent (E8, E9) | Yes — Jio and Airtel packs | None found | None found |
| Confidence in magnitude (A2, A3, A6, A9) | Medium: mechanism proven (E8, E9); take-up and incrementality unsourced | Low: mass prepaid base is price-sensitive (E1, E7) | Low: no OEM has committed volume |
| First decisive read (A2, A3, A6, A9, A11) | Month 1 pack counts; month 6 holdout | Month 5 pilot conversion | Month 3 OEM letters of intent; month 19 first term expiry |
| Durability past month 24 (A11) | Recurring while the plan is active | Recurring retail billing | 70% of each cohort lapses at term end |
| TRAI 2016 exposure (E10) | Content bundle only; data stays content-neutral | None | Zero-rating excluded by design |
| Netflix engineering (A12) | Extend the existing entitlement API | New carrier-billing API per operator | Device certification and a device-entitlement service |
| Reversibility (A13) | Medium: 36-month term, exit after month 12 | High: 90-day exit | Low: devices in market carry 12-month terms |

## Recommendation

**Recommended: Option 1, the operator-billed hard bundle, as the lead deal with Jio, Airtel and Vi.
Run Option 2 in parallel from month 0 as a complement for the mass prepaid base.** The choice is made
on the month-24 row for incremental paid memberships. That row is the goal, which is to sell more
Netflix subscriptions.

Why the matrix favors Option 1:

1. **Most incremental memberships.** Option 1 gives ≈3.3M, against ≈2.4M for Option 3 and ≈1.1M for
   Option 2 (A1–A3, A5–A7, A9, A11).
2. **Revenue does not separate the options.** All three net about ₹100–115 crore/yr at month 24 (A3,
   A4, A7, A8, A10, A11, E7). The matrix therefore ranks them on memberships and confidence, not on
   revenue.
3. **It is the only option already proven in India.** Jio and Airtel sell Netflix packs today
   (E8, E9), so Netflix extends an existing entitlement integration in 12 weeks rather than building
   a new one (A12). It also has the earliest first read: operator pack counts in month 1 (A2 probe).
4. **Its weakness is known and has a price fix.** Option 1 has the thinnest revenue safety margin:
   break-even incrementality is 40% against an assumed 50% (A3, A4). The flip section turns that
   into a repricing rule, so the cost of the choice is visible from the start.

**Why add Option 2 alongside it.** Option 2 reaches a different cohort by design. It serves the ≈496M
prepaid users outside Option 1's top-spend cohort (A1, A5), so the two options do not cannibalize each
other. It needs no retail price concession and stays net-positive above ≈29% incrementality (A7, A8).
Together they add ≈4.4M incremental memberships by month 24, about +28% on Netflix's India base, and
≈₹215 crore/yr of net revenue (E6, A3, A7). Option 2 should start in month 0, not in a later wave.
Its sign-ups ramp over months 3–24, so each quarter of delay removes about 3/21 ≈ 14% of its month-24
effect (A6).

**Runner-up: Option 3 — OEM-funded handset preload.** It ranks second on incremental memberships
(≈2.4M; A9, A11), nets about the same revenue, and costs Netflix no marketing cash, because the OEM
pays for the term (A10). It is second for three reasons, each drawn from the evidence file:

- No OEM has committed any volume, so its whole effect rests on an unsourced A9.
- 70% of each term cohort lapses when its term ends (A11), so its base does not recur the way
  Option 1's does.
- Its decisive read, the first term expiry, arrives only in month 19 (A9, A11).

Next step for Option 3: take it only as far as OEM letters of intent, which is the cheap A9 probe due
by month 3. Build nothing until then.

**Honest target statement.** The inbox item sets no numeric target. On base-case assumptions the
recommended pair lifts Netflix's paid base in India by ≈+28% within 24 months (E6, A1–A3, A5–A7). Every
link in that estimate is a labelled assumption, and none of it is sourced: take-up (A2, A6) and
incrementality (A3, A7) could each be off by half. No part of the recommendation relies on zero-rating,
which TRAI prohibits (E10).

## Evidence that would flip the choice

The thresholds below sit at the crossovers in the matrix. Option 1's 3.3M incremental memberships
fall to Option 3's 2.4M at 2.4 ÷ 3.3 ≈ 73% of Option 1's base case (A1–A3, A9, A11).

- **The Option 1 holdout shows lower incrementality.** At 6% take-up, Option 1 loses its lead on
  memberships to Option 3 below **≈37% incrementality**, since 2.4M ÷ 6.6M ≈ 37% (A2, A3). If the
  90-day holdout reads below 37%, **flip to Option 3** as the lead deal, provided its letters of
  intent have arrived (A9).
  - *Named band, 37–40%.* Option 1's net revenue turns negative at 40% (A4). In that band Option 1
    still leads on memberships but loses money. Inside the band the response is to **reprice, not
    switch**: raise the wholesale rate to at least (1 − incrementality) × ₹149, for example ≈₹92 at
    38% (A4, E7). The band is justified because the goal is memberships, and a rate change restores
    revenue without giving up the 3.3M.
- **Operator take-up is lower.** At 50% incrementality, Option 1 falls below Option 3 if take-up is
  under **≈4.4%** of the top-spend cohort by month 24, since 2.4M ÷ 55M ≈ 4.4% (A1, A2, A3). The operators'
  month-1 counts on the existing packs (E8, E9) give the first read on A2.
- **OEM volume is higher.** Option 3 yields ≈0.8M incremental memberships for every 1M handsets a year
  (A9, A11). If signed letters of intent commit **≥4.1M handsets a year** by month 3, Option 3 exceeds
  Option 1's 3.3M (A9) and **becomes the lead deal**. Option 1 would still continue as the complement
  for the top-spend cohort.
- **Option 2 would need three times its assumed conversion to lead.** It passes Option 1 only if
  cumulative sign-ups reach **≈1.8%** of the reached base, 3.3 ÷ 1.1 ≈ 3 times A6 (A6, A7). The month-5
  pilot answers that.
- **A regulatory shift.** TRAI's 2016 regulation covers data tariffs (E10). If TRAI extended it to
  content bundled inside tariffs, Option 1 could no longer be sold as a plan feature. The lead would then
  pass to Option 3, which is sold with the handset rather than inside a tariff.
