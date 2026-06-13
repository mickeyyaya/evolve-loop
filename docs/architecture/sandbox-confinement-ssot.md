# Sandbox Confinement SSOT (DetectNested + ShouldWrap)

**Status:** Implemented (2026-06-13) · **Scope:** `internal/adapters/sandbox`, `internal/bridge`, `internal/preflight`

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
