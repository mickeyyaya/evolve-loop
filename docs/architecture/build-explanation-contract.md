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

1. `Build Binding` with the exact cycle and full base SHA
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

Audit and Retro must cite the document and every material path with concrete
`path:line` evidence whose line exists in the current file or, for deletions,
the sealed base blob. Symlink targets, gitlink commit IDs, and empty files each
have one citable identity line. Audit reports `NEEDS_CORRECTION` when prose drifts; that negative
judgment forces Audit failure. Retro uses the same status and requires every
correction ID to exist in `carryover-todos.json` with a non-empty action.
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
- Fleet rebase changes the sealed base, invalidates the old snapshot, and
  requires Build to regenerate the explanation before Audit.
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
