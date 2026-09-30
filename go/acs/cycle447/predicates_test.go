//go:build acs

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
		out, code := runGoTest(t, "AgyPickerSelect", true, true, bridgePkg)
		requireTestsRan(t, out, 1)
		if code != 0 {
			t.Errorf("picker channel declared but AgyPickerSelect tests fail (exit=%d)\n%s", code, out)
		}
	default:
		t.Errorf("agy-tmux params.model_tier.channel = %q — must be an effective channel (flag|repl|picker), noop silently drops the tier", spec.Channel)
	}
}

// acs-predicate: config-check — the distinct per-tier defaults ARE the
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

func docPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "model-discovery-and-catalog.md")
}

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
