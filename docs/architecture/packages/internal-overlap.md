# internal/overlap

> The spec of record: [fleet-landing-queue.md](../fleet-landing-queue.md) §6 (the overlap proof), §7 (the catalogs) and §8 (the tiers). The plan (component Q3, decisions D5, D6, D7, D36 and D37): [concurrent-cycle-landing-2026-10.md](../../plans/concurrent-cycle-landing-2026-10.md). The decision: [ADR-0128](../adr/0128-landing-queue-tiered-reverification.md).

## Purpose

`internal/overlap` is the overlap proof of the landing queue. It has two parts:

- `Prove`, a pure function. It takes the lane change `L`, the peer delta `P`, the composition facts, the module map at the composed tree `C` and the catalogs. It gives the tier, the rules that fired, the evidence, the evidence digest and the selection inputs (`pkgs(L)` and `pkgs(P)`).
- `LoadModule`, the adapter for `go list`. It reads the module map at `C`: the file lists and the module-internal dependencies of each package, as the union of the four tag sets.

Nothing in production calls the package yet. Components Q13 and Q14 wire it.

The package is protected surface (`guards.ProtectedSurfaceManifest`, plan D50). Its tier decides if an audit runs again, and it calls `explanationdocs.IsPlainPath`.

## Design

- **A pure decision and a separate adapter.** `Prove` reads no file and starts no process, so each rule has a test without a toolchain. `LoadModule` is the only part that runs a command. It takes a `sysexec.RunFunc`, and a real module under `testdata/repo` tests it.
- **The steps of §8.**
  1. Each precondition rule gives T4: a genuine conflict, a base that is not an ancestor, or a missing audited tree. The proof stops, with no evidence.
  2. A peer delta that holds only bookkeeping paths gives T1 by `bookkeeping_peer`. An empty peer delta gives T1 by its own rule, `empty_peer`.
  3. `CompileRed` gives T4 by `compile`, before the other rules of step 3. Then these rules can fire from the evidence: `shared_path`, `package_edge`, `build_zone`, `unknown` and `derived`. The tier is the strictest tier of the rules that fired. `stricter` compares the tiers by a rank, not by their spelling. No rule gives T1 by `disjoint`.
- **A plain path.** `explanationdocs.IsPlainPath` accepts an ASCII path with no `:`, `\` or `~`. No component ends in a dot or a space, so `.` and `..` are refused. Each component stays under the name limit of a filesystem. Such a path resolves to itself alone on every supported filesystem. Windows device names are out of scope: they redirect input and output and do not alias another path, so the digest check refuses them.
- **The zones.** Each path gets one zone. A path that `explanationdocs.IsPlainPath` refuses is unknown. For the other paths, the order is bookkeeping, the build zone, the gate zone, a derived output, a path under `go/` and prose. Each other path is unknown.
- **The catalogs are an input.** The `Catalogs` interface gives bookkeeping (§7.2), the build zone and the gate zone (§7.3). It also gives the derived outputs and the entries that fired (§7.1), and the test data reads (§7.4). The homes of these catalogs are other components (Q1, Q2 and Q5). The wiring gives the real catalogs.
- **The read roots are code.** The spec puts them in the proof package: `docs/conventions/ste100-writing.md`, `docs/research/` and `docs/reference/`. Markdown under `docs/` or `solutions/` outside them is prose.
- **A derived output is per side.** `DerivedOutput(side, path)` lets the catalog say that a diff lies inside the generated region on one side only. A path in both `L` and `P` leaves `shared_paths` only when it is a derived output on both sides.
- **Package ownership.** A path under `go/` belongs to each package whose file lists name it, or whose directory holds it in a `testdata/` tree. A deleted Go file (`DeletedAtC`) counts by its directory. Each other deleted path under `go/` is unknown. When the module path is not known, a deleted path is unknown.
- **The closure.** `closure(s)` is `s` and its module-internal `Deps` at `C`. A package that is not in the map adds only itself. The edges are pairs `Edge{From, To}` in both directions (O11).
- **The evidence digest.** It is the SHA-256 of the JSON of the evidence and of the blob ids (`Input.Blobs`) of each evidence path. Every list is sorted and has no duplicates, so the digest does not depend on the input order.
- **The blobs that the digest binds.** The paths of `shared_paths`, `build_zone`, `gate_zone`, `data_edges` and `unknown` are bound. Each peer path under `go/` that a package of an edge owns is bound too (`edgePeerPaths`, both directions). Thus a new peer change behind an edge changes the digest (plan D52). Each composition regenerates a derived output, and the audit binds the lane paths.
- **Renames.** `L` and `P` come from `git diff --no-renames`, so a rename is two paths: the old path and the new path. Each caller must give both paths. `DeletedAtC` holds the old path.
- **The adapter.** `LoadModule` runs `go list -json ./...` in `<repo>/go` once for each tag set: none, `integration`, `acs`, and `e2e,evolve_test_phases`. The Deps of `-json` give the same graph as the `-f` template of §6, with one call for each tag set. A failure of a call returns an error. The caller adds its text to `Input.Failures`, so the proof is T3 or stricter.

## Invariants

- **Unknown is never T1** when the proof reaches step 3. `TestProof_UnknownIsNeverT1` is a `rapid` property over unknown paths on each side and over input failures.
- **The digest does not depend on the input order.** `TestEvidenceDigest_DoesNotDependOnInputOrder` is a `rapid` property over the order of the paths, the failures, the packages, the files and the dependencies. `TestEvidenceDigest_BindsTheBlobsOfEachEvidencePath` shows that the digest changes with the blob of an evidence path, and only with such a blob.
- **Bookkeeping never raises the tier.** `TestProof_BookkeepingNeverRaisesTheTier` and `TestProof_ABookkeepingOnlyPeerIsT1BeforeStepThree`.
- **Edges count in both directions.** `TestProof_APeerChangeInTheLaneClosureIsT3` and `TestProof_ALaneChangeInThePeerClosureIsT3`. A shared leaf import is not an edge (`TestProof_ASharedLeafImportIsT1`).
- **The graph is the union of the four tag sets.** `TestGraph_IsTheUnionOfTheFourTagSets` uses a fixture package with a file for each tag set. A file with `e2e && !evolve_test_phases` adds no dependency.

## Findings

- **A deleted file needs a fact that `L` does not hold.** `git diff --name-only` gives no status. The proof thus takes `DeletedAtC`, the paths of `L ∪ P` that `C` does not hold.
- **The import path of a deleted directory is the module path and the directory.** In one module, a package directory always gives this import path, so the proof needs no lookup in the package map.
