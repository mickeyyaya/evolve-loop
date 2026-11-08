//go:build acs

package cycle2225

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageevidence"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var now = time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

func clock() time.Time { return now }

func usageScreen(scope string, pct int, resets string) string {
	return strings.Join([]string{
		"Current session",
		"████                                               7% used",
		"Resets 4h",
		"Current week (all models)",
		"████████████████████████████████                   64% used",
		"Resets 90h",
		"Current week (" + scope + ")",
		"██████████████████████████████████████████████████ " + fmt.Sprint(pct) + "% used",
		"Resets " + resets,
		"",
		"Esc to cancel",
	}, "\n")
}

func allModelsExhaustedScreen() string {
	return strings.Join([]string{
		"Current session",
		"████                                               7% used",
		"Resets 4h",
		"Current week (all models)",
		"██████████████████████████████████████████████████ 100% used",
		"Resets 90h",
		"",
		"Esc to cancel",
	}, "\n")
}

type probed struct {
	root string
	log  string
}

func probeClaude(t *testing.T, screen string) probed {
	t.Helper()
	return probeClaudeReading(t, screen, func(family, text string) []quotastate.UsageWindow { return bridge.UsageWindows(family, text, now) }, clock)
}

func probeClaudeThroughTheProductionReader(t *testing.T, screen string) probed {
	t.Helper()
	return probeClaudeReading(t, screen, usageevidence.ReadWindowsNow, time.Now)
}

func probeClaudeReading(t *testing.T, screen string, read func(family, text string) []quotastate.UsageWindow, storeClock func() time.Time) probed {
	t.Helper()
	root := t.TempDir()
	var log bytes.Buffer
	prober := &usageprobe.Prober{
		Families: []string{"claude"},
		Probe:    func(context.Context, string) (string, error) { return screen, nil },
		Classify: func(string, string) bool { return false },
		Windows:  read,
		Store:    clihealth.NewStore(root, storeClock),
		Log:      &log,
	}
	prober.Run(context.Background())
	return probed{root: root, log: log.String()}
}

func claudeChain(tier string) llmroute.Plan {
	return llmroute.Plan{Candidates: []string{"claude-tmux", "agy-claude-tmux", "codex-tmux"}, Tiers: []string{tier}}
}

func runNamedTests(t *testing.T, pkg string, names []string) {
	t.Helper()
	moduleDir, err := acsassert.ModuleDir(".")
	if err != nil {
		t.Fatalf("RED: module dir: %v", err)
	}
	out, errOut, code, runErr := acsassert.SubprocessOutput("go", "-C", moduleDir, "test", "-count=1", "-v", pkg,
		"-run", "^("+strings.Join(names, "|")+")$")
	if runErr != nil || code != 0 {
		t.Fatalf("RED: %s tests exit=%d err=%v\n%s", pkg, code, runErr, acsassert.OutputTail(out+errOut, 30))
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("RED: %s did not run and pass in %s", name, pkg)
		}
	}
}

func TestC2225_001_AnExhaustedOpusWeekBenchesClaudeOpusUntilItsResetAndNotTheFamily(t *testing.T) {
	run := probeClaude(t, usageScreen("Opus", 100, "5h"))

	store := clihealth.NewStore(run.root, clock)
	bench, ok := store.ActiveModels()[clihealth.Entry{Family: "claude", Model: "Opus"}.Key()]
	if !ok {
		t.Fatalf("RED: the Opus week did not bench claude/Opus: %v\n%s", store.ActiveModels(), run.log)
	}
	if lo, hi := now.Add(5*time.Hour), now.Add(5*time.Hour+10*time.Minute); !bench.BenchedUntil.After(lo) || bench.BenchedUntil.After(hi) {
		t.Errorf("RED: benched until %v, want the Opus week's 5h reset", bench.BenchedUntil)
	}
	if len(store.Active()) != 0 {
		t.Errorf("RED: the claude family bench changed: %v", store.Active())
	}
}

func TestC2225_002_AnOpusWeekBelowTheCapBenchesNothingAndAnAllModelsCapBenchesTheFamilyOnly(t *testing.T) {
	below := clihealth.NewStore(probeClaude(t, usageScreen("Opus", 60, "5h")).root, clock)
	if len(below.ActiveModels()) != 0 || len(below.Active()) != 0 {
		t.Errorf("RED: an Opus week at 60%% benched something: models=%v families=%v", below.ActiveModels(), below.Active())
	}

	capped := clihealth.NewStore(probeClaude(t, allModelsExhaustedScreen()).root, clock)
	if _, ok := capped.Active()["claude"]; !ok {
		t.Errorf("RED: an exhausted all-models week must bench the claude family: %v", capped.Active())
	}
	if len(capped.ActiveModels()) != 0 {
		t.Errorf("RED: an all-models cap wrote a model bench: %v", capped.ActiveModels())
	}
}

