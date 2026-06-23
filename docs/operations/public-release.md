# Public Release Process — `evolve-loop` (private) → `evolveloop` (public)

How a release reaches the public open-source mirror. This is **separate** from
`evolve release X.Y.Z`, which version-bumps and propagates to the Claude plugin
marketplace. This document is about the GitHub public repo.

> **Status (2026-06-23):** Public repo **[mickeyyaya/evolveloop](https://github.com/mickeyyaya/evolveloop)**
> is live (Apache-2.0, clean-slate). The convergence of private `main` to
> publish-readiness is **in progress** — see [Convergence status](#convergence-status).
> Until convergence completes, releases use the **full transform set** (manual
> procedure below). After it completes, the transform shrinks to drop-binary +
> squash, and is automated by `evolve publish-mirror` (planned).

---

## Topology

| Repo | Role | History | Identity |
|---|---|---|---|
| `mickeyyaya/evolve-loop` (private) | **Source of truth.** All development, campaigns, dogfooding. | Full (2,200+ commits) | MIT → (converging to Apache); module `…/evolve-loop/go` → (converging to `…/evolveloop/go`) |
| `mickeyyaya/evolveloop` (public) | **Derived release mirror.** Never hand-edited. | Clean-slate; one squashed commit per release | Apache-2.0; module `…/evolveloop/go`; PII-scrubbed; no tracked binary |

The public repo is a **derived snapshot** of the source of truth. You don't edit
it directly; you regenerate it.

## The two invariants (non-negotiable)

1. **Never `git push` private → public.** The private history carries
   `/Users/<user>` paths and a personal email across thousands of commits.
   A push would leak all of it permanently (GitHub caches/indexes are not
   reliably purgeable). Every release **re-snapshots a clean tree** — it never
   transfers history.
2. **The public tree must pass the sanitizer gate before any push.** Run the
   `ecc:opensource-sanitizer` agent on the snapshot; require **PASS** (or
   PASS-WITH-WARNINGS with only non-blocking items). No PASS, no push.

## History model: append-per-release

Each release **appends one squashed commit** (`Release vX.Y.Z`) to public `main`
and tags it `vX.Y.Z`. No force-push, no history rewrite — stable SHAs and tags,
friendly to anyone who has cloned, forked, or starred. The public `git log`
shows release-by-release; it never shows private development churn.

---

## The release flow

Given a private `main` you want to publish (ideally at a release tag):

1. **Tag-gate.** Confirm private `main` is CI-green and at the version you intend
   to publish (reuse `evolve release-preflight` / `evolve release-consistency`).
2. **Snapshot + transform.** In an isolated worktree off the release commit,
   apply the [transform set](#transform-set), then stage the tree on an **orphan
   branch** (which severs all history). Two exclusions matter:
   - **`go/evolve` is *tracked* in private** (for self-deploy) — it is **not**
     gitignored here, so it must be **explicitly removed** (`git rm --cached`)
     before committing, or the 12 MB binary lands in public.
   - Gitignored runtime state (`.evolve/runs/`, per-cycle worktrees) is excluded
     automatically by the orphan `git add`.
3. **Sanitizer gate.** Run `ecc:opensource-sanitizer` on the scratch tree →
   require PASS.
4. **Append + push + tag.** Commit the tree as one `Release vX.Y.Z` commit on a
   throwaway branch, then push it to public `main` (fast-forward), and tag.
   Push **by URL** so it never touches the private repo's `origin` remote:
   `git push https://github.com/mickeyyaya/evolveloop.git <branch>:main`.
5. **Verify public.** License renders as Apache-2.0; README intact; `git ls-remote`
   shows only the expected refs; no PII / no `go/evolve` in the published tree.

### Transform set

What must change between the private tracked tree and the public tree. This
shrinks as [convergence](#convergence-status) lands the durable transforms into
private `main`.

| Transform | Why | After convergence |
|---|---|---|
| Drop `go/evolve` binary + `go/bin/**` | Embeds DWARF source paths w/ username; users build from source | **stays** a transform (private keeps the tracked binary for self-deploy) |
| Remove the `chore(build)` commit-prefix entry | Its required paths (`go/evolve`, `go/bin/**`) are gitignored in public → an un-satisfiable "dead prefix" | **stays** a transform |
| Apache LICENSE + NOTICE + manifest `license` fields | Public is Apache-2.0 | **gone** (landed in private) |
| PII scrub (`/Users/<user>`, personal email, `user@host` fixtures, dasherized `-Users-<user>-` paths) + `projecthash` golden fix | No personal data in public | **gone** (landed in private) |
| Swap in the condensed public README | Public gets the developer pitch; private keeps full detail in `docs/` | **gone** (private adopts the same README; detail stays in `docs/`) |
| Module path `…/evolve-loop/go` → `…/evolveloop/go` (882 files) | Public must be `go install`-able at its own URL | **gone** (private renamed) — the big one, deferred; see B2 below |

> **Note on test side-effects of the scrub:** changing path strings invalidates
> the `internal/projecthash` golden vectors (they hash literal paths). Recompute
> them via the canonical bash pipeline `printf '%s' "$INPUT" | shasum -a 256 |
> head -c 8` so they remain a real cross-impl check — do **not** paste the Go
> output blindly (that makes the test tautological).

---

## Convergence status

Goal: make private `main` publish-ready so the transform set shrinks to
drop-binary + remove-prefix + squash, which the Go command can then fully
automate.

- **B1 (in progress, low-conflict):** Apache relicense, permanent PII scrub,
  public README adoption. Landed on private `main` via the gated `--class manual`
  path. Safe to do while flag campaigns are open (touches docs/manifests/fixtures,
  not the flag code).
- **B2 (deferred, high-conflict):** the 882-file module rename. **Must wait until
  the open campaign branches (`flag-antirename`, `flag-campaign-10`) land or
  close** — the rename rewrites nearly every Go file and would force a brutal
  rebase on each campaign branch. Do it when `main` is quiet.

---

## Manual procedure (current — full transform set)

Until `evolve publish-mirror` exists, this is the exact, reproducible procedure,
run from a clean private `main` at the release commit.

```text
# 1. Provision an ISOLATED worktree at the release commit. Do NOT run the orphan
#    checkout in your dev checkout — it would rewrite your current branch context.
git worktree add --detach ../evolveloop-release <release-commit>
cd ../evolveloop-release

# 2. Apply the transform set in this worktree: module rename, Apache LICENSE +
#    NOTICE + manifests, PII scrub (+ projecthash golden fix), swap in the public
#    README, remove the chore(build) commit-prefix entry. After convergence most
#    of these are already in the tree and this step shrinks (see Transform set).

# 3. Stage on an orphan branch, then EXPLICITLY drop the tracked binary.
#    `git checkout --orphan` gives the branch no PARENT (history severed) but its
#    index is pre-populated from HEAD — it is NOT empty. `git add -A` overlays the
#    step-2 transform edits; `git rm --cached` then strips the binary from the
#    index (leaving it on disk). Order matters — remove AFTER the add, or the add
#    re-stages it. checkout/add/rm are NOT ship-gated.
git checkout --orphan public-release
git add -A
git rm --cached go/evolve            # the only tracked binary (go/bin is untracked)

# 4. Verify the staged tree: no go/evolve / no go/bin entries
#    (git diff --cached --name-only | grep -E '^go/(evolve$|bin/)'  → empty);
#    symlinks are mode 120000 (git ls-files -s -- .agents/skills/loop);
#    0 PII residuals (git grep --cached for the username / personal email / a
#    /Users/<user> path); LICENSE is Apache; README is the public one.

# 5. Sanitizer gate: run the ecc:opensource-sanitizer agent on the staged tree.
#    Require PASS (or PASS-WITH-WARNINGS, non-blocking only). No PASS → stop.

# 6. Commit, tag, push. git commit / git push are ship-gate-denied in the dev
#    repo, so the OPERATOR runs these (your own terminal, or a script the gate
#    sees as `bash …`). You are the principal; the gate guards agent commits to
#    the dev codebase, not an external-repo snapshot. Create the tag BEFORE
#    pushing it.
git commit -m "Release vX.Y.Z"
git tag vX.Y.Z
git push https://github.com/mickeyyaya/evolveloop.git public-release:main
git push https://github.com/mickeyyaya/evolveloop.git vX.Y.Z

# 7. Verify public (gh repo view; git ls-remote → main + the tag; license +
#    README + no-binary spot-check), then remove the worktree. --force is needed
#    because go/evolve is still on disk (it was rm'd from the index, not deleted).
git worktree remove --force ../evolveloop-release
```

> **Why a script/terminal for the commit:** the ship-gate (`evolve guard ship`)
> regex-blocks `git commit`/`git push` to force attestation on commits to the
> **dev codebase**. Publishing a *separate* external repo is outside its intent —
> a false-positive. `gh repo create` is **not** gated. The operator running the
> snapshot's commit is the sanctioned path; never weaken the dev-repo gate itself.

## Planned automation: `evolve publish-mirror`

A Go subcommand (this project is all-Go, no scripts) that runs steps 1–5
deterministically: resolve the release commit, snapshot the tracked tree, apply
the residual transforms, invoke the sanitizer gate, squash, push-by-URL, tag,
and verify. Tracked in the release-process work; see the engineering tasks.

---

## First-release reconciliation note (delete after B1 lands)

The public repo already contains the initial release + a README refinement made
directly on the mirror (2026-06-23). The first run of the formalized process must
reproduce ~that state from private (so it doesn't clobber the refined README).
Once B1 lands the public README into private, the pipeline output and the current
public tree converge.
