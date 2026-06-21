// Package mergegate is the merge-to-main gate's deterministic kernel: the
// cadence advisor that decides WHEN a completed campaign milestone is promoted to
// main, and the promoter that performs that promotion under the hardened ship
// path.
//
// WHAT it does. Two pure-ish, injectable units:
//   - DecideCadence (cadence.go) — the advisor. Given objective milestone progress
//     (campaign wave completion) plus quality signals (audit/CI/ledger/severity)
//     and the resolved policy thresholds, it decides whether to fire the gate now
//     and at what cadence (per-wave | batched | feature-complete | defer). PURE.
//   - Promoter (promoter.go) — the kernel actuator. On an enforce-stage PASS it
//     promotes the milestone's integration branch to main via the existing
//     swarm merge-train + ship path; at off/shadow/advisory it records the
//     would-be promotion and does nothing. Humble Object over injected git seams.
//
// HOW it stays safe. The advisor is DEFER-WINS: any single safety violation forces
// defer regardless of progress, so the LLM gate phase that runs alongside it can
// only ever be MORE conservative ("model proposes, kernel disposes"). The promoter
// never invents a merge path — it drives swarm.RunMergeTrain (acceptance-gated,
// conflict-abort-clean) so the integrity floor, ship.lock, attestation, and
// auto-rollback all apply unchanged.
//
// WHY it is a leaf. mergegate imports only the stdlib (and swarm seams in the
// promoter): the cadence decision is the single most safety-critical judgment in
// the system, so it is isolated, dependency-light, and exhaustively unit-testable
// without a repo, a campaign, or git. The composition root maps policy +
// router signals into the input structs and injects the git seams.
package mergegate
