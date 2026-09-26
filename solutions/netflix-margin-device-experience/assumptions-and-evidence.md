# Assumptions and Evidence — Netflix operating margin +1pp via the device experience

Every number in `options/*.md` and `recommendation.md` cites an entry below. `E<n>` entries are
sourced facts (public filings, Netflix's own engineering publications, third-party panel data,
peer-reviewed research). `A<n>` entries are **assumptions**: numbers we could not source publicly,
stated with the reasoning behind them and the probe that would replace them with a measurement.
Netflix does not disclose delivery cost, partner-payment totals, device mix, or churn by device.
Every option therefore rests on at least one assumption, and the recommendation takes that into account.

## Baseline arithmetic (derived only from the entries below)

- FY2025 operating income ≈ 29.5% × $45.2B ≈ **$13.3B** (E1).
- **1 percentage point of operating margin ≈ $452M of operating income** at FY2025 revenue (E1),
  or ≈ $512M at the FY2026 guidance midpoint of $51.2B (E2).
- For a **cost** lever, Δmargin ≈ savings ÷ revenue.
- For a **revenue** lever, Δmargin = (OI + ΔR·c) ÷ (R + ΔR) − OI ÷ R ≈ ΔR·(c − 0.295) ÷ R, where
  c is the contribution margin on incremental revenue (A10).
- **Timing basis (one for all options).** Every option starts in month 0. Each option's lag and
  ramp is a labelled assumption: A3 for Option 1, A7 and A14 for Option 2, and A13 for Option 3. A
  "year N" effect is the annualized run-rate at the end of month 12·N, at FY2025 revenue (E1).

## Assumptions

- **A1** — Streaming delivery cost is **≈2% of revenue, ≈$0.9B/yr**. This covers Open Connect
  appliance depreciation, transit and peering, and delivery operations. *Unsourced.* Netflix does
  not break it out. The upper bound is E4: FY2024 "other cost of revenues" was $5.74B, and that line
  also contains payment processing, customer service and content-related costs. The figure is
  low relative to that line because embedded appliances sit inside ISPs, which supply power, space and
  connectivity free of charge (E6). Probe: internal cost-center ledger for Open Connect.
- **A2** — **50%** of delivery cost scales with bits delivered within 24 months, through deferred
  appliance purchases and lower transit at peak. The rest is fixed operations. *Unsourced estimate.*
- **A3** — AV1 share of streaming can be raised from ~30% (E5) to **~60%** of streamed hours
  within 24 months. The levers are AV1 decode in partner-device certification requirements, AV1 as
  the default on every capable TV, and film-grain synthesis rollout. The ramp is assumed **linear
  over months 1–24** (≈45% at month 12), because it is paced by TV fleet turnover. *Operator-stated
  target, not a Netflix disclosure; the linear shape is our estimate.*
- **A4** — Hours moved from AVC/HEVC to AV1 save the same **one-third** of bits that Netflix
  measured on existing AV1 sessions (E5). This assumes the newly migrated hours look like today's
  AV1 hours. That is optimistic, because late-migrating devices skew older and lower resolution.
- **A5** — Payments to marketing partners (CE manufacturers, MVPDs, mobile operators, ISPs; E7) are
  **≈60% of non-advertising marketing expense ≈ $0.7B/yr**. Non-advertising marketing was $1.14B in
  FY2024 (E8). The remaining ≈40% is marketing payroll and other items. *Unsourced split.*
  Probe: the internal marketing ledger, which gives the partner-payment total in one quarter.
- **A6** — Repricing partner deals from fixed and placement-based fees to **pay-per-verified-retained-
  member** cuts partner payments by **25%** at renewal, after dropping placements with no measured
  incremental lift. *Unsourced estimate.* Probe: a 90-day holdout-based incrementality test on
  one partner's Netflix-button or pre-install cohort. It measures placement lift. It does not
  show how partners react to the new rate card.
- **A7** — **50%** of partner-payment contract value comes up for renewal within 12 months, and all of it
  within 36 months. *Unsourced.* Typical multi-year CE and operator deal length is assumed.
- **A8** — **25%** of paid memberships use a low-end or legacy TV device as their primary device:
  older smart-TV platforms and low-memory streaming sticks, where startup time, rebuffering and app
  crashes are materially worse than on the fleet median. *Unsourced.* Probe: device-class telemetry.
- **A9** — Fixing the playback experience on that cohort cuts the cohort's monthly gross churn by
  **0.15pp**, from 1.8% (E9) to 1.65%, about 8% relative. The fix set is a lighter app runtime,
  per-device bitrate ladders, startup prefetch, crash fixes, and AV1 or lower-bitrate ladders where
  decode allows. Blended over the whole base this is **0.0375pp/month**. *Unsourced.* The direction
  is supported by E3, E5 and E10. The magnitude is the single largest unknown in this document.
  Probe: 90-day A/B holdout on the device cohort.
- **A10** — Contribution margin on retained or incremental subscription revenue is **c = 85%**.
  Content amortization is fixed in the short run. The variable costs are payment processing,
  delivery and customer service. *Unsourced estimate.* This is consistent with content amortization
  being the dominant cost line in E4.
- **A11** — Revenue per retained member ≈ **$12/month (≈$144/yr)**. This is derived from E1 and E3:
  $45.2B ÷ the average of 302M and 325M members ÷ 12 ≈ $12.0. The FY2024 reported average monthly
  revenue per paying membership was $11.70 (E3).
