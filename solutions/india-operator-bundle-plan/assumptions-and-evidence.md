# Assumptions and Evidence — Netflix × Indian telecom operator bundle plan

Every number in `options/*.md` and `recommendation.md` cites an entry below. `E<n>` entries are
sourced facts: operator quarterly results, TRAI data and regulations, press reports of published
bundles, and Netflix's own partner documentation. Each one names its source. `A<n>` entries are
**assumptions**. They are numbers we could not source publicly. Each one states the reasoning behind
it and the probe that would replace it with a measurement. Netflix and the operators do not publish
bundle take-up, incrementality or wholesale rates, so every option rests on at least one assumption.
The recommendation takes that into account.

## Baseline arithmetic (derived only from the entries below)

- **Big-three mobile base** ≈ 533.3M (Jio, E1) + 376.5M (Airtel, E2) + 193.1M (Vi, E3) ≈
  **1,103M** subscribers at the end of June 2026. That is ≈84% of the 1,306M wireless subscriptions
  TRAI counted for July 2026 (E4). BSNL, MTNL and FWA make up the rest.
- **Retail price basis.** Every option is valued on the Netflix **Mobile** plan at **₹149/month**
  (E7). That is the lowest retail tier and the plan the ₹1,099 Jio pack carries (E8). Using the
  cheapest tier understates both the upside and the dilution for bundles that carry Basic (₹199, E7).
- **Scale reference.** Netflix had **more than 16M** subscribers in India in January 2026 (E6).
  "+1M incremental memberships" is therefore about +6% on the India base (E6).
- **Timing basis (one for all options).** Every option starts in **month 0**. The comparison
  horizon is **month 24**. A revenue figure is the annualized run-rate at month 24. Each option's lag
  and ramp is a labelled assumption: A2 for Option 1, A6 for Option 2 and A9 for Option 3.
- **Revenue identity.** An activated partner membership that Netflix would not otherwise have had
  adds its partner price. A non-incremental one (an existing payer, or someone who would have joined
  at retail anyway) *loses* the gap between retail and the partner price. So net revenue per month ≈
  incremental × partner price − non-incremental × (retail − partner price). Net revenue is positive
  only while incrementality exceeds 1 − partner price ÷ retail.

## Assumptions

- **A1** — Option 1's **addressable cohort is the top 10% of the big-three base by spend, ≈110M
  subscribers**. These are postpaid, family-plan and high-value prepaid-pack users. The proxy is
  India's ≈10% postpaid share (E5). *Unsourced sizing.* Operators do not publish spend deciles.
  Probe: each operator's count of subscribers on plans priced at or above its ₹1,099 84-day pack (E8)
  or its postpaid base (E2, E3).
- **A2** — **6% of the A1 cohort** holds a Netflix-inclusive plan at month 24, ≈6.6M bundle
  memberships. Take-up ramps linearly from launch in month 3 to month 24. *Unsourced.* Jio and
  Airtel already sell such packs (E8, E9), but neither publishes take-up. Probe: the operators'
  current active counts on the E8/E9 packs, which they can share under NDA in the first month.
- **A3** — **50% of Option 1 bundle memberships are incremental.** The other half are existing
  direct Netflix payers who move to the bundle, or people who would have joined at retail anyway.
  *Unsourced.* The high-spend cohort overlaps Netflix's affluent existing base (E6), so this share
  could be lower. Probe: at activation, the share of bundle accounts that match an existing or
  recently lapsed Netflix account, plus a 90-day matched holdout of non-offered subscribers.
- **A4** — **Option 1 wholesale price = 60% of Mobile retail ≈ ₹89/month** per activated
  membership (60% × ₹149, E7), whatever the tier. This is our opening position, not a
  disclosed rate. Break-even incrementality at this price is 1 − 60% = **40%** (see the revenue
  identity above).
