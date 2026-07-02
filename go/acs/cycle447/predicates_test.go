//go:build acs

// Package cycle447 materialises the cycle-447 acceptance criteria (goal:
// 6fcbcd226cb22fe4ab63407907fb5e2f3a570b4939cb0a2d79f61e97e60d2f0a — model-
// switch through the abstract layer for every LLM CLI; close the one broken
// cell: agy-tmux, whose params.model_tier is channel:"noop" with a flat
// model_tier_map).
//
// Three ## top_n tasks (triage-report.md), strict dependency chain:
//
//	Task 1 (agy-model-channel-probe-and-wire, M, P1): probe the installed agy
//	  live (incident cycle-154 forbids trusting the stale KB `-m` claim), wire
//	  the least-code effective channel (preference flag→repl→picker-select),
//	  and replace the flat model_tier_map with real distinct per-tier ids.
//	Task 2 (model-tier-matrix-parity-pin, S, P2): parity test over the
//	  embedded *-tmux manifests — every tmux CLI translates intent.ModelTier
//	  through SOME effective channel; no multi-model CLI may be noop; includes
//	  a noop-rejection negative fixture and an integration-style agy tier=deep
//	  launch assertion.
//	Task 3 (model-channel-translation-docs, S, P3): per-CLI channel table in
//	  docs/architecture/model-discovery-and-catalog.md, cross-checked against
//	  the manifests (projection, never a second mapping).
//
// AC map (1:1, R9.3 floor-binding; predicates for ## top_n tasks only):
//
//	T1 AC1 fresh live probe evidence in build-report      → manual+checklist
//	                                          (Auditor judgment on liveness —
//	                                           see test-report.md checklist)
//	T1 AC2 agy tier=deep realizes an effective emission    → C447_001 (positive)
//	T1 AC3 manifest tier map has ≥2 distinct models        → C447_002 (edge)
//	T1 AC4 `auto` sentinel emits NO model for agy          → C447_003 (negative)
//	T1 AC5 builder unit tests: Agy auto-sentinel +
//	       unknown-tier named tests all green              → C447_004 (semantic)
//	T1 AC6 repl seed-path test exists & passes if repl     → C447_005 (edge,
//	                                                          conditional)
//	T2 AC1 parity: one subtest per *-tmux manifest         → C447_006 (positive)
//	T2 AC2 noop-rejection negative fixture subtest         → C447_007 (negative)
//	T2 AC3 dispatched agy tier=deep carries deep model     → C447_008 (semantic)
//	T2 AC4 bridge vet + -race suite green (regression)     → C447_009 (regression)
//	T3 AC1 doc channel row per manifest matches actual     → C447_010 (positive,
//	                                                          glob-driven edge)
//	T3 AC2 no table row documents a CLI as noop            → C447_011 (negative)
//	T3 AC3 doc states Realizer seam, tier vocabulary,
//	       auto-sentinel omission, overlay order           → C447_012 (semantic)
//
// 1:1 enforcement: 12 predicates + 1 manual+checklist = 13 ACs, each AC
// exactly one disposition, none double-counted. ✓
//
// Builder test-name contract (enforced by C447_004/005/006/008; also in the
// agent-mailbox handoff):
//
//	auto-sentinel unit test name contains  "Agy" and "AutoSentinel"
//	unknown-tier  unit test name contains  "Agy" and "UnknownTier"
//	repl seed-path test name contains      "SeedsREPL" or "REPLSeed"
//	parity test names contain              "Parity", subtests named by
//	                                       manifest base (e.g. "agy-tmux")
//	integration launch test name contains  "AgyTierDeep"
//
// RED strategy (verified in test-report.md "RED Run Output"): C447_001/002/
// 005 fail on the current manifest (channel=noop, flat map). C447_004/006/
// 007/008 fail via the no-matching-tests guard (requireTestsRan — a bare
// `go test -run NoMatch` exits 0, the degenerate-predicate trap). C447_010/
// 011/012 fail on the current doc (no channel table, no Realizer/sentinel
// statements). C447_003 and C447_009 are pre-existing GREEN pins: C447_003 is
// trivially green while the channel is noop but becomes the load-bearing
// negative the moment C447_001 forces a non-noop channel (the pair is only
// jointly satisfiable by a correct wiring); C447_009 pins no-regression.
//
// Adversarial diversity (skills/adversarial-testing SKILL §6):
//
//	Negative:   C447_003 (auto sentinel must emit nothing even once the
//	            channel is live), C447_007 (synthetic multi-model noop
//	            manifest must be REJECTED), C447_011 (no doc table row may
//	            say noop)
//	Edge/OOD:   C447_002 (flat map = boundary of "translates"), C447_005
//	            (channel-conditional seed-path coverage), C447_010's glob
//	            (a future *-tmux manifest without a doc row fails)
//	Semantic:   C447_004 (unknown-tier behavior is distinct from happy-path
//	            emission), C447_008 (model reaching the PANE is distinct from
//	            Realize() emitting it), C447_012 (doc states the seam/rules,
//	            not merely a table)
package cycle447

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const bridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"

