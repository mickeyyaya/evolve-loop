# Option 1 — Operator-billed hard bundle on high-value plans

## Deal mechanism

- **Who bills:** The operator. Netflix is part of the plan or recharge price, and the subscriber sees
  one operator charge.
- **Who bears acquisition cost:** The operator. It pays Netflix a wholesale fee for every activated
  membership and funds all plan marketing.
- **Entitlement delivery:** Operator-ID sign-on. The subscriber activates Netflix from the operator app
  or SMS link. Netflix binds the mobile number to a Netflix account through its partner activation
  integration.

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

Today the model exists only as two 84-day prepaid packs at ₹1,099 and ₹1,499 (E8, E9). This option
extends it across each operator's top-spend cohort, ≈110M subscribers (A1):

- Netflix Mobile or Basic goes into postpaid and family plans. Airtel has 30M postpaid customers
  (E2) and Vi has 31.9M (E3).
- The pack ladder gets a 28-day prepaid step. That brings the entry price below today's ₹1,099
  84-day threshold (E8).
- Each operator decides where Netflix sits in its plan ladder. Netflix sets the wholesale rate card
  and a minimum-guarantee floor.

The subscriber gets Netflix without a separate payment. The operator uses Netflix to move users up
its plan ladder and to cut churn. Jio's monthly churn is already 1.6% (E1).

## Causal chain to incremental paid subscriptions

1. The operator's own sales force and app promote a plan that includes Netflix, so there is no
   separate checkout and no card or UPI mandate (E5: over 90% of the base is prepaid).
2. 6% of the top-spend cohort is on such a plan by month 24, ≈6.6M bundle memberships (A1, A2).
3. Half of them are new to Netflix (A3). Every one of them pays Netflix wholesale through the
   operator (A4).

## Quantified expected effect

- **Uplift at month 24:** ≈**3.3M incremental paid Netflix memberships** (6.6M bundle memberships ×
  50% incrementality; A1, A2, A3). That is about +21% on Netflix's India base of more than 16M (E6).
- **Net revenue run-rate at month 24:** incremental memberships add 3.3M × ₹89 × 12 ≈ ₹352 crore/yr
  (A3, A4). Non-incremental memberships dilute by 3.3M × (₹149 − ₹89) × 12 ≈ ₹238 crore/yr (A3, A4,
  E7). The net is ≈ **+₹114 crore/yr**.
- **The net is fragile.** At a ₹89 wholesale price, net revenue turns negative once incrementality
  falls below 40% (A4). A4's price rests entirely on the A3 read-out.

## Parties, value exchange and commercials

- **Parties:** Netflix; Jio, Airtel and Vi (one agreement each).
- **Value exchange:** Netflix gains reach into the top-spend cohort and gets billing through the
  operator. The operator gets a plan-upgrade and retention lever at a wholesale price.
- **Pricing tiers:** wholesale is set at 60% of retail, per activated membership per month (A4).
  That is ≈₹89 on Mobile, and a Basic rate on the same 60% basis, ≈₹119 (60% × ₹199, E7, A4).
- **Revenue split:** the operator keeps the whole plan price and pays Netflix the wholesale rate
  above. There is no percentage split of the plan.
- **Minimum commitments (A13):** a minimum guarantee of 500k paid activations per operator in year 1.
  The operator pays any shortfall at the wholesale rate. Netflix commits to the rate card for 24 months.

## Technical commitments Netflix would support

- **Integration surface:** the partner entitlement and activation API that Jio and Airtel already use
  for their packs (E8, E9). New plan SKUs map to Netflix tiers. Activation binds the mobile number to
  a Netflix account. The API also handles suspension on non-renewal, and monthly entitlement
  reconciliation files support billing audit.
- **Network-aware delivery:** Netflix offers embedded Open Connect Appliances at no charge (E11). This
  cuts the operator's transit cost for Netflix traffic without touching data tariffs.
- **Timeline:** 12 weeks to add new SKUs for Jio and Airtel, which are already integrated. 6 months
  for Vi, which starts from scratch (A12).
- **Who operates what:** the operator owns billing, the plan catalogue, first-line billing care and
  the activation UX in its app. Netflix owns the member account, playback, content and the uptime
  of the entitlement API. The operator hosts any embedded appliances. Netflix operates them
  remotely (E11).
- **Regulatory design constraint:** the bundle includes Netflix *content*. It never prices Netflix
  *data* differently from other data. Data allowances stay content-neutral, so the design stays
  inside TRAI's 2016 ban on discriminatory tariffs (E10).

## Top two risks and early detection

1. **Cannibalization is worse than assumed.** The top-spend cohort overlaps Netflix's existing
   affluent payers (E6). If incrementality falls below 40%, the deal loses revenue (A3, A4).
   *Detect:* each week from launch, the share of activations that match an existing or recently
   lapsed Netflix account. A share above 55% by week 6 means incrementality is heading below 45%.
   The 90-day matched holdout then confirms it (A3 probe).
2. **Activations without viewing.** The operator counts Netflix as a plan feature. Subscribers never
   start streaming, so at renewal the operator pushes for a lower wholesale rate (A4). *Detect:*
   the share of activated accounts that stream in their first 30 days, per operator, each month.
   A share below 60% in the first two monthly cohorts is the signal (A2 probe).

## Term-sheet outline

- **Term:** 36 months, with a rate-card review at month 24 (A13).
- **Rate card:** Mobile ≈₹89 and Basic ≈₹119 per activated membership per month (A4, E7).
- **Minimum guarantee:** 500k paid activations per operator in year 1 (A13).
- **Data sharing:** monthly activation, streaming and churn data by plan SKU.
- **Exit:** either side may exit on 90 days' notice after month 12, with existing activations
  honoured to plan end.
- **Compliance:** every tariff containing the bundle stays content-neutral on data (E10).