- **A5** — Option 2's offer **reaches 50% of the big-three base outside the A1 cohort**:
  50% × (1,103M − 110M) ≈ **496M** prepaid subscribers, through the operator app, the recharge flow
  and SMS. The cohort excludes A1 by design, so Options 1 and 2 do not compete for the same user.
  *Unsourced.* The 50% stands for smartphone users who open the operator app or recharge flow.
  Probe: monthly active users of each operator's app.
- **A6** — **0.6% of the A5 reach, ≈3.0M people**, buy a carrier-billed Netflix membership by month 24.
  Sign-ups ramp linearly from launch in month 3. **60% of them, ≈1.8M, are still active at month 24.**
  The other 40% lapse because their prepaid balance is short at renewal. *Unsourced.* Retail Mobile at
  ₹149 (E7) is ≈69% of Jio's monthly ARPU of ₹215.6 (E1), so price sensitivity is high. Probe:
  conversion per 1,000 offer impressions in an 8-week pilot on one operator.
- **A7** — **60% of Option 2's active memberships are incremental, ≈1.08M.** They are users without a
  card or UPI mandate set up for Netflix, or users who only act when prompted in the recharge flow.
  The other 40% are existing payers moving their payment method, or users who would have joined by UPI.
  *Unsourced.* Probe: the share of carrier-billed accounts that match an existing Netflix account.
- **A8** — Option 2 commercials: the operator keeps **15% of carrier-billed revenue** as the
  payment-rail fee and earns a **₹300 bounty per paid sign-up** that survives its first renewal. Netflix
  therefore nets ≈85% × ₹149 ≈ ₹127/month per carrier-billed membership (E7). This is our opening
  position. Direct-carrier-billing fees in India are not published.
- **A9** — Option 3 attaches a bundle to **3M 5G handsets a year** sold through operator retail and
  OEM partners. Sales start in **month 7**, after a 6-month device-certification and firmware cycle.
  That gives ≈4.5M bundle devices by month 24. *Unsourced.* No OEM has published such a volume. 3M
  handsets a year is about 1% of Jio's 5G base (E1). Probe: signed OEM letters of intent with committed
  volumes before month 3.
- **A10** — The OEM pays Netflix a wholesale **12-month Mobile term at 40% of retail ≈ ₹60/month,
  ≈₹715 per term** (40% × ₹149 × 12, E7), upfront and non-refundable. The OEM funds the price out of
  its 5G-upgrade marketing budget. That costs the OEM ≈₹215 crore a year at A9 volume. *Unsourced.*
  This is our opening position.
- **A11** — **30% of expired Option 3 terms convert to paid retail**, and **70% of Option 3 memberships
  are incremental**. Buyers upgrading to 5G handsets are mostly not current Netflix payers. *Unsourced.*
  Probe: the renewal rate of the first term cohort. An earlier proxy comes from engagement in term
  months 1–3 and from the share of users who set up a payment mandate at the month-11 renewal prompt.
- **A12** — Integration timelines: **12 weeks** to extend an existing operator bundle integration to
  new plan SKUs (Jio and Airtel already run one, E8, E9); **6 months** for an operator without one (Vi)
  and for a new device-preload program; **12 weeks** per operator for carrier billing. These are
  *our estimates*, not Netflix disclosures.
- **A13** — Term-sheet minimum commitments, term lengths and review points in the options are
  **opening negotiation positions** set by this plan. They are not forecasts, and none is sourced.

## Evidence

- **E1** — Reliance Jio, Q1 FY27 (April–June 2026): **533.3M subscribers** after 8.9M net adds, ARPU
  **₹215.6/month** (up from ₹208.8 a year earlier), monthly churn **1.6%**, 5G base about **285M**, and
  JioAirFiber above 14M. Source: TelecomTalk,
  https://telecomtalk.info/jio-adds-million-subscribers-q1fy27-arpu-rs215/1009790/
- **E2** — Bharti Airtel, Q1 FY27: India mobile ARPU **₹264** (₹250 a year earlier), **376.5M** India
  mobile subscribers and **30M** postpaid customers, after its highest-ever quarterly postpaid adds.
  Sources: TelecomTalk,
  https://telecomtalk.info/bhartiairtel-arpu-rs264-postpaid-base-30million-q1fy27/1010410/ ;
  Business Standard,
  https://www.business-standard.com/companies/quarterly-results/bharti-airtel-q1-results-net-profit-rises-37-3-on-subscriber-upgrades-126080401181_1.html