// neutralizeCatalog points the live-catalog overlay at an empty directory
// once for the whole predicate package, so every manifest assertion below
// tests the manifest's OWN tier-map defaults (Task 1C: sane behavior when the
// catalog is absent/stale) instead of whatever live catalog the audit host
// happens to hold. Builder-side unit tests cover the overlay path.
var neutralizeCatalog = sync.OnceFunc(func() {
	dir, err := os.MkdirTemp("", "cycle447-empty-catalog-")
	if err == nil {
		bridge.SetModelCatalogDirFn(func() string { return dir })
	}
})

func agyManifest(t *testing.T) bridge.Manifest {
	t.Helper()
	neutralizeCatalog()
	m, err := bridge.LoadManifest("agy-tmux")
	if err != nil {
		t.Fatalf("LoadManifest(agy-tmux): %v", err)
	}
	return m
}

func runGoTest(t *testing.T, runFilter string, race, verbose bool, pkgs ...string) (out string, code int) {
	t.Helper()
	args := []string{"test", "-count=1"}
	if race {
		args = append(args, "-race")
	}
	if verbose {
		args = append(args, "-v")
	}
	if runFilter != "" {
		args = append(args, "-run", runFilter)
	}
	args = append(args, pkgs...)
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", args...)
	return stdout + "\n" + stderr, code
}

// requireTestsRan closes the degenerate-predicate trap: `go test -run X` with
// no matching test exits 0 with "no tests to run", which would green a
// predicate on unwritten work.
func requireTestsRan(t *testing.T, out string, min int) {
	t.Helper()
	if strings.Contains(out, "no tests to run") {
		t.Errorf("no tests matched the -run filter (\"no tests to run\") — required tests are unwritten")
		return
	}
	if got := strings.Count(out, "=== RUN"); got < min {
		t.Errorf("only %d test(s) ran, need >= %d", got, min)
	}
}

// TestC447_001_AgyDeepTierRealizesEffectiveChannel (T1 AC2, positive,
// behavioral): the agy-tmux manifest must declare a non-noop model_tier
// channel, and Realize() at tier=deep must carry the manifest's deep model
// into the Realization (launch flag pair or REPL input line). RED today:
// channel is "noop" → nothing is emitted.
func TestC447_001_AgyDeepTierRealizesEffectiveChannel(t *testing.T) {
	m := agyManifest(t)
	spec, ok := m.Params["model_tier"]
	if !ok {
		t.Fatalf("agy-tmux manifest has no params.model_tier entry")
	}
	switch spec.Channel {
	case "flag", "repl":
		deep := m.ModelTierMap["deep"]
		if deep == "" {
			t.Fatalf("agy-tmux model_tier_map has no deep entry")
		}
		r := bridge.Realize(m, bridge.LaunchIntent{ModelTier: "deep"})
		emitted := strings.Join(append(append([]string{}, r.LaunchFlags...), r.REPLInput...), "\n")
		if !strings.Contains(emitted, deep) {
			t.Errorf("Realize(agy-tmux, tier=deep) did not emit the deep model %q via channel %q\nLaunchFlags=%v REPLInput=%v",
				deep, spec.Channel, r.LaunchFlags, r.REPLInput)
		}
	case "picker":
		// Picker-select fallback path: delegate to the builder's picker
		// selection tests (modelquery/CaptureModelPicker seam extension).
		out, code := runGoTest(t, "AgyPickerSelect", true, true, bridgePkg)
		requireTestsRan(t, out, 1)
		if code != 0 {
			t.Errorf("picker channel declared but AgyPickerSelect tests fail (exit=%d)\n%s", code, out)
		}
	default:
		t.Errorf("agy-tmux params.model_tier.channel = %q — must be an effective channel (flag|repl|picker), noop silently drops the tier", spec.Channel)
	}
}

