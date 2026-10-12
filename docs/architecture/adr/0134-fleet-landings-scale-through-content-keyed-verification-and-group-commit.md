# ADR-0134: Fleet landings scale through content-keyed verification, independent order and group commit

- **Status:** Proposed (2026-10-13). Each decision moves to Accepted when its component leaves shadow ([plan](../../plans/fleet-landing-at-scale-2026-10.md) §8).
- **Plan:** [fleet-landing-at-scale-2026-10.md](../../plans/fleet-landing-at-scale-2026-10.md). **Research:** [fleet-landing-at-scale-2026-10.md](../../research/fleet-landing-at-scale-2026-10.md). **Design:** [fleet-landing-queue.md](../fleet-landing-queue.md), the spec of record, which the amended rules join when they land.
- **Amends:** [ADR-0128](0128-landing-queue-tiered-reverification.md). D3 (strict FIFO) holds only between candidates with an edge. D10 (a review only at the head) becomes a review bound to the evidence digest. The head checks keys, and gates run before the head.
- **Relates to:**
  - [ADR-0039](0039-failure-floor-and-failure-signal-contract.md) §8.1: each owner keeps its own two-phase landing intent inside a group commit;
  - [ADR-0072](0072-system-failure-policy-and-halt.md): a red `main` stays a system failure, so optimistic landing is out;
  - [ADR-0127](0127-push-only-event-channels.md): waiters wake on `ship.landed` and queue events.
- **Evidence:**
  - Ship scans before its fast-forward check, and each scan runs `-count=1` (research F2.1, F2.2).
  - `cmd/evolve` took 298 s of the 354 s scan of cycle 1865, and it imports 84% of the module (F2.3, F2.6).
  - A divergence reruns the whole scan (F2.7).
  - At N = 200, today's path costs about 20,100 scans for one wave. Any serial step that holds a scan caps the queue far below the load (F3.2, F3.3).
  - At N ≥ 20 almost every landing overlaps a peer through the import graph (F3.4, F2.15).

## Context

A fleet lane ships by a fast-forward of its cycle branch. Each peer landing makes the other lanes diverge, and each divergence reruns a scan of about four minutes. ADR-0128 designs a queue with tiers, but its head still runs the gates and the scan, one candidate at a time. The operator asked on 2026-10-13 for the fastest way to handle conflicts and rebases with 200 concurrent lanes.

## Decision

1. Ship checks the fast-forward before the repo-contract scan.
2. A package test run has a content key: its files, its read roots, and the variables in its test log.
3. A green result is stored by its key. A hit with an equal key is a proof for that tree.
4. An unknown read, or a read outside the module in the test log, keys on the whole tree.
5. `cmd/evolve` runs in process shards now. Its tests move to their packages under a ratchet.
6. A candidate verifies on its predicted prefix before it is the head.
7. The head recomputes its keys on the real tip and runs only the runs whose key changed.
8. Key-disjoint candidates land in any order. Other candidates keep ticket order.
9. A T3 review binds the evidence digest and can run before the head.
10. The holder of `ship.lock` lands the verified run of the chain with one push.
11. Each landed commit point has its own verification record, and ship acceptance checks it.
12. Each owner finishes its own ledger and inbox work on `ship.landed`.
13. Dispatch admits todos by cell; the cell of the hub packages admits one lane.
14. A lane refreshes its base at the start of the audit when the composition is T1 or T2.
15. There is one queue for each project and branch, and one verification pool for each host.
16. Each part runs in shadow before enforce, and a seeded simulation at N = 2, 20 and 200 comes first.

### Why the content key needs the test log

The data-edge catalog of ADR-0128 holds test reads only ([spec](../fleet-landing-queue.md) §7.4). Tests also read `.evolve/*` through production loaders, and they read environment variables. The test log of each run records both, so the key holds them, and an outside read keys on the whole tree. The reads of a child process stay a limit, and the shadow escape count gates the enforce stage.

### Why an order change is safe here

ADR-0128 D3 kept strict FIFO, because an order change needs every path of the speculation tree verified. Key-disjoint candidates change no key input of each other's runs. Both orders thus give equal keys, so one verification covers both paths. An ejection changes no key of a key-disjoint peer, and only its own cell composes again. The head recomputes each key on the real tip, so a wrong claim of disjointness causes a run, not an untested landing.

## Consequences

- **Faster rebases.** A rebase reruns only the package runs whose inputs changed. The plan targets 60 s at the median at N = 2, against 113 to 381 s today.
- **No serial scan.** The lock holds only a key check and one push for a group, so throughput follows the verification workers.
- **Integrity kept.** Every commit point on `main` has a record for its exact tree. No tree lands untested, and the audit binds `T0` as before.
- **Cost:** two new packages (`internal/testkey`, `internal/testverdict`), a simulator, and changes to protected ship and queue code.
- **Risk:** a key that misses a read gives a false hit. The control is the whole-tree key for unknown reads, the shadow escape count, and CI on `main`.
- **Accepted limit:** at most about 139 Go lanes can be pairwise disjoint in this module. Lanes beyond that wait at dispatch or work outside `go/`.

## Rejected

| Option | The fact that rules it out |
|---|---|
| ADR-0128 as designed, with a faster scan only | Its serial head caps the queue at about 33 landings in one hour; the load at N = 200 is about 204 (research F3.2). |
| Batches that test only the top tree, with bisection | The commits under the top stay untested (research P6.2). |
| Optimistic landing with culprit finding after the landing | A red reaches `main`, which ADR-0072 treats as a system failure. |
| Avoidance only | Every landing still makes the other lanes diverge, and the serial scan stays (research F3.2). |
| The Go test cache as the landing cache | It misses reads outside the module (research F2.18, P7.3). |
| `git rerere` on the landing path | A recorded resolution has no audit verdict (research P8.3). |
