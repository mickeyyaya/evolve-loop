//go:build acs

package cycle2106

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseregistrar"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func init() {
	absent := filepath.Join(os.TempDir(), "cycle2106-no-model-catalog")
	bridge.SetModelCatalogDirFn(func() string { return absent })
}

type recordedRun struct {
	binary string
	args   []string
}

type runRecorder struct {
	mu   sync.Mutex
	runs []recordedRun
	rc   int
}

func (r *runRecorder) run(_ context.Context, name, _ string, args, _ []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runs = append(r.runs, recordedRun{binary: name, args: slices.Clone(args)})
	if len(args) > 0 && args[0] == "--version" {
		_, _ = stdout.Write([]byte("v9.9.9\n"))
		return 0, nil
	}
	return r.rc, nil
}

func (r *runRecorder) probeArgv(binary string) ([]string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, run := range r.runs {
		if run.binary == binary && !(len(run.args) == 1 && run.args[0] == "--version") {
			return run.args, true
		}
	}
	return nil, false
}

func doctorEngine(rec *runRecorder) *bridge.Engine {
	return bridge.NewEngine(bridge.Deps{
		LookupEnv: func(string) (string, bool) { return "", false },
		LookPath:  func(b string) (string, error) { return "/usr/bin/" + b, nil },
		Runner:    rec.run,
		Now:       time.Now,
	})
}

func TestC2106_001_AgyDeepProbeArgvCarriesTheAgyManifestFastModel(t *testing.T) {
	man, err := bridge.LoadManifest("agy")
	if err != nil {
		t.Fatalf("load agy manifest: %v", err)
	}
	fast := man.ModelTierMap["fast"]
	if fast == "" {
		t.Fatal("test assumption broken: the agy manifest has no fast model")
	}
	rec := &runRecorder{}

	rep, _ := doctorEngine(rec).Doctor(context.Background(), "agy", true)

	if len(rep.Results) != 1 || !rep.Results[0].DeepProbe.Ran {
		t.Fatalf("the agy deep probe did not run: %+v", rep.Results)
	}
	argv, ok := rec.probeArgv("agy")
	if !ok {
		t.Fatalf("no agy probe invocation was recorded: %+v", rec.runs)
	}
	at := slices.Index(argv, fast)
	if at < 1 || argv[at-1] != "--model" {
		t.Errorf("agy probe argv %q does not pass the manifest fast model %q after --model", argv, fast)
	}
}

func TestC2106_002_TheProbeFastModelFollowsTheManifestNotALiteral(t *testing.T) {
	man, err := bridge.LoadManifest("agy")
	if err != nil {
		t.Fatalf("load agy manifest: %v", err)
	}
	rec := &runRecorder{}
	rep, _ := doctorEngine(rec).Doctor(context.Background(), "agy", true)
	if len(rep.Results) != 1 || !rep.Results[0].DeepProbe.Ran {
		t.Fatalf("the agy deep probe did not run: %+v", rep.Results)
	}
	argv, _ := rec.probeArgv("agy")
	for _, tier := range []string{"balanced", "deep", "top"} {
		other := man.ModelTierMap[tier]
		if other != "" && other != man.ModelTierMap["fast"] && slices.Contains(argv, other) {
			t.Errorf("agy probe argv %q carries the %s model %q, not the fast one", argv, tier, other)
		}
	}
	if !slices.Contains(argv, man.ModelTierMap["fast"]) {
		t.Errorf("agy probe argv %q has no model from the manifest tier map", argv)
	}
}

func TestC2106_003_ProbePreservationHeadlessFamiliesStillProbeAndTmuxDriversStillSkip(t *testing.T) {
	for _, tc := range []struct {
		cli, binary string
		wantRan     bool
	}{
		{"claude-p", "claude", true},
		{"codex", "codex", true},
		{"agy", "agy", true},
		{"claude-tmux", "claude", false},
		{"codex-tmux", "codex", false},
		{"agy-tmux", "agy", false},
		{"ollama-tmux", "ollama", false},
	} {
		rec := &runRecorder{}
		rep, _ := doctorEngine(rec).Doctor(context.Background(), tc.cli, true)
		if len(rep.Results) != 1 {
			t.Fatalf("%s: results = %d, want 1", tc.cli, len(rep.Results))
		}
		if got := rep.Results[0].DeepProbe; got.Ran != tc.wantRan || (tc.wantRan && !got.Passed) {
			t.Errorf("%s: deep probe = %+v, want ran=%v and passed on exit 0", tc.cli, got, tc.wantRan)
		}
		if _, ran := rec.probeArgv(tc.binary); ran != tc.wantRan {
			t.Errorf("%s: probe invoked = %v, want %v", tc.cli, ran, tc.wantRan)
		}
	}
}

func TestC2106_004_AFailingProbeStillBlocksTheVerdict(t *testing.T) {
	rec := &runRecorder{rc: 1}
	rep, code := doctorEngine(rec).Doctor(context.Background(), "agy", true)
	if len(rep.Results) != 1 {
		t.Fatalf("results = %d, want 1", len(rep.Results))
	}
	got := rep.Results[0]
	if !got.DeepProbe.Ran || got.DeepProbe.Passed || got.Verdict != "blocked" || code != 2 {
		t.Errorf("failing agy probe: deep=%+v verdict=%q exit=%d, want ran, not passed, blocked, 2", got.DeepProbe, got.Verdict, code)
	}
}

