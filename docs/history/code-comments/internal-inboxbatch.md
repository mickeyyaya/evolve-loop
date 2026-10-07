# Comment history: `internal/inboxbatch`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/inboxbatch/classify.go:56` — above `clusterCampaign := make([]string, len(items))`

```text
// Campaign is a PARTITION, not a signal: the operator's campaign field is
// the explicit "this is one initiative" declaration, so an inferred edge
// (file-area, dep, connects) must never merge two DISTINCT non-empty
// campaigns — measured live (2026-07-28), area chains fused 17 campaigns
// into one ~37-item cluster serialized over 9 "run the previous batch
// first" batches. clusterCampaign tracks each ROOT's campaign claim so the
// guard holds transitively too: a campaign-less item may join a campaign's
// cluster, but can never become the bridge that unions two campaigns. A
// blocked cross-campaign dep still executes in order — the dispatch
// freshness gate holds a lane until its deps land; only co-batching is
// prevented.
```

### `go/internal/inboxbatch/classify.go:106` — above `type clusterOut struct {`

```text
// Rank clusters as UNITS (go-reviewer HIGH, 2026-07-16): a flat per-chunk
// weight sort let a continuation outrank its own predecessor whenever a
// low-weight dep chain gated a high-weight item past the chunk boundary —
// the render's "run the previous batch first" note then pointed nowhere.
// Sorting whole clusters by their max member weight keeps chunks adjacent
// and predecessor-first BY CONSTRUCTION, and ranks the cluster where its
// most urgent item deserves.
```

### `go/internal/inboxbatch/classify_campaign_partition_test.go:3` — above `import (`

```text
// classify_campaign_partition_test.go — campaign is a PARTITION, not a signal.
// The operator's campaign field is the explicit "this is one initiative"
// declaration; an inferred file-area or dep edge must never merge two DISTINCT
// non-empty campaigns into one cluster. Measured on the live 84-item backlog
// (2026-07-28): a chain of small shared areas fused 17 campaigns into one
// ~37-item cluster rendered as batches 1-9, each marked "run the previous
// batch first" — a 9-cycle serialized chain over unrelated initiatives.
```

### `go/internal/inboxbatch/classify_test.go:177` — above `func TestClassify_ContinuationNeverOutranksPredecessor(t *testing.T) {`

```text
// TestClassify_ContinuationNeverOutranksPredecessor — go-reviewer HIGH
// regression (2026-07-16, their exact counter-example): a long LOW-weight dep
// chain gating one HIGH-weight item pushes it into a continuation chunk; the
// old flat weight sort ranked that continuation ABOVE its own predecessor.
// Batches now rank as CLUSTER UNITS (cluster max weight), chunks staying
// adjacent and predecessor-first — so the render's "run the previous batch
// first" note always points at the line above.
```

### `go/internal/inboxbatch/consoleroute.go:3` — above `import "strings"`

```text
// consoleroute.go — ADR-0074 I1: routing authority is typed plumbing, not
// prose. An inbox item is either lane-dispatchable or console-routed
// (operator-owned); this file is the ONE classifier every consumer shares —
// the plan-time gate (fleet.TodosFromTriage via RoutedResolver), triage
// prompt composition (advisory visibility), and inboxmover.Claim (handoff).
// Born from cycles 1034/1035/1036 (2026-07-22): one wave burned three
// pipelines on tasks whose fix surface is the ProtectedSurfaceManifest, which
// a cycle structurally cannot write; the routing existed only as annotations
// nothing consumed.
```

### `go/internal/inboxbatch/consoleroute.go:26` — above `const pipelineKindPrefix = "pipeline-"`

```text
// pipelineKindPrefix marks pipeline-integrity work (pipeline-repair,
// pipeline-integrity, …): fixed hands-on in the console by operator policy
// (ADR-0072 halt autofiles this kind), so the kind alone routes the item out
// of lane reach — no files[] entry required. Provenance: wave 6, cycle 1688
// (2026-09-15) surfaced 15 lane-eligible pipeline-* items (8 with no files[]
// at all); one was claimed and failed triage before the breaker caught it.
```

