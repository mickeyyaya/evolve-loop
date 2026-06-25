# Sandbox Confinement SSOT (DetectNested + ShouldWrap)

**Status:** Implemented (2026-06-13); extended 2026-06-25 (see "Update: measure-don't-guess + verified fallback") · **Scope:** `internal/adapters/sandbox`, `internal/bridge`, `internal/preflight`, `internal/looppreflight`, `internal/policy`

## Request / trigger

During a shadow soak (cycles 324–326) every cycle failed at scout with
`exit=80 ExitREPLBootTimeout`: the `claude` REPL never booted within 60s. Root
trigger was an operator-supplied invalid value `EVOLVE_SANDBOX=1`, but the
investigation surfaced a deeper architectural flaw, fixed here per the
"no workaround — redesign from the architecture perspective" directive.

## Root cause (architecture)

"Should the inner OS sandbox (`sandbox-exec`/`bwrap`) wrap a phase launch?" was
decided inconsistently across the codebase:

| Concern | Before (duplicated / split) |
|---|---|
| Nested-Claude detection | `preflight` read `CLAUDECODE`; `bridge.isNestedClaude` read `CLAUDE_CODE_ENTRYPOINT`/`CLAUDE_CODE_SESSION_ID` — **two heuristics** |
| "Can we confine here?" | bridge used `ProbeResult.Available` (binary-on-PATH); preflight used `decideSandbox().ExpectedToWork` (capability) — **two notions** |
| Wrap policy vs mode | bridge skipped the wrap under nested-Claude **only for `auto` mode** |

Consequence: the bridge hot path used the weakest signal (binary-presence) and
an `auto`-only nested skip. So a *correctly spelled* `EVOLVE_SANDBOX=on` — and
any unrecognized value — still wrapped under nested Claude, where macOS
`sandbox_apply()` returns EPERM and the wrapped REPL hangs. Preflight already
*knew* the host couldn't confine (`ExpectedToWork=false`, `InnerSandbox=false`)
and promised "degrades gracefully", but the bridge never consumed that decision.
A Single-Source-of-Truth violation.

## Approaches considered

1. **Normalize the bad value only** (`EVOLVE_SANDBOX=1` → `auto`). Rejected as a
   workaround: fixes the one trigger, leaves `on`-mode-under-nested hanging and
   the duplication intact. (Kept as a *defensive* secondary measure.)
2. **Bridge consumes preflight's decision** (`import preflight`). Wrong
   dependency direction — preflight is higher-level orchestration; bridge is a
   lower launch layer.
3. **SSOT in the lowest shared layer `adapters/sandbox`** (chosen). Both bridge
   and preflight depend *downward* on it; no import cycle (the package is
   stdlib-only).

## Design (chosen)

Two functions in `internal/adapters/sandbox` are the single source of truth:

- `DetectNested(getenv func(string) string) bool` — one nested-Claude heuristic
  (union of `CLAUDECODE` / `CLAUDE_CODE_ENTRYPOINT` / `CLAUDE_CODE_SESSION_ID`,
  with `CLAUDECODE_TYPE=host` overriding). Replaces both prior heuristics.
- `ShouldWrap(nested bool, probe ProbeResult) (bool, string)` — the wrap policy:
  wrap **iff** OS supported ∧ binary available ∧ **not nested**. The nested
  exclusion is universal (all modes, all OSes): under an outer Claude session
  the inner sandbox is redundant (outer already confines) and on macOS
  non-functional. Returns a non-empty reason for callers to surface.

Consumers:

- **bridge** (`sandbox_wrap.go`): `off` → skip; else `ShouldWrap(DetectNested(env), probe)`.
  `!wrap` degrades for **all** modes; `on` emits a loud `UNCONFINED` WARN.
  `isNestedClaude` deleted.
- **preflight**: nested detection → `DetectNested`; the `InnerSandbox` boolean →
  `ShouldWrap` (reason strings stay preflight-local — presentation, not
  decision). `decideSandbox().ExpectedToWork` is retained as the distinct
  *capability report* in the host profile.

**Pattern:** dependency inversion onto a single policy owner. Because preflight
and the bridge now apply the *same* decision, preflight's "degrades gracefully"
promise is true by construction.

