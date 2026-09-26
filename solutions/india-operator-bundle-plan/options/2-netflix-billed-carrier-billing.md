# Option 2 — Netflix-billed add-on sold through carrier billing, with a paid-conversion bounty

## Deal mechanism

- **Who bills:** Netflix. Netflix is merchant of record at full retail price. It charges through
  carrier billing, which debits the prepaid balance or adds a line to the postpaid invoice. The
  operator is only the payment rail.
- **Who bears acquisition cost:** Netflix. It pays the operator a bounty for every paid sign-up plus a
  payment-rail fee. The operator gives up no plan margin.
- **Entitlement delivery:** A standard Netflix account created in the Netflix app or on the Netflix
  site. Carrier billing is just one of the payment methods there. No operator login is involved.

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

This option targets the mass prepaid base that Option 1's high-value plans never reach. That is
≈496M subscribers outside the top-spend cohort (A1, A5). The operator places a Netflix offer in its
app, in the recharge flow and in SMS. One tap subscribes the user at the retail ₹149 Mobile price
(E7) and charges the prepaid balance. The retail price is unchanged, and Netflix keeps the
customer relationship.

## Causal chain to incremental paid subscriptions

1. More than 90% of India's base is prepaid (E5). Many prepaid users have no card or UPI mandate set
   up for Netflix. Carrier billing lets them pay from a balance they already top up.
2. The operator's recharge flow puts the offer in front of users at the moment they are spending (A5).
3. By month 24, ≈3.0M people sign up, ≈1.8M stay active (A6), and ≈60% of those are new to Netflix (A7).

## Quantified expected effect

- **Uplift at month 24:** ≈**1.1M incremental paid Netflix memberships** (1.8M active × 60%
  incrementality; A5, A6, A7). That is about +7% on Netflix's India base (E6).
- **Net revenue run-rate at month 24:** incremental memberships add 1.08M × ₹127 × 12 ≈ ₹165 crore/yr
  (A7, A8). Existing payers who switch to carrier billing now pay the rail fee, a loss of
  0.72M × ₹22 × 12 ≈ ₹19 crore/yr (A7, A8). Bounties total 3.0M × ₹300 ≈ ₹90 crore over 24 months,
  about ₹45 crore/yr (A6, A8). The net is ≈ **+₹101 crore/yr**.
- **The net holds up.** No membership is sold below retail. Per active membership, Netflix nets
  ≈₹127 × incrementality, minus ≈₹22 × (1 − incrementality) in rail fees on switched payers, minus
  ≈₹21 a month of bounty (₹45 crore/yr ÷ 1.8M ÷ 12). So net revenue stays positive while
  incrementality is above ≈29% (A6, A7, A8).

## Parties, value exchange and commercials

- **Parties:** Netflix; each operator's carrier-billing gateway, run directly or through an aggregator.
- **Value exchange:** Netflix gets a payment rail and placement in the operator's recharge flow. The
  operator gets a fee on every rupee it collects and a bounty for each new payer, without discounting
  its own plans.
- **Pricing tiers:** Netflix sells all four retail tiers, Mobile ₹149, Basic ₹199, Standard ₹499 and
  Premium ₹649 (E7). The consumer price does not change.
- **Revenue split:** the operator keeps 15% of carrier-billed revenue (A8).
- **Bounty:** ₹300 per paid sign-up that survives its first renewal (A8).
- **Minimum commitments (A13):** the operator guarantees placement in the recharge flow and the app
  home screen for 12 months. Netflix commits to the fee and the bounty for 24 months.

## Technical commitments Netflix would support

- **Integration surface:** the carrier-billing API, with charge, recurring renewal, refund and
  balance-check calls against the prepaid wallet or postpaid invoice. The operator is added as a
  payment method in Netflix checkout. An offer-placement deeplink goes into the operator app, and
  conversion-attribution events feed the bounty.
- **Timeline:** 12 weeks per operator for carrier billing, and 4 weeks for the placement deeplink
  (A12). Launch lands in month 3 on the shared timing basis (A6).
- **Who operates what:** Netflix owns pricing, the member account, checkout, receipts and all customer
  care. The operator (or its aggregator) runs the charging gateway and the placement surfaces, and is
  responsible for the uptime of balance checks. Netflix hosts the attribution service that computes
  the bounty.
- **Regulatory design constraint:** only the subscription is carrier-billed. Data is never priced
  differently, so the option does not touch TRAI's 2016 discriminatory-tariff ban (E10).

## Top two risks and early detection

1. **Conversion is too low for the mass prepaid base.** Retail Mobile at ₹149 is ≈69% of Jio's
   monthly ARPU (E1, E7), so the 0.6% cumulative sign-up rate in A6 may not arrive. *Detect:* paid
   sign-ups per 1,000 offer impressions in the 8-week single-operator pilot. A rate below half of
   the ramp A6 implies for that point is the signal.
2. **Renewal fails on an empty prepaid balance.** Balance-driven lapses exceed the 40% in A6 and
   erode the active base. *Detect:* the failure rate of the first renewal charge in each weekly cohort,
   from week 5 after launch. A failure rate above 30% is the early-warning line (A6).

## Term-sheet outline

- **Term:** 24 months, with a fee review at month 12 (A13).
- **Fees:** 15% payment-rail fee and a ₹300 bounty per retained paid sign-up (A8).
- **Placement:** guaranteed slots in the recharge flow and on the app home screen (A13).
- **Customer ownership:** Netflix is merchant of record. The operator gets no subscriber data beyond
  what billing needs.
- **Exit:** 90 days' notice by either side. Carrier-billed members are offered a switch to UPI or card.