### `go/internal/inboxbatch/consoleroute.go:34` — above `const KindPipelineRepair = pipelineKindPrefix + "repair"`

```text
// KindPipelineRepair is the kind the ADR-0072 halt autofiles for the operator
// (cmd_loop_escalation.go) — the one symbol the writer and this classifier
// share, so the two can never drift apart.
```

### `go/internal/inboxbatch/consoleroute.go:39` — above `func ConsoleRouted(it Item, isProtected func(string) bool) (bool, string) {`

```text
// ConsoleRouted reports whether the item is operator-owned (not lane-
// dispatchable) and why. isProtected is the control-plane SCOPE predicate the
// routing roots inject (guards.IsProtectedScope — a path that is, or a
// directory that contains, protected surface; nil disables only the derived
// surface rules — the explicit route field and the kind still apply).
//
// Precedence: route "console-*" always routes; a declared protected FILE (a
// file spelling the scope predicate judges protected — for a file spelling
// scope IS membership) always routes — triage's breaker and the ship tripwire
// judge membership of each file with no route exception, so no override can
// make it lane work, only a doomed lane (F35: two live operator overrides,
// 2026-09-26). Residual (F35c): a declared DIRECTORY that is itself inside a
// protected directory fragment ("go/internal/bridge/") still reads as scope
// here and stays overridable — binding it needs membership injected beside
// scope. Any other
// derivation (a pipeline-* kind, a protected directory scope, a mention)
// routes unless route:"lane" AND the item is operator-authored (InjectedBy
// empty) — agent-autofiled items cannot widen agent authority by
// self-declaring lane dispatch of control-plane work (ADR-0073 clamp-parity:
// the field is unauthenticated, so the achievable floor is that an
// agent-authored override never *widens* what an agent may do).
//
// The surface: every whitespace token of each files[] entry (real items write
// "path (why)" and "(see path)" shapes) is judged in scope — a declared
// directory holding protected files routes (F29); with no DECLARED surface
// (Item.DeclaredSurface: no path-shaped token), the files the record's own
// text names are judged instead. Only surfaces ALREADY on the manifest match —
// a task that will CREATE a new gate-shaped file is caught later by the ship
// tripwire + disposition handoff, not here.
```

### `go/internal/inboxbatch/consoleroute.go:134` — above `for _, p := range it.mentions {`

```text
// F29: with no declared surface, the FILES the record's own text names ARE
// its surface — the one triage's breaker would otherwise derive only after
// a lane paid for scout and triage (18 of ~60 cycles died that way). A
// mention can only move an item TO the console, never widen lane authority,
// and it is a file spelling, so the injected scope predicate reduces to
// membership for it. It never binds: the text may only cite the file.
```

### `go/internal/inboxbatch/consoleroute.go:165` — above `func RoutedResolver(dir string, isProtected func(string) bool) func(id string) (bool, string) {`

```text
// RoutedResolver loads dir once and returns the id→(routed, reason) closure
// the plan-time gate (fleet.TodosFromTriage) consumes. Unknown ids are
// dispatchable — scout-originated work has no inbox item and must never be
// blocked. Construct per wave so mid-batch inbox changes are seen fresh; a
// load failure resolves everything dispatchable (fail-open like LoadDir: a
// broken backlog must not stop the queue — ADR-0072 never-stop).
```

### `go/internal/inboxbatch/consoleroute_parity_test.go:8` — above `func TestConsoleRouted_LaneOverrideCannotRelaxAProtectedFile(t *testing.T) {`

