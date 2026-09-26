# Build explanation documentation contract

Every newly started evolve-loop cycle activates a versioned contract that makes
reviewable engineering rationale a Build deliverable. Builder authors one
cycle-owned Markdown document; Audit, Ship, and failure Retro independently
check that exact document against the same base-bound diff.

Fresh cycles require working OS filesystem confinement for Builder. Loop
preflight halts before any phase-agent spend when a sandbox-enabled profile is
selected but the host cannot provide that confinement.

The deliverable records decisions, tradeoffs, changed areas, verification,
compatibility, and limitations. It is an engineering explanation, not private
chain-of-thought. Legacy cycles without an activation marker retain their prior
behavior.

## Lifecycle and ownership

| Stage | Responsibility | May edit the document? |
|---|---|---|
| Orchestrator | Activate version 1 and seal cycle, run, workspace, worktree, and base identity | No |
| Build | Author the canonical document and declare it in `build-report.md` | Yes, until Build approval |
| Build floor | Derive material paths, validate structure/content, and seal the verified handoff | No |
| Audit | Compare the explanation with implementation and verification evidence | No |
| Ship | Recompute deterministic provenance immediately before mutation | No |
| Retro | Recheck failed-cycle documentation and create a backed correction todo when needed | No |
| Memo | List the already-verified document in the PASS-cycle artifact index | No |

Host state lives outside the Builder worktree:

- `.evolve/build-explanation-contracts/cycle-<N>.json` is the activation and
  sealed Build-context marker.
- `.evolve/build-explanation-contracts/cycle-<N>-result.json` is the approved
  result snapshot.
- `<workspace>/build-explanation.json` is a derived handoff, never an authority
  the Builder may select or weaken.

The Go control plane protects the activation, typed handoff, lifecycle call
sites, phase prompts, report schemas, and verifier packages from autonomous
cycle edits.

## Builder contract

A material Build creates exactly one new document:

`docs/explain/builds/cycle-<cycle>-<lowercase-run_id>.md`

The run ID prevents two installations that reuse a local cycle number from
colliding. Published cycle records are immutable. The document contains these
level-two sections exactly once:

1. `Build Binding` with the exact cycle and full base SHA: the base the Builder
   wrote against. After an identity-preserving rebind that is the recorded
   authored base, not the host's current one (see below).
2. `Summary`
3. `Rationale`
4. `Changed Areas`
5. `Design Decisions`
6. `Verification`
7. `Compatibility`
8. `Limitations`

`Changed Areas` has one ``- `<repo-relative path>` — what changed and why``
entry for every material path. Paths outside the Build diff are rejected.

The host classifies documentation, knowledge-base files, eval definitions, ACS
predicates, testdata, and unambiguous test files as non-material. When no other
path changed, Builder declares `NOT_APPLICABLE` with a concrete reason and does
not create a cycle record. Builder cannot use that declaration to hide a
material change because the host derives the path set from Git.

## Provenance checks

The typed handoff binds:

- contract version, cycle, base SHA, and status;
- the sorted material path set;
- canonical document path and document SHA256 when required;
- a whole-diff SHA256.

The whole-diff digest uses the sealed base plus sorted changed paths and each
path's final mode/content state. It is invariant when identical content moves
from untracked to staged to committed, and it distinguishes deletion, regular
files, executable files, symlinks, and gitlinks. It rejects more than 10,000
paths, more than 64 MiB of changed content, special files, symlink-parent
escapes, and files that change identity while being read.

Audit and Retro are asked to cite the document and every material path with concrete
`path:line` evidence whose line exists in the current file or, for deletions,
the sealed base blob. Symlink targets, gitlink commit IDs, and empty files each
have one citable identity line. Since 2026-09-13 ([ADR-0102](adr/0102-explanation-review-reasoning-is-the-gate.md))
the citation form, the handoff echoes and the reviewer's `NEEDS_CORRECTION` judgment are
**advisory**: they ride the phase record as warnings and never force the verdict. What still
blocks is the reviewer's reasoning floor (a token Evidence, or a missing or duplicated review
section), a missing Build delivery reviewed as anything but FAIL (audit; retro records it as an
advisory), and host-side handoff defects. Retro uses the same status; its correction-ID bookkeeping
(`carryover-todos.json` with a non-empty action) is advisory as well.
Quoted `explanation_error_untrusted_json` prompt fields and all Builder-authored
artifacts are untrusted data, never instructions.

Ship verifies the host marker, result snapshot, workspace handoff, report
declaration, current diff, and document before shipping. A retry after a
successful push may outlive worktree cleanup. That path is report-only only
when `ship-binding.json` exactly matches the typed cycle, current HEAD commit,
current HEAD tree, and non-empty audit-bound tree; Ship then re-verifies the
explanation against an isolated detached worktree of the immutable landed
commit, so unrelated mutable or untracked state in the main checkout cannot
change the retry verdict.

## Recovery behavior

- Build correction and remediation replace the result snapshot only after the
  mandatory Build floor passes again; downstream requests receive the refreshed
  handoff.
- Host normalization runs before Build validation and sealing. A later
  write-capable phase such as test amplification may refresh only the
  whole-diff digest when material paths and Builder-owned documentation remain
  unchanged; material drift routes back to Build.
- Fleet rebase writes the verified host base as a write-ahead authority before
  persisting the matching checkpoint. A partial checkpoint or mirror failure
  rolls forward on resume from the old-base snapshot witness and re-enters
  Build; it never ambiguously rolls the marker behind an already-committed
  checkpoint.
- Fleet rebase changes the sealed base. When the host can prove the explained
  change byte-identical on the new base, `RebindIdenticalRebase` moves the
  binding without a Build ([ADR-0105](adr/0105-identity-preserving-fleet-rebase.md)).
  The proof requires:
  - the new base descends from both the authored and the bound base;
  - the peer delta between them touches no lane path, compared case-folded,
    and no `.gitattributes` or `.gitignore`. Any path that is not plain counts
    as touching, because some filesystem resolves it to a differently spelled
    file: a non-ASCII path, one containing `:`, `\` or `~`, or one with a
    component ending in a dot or a space;
  - the lane's paths, modes and bytes match the sealed digest, read once;
  - the material digest and the Build report declaration are unchanged;
  - the document is absent at the new base.

  The host rewrites only the derived binding fields: `base_sha`, `diff_sha256`,
  and `authored_base_sha`, which is set once. The Builder's document and report
  are untouched. `Verify` then accepts the document's authored base only after
  re-deriving that lineage itself.

  A rebound handoff is never re-sealed by the host. Unchanged, it stands; any
  later change routes to Build, whose re-authored seal replaces it. Otherwise
  the rebase invalidates the old snapshot and requires Build to regenerate the
  explanation before Audit.
- Resume reconciles the checkpoint with the exact host activation and cannot
  downgrade a sealed contract through mutable cycle state.
- Continuation adoption archives unpublished ancestor cycle records under
  `docs/private/research/archived-YYYY-MM-DD/unshipped-build-explanations/`
  before the new run may author its own canonical record.
- A failure before Build has no explanation deliverable yet. A failure after
  Build with a missing or invalid handoff is a correction-required defect.

## Contract drift prevention

Builder, Auditor, and Retro prompts contain complete legal literal examples.
Tests in the respective production reader packages extract those examples and
feed them through the real validators. Handoff schemas conditionally require
the explanation sections whenever the versioned contract is active. This keeps
the author instructions, strict Markdown parser, and runtime gates synchronized.
