# Comment history: `internal/phasecoherence`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/phasecoherence/artifact_coherence_test.go:1` — above `package phasecoherence`

```text
// artifact_coherence_test.go — cycle-238 task
// `persona-output-artifact-coherence` (RED first). API contract for Builder
// (architecture blueprint B11, reuses Options/Violation from coherence.go):
//
//	func CheckArtifactNames(opts Options) ([]Violation, error)
//
// Semantics pinned by these tests (architecture R7 + eval
// persona-output-artifact-coherence C2/C3/C4):
//   - persona side: FIRST whitespace/quote-delimited token ending in .md on
//     the `output-format:` frontmatter line; no output-format: line → skip
//     (eval C3).
//   - profile side: path.Base(output_artifact) with {cycle}-style template
//     segments stripped (they live in the dir part).
//   - basename mismatch ⇒ Severity WARN Violation whose Message carries BOTH
//     names (eval vocabulary "mismatch").
//   - persona declares output-format but the paired profile has NO
//     output_artifact ⇒ WARN (eval C4 pins "flagged as WARN"; this
//     deliberately overrides blueprint B11's "skip" — the eval is the audit
//     authority, and a persona promising an artifact nobody contracts for is
//     exactly the I-3(d) incoherence class).
//
// Forward-protection for the I-3(d) incident: persona said plan-review.md,
// profile said plan-review-report.md → batch-fatal exit=81 ×2.
```

### `go/internal/phasecoherence/artifact_coherence_test.go:49` — above `func TestArtifactCoherence_Mismatch(t *testing.T) {`

```text
// TestArtifactCoherence_Mismatch — eval C2 (name pinned): exact replica of
// the I-3(d) incident shape.
```

### `go/internal/phasecoherence/coherence.go:65` — above `if !strings.HasSuffix(name, "-reference") && !dispatchNone(opts, name) {`

```text
// "-reference" personas are documentation (auditor-reference
// etc.), never dispatched — no profile expected. Likewise a
// persona whose frontmatter declares `dispatch: none`
// (operator: monitoring persona driven outside the
// profile/subagent system) is intentionally unpaired.
// Everything else unpaired is a visibility WARN, not a silent
// skip: cycle-270's debugger died at launch (exit=10) because
// its persona existed and its profile didn't, and nothing said
// so until the route fired (inbox
// dispatchable-agent-profile-completeness).
```

### `go/internal/phasecoherence/coherence_adversarial_test.go:3` — above `import (`

```text
// coherence_adversarial_test.go — cycle-281 test amplification.
// Targets the uncovered branches in Check (76.1%) and CheckArtifactNames
// (67.3%) identified by the cycle-281 coverage baseline (88.6% total).
// All tests are black-box (spec-derived), never reading the implementation.
```

### `go/internal/phasecoherence/coherence_test.go:1` — above `package phasecoherence`

```text
// Package phasecoherence cross-checks the parallel hand-authored phase
// surfaces (agents/evolve-<name>.md persona frontmatter vs
// .evolve/profiles/<name>.json) for contradictions — Invariant-1 meta-gate
// (campaign retro §4, migration step 4; architecture-design Option B).
//
// coherence_test.go — cycle-238 task `persona-tools-coherence-gate`
// (RED first). API contract for Builder (architecture blueprint B6):
//
//	type Violation struct {
//	    Persona  string // base name, e.g. "builder" (evolve- prefix stripped)
//	    Kind     string // "disallowed" | "undeclared" (tools checks)
//	    Severity string // "WARN" for both drift directions
//	    Message  string // eval vocabulary: contradiction|mismatch|disallowed|undeclared
//	}
//	type Options struct {
//	    AgentsFS   fs.FS             // root CONTAINING agents/ (prompts.Loader layout)
//	    ProfilesFS fs.FS             // profiles dir root: <name>.json at top level (profiles.Loader layout)
//	    Overrides  map[string]string // persona name → OS file path substituting agents/evolve-<name>.md
//	}
//	func Check(opts Options) ([]Violation, error)
//
// Semantics pinned by these tests (architecture R4/R5/R6/R11):
//   - persona agents/evolve-<name>.md pairs with <name>.json; a missing
//     profile → "unpaired" WARN (cycle-270 fix; "-reference" docs exempt —
//     see unpaired_test.go); a missing persona side is silent (Check
//     iterates personas); non `evolve-`-prefixed .md ignored.
//   - `tools:` frontmatter only; `tools-gemini:`/`tools-generic:` are NOT the
//     tools line (multi-CLI coherence is out of scope). No tools: line → skip.
//   - profile without allowed_tools → no constraint → skip.
//   - normalization: base name before "(" — "Bash" ↔ "Bash(x:*)", "Skill" ↔
//     "Skill(code-review-simplify)"; disallowed_tools is NOT consulted.
//   - Kind "disallowed": persona declares a tool absent from allowed_tools.
//   - Kind "undeclared": profile allows a tool the persona omits (live
//     builder drift, scout F1).
//   - both directions are Severity WARN (eval C1 needs exit 0 on live tree).
```