```text
// consoleroute_parity_test.go — F35 (2026-09-26): the seed refuses at least
// everything triage's breaker would. The breaker and the ship tripwire judge
// MEMBERSHIP of each file with no route exception, so two spellings let a
// declared protected FILE through the seed classifier only for a lane to die
// at triage after paying for scout: an operator route:"lane" on a declared
// protected file (two live items on 2026-09-26, reduced below to their routing
// fields), and a path:line locator ("x.go:178") the path-shape check refused
// to read as a path at all. These tests use the real routing predicate.
```

### `go/internal/inboxbatch/consoleroute_revise_test.go:3` — above `import (`

```text
// consoleroute_revise_test.go — RED contracts for the architect-review
// revisions (ADR-0074): (F4) route:"lane" override provenance clamp — an
// agent-autofiled item may not override a protected derivation; (F5) the
// derivation scans ALL tokens of a files entry, not just the first; plus the
// id-resolver the plan-time gate (fleet.TodosFromTriage) consumes.
```

### `go/internal/inboxbatch/consoleroute_revise_test.go:15` — above `func TestConsoleRouted_AutofiledLaneOverrideIgnored(t *testing.T) {`

```text
// F4: autofiled provenance (injected_by set) ignores the lane override when a
// protected surface is declared — clamp-parity with ADR-0073: agent-authored
// fields cannot widen agent authority; only operator-authored items may force
// lane dispatch of a protected-surface task.
```

### `go/internal/inboxbatch/consoleroute_surface_test.go:12` — above `func decode(t *testing.T, record string) inboxbatch.Item {`

```text
// consoleroute_surface_test.go — F29: the seed-time console classifier sees the
// surface triage's breaker later checks. 18 of the ~60 cycles before 2026-09-26
// died at triage with TRIAGE_PROTECTED_SURFACE after a full scout+triage spend;
// every one traced to five inbox items whose protected surface was either the
// kind (F25), a declared DIRECTORY holding protected files, or — with no
// declared files[] — a protected file the item's own text names. The routing
// roots inject the SCOPE projection (guards.IsProtectedScope); these tests use
// the real predicate, not a stub.
```

### `go/internal/inboxbatch/consoleroute_surface_test.go:88` — above `func TestConsoleRouted_DirectoryMentionsAreContextNotSurface(t *testing.T) {`

```text
// TestConsoleRouted_DirectoryMentionsAreContextNotSurface (architecture review
// F29): prose naming a DIRECTORY or package names no file a lane would change —
// routing on it would over-route ordinary work ("the stall shows in
// go/internal/core", an import path, a checkout root ending in /go).
```

### `go/internal/inboxbatch/consoleroute_surface_test.go:118` — above `func TestConsoleRouted_PlaceholdersDeclareNothing(t *testing.T) {`

```text
// TestConsoleRouted_PlaceholdersDeclareNothing (go review F29 MAJOR): a files[]
// holding a placeholder or a bare file name declares no surface, so the text
// is still read — "TBD" must not switch the derivation off.
```

### `go/internal/inboxbatch/consoleroute_surface_test.go:147` — above `func TestConsoleRouted_MentionDerivationKeepsTheClamp(t *testing.T) {`

```text
// TestConsoleRouted_MentionDerivationKeepsTheClamp: the ADR-0073 clamp holds
// for the derivation — an operator-authored route:"lane" is honored, an
// agent-autofiled one is ignored — and a nil predicate disables it (the
// hasClaimableInboxWork posture), exactly like the files rule.
```

### `go/internal/inboxbatch/consoleroute_surface_test.go:164` — above `func TestConsoleRouted_AnnotationWordsAreNotSurface(t *testing.T) {`

```text
// TestConsoleRouted_AnnotationWordsAreNotSurface (architecture review F29): only
// the path-shaped files[] tokens are the declared surface. An annotation word
// ("(go test)" yields "go") read in scope would name /go/ — a prefix of
// /go/acs/regression/ — and console-route ordinary lane work; a path-shaped
// top-level directory ("go/", "skills/audit") is still judged in scope.
```

### `go/internal/inboxbatch/consoleroute_test.go:3` — above `import (`