func TestC2225_003_AnOpusModelBenchDemotesClaudeOnDeepAndKeepsItFirstOnBalancedAndFast(t *testing.T) {
	run := probeClaude(t, usageScreen("Opus", 100, "5h"))

	deep := bridgechain.ApplyCLIHealthBench(run.root, "audit", claudeChain("deep"), clock, nil)
	if want := []string{"agy-claude-tmux", "codex-tmux", "claude-tmux"}; !slices.Equal(deep.Candidates, want) {
		t.Errorf("RED: deep chain = %v, want %v", deep.Candidates, want)
	}
	for _, tier := range []string{"balanced", "fast"} {
		out := bridgechain.ApplyCLIHealthBench(run.root, "audit", claudeChain(tier), clock, nil)
		if want := []string{"claude-tmux", "agy-claude-tmux", "codex-tmux"}; !slices.Equal(out.Candidates, want) {
			t.Errorf("RED: %s chain = %v, want %v: only the tiers that resolve to Opus are demoted", tier, out.Candidates, want)
		}
	}
}

func TestC2225_004_AnOpusModelBenchPastItsResetDemotesNothing(t *testing.T) {
	run := probeClaude(t, usageScreen("Opus", 100, "5h"))
	later := func() time.Time { return now.Add(6 * time.Hour) }

	out := bridgechain.ApplyCLIHealthBench(run.root, "audit", claudeChain("deep"), later, nil)
	if want := []string{"claude-tmux", "agy-claude-tmux", "codex-tmux"}; !slices.Equal(out.Candidates, want) {
		t.Errorf("RED: deep chain = %v after the Opus reset, want %v", out.Candidates, want)
	}
}

func TestC2225_005_AScopeWordTheManifestDoesNotMapBenchesNothingAndKeepsTheWarn(t *testing.T) {
	run := probeClaudeThroughTheProductionReader(t, usageScreen("Fable", 100, "5h"))

	for _, tier := range []string{"fast", "balanced", "deep", "top"} {
		out := bridgechain.ApplyCLIHealthBench(run.root, "audit", claudeChain(tier), time.Now, nil)
		if want := []string{"claude-tmux", "agy-claude-tmux", "codex-tmux"}; !slices.Equal(out.Candidates, want) {
			t.Errorf("RED: %s chain = %v, want %v: model_match maps no Fable", tier, out.Candidates, want)
		}
	}
	if !strings.Contains(run.log, "WARN") || !strings.Contains(run.log, "Fable") {
		t.Errorf("RED: the unmapped drained window lost its WARN:\n%s", run.log)
	}
	store := clihealth.NewStore(run.root, nil)
	if len(store.Active()) != 0 || len(store.ActiveModels()) != 0 {
		t.Errorf("RED: an unmapped word benched: families=%v models=%v", store.Active(), store.ActiveModels())
	}
	if strings.Contains(run.log, "benched claude/Fable") {
		t.Errorf("RED: the log claims a bench for an unmapped word:\n%s", run.log)
	}
}

func explainThroughTheEvidencePath(t *testing.T, scope string) (*clihealth.Store, string) {
	t.Helper()
	root := t.TempDir()
	var log bytes.Buffer
	explain := usageevidence.New(usageevidence.Options{
		ProjectRoot: root, EvolveDir: filepath.Join(root, ".evolve"), Act: true, Log: &log,
		Probe: func(context.Context, string) (string, error) { return usageScreen(scope, 100, "5h"), nil },
	})
	explain(context.Background(), "claude-tmux", time.Time{})
	return clihealth.NewStore(root, nil), log.String()
}