// TestC447_002_AgyTierMapHasDistinctModels (T1 AC3, edge — the flat map is
// the boundary case of "translates"): the manifest's own offline defaults
// must offer >= 2 distinct models across fast/balanced/deep, so tier choice
// is meaningful even when the live catalog is absent/stale. RED today: all
// three tiers map to gemini-3.5-flash.
// acs-predicate: config-check — the distinct per-tier defaults ARE the
// manifest-config deliverable of Task 1C; asserted on the loaded manifest
// with the catalog overlay neutralized.
func TestC447_002_AgyTierMapHasDistinctModels(t *testing.T) {
	m := agyManifest(t)
	distinct := map[string]struct{}{}
	for _, tier := range []string{"fast", "balanced", "deep"} {
		v := m.ModelTierMap[tier]
		if v == "" {
			t.Errorf("agy-tmux model_tier_map missing tier %q", tier)
			continue
		}
		distinct[v] = struct{}{}
	}
	if len(distinct) < 2 {
		t.Errorf("agy-tmux model_tier_map is flat (%v) — need >= 2 distinct per-tier models", m.ModelTierMap)
	}
}

// TestC447_003_AgyAutoSentinelOmitted (T1 AC4, negative): "auto" is the
// loop's resolve-me sentinel, never a concrete model (cycle-262). Whatever
// channel Task 1 wires, tier=auto must emit NO model realization for agy.
// Pre-existing GREEN pin today (noop emits nothing for every tier) — it
// becomes the load-bearing negative the moment C447_001 forces a non-noop
// channel: the pair is only jointly satisfiable by a correct wiring.
func TestC447_003_AgyAutoSentinelOmitted(t *testing.T) {
	m := agyManifest(t)
	r := bridge.Realize(m, bridge.LaunchIntent{ModelTier: "auto"})
	if len(r.REPLInput) != 0 {
		t.Errorf("tier=auto must not seed REPL input, got %v", r.REPLInput)
	}
	for _, tok := range r.LaunchFlags {
		if tok == "auto" {
			t.Errorf("tier=auto leaked the sentinel into LaunchFlags: %v", r.LaunchFlags)
		}
	}
}

// TestC447_004_AgyChannelUnitTestsGreen (T1 AC5, semantic): the builder's
// red-first unit tests for the new agy channel must exist and pass, and must
// include the named auto-sentinel and unknown-tier tests (scout verifiableBy:
// `go test -race -run 'Agy' ./internal/bridge/` all PASS including named
// auto-sentinel and unknown-tier tests). Name contract: substrings
// "Agy"+"AutoSentinel" and "Agy"+"UnknownTier". RED today: no such tests.
func TestC447_004_AgyChannelUnitTestsGreen(t *testing.T) {
	out, code := runGoTest(t, "Agy", true, true, bridgePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("-run 'Agy' bridge tests failed (exit=%d)\n%s", code, out)
	}
	for _, pat := range []string{`=== RUN\s+\S*Agy\S*AutoSentinel`, `=== RUN\s+\S*Agy\S*UnknownTier`} {
		if !regexp.MustCompile(pat).MatchString(out) {
			t.Errorf("required named test missing from -run 'Agy' output: %s", pat)
		}
	}
}