### Equivalence (behaviour-preserving for preflight)

`ShouldWrap(nested, probe)` equals the old `InnerSandbox = !nested && ExpectedToWork`
in every reachable cell: when `!nested`, capability collapses to availability;
when `nested`, both are false.

## Tests

- `adapters/sandbox/confinement_test.go`: `DetectNested` (6 cases incl. host
  override) + `ShouldWrap` (7 cases incl. the darwin/linux nested→skip cells).
- `bridge/sandbox_wrap_test.go`: `on`-mode + nested → no wrap + WARN (the closed
  footgun); unknown value normalize; existing off/auto/on/darwin cases.
- All consumers green with `-race`; full `go build ./...`, `looppreflight` green.

---

## Update: measure-don't-guess + verified fallback (2026-06-25)

### Request / trigger

The 2026-06-13 SSOT closed the `on`-mode-under-nested footgun but left two
coupled defects in how the nested fallback is **decided** and **trusted**:

- **P1 — the wrap decision is an env-var GUESS, not a measurement.** `DetectNested`
  reads `CLAUDECODE*`; a wrong guess is asymmetric and severe: a *broken/SIP-weird
  standalone* host is guessed "not nested → wrap" and **hangs the REPL boot
  (exit 80)**; a stray env var falsely marks nested → silently drops confinement.
- **P2 — the fallback silently ASSUMES the outer session confines us.** When the
  wrap is skipped, the loop proceeds on the unverified belief that the outer
  Claude session confines phase writes; even `EVOLVE_SANDBOX=on` only WARNed and
  ran unconfined. The preflight message "the bridge degrades gracefully" concealed
  the loss of the inner write-confinement guarantee.

Both are one disease: **posture decided by guessing the environment rather than
measuring it.**

### Design (chosen) — five dormant, independently-green slices

1. **Capability probe (P1).** `ProbeResult` gains measured `Capable` +
   `CapabilityChecked`; `Probe()` execs a trivial confined `/usr/bin/true` once
   (bounded 3s, `WaitDelay`-guarded; timeout/error ⇒ not capable), cached. The
   env heuristic degrades to an explanatory reason only.
2. **Subtractive `ShouldWrap`.** A MEASURED-incapable sandbox is demoted to skip
   (fixes the broken-standalone hang); capability can only DEMOTE a would-be wrap,
   never PROMOTE a nested skip — the non-regression invariant is
   **`new_wrap ⟹ old_wrap`**. An unchecked probe is byte-identical to the legacy
   path (this supersedes the "Equivalence" section above).
3. **Measured `ExpectedToWork` (preflight).** An opt-in `Options.SandboxCapable`
   seam (nil ⇒ legacy) lets a measured-incapable result subtractively demote
   `Sandbox.ExpectedToWork`; `innerProbe` carries the measurement so `InnerSandbox`
   stays consistent. JSON shape (schema_version 3) unchanged.
4. **Honest WARN (P2).** The preflight WARN now states the truth — "source-writing
   phases run **UNCONFINED at the inner layer** — the outer Claude Code session +
   Tier-1 hooks are the only confinement" — replacing "degrades gracefully".
5. **Verified-fallback canary (P2).** Config dial `sandbox.nested_fallback`
   (`off` default / `shadow` / `enforce`, via `parseGateStage`) gates a preflight
   write-canary that VERIFIES the outer environment blocks an out-of-allowlist
   write: `shadow` WARNs when unverified, `enforce` HALTs. Default `off` ⇒ the
   canary never runs and a fresh `policy.json` never halts. `enforce` is opt-in
   for genuinely-confined outer sessions: under bypass-permissions Claude the
   outer does not confine writes, so the canary reports unverified by design.

### Pattern

Measure the real confinement posture (capability probe, write-canary), surface it
truthfully (honest WARN), gate on it (Stage dial). All defaults are behavior-
neutral: the capability path is subtractive, the dial defaults off. Seams are
injected (DI) and the dial is config-as-SSOT (`policy.SandboxConfig()`, no Go
literal) — consistent with the `RecoveryConfig`/`parseGateStage` pattern.
