package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

type modelProbeCall struct{ driver, model string }

func benchExpiredModel(t *testing.T, root, family, model string, strikes int) {
	t.Helper()
	now := time.Now()
	if err := clihealth.NewStore(root, nil).Bench(clihealth.Entry{
		Family: family, Model: model, Reason: clihealth.UsageProbePattern,
		BenchedAt: now.Add(-2 * time.Hour), BenchedUntil: now.Add(-time.Hour), Strikes: strikes,
	}); err != nil {
		t.Fatal(err)
	}
}

func recordingModelProbe(calls *[]modelProbeCall, rc int, pattern, scrollback string) modelLiveProbe {
	return func(driver, model string) (int, string, string) {
		*calls = append(*calls, modelProbeCall{driver, model})
		return rc, pattern, scrollback
	}
}

func modelBenchEntry(t *testing.T, root, family, model string) (clihealth.Entry, bool) {
	t.Helper()
	benches, _ := clihealth.NewStore(root, nil).Load()
	e, ok := benches[clihealth.Entry{Family: family, Model: model}.Key()]
	return e, ok
}

func TestCanary_AnExpiredModelBenchIsProbedWithItsModel(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	benchExpiredModel(t, root, "claude", "Opus", 1)
	var calls []modelProbeCall
	var out bytes.Buffer

	runCLIHealthModelCanary(context.Background(), root, recordingModelProbe(&calls, 0, "", ""), &out)

	if len(calls) != 1 || calls[0].driver != "claude-tmux" || !strings.EqualFold(calls[0].model, "opus") {
		t.Fatalf("probe calls = %v, want one probe of claude-tmux with the Opus model\n%s", calls, out.String())
	}
	if e, present := modelBenchEntry(t, root, "claude", "Opus"); present {
		t.Errorf("a clean model probe left the bench: %+v", e)
	}
}

func TestCanary_AModelWallReBenchesThatModelWithAStrike(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	benchExpiredModel(t, root, "claude", "Opus", 1)
	var calls []modelProbeCall
	var out bytes.Buffer

	runCLIHealthModelCanary(context.Background(), root, recordingModelProbe(&calls, 85, "rate_limit", "no reset hint in this pane"), &out)

	e, present := modelBenchEntry(t, root, "claude", "Opus")
	if !present {
		t.Fatalf("a walled model probe lost the bench\n%s", out.String())
	}
	if e.Strikes != 2 || e.Family != "claude" || e.Model != "Opus" {
		t.Errorf("re-bench = %+v, want claude/Opus at 2 strikes", e)
	}
	if !e.BenchedUntil.After(time.Now()) {
		t.Errorf("BenchedUntil = %v, want a future end", e.BenchedUntil)
	}
	if benches, _ := clihealth.NewStore(root, nil).Load(); len(benches) != 1 {
		t.Errorf("benches = %v; the wall must not add a family bench", benches)
	}
}