```text
// consoleroute_test.go — RED contract for ADR-0074 I1 (typed routing authority).
// Cycles 1034/1035/1036 (2026-07-22, one wave) each burned a full pipeline on a
// task whose fix surface is the pipeline's own control plane
// (ProtectedSurfaceManifest paths a cycle may not write). The routing signal
// existed only as prose annotations no component consumed. This file pins the
// SINGLE deterministic classifier both consumers (triage prompt composition and
// inboxmover.Claim) must share: route-field plumbing plus a protected-fix-surface
// derivation, with an explicit operator override.
```

### `go/internal/inboxbatch/consoleroute_test.go:42` — above `func TestConsoleRouted_ProtectedFilesAutoRoute(t *testing.T) {`

```text
// Derived form: any declared fix-surface file on the protected manifest routes
// the item out of lane reach even without a route field — the statically
// blocked class (1035 guard-phase-hook, 1036 role.go, cycle-858 policy.json).
```

### `go/internal/inboxbatch/consoleroute_test.go:67` — above `func TestConsoleRouted_ExplicitLaneOverrideWins(t *testing.T) {`

```text
// route:"lane" is the operator override for a heuristic's false positives — a
// declared directory that only holds protected files is lane work when the
// change avoids them. It cannot relax a declared protected FILE (F35): the
// breaker and the tripwire refuse that whatever the route; an item that only
// READS the surface declares the files it changes, not the ones it reads.
```

### `go/internal/inboxbatch/consoleroute_test.go:135` — above `func TestConsoleRouted_PipelineKindRoutesConsole(t *testing.T) {`

```text
// Kind form (wave 6, cycle 1688, 2026-09-15): a `pipeline-*` item is
// pipeline-integrity work, which the operator owns (pipeline_fixes_console_first;
// the wave goal names it console-owned). The queue held 15 such items lane-eligible,
// 8 with no files[] list at all, so the files-derived rule had nothing to match:
// a lane claimed one, scout and triage ran, and the triage breaker refused the
// card for naming a protected surface — a full FAIL seal for work no lane may do.
// The kind is a first-class field the classifier must read, not prose.
```

### `go/internal/inboxbatch/consoleroute_test.go:168` — above `func TestConsoleRouted_PipelineKindHonorsOperatorLaneOverrideOnly(t *testing.T) {`

```text
// The operator's explicit route:"lane" override applies to the kind rule
// exactly as it applies to the files rule: honored for an operator-authored
// item, ignored for an agent-autofiled one (the ADR-0072 halt escalation
// autofiles pipeline-repair items — those stay console-owned however they
// are annotated).
```

### `go/internal/inboxbatch/continuation_test.go:9` — above `func TestItem_ContinuationRoundTrip(t *testing.T) {`

```text
// continuation_test.go — ADR-0076 slice C schema: a FAILed cycle with
// salvageable preserved work stamps its item with a continuation binding; the
// next claim adopts it instead of restarting cold. The field must round-trip
// tolerantly through LoadDir (absent = nil, never an error).
```

### `go/internal/inboxbatch/cycledirs_test.go:9` — above `func TestCycleDirs_ListsCycleSubdirsAscendingAndTolerantOfAbsence(t *testing.T) {`

```text
// CycleDirs is the ONE scan for every lifecycle directory the promoter nests
// by cycle (processing/, processed/, rejected/): ascending cycle order, files
// and non-cycle names ignored, a missing parent an empty list — the layout
// the dispatch-state resolver was blind to when cycle 1682 re-pinned a lane
// to an item cycle 1679 had already shipped.
```

### `go/internal/inboxbatch/deliverable_kind_test.go:9` — above `func TestLoadFile_DeliverableKind(t *testing.T) {`

```text
// TestLoadFile_DeliverableKind — ADR-0099 slice 2: an inbox item declares the
// kind of thing it wants built; the harness projects it into the Task Contract
// block (the same single-source discipline as acceptance[]).
```

