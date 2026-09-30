# Comment history: `internal/modelquery`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/modelquery/agy_test.go:3` — above `import (`

```text
// agy_test.go — agy must be enumerated by `agy models`, NOT by driving its
// /model picker.
//
// The live incident this pins (observed 2026-08-28, catalog written
// 2026-08-14 with source:"live"): agy's picker separates the model from a
// SEPARATE effort slider, so the pane reads
//
//	Switch Model
//	  Gemini 3.7 Flash
//	  Gemini 3.1 Pro
//	  Effort  ◂ ●━━━━◉─────○ ▸   low  medium  high
//
// The picker parser faithfully captured those unsuffixed names — and they are
// NOT valid `--model` arguments. agy requires model and effort COMBINED
// ("Gemini 3.7 Flash (Low)"), so every tier resolved to a name agy rejects:
//
//	⎿ model Gemini 3.1 Pro is not recognized as a known model or custom model
//	  in settings. Using "Gemini 3.5 Flash (Medium)" instead.
//
// agy does not exit non-zero on that — it warns once and serves the fallback
// for the whole session, so router/memo silently ran Gemini 3.5 Flash (Medium)
// at every tier. `agy models` emits "<id>\t<display name>" with the effort
// already baked in, which is exactly the string --model accepts (both halves
// verified live), so it is the only faithful source.
```

### `go/internal/modelquery/agy_test.go:102` — above `func TestAgyLister_HeaderAndBannerRowsNeverBecomeModels(t *testing.T) {`

```text
// A tab is not proof of a model row. Found by adversarial review: if a future
// `agy models` grows a header, a naive "has a tab" rule contributes
// "DISPLAY NAME" as a model — which would then be written into the catalog as
// a tier model and rejected at launch, reproducing the exact incident class
// this file exists to close. Not a live defect on 1.1.22; pinned so it cannot
// become one.
```

### `go/internal/modelquery/effort_live_test.go:5` — above `import (`

```text
// effort_live_test.go — the guard that would have caught the original defect.
//
// Every unit test in effort_test.go asserts against help text CAPTURED at a
// point in time. That is exactly the weakness that let two live bugs sit
// undetected for weeks: a fixture proves the parser handles what the CLI USED
// to print, never what it prints today. The agy incident is the canonical
// case — three tests pinned "Gemini Flash 3.7 (High)" and stayed green for two
// weeks while agy rejected that very string at every launch.
//
// So this test asks the REAL binary. It runs under -tags integration and skips
// cleanly wherever the CLI is not installed, because "not installed" and
// "installed but no longer publishes its ladder" must never look alike.
//
// KNOW WHAT THIS DOES NOT COVER. CI provisions no agent CLIs, so on the
// runners every case here SKIPS — it reports SKIP, never PASS, and a ladder
// that changed upstream would not be caught there. These are an OPERATOR/dev
// guard, run where the CLIs actually live; treating a green CI as evidence
// they passed is the same "cited for more than it checks" error the rest of
// this work is about. The honest reading of a CI run is: unit tests covered
// the parser, nothing checked it against reality.
```

### `go/internal/modelquery/effort_test.go:25` — above `const realClaudeHelp = '  --debug                               Enable debug mode`

```text
// Captured from `claude --help` on Claude Code v2.1.248. The enum is on the
// CONTINUATION line, not the flag line — a line-local parser finds nothing.
```

### `go/internal/modelquery/family.go:5` — above `var familyTokens = []struct {`

```text
// family.go implements design point D7 (FAMILY CONSTRAINT) of the
// latest-model-preference feature: two pure, deterministic, zero-I/O helpers
// that keep a CLI's candidate model set family-pure BEFORE classification /
// newest-wins.
//
// Live evidence (inbox 2026-07-02 latest-model-preference): the agy classifier
// flapped an identical list Sonnet-4.6→GPT-OSS-120B and agy's tier map carried
// Claude/GPT-OSS models — a family violation for a Gemini-only CLI. Cross-family
// coverage belongs to the cli_fallback chain, never a CLI's own tier map. A
// deterministic family filter makes that purity structural instead of an
// operator hand-correction, and it composes with the D2 NewestInLineage
// comparator (see FilterByFamily → NewestInLineage in the acceptance scenario).
```