func TestC2225_006_TheEvidencePathBenchesAMappedWordAndKeepsTheWarnForAnUnmappedOne(t *testing.T) {
	unmapped, unmappedLog := explainThroughTheEvidencePath(t, "Fable")
	if len(unmapped.ActiveModels()) != 0 || len(unmapped.Active()) != 0 {
		t.Errorf("RED: the evidence path benched an unmapped word: models=%v families=%v\n%s", unmapped.ActiveModels(), unmapped.Active(), unmappedLog)
	}
	if !strings.Contains(unmappedLog, "WARN") || !strings.Contains(unmappedLog, "Fable") {
		t.Errorf("RED: the unmapped drained window lost its WARN:\n%s", unmappedLog)
	}

	mapped, mappedLog := explainThroughTheEvidencePath(t, "Opus")
	if _, ok := mapped.ActiveModels()[clihealth.Entry{Family: "claude", Model: "Opus"}.Key()]; !ok {
		t.Errorf("RED: the mapped Opus week did not bench claude/Opus through the evidence path: %v\n%s", mapped.ActiveModels(), mappedLog)
	}
	if len(mapped.Active()) != 0 {
		t.Errorf("RED: the claude family bench changed: %v", mapped.Active())
	}
}

func TestC2225_007_TheProductionUsageProberBenchesAMappedWordAndNoUnmappedOne(t *testing.T) {
	runNamedTests(t, "./cmd/evolve", []string{
		"TestNewUsageProber_AnExhaustedFableWeekBenchesNoModelThroughTheProductionProber",
		"TestNewUsageProber_AnExhaustedOpusWeekBenchesClaudeOpusThroughTheProductionProber",
	})
}

func TestC2225_008_TheCanaryProbesAnExpiredModelBenchWithItsModelAndClearsItOnSuccess(t *testing.T) {
	runNamedTests(t, "./cmd/evolve", []string{
		"TestCanary_AnExpiredModelBenchIsProbedWithItsModel",
		"TestRunPreWaveProbes_ProbesAnExpiredModelBenchBeforeTheWaveAdmitsALane",
	})
}

func TestC2225_009_TheCanaryKeepsOrReBenchesAModelWhenTheProbeDoesNotProveRecovery(t *testing.T) {
	runNamedTests(t, "./cmd/evolve", []string{
		"TestCanary_AModelWallReBenchesThatModelWithAStrike",
		"TestCanary_AModelProbeThatBootedAnotherModelKeepsTheBenchUntouched",
		"TestCanary_AModelBenchStillInForceIsNotProbed",
		"TestCanary_AModelProbeRefusedByTheSeatTokenHolderKeepsTheBenchUntouched",
		"TestCanary_TheFamilyCanaryNeverProbesAModelBenchAsAFamily",
	})
}

func TestC2225_010_TheScopeWordToModelMapIsManifestDataAndDecidesAlone(t *testing.T) {
	manifest, err := bridge.LoadManifest("claude-tmux")
	if err != nil {
		t.Fatalf("RED: load claude-tmux: %v", err)
	}
	usage, ok := manifest.Control("usage")
	if !ok || usage.Windows == nil {
		t.Fatal("RED: claude-tmux declares no usage windows")
	}
	if !usage.Windows.ModelMatches("Opus", "opus") || !usage.Windows.ModelMatches("Opus", "claude-opus-5-5") {
		t.Errorf("RED: the manifest model_match does not map Opus to Opus models: %v", usage.Windows.ModelMatch)
	}
	if usage.Windows.ModelMatches("Opus", "sonnet") || usage.Windows.ModelMatches("Fable", "opus") {
		t.Errorf("RED: the manifest model_match over-matches: %v", usage.Windows.ModelMatch)
	}
	if model, mapped := bridge.UsageModelFor("claude-tmux", "Opus"); !mapped || !strings.Contains(strings.ToLower(model), "opus") {
		t.Errorf("RED: UsageModelFor(claude-tmux, Opus) = %q, %v; want the manifest's Opus tier model", model, mapped)
	}
	if _, mapped := bridge.UsageModelFor("claude-tmux", "Fable"); mapped {
		t.Error("RED: UsageModelFor maps Fable though the manifest declares no such word")
	}

	spec := quotastate.WindowSpec{ValueRegex: `(?P<pct>\d+)%`, ValueDirection: quotastate.DirectionUsed, ModelMatch: map[string]string{"Fable": "(?i)sonnet"}}
	if !spec.ModelMatches("Fable", "claude-sonnet-5-5") || spec.ModelMatches("Opus", "claude-opus-5-5") {
		t.Error("RED: a map declared as data must alone decide which word matches which model")
	}
	spec.ModelMatch = map[string]string{"Opus": "("}
	if err := spec.Validate(); err == nil {
		t.Error("RED: an uncompilable model_match regex passed validation")
	}
}

