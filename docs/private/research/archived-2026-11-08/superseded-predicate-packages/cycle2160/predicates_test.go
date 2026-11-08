//go:build acs

package cycle2160

import (
	"bytes"
	"context"
	"path/filepath"
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

func usagePane(modelWord string) string {
	return strings.Join([]string{
		"Current session",
		"████                                               7% used",
		"Resets 4h",
		"Current week (all models)",
		"████████████████████████████████                   64% used",
		"Resets 90h",
		"Current week (" + modelWord + ")",
		"██████████████████████████████████████████████████ 100% used",
		"Resets 5h",
		"",
		"Esc to cancel",
	}, "\n")
}

type probed struct {
	root string
	log  string
}

func probeClaudeUsage(t *testing.T, pane string) probed {
	t.Helper()
	root := t.TempDir()
	var log bytes.Buffer
	prober := &usageprobe.Prober{
		Families: []string{"claude"},
		Probe:    func(context.Context, string) (string, error) { return pane, nil },
		Classify: func(string, string) bool { return false },
		Windows:  func(family, text string) []quotastate.UsageWindow { return bridge.UsageWindows(family, text, now) },
		ModelMapped: func(_, word string) bool {
			_, mapped := bridge.UsageModelFor("claude-tmux", word)
			return mapped
		},
		Store: clihealth.NewStore(root, clock),
		Log:   &log,
	}
	prober.Run(context.Background())
	return probed{root: root, log: log.String()}
}

func chain(tier string) llmroute.Plan {
	return llmroute.Plan{Candidates: []string{"claude-tmux", "agy-claude-tmux", "codex-tmux"}, Tiers: []string{tier}}
}

func TestC2160_001_AnExhaustedOpusWeekReadFromTheManifestBenchesClaudeOpusAndNotTheFamily(t *testing.T) {
	run := probeClaudeUsage(t, usagePane("Opus"))

	store := clihealth.NewStore(run.root, clock)
	bench, ok := store.ActiveModels()[clihealth.Entry{Family: "claude", Model: "Opus"}.Key()]
	if !ok {
		t.Fatalf("RED: the Opus week read through the claude manifest did not bench claude/Opus: %v\n%s", store.ActiveModels(), run.log)
	}
	if lo, hi := now.Add(5*time.Hour), now.Add(5*time.Hour+10*time.Minute); !bench.BenchedUntil.After(lo) || bench.BenchedUntil.After(hi) {
		t.Errorf("RED: benched until %v, want the Opus week's 5h reset", bench.BenchedUntil)
	}
	if len(store.Active()) != 0 {
		t.Errorf("RED: the claude family bench changed: %v", store.Active())
	}
}

func TestC2160_002_AnOpusModelBenchDemotesClaudeOnDeepAndKeepsItFirstOnBalanced(t *testing.T) {
	run := probeClaudeUsage(t, usagePane("Opus"))

	deep := bridgechain.ApplyCLIHealthBench(run.root, "audit", chain("deep"), clock, nil)
	balanced := bridgechain.ApplyCLIHealthBench(run.root, "audit", chain("balanced"), clock, nil)

	if want := []string{"agy-claude-tmux", "codex-tmux", "claude-tmux"}; !slices.Equal(deep.Candidates, want) {
		t.Errorf("RED: deep chain = %v, want %v", deep.Candidates, want)
	}
	if want := []string{"claude-tmux", "agy-claude-tmux", "codex-tmux"}; !slices.Equal(balanced.Candidates, want) {
		t.Errorf("RED: balanced chain = %v, want %v", balanced.Candidates, want)
	}
}

func TestC2160_003_AScopeWordTheManifestDoesNotMapBenchesNothingAndKeepsTheWarn(t *testing.T) {
	run := probeClaudeUsage(t, usagePane("Fable"))

	for _, tier := range []string{"fast", "balanced", "deep", "top"} {
		out := bridgechain.ApplyCLIHealthBench(run.root, "audit", chain(tier), clock, nil)
		if want := []string{"claude-tmux", "agy-claude-tmux", "codex-tmux"}; !slices.Equal(out.Candidates, want) {
			t.Errorf("RED: %s chain = %v, want %v: model_match maps no Fable", tier, out.Candidates, want)
		}
	}
	if !strings.Contains(run.log, "WARN") || !strings.Contains(run.log, "Fable") {
		t.Errorf("RED: the unmapped drained window lost its WARN:\n%s", run.log)
	}
	store := clihealth.NewStore(run.root, clock)
	if active := store.Active(); len(active) != 0 {
		t.Errorf("RED: an unmapped word benched a family: %v", active)
	}
	if models := store.ActiveModels(); len(models) != 0 {
		t.Errorf("RED: an unmapped word wrote a model bench row: %v\n%s", models, run.log)
	}
	if strings.Contains(run.log, "benched claude/Fable") {
		t.Errorf("RED: the log claims a bench for an unmapped word:\n%s", run.log)
	}
}

func TestC2160_004_TheCanaryProbesAnExpiredModelBenchWithItsModelAndClearsItOnSuccess(t *testing.T) {
	moduleDir, err := acsassert.ModuleDir(".")
	if err != nil {
		t.Fatalf("RED: module dir: %v", err)
	}
	out, errOut, code, runErr := acsassert.SubprocessOutput("go", "-C", moduleDir, "test", "-count=1", "-v", "./cmd/evolve",
		"-run", "^(TestCanary_AnExpiredModelBenchIsProbedWithItsModel|TestCanary_AModelWallReBenchesThatModelWithAStrike|TestRunPreWaveProbes_ProbesAnExpiredModelBenchBeforeTheWaveAdmitsALane)$")
	if runErr != nil || code != 0 {
		t.Fatalf("RED: canary tests exit=%d err=%v\n%s", code, runErr, acsassert.OutputTail(out+errOut, 30))
	}
	for _, name := range []string{"TestCanary_AnExpiredModelBenchIsProbedWithItsModel", "TestCanary_AModelWallReBenchesThatModelWithAStrike", "TestRunPreWaveProbes_ProbesAnExpiredModelBenchBeforeTheWaveAdmitsALane"} {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("RED: %s did not run and pass", name)
		}
	}
}

