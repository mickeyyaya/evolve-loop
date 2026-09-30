# Comment history: `internal/guards`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/guards/apicover_named_test.go:3` — above `package guards`

```text
// apicover_named_test.go — public-API coverage (ADR-0050 Phase 5). Names and
// exercises the exported guard TYPES apicover flagged uncovered in this
// package. The existing *_test.go files only call the New* constructors, so the
// bare type identifiers (Chain/DocDelete/Quota/Ship) never appear as tokens in
// test source — apicover reports the types UNCOVERED even though their methods
// are tested. Each test below names the concrete type via a typed declaration
// AND asserts a real contract: the type satisfies core.Guard and its Decide
// returns the documented Allow verdict (Rule 9 — no bare `var _ T` padding).
```

### `go/internal/guards/chain.go:1` — above `package guards`

```text
// Package guards is the in-process trust kernel — host-agnostic Go
// implementations of the bash scripts/guards/*.sh + scripts/hooks/*.sh.
//
// Each guard satisfies core.Guard and is invokable via
// `evolve guard <name>` (wired in cmd/evolve/cmd_guard.go in task #17).
```

### `go/internal/guards/docdelete.go:60` — above `func isDocsDest(p string) bool {`

```text
// isDocsDest reports whether the mv destination stays inside the single
// documentation root. Since the 2026-08-05 doc-root consolidation, any move
// whose destination is under docs/ is a reorganization, not a deletion — this
// includes the archive home docs/private/research/archived-YYYY-MM-DD/.
// knowledge-base/ is deliberately NOT a valid destination: its research/
// subtree is retired (content lives in docs/research), and cycles/ is a
// runtime write surface, not documentation.
```

### `go/internal/guards/docdelete_test.go:10` — above `func TestDocDelete_Name(t *testing.T) {`

```text
// DocDelete is the port of scripts/hooks/doc-deletion-guard.sh.
// Rules:
//   - rm command targeting docs/** or knowledge-base/** → DENY
//   - mv from docs/** or knowledge-base/** → DENY unless the dest stays under
//     docs/ (reorganization within the single doc root is never a deletion;
//     the archive home is docs/private/research/archived-YYYY-MM-DD/ since
//     the 2026-08-05 doc-root consolidation — knowledge-base/research/ is
//     retired as an archive target)
//   - constructor policy can bypass
//   - Edit / Write tools pass through (cannot delete files in place)
```

### `go/internal/guards/explanation_activation_belt_test.go:25` — above `func TestExplanationActivationBelt_SharedByShipAndAudit(t *testing.T) {`

```text
// TestExplanationActivationBelt_SharedByShipAndAudit pins the single-sourcing
// of the activation cross-check (architecture review 2026-09-01, HIGH):
// CycleBinding.ContractVersion==0 encodes BOTH "legacy cycle" and "a caller
// dropped the field", and only the host activation marker can tell them
// apart. Ship carried the belt inline; audit had NONE — version 0 silently
// disabled its explanation gate (Verify -> !active -> nil). Both gates must
// consult the host through explanationdocs.CrossCheckActivation, and the
// field-comparison belief must not live in a phase package.
```

### `go/internal/guards/explanation_review_ssot_test.go:25` — above `func TestExplanationReviewGates_ShareContractCore(t *testing.T) {`

```text
// TestExplanationReviewGates_ShareContractCore pins the single-sourcing of the
// explanation-review contract (architecture review 2026-09-01, CRITICAL): the
// status enum, build-status match, required/not_applicable document switch,
// and evidence-reference check were stated twice — once per phase gate — with
// nothing binding the copies. Both gates must now delegate the shared core to
// explanationdocs.ValidateReviewedHandoff, and the belief literal must not
// reappear in either phase package. Per-phase policy (heading, missing-handoff
// verdict, NEEDS_CORRECTION disposition) legitimately stays in the gates.
```

### `go/internal/guards/explanation_review_ssot_test.go:48` — above `if !functionCalls(t, path, "validateExplanationReview", "reportdoc.ReasonedReview") {`