// TestC447_005_AgyReplSeedPathCovered (T1 AC6, edge, conditional): if the
// probe lands on the repl channel (the scout's expected outcome), the
// previously consumer-less REPLInput seed path in driver_tmux_repl.go gets
// its first production consumer and the missing seed-path test is owed (goal
// B.2; name contract: "SeedsREPL" or "REPLSeed"). A flag/picker outcome makes
// the seed path not-applicable (SKIP, covered by C447_001's branch instead).
// RED today: channel is still noop.
func TestC447_005_AgyReplSeedPathCovered(t *testing.T) {
	m := agyManifest(t)
	switch ch := m.Params["model_tier"].Channel; ch {
	case "repl":
		out, code := runGoTest(t, "SeedsREPL|REPLSeed", true, true, bridgePkg)
		requireTestsRan(t, out, 1)
		if code != 0 {
			t.Errorf("repl channel wired but seed-path tests fail (exit=%d)\n%s", code, out)
		}
	case "flag", "picker":
		t.Skipf("channel=%s — repl seed path not the chosen channel", ch)
	default:
		t.Errorf("agy-tmux model_tier channel = %q — still noop, no effective channel wired", ch)
	}
}

// tmuxManifestBases globs the WORKTREE's embedded-manifest sources so the
// parity and doc predicates are completeness-checked against what actually
// ships — a future *-tmux CLI added without a parity subtest or doc row
// fails, no hardcoded CLI list.
func tmuxManifestBases(t *testing.T) []string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	files, err := filepath.Glob(filepath.Join(root, "go", "internal", "bridge", "manifests", "*-tmux.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no *-tmux.json manifests found under %s (err=%v)", root, err)
	}
	bases := make([]string, 0, len(files))
	for _, f := range files {
		bases = append(bases, strings.TrimSuffix(filepath.Base(f), ".json"))
	}
	return bases
}

// TestC447_006_TmuxMatrixParityAllEffective (T2 AC1, positive): the parity
// pin must exist — one subtest per embedded *-tmux manifest asserting the CLI
// translates intent.ModelTier through SOME effective channel (flag emits the
// flag+value, repl emits REPLInput, ollama is classified effective-positional
// — never force-migrated). Subtest name contract: the manifest base name
// (e.g. "agy-tmux") appears in the -v output. RED today: no Parity tests in
// the bridge package.
func TestC447_006_TmuxMatrixParityAllEffective(t *testing.T) {
	out, code := runGoTest(t, "Parity", true, true, bridgePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("-run 'Parity' bridge tests failed (exit=%d)\n%s", code, out)
	}
	for _, base := range tmuxManifestBases(t) {
		if !strings.Contains(out, base) {
			t.Errorf("parity run has no subtest for manifest %q", base)
		}
	}
}

// TestC447_007_ParityNoopRejectionNegativeFixture (T2 AC2, negative — the
// value of the pin): a synthetic multi-model manifest with channel:"noop"
// must be REJECTED by the parity rule. Without this fixture the parity test
// cannot distinguish "every CLI passes" from "the rule accepts anything".
// Name contract: a Parity subtest whose name contains "noop"/"Noop". RED
// today with C447_006.
func TestC447_007_ParityNoopRejectionNegativeFixture(t *testing.T) {
	out, code := runGoTest(t, "Parity", true, true, bridgePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("-run 'Parity' bridge tests failed (exit=%d)\n%s", code, out)
	}
	if !regexp.MustCompile(`(?i)=== RUN\s+\S*parity\S*noop|=== RUN\s+\S*noop`).MatchString(out) {
		t.Errorf("no noop-rejection negative subtest in the parity run — the pin is happy-path-only")
	}
}

// TestC447_008_AgyDeepLaunchCarriesModelToPane (T2 AC3, semantic —
// integration-style): a dispatched agy launch at tier=deep must carry the
// RESOLVED deep model to the pane (the model reaching the pane is a distinct
// property from Realize() emitting it — seed timing vs trust-prompt
// auto-responder is hypothesis H3). Name contract: test name contains
// "AgyTierDeep". Must actually run and PASS (an all-SKIP result proves
// nothing). RED today: no such test.
func TestC447_008_AgyDeepLaunchCarriesModelToPane(t *testing.T) {
	out, code := runGoTest(t, "AgyTierDeep", true, true, bridgePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("AgyTierDeep integration test failed (exit=%d)\n%s", code, out)
	}
	if !strings.Contains(out, "--- PASS") {
		t.Errorf("AgyTierDeep ran but nothing PASSed (all skipped?) — pane delivery unproven\n%s", out)
	}
}