func TestC2225_011_NoGoSourceOutsideTestsHoldsAScopeWordToModelTable(t *testing.T) {
	moduleDir, err := acsassert.ModuleDir(".")
	if err != nil {
		t.Fatalf("RED: module dir: %v", err)
	}
	files, err := acsassert.WalkModule(moduleDir, func(rel string) bool {
		return strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go") &&
			(strings.HasPrefix(rel, "internal/") || strings.HasPrefix(rel, "cmd/"))
	})
	if err != nil {
		t.Fatalf("RED: walk module: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("RED: the walk found no Go source: the check read nothing")
	}
	for _, rel := range files {
		raw, readErr := os.ReadFile(filepath.Join(moduleDir, rel))
		if readErr != nil {
			t.Fatalf("RED: read %s: %v", rel, readErr)
		}
		body := string(raw)
		for _, banned := range []string{`"(?i)opus"`, `"Opus":`, `"Fable":`} {
			if strings.Contains(body, banned) {
				t.Errorf("RED: %s holds %s: the scope-word map belongs in the manifest", rel, banned)
			}
		}
	}
}

func TestC2225_012_TheUnmappedRuleHasOneMechanismTheReadersFlagAndNoProberSeam(t *testing.T) {
	if _, seam := reflect.TypeOf(usageprobe.Prober{}).FieldByName("ModelMapped"); seam {
		t.Error("RED: usageprobe.Prober still carries a ModelMapped seam beside the reader's Unmapped flag: two mechanisms decide one rule")
	}

	unmappedFable := usageprobe.Prober{
		Families: []string{"claude"},
		Probe:    func(context.Context, string) (string, error) { return usageScreen("Fable", 100, "5h"), nil },
		Classify: func(string, string) bool { return false },
		Windows:  usageevidence.ReadWindowsNow,
	}
	var log bytes.Buffer
	unmappedFable.Log = &log
	unmappedFable.Store = clihealth.NewStore(t.TempDir(), nil)
	unmappedFable.Run(context.Background())
	if len(unmappedFable.Store.ActiveModels()) != 0 || len(unmappedFable.Store.Active()) != 0 {
		t.Errorf("RED: the reader's flag alone must keep an unmapped word unbenched: %v %v\n%s", unmappedFable.Store.ActiveModels(), unmappedFable.Store.Active(), log.String())
	}

	flagged := quotastate.UsageWindow{Family: "claude", Scope: "Current week (Fable)", Model: "Fable", Exhausted: true, Unmapped: true}
	var flagLog bytes.Buffer
	byFlag := usageprobe.Prober{
		Families: []string{"claude"},
		Probe:    func(context.Context, string) (string, error) { return "pane", nil },
		Classify: func(string, string) bool { return false },
		Windows:  func(string, string) []quotastate.UsageWindow { return []quotastate.UsageWindow{flagged} },
		Store:    clihealth.NewStore(t.TempDir(), nil),
		Log:      &flagLog,
	}
	byFlag.Run(context.Background())
	if len(byFlag.Store.ActiveModels()) != 0 || !strings.Contains(flagLog.String(), "WARN") {
		t.Errorf("RED: a window flagged Unmapped must bench nothing and keep its WARN: %v\n%s", byFlag.Store.ActiveModels(), flagLog.String())
	}
}

func TestC2225_013_EveryExportOfTheTouchedEnforcedPackagesIsNamedByATest(t *testing.T) {
	moduleDir, err := acsassert.ModuleDir(".")
	if err != nil {
		t.Fatalf("RED: module dir: %v", err)
	}
	packages := []string{"./internal/bridge", "./internal/bridgechain", "./internal/clihealth", "./internal/quotastate",
		"./internal/repocontract", "./internal/usageevidence", "./internal/usageprobe"}
	for _, pkg := range packages {
		out, errOut, _, _ := acsassert.SubprocessOutput("go", "-C", moduleDir, "run", "./cmd/apicover", "-enforce", pkg)
		if !strings.Contains(out, "summary:") {
			t.Fatalf("RED: apicover on %s printed no summary, so it checked nothing:\n%s", pkg, acsassert.OutputTail(out+errOut, 20))
		}
		if strings.Contains(out, "UNCOVERED") {
			t.Errorf("RED: %s has an exported symbol no test names (CI api-coverage enforce fails):\n%s", pkg, acsassert.OutputTail(out, 12))
		}
	}
}