### `go/internal/inboxbatch/item.go:35` — above `Class      string   'json:"class"'`

```text
// Class is the item's declared archetype ("pipeline-architecture",
// "task-contract-design", …). Authors have been writing it into inbox JSON
// for a while; it was silently dropped at load until cycle-1190. It is the
// routing signal downstream archetype detectors key off (IsOperatorState).
```

### `go/internal/inboxbatch/item.go:45` — above `Route string 'json:"route"'`

```text
// Route is the ADR-0074 dispatch-authority field: "console-*" values mark
// the item operator-owned (never lane-dispatchable), "lane" is the explicit
// override for protected-files false positives. Empty = derive (see
// ConsoleRouted).
```

### `go/internal/inboxbatch/item.go:50` — above `InjectedBy string 'json:"injected_by"'`

```text
// InjectedBy carries autofile provenance (retrofile, chronicle-escalation,
// …). ADR-0074 clamp: an agent-autofiled item may NOT lane-override a
// protected-surface derivation — agent-authored fields cannot widen agent
// authority (ADR-0073 clamp-parity vocabulary).
```

### `go/internal/inboxbatch/item.go:55` — above `Continuation *continuation.Continuation 'json:"continuation,omitempty"'`

```text
// Continuation (ADR-0076 slice C) binds a FAILed cycle's preserved,
// snapshot-committed work to this item so the next attempt resumes instead
// of restarting cold. Machine-consumed only (never rendered into the triage
// prompt); validated at adoption time, tolerant here. Nil = fresh start.
```

### `go/internal/inboxbatch/item.go:60` — above `Acceptance []string 'json:"acceptance,omitempty"'`

```text
// Acceptance is the item's verbatim acceptance criteria. It is the SINGLE
// source the harness projects into the tdd, build and audit prompts' Task Contract block
// (ADR-0098) — never re-typed by an agent, so the builder and the auditor
// grade against the same words.
```

### `go/internal/inboxbatch/item.go:65` — above `DeliverableKind string 'json:"deliverable_kind,omitempty"'`

```text
// DeliverableKind is what the item wants built — "code" (default) or
// "document" (ADR-0099: a solutions/<id>/ deliverable with candidate options
// and a recommendation). Projected into the Task Contract block; the cycle's
// authoritative kind is what triage declares in its report header.
```

### `go/internal/inboxbatch/item.go:73` — above `mentions []string`

```text
// mentions are the FILES the record's own text names, derived at decode
// (UnmarshalJSON). With no declared surface they are the item's surface for
// the console classifier (F29) — the surface triage's breaker would
// otherwise derive only after a lane has paid for scout and triage.
```

### `go/internal/inboxbatch/item.go:102` — above `func (it Item) DeclaredSurface() bool {`

```text
// DeclaredSurface reports whether the item DECLARES its fix surface: at least
// one files[] token shaped like a repo path (a slash-separated path). It is the
// ONE home of that belief — the console classifier's "a declared surface wins"
// rule and the seed's admissibility tie-break both read it (F29) — and a
// placeholder ("TBD", "N/A", "()") or a bare file name ("role.go", which the
// triage LLM would resolve into the tree) declares nothing.
```

### `go/internal/inboxbatch/item.go:112` — above `func declaredTokens(files []string) []string {`

```text
// declaredTokens is the ONE token set a declared surface consists of: the
// path-shaped files[] tokens. The console classifier judges exactly these in
// scope and DeclaredSurface asks whether any exist, so an annotation word
// ("(go test)" yields "go") is never read as a directory (F29 review).
```

### `go/internal/inboxbatch/item.go:141` — above `var lineLocatorRE = regexp.MustCompile('(?::\d+(?:[-,:]\d+)*|#L\d+(?:-L?\d+)?)$')`

