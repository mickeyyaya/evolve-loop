# Comment history: `internal/core/advisor`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/core/advisor/advisor.go:1` — above `package advisor`

```text
// Package advisor is unit 04 of the component breakdown (ADR-0103): the phase
// advisor — the DynamicLLM brain behind router.Proposer and router.Planner.
// One Advisor composes the per-transition routing prompt and the whole-cycle
// plan prompt over router.RouteInput, launches the router persona through a
// leaf-owned Launcher port (core adapts its Bridge to it once), walks the
// router profile's CLI fallback chain, persists the redacted prompt/response
// capture and the OTel-GenAI decision span, and parses the strict-JSON
// proposal or plan with the mint recursion guard. Every output is ADVISORY:
// the pure router clamp re-validates it against the kernel floor, and any
// failure is returned as an error so the caller degrades to the static path.
// The Advisor holds its collaborators explicitly — the launcher, the identity,
// the capture writer, the profile loader, the recent-files reader, the depth
// guard, the overlay resolver and the Signal Center accessor — never runs git,
// never writes stderr, and reports its six failure modes as advisor.warning
// under module advisor. Design: docs/architecture/decomposition/04-advisor.md.
```

### `go/internal/core/advisor/advisor.go:70` — above `type Identity struct {`

```text
// Identity is the immutable dispatch identity shared by the control-plane
// advisors (the phase advisor, the failure advisor) — the fields that select
// WHICH llm brain answers, independent of the per-call operand (prompt /
// artifact file / completion contract, which vary Plan vs Propose vs Advise
// and stay per-call). It formalizes the byte-identical {cli,model,profile,
// persona} field set both advisors carried separately (ADR-0052 WS1-S1, Value
// Object): one home per identity belief, never two structs drifting apart.
// core.AgentIdentity is an alias of it.
//
// It is deliberately NOT the bridge-launch call itself — the advisors thread
// context differently (the phase advisor uses context.Background; the failure
// advisor threads the caller's ctx) — so only the field-set used to build the
// launch request is shared.
```

### `go/internal/core/advisor/advisor.go:141` — above `type DepthCheck func(env map[string]string) bool`

```text
// DepthCheck is the injectable recursion-depth guard (defense-in-depth,
// ADR-0052 §4.3): true for the dispatch env refuses the launch. nil skips it.
```

### `go/internal/core/advisor/advisor.go:249` — above `func (a *Advisor) Propose(in router.RouteInput) (*router.Proposal, error) {`

```text
// Propose implements router.Proposer — the per-transition "insert this
// optional phase?" advice under the ADR-0027 stdout completion contract.
```

### `go/internal/core/advisor/advisor.go:264` — above `func (a *Advisor) Plan(in router.RouteInput) (*router.PhasePlan, error) {`

```text
// Plan implements router.Planner: the upfront whole-cycle run/skip plan
// (ADR-0024 §2 hybrid cadence — the cheap, coherent upfront decision). The
// returned plan is ADVISORY; the kernel clamp re-validates it against the
// floor. It writes routing-plan.json and parses a JSON array; any failure
// returns an error so the caller degrades to the static path.
```

### `go/internal/core/advisor/advisor.go:273` — above `func (a *Advisor) RePlan(in router.RouteInput) (*router.PhasePlan, error) {`

```text
// RePlan is the post-scout re-plan (ADR-0052 WS1-S3): a SECOND whole-cycle
// plan computed once scout's handoff has populated in.Signals, so need is
// MEASURED rather than inferred from goal text. It shares plan with the
// initial Plan — same compose→dispatch→parse path — writes the DISTINCT
// routing-replan.json artifact and stamps replan_depth=1 on its decision
// span. The orchestrator calls it in shadow every cycle post-scout under the
// cfg.RouterReplan dial (cyclerun_replan.go); the plan is ADVISORY and the
// kernel clamp re-validates it exactly as it does the initial plan.
```

### `go/internal/core/advisor/advisor_test.go:3` — above `import (`

```text
// advisor_test.go — the construction contract (ADR-0103 unit 04 §6 tests 1,
// 2, 9, 38): the depth guard, the twelve error texts, the happy-path silence,
// the live Center accessor and its Null Object, the registered codes and the
// positional request shapes. Every export is named here or in a sibling file
// (apicover counts package-local tests only).
```

### `go/internal/core/advisor/capture.go:15` — above `type Span struct {`

