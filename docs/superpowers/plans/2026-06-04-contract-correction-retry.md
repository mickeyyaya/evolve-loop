# Contract-Correction Retry — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When a finished phase's deliverable violates its contract, re-dispatch the phase with the violation text as a correction directive (bounded to `EVOLVE_CONTRACT_CORRECTION_RETRIES`, default 2) instead of aborting the cycle on the first reject.

**Architecture:** A localized correction loop at the existing deliverable-review reject point in the orchestrator (`orchestrator.go:1531`). On `!rr.Approve`, compose a correction directive from `rr.Reason`, set it on `PhaseRequest.CorrectionDirective`, re-run `runner.Run`, and re-review — up to N corrections, then abort with today's error (the preserved floor). The directive flows `PhaseRequest → BridgeRequest → injectCorrectionPrefix` (a `## Correction` block at the existing `bridge.go:125` prompt-prefix seam, CLI-agnostic). Default-on; `=0` is byte-identical to today.

**Tech Stack:** Go; `go test` table tests; the `envchain` typed-env helpers; the existing `core.DeliverableReviewer` seam.

**Design spec:** `docs/superpowers/specs/2026-06-04-orchestrator-contract-correction-retry-design.md`.

**Scope note / refinement from the spec:** the spec §4.1 wrote "wraps the existing run + bridge-timeout retry loop." During planning the bridge-timeout retry was found to be the orchestrator's `for attempt` loop (1414), a large fragile block. This plan instead uses a **localized** correction loop that re-runs `runner.Run` directly per correction (runner.Run still does its internal CLI-fallback chain). Consequence: a correction re-dispatch that hits a transient bridge error aborts rather than re-trying the timeout — a rare edge case, documented. Same final behavior for the common case; far simpler and reliably testable.

---

## File Structure

| File | Responsibility | Change |
|---|---|---|
| `go/internal/envchain/keys.go` | env-var name + default/max constants | add `KeyContractCorrectionRetries`, `DefContractCorrectionRetries=2`, `MaxContractCorrectionRetries=5` |
| `go/internal/core/phase.go` | `PhaseRequest` | add `CorrectionDirective string` |
| `go/internal/core/ports.go` | `BridgeRequest` | add `CorrectionDirective string` |
| `go/internal/adapters/bridge/bridge.go` | prompt assembly | add `injectCorrectionPrefix`, wire at the `:125` seam |
| `go/internal/phases/runner/runner.go` | builds `BridgeRequest` | copy `req.CorrectionDirective` through |
| `go/internal/core/orchestrator.go` | cycle loop | add `composeCorrection`, `resolveContractCorrectionRetries`, the correction loop at `:1531` |
| `CLAUDE.md` | env-var reference | add the new knob row |

---

## Task 1: env knob — `EVOLVE_CONTRACT_CORRECTION_RETRIES`

**Files:**
- Modify: `go/internal/envchain/keys.go`
- Test: `go/internal/envchain/keys_test.go` (create if absent; else append)

- [ ] **Step 1: Write the failing test**

```go
// in go/internal/envchain/keys_test.go (package envchain)
func TestContractCorrectionRetriesConstants(t *testing.T) {
	if KeyContractCorrectionRetries != "EVOLVE_CONTRACT_CORRECTION_RETRIES" {
		t.Errorf("key = %q", KeyContractCorrectionRetries)
	}
	if DefContractCorrectionRetries != 2 {
		t.Errorf("default = %d, want 2", DefContractCorrectionRetries)
	}
	if MaxContractCorrectionRetries != 5 {
		t.Errorf("max = %d, want 5", MaxContractCorrectionRetries)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go && go test ./internal/envchain/ -run TestContractCorrectionRetriesConstants`
Expected: FAIL — `undefined: KeyContractCorrectionRetries`

- [ ] **Step 3: Add the constants**

In `go/internal/envchain/keys.go`, add to the name `const (...)` block:

```go
	// KeyContractCorrectionRetries bounds how many times the orchestrator
	// re-dispatches a phase with a correction directive after a deliverable
	// contract violation. Range [0,5]; 0 disables (immediate abort, the
	// pre-feature behavior). Out-of-range/unparseable → DefContractCorrectionRetries.
	KeyContractCorrectionRetries = "EVOLVE_CONTRACT_CORRECTION_RETRIES"
```