### `go/internal/modelquery/family_test.go:8` — above `func TestFamilyOf_ClassifiesKnownFamilies(t *testing.T) {`

```text
// family_test.go is the RED contract for design point D7 (FAMILY CONSTRAINT) of
// the latest-model-preference feature. It pins the two pure, deterministic Go
// helpers a family-pure candidate set needs BEFORE classification / newest-wins:
//
//	FamilyOf(id) string              — classify a raw model id into a family
//	FilterByFamily(ids, allowed...)  — keep only ids in an allowed family
//
// Live evidence motivating D7 (inbox 2026-07-02 latest-model-preference): the
// agy classifier FLAPPED an identical list Sonnet-4.6→GPT-OSS-120B and agy's
// tier map carried Claude/GPT-OSS models — a family violation for a Gemini-only
// CLI. Cross-family coverage belongs to the cli_fallback chain, never a CLI's
// own tier map. A deterministic family filter makes that purity structural
// instead of an operator hand-correction. Authored by the TDD engineer — the
// Builder implements internal/modelquery/family.go and must NOT modify this file.
```

### `go/internal/modelquery/lineage_test.go:63` — above `func TestLineageKey_DatedSnapshotsStayDistinct_KnownLimitation(t *testing.T) {`

```text
// TestLineageKey_DatedSnapshotsStayDistinct_KnownLimitation pins a DELIBERATE
// conservative behavior (adversarial-review finding 3): date-stamped snapshot
// ids of the same line ("gpt-4o-2024-08-06" vs "gpt-4o-2024-11-20") keep
// DIFFERENT keys because only the first dotted numeric run is stripped, so
// PromoteLatest is a no-op for them — the classifier's pick is kept, never
// substituted. That is the fail-safe side of the design (an uncertain
// identity must never substitute); the cost is no automatic promotion across
// dated snapshots. None of the four live CLIs report dated ids today; the
// follow-up is queued as lineage-datestamp-normalization. If this test
// starts failing because normalization was implemented, move these cases to
// the mustMatch table with collision review for size/tag suffixes
// (":8b" vs ":70b" contain digits and must NOT be stripped).
```

### `go/internal/modelquery/ollama_classifier_test.go:88` — above `func TestOllamaListMetadataExceptionDocumented(t *testing.T) {`

```text
// TestOllamaListMetadataExceptionDocumented pins GAP 2's required call-site
// comment (scout mailbox → Auditor: "ollama list is an ALLOWED metadata
// exception (must be commented + tested as no-model-reached)"). A grep-gamed
// magic string alone would be a degenerate predicate (cycle-85 lesson), so
// this test also runs alongside TestOllamaListerReachesNoModel, which
// exercises the actual no-prompt behavior the comment documents.
```

### `go/internal/modelquery/picker_test.go:8` — above `const codexPickerPane = '╭──────────────────────────────────────────────╮`

```text
// The fixtures below are verbatim captures of each CLI's /model picker pane
// (tmux capture-pane), collected live on 2026-06-01. They are the ground-truth
// regression corpus for the per-CLI parsers.
```

### `go/internal/modelquery/query_test.go:193` — above `func TestRefreshFamilyFilterAppliedBeforeClassify(t *testing.T) {`

```text
// TestRefreshFamilyFilterAppliedBeforeClassify pins the D7 (FAMILY CONSTRAINT)
// production wiring of latest-model-preference: RefreshDeps.AllowedFamilies
// must filter each CLI's live-queried id list down to its allowed families
// BEFORE the ids reach the Classifier — the exact live-evidence incident (agy
// classifier flapped an identical list Sonnet-4.6->GPT-OSS-120B because
// cross-family ids reached classification at all). A no-op wiring (Classify
// still sees the raw list) fails this assertion even if the final catalog
// happens to look right by luck. RED today: RefreshDeps has no
// AllowedFamilies field (compile failure).
```