```text
// Span is the OTel-GenAI decision span (ADR-0052 WS3-S3) persisted per
// advisor call as advisor-span-<kind>.json. Field keys follow the OTel GenAI
// semantic conventions so a collector can ingest the file directly. PromptSHA/
// ResponseSHA bind the REDACTED capture artifacts — the same identity the
// ledger (WS3-S2) and the replay path (WS3-S5) key off, so all three agree.
// ReplanDepth (WS1-S3) varies: 0 for the initial Plan, 1 for the post-scout
// RePlan — so it records real behavior, not locked surface. It is always
// emitted (no omitempty): a depth of 0 is a meaningful "initial plan", not
// absence. core.AdvisorSpan is an alias of it.
```

### `go/internal/core/advisor/capture_test.go:3` — above `import (`

```text
// capture_test.go — the redacted capture (ADR-0103 unit 04 §6 tests 3, 20,
// 21; the core capture/span tests moved verbatim in intent).
```

### `go/internal/core/advisor/catalog.go:27` — above `func WriteCatalogWithOnDemand(b *strings.Builder, cards []router.PhaseCard, onDemand []string) {`

```text
// WriteCatalogWithOnDemand renders the SELECT menu, biasing toward reuse over
// minting: a selectable phase already has a tuned persona + profile, so
// minting should be the exception (YAGNI for new phases). Cards carry the
// spec's advisor-facing metadata (ADR-0038); relevance judgment is the
// advisor LLM's job — Go only bounds the token cost. When the catalog exceeds
// the enriched cap, Optional (SELECTable) phases take the enriched slots —
// spine phases run via the mandatory config regardless. Deterministic order
// (catalog order, stable partition) ⇒ prompt-prefix-cache friendly. Emits
// nothing when the catalog is empty (legacy built-in-only path). The phases
// that declined a slot are then named in a single line.
//
// The index is the difference between HIDING a phase and REMOVING it. Declining
// exists so 53 never-selected cards stop crowding out 12 enriched slots; if the
// declined set then vanished from the prompt entirely, the advisor could not
// learn those phases exist and the fix would trade one invisibility defect for
// another — the exact class this repo has spent the week removing.
```

### `go/internal/core/advisor/catalog.go:111` — above `if len(c.AllowedCLIs) > 0 {`

```text
// Project this phase's own dispatch guardrails (cycle-436 MR1) so an
// advisor proposing {cli,tier} for it has the legal bounds in hand instead
// of guessing blind. Omitted entirely when the phase carries no per-phase
// guardrail (the common case today).
```

### `go/internal/core/advisor/catalog_test.go:3` — above `import (`

```text
// catalog_test.go — the SELECT menu (ADR-0103 unit 04 §6 test 32; the core
// on-demand index tests moved verbatim in intent; the ACS-named overflow
// tests stay in core over the facade).
```

### `go/internal/core/advisor/context.go:58` — above `func writeCLIHealth(b *strings.Builder, in router.RouteInput) {`

```text
// writeCLIHealth renders the environmental CLI health: benched families mean
// dispatch chains start at their fallback — the advisor should plan around the
// degraded family (fewer inserts routed there; scope sized for the fallback
// carrying the cycle) instead of discovering it one phase at a time
// (cycle-283). Family-sorted; a quota wall is named WALLED/unavailable.
```

### `go/internal/core/advisor/decision.go:51` — above `func (d decision) completion() string { return decisionRows[d].completion }`

```text
// completion is the bridge completion contract: every decision uses the
// uniform artifact contract — the brain WRITES its artifact and the bridge
// reads it back. The proposal completed on REPL-idle stdout (ADR-0027) until
// 2026-09-14: its prompt carries the same deliverable contract as the plans
// ("write routing-proposal.json"), so the model wrote the file while the
// kernel read the scrollback, found only the prompt's echoed JSON example,
// and raised ADVISOR_RESPONSE_UNPARSEABLE on every proposal (cycles
// 1673–1677; docs/incidents/2026-09-14-router-proposal-read-the-scrollback.md).
```

### `go/internal/core/advisor/fixture_test.go:3` — above `import (`

```text
// fixture_test.go — the ONE rich RouteInput the unit-04 goldens
// (ADR-0103, docs/architecture/decomposition/04-advisor.md §6 step 1) were
// captured over on the pre-extraction code and are replayed through the leaf:
// every prompt section rendered at once — a 13-card catalog spanning the
// three enrichment buckets, three on-demand names, 23 carryover todos across
// every priority spelling with one 700-rune action, two benches (one walled),
// recall memory, two unavailable phases, conditional rules + triggers +
// rubric hints, a 4100-rune goal and all four signal blocks.
```

### `go/internal/core/advisor/golden_test.go:3` — above `import (`

```text
// golden_test.go — the goldens captured on 8e8f080f (the pre-extraction
// code) replayed through the leaf: every prompt shape, the launch request per
// decision, the capture artifacts (ADR-0103 unit 04 §6 tests 4-8).
```

