# ADR-0081 — Boot base-divergence halt + in-band ledger seal anchor

- **Status:** Accepted
- **Date:** 2026-07-29
- **Cycle:** 1189
- **Supersedes / extends:** ADR-0048 (ledger epoch-anchor)

## Context

Two independent failure classes, both "the system stayed quiet when it should
have spoken, or spoke when it should have stayed quiet".

**1. Lanes cut from a stale base.** A fleet lane's worktree is branched from the
project root's HEAD at boot. Nothing at boot compared that HEAD against
`origin/<base>`. When the local base had fallen behind origin, every lane in the
batch was built on stale history and the ship at the end failed with
`GIT_PUSH_REJECTED` — *after* the batch's work was already spent (cycle-969).
`go/internal/looppreflight` had zero `origin/`, `fetch origin`, or divergence
references. The reconcile is a single operator command (`evolve sync-main`), so
the whole cost was paid for a check that did not exist.

**2. `evolve ledger verify` crying wolf.** `Verify` walked the chain from line 1.
A predecessor's bytes were rewritten post-hoc (the ledger-1740 damage), so the
walk reported `BROKEN` forever. ADR-0048 answered this with an out-of-band
epoch anchor (`ledger-anchor.json` + `evolve ledger anchor <seq>`): trust the
prefix, verify forward. That works, but the anchor lives in a sidecar file that
can be lost, copied between checkouts, or diverge from the chain it describes —
and a recovery that resumes the chain leaves no sign-off *inside* the auditable
record.

## Decision

**1. `base-divergence` boot check (Halt).** `looppreflight.Run` gains a check
that fetches origin ITSELF and compares. Ordering of verdicts is deliberate:

| Condition | Level | Why |
|---|---|---|
| local base behind origin (behind > 0) | **Halt** | every lane would be stale; names `evolve sync-main` in the halt text |
| ahead-only (unpushed local work) | Pass | normal; halting here would bench healthy boots |
| fetch/compare failed | **Warn** | UNVERIFIED is surfaced, never a silent pass — and never a halt, so a transient network fault cannot bench a ready boot |
| not a work tree / detached HEAD / no `origin` | Pass | nothing to compare; reason carried in Detail |

The check performs the fetch rather than reading a local `origin/<base>` ref,
because a stale operator-prepared ref is precisely the thing that is wrong in
this failure mode. It compares against `FETCH_HEAD` — written by *this*
invocation — so the comparison cannot read stale state. The whole probe is
bounded by `baseDivergenceTimeout` (60s) so a wedged remote cannot hang boot.

**2. In-band `reset-seal-*` epoch anchor.** `Verify`/`VerifyDeep` now resolve the
walk anchor through one helper, `effectiveAnchorSHA`, which returns the LATER of:

- the out-of-band `ledger-anchor.json` line (ADR-0048, unchanged), and
- the last in-band entry whose `Role` is `operator` and which carries the
  `reset-seal-` marker in **either** `Kind` **or** `cycle_label`, **and** which
  is itself hash-valid from its own predecessor line.

The two marker fields are both accepted because the ledger genuinely carries two
shapes: the production writer (`core.SealCycle`, `go/internal/core/reset.go`)
emits `kind:"reset"` with `cycle_label:"reset-seal-cycle-<N>"`, while the
doc-level shape puts the marker in `kind`. The first implementation matched
`Kind` only and was therefore **inert on every real ledger** — unit-green,
live-red, with `evolve ledger verify` still crying wolf on the adjudicated
line-1740 damage (cycle-1191).

The self-validity requirement is the seal **trust guard**: a seal whose own
`prev_hash` does not chain from its predecessor never moves the anchor. Without
it, appending one operator-role `reset-seal-*` line with a forged `prev_hash`
would silence verification of the entire prefix behind it, converting the epoch
anchor from a preservation remedy into a chain-integrity bypass.

Both mean the same thing to the walk — "history before me is preserved but no
longer chain-validated" — so they share one resolution path and cannot diverge
on what "intact" means. Damage before the anchor is informational; the damaged
lines are **preserved, never deleted**.

## Consequences

- **Sealing is still a trust decision.** The in-band anchor requires
  `Role: operator`; a phase agent writing `reset-seal-*` under its own role
  cannot silence `Verify`.
- **Sealing the past never blanket-silences the present.** A break *after* the
  last anchor still returns `core.ErrLedgerChainBroken`, so the ship gate still
  trips. This is the load-bearing negative — an implementation that "fixes" the
  wolf-cry by short-circuiting `Verify` to nil fails it.
- **A stale sidecar anchor still fails loudly.** When `ledger-anchor.json` names
  a SHA no line carries, it is passed through unchanged so `walkChain` reports
  "anchor not found" rather than degrading to "no anchor, verify everything".
- **Boot can now halt on a condition it previously ignored.** That is the point,
  but it means a batch launched against a behind base stops at boot with a named
  next step instead of failing at ship.

## Addendum (cycle-1194) — automated recovery must not gain operator trust

**Defect.** The in-band seal's Role/CycleLabel are self-declared fields with
no authentication of their own — the hash-chain guard (`sealChainsFromPrev`)
proves only that a line's bytes chain from its predecessor, which requires no
secret, not who wrote it. `core.AutosealStaleMarker` (unattended boot
self-heal, triggered merely by a dead owner PID — e.g. any process crash) and
an explicit human `evolve cycle reset` both called `SealCycle`, which wrote
the identical `Role: "operator"` in both cases. That let anything able to
arrange for an owning process to die mint a trust-anchor-eligible seal with no
human sign-off at all, silently freezing verification of everything before it.

**Fix.** `SealOptions.AutomatedRecovery` (set only by `AutosealStaleMarker`)
makes `SealCycle` write a distinct role, `"operator-autoseal"`, instead of
`"operator"` for the unattended path. `anchor.go`'s `isOperatorSeal` already
matched `Role` exactly, so no reader-side change was needed — only the writer
needed to stop conflating the two trust levels. The automated path still
clears the role-gate block (its actual job); it simply never becomes
epoch-anchor-eligible. A genuine human `evolve cycle reset` is unaffected.

## Alternatives rejected

- **Compare against the local `origin/<base>` ref without fetching.** Free, and
  wrong: the stale ref is the failure mode.
- **Auto-reconcile at boot** (fetch + rebase/merge). Rejected — reconciling is an
  operator decision with conflict risk; the loop halts and names the command
  instead of silently rewriting the operator's base.
- **Halt on any divergence including ahead-only.** Would bench every boot with
  unpushed local work; `behind` is the only condition that produces the
  rejected push.
- **A second, parallel anchor mechanism for the in-band seal.** Rejected — two
  resolution paths would eventually disagree about what "intact" means, which is
  the exact class of defect the shared `walkChain` was introduced to prevent.