```text
// ADR-0102: the reasoning-floor ladder (section → fields → floor) has
// ONE home, reportdoc.ReasonedReview; a gate that locates the section
// or applies the floor itself has forked the ladder.
```

### `go/internal/guards/integrity_surface.go:19` — above `var ProtectedSurfaceManifest = []ProtectedSurfaceEntry{`

```text
// ProtectedSurfaceManifest is the SINGLE SOURCE OF TRUTH for the pipeline
// INTEGRITY CONTROL PLANE: the deterministic gates that grade a cycle, the
// campaign metric SSOT, the guards themselves, the campaign contract, the
// grading rubrics, and the PreToolUse hook wiring.
//
// No autonomous phase agent may modify these — a cycle must never be able to
// edit the gate that judges it. This is the structural fix for the cycle-20
// breach, where the build agent edited
// go/acs/regression/flagreaders/readers_test.go (and the registry SSOT) to bless
// its own split-const dodge, and the audit approved the self-edit.
//
// Legitimate control-plane changes (e.g. an operator hardening a gate) go through
// human-gated `evolve ship --class manual` OUTSIDE any cycle, where the role
// guard's CycleID==0 path allows them — never an autonomous `--class cycle`.
//
// Fragments are matched anywhere in the slash-normalized path, so the boundary
// holds regardless of the file's physical root (a per-cycle worktree, the branch
// root, or main). The intentionally NARROW fragments (e.g. go/acs/regression/ but
// NOT go/acs/, registry_table.go but NOT the whole flagregistry package) preserve
// legitimate cycle behavior: a cycle still writes its own go/acs/cycleN/
// predicates and ordinary source.
//
// The same manifest has a second projection, IsProtectedScope (F29), used only
// to ROUTE inbox work: an item that DECLARES a directory holding protected files
// (go/internal/core/) is console work at seed time. It never changes what a
// cycle may write — the role guard, the ship tripwire, the fleet preflight and
// triage's breaker all keep membership (IsProtectedSurface).
//
// The manifest is deliberately a COMPILED Go value, not config: the boundary
// must not be config-softenable (a .evolve/policy.json knob here would let a
// cycle write the knob that disarms the guard). Two durable tripwires check it
// on different lanes, so neither can silently rot as the trust kernel grows:
// go/acs/regression/protectedsurface walks the repo by NAME (any *_gate.go or
// *guard*.go file must be covered here, L4, architecture review 2026-07-16);
// TestProtectedSurface_CoversEveryExplanationLifecycleCallSite (guards package)
// walks it by CALL SITE (any file that calls into the explanation-lifecycle
// API, derived from build_explanation_handoff.go's and explanationdocs'
// declarations, must be covered here — closes the #549 class where a
// relocation changed WHICH file holds the code without changing its name).
```

### `go/internal/guards/integrity_surface.go:75` — above `{Fragment: "/go/internal/commitgate/", Rationale: "the pre-commit quality gate (attestation writer the manual-ship reade…`

```text
// L4 additions (architecture review 2026-07-16): gate-shaped surfaces the
// pre-L4 list left silently in-cycle-writable. Dir entries where every file
// in the package is control plane; file entries where the package (core,
// phases/ship) is otherwise legitimate cycle territory.
```

### `go/internal/guards/integrity_surface.go:92` — above `{Fragment: "/go/internal/core/disposition_gate.go", Rationale: "ADR-0074 disposition-contract gate (retro's verdict-on-t…`

```text
// ADR-0074 S2 (operator-enrolled 2026-07-22, cycle-1034 salvage): the
// disposition gate grades the retro that grades failed cycles — a cycle
// able to edit it could route its own failure classification.
```

### `go/internal/guards/integrity_surface.go:215` — above `func IsProtectedScope(path string) bool {`