### `go/internal/core/advisor/importgraph_test.go:3` — above `import (`

```text
// importgraph_test.go — the package is a leaf under core (ADR-0103 unit 04
// §2): stdlib plus the fourteen named internal packages, never internal/core
// itself, clihealth, gitexec or the carryover lifecycle (the compiler is the
// cycle guard; this is the leaf-ness declaration — signalcenter/
// importgraph_test.go idiom).
```

### `go/internal/core/advisor/launch.go:38` — above `func (a *Advisor) preflight(in router.RouteInput, d decision) error {`

```text
// preflight refuses a launch that cannot proceed: no launcher, no workspace,
// or the WS1-S2 recursion guard (defense-in-depth, ADR-0052 §4.3 — the
// PRIMARY guard is the mint denylist in MintConfigsFrom) signalling a nested
// invocation, degrading the cycle to the static path rather than nesting
// brains.
```

### `go/internal/core/advisor/launch_test.go:3` — above `import (`

```text
// launch_test.go — the launch: preflight order, the fourteen threaded
// fields, the router-profile fallback chain and its faults, the skill
// overlays, the profile path (ADR-0103 unit 04 §6 tests 13-19, 27, 28; the
// core launch/fallback/amplify/skilloverlay/replan tests moved verbatim in
// intent against the exported spellings).
```

### `go/internal/core/advisor/launch_test.go:173` — above `func TestLaunch_WalksTheProfileFallbackChainAndReportsExhaustion(t *testing.T) {`

```text
// Test 16 — the fallback chain walk (the cycle-435 class): a trigger exit
// advances to the profile's fallback, every candidate flows through the
// port, exhaustion is one dispatch signal naming the chain, a non-trigger
// exit never reroutes, and an absent or explicit-empty cli_fallback
// dispatches exactly once.
```

### `go/internal/core/advisor/limits_test.go:3` — above `import (`

```text
// limits_test.go — the clean-code limits the design promises (ADR-0103 unit
// 04 §4), enforced by a test rather than by review: every function < 50
// lines, nesting depth ≤ 4, every file < 800 lines (signalcenter/limits_test.go
// idiom; comments inside a function count, its doc comment does not).
```

### `go/internal/core/advisor/mint.go:12` — above `var reservedNames = map[string]struct{}{`

```text
// reservedNames are the control-plane identities a minted phase may never
// assume — the WS1-S2 recursion guard (PRIMARY). The advisor composes the
// executed spine; minting a router/advisor would let a brain schedule a brain,
// breaking the compose-vs-execute layering (ADR-0052 D1). The AgentLabel values
// the advisors dispatch under ("router"/"failure-advisor") are the canonical
// members; aliases cover the persona slug and bare role words.
```

### `go/internal/core/advisor/mint_test.go:3` — above `import (`

```text
// mint_test.go — the recursion guard and the mint drops reported at decision
// time (ADR-0103 unit 04 §6 tests 25, 26; the core mint tests moved verbatim
// in intent through Plan).
```

### `go/internal/core/advisor/mint_test.go:15` — above `func TestMintConfigsFrom_RejectsAdvisorRoleMint(t *testing.T) {`

```text
// The WS1-S2 recursion guard (ADR-0052 D1, primary defense): a mint whose
// name is a reserved control-plane identity is dropped with an observable
// reason; legitimate mints pass through untouched.
```

### `go/internal/core/advisor/parse.go:40` — above `func ParseProposal(stdout string) (*router.Proposal, error) {`

````text
// ParseProposal extracts the strict-JSON proposal from the response the
// bridge read back — since 2026-09-14 the routing-proposal.json artifact's
// content (a bare object), before that the REPL scrollback, which echoed the
// PROMPT and its JSON example. The LAST balanced object is taken either way
// (an answer is last; a prompt echo is not), tolerant of a ```json fence /
// surrounding prose. Empty/unparseable → error (caller degrades to static).
// PURE and Center-free.
````

### `go/internal/core/advisor/parse.go:78` — above `func ParsePhasePlan(stdout string) (ParsedPlan, error) {`

```text
// ParsePhasePlan extracts the strict-JSON whole-cycle plan from the LLM stdout.
// The wire format is a bare array of {phase, run, justification}; like
// ParseProposal it takes the LAST balanced array so the prompt's echoed JSON
// example (present in the captured scrollback under the ADR-0027 stdout
// contract) is not mistaken for the answer. An empty or unparseable body is an
// error (caller degrades to the deterministic static plan). PURE and
// Center-free.
```

### `go/internal/core/advisor/parse_test.go:3` — above `import (`