// TestC447_009_BridgeRegressionVetAndRace (T2 AC4, regression, pre-existing
// GREEN pin): the touched package must stay vet-clean and -race green —
// claude/codex/ollama translation behavior byte-identical is a hard
// constraint of the goal, and cycle-413 taught that a cycle can pass its own
// scoped checks while breaking repo CI.
func TestC447_009_BridgeRegressionVetAndRace(t *testing.T) {
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", "vet", bridgePkg)
	if code != 0 {
		t.Errorf("go vet %s exit=%d\n%s%s", bridgePkg, code, stdout, stderr)
	}
	out, code := runGoTest(t, "", true, false, bridgePkg)
	if code != 0 {
		t.Errorf("bridge -race suite exit=%d\n%s", code, out)
	}
}

// docPath resolves the model-discovery doc inside the worktree.
func docPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "model-discovery-and-catalog.md")
}

// manifestChannel reads params.model_tier.channel straight from the worktree
// manifest JSON (the shipped SSOT the doc rows must project).
func manifestChannel(t *testing.T, base string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), "go", "internal", "bridge", "manifests", base+".json"))
	if err != nil {
		t.Fatalf("read manifest %s: %v", base, err)
	}
	var m struct {
		Params map[string]struct {
			Channel string `json:"channel"`
		} `json:"params"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parse manifest %s: %v", base, err)
	}
	return m.Params["model_tier"].Channel
}

// TestC447_010_DocChannelTableMatchesManifests (T3 AC1, positive + glob-driven
// edge): the doc must carry a per-CLI channel table where every *-tmux
// manifest has a row stating its ACTUAL channel — a projection cross-checked
// against the manifests, never a hand-trusted second mapping. ollama's
// designed noop is documented as "positional" (its model is the positional
// arg of `ollama run <model>`, driver_ollamatmux.go). RED today: no table.
func TestC447_010_DocChannelTableMatchesManifests(t *testing.T) {
	doc := docPath(t)
	for _, base := range tmuxManifestBases(t) {
		cli := strings.TrimSuffix(base, "-tmux")
		expect := manifestChannel(t, base)
		if expect == "noop" {
			expect = "positional"
		}
		if !acsassert.LineContainsAll(doc, cli, expect) {
			t.Errorf("doc has no table row pairing %q with its actual channel %q", cli, expect)
		}
	}
}

// TestC447_011_NoTableRowDocumentsNoop (T3 AC2, negative): no channel-table
// row may document a CLI as "noop" — the goal's cycle-level acceptance is
// zero silent-noop cells. Scoped to markdown table rows (lines starting with
// "|") so the doc may still NARRATE the noop history in prose. Also requires
// the agy row to exist, so this is RED today (no table at all), not
// vacuously green.
func TestC447_011_NoTableRowDocumentsNoop(t *testing.T) {
	raw, err := os.ReadFile(docPath(t))
	if err != nil {
		t.Fatalf("read doc: %v", err)
	}
	agyRow := false
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		if strings.Contains(line, "agy") {
			agyRow = true
		}
		if strings.Contains(line, "noop") {
			t.Errorf("channel-table row documents a noop cell: %s", strings.TrimSpace(line))
		}
	}
	if !agyRow {
		t.Errorf("doc has no channel-table row for agy")
	}
}

// TestC447_012_DocStatesSeamVocabAndSentinelRule (T3 AC3, semantic): beyond
// the table, the doc must state the Realizer as the single translation seam,
// the fast/balanced/deep tier vocabulary, the auto-sentinel omission rule,
// and the catalog-overlay resolution order. RED today: "Realizer" and the
// sentinel rule are absent (vocabulary and overlay are pre-existing pins).
func TestC447_012_DocStatesSeamVocabAndSentinelRule(t *testing.T) {
	doc := docPath(t)
	acsassert.FileContains(t, doc, "Realizer")
	acsassert.FileMatchesRegex(t, doc, `(?i)auto.{0,120}(sentinel|omit)`)
	for _, vocab := range []string{"fast", "balanced", "deep"} {
		acsassert.FileContains(t, doc, vocab)
	}
	if !acsassert.FileContainsAny(doc, "overlay", "Overlay") {
		t.Errorf("doc does not state the catalog-overlay resolution order")
	}
}