and to the defaults `const (...)` block:

```go
	// DefContractCorrectionRetries: 2 correction re-dispatches before abort.
	DefContractCorrectionRetries = 2
	// MaxContractCorrectionRetries is the hard ceiling on correction re-dispatches.
	MaxContractCorrectionRetries = 5
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd go && go test ./internal/envchain/ -run TestContractCorrectionRetriesConstants`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go/internal/envchain/keys.go go/internal/envchain/keys_test.go
git commit -m "feat(envchain): EVOLVE_CONTRACT_CORRECTION_RETRIES knob (default 2)"
```

---

## Task 2: `injectCorrectionPrefix` (bridge prompt prefix)

**Files:**
- Modify: `go/internal/adapters/bridge/bridge.go`
- Test: `go/internal/adapters/bridge/bridge_test.go` (append; package `bridge`)

- [ ] **Step 1: Write the failing test**

```go
func TestInjectCorrectionPrefix(t *testing.T) {
	// Empty directive = identity (off path byte-identical).
	if got := injectCorrectionPrefix("BODY", ""); got != "BODY" {
		t.Errorf("empty directive must pass through, got %q", got)
	}
	// Non-empty prepends a ## Correction block above the body.
	got := injectCorrectionPrefix("BODY", "fix the Verdict section")
	if !strings.HasPrefix(got, "## Correction\n\n") {
		t.Errorf("missing Correction header: %q", got)
	}
	if !strings.Contains(got, "fix the Verdict section") || !strings.HasSuffix(got, "BODY") {
		t.Errorf("directive/body not assembled: %q", got)
	}
}
```

(Ensure `strings` is imported in the test file.)

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go && go test ./internal/adapters/bridge/ -run TestInjectCorrectionPrefix`
Expected: FAIL — `undefined: injectCorrectionPrefix`

- [ ] **Step 3: Implement (mirror `injectRulesPrefix`)**

In `go/internal/adapters/bridge/bridge.go`, next to `injectRulesPrefix`:

```go
// injectCorrectionPrefix prepends a "## Correction" block carrying the
// orchestrator's contract-correction directive (the previous deliverable was
// rejected; fix it). Empty directive passes through unchanged. Applied at the
// same CLI-agnostic seam as injectRulesPrefix, OUTERMOST so it lands at the very
// top of the prompt where the agent sees the correction first.
func injectCorrectionPrefix(prompt, directive string) string {
	if directive == "" {
		return prompt
	}
	return "## Correction\n\n" + directive + "\n\n---\n\n" + prompt
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd go && go test ./internal/adapters/bridge/ -run TestInjectCorrectionPrefix`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go/internal/adapters/bridge/bridge.go go/internal/adapters/bridge/bridge_test.go
git commit -m "feat(bridge): injectCorrectionPrefix (## Correction prompt block)"
```

---

## Task 3: thread `CorrectionDirective` end-to-end (fields + wiring)

**Files:**
- Modify: `go/internal/core/phase.go` (PhaseRequest), `go/internal/core/ports.go` (BridgeRequest)
- Modify: `go/internal/adapters/bridge/bridge.go:125` (wire), `go/internal/phases/runner/runner.go:404` (copy)
- Test: `go/internal/adapters/bridge/bridge_test.go` (prompt contains the block end-to-end)

- [ ] **Step 1: Write the failing test** (bridge assembles the directive into the launched prompt)

Find the existing bridge test that captures the materialized prompt (search `bridge_test.go` for a test that inspects `inproc.Prompt` or the prompt file). Add:

```go
func TestLaunch_CorrectionDirectiveAppearsInPrompt(t *testing.T) {
	// Arrange: a BridgeRequest carrying a CorrectionDirective (mirror the
	// arrangement of the existing SystemPrompt prompt-assembly test in this file).
	// Act: assemble the prompt via the same path the existing SystemPrompt test uses.
	// Assert: the assembled prompt contains "## Correction" and the directive text,
	// and (off case) an empty CorrectionDirective leaves the prompt unchanged.
}
```

Model this on the existing `SystemPrompt`/`injectRulesPrefix` assembly test in `bridge_test.go` (same seam). If that test inspects `injectRulesPrefix(...)` output directly, assert on `injectCorrectionPrefix(injectRulesPrefix(...), directive)` instead.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go && go test ./internal/adapters/bridge/ -run TestLaunch_CorrectionDirectiveAppearsInPrompt`
Expected: FAIL — `req.CorrectionDirective` undefined.