```text
// IsProtectedScope reports whether path IS, or CONTAINS, protected surface — the
// question a DECLARED fix surface poses (F29): an inbox item declaring
// `"files": ["go/internal/core/"]` will change files inside that directory, and
// the file-level fragments IsProtectedSurface matches can never be contained in
// the directory's own spelling. Only a directory spelling (a trailing slash, or
// a last segment with no extension) widens beyond membership, so
// IsProtectedSurface(p) ⇒ IsProtectedScope(p) always holds
// (TestIsProtectedScope_ImpliedByMembership). It is a second PROJECTION of the
// one manifest, used where a declared surface is judged — the seed-time console
// classifier's routing roots — while the ship tripwire, the role write-guard,
// the fleet preflight and triage's breaker keep membership: the seed must
// refuse at least everything the breaker would, never make the breaker stricter.
```

### `go/internal/guards/integrity_surface_directory_test.go:8` — above `func TestIsProtectedScope_DirectorySpellingCoversTheSurfaceInside(t *testing.T) {`

```text
// TestIsProtectedScope_DirectorySpellingCoversTheSurfaceInside (F29): the SCOPE
// projection answers the question a declared fix surface poses — a directory
// spelling (trailing slash, or a last segment with no extension) is in scope
// when a manifest fragment lies inside it. Before F29 the console classifier
// had only membership, so `tokenopt-handoff-digests-per-edge-remainder` (files
// go/internal/core/, go/internal/phases/runner/) reached a lane four times
// (cycles 1646, 1651, 1654, 1656) and triage's breaker refused its card each
// time.
```

### `go/internal/guards/integrity_surface_directory_test.go:46` — above `func TestIsProtectedSurface_MembershipStaysMembership(t *testing.T) {`

```text
// TestIsProtectedSurface_MembershipStaysMembership: the MEMBERSHIP projection
// the ship tripwire, the role write-guard, the fleet preflight and triage's
// breaker use is NOT widened to scope — a directory that merely contains
// protected files is not itself a member (the breaker must never get stricter
// than the seed that screens for it). Its one F29 fix: a path NAMING a
// protected directory without the trailing slash (a package or import path)
// is a member of that directory.
```

### `go/internal/guards/integrity_surface_explanation_callsites_test.go:16` — above `func explanationLifecycleVocabulary(t *testing.T) map[string]bool {`

```text
// integrity_surface_explanation_callsites_test.go closes a real gap: #549
// relocated five explanation-lifecycle call sites out of files the manifest
// protected (cyclerun_review.go, resume.go, audit.go, runner.go) into files
// it does not (cyclerun_postreview.go, resume_execution.go, resume_bootstrap.go,
// phases/audit/classification.go, phases/runner/dispatch.go). Two independent
// beliefs about "where the protected call sites are" — the manifest and the
// four AST pin tables/columns in build_explanation_wiring_test.go,
// explanation_activation_belt_test.go and explanation_review_ssot_test.go —
// had drifted apart, which is exactly the class the repo's
// single-source-with-projection rule forbids.
//
// Rather than hand-pinning the five files (which only re-creates a fifth
// belief that can drift again), explanationLifecycleVocabulary derives the
// protected CALL-SITE vocabulary from beliefs that already exist: every
// callee, value and gate-entry-point column of those pin tables, plus the
// function names build_explanation_handoff.go and the explanationdocs
// package themselves declare (both already manifest rows whose stated
// purpose IS the lifecycle API). explanationCallSiteFiles then scans the
// tree for any file containing a call, or a struct-literal field assigned by
// value, that resolves to a name in that vocabulary. A file hit by the scan
// but absent from the manifest is the actual defect this closes.
//
// This closes the classes it can see: a call, or a struct-literal field
// assigned a call or a bare/qualified function value, whose resolved name is
// already known to be part of the lifecycle API. It does NOT make every
// future relocation structurally impossible — see
// TestExplanationHandoffVocabulary_IncludesKnownLifecycleFunctions and
// TestExplanationDocsVocabulary_IncludesKnownVerificationFunctions below,
// which pin that the two DERIVATION SOURCES themselves keep naming the
// functions this file's vocabulary depends on, so a future split of either
// source file is caught directly rather than by incidental file-level
// coverage. The scanner's root is go/internal only (root ".." from this
// package) — go/cmd and go/acs are not walked, matching the scope of the
// five files this change was written to protect; no call site into this
// vocabulary lives outside go/internal today.
```