### `go/internal/phasecoherence/coherence_test.go:206` — above `agents, profs := fixtures(`

```text
// Contract updated for inbox dispatchable-agent-profile-completeness
// (cycle-270): a persona without a profile is now an "unpaired" WARN —
// see unpaired_test.go for its pins. Still SILENT here: a profile
// without a persona (Check iterates personas only) and the
// certain-to-exist non-persona .md files in agents/ (AGENTS.md,
// agent-templates.md — architecture risk table).
```

### `go/internal/phasecoherence/coherence_test.go:370` — above `func TestCoherence_DispatchNonePersonaExempt(t *testing.T) {`

```text
// TestCoherence_DispatchNonePersonaExempt — a persona explicitly marked
// `dispatch: none` (operator: monitoring persona driven outside the
// profile/subagent system) is intentionally unpaired; it must not WARN.
// Contrast: an unmarked unpaired persona still WARNs (cycle-270 debugger
// died exit=10 at launch precisely because nothing surfaced the gap).
```

### `go/internal/phasecoherence/gitignore_birth_test.go:11` — above `func TestTrackedCorpusDirsAllowNewBirths(t *testing.T) {`

```text
// gitignore_birth_test.go — tracked-corpus birth coherence (2026-08-05,
// cycle-1345/1346/1348 identical-fingerprint batch HALT).
//
// The .gitignore ladder ignores .evolve/* and re-includes the surfaces that
// SHIP (profiles, phases, inbox, plugin, …). The failure class this test
// pins: a directory holds a TRACKED corpus (so the repo's operative design
// says "these files ship") while the ladder has no re-include for it — every
// NEW file there is born ignored. `git check-ignore` cannot flag the
// pathspec (tracked content exempts the dir), so ship staging keeps it, and
// `git add` refuses the whole pathspec (rc=1 "ignored by one of your
// .gitignore files") — a deterministic ship-killer for exactly the lanes
// whose deliverable is a new file in that corpus. .evolve/evals (54 tracked
// eval definitions, corpus since 2026-06-01, ladder line missing) halted the
// 2026-08-05 batch this way.
//
// Contract: every (directory, extension) pair present in the TRACKED corpus
// under .evolve/ must admit a NEW birth in at least ONE of the two shapes the
// ladder's own carve-outs use:
//
//   - a new NAME beside the exemplar (extension corpora: .evolve/evals/*.md), or
//   - the exemplar's exact NAME in a new sibling directory (exact-name
//     corpora: .evolve/phases/<new>/phase.json).
//
// A pair that admits neither is a ship-killer unless it is in the reasoned
// designed-ignored allowlist below (surfaces where tracked entries are
// grandfathered legacy and new births are runtime state BY DESIGN).
```

### `go/internal/phasecoherence/gitignore_birth_test.go:52` — above `".evolve/inbox-parked/": "grandfathered manual park; no active writer; parked content must not auto-ship",`

```text
// One-off manual park (e3509917, R100 rename out of inbox/); zero
// active Go writers — `grep -rn inbox-parked go/` is empty as of
// 2026-08-05. "Parked" means held-not-yet-decided: new content there
// must NOT auto-ship, so allowlist beats carve-out (adversarial
// review finding 1 on the cycle-1348 fix).
```

### `go/internal/phasecoherence/persona_strip_operational_test.go:3` — above `import (`

```text
// persona_strip_operational_test.go — pins that CompactPrompts stripping
// (prompts.StripOnDemandSections, default ON since policy.go CompactPrompts=true)
// never removes OPERATIONAL directives from a dispatched persona.
//
// Incident (2026-08-10, cycles 1390-1429): agents/evolve-auditor.md carried its
// "## Reference Index" marker at line 75 of 272 — every section appended after
// it over months (Verdict Rules, STOP CRITERION, completion gates, POSTHOC,
// the MANDATORY continuation-disposition contract) was silently stripped from
// every dispatched audit prompt. Result: 15/30 FAILs on disposition-preflight,
// 0/11 continuation passes, auditors approving work the gate then force-FAILed.
// The compaction test suite guarded only that stripping REMOVED bytes
// (compaction_coverage_test.go), never that it KEPT the load-bearing ones.
```

### `go/internal/phasecoherence/persona_strip_operational_test.go:27` — above `var operationalSentinelRE = regexp.MustCompile('MANDATORY|STOP CRITERION|Completion Gates|force-FAIL|REQUIRED|POSTHOC|Ve…`