func TestC2160_005_TheScopeWordToModelMapIsManifestDataWithNoGoList(t *testing.T) {
	manifest, err := bridge.LoadManifest("claude-tmux")
	if err != nil {
		t.Fatalf("RED: load claude-tmux: %v", err)
	}
	usage, ok := manifest.Control("usage")
	if !ok || usage.Windows == nil {
		t.Fatal("RED: claude-tmux declares no usage windows")
	}
	if !usage.Windows.ModelMatches("Opus", "opus") || !usage.Windows.ModelMatches("Opus", "claude-opus-5-5") {
		t.Errorf("RED: the claude manifest's model_match does not map Opus to Opus models: %v", usage.Windows.ModelMatch)
	}
	if usage.Windows.ModelMatches("Opus", "sonnet") {
		t.Errorf("RED: the claude manifest's Opus entry matches a Sonnet model")
	}

	custom := quotastate.WindowSpec{ValueRegex: `(?P<pct>\d+)%`, ValueDirection: quotastate.DirectionUsed, ModelMatch: map[string]string{"Fable": "(?i)sonnet"}}
	if !custom.ModelMatches("Fable", "claude-sonnet-5-5") || custom.ModelMatches("Opus", "claude-opus-5-5") {
		t.Errorf("RED: a map declared as data must alone decide which word matches which model")
	}
	if err := (quotastate.WindowSpec{ValueRegex: `(?P<pct>\d+)%`, ValueDirection: quotastate.DirectionUsed, ModelMatch: map[string]string{"Opus": "("}}).Validate(); err == nil {
		t.Error("RED: an uncompilable model_match regex passed validation")
	}
}

func TestC2160_006_AProbeThatBootedAnotherModelKeepsTheModelBenchInsteadOfClearingIt(t *testing.T) {
	moduleDir, err := acsassert.ModuleDir(".")
	if err != nil {
		t.Fatalf("RED: module dir: %v", err)
	}
	const name = "TestCanary_AModelProbeThatBootedAnotherModelKeepsTheBenchUntouched"
	out, errOut, code, runErr := acsassert.SubprocessOutput("go", "-C", moduleDir, "test", "-count=1", "-v", "./cmd/evolve", "-run", "^"+name+"$")
	if runErr != nil || code != 0 {
		t.Fatalf("RED: exit=%d err=%v\n%s", code, runErr, acsassert.OutputTail(out+errOut, 30))
	}
	if !strings.Contains(out, "--- PASS: "+name) {
		t.Errorf("RED: %s did not run and pass", name)
	}
}