### `go/internal/guards/integrity_surface_explanation_callsites_test.go:67` — above `for _, pin := range explanationActivationBeltPins {`

```text
// The activation belt's gate ENTRY POINTS (e.g. verifyExplanationDocumentation)
// are function names too, even though the belt test pins them as the
// function being checked rather than a callee — a struct-literal field
// assigned one of them by value (audit.go's `CheckExplanation:
// verifyExplanationDocumentation`) is exactly the wiring
// explanationCallSiteFiles' KeyValueExpr match exists to catch
// (architecture review 2026-09-12 round 2, HIGH: verifyNativeExplanation
// happened to already be a call-pin callee; verifyExplanationDocumentation
// was not in vocab anywhere, so this same class of wiring for Audit was
// invisible).
```

### `go/internal/guards/integrity_surface_explanation_callsites_test.go:97` — above `func parseFuncDeclNames(path string) (map[string]bool, error) {`

```text
// parseFuncDeclNames returns every top-level FREE function name declared in
// path (methods excluded), by parsing it once rather than importing it. A
// method is never callable by its bare short name — `apply` or `Review`
// declared on a receiver can only ever appear as "x.apply"/"x.Review" at a
// call site — so including one in a bare-name vocabulary would be pure dead
// weight at best and a same-named free-function collision risk at worst
// (architecture review 2026-09-12 round 2, MEDIUM). It is a pure function —
// no *testing.T — precisely so its error path is a normal assertion (see
// TestParseFuncDeclNames_MissingFileErrors) rather than a Fatal a test can't
// observe.
```

### `go/internal/guards/integrity_surface_explanation_callsites_test.go:127` — above `const explanationDocsExcludedDocumentPath = "DocumentPath"`

```text
// explanationDocsExcludedDocumentPath is the one exported explanationdocs
// function deliberately NOT part of the call-site vocabulary: pure path
// formatting for a prompt hint (cycle+runID -> a filename string), making no
// trust decision — the actual verification a caller of this path would need
// is Verify/VerifyLanded/CheckBuild, which ARE in the vocabulary. Excluding
// it matters operationally, not just semantically: its only non-test caller,
// go/internal/phases/build/build.go, is deliberately NOT a protected
// surface, and including this name would force a manifest row for the whole
// build phase over a prompt hint (architecture review 2026-09-12 round 2).
```

### `go/internal/guards/integrity_surface_explanation_callsites_test.go:197` — above `func TestExplanationHandoffVocabulary_IncludesKnownLifecycleFunctions(t *testing.T) {`

```text
// TestExplanationHandoffVocabulary_IncludesKnownLifecycleFunctions pins a
// FLOOR on build_explanation_handoff.go's own declarations, independent of
// which files currently call them. A future split of that file (the #549
// shape, one level up) that drops one of these names from the file — without
// updating this floor — fails here, rather than passing silently because
// some OTHER file happens to also be anchored via a different vocabulary
// term (see architecture review 2026-09-12, HIGH: file-level anchoring alone
// cannot detect this).
```

### `go/internal/guards/integrity_surface_explanation_callsites_test.go:247` — above `func explanationLifecycleImportAliases(root string) ([]string, error) {`

```text
// explanationLifecycleImportAliases reports every file under root that
// imports "…/explanationdocs" under an alias, or dot-imports it. Either form
// defeats the qualified-name matching explanationCallSiteFiles and
// expressionName rely on (a call through an alias resolves to "alias.Fn", a
// dot-import resolves to bare "Fn") — this is a documented precondition
// (architecture review 2026-09-12, MEDIUM); this function enforces it rather
// than leaving it merely asserted in a comment. It skips _test.go files, like
// explanationCallSiteFiles does, since a fixture file's own import style has
// no bearing on whether it can defeat that scanner (which never reads
// _test.go files in the first place).
```

### `go/internal/guards/integrity_surface_explanation_callsites_test.go:345` — above `func explanationCallSiteFiles(root string, vocab map[string]bool) ([]string, error) {`

```text
// explanationCallSiteFiles walks every non-test .go file under root and
// returns the slash-separated paths (relative to root) of every file
// containing a call, or a struct-literal field assigned by value (a call, a
// bare function, or a qualified function), whose resolved name is in vocab.
// It reuses expressionName (build_explanation_wiring_test.go), the same
// resolver the existing pin tables are checked against, so a qualified name
// like "explanationdocs.RefreshResult" is matched identically in both
// places, for both a call and a value position. vendor/ and testdata/
// subtrees are never a cycle's own dispatch code, so they are skipped; a
// file that fails to parse is skipped rather than aborting the whole scan,
// since a malformed file is a build/vet concern, not a protected-surface
// one; _test.go files are skipped because a fixture exercising a lifecycle
// function (e.g. build_explanation_state_test.go, which calls
// projectBuildExplanation directly) is not a dispatch call site and does not
// itself need protection.
//
// The struct-literal-value case matters because that assignment IS the call
// site if it is ever carved into its own file: the decision of which
// function runs lives at the assignment, not at the point something later
// invokes the resulting field (architecture review 2026-09-12, MEDIUM;
// example: audit.go's `CheckExplanation: verifyExplanationDocumentation`).
// expressionName resolves a value uniformly whether it is a bare identifier
// or a qualified selector, so a const/data-field reference that happens to
// share a vocabulary NAME (there are exactly two on this tree today —
// orchestrator.go's explanationdocs.CurrentContractVersion and cyclerun.go's
// o.explanationContractVersion, both already protected manifest rows for
// unrelated reasons) is treated no differently from a genuine function
// reference; verified empirically that this introduces no unprotected hit.
//
// Known limitation: this cannot resolve a call through an interface value or
// a method expression whose receiver type isn't syntactically named at the
// call site (e.g. `a.hooks.explanationCheck(a.req)` resolves to
// "a.hooks.explanationCheck", never to a vocabulary function name) — the
// WIRING site this function DOES catch is what matters, because that is
// where the decision of which function to run is actually made.
```

### `go/internal/guards/integrity_surface_manifest_test.go:3` — above `import (`

```text
// integrity_surface_manifest_test.go — regression pins for the L4
// protected-surface SSOT refactor (architecture review 2026-07-16). The
// fragment list became the exported ProtectedSurfaceManifest registry; these
// tests prove the refactor is behavior-preserving (every pre-refactor fragment
// survives byte-for-byte and still denies) and that every entry — including the
// L4 perimeter additions — satisfies the invariants IsProtectedSurface's
// Contains matching relies on. The durable cross-tree tripwire (every
// gate-shaped file must be covered by the manifest) lives in
// go/acs/regression/protectedsurface.
```

### `go/internal/guards/integrity_surface_test.go:5` — above `func TestIsProtectedSurface(t *testing.T) {`

```text
// TestIsProtectedSurface pins the pipeline integrity control plane: the gates,
// metric SSOT, guards, contract, grading rubrics, and hook wiring that no
// autonomous cycle may modify (the cycle-20 breach edited
// go/acs/regression/flagreaders/readers_test.go to bless its own dodge). The
// match is on a slash-normalized fragment so it holds wherever the file lives
// (cycle worktree, branch root, or main).
```

### `go/internal/guards/integrity_surface_test.go:13` — above `"/Users/x/.evolve/worktrees/cycle-21/go/acs/regression/flagreaders/readers_test.go",`

```text
// the exact cycle-20 breach file, inside a cycle worktree
```

### `go/internal/guards/role.go:90` — above `if IsProtectedSurface(path) {`

```text
// INTEGRITY BOUNDARY (control-plane sandbox, ADR-0064): no phase may modify the
// gate/metric/guard/contract that grades its own cycle. Overrides the
// worktree/workspace allowances below and applies to every phase; a hit is
// denied AND alarmed. See guards.IsProtectedSurface for the surface + rationale.
```

### `go/internal/guards/role.go:104` — above `if cs.Phase == "retro" && isLessonsCorpusPath(path) {`

```text
// Retro lessons allowance — documented in this file's header since phase-1
// but implemented only now (cycles 1036/1041/1042 pinned the doc-only gap):
// the retrospective phase persists failure-lesson YAMLs to the durable
// corpus. Path-fragment anchored (the guard carries no project root; the
// fragment idiom mirrors IsProtectedSurface) and phase-scoped, AFTER the
// control-plane boundary above so no protected path can ride it.
```

### `go/internal/guards/role_integrity_test.go:11` — above `func TestRole_DeniesControlPlaneEditInBuildPhase(t *testing.T) {`

```text
// TestRole_DeniesControlPlaneEditInBuildPhase is the cycle-20 regression: a build
// phase — which IS allowed to write source into its worktree — must STILL be
// denied (with an alarm) from editing the gate/metric/guard/contract that grades
// it. The control-plane boundary overrides the worktree allowance.
```

### `go/internal/guards/role_integrity_test.go:27` — above `"go/acs/regression/flagreaders/readers_test.go",`

```text
// the EXACT cycle-20 breach file
```

### `go/internal/guards/role_integrity_test.go:188` — above `func TestRole_DeniesRelocatedExplanationCallSitesInBuildPhase(t *testing.T) {`

```text
// TestRole_DeniesRelocatedExplanationCallSitesInBuildPhase is the #549
// regression: five files gained an explanation-lifecycle call site when #549
// carved them out of a protected file (cyclerun_review.go, resume.go,
// audit.go, runner.go) into one the manifest did not yet name. A build phase
// must be denied (with an alarm) from editing any of them, exactly like the
// files it was already denied before the move — and two neighboring files
// that were NEVER part of this call chain must stay allowed, proving the fix
// is file-narrow rather than a blanket lockdown of the packages involved.
```

### `go/internal/guards/role_lessons_test.go:3` — above `import (`

```text
// role_lessons_test.go — unit pins for the retro lessons-corpus allowance
// (cycles 1036/1041/1042: the header documented the allowance, Decide never
// implemented it, so retros could not persist lesson YAMLs and learning
// durability silently broke). The ACS suites acs/cycle{1036,1041,1042} carry
// the behavioral contract under -tags acs; these pins keep the crux in the
// default sweep.
```

### `go/internal/guards/role_lessons_test.go:36` — above `d := r.Decide(context.Background(), lessonsGuardInput("/repo/.evolve/instincts/lessons/cycle-7-lesson.yaml"))`

```text
// Fixed non-/tmp path: Decide never stats the write path, and a t.TempDir()
// path would ride the /tmp always-safe allowance on Linux (the exact
// platform split that flunked PR #351's ubuntu job while macos passed).
```

### `go/internal/guards/ship.go:44` — above `stripped := stripHeredocs(cmd)`

```text
// v11.8.3+: strip heredoc bodies before the verb regex so commit
// message bodies that legitimately mention `git push` / `git commit`
// (e.g. describing what a script does) don't trip the gate. Mirrors
// the awk pre-processor in legacy/scripts/guards/ship-gate.sh.
```

### `go/internal/guards/ship.go:52` — above `if shipScriptRe.MatchString(cmd) || nativeShipRe.MatchString(cmd) {`

```text
// Verb present — require the canonical script path OR the native
// `evolve ship` CLI (v11.3.0+).
```

### `go/internal/guards/ship_heredoc_test.go:113` — above `body := 'evolve ship --class manual "$(cat <<'EOF'`

```text
// Native evolve ship invocation with commit message body that
// legitimately mentions `git push` and `git commit` (the v11.7.5
// failure mode that triggered this fix).
```