```text
// operationalSentinelRE marks a line as an operational directive that must
// survive prompt compaction. Deliberately coarse: a false positive costs a
// persona author a relocation above the marker; a false negative re-arms the
// cycle-1390-1429 lobotomy class.
```

### `go/internal/phasecoherence/persona_strip_operational_test.go:49` — above `func TestAuditorStripKeepsOperationalContract(t *testing.T) {`

```text
// TestAuditorStripKeepsOperationalContract is the incident's direct regression
// pin: the exact anchors whose loss caused the 2026-08 zero-ship batches must
// survive compaction of the REAL auditor persona.
```

### `go/internal/phasecoherence/persona_strip_operational_test.go:85` — above `pendingCuration := map[string]bool{}`

```text
// Curation complete 2026-08-10: every dispatched persona's marker sits at
// EOF. New entries here need an incident-grade justification.
```

### `go/internal/phasecoherence/provenance_amplification_test.go:3` — above `import (`

```text
// Cycle-242 amplification tests — encode the cycle-241 audit HIGH finding:
// CheckProvenance only compared tree_sha against the ledger, never directly
// against expected.TreeSHA, so a bad tree_sha with no ledger produced zero
// violations. These tests pin the direct check AND the dedup contract
// (exactly one tree_sha violation when both the direct check and the ledger
// cross-check would fire — scout B2 risk; TestProvenanceGate_LedgerCrossCheck
// already requires len==1 for that scenario).
//
// TDD contract (cycle 242): Builder makes these pass WITHOUT modifying them.
```

### `go/internal/phasecoherence/provenance_amplification_test.go:133` — above `func TestCheckProvenance_LedgerAndDirectMismatch_SingleTreeSHAViolation(t *testing.T) {`

```text
// Dedup contract (scout B2): when a ledger entry exists AND agrees with
// expected.TreeSHA, a mismatched artifact tree_sha must yield exactly ONE
// tree_sha violation — not one from the direct check plus one from the
// ledger cross-check. This is the same invariant the pre-existing
// TestProvenanceGate_LedgerCrossCheck (len==1) enforces; restated here so
// the dedup requirement is explicit in the cycle-242 contract.
```

### `go/internal/phasecoherence/tracked_artifact_hygiene_test.go:12` — above `func TestNoRuntimeArtifactsTracked(t *testing.T) {`

```text
// tracked_artifact_hygiene_test.go — the "code and docs only" upstream contract
// (2026-08-27 repo-hygiene sweep).
//
// The failure class this test pins: the pipeline writes its runtime records
// WORKSPACE-RELATIVE (sessionrecord.FileName, bridge/tmux_inject.go's
// .bridge-inbox/<agent>-inject.txt, cmd_models_live.go's model-classifier-*,
// the ACS predicates' `go test -coverprofile=coverage.<pkg><cycle>.txt`), and
// the resolved workspace is SOMETIMES THE REPO ROOT — a console run, a nested
// sandbox, a lane whose worktree is the project dir. A broad `git add -A` then
// sweeps the artifact into a commit, where it ships to every clone and every
// plugin-marketplace install, forever. Twenty such files were found tracked on
// 2026-08-27 (~655 KB), the oldest dating to cycle-104.
//
// WHY A TEST AND NOT JUST A .gitignore RULE: an ignore rule only governs a
// FUTURE birth. It is silent about a file that was committed BEFORE the rule
// existed (git tracks it regardless) or that arrives via `git add -f`. Every
// artifact below was already matched-or-matchable by the spirit of the ignore
// ladder and still sat in the tree for months. The ladder states the intent;
// this test is the enforcement, and it runs in the ship-time repo-contract pack
// (ship/repocontract.go repoContractPackages includes ./internal/phasecoherence/...)
// so a regression fails the cycle that introduced it rather than a later audit.
//
// Kept deliberately narrow: only artifact classes with a KNOWN production
// writer are listed. This is not a general "no binaries" or "no large files"
// guard — staged-binary policy already lives in ship/binary_staging_guard.go
// (which allowlists go/bin/** and the marketplace-distributed go/evolve), and
// duplicating it here would give two places to update and one to forget.
```

### `go/internal/phasecoherence/tracked_artifact_hygiene_test.go:72` — above `re:   regexp.MustCompile('^lint-baseline\.txt$'),`

```text
// Exact root path, not a class: found tracked in the 2026-08-27
// sweep with NO identified production writer (2026-09-01 review
// re-confirmed: zero references across go/, skills/, .github/).
// Kept as a rule anyway — the guard's contract is "every artifact
// this sweep removed stays removed", and an unmatched file was
// re-addable without any test going red.
```

### `go/internal/phasecoherence/unpaired_test.go:1` — above `package phasecoherence`