```text
// parse_test.go — the pure parsers (ADR-0103 unit 04 §6 tests 22-24, 26, 36,
// 37; the core scrollback/failure/replay/tier tests moved verbatim in intent).
```

### `go/internal/core/advisor/prompt.go:64` — above `func buildPlanPrompt(in router.RouteInput) string {`

```text
// buildPlanPrompt renders the WHOLE-CYCLE planning context (ADR-0024 §2): the
// same objective digest + rubric as buildRoutingPrompt, but it asks the advisor
// to decide run/skip for EVERY phase of the cycle in one coherent pass, as a
// strict-JSON array. The plan is advisory — the kernel clamp re-validates it.
// It is the legacy inline framing ComposePlanPrompt falls back to without a
// persona.
```

### `go/internal/core/advisor/prompt.go:86` — above `func writePlanResponseSchema(b *strings.Builder) {`

```text
// writePlanResponseSchema renders the whole-cycle plan's response contract —
// the optional MINT block, the optional per-phase {cli,tier} dispatch
// proposal with the operator's model-tier policy, and the strict-JSON
// example — shared by ComposePlanPrompt (persona path, PRODUCTION) and
// buildPlanPrompt (legacy fallback), so the two prompt-assembly paths can
// never diverge again the way they did at #293 (the persona path never
// called this section at all).
```

### `go/internal/core/advisor/prompt.go:118` — above `func (a *Advisor) ComposePlanPrompt(in router.RouteInput, artifactFile string) string {`

```text
// ComposePlanPrompt builds the whole-cycle planning prompt the uniform way: the
// persona body (agents/evolve-router.md — identity, job, mint guidance, output
// contract) followed by the DYNAMIC per-cycle context (objective digest, recall
// memory, catalog, decision rubric) appended in Go, exactly as a phase appends
// its cycle context. When no persona was injected it falls back to the legacy
// fully-inline framing (buildPlanPrompt) so the advisor still functions.
// artifactFile is the raw plan artifact the prompt instructs the model to
// write — the ABSOLUTE workspace path the launch also tells the bridge to
// watch (a relative path lands in the REPL's cwd, which under claude-tmux is
// NOT the workspace, so the bridge never sees it — the cycle-210 failure).
// It is the exported entry the seam's test facade reaches with an arbitrary
// artifact name, so it does the ONE string→decision map for the compose-time
// stamp; production (Plan/RePlan) composes from its decision directly.
```

### `go/internal/core/advisor/prompt.go:157` — above `func (a *Advisor) gatherRecon(in router.RouteInput, d decision) router.ReconDigest {`

```text
// gatherRecon collects the deterministic pre-plan recon (ADR-0052 WS2-S0b)
// through the injected recent-files reader (core's git reader in production;
// the Null Object in a git-less environment) and FAILS OPEN: a reader error
// is one ADVISOR_RECON_GIT_FAILED and yields no changed files, so
// router.BuildReconDigest simply omits the file-derived facts. The backlog/
// carryover/goal facts come from the already-threaded RouteInput, so they
// survive regardless. An empty project root reads nothing (configuration,
// not a fault).
```

### `go/internal/core/advisor/prompt_test.go:3` — above `import (`

```text
// prompt_test.go — the prompt composers and the routing context (ADR-0103
// unit 04 §6 tests 29, 33-35; the core rubric/failure/deliverable-kind/
// recall/clihealth/persona/absolute-path/mint-documentation tests moved
// verbatim in intent).
```

### `go/internal/core/advisor/todos.go:51` — above `ordered := append([]router.CarryoverTodo(nil), todos...)`

```text
// When the array exceeds the count cap, render the HIGHEST-PRIORITY /
// MOST-RECENT entries rather than a naive insertion-order (oldest-first)
// prefix — the old todos[:20] silently hid the newest, most severe items
// (e.g. cycle-505's leak) behind "N omitted". Sort a COPY (stable, so ties
// keep on-disk order) — never mutate the caller's slice.
```

### `go/internal/core/advisor/todos_test.go:3` — above `import (`

```text
// todos_test.go — the carryover-todo section (ADR-0103 unit 04 §6 tests 30,
// 31; the ACS-named length/order tests stay in core over the facade).
```

### `go/internal/core/advisor/todos_test.go:44` — above `if !strings.HasPrefix(lines[0], "- [P0] cycle-40-todo-00:") || !strings.HasPrefix(lines[1], "- [P0] cycle-39-todo-21:") …`

```text
// The four P0/p0 todos lead (rank 6), most recent first; then the P1/H/HIGH
// block (rank 5) in which cycle-40-todo-01 and cycle-39-todo-02 keep their
// on-disk order among equal cycles.
```