func TestC2160_007_TheProductionUsageProberReadsModelMatchFromTheManifestBeforeBenchingAModel(t *testing.T) {
	moduleDir, err := acsassert.ModuleDir(".")
	if err != nil {
		t.Fatalf("RED: module dir: %v", err)
	}
	names := []string{
		"TestNewUsageProber_WiresTheManifestModelMatchIntoTheProber",
		"TestNewUsageProber_AnExhaustedFableWeekBenchesNoModelThroughTheProductionProber",
		"TestNewUsageProber_AnExhaustedOpusWeekBenchesClaudeOpusThroughTheProductionProber",
	}
	out, errOut, code, runErr := acsassert.SubprocessOutput("go", "-C", moduleDir, "test", "-count=1", "-v", "./cmd/evolve",
		"-run", "^("+strings.Join(names, "|")+")$")
	if runErr != nil || code != 0 {
		t.Fatalf("RED: exit=%d err=%v\n%s", code, runErr, acsassert.OutputTail(out+errOut, 30))
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("RED: %s did not run and pass", name)
		}
	}
}

func explainThroughTheEvidencePath(t *testing.T, word string) (*clihealth.Store, string) {
	t.Helper()
	root := t.TempDir()
	var log bytes.Buffer
	explain := usageevidence.New(usageevidence.Options{
		ProjectRoot: root, EvolveDir: filepath.Join(root, ".evolve"), Act: true, Log: &log,
		Probe: func(context.Context, string) (string, error) { return usagePane(word), nil },
	})
	explain(context.Background(), "claude-tmux", time.Time{})
	return clihealth.NewStore(root, nil), log.String()
}

func TestC2160_008_AnUnmappedScopeWordBenchesNothingThroughTheEvidencePathAndKeepsTheWarn(t *testing.T) {
	store, log := explainThroughTheEvidencePath(t, "Fable")

	if models := store.ActiveModels(); len(models) != 0 {
		t.Errorf("RED: the evidence path wrote a model bench for a word model_match does not map: %v\n%s", models, log)
	}
	if len(store.Active()) != 0 {
		t.Errorf("RED: the evidence path benched a family: %v", store.Active())
	}
	if !strings.Contains(log, "WARN") || !strings.Contains(log, "Fable") {
		t.Errorf("RED: the unmapped drained window lost its WARN:\n%s", log)
	}
	if strings.Contains(log, "benched claude/Fable") {
		t.Errorf("RED: the log claims a bench for an unmapped word:\n%s", log)
	}
}

func TestC2160_009_AMappedScopeWordStillBenchesItsModelThroughTheEvidencePath(t *testing.T) {
	store, log := explainThroughTheEvidencePath(t, "Opus")

	if _, ok := store.ActiveModels()[clihealth.Entry{Family: "claude", Model: "Opus"}.Key()]; !ok {
		t.Errorf("RED: the mapped Opus week did not bench claude/Opus through the evidence path: %v\n%s", store.ActiveModels(), log)
	}
	if len(store.Active()) != 0 {
		t.Errorf("RED: the claude family bench changed: %v", store.Active())
	}
}

func TestC2160_010_AProberThatOnlyReadsWindowsThroughTheSharedReaderBenchesNoUnmappedWord(t *testing.T) {
	root := t.TempDir()
	var log bytes.Buffer
	prober := &usageprobe.Prober{
		Families: []string{"claude"},
		Probe:    func(context.Context, string) (string, error) { return usagePane("Fable"), nil },
		Classify: func(string, string) bool { return false },
		Windows:  usageevidence.ReadWindowsNow,
		Store:    clihealth.NewStore(root, nil),
		Log:      &log,
	}
	prober.Run(context.Background())

	if models := clihealth.NewStore(root, nil).ActiveModels(); len(models) != 0 {
		t.Errorf("RED: a Prober with no ModelMapped seam benched an unmapped word: %v\n%s", models, log.String())
	}
}