- **A12** — Retained members who would otherwise have churned then leave at the base gross-churn rate
  of 1.8%/month (E9). This is conservative: it ignores that they are now on a fixed device experience.
- **A13** — Option 3's fix set takes **6 months** to build and to ship to the whole low-end cohort.
  The lighter runtime and the crash fixes need a client release on each legacy platform. The churn
  cut (A9) starts in **month 7**, and nothing is credited before then. *Unsourced estimate.* The
  recommendation still holds up to a build of ≈17 months (see its flip section). Probe: the
  device-platform team's delivery plan in month 1; the A9 holdout then reads out by month 9.
- **A14** — Option 2 needs a **3-month** attribution build (the A6 holdout) before the first
  repricing. Renewal dates are spread evenly within A7's windows. The contracts that renew in
  months 1–3 (12.5% of contract value) therefore renew on current terms, and are next repriced
  after month 36. Repriced share of contract value is 37.5%, 62.5%, 87.5% and 100% at the end of
  months 12, 24, 36 and 48. Each further quarter of delay forfeits another 12.5% until month 36.
  *Unsourced.* Probe: the partner-contract renewal calendar.

## Evidence

- **E1** — FY2025: revenue **$45.2B** (+16% y/y), operating margin **29.5%** (+3 points), ad revenue
  above $1.5B. Source: Netflix Q4 2025 shareholder letter (Jan 2026),
  https://s22.q4cdn.com/959853165/files/doc_financials/2025/q4/FINAL-Q4-25-Shareholder-Letter.pdf ;
  SEC exhibit https://www.sec.gov/Archives/edgar/data/1065280/000106528026000033/ex991_q425.htm
- **E2** — FY2026 guidance: revenue **$50.7B–$51.7B**, operating margin target **31.5%** (at 1/1/26
  F/X). Source: same Q4 2025 shareholder letter as E1.
- **E3** — Paid memberships: **301.6M** at end of 2024 (FY2024 10-K), and **325M+** crossed in Q4 2025
  (E1 letter). FY2024 average monthly revenue per paying membership: **$11.70**. Source: Netflix FY2024
  Form 10-K, https://www.sec.gov/Archives/edgar/data/1065280/000106528025000044/nflx-20241231.htm
- **E4** — FY2024 cost structure: revenue $39.00B; operating income $10.42B; content amortization
  **$15.30B**; other cost of revenues **$5.74B**; sales and marketing **$2.918B**; technology and
  development $2.925B. Source: FY2024 Form 10-K (E3 link).
- **E5** — AV1 powers **~30%** of Netflix streaming and is its second most-used codec. AV1 sessions use
  **one-third less bandwidth** than AVC and HEVC and have **45% fewer buffering interruptions**. AV1
  with film-grain synthesis cuts bitrate by **66%** on the grainy titles tested. Source: Netflix
  TechBlog, "AV1 — Now Powering 30% of Netflix Streaming" (Dec 2025),
  https://netflixtechblog.com/av1-now-powering-30-of-netflix-streaming-02f592242d80
- **E6** — Embedded Open Connect Appliances are provided to qualifying ISPs **at no charge**. Netflix
  supplies the hardware and the ISP supplies power, space and connectivity. Netflix therefore bears
  appliance capital cost, not per-bit transit, for embedded traffic. Source: Netflix Open Connect
  overview, https://openconnect.netflix.com/en/ and
  https://openconnect.netflix.com/Open-Connect-Overview.pdf
- **E7** — "Marketing expenses consist primarily of advertising expenses and certain payments made to
  our marketing partners, including consumer electronics ('CE') manufacturers, multichannel video
  programming distributors ('MVPDs'), mobile operators and ISPs." Source: Netflix Form 10-K FY2022
  (the same wording appears FY2019–FY2023),
  https://www.sec.gov/Archives/edgar/data/1065280/000106528023000035/nflx-20221231.htm
- **E8** — Advertising expenses were **$1,779M** (2024), $1,732M (2023) and $1,586M (2022). With E4's
  $2,918M sales and marketing, non-advertising marketing was **≈$1.14B** in FY2024. Source: Netflix
  FY2024 annual report (Form ARS),
  https://www.sec.gov/Archives/edgar/data/1065280/000119312525084431/d914423dars.pdf
- **E9** — Netflix had the lowest churn among US premium SVOD services: **1.8% gross** and **1.0% net**
  monthly churn as of Sept 2024 (Antenna estimates). The premium-SVOD weighted average was 5.3% gross.
  Source: Antenna, https://x.com/AntennaData/status/1867594135705624621 ; context:
  https://www.antenna.live/insights/antennas-2024-top-subscription-insights-net-churn
- **E10** — This study used 23M views and 6.7M viewers on Akamai and applied quasi-experimental causal
  designs. Viewers begin abandoning when startup exceeds **2s**. Each extra second adds **5.8%** to
  abandonment. A rebuffer delay equal to 1% of the video's duration reduces play time by **5%**.
  Source: Krishnan & Sitaraman, "Video Stream Quality Impacts Viewer Behavior", ACM IMC 2012,
  https://people.cs.umass.edu/~ramesh/Site/HOME_files/imc208-krishnan.pdf