```text
// lineLocatorRE matches the source locator authors append when citing a file
// ("a.go:178", "a.go:178:5", "a.go:189,205,221", "a.go:10-20", "a.go#L10-L20")
// — any trailing ":<digits>" run joined by "-", "," or ":", so a date-shaped suffix
// ("x.md:2026-09-26") strips too, equally not part of the path. It is how the
// file is cited, not part of its path — kept, the ":" failed the path shape
// and the file went unjudged while triage's breaker still matched it (F35).
```

### `go/internal/inboxbatch/layout.go:39` — above `func ParseProcessingCycle(dirName string) (cycle int, ok bool) {`

```text
// ParseProcessingCycle inverts ProcessingCycleDir's basename: the claiming
// cycle of a "cycle-<N>" directory name, or ok=false for anything else
// (notes, stray files, a name with extra segments, and "cycle-0": cycles start
// at 1 and 0 is the "pending at the root" sentinel of inboxmover.Location).
```

### `go/internal/inboxbatch/layout_test.go:33` — above `func TestParseProcessingCycle_RejectsZero(t *testing.T) {`

```text
// cycles start at 1 (core/alloc.go), and Location.Cycle == 0 means "pending at
// the root" — so "cycle-0" must never parse as a claim, and Claim must refuse
// to write it.
```

### `go/internal/inboxbatch/rules.go:16` — above `type Rule interface {`

```text
// Rule is one grouping signal (Strategy). Rules are independent and
// composable: Classify unions the edges of every rule it is given. Adding a
// future signal (e.g. title-token similarity) is a new Rule, not a rewrite.
// Single-method by design (go-reviewer 2026-07-16): each Edge carries its own
// human-readable Reason, so a separate rule-name accessor was vestigial.
```

### `go/internal/inboxbatch/rules.go:26` — above `func DefaultRules() []Rule {`

```text
// DefaultRules is the compiled rule set: the STRONG structural signals —
// campaign, package area, and hard dependency edges. connects_to is
// deliberately excluded: it is prose navigation ("link liberally" is the inbox
// house style) and real-backlog validation showed its transitive closure
// chains half the backlog into one mega-cluster; opt in via ConnectsRule when
// a tighter backlog warrants it. Order is presentation-only (edges union).
//
// root_cause is excluded PERMANENTLY, and not for the connects_to reason (too
// many edges) but the opposite one: it yields zero edges. An exact-match rule
// on that field was built and reverted (cycle-1204; .evolve/state.json
// failedApproaches[54]). Measuring the real backlog settled it: all 20 non-empty
// root_cause values were unique free-form prose, 122-1564 bytes, no two alike —
// unlike campaign/dep ids, which are short reusable keys, root_cause is a
// per-defect narrative, structurally a sibling of notes/evidence. Equality
// binding therefore emits no edges on real data, and normalizing or fuzzing the
// match only manufactures false ones, since the values describe genuinely
// unrelated defects. Do not re-add it.
```

### `go/internal/inboxbatch/rules.go:86` — above `const minAreaDepth = 2`

```text
// minAreaDepth is the discriminative FLOOR — the same argument as
// hubAreaMaxItems applied from below. A path whose directory is a single
// top-level segment ("agents", "skills", "go") names a BAG of unrelated files,
// not a unit of work: one worktree, one build and one audit cannot meaningfully
// carry "everything that touches agents/". Such an area binds nothing.
//
// Without this floor, every persona file in the repo collapsed to the area
// "agents" and every top-level skill file to "skills". Measured on the 84-item
// backlog (2026-07-27), two "agents" edges chained three unrelated campaigns
// (chronicle-2026-07 <-> pipeline-integrity <-> convergence-2026-07) into a
// single 43-item cluster — 51% of the backlog, chunked into 11 batches each
// marked "run the previous batch first". These shallow areas slip UNDER
// hubAreaMaxItems, so the ceiling alone never caught them.
```

### `go/internal/inboxbatch/rules_area_floor_test.go:3` — above `import "testing"`

