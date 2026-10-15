# Comment history: `internal/phases/scout`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero h2

### `go/internal/phases/scout/domain_default_test.go:10` — above `func TestComposePrompt_RendersDomainDefaults(t *testing.T) {`

```text
// TestComposePrompt_RendersDomainDefaults — ADR-0099: the scout (whose report
// declares the kind and names the deliverable paths) sees the project default
// kind the orchestrator seeds from .evolve/domain.json and the registry's
// document root, so its prose never restates either; absent ⇒ byte-identical.
```

### `go/internal/phases/scout/lanescope_prompt_test.go:3` — above `import (`

```text
// Cycle-776 RED contract — fleet-lane-provisioning-split (residual slice).
// Cycle-766 pinned lane identity into Context["fleet_scope"] for every phase,
// but only triage RENDERS it into the LLM prompt (triage.go). Cycle-776's own
// run is the live proof of the gap: scout-prompt.txt carried no lane scope, so
// scout scouted three out-of-scope tasks and triage had to defer all of them.
//
// Contract encoded here (Builder implements, must NOT modify these tests):
//
//  1. RENDER: when the lane scope is present (Context["fleet_scope"] or the
//     typed envelope at enforce), scout's composed prompt carries a
//     fleet_scope directive line naming the assigned todo ids — scout must
//     scout ONLY within them.
//  2. ISOLATION (negative): each lane's prompt names only its OWN ids; the
//     other lane's ids must not appear (cycle-640 cross-lane drift).
//  3. GATE TEETH: a lane-scoped scout prompt instructs the Decision Trace to
//     echo the pinned goal_hash — without the echo the scout→triage
//     coherence gate (lanescope.go) fails open forever (cycle-776's own
//     scout omitted it and the gate never fired).
//  4. EDGE: no lane scope ⇒ no fleet_scope line (sequential cycles
//     byte-identical; guards against over-rendering).
```

### `go/internal/phases/scout/recent_outcomes_prompt_test.go:3` — above `import (`

```text
// Chronicle S3 RED contract (cycle-784, chronicle-s3-digest-wiring, task
// inject-recent-outcomes-prompts). At digest stage=enforce the orchestrator
// injects Context["recent_outcomes"] (the seeded recent-outcomes.md bytes);
// scout's ComposePrompt must render it as a context line APPENDED AFTER the
// existing stable-prefix lines (strategy/goal/challenge_token — cache-friendly
// ordering). When the key is absent or empty the composed prompt is
// byte-identical to today's output (the shadow-stage regression pin).
//
// Builder implements; must NOT modify these tests.
```

### `go/internal/phases/scout/scout.go:56` — above `strategy, goal, tok := req.Context["strategy"], req.Context["goal"], req.Context["challengeToken"]`

```text
// ADR-0050 §3.10 Slice 3: read strategy/goal/challengeToken from the typed
// envelope at enforce, the legacy Context map below it (byte-identical —
// Active() is false unless enforce, and the typed getters mirror these keys).
```

### `go/internal/phases/scout/scout.go:67` — above `if kind := req.Context[core.CtxKeyDeliverableKindDefault]; kind != "" {`

```text
// ADR-0099 slice 2: the project's default deliverable kind (.evolve/domain.json)
// — a task that declares no kind inherits it; scout writes the cycle's
// `deliverable_kind:` header line from the tasks it selects.
```

### `go/internal/phases/scout/scout.go:83` — above `if tok != "" {`

```text
// Cycle-135 fix (PR 6): plumb the cycle's challenge token into the
// Cycle Context block so scout doesn't have to mint its own. Per
// agent-templates.md PR 5, every phase report must include
// `<!-- challenge-token: <value> -->` on line 2 — the source-of-truth
// is the orchestrator-minted token, surfaced via
// Context["challengeToken"]. Scout was previously inventing tokens
// (cycle 134 audit C1: `no-token-manual-run-cycle-134`; cycle 135
// audit C1: `59576594e2e8d5c3` mint vs `5b96ecb69a0c848f` truth)
// when this line was absent. Pairs with the runner-side write of
// `<workspace>/challenge-token.txt` so the fallback source (per
// agent-templates.md PR 5 precedence step 2) is also populated.
```