```text
// unpaired_test.go — inbox dispatchable-agent-profile-completeness
// (2026-06-10T09-42Z): cycle-270's debugger persona existed, its profile
// didn't, and the route died exit=10 at launch with no earlier signal —
// because Check() silently skipped persona↔profile pairs with a missing
// side. These tests pin the visibility fix:
//
//  1. A persona without a profile → Kind "unpaired" WARN naming both paths
//     (the runner's typed fail-fast is the runtime half, landed cycle 276;
//     this is the look-ahead half).
//  2. "-reference" personas (auditor-reference, …) are documentation by
//     convention — never dispatched, no profile expected, NO violation.
//  3. TestRepoPersonaProfilePairing — the drift gate against the REAL tree:
//     every dispatchable persona is paired, with an explicit allowlist for
//     the intentional singletons. New unpaired personas fail CI here, which
//     is even earlier than batch preflight.
```

### `go/internal/phasecoherence/unpaired_test.go:32` — above `map[string]string{},`

```text
// no profile — the cycle-270 shape
```

### `go/internal/phasecoherence/unpaired_test.go:119` — above `profileOnly := map[string]string{`

```text
// Direction-B allowlist: TRACKED profiles whose prompt source is not an
// agents/evolve-*.md persona. Runtime-minted stubs no longer need entries
// here: they are untracked runtime mints (not necessarily gitignored), so
// trackedProfiles excludes them structurally (the cycle-~1326 firing was
// patched with per-name entries;
// the cd49274beab2 storm, cycles 1402/1403/1405, proved that ratchet
// re-arms on every new mint and killed a whole batch — see
// unpaired_tracked_test.go).
```

### `go/internal/phasecoherence/unpaired_test.go:181` — above `t.Logf("untracked profile %q: runtime-minted state, not bound", name)`

```text
// Untracked = runtime mint (untracked, not necessarily
// gitignored; the minter pairs phase->profile at dispatch
// time). It never lands on main, so it is not repo config
// and Direction B must not
// bind it — binding it is what turned the live plane's ship
// gate red for every lane (fingerprint cd49274beab2).
```

### `go/internal/phasecoherence/unpaired_tracked_edge_test.go:3` — above `import (`

```text
// unpaired_tracked_edge_test.go — edge-case pins for the #421 tracked-only
// Direction-B binding, added with the 2026-08-09 zero-ship batch postmortem
// (docs/incidents/2026-08-09-zero-ship-batch.md). The base regression file
// (unpaired_tracked_test.go) pins tracked-bound / untracked-unbound / loud
// non-repo error; these close the corners the adversarial review flagged:
//   - error fidelity: the wrapped error must carry git's stderr ("not a git
//     repository"), not just "exit status 128" — that line is the fallback
//     diagnostic an operator reads during an incident.
//   - empty tracked set: a repo with NO tracked profiles returns an empty
//     set + nil error — the input that must trigger the caller's loud
//     bind-all fallback rather than silently unbinding the gate.
//   - staged-but-uncommitted counts as tracked (the stricter direction; no
//     CI-vs-plane skew that weakens the gate).
//   - nested tracked profiles must NOT alias a same-named top-level stub
//     into the binding set (basename collision).
```

### `go/internal/phasecoherence/unpaired_tracked_test.go:3` — above `import (`

```text
// unpaired_tracked_test.go — regression pin for the cycle-1402/1403/1405
// ship|gate-block storm (2026-08-09 batch, fingerprint cd49274beab2):
// Direction B of TestRepoPersonaProfilePairing bound EVERY on-disk profile
// by name, so a runtime-minted stub — untracked, though NOT gitignored
// (`git check-ignore` says .evolve/profiles/defect-disposition-ledger.json
// is not ignored; it is merely untracked) — with no persona red'd the
// ship-time repo-contract pack on the live plane and blocked three
// audit-green lane ships in one batch — the identical-fingerprint ceiling
// then halted the whole batch. A profile git does not track can never reach
// main (CI checkouts do not contain it), so it is runtime state, not repo
// config, and must not be in the Direction-B binding set.
```

### `go/internal/phasecoherence/unpaired_tracked_test.go:45` — above `writeProfile("defect-disposition-ledger")`

```text
// the real incident stub, never added
```

### `go/internal/phasecoherence/unpaired_tracked_test.go:78` — above `root := mirrorTrackedProfiles(t, real, pre)`

```text
// The live tree is never mutated: a phase sandbox denies writes under
// .evolve/profiles (cycles 1676/1679 red on EPERM here — an instrument
// fault charged as a defect), and a test that writes into tracked repo
// config is a hygiene defect regardless. The real tracked set is mirrored
// into a temp git repo and the decoy is planted THERE.
```