- **E3** — Vodafone Idea (Vi), Q1 FY27: **193.1M** subscribers (its first quarter of net adds since the
  merger), customer ARPU **₹195** excluding M2M, and **31.9M** postpaid subscribers. Source: MediaNama,
  https://www.medianama.com/2026/08/223-vodafone-idea-193-1-million-arpu-10-2/
- **E4** — TRAI subscription data: India's wireless subscriptions (mobile plus fixed wireless) rose
  from 1,300.25M at the end of June 2026 to **1,306.27M** at the end of July 2026. Source: TelecomTalk
  reporting TRAI data,
  https://telecomtalk.info/india-adds-6million-wireless-subscribers-july2026-trai/1011300/
- **E5** — About **10%** of India's roughly 1.3bn mobile subscribers are postpaid, up from 8–9% in
  earlier years, per TRAI data. Over 90% of the base is therefore prepaid. Source: Business Standard
  (July 2026),
  https://www.business-standard.com/amp/industry/news/arpu-convergence-belies-a-persistent-prepaid-postpaid-tariff-divergence-126071500856_1.html
- **E6** — Netflix had **more than 16M** subscribers in India as of January 2026, per Media Partners
  Asia. That was about **6%** of India's 272M OTT subscriptions in 2025, and about **10%** of the streaming
  video market by value. The Media Partners Asia estimate for mid-2024 was 12M. Sources: Business Standard,
  https://www.business-standard.com/industry/news/netflix-s-india-decade-from-hbo-moment-to-a-hunt-for-mass-reach-126011000017_1.html ;
  Business Standard,
  https://www.business-standard.com/companies/news/india-hooked-to-netflix-s-content-subscribers-and-revenue-surge-in-q2-cy24-124071900253_1.html
  (both taken from search excerpts, because the pages refused a direct fetch).
- **E7** — Netflix India retail prices per month: **Mobile ₹149, Basic ₹199, Standard ₹499, Premium
  ₹649**. Source: Digit, https://www.digit.in/digit-binge/ott/netflix/
- **E8** — Jio and Netflix, August 2023: prepaid packs at **₹1,099** (Netflix Mobile, 2GB/day) and
  **₹1,499** (Netflix Basic, 3GB/day), both valid 84 days. This was the first time Netflix partnered
  with an Indian operator on a prepaid plan. Source: TechCrunch,
  https://techcrunch.com/2023/08/18/netflix-inks-deal-with-reliance-jio-to-expand-india-presence/
- **E9** — Airtel sells a **₹1,499** prepaid pack that includes Netflix Basic, valid 84 days, with
  3GB/day. Source: NewsBytes,
  https://www.newsbytesapp.com/news/business/airtel-jio-prepaid-plans-with-free-netflix-subscription/story
- **E10** — TRAI's *Prohibition of Discriminatory Tariffs for Data Services Regulations, 2016*
  (8 February 2016) bars any service provider from offering or charging discriminatory tariffs for data
  services on the basis of content. That outlaws zero-rating. Only emergency-service tariffs are exempt,
  and they must be reported to TRAI within seven days. The penalty is **₹50,000 a day, up to ₹50 lakh**.
  Sources: TRAI, https://trai.gov.in/sites/default/files/2024-09/Regulation_08022016_0.pdf ;
  Telecompaper, https://www.telecompaper.com/news/india-adopts-ban-on-zero-rating-internet-services--1126923
- **E11** — Netflix Open Connect: embedded Open Connect Appliances "are provided to qualifying ISP
  partners at no charge". The ISP supplies rack space, power and connectivity, and each deployment
  offloads Netflix traffic from peering and transport circuits. Source: Netflix Open Connect,
  https://openconnect.netflix.com/en/
