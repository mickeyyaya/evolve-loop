# Option 3 — 5G handset preload with an OEM-funded 12-month Netflix term

## Deal mechanism

- **Who bills:** Operator retail, once, at the handset sale. The 12-month Netflix term sits inside the
  handset bundle price. It is bought wholesale and upfront from Netflix, and it never recurs.
- **Who bears acquisition cost:** The handset OEM. It pays the term's wholesale price out of its
  5G-upgrade marketing budget. Neither the operator's plan margin nor Netflix's marketing pays.
- **Entitlement delivery:** Device preload. The Netflix app ships preinstalled, and the 12-month
  entitlement is provisioned against the device at first boot. The user needs no code and no
  operator login.

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

The option targets the funnel stage where a user buys a new 5G handset. Jio alone has about 285M 5G
users (E1). Under the deal:

- An OEM builds a bundle SKU with a year of Netflix Mobile included.
- The operator's stores sell it.
- Netflix provisions the entitlement at first boot.

At month 13 the member is asked to continue at the retail ₹149 (E7).

**The option does not zero-rate Netflix data.** An early version of this idea paired preload with
zero-rated Netflix data. TRAI's 2016 regulation prohibits that (E10), so the design drops it.
Network-aware delivery comes instead from embedded Open Connect Appliances, which are free to the
operator and content-neutral for the subscriber (E11).

## Causal chain to incremental paid subscriptions

1. The OEM uses a year of Netflix to sell a higher-priced 5G handset (A9, A10).
2. Each bundled handset carries a paid, OEM-funded Netflix membership from its first boot. By month
   24, ≈4.5M such devices have been sold, and the ≈3.0M sold in months 13–24 are still in term (A9).
3. When a term expires, 30% of members convert to paying retail themselves (A11). About 70% of all
   Option 3 memberships are new to Netflix (A11).

## Quantified expected effect

- **Uplift at month 24:** ≈**2.4M incremental paid Netflix memberships**. That is 3.0M in term × 70%
  incrementality plus 0.45M converted × 70% (A9, A11). It is about +15% on the India base (E6).
- **Net revenue run-rate at month 24:** in-term incremental memberships add 2.1M × ₹60 × 12 ≈ ₹151
  crore/yr (A10, A11). In-term non-incremental memberships dilute by 0.9M × ₹89 × 12 ≈ ₹96 crore/yr
  (A10, A11, E7). Incremental converters add 0.315M × ₹149 × 12 ≈ ₹56 crore/yr (A11, E7). The net is
  ≈ **+₹111 crore/yr**.
- **The effect has a cliff.** When each cohort's term ends, 70% of it lapses (A11). The month-24
  number therefore depends on continued OEM volume. It is not a recurring base.

## Parties, value exchange and commercials

- **Parties:** Netflix, one or more handset OEMs, and operator retail (Jio, Airtel and Vi stores).
- **Value exchange:** the OEM gets a differentiator for its 5G upgrade. The operator gets a 5G handset
  sale and a data-heavy user. Netflix gets a paid membership at first boot and a chance to convert it
  to retail at month 13.
- **Pricing tiers:** a 12-month Mobile term at 40% of retail, ≈₹715 per term (A10, E7). A Basic term
  is available on the same 40% basis.
- **Revenue split:** Netflix receives the full wholesale term price. The operator earns its normal
  handset-retail margin. Netflix pays no share.
- **Minimum commitments (A13):** each OEM commits to 1M terms a year, paid quarterly in advance.

## Technical commitments Netflix would support

- **Integration surface:** device-partner certification of each bundle SKU. The preload runs inside
  the OEM's firmware. A device-entitlement API provisions the 12-month term against a device
  identifier at first boot. A renewal prompt and payment-mandate capture (UPI, card or carrier
  billing) come in month 11.
- **Timeline:** 6 months to certify the first SKUs and to ship firmware with the preload (A12).
  Bundle sales start in month 7 (A9).
- **Who operates what:** the OEM owns the preload, the firmware and payment for the terms. Netflix
  runs the entitlement service and certification, and owns the member relationship from first boot.
  The operator runs the retail sale and handset stock. The operator hosts any embedded Open Connect
  Appliances, and Netflix operates them (E11).
- **Regulatory design constraint:** zero-rating is excluded under TRAI's 2016 regulation (E10). Data
  consumed on bundle devices is charged under the subscriber's ordinary, content-neutral tariff.

## Top two risks and early detection

1. **OEMs will not fund the term.** At A9 volume the programme costs an OEM ≈₹215 crore a year
   (A9, A10). No OEM has committed. *Detect:* signed letters of intent with committed volumes by
   month 3. Volume below 1.5M handsets a year halves this option's effect (A9 probe).
2. **The month-13 conversion cliff.** Fewer than 30% of expiring terms convert to retail (A11).
   *Detect:* streaming hours in term months 1–3 of the first cohort, and the share of users who set
   up a payment mandate at the month-11 prompt. A mandate capture rate below 20% signals that
   conversion will come in under 30% (A11 probe).

## Term-sheet outline

- **Term:** 24 months, with a volume review at month 12 (A13).
- **Price:** ≈₹715 per 12-month Mobile term, prepaid quarterly and non-refundable (A10).
- **Minimum commitment:** 1M terms a year per OEM (A13).
- **Certification:** Netflix certifies each SKU before it ships. The OEM ships Netflix app updates.
- **Member ownership:** Netflix owns the account from first boot. The OEM gets aggregate activation
  counts only.
- **Compliance:** no zero-rating or other content-based data pricing on bundle devices (E10).
