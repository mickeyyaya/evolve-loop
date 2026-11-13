# I14 PR (c) enforce actor — assumptions and evidence

Every claim in the options and the recommendation cites an entry here: `A<n>` for an assumption, `E<n>` for evidence. Checked against base `722ecf0d2` on 2026-11-14. No number here is a measured cost. No option has a measured edit time.

## Assumptions

- **A1** — The EnforceActor must land a pool by fast-forward and must not bypass the driver. Source: inbox acceptance 1 (operator-stated).
- **A2** — A delivered item must stay off the menu until it is merged or ejected. Source: inbox acceptance 2 (operator-stated).
- **A3** — Resume of a lane checkpoint must not reach `main` at enforce. Source: inbox acceptance 3 (operator-stated).
- **A4** — The item asks for a design card and a review before any code. This cycle is a document cycle, and the deterministic floor is `evolve solution check`. Source: scout report, task `i14-enforce-actor-design-card`.
- **A5** — Driver code is a protected surface for this lane. Its change belongs to the cycle after the card. Source: the eval grader that denies a diff in `go/internal/integration`, `go/internal/phases/ship` and `cmd_loop_wavesync.go`.

## Evidence

- **E1** — Section 9 of the parent spec lists seven open inputs: F2, F3, F11, F12 and Findings 3, 4, 5. Citation: `docs/architecture/integration-driver.md` §9.
- **E2** — No `EnforceActor` exists at base. The stage table gives enforce no actor, so the driver refuses to start at enforce. Citation: `docs/architecture/integration-driver.md` §3 and §7 "Gate order".
- **E3** — Post-ship calls `isLanded` at a delivered ship and releases the item when it is false. Citation: `go/internal/phases/ship/postship.go:153`, `:196`.
- **E4** — `Merger.Land` returns an `Outcome` with `Landed`, `Ejected`, `Flaked` and `Inbox`. It takes `CI` and `Main` ports. Citation: `go/internal/integration/batchpr.go:25-57`.
- **E5** — `Ejection.WriteBrief` and `ReadBrief` exist, so a brief file has a home. Citation: `go/internal/integration/brief.go:63`, `:81`.
- **E6** — `evolve loop` has a `Resume` option on the loop config. The lane identity is not part of it. Citation: `go/cmd/evolve/cmd_loop.go:40`. The reading that a lane checkpoint resumes as a non-lane is from §9 Finding 5 of the parent spec, not re-run here.
- **E7** — The parent spec holds that the shadow scope was built from its design with no further card round. It ran three review rounds on the card. Citation: `docs/architecture/integration-driver.md` §10.
- **E8** — Resume leases for paused cycles exist since cycle 2353. They do not cover the landing path. Citation: `docs/explain/builds/cycle-2353-01m7aj10tnja6akzk7r2ambk4z.md`.