- [ ] **Step 3a: Add the field to `BridgeRequest`** (`go/internal/core/ports.go`, after `SystemPrompt`)

```go
	// CorrectionDirective, when non-empty, is prepended as a "## Correction"
	// block (the orchestrator's contract-correction retry — the previous
	// deliverable was rejected; fix it). Empty = no-op. See injectCorrectionPrefix.
	CorrectionDirective string `json:"correction_directive,omitempty"`
```

- [ ] **Step 3b: Add the field to `PhaseRequest`** (`go/internal/core/phase.go`, after `Env`)

```go
	// CorrectionDirective is set by the orchestrator's contract-correction loop
	// on a re-dispatch after a deliverable reject; the runner copies it into the
	// BridgeRequest. Empty on the first dispatch.
	CorrectionDirective string `json:"correction_directive,omitempty"`
```

- [ ] **Step 3c: Wire the bridge seam** (`go/internal/adapters/bridge/bridge.go:125`)

Change:

```go
	inproc.Prompt = injectRulesPrefix(injectPolicyPrefix(body, resolvePolicy(req.Agent, req.Env)), req.SystemPrompt)
```

to:

```go
	inproc.Prompt = injectCorrectionPrefix(injectRulesPrefix(injectPolicyPrefix(body, resolvePolicy(req.Agent, req.Env)), req.SystemPrompt), req.CorrectionDirective)
```

- [ ] **Step 3d: Copy in the runner** (`go/internal/phases/runner/runner.go:404`, in the `core.BridgeRequest{...}` literal, after `SystemPrompt: sysPrompt,`)

```go
			CorrectionDirective: req.CorrectionDirective,
```

- [ ] **Step 4: Run tests**

Run: `cd go && go build ./... && go test ./internal/adapters/bridge/ ./internal/phases/runner/ ./internal/core/`
Expected: PASS (build clean; the new test passes).

- [ ] **Step 5: Commit**

```bash
git add go/internal/core/phase.go go/internal/core/ports.go go/internal/adapters/bridge/bridge.go go/internal/phases/runner/runner.go go/internal/adapters/bridge/bridge_test.go
git commit -m "feat(core,bridge,runner): thread CorrectionDirective into the launch prompt"
```

---

## Task 4: `composeCorrection` (pure directive builder)

**Files:**
- Modify: `go/internal/core/orchestrator.go`
- Test: `go/internal/core/orchestrator_test.go` (or a new `correction_test.go` in package `core`)

- [ ] **Step 1: Write the failing test**

```go
func TestComposeCorrection(t *testing.T) {
	got := composeCorrection("audit deliverable failed contract: [missing_section] required section 'Verdict' not found")
	if !strings.Contains(got, "REJECTED") {
		t.Errorf("missing rejection framing: %q", got)
	}
	if !strings.Contains(got, "missing_section") {
		t.Errorf("must embed the violation reason: %q", got)
	}
	if !strings.Contains(got, "contracted path") {
		t.Errorf("must instruct writing at the contracted path: %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go && go test ./internal/core/ -run TestComposeCorrection`
Expected: FAIL — `undefined: composeCorrection`

- [ ] **Step 3: Implement**

In `go/internal/core/orchestrator.go` (near `resolvePhaseMaxAttempts`):