type fakeBridge struct{}

func (fakeBridge) Launch(context.Context, core.BridgeRequest) (core.BridgeResponse, error) {
	return core.BridgeResponse{}, nil
}

func (fakeBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func mintCfg(cli string) phaseconfig.PhaseConfig {
	return phaseconfig.PhaseConfig{
		PhaseSpec: phasespec.PhaseSpec{Name: "minted-ollama-probe", Optional: true},
		Dispatch:  phaseconfig.Dispatch{CLI: cli, ModelTierDefault: "balanced"},
		Prompt:    "You are a probe phase.",
	}
}

func newRegistrar(t *testing.T) phaseregistrar.Registrar {
	t.Helper()
	return phaseregistrar.Registrar{
		Bridge:      fakeBridge{},
		Prompts:     prompts.NewFromFS(fstest.MapFS{}),
		ProfilesDir: filepath.Join(t.TempDir(), "profiles"),
		PhasesDir:   filepath.Join(t.TempDir(), "phases"),
	}
}

func persistedCLI(t *testing.T, dir string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "minted-ollama-probe.json"))
	if err != nil {
		t.Fatalf("read persisted profile: %v", err)
	}
	var prof profiles.Profile
	if err := json.Unmarshal(raw, &prof); err != nil {
		t.Fatalf("unmarshal persisted profile: %v", err)
	}
	return prof.CLI
}

func TestC2106_005_BareOllamaResolvesToItsTmuxDriverLikeEveryOtherFamily(t *testing.T) {
	resolved, err := bridge.Resolve(bridge.DefaultDriverSet(), "ollama")
	if err != nil || resolved.Driver != "ollama-tmux" {
		t.Fatalf("test assumption broken: Resolve(ollama) = %q, %v, want ollama-tmux", resolved.Driver, err)
	}
	r := newRegistrar(t)

	if _, err := r.Register(mintCfg("ollama")); err != nil {
		t.Fatalf("minting a phase with the bare family ollama failed: %v", err)
	}

	if got := persistedCLI(t, r.ProfilesDir); got != resolved.Driver {
		t.Errorf("persisted profile cli = %q, want %q (the bridge.Resolve driver of the ollama family)", got, resolved.Driver)
	}
}

func TestC2106_006_AnUnknownFamilyIsStillRefusedAtMintAndNothingIsPersisted(t *testing.T) {
	r := newRegistrar(t)

	_, err := r.Register(mintCfg("not-a-real-cli-family"))

	if err == nil {
		t.Fatal("minting with an unknown family returned nil")
	}
	if _, statErr := os.Stat(filepath.Join(r.ProfilesDir, "minted-ollama-probe.json")); statErr == nil {
		t.Error("an unknown-family mint persisted a profile")
	}
	if errors.Is(err, os.ErrNotExist) {
		t.Errorf("the refusal is a file error, not a driver refusal: %v", err)
	}
}

func TestC2106_007_TheDoctorArgvGuardPassesOnTheTreeAndFailsOnAFixture(t *testing.T) {
	root := acsassert.RepoRoot(t)
	mod, err := acsassert.ModuleDir(filepath.Join(root, "go"))
	if err != nil {
		t.Fatalf("module dir: %v", err)
	}
	names := []string{"TestDoctorSource_HasNoLLMArgvLiteral", "TestDoctorArgvGuard_FlagsAFixtureWithALiteral"}

	out, errOut, code, runErr := acsassert.SubprocessOutput("go", "-C", mod, "test", "-count=1", "-v",
		"-run", "^("+strings.Join(names, "|")+")$", "./internal/bridge")

	if runErr != nil || code != 0 {
		t.Fatalf("the guard tests failed or are absent (exit %d): %v\n%s", code, runErr, acsassert.OutputTail(out+errOut, 25))
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("guard test %s did not run and pass", name)
		}
	}
}

func TestC2106_008_AManifestThatFailsToLoadBlocksTheDeepProbeAndIsReported(t *testing.T) {
	for _, tc := range []struct{ cli, binary string }{
		{"agy", "agy"},
		{"codex", "codex"},
		{"claude-p", "claude"},
	} {
		project := t.TempDir()
		manifests := filepath.Join(project, ".evolve", "bridge-manifests")
		if err := os.MkdirAll(manifests, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(manifests, tc.cli+".json"), []byte("{corrupt manifest"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("EVOLVE_PROJECT_ROOT", project)
		rec := &runRecorder{}

		rep, code := doctorEngine(rec).Doctor(context.Background(), tc.cli, true)

		if len(rep.Results) != 1 {
			t.Fatalf("%s: results = %d, want 1", tc.cli, len(rep.Results))
		}
		got := rep.Results[0]
		if argv, ran := rec.probeArgv(tc.binary); ran {
			t.Errorf("%s: the probe ran on a zero manifest with argv %q instead of failing", tc.cli, argv)
		}
		if got.DeepProbe.Passed || got.Verdict != "blocked" || code != 2 {
			t.Errorf("%s: deep=%+v verdict=%q exit=%d, want not passed, blocked, 2", tc.cli, got.DeepProbe, got.Verdict, code)
		}
		raw, err := json.Marshal(got.DeepProbe)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), "invalid JSON") {
			t.Errorf("%s: the deep probe report %s does not carry the manifest load error", tc.cli, raw)
		}
	}
}
