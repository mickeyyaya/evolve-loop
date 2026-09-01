# ADR-0094 — Runtime corpora stay tracked upstream; each removal has a named prerequisite

- **Status:** Accepted (2026-08-27) — removal **deferred, not declined**. This ADR records
  what blocks each removal so the debt is queued rather than rediscovered by the next sweep.
- **Driving incident:** the 2026-08-27 repo-hygiene sweep. 20 runtime artifacts (~655 KB,
  oldest from cycle-104) were found tracked on `main` and removed; the sweep then had to
  decide what to do about four *much larger* bodies of tracked content that are also
  neither code nor documentation — together **2,922 files and 27.5 MB: 41% of the tracked
  tree by file count**. Three of the four turned out to be load-bearing, and a removal
  driven by appearances would have been a product regression.
  (Sizes throughout are `git ls-tree -l` blob sizes against `origin/main`, not `du` —
  `du` reports ~6 MB for `knowledge-base/` because 1,767 tiny files pad out to 4 KB blocks.)
- **Landed with:** `.gitignore` class rules for the leaked-artifact classes, and
  `go/internal/phasecoherence/tracked_artifact_hygiene_test.go`, which fails the ship-time
  repo-contract pack if any of those classes is tracked again.
- **Related:** [ADR-0055](0055-cycle-dossier.md) — the cycle dossier itself, and the
  `dossier-closeout` floor gate that makes `knowledge-base/cycles/` load-bearing.
- **Related:** `go/internal/phasecoherence/gitignore_birth_test.go` — the sibling contract
  (a tracked corpus must admit new births). This ADR's guard is its mirror image: a
  runtime artifact must **not** be tracked at all.

## Problem

evolve-loop's automation writes its records into the working tree, and ships by staging
broadly. The tracked tree has therefore accreted content in three tiers, which look alike
from the outside and are not alike at all:

1. **Leaked artifacts** — no design intent, swept in by `git add -A`. Removed 2026-08-27.
2. **Protocol-committed corpora** — deliberately `git add`ed by production code, and read
   back by a gate. Look like tier 1. Are not.
3. **Accreted source** — real compiling code produced per cycle.

The operative question for any cleanup is not "is this code or docs?" but **"does anything
read it, and from where?"**. `knowledge-base/README.md` opens with *"This directory is a
runtime write surface, not part of the documentation tree"* — and yet removing it fails
the cycle. A sweep that trusts the README breaks the pipeline.

## Decision

The four bodies below **stay tracked**. Each entry names the mechanism that requires it and
the prerequisite work for removal. None may be untracked as a hygiene change; each needs
its own TDD + audit cycle.

### 1. `go/evolve` — 20.1 MB compiled binary — *keep*

**Not** a leak. It is the **distribution mechanism** for plugin users: `skills/loop/SKILL.md:24`
resolves the binary from `*/marketplaces/evolve-loop/go/evolve` and
`*/cache/evolve-loop/evolve-loop/*/go/evolve`. The Claude Code plugin marketplace **clones
the repo with no build step**, so untracking it breaks `/evo:loop` for every user without a
Go toolchain. It is further wired into `releasepipeline.go` (`resolveEvolveBin` fallback,
`defaultReleaseVerify`), `ship/binary_staging_guard.go` (the >1 MB staged-executable
allowlist, beside `go/bin/**`), `core/leak_recovery.go:269` (`buildArtifacts` — a mid-cycle
rebuild is discarded, never relocated: cycle-153 binary drift), and
`core/cyclerun_review.go:386`.

`CHANGELOG:558` already lists "go/evolve untracking" as a follow-up. That follow-up is a
**feature, not a cleanup**.

> **Prerequisite:** a bootstrap path that fetches the platform binary from GitHub Releases
> on first plugin use (the logic `install.sh` already implements for the CLI install), so
> the marketplace clone no longer has to carry it. Then, in one change: drop the resolver
> arms, the release-pipeline fallback, and the staging allowlist entry.

### 2. `knowledge-base/cycles/` — 1,767 files / 1.9 MB — *keep*