func TestCanary_AModelBenchStillInForceIsNotProbed(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, err := clihealth.NewStore(root, nil).BenchModelUntil("claude", "Opus",
		clihealth.Wall{Pattern: clihealth.UsageProbePattern, Evidence: "Opus week 100% used", Reset: time.Now().Add(5 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	var calls []modelProbeCall

	runCLIHealthModelCanary(context.Background(), root, recordingModelProbe(&calls, 0, "", ""), &bytes.Buffer{})

	if len(calls) != 0 {
		t.Errorf("probe calls = %v, want none before the Opus week resets", calls)
	}
	if _, present := modelBenchEntry(t, root, "claude", "Opus"); !present {
		t.Error("the model bench in force was cleared")
	}
}

func TestCanary_ACancelledContextLeavesTheModelBenchUntouched(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	benchExpiredModel(t, root, "claude", "Opus", 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var calls []modelProbeCall

	runCLIHealthModelCanary(ctx, root, recordingModelProbe(&calls, 0, "", ""), &bytes.Buffer{})

	if len(calls) != 0 {
		t.Errorf("probe calls = %v after cancel, want none", calls)
	}
	if _, present := modelBenchEntry(t, root, "claude", "Opus"); !present {
		t.Error("a cancelled canary cleared the bench")
	}
}

func TestCanary_TheFamilyCanaryNeverProbesAModelBenchAsAFamily(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	benchExpiredModel(t, root, "claude", "Opus", 1)
	probed := 0

	runCLIHealthCanary(context.Background(), root, func(string) (int, string, string) { probed++; return 0, "", "" }, &bytes.Buffer{})

	if probed != 0 {
		t.Errorf("the family canary probed %d time(s) for a model-only bench", probed)
	}
	if _, present := modelBenchEntry(t, root, "claude", "Opus"); !present {
		t.Error("the family canary cleared a model bench")
	}
}

func TestRunPreWaveProbes_ProbesAnExpiredModelBenchBeforeTheWaveAdmitsALane(t *testing.T) {
	root, evolveDir := preWaveProbeRoot(t, `{}`)
	benchExpiredModel(t, root, "claude", "Opus", 1)
	var calls []modelProbeCall
	prev := preWaveModelLiveProbeFn
	preWaveModelLiveProbeFn = func(context.Context, string, bridge.Deps) modelLiveProbe {
		return recordingModelProbe(&calls, 0, "", "")
	}
	t.Cleanup(func() { preWaveModelLiveProbeFn = prev })
	var log bytes.Buffer

	if err := runPreWaveProbes(context.Background(), root, evolveDir, nil, &log); err != nil {
		t.Fatalf("runPreWaveProbes: %v\n%s", err, log.String())
	}

	if len(calls) != 1 || calls[0].driver != "claude-tmux" || !strings.EqualFold(calls[0].model, "opus") {
		t.Fatalf("probe calls = %v, want the pre-wave step to probe claude-tmux with the Opus model\n%s", calls, log.String())
	}
	if _, present := modelBenchEntry(t, root, "claude", "Opus"); present {
		t.Error("the pre-wave canary left a recovered model bench in place")
	}
}

func TestCanary_AModelBenchOfAnUnknownFamilyIsLeftUntouched(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	benchExpiredModel(t, root, "nosuchfamily", "Opus", 1)
	var calls []modelProbeCall
	var out bytes.Buffer

	runCLIHealthModelCanary(context.Background(), root, recordingModelProbe(&calls, 0, "", ""), &out)

	if len(calls) != 0 {
		t.Errorf("probe calls = %v, want none for an unresolvable family", calls)
	}
	if _, present := modelBenchEntry(t, root, "nosuchfamily", "Opus"); !present {
		t.Errorf("an unresolvable family cleared its bench\n%s", out.String())
	}
}

func TestCanary_AScopeWordTheManifestDoesNotMapClearsItsBench(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	benchExpiredModel(t, root, "claude", "NoSuchWord", 1)
	var calls []modelProbeCall

	runCLIHealthModelCanary(context.Background(), root, recordingModelProbe(&calls, 0, "", ""), &bytes.Buffer{})

	if len(calls) != 0 {
		t.Errorf("probe calls = %v, want none for an unmapped word", calls)
	}
	if _, present := modelBenchEntry(t, root, "claude", "NoSuchWord"); present {
		t.Error("an unmapped scope word kept a bench that demotes nothing")
	}
}

func TestCanary_ACancelDuringTheModelProbeLeavesTheBenchAndStops(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	benchExpiredModel(t, root, "claude", "Opus", 1)
	ctx, cancel := context.WithCancel(context.Background())
	probe := func(string, string) (int, string, string) { cancel(); return 0, "", "" }

	runCLIHealthModelCanary(ctx, root, probe, &bytes.Buffer{})

	if _, present := modelBenchEntry(t, root, "claude", "Opus"); !present {
		t.Error("a probe cancelled midway cleared the bench")
	}
}

func TestCanary_AProbeFailureThatIsNotAWallClearsTheModelBench(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	benchExpiredModel(t, root, "claude", "Opus", 1)
	var calls []modelProbeCall
	var out bytes.Buffer

	runCLIHealthModelCanary(context.Background(), root, recordingModelProbe(&calls, 1, "", "boom"), &out)

	if len(calls) != 1 {
		t.Fatalf("probe calls = %v, want one", calls)
	}
	if _, present := modelBenchEntry(t, root, "claude", "Opus"); present {
		t.Errorf("a non-wall failure kept the bench\n%s", out.String())
	}
	if !strings.Contains(out.String(), "not a wall") {
		t.Errorf("log = %q, want the not-a-wall verdict", out.String())
	}
}

func TestCanary_ModelCanaryDoesNothingWhenCLIHealthIsOff(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"cli_health":{"enabled":false}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	benchExpiredModel(t, root, "claude", "Opus", 1)
	var calls []modelProbeCall

	runCLIHealthModelCanary(context.Background(), root, recordingModelProbe(&calls, 0, "", ""), &bytes.Buffer{})

	if len(calls) != 0 {
		t.Errorf("probe calls = %v with cli_health off, want none", calls)
	}
}

func TestCanary_AModelProbeThatBootedAnotherModelKeepsTheBenchUntouched(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	benchExpiredModel(t, root, "claude", "Opus", 2)
	before, _ := modelBenchEntry(t, root, "claude", "Opus")
	var calls []modelProbeCall
	var out bytes.Buffer

	runCLIHealthModelCanary(context.Background(), root, recordingModelProbe(&calls, bridge.ExitModelMismatch, "", "booted the wrong model"), &out)

	if len(calls) != 1 {
		t.Fatalf("probe calls = %v, want one", calls)
	}
	after, present := modelBenchEntry(t, root, "claude", "Opus")
	if !present {
		t.Fatalf("a probe that booted a model outside its family cleared the bench\n%s", out.String())
	}
	if after.Strikes != before.Strikes || !after.BenchedUntil.Equal(before.BenchedUntil) {
		t.Errorf("bench changed from %+v to %+v; a model mismatch is neither a wall nor a recovery", before, after)
	}
	if !strings.Contains(out.String(), "outside its family") || strings.Contains(out.String(), "not a wall") {
		t.Errorf("log = %q, want the kept-bench verdict naming the mismatch", out.String())
	}
}

func TestCanary_AModelProbeRefusedByTheSeatTokenHolderKeepsTheBenchUntouched(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	benchExpiredModel(t, root, "claude", "Opus", 2)
	before, _ := modelBenchEntry(t, root, "claude", "Opus")
	var calls []modelProbeCall
	var out bytes.Buffer

	runCLIHealthModelCanary(context.Background(), root, recordingModelProbe(&calls, bridge.ExitSeatTokenRefused, "", "seat token held"), &out)

	if len(calls) != 1 {
		t.Fatalf("probe calls = %v, want one", calls)
	}
	after, present := modelBenchEntry(t, root, "claude", "Opus")
	if !present {
		t.Fatalf("a probe refused by the seat token holder cleared the bench\n%s", out.String())
	}
	if after.Strikes != before.Strikes || !after.BenchedUntil.Equal(before.BenchedUntil) {
		t.Errorf("bench changed from %+v to %+v; a seat-token refusal is neither a wall nor a recovery", before, after)
	}
	if !strings.Contains(out.String(), "seat token") || strings.Contains(out.String(), "not a wall") {
		t.Errorf("log = %q, want the kept-bench verdict naming the seat token refusal", out.String())
	}
}