```text
// rules_area_floor_test.go — the discriminative FLOOR on file-area grouping.
//
// fileAreaRule already has a discriminative CEILING (hubAreaMaxItems): an area
// referenced by more than 5 items binds nothing, because "go/internal/core
// appears in half the real backlog" is not a signal. The same argument applies
// from below and was missing: fileArea caps at areaDepth=3 segments but had no
// minimum, so every persona file in the repo collapsed to the single area
// "agents" and every top-level skill file to "skills". Those are bags of
// unrelated files, not units of work — one worktree/build/audit cannot
// meaningfully carry "everything that touches agents/".
//
// Live consequence measured on the 84-item backlog (2026-07-27): two "agents"
// edges chained three unrelated campaigns —
//
//	agents: chronicle-s7a-historian-shadow [chronicle-2026-07]
//	     <-> inbox-console-worklist-view   [pipeline-integrity]
//	     <-> acs-metapredicate-suite-scope [convergence-2026-07]
//
// producing one 43-item cluster (51% of the backlog) chunked into 11 batches
// each marked "run the previous batch first" — an 11-cycle serialized chain,
// and the same mega-cluster pathology hubAreaMaxItems was introduced to kill.
// With the floor: largest cluster 27 (32%), and the convergence campaign splits
// into its own clean 11-item cluster.
```

### `go/internal/inboxbatch/rules_rootcause_regression_test.go:3` — above `import (`

```text
// rules_rootcause_regression_test.go — the guard-rail against reintroducing the
// cycle-1204 audit-REJECTED root-cause binding design.
//
// Cycle-1204 proposed a `rootCauseRule` that bound inbox items by exact string
// equality on a free-form prose field (`root_cause`) and placed it in
// DefaultRules(), i.e. default-on for every backlog. The audit rejected it on
// two grounds, and the production code never landed:
//
//	D1 (no-op on real data): measured against the 67 live .evolve/inbox items,
//	all 20 non-empty root_cause values were UNIQUE prose (median 317 bytes).
//	Exact-match grouping over unique strings binds nothing — the rule paid
//	rule-set complexity for zero edges.
//
//	D2 (unbounded fusion): it carried neither discriminative guard its siblings
//	have — no hubAreaMaxItems-style CEILING, no minAreaDepth-style FLOOR. The
//	moment a normalising producer landed upstream (lowercase, collapse
//	whitespace, truncate), the campaign-less backlog would collapse into one
//	over-fused cluster, the exact mega-cluster pathology hubAreaMaxItems and
//	minAreaDepth were each introduced to kill (see rules_area_floor_test.go).
//
// So there is no feature to regression-test; the regression worth pinning is
// DEFENSIVE. DefaultRules() must stay the three bounded structural signals
// (campaign = explicit operator declaration, file-area = ceiling AND floor,
// deps = hard structural references), and none of them may derive grouping from
// a shared free-form prose field.
//
// `Item` has no `root_cause` field (it never landed), so the prose analogue
// asserted here is `Item.Title`: the live field that is genuinely
// unstructured, author-written and not a vocabulary (Class/Priority/Kind are
// enums). It is the closest faithful stand-in for the rejected field, and it
// keeps the guard meaningful without adding the field the audit rejected.
//
// This test is intentionally hostile to a future 4th default rule. That is not
// a ban on ever adding one — it is a demand that adding one come with a
// discriminative bound and a non-tautological eval against real backlog data,
// which is precisely what D1/D2 found missing.
```

## inbox prioritization P2

### `go/internal/inboxbatch/classify_test.go:96` — above `func TestClassify_DepsDoNotAffectGroupingOrOrdering(t *testing.T) {`

```text
// TestClassify_DepsDoNotAffectGroupingOrOrdering pins the post-removal contract: Deps is
// carried on Item (kept for now — see the cycle-1724 build report) but neither binds edges
// nor influences ordering. Items sharing only a Deps chain stay separate singletons, and a
// campaign-bound heavier child sorts BEFORE a lighter parent it declares a dependency on.
```