Per-cycle closeout dossiers. Despite its README, this is a **protocol-committed corpus**:

- `go/internal/dossier/write.go` `Write(d, dir, commit=true)` → `commitPairGit` **git-adds
  and commits** `cycle-N.{json,md}` under the shared git-mutation lock, so the pair lands
  as "the ONE committed artifact" and the next phase's tree-diff guard does not see an
  untracked pair.
- `go/cmd/evolve/cmd_dossier.go:47` — `evolve dossier verify` **FAILs** when
  `.evolve/policy.json` `floor` enrolls `dossier-closeout` (it does) and the directory is
  absent or empty.
- `go/internal/guards/docdelete.go` — a PreToolUse guard **denies** `rm … knowledge-base/`
  outright.
- `go/internal/core/boot_preflight.go:46` — boot recovery deliberately excludes the tree
  from quarantine.

Gitignoring the directory would make the protocol's `git add` fail `rc=1` — precisely the
ship-killer class `gitignore_birth_test.go` exists to prevent.

> **Prerequisite:** decide whether the dossier is a *record* (belongs in git) or *telemetry*
> (belongs in `.evolve/`). If telemetry: change `Write` to `commit=false`, move the corpus
> under `.evolve/dossiers/`, repoint `dossier.ReadCommitted`, `cmd_dossier.go`,
> `core/cyclerun_chronicle.go:70` and `boot_preflight.go`, and drop `knowledge-base` from
> `docdelete.go`'s regex. All consumers already read from **disk**, not git, so the corpus
> keeps working — only cross-worktree visibility for fleet lanes needs re-checking.

### 3. `.evolve/evals/` — 689 files / 1.1 MB — *keep*

Deliberate. The `.gitignore` re-include ladder (`!.evolve/evals/` → `.evolve/evals/*` →
`!.evolve/evals/*.md`) exists specifically so eval-materialization lanes can deliver a new
eval, and `gitignore_birth_test.go` pins it. `evalgate/materialization.go:77` requires the
selected slug's file on disk. No action.

> **Note for future sweeps:** a checkout behind `origin/main` shows hundreds of these as
> "untracked". That is staleness, not a hygiene defect. Re-baseline against `origin/main`
> before concluding anything about a corpus.

### 4. `go/acs/cycle*/` — 403 directories, 465 files / 4.4 MB — *keep*

Per-cycle acceptance-criteria predicates. Real Go behind `//go:build acs`, so
`go test ./...` never builds them. Accretion, not junk — but unbounded, **and it rots**:
running `go test -tags acs ./acs/cycle476/...` during this sweep failed with
`only 2 test(s) ran, need >= 3` — `TestC476_002` pins three `internal/core` test names by
string, and `TestSanitizeAdvisorTier_RejectsHighTopAndRawModel` was renamed away sometime
in the ~1,100 cycles since. A predicate that names its subjects by string decays silently
the moment the code it guards is refactored, and nothing runs it in the default suite to
notice. This is a pre-existing failure, unrelated to the hygiene change (which touches
zero files under `internal/core`), and is the strongest argument for the retention policy
below: a corpus nobody runs is a corpus nobody can trust.

> **Prerequisite:** a retention policy (e.g. keep the last N cycles plus any predicate a
> live eval still references) rather than a bulk delete. `go/acs/**/evolve` is already
> gitignored after an 18 MB binary was committed into one of these dirs.

## Consequences

- The tracked tree keeps 27.5 MB of non-code, non-docs content, of which the 20.1 MB binary
  is the single largest item and the one with the clearest exit path.
- The **tier-1** class cannot silently return: `tracked_artifact_hygiene_test.go` runs in
  `repoContractPackages` (`go/internal/phases/ship/repocontract.go:58`), so a re-leak fails
  the cycle that introduced it, naming the path and the `git rm --cached` fix.
- A future sweep that reaches the same conclusions has this document instead of repeating
  the investigation. The generalizable rule it encodes: **trace the consumer before
  trusting the label** — a README, a filename, or a directory's "runtime" naming says
  nothing about whether a gate reads it.