### `go/internal/phases/scout/scout.go:97` — above `if scope := runner.LaneScope(req); scope != "" {`

```text
// Cycle-776 (fleet-lane-provisioning-split residual): cycle-766 pinned the
// lane scope into Context, but only triage rendered it — so scout scouted
// out-of-scope tasks (cycle-640/-776 incidents). Render the directive, and
// instruct the Decision Trace goal_hash echo so the scout→triage coherence
// gate (core/lanescope.go) can fire on a mismatch instead of failing open.
```

### `go/internal/phases/scout/scout.go:104` — above `if paths := req.Context["fleet_scope_paths"]; paths != "" {`

```text
// The LIVE record per assigned id (cycle-1548: a bare id name-searched
// into a consumed namesake from a halt cured two weeks earlier — 17
// records shared the id). The path alone is not enough; the directive
// must forbid the name-search it replaces, or the agent still greps.
```

### `go/internal/phases/scout/scout.go:124` — above `strategy := req.Context["strategy"]`

```text
// ADR-0050 §3.10 Slice 3: the strategy gate is verdict-affecting
// (convergence-confirmation → SKIPPED), so Classify reads the typed envelope at
// enforce too. Byte-identical below enforce (Active() == false).
```

### `go/internal/phases/scout/scout.go:155` — above `ContractVerifier func() runner.ContractVerifier`

```text
// ContractVerifier is the deliverables gate's verifier accessor for the
// verdict engine (runner.Options.ContractVerifier): one verifier for gate
// and engine (research F22). nil = the catalog-aware default.
```

### `go/internal/phases/scout/scout.go:161` — above `PhaseIO config.Stage`

```text
// PhaseIO threads the EVOLVE_PHASE_IO stage into the reconcile rung (ADR-0050
// §3.10 Slice 1). Zero value (StageOff) = byte-identical.
```

### `go/internal/phases/scout/scout_phaseio_test.go:11` — above `func TestScout_ComposePrompt_TypedVsMap_Identical(t *testing.T) {`

```text
// ADR-0050 §3.10 Slice 3: scout reads strategy/goal/challengeToken from the typed
// envelope at enforce (req.Input.Active()) and the legacy Context map below it. The
// strategy read is verdict-affecting (convergence-confirmation → SKIPPED), so both
// ComposePrompt AND Classify must consult the typed source.
```

### `go/internal/phases/scout/scout_test.go:255` — above `func TestComposePrompt_InjectsGoalTextFromContext(t *testing.T) {`

```text
// TestComposePrompt_InjectsGoalTextFromContext (cycle-108 bug #3): the
// dispatcher routes --goal-text into Context["goal"]; Scout's prompt
// must surface it so Scout treats the operator's goal as a constraint
// when choosing between backlog work and the meta-goal.
```

### `go/internal/phases/scout/scout_test.go:289` — above `func TestComposePrompt_InjectsChallengeTokenFromRequest(t *testing.T) {`

```text
// TestComposePrompt_InjectsChallengeTokenFromRequest (cycle-135 lesson):
// the auditor mandates `<!-- challenge-token: <value> -->` on line 2 of
// every phase report. PR 5 centralized the contract in agent-templates.md;
// PR 6 plumbs the actual token VALUE into scout's prompt via the
// PhaseRequest.ChallengeToken field already on the struct (ports.go:148).
// Without this fix, scout has to mint its own token or fall back to a
// placeholder — cycle 135 audit C1: scout used `59576594e2e8d5c3` instead
// of the actual `5b96ecb69a0c848f` from challenge-token.txt.
```

### `go/internal/phases/scout/scout_test.go:327` — above `func TestClassify_SelectedTasksHeading(t *testing.T) {`

```text
// TestClassify_SelectedTasksHeading — cycle-192: the scout template's backlog
// heading is "## Selected Tasks" with "### Task N:" subheadings; the classifier
// must accept it (and the legacy "## Proposed Tasks" + list-item shape) or every
// current scout report false-FAILs.
```