```go
// composeCorrection turns a deliverable-reject reason into the correction
// directive injected into the phase re-dispatch (## Correction prompt block).
func composeCorrection(reason string) string {
	return "Your previous output for this phase was REJECTED by the deliverable contract check:\n\n" +
		reason +
		"\n\nFix the deliverable so it satisfies the contract — write it at the EXACT contracted path " +
		"with all required sections / valid structure — then finish. Do not change unrelated files."
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd go && go test ./internal/core/ -run TestComposeCorrection`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go/internal/core/orchestrator.go go/internal/core/orchestrator_test.go
git commit -m "feat(core): composeCorrection directive builder"
```

---

## Task 5: `resolveContractCorrectionRetries` (env resolver, allows 0)

**Files:**
- Modify: `go/internal/core/orchestrator.go`
- Test: `go/internal/core/orchestrator_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestResolveContractCorrectionRetries(t *testing.T) {
	cases := []struct{ in map[string]string; want int }{
		{nil, 2},                                                  // default
		{map[string]string{"EVOLVE_CONTRACT_CORRECTION_RETRIES": "0"}, 0}, // disable allowed
		{map[string]string{"EVOLVE_CONTRACT_CORRECTION_RETRIES": "3"}, 3},
		{map[string]string{"EVOLVE_CONTRACT_CORRECTION_RETRIES": "99"}, 5}, // clamp to max
		{map[string]string{"EVOLVE_CONTRACT_CORRECTION_RETRIES": "-1"}, 2}, // invalid → default
		{map[string]string{"EVOLVE_CONTRACT_CORRECTION_RETRIES": "x"}, 2},  // unparseable → default
	}
	for _, c := range cases {
		if got := resolveContractCorrectionRetries(c.in); got != c.want {
			t.Errorf("env=%v → %d, want %d", c.in, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go && go test ./internal/core/ -run TestResolveContractCorrectionRetries`
Expected: FAIL — `undefined: resolveContractCorrectionRetries`

- [ ] **Step 3: Implement** (note: 0 is VALID here — use `Int` then clamp, not `IntMin` with min 1)

```go
// resolveContractCorrectionRetries reads EVOLVE_CONTRACT_CORRECTION_RETRIES.
// 0 is valid (disable → immediate abort). Negative/unparseable → default 2;
// above the ceiling → clamped to 5.
func resolveContractCorrectionRetries(env map[string]string) int {
	n := envchain.Int(envchain.KeyContractCorrectionRetries, env, envchain.DefContractCorrectionRetries)
	if n < 0 {
		return envchain.DefContractCorrectionRetries
	}
	if n > envchain.MaxContractCorrectionRetries {
		return envchain.MaxContractCorrectionRetries
	}
	return n
}
```

Verify `envchain.Int` returns the default on an unparseable value (read `internal/envchain/typed.go:23`). If `Int` returns 0 (not default) on unparseable, instead guard: resolve the raw and fall back. Adjust the test/impl to match `Int`'s actual contract — they must agree.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd go && go test ./internal/core/ -run TestResolveContractCorrectionRetries`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go/internal/core/orchestrator.go go/internal/core/orchestrator_test.go
git commit -m "feat(core): resolveContractCorrectionRetries (0=disable, clamp [0,5])"
```

---

## Task 6: the correction loop (integration at `orchestrator.go:1531`)

**Files:**
- Modify: `go/internal/core/orchestrator.go:1523-1535`
- Test: `go/internal/core/reviewer_test.go` (extend; reuse `recordingReviewer`, `buildRunners`, the existing harness)

- [ ] **Step 1: Add a sequenced reviewer test fake**

In `go/internal/core/reviewer_test.go`, add a fake that returns a scripted sequence of results for a target phase (so we can script reject→reject→approve). Model it on the existing `recordingReviewer`:

```go
// sequencedReviewer returns results[i] on the i-th Review of `phase`; once the
// slice is exhausted it returns the last element. Other phases approve.
type sequencedReviewer struct {
	phase   string
	results []ReviewResult
	calls   int
}

func (s *sequencedReviewer) Review(_ context.Context, in ReviewInput) ReviewResult {
	if in.Phase != s.phase {
		return ReviewResult{Approve: true}
	}
	i := s.calls
	s.calls++
	if i >= len(s.results) {
		i = len(s.results) - 1
	}
	return s.results[i]
}
```

- [ ] **Step 2: Write the failing tests**

```go
// reject once then approve → exactly one correction re-dispatch, cycle proceeds.
func TestCorrectionLoop_RejectThenApprove(t *testing.T) {
	st, led := newTestState(t) // use whatever the existing reviewer_test.go uses to build state+ledger
	rev := &sequencedReviewer{phase: "build", results: []ReviewResult{
		{Approve: false, Reason: "deliverable missing required header"},
		{Approve: true},
	}}
	o := NewOrchestrator(st, led, buildRunners(nil), WithReviewer(rev))
	// Drive one cycle through 'build' (mirror the existing reviewer_test cycle driver).
	// Assert: no abort error; rev.calls == 2 (initial + 1 correction).
}

// always reject → abort after maxCorrections (default 2), error mentions the count.
func TestCorrectionLoop_ExhaustsThenAborts(t *testing.T) {
	rev := &recordingReviewer{
		default_:  ReviewResult{Approve: true},
		overrides: map[string]ReviewResult{"build": {Approve: false, Reason: "still malformed"}},
	}
	o := NewOrchestrator(st, led, buildRunners(nil), WithReviewer(rev))
	// Assert: cycle aborts with an error containing "after 2 correction" and "still malformed".
}

// EVOLVE_CONTRACT_CORRECTION_RETRIES=0 → immediate abort on first reject (byte-identical to today).
func TestCorrectionLoop_DisabledIsImmediateAbort(t *testing.T) {
	// Set the env on the per-cycle request (phaseReq.Env) = {"EVOLVE_CONTRACT_CORRECTION_RETRIES":"0"}.
	rev := &recordingReviewer{default_: ReviewResult{Approve: true},
		overrides: map[string]ReviewResult{"build": {Approve: false, Reason: "x"}}}
	// Assert: aborts after exactly ONE Review of build (rev count for build == 1), error matches today's "rejected".
}
```

Adapt `newTestState`, the cycle driver, and the env-injection to the EXACT helpers already in `reviewer_test.go` (read `TestReviewGate_RejectAborts` at `reviewer_test.go:86` — copy its setup verbatim and only change the reviewer + assertions). Add a recording runner (or reuse `buildRunners`) that captures the last `PhaseRequest` so a fourth assertion can check `phaseReq.CorrectionDirective` was non-empty + contained the reason on the re-dispatch.

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd go && go test ./internal/core/ -run TestCorrectionLoop`
Expected: FAIL — the loop doesn't exist yet (reject still aborts immediately; `RejectThenApprove` aborts).

- [ ] **Step 4: Implement the correction loop**

Replace `orchestrator.go:1523-1535` (the `if o.reviewer != nil && resp.Verdict != VerdictSKIPPED { ... }` block) with:

```go
		if o.reviewer != nil && resp.Verdict != VerdictSKIPPED {
			rin := ReviewInput{
				Phase:       string(next),
				Response:    resp,
				Workspace:   cs.WorkspacePath,
				Worktree:    phaseWorktree,
				ProjectRoot: req.ProjectRoot,
			}
			rr := o.reviewer.Review(ctx, rin)
			maxCorrections := resolveContractCorrectionRetries(phaseReq.Env)
			for corr := 1; !rr.Approve && corr <= maxCorrections; corr++ {
				fmt.Fprintf(os.Stderr, "[orchestrator] phase %s: contract violation (correction %d/%d) — re-dispatching with correction: %s\n",
					next, corr, maxCorrections, rr.Reason)
				if lerr := o.ledger.Append(ctx, LedgerEntry{
					TS: o.now().UTC().Format(time.RFC3339), Cycle: cycle, Role: string(next),
					Kind: "contract_correction", ExitCode: 0,
				}); lerr != nil {
					fmt.Fprintf(os.Stderr, "[orchestrator] WARN contract_correction ledger append: %v\n", lerr)
				}
				phaseReq.CorrectionDirective = composeCorrection(rr.Reason)
				obsCancel := o.observer.Start(ctx, string(next), phaseReq)
				resp, err = runner.Run(ctx, phaseReq)
				if obsCancel != nil {
					obsCancel()
				}
				if err != nil {
					return result, fmt.Errorf("phase %q correction %d dispatch failed: %w", next, corr, err)
				}
				rin.Response = resp
				rr = o.reviewer.Review(ctx, rin)
			}
			phaseReq.CorrectionDirective = "" // reset so it never leaks to the next phase
			if !rr.Approve {
				return result, fmt.Errorf("review gate: phase %q deliverable rejected after %d correction(s): %s", next, maxCorrections, rr.Reason)
			}
		}
```

Notes for the implementer:
- `maxCorrections == 0` ⇒ the `for` never enters ⇒ immediate abort on reject (byte-identical to today, except the error text adds "after 0 correction(s)" — if a test pins the OLD exact message, keep the old message when `maxCorrections == 0`: guard the abort string).
- The directive is reset to "" after the loop so a subsequent phase's `phaseReq` (same struct, reused) is clean.
- This re-runs `runner.Run` directly (no bridge-timeout retry on corrections — see the scope note at the top).
- `resp` and `err` are the outer-scope vars already declared at `:1406`.

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd go && go test ./internal/core/ -run TestCorrectionLoop` then `cd go && go test -race ./internal/core/`
Expected: PASS, no race.

- [ ] **Step 6: Commit**

```bash
git add go/internal/core/orchestrator.go go/internal/core/reviewer_test.go
git commit -m "feat(core): contract-correction retry loop (re-dispatch <=N before abort)"
```

---

## Task 7: docs (CLAUDE.md row + spec refinement note)

**Files:**
- Modify: `CLAUDE.md` (env-var reference table), the design spec (note the localized-loop refinement)

- [ ] **Step 1: Add the CLAUDE.md row** under "Current behavior (env-var reference)":

```
| Contract correction retry | `EVOLVE_CONTRACT_CORRECTION_RETRIES` | `2` (default-on) | On a deliverable-contract violation (DeliverableReviewer `Approve=false`) the orchestrator re-dispatches the phase with the violation text injected as a `## Correction` prompt block (`injectCorrectionPrefix`), up to N times, before aborting the cycle (today's behavior is the floor). `0` = immediate abort (byte-identical pre-feature). Separate from `EVOLVE_PHASE_MAX_ATTEMPTS` (bridge-timeout retries). Within-phase (no new cycle numbers); each correction logs a `contract_correction` ledger entry. Scope: well-formedness only — semantic/audit-FAIL is out. Impl: `orchestrator.go` correction loop + `core.PhaseRequest/BridgeRequest.CorrectionDirective`. |
```

- [ ] **Step 2: Append a "Refinement" note** to `docs/superpowers/specs/2026-06-04-orchestrator-contract-correction-retry-design.md` recording that the implemented loop is localized (re-runs `runner.Run` directly; corrections do not get the bridge-timeout retry), and why.

- [ ] **Step 3: Verify full build + tests**

Run: `cd go && go build ./... && go test ./... 2>&1 | grep -c '^FAIL'`
Expected: `0`

- [ ] **Step 4: Commit** (pure docs)

```bash
git add CLAUDE.md docs/superpowers/specs/2026-06-04-orchestrator-contract-correction-retry-design.md
git commit -m "docs: EVOLVE_CONTRACT_CORRECTION_RETRIES + localized-loop refinement note"
```

---

## Self-review checklist (run before execution)

- **Spec coverage:** R1 (re-dispatch w/ directive) → Tasks 3,4,6. R2 (bound 2) → Tasks 1,5,6. R3 (scope = contract only) → Task 6 (only fires on DeliverableReviewer reject). R4 (default-on, 0=off) → Tasks 1,5,6. R5 (TDD/coverage) → every task. R6 (observable, no fabricated cycles) → Task 6 ledger entry + within-phase loop.
- **Placeholder scan:** Task 3 Step 1 and Task 6 Step 2 reference existing harness helpers by name (`bridge_test.go` SystemPrompt test; `reviewer_test.go:86`, `recordingReviewer`, `buildRunners`) rather than inlining an invented harness — the implementer copies the real setup. This is intentional (the harness exists; reproducing it blind would be wrong), not a placeholder for the logic under test, which is fully specified.
- **Type consistency:** `CorrectionDirective` (both structs), `composeCorrection(string) string`, `resolveContractCorrectionRetries(map[string]string) int`, `injectCorrectionPrefix(string,string) string` — names consistent across tasks.
- **Coverage gate:** after Task 6, run `cd go && go test -cover ./internal/core/ ./internal/adapters/bridge/ ./internal/envchain/ ./internal/phases/runner/` and confirm the new functions are exercised (aim ≥95% on changed packages, per house rule).
- **Off-path:** Task 6 Step 4 note — preserve today's EXACT abort message when `maxCorrections == 0` if any existing test pins it.

---

## Execution

Subagent-driven TDD (fresh implementer per task + spec-then-quality review), ≥95% gate, then `superpowers:finishing-a-development-branch`. Branch `feat/contract-correction-retry` (already created off `main`). Each commit goes through the sanctioned `evolve ship --class manual` path (commit-gate review) — code tasks get code-simplifier + go-reviewer; the docs task skips reviewers.
