package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

func TestEngineLaunch_DurationStopsBeforeTokenResolution(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "duration-freeze", "")
	artifact := filepath.Join(ws, "artifact.md")
	fr := &fakeRunner{writeArtifactPath: artifact, writeArtifactBody: "OK\n"}
	t0 := time.Date(2026, 9, 11, 8, 0, 0, 0, time.UTC)
	times := []time.Time{t0, t0.Add(2 * time.Second), t0.Add(37 * time.Second)}
	nowCalls := 0
	now := func() time.Time {
		if nowCalls >= len(times) {
			return times[len(times)-1]
		}
		got := times[nowCalls]
		nowCalls++
		return got
	}
	eng := NewEngine(Deps{
		Runner: fr.runner(), LookupEnv: mapLookup(nil), Now: now,
		TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) {
			// Simulate enrichment consuming later clock budget. The attempt's end
			// must already be frozen before this callback begins.
			_ = now()
			return tokenusage.Result{Usage: cyclestate.TokenUsage{Output: 4}, Source: tokenusage.SourceTranscript}, nil
		},
	})

	if _, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: prof, Model: "claude-opus-4-1", Prompt: "work",
		Workspace: ws, ArtifactPath: artifact, Agent: "build",
	}); err != nil {
		t.Fatal(err)
	}
	rec := readRecords(t, ws)[0]
	if rec.DurationMS != 2000 {
		t.Fatalf("duration_ms = %d, want 2000 before resolver overhead", rec.DurationMS)
	}
	if rec.StartedAt != t0.Format(time.RFC3339Nano) || rec.EndedAt != t0.Add(2*time.Second).Format(time.RFC3339Nano) {
		t.Fatalf("frozen window = %s..%s", rec.StartedAt, rec.EndedAt)
	}
}

func TestEngineLaunch_RecordsFinalCodexSelectorAfterLateOverride(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "dispatch-selector", "")
	artifact := filepath.Join(ws, "artifact.md")
	fr := &fakeRunner{writeArtifactPath: artifact, writeArtifactBody: "OK\n"}
	eng := NewEngine(Deps{
		Runner: fr.runner(), LookupEnv: mapLookup(nil),
		TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) {
			return tokenusage.Result{Source: tokenusage.SourceNone}, nil
		},
	})

	if _, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "codex", Profile: prof, Model: "provider-default", Prompt: "work",
		Workspace: ws, ArtifactPath: artifact, Agent: "build",
		ExtraFlags: []string{"-m", "gpt-5.6-sol"},
	}); err != nil {
		t.Fatal(err)
	}
	rec := readRecords(t, ws)[0]
	if rec.RequestedModel != "provider-default" || rec.Model != "provider-default" {
		t.Fatalf("requested selector lost: %+v", rec)
	}
	if rec.DispatchedModel != "gpt-5.6-sol" || rec.DispatchSource != "argv" {
		t.Fatalf("final dispatched selector not captured: %+v; argv=%v", rec, firstRunnerArgs(fr))
	}
}

func TestEngineLaunch_ModelLikePositionalTextIsNotDispatchIdentity(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "dispatch-boundary", "")
	artifact := filepath.Join(ws, "artifact.md")
	fr := &fakeRunner{writeArtifactPath: artifact, writeArtifactBody: "OK\n"}
	eng := NewEngine(Deps{Runner: fr.runner(), LookupEnv: mapLookup(nil)})

	if _, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "codex", Profile: prof, Model: "gpt-5.6-sol", Prompt: "work",
		Workspace: ws, ArtifactPath: artifact, Agent: "build",
		ExtraFlags: []string{"--", "--model=private prompt text"},
	}); err != nil {
		t.Fatal(err)
	}
	rec := readRecords(t, ws)[0]
	if rec.DispatchedModel != "gpt-5.6-sol" || rec.DispatchSource != modelDispatchArgv {
		t.Fatalf("positional text impersonated a selector: %+v", rec)
	}
}

func TestEngineLaunch_ProfileFlagsPreserveDispatchProvenance(t *testing.T) {
	tests := []struct {
		name       string
		profileRaw []string
		extra      []string
		wantModel  string
		wantSource string
	}{
		{
			name:       "unknown profile option arity cannot promote its value",
			profileRaw: []string{"--append-system-prompt", "--model=private profile text"},
			wantSource: modelDispatchUnknown,
		},
		{
			name:       "profile terminator also bounds direct extras",
			profileRaw: []string{"--"},
			extra:      []string{"--model=private positional text"},
			wantModel:  "gpt-5.6-sol",
			wantSource: modelDispatchArgv,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			prof := writeProfileWithRawFlags(t, ws, "codex", tc.profileRaw)
			artifact := filepath.Join(ws, "artifact.md")
			fr := &fakeRunner{writeArtifactPath: artifact, writeArtifactBody: "OK\n"}
			eng := NewEngine(Deps{Runner: fr.runner(), LookupEnv: mapLookup(nil)})

			if _, err := eng.Launch(context.Background(), core.BridgeRequest{
				CLI: "codex", Profile: prof, Model: "gpt-5.6-sol", Prompt: "work",
				Workspace: ws, ArtifactPath: artifact, Agent: "build", ExtraFlags: tc.extra,
			}); err != nil {
				t.Fatal(err)
			}
			rec := readRecords(t, ws)[0]
			if rec.DispatchedModel != tc.wantModel || rec.DispatchSource != tc.wantSource {
				t.Fatalf("dispatch provenance = model %q source %q; want model %q source %q; argv=%v",
					rec.DispatchedModel, rec.DispatchSource, tc.wantModel, tc.wantSource, firstRunnerArgs(fr))
			}
		})
	}
}

func TestEngineLaunch_RepeatedCodexDedicatedSelectorsAreUnknown(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfileWithRawFlags(t, ws, "codex", []string{"-m", "gpt-5.6-terra"})
	artifact := filepath.Join(ws, "artifact.md")
	fr := &fakeRunner{exit: 2}
	eng := NewEngine(Deps{Runner: fr.runner(), LookupEnv: mapLookup(nil)})

	resp, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "codex", Profile: prof, Model: "gpt-5.6-sol", Prompt: "work",
		Workspace: ws, ArtifactPath: artifact, Agent: "build",
	})
	if err == nil || resp.ExitCode != 2 {
		t.Fatalf("Launch = exit %d, error %v; want simulated provider argument conflict", resp.ExitCode, err)
	}

	args := firstRunnerArgs(fr)
	if !fr.argvContainsPair("-m", "gpt-5.6-sol") || !fr.argvContainsPair("-m", "gpt-5.6-terra") {
		t.Fatalf("test did not reach the provider-invalid repeated selector vector: %v", args)
	}
	rec := readRecords(t, ws)[0]
	if rec.DispatchedModel != "" || rec.DispatchSource != modelDispatchUnknown {
		t.Fatalf("provider-invalid repeated selectors claimed model %q source %q; argv=%v",
			rec.DispatchedModel, rec.DispatchSource, args)
	}
}

func TestEngineLaunch_IdenticalCodexSelectorIsDeduplicatedBeforeAttribution(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	t.Setenv("HOME", t.TempDir())
	ws := t.TempDir()
	model := "gpt-5.6-sol"
	prof := writeProfileWithRawFlags(t, ws, "codex-tmux", []string{"-m", model})
	artifact := filepath.Join(ws, "artifact.md")
	if err := os.WriteFile(artifact, []byte("OK\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tmux := &fakeTmux{paneSeq: []string{"›"}}
	eng := NewEngine(Deps{
		Tmux: tmux, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil),
		CaptureBaseline: zeroBaselineCapture,
	})

	resp, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "codex-tmux", Profile: prof, Model: model, Prompt: "work",
		Workspace: ws, ArtifactPath: artifact, Agent: "build",
	})
	if err != nil || resp.ExitCode != ExitOK {
		t.Fatalf("Launch = exit %d, error %v", resp.ExitCode, err)
	}

	launch := launchedCmd(tmux, "codex")
	if count := strings.Count(launch, "-m "+model); count != 1 {
		t.Fatalf("identical model selector count = %d, want 1 in %q", count, launch)
	}
	rec := readRecords(t, ws)[0]
	if rec.DispatchedModel != model || rec.DispatchSource != modelDispatchArgv {
		t.Fatalf("deduplicated dispatch = model %q source %q; launch=%q",
			rec.DispatchedModel, rec.DispatchSource, launch)
	}
}

func TestEngineLaunch_DispatchMatchesMixedFormClamp(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	t.Setenv("HOME", t.TempDir())
	manifest, err := LoadManifest("codex-tmux")
	if err != nil {
		t.Fatal(err)
	}
	unsafeModel := "gpt-not-on-chatgpt-plan"
	tests := []struct {
		name         string
		model        string
		profileModel string
		raw          []string
		wantSelector string
	}{
		{
			name: "inline selector", model: "auto", profileModel: "", raw: []string{"--model=" + unsafeModel},
			wantSelector: "--model=" + manifest.ChatGPTDefaultModel,
		},
		{
			name: "config selector", model: unsafeModel, raw: []string{"-c", `model="` + unsafeModel + `"`},
			wantSelector: "-m " + manifest.ChatGPTDefaultModel,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			prof := writeProfileWithModelAndRawFlags(t, ws, tc.profileModel, "codex-tmux", tc.raw)
			artifact := filepath.Join(ws, "artifact.md")
			if err := os.WriteFile(artifact, []byte("OK\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			tmux := &fakeTmux{paneSeq: []string{"›"}}
			eng := NewEngine(Deps{
				Tmux: tmux, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil),
				CaptureBaseline: zeroBaselineCapture,
			})

			resp, err := eng.Launch(context.Background(), core.BridgeRequest{
				CLI: "codex-tmux", Profile: prof, Model: tc.model, Prompt: "work",
				Workspace: ws, ArtifactPath: artifact, Agent: "build",
			})
			if err != nil || resp.ExitCode != ExitOK {
				t.Fatalf("Launch = exit %d, error %v", resp.ExitCode, err)
			}

			launch := launchedCmd(tmux, "codex")
			if !strings.Contains(launch, tc.wantSelector) {
				t.Fatalf("effective selector %q was not clamped in final launch: %q", tc.wantSelector, launch)
			}
			rec := readRecords(t, ws)[0]
			if rec.DispatchedModel != manifest.ChatGPTDefaultModel || rec.DispatchSource != modelDispatchArgv {
				t.Fatalf("recorded dispatch = model %q source %q, final launch = %q",
					rec.DispatchedModel, rec.DispatchSource, launch)
			}
		})
	}
}

func firstRunnerArgs(runner *fakeRunner) []string {
	if len(runner.calls) == 0 {
		return nil
	}
	return runner.calls[0].args
}

func writeProfileWithRawFlags(t *testing.T, dir, cli string, raw []string) string {
	return writeProfileWithModelAndRawFlags(t, dir, "gpt-5.6-sol", cli, raw)
}

func writeProfileWithModelAndRawFlags(t *testing.T, dir, model, cli string, raw []string) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"name":               "dispatch-profile-provenance",
		"model":              model,
		"allowed_tools":      []string{"Read", "Write"},
		"auto_respond":       map[string]any{"destructive_ops": false, "timeout_s": 60},
		"prompt_overrides":   []string{},
		"extra_flags_by_cli": map[string][]string{cli: raw},
	})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "profile-with-raw-flags.json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEngineLaunch_OmittedModelRecordsUnknownCLIDefault(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "dispatch-default", "")
	artifact := filepath.Join(ws, "artifact.md")
	fr := &fakeRunner{writeArtifactPath: artifact, writeArtifactBody: "OK\n"}
	eng := NewEngine(Deps{
		Runner: fr.runner(), LookupEnv: mapLookup(nil),
		TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) {
			return tokenusage.Result{Source: tokenusage.SourceNone}, nil
		},
	})

	if _, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: prof, Model: "deep", Prompt: "work",
		Workspace: ws, ArtifactPath: artifact, Agent: "audit",
	}); err != nil {
		t.Fatal(err)
	}
	rec := readRecords(t, ws)[0]
	if rec.DispatchedModel != "" || rec.DispatchSource != "cli_default" {
		t.Fatalf("omitted selector was reported as concrete: %+v", rec)
	}
	for _, call := range fr.calls {
		for _, arg := range call.args {
			if arg == "deep" {
				t.Fatalf("unresolved selector unexpectedly reached argv: %v", call.args)
			}
		}
	}
}

func TestEngineLaunch_FailedDispatchStillRecordsSelectorAndCause(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "failed-dispatch", "")
	artifact := filepath.Join(ws, "artifact.md")
	fr := &fakeRunner{exit: ExitMissingBinary, err: errors.New("binary absent")}
	eng := NewEngine(Deps{Runner: fr.runner(), LookupEnv: mapLookup(nil)})

	resp, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: prof, Model: "claude-opus-4-1", Prompt: "work",
		Workspace: ws, ArtifactPath: artifact, Agent: "audit",
	})
	if err == nil || resp.ExitCode != ExitMissingBinary {
		t.Fatalf("Launch = exit %d err %v", resp.ExitCode, err)
	}
	rec := readRecords(t, ws)[0]
	if rec.DispatchedModel != "claude-opus-4-1" || rec.ExitCode != ExitMissingBinary || rec.CauseCode != "missing_binary" {
		t.Fatalf("failed dispatch record = %+v", rec)
	}
}

func TestEngineLaunch_ResolverWarningCarriesAttemptContext(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "resolver-context", "")
	artifact := filepath.Join(ws, "artifact.md")
	fr := &fakeRunner{writeArtifactPath: artifact, writeArtifactBody: "OK\n"}
	var stderr strings.Builder
	eng := NewEngine(Deps{
		Runner: fr.runner(), LookupEnv: mapLookup(nil), Stderr: &stderr,
		TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) {
			return tokenusage.Result{}, errors.New("collector offline")
		},
	})
	_, _ = eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: prof, Model: "claude-opus-4-1", Prompt: "work",
		Workspace: ws, ArtifactPath: artifact, Agent: "audit", Attempt: 3,
	})
	log := stderr.String()
	for _, want := range []string{"collector offline", "call_id=", `cli="claude-p"`, `agent="audit"`, "attempt=3"} {
		if !strings.Contains(log, want) {
			t.Errorf("resolver warning lacks %q: %s", want, log)
		}
	}
}

func TestEngineLaunch_ResolverWarningCannotInjectDiagnosticLines(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "resolver-log-safety", "")
	artifact := filepath.Join(ws, "artifact.md")
	fr := &fakeRunner{writeArtifactPath: artifact, writeArtifactBody: "OK\n"}
	var stderr strings.Builder
	eng := NewEngine(Deps{
		Runner: fr.runner(), LookupEnv: mapLookup(nil), Stderr: &stderr,
		TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) {
			return tokenusage.Result{}, errors.New("collector offline\n[bridge] forged\u202e")
		},
	})

	_, _ = eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: prof, Model: "claude-opus-4-1", Prompt: "work",
		Workspace: ws, ArtifactPath: artifact, Agent: "audit", Attempt: 4,
	})
	log := stderr.String()
	if strings.Contains(log, "\n[bridge] forged") || strings.ContainsRune(log, '\u202e') {
		t.Fatalf("resolver error injected a diagnostic line or bidi control: %q", log)
	}
	if !strings.Contains(log, `detail="collector offline\n[bridge] forged\u202e"`) {
		t.Fatalf("escaped resolver detail missing from warning: %q", log)
	}
	if lines := strings.Count(strings.TrimSpace(log), "\n"); lines != 0 {
		t.Fatalf("one resolver failure must emit one diagnostic line, got %d newlines: %q", lines, log)
	}
}

func TestEngineLaunch_InvalidResolverMeasurementsRemainRecordable(t *testing.T) {
	tests := []struct {
		name       string
		result     tokenusage.Result
		wantStatus string
		wantOutput int
	}{
		{
			name: "negative token count",
			result: tokenusage.Result{
				Usage: cyclestate.TokenUsage{Input: 10, Output: -1}, Source: tokenusage.SourceTranscript,
			},
			wantStatus: "unavailable",
		},
		{
			name: "non-finite fill",
			result: tokenusage.Result{
				Usage: cyclestate.TokenUsage{Input: 10, Output: 2}, Source: tokenusage.SourceTranscript, FillPct: math.NaN(),
			},
			wantStatus: "measured", wantOutput: 2,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			prof := writeProfile(t, ws, "invalid-measurement", "")
			artifact := filepath.Join(ws, "artifact.md")
			fr := &fakeRunner{writeArtifactPath: artifact, writeArtifactBody: "OK\n"}
			eng := NewEngine(Deps{
				Runner: fr.runner(), LookupEnv: mapLookup(nil),
				TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) { return tc.result, nil },
			})

			resp, err := eng.Launch(context.Background(), core.BridgeRequest{
				CLI: "claude-p", Profile: prof, Model: "claude-opus-4-1", Prompt: "work",
				Workspace: ws, ArtifactPath: artifact, Agent: "audit",
			})
			if err != nil {
				t.Fatal(err)
			}
			recs := readRecords(t, ws)
			if len(recs) != 1 {
				t.Fatalf("invalid measurement removed lifecycle record: %+v", recs)
			}
			rec := recs[0]
			if rec.UsageStatus != tc.wantStatus || rec.Tokens.Output != tc.wantOutput || resp.Tokens.Output != tc.wantOutput {
				t.Fatalf("normalized measurement = rec %+v resp %+v", rec, resp.Tokens)
			}
			if rec.FillPct != tokenusage.FillPctUnmeasured {
				t.Fatalf("invalid fill = %v, want sentinel %v", rec.FillPct, tokenusage.FillPctUnmeasured)
			}
		})
	}
}

func TestEngineLaunch_AgyLogOpenFailureDoesNotClaimProcessDispatch(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "agy-log-open", "")
	artifact := filepath.Join(ws, "artifact.md")
	eng := NewEngine(Deps{Runner: (&fakeRunner{}).runner(), LookupEnv: mapLookup(nil)})

	resp, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "agy", Profile: prof, Model: "auto", Prompt: "work",
		Workspace: ws, ArtifactPath: artifact, Agent: "build",
		StdoutLog: ws, // opening a directory as a file fails before Runner dispatch
	})
	if err == nil || resp.ExitCode != ExitBadFlags {
		t.Fatalf("Launch = exit %d err %v, want pre-dispatch bad-flags failure", resp.ExitCode, err)
	}
	rec := readRecords(t, ws)[0]
	if rec.DispatchedModel != "" || rec.DispatchSource != "not_started" {
		t.Fatalf("pre-dispatch log failure claimed a process selector: %+v", rec)
	}
}

func TestEngineLaunch_WarmNamedSessionDoesNotClaimGeneratedSelector(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "warm-session", "")
	artifact := filepath.Join(ws, "artifact.md")
	frames := make([]string, 20)
	for i := range frames {
		frames[i] = "working ❯"
	}
	base := &FakeTmuxController{Existing: map[string]bool{NamedSessionName("warm"): true}, CaptureFrames: frames}
	tmux := &artifactOnPasteTmux{FakeTmuxController: base, artifact: artifact}
	eng := NewEngine(Deps{
		Tmux: tmux, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil),
	})

	if _, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: prof, Model: "deep", Prompt: "work",
		Workspace: ws, Worktree: ws, ProjectRoot: ws, ArtifactPath: artifact,
		Agent: "audit", SessionName: "warm",
	}); err != nil {
		t.Fatal(err)
	}
	rec := readRecords(t, ws)[0]
	if rec.DispatchedModel != "" || rec.DispatchSource != "resumed_session" {
		t.Fatalf("warm session claimed a generated selector: %+v", rec)
	}
}

func TestModelDispatchForTmuxCapturesOnlyLaunchBoundary(t *testing.T) {
	realization := Realization{modelDispatchEffect: modelDispatchFromArgs("codex-tmux", modelDispatch{},
		[]string{"-c", `model="gpt-5.6-sol"`})}
	got := modelDispatchForTmux("codex-tmux", realization, []string{"--model=gpt-override"})
	if got.model != "gpt-override" || got.source != "argv" {
		t.Fatalf("modelDispatchForTmux = %+v", got)
	}
	dedicated := modelDispatchForTmux("codex-tmux", Realization{
		modelDispatchEffect: modelDispatchFromArgs("codex-tmux", modelDispatch{},
			[]string{"-m", "gpt-5.6-terra"}),
	}, []string{"-c", `model="gpt-5.6-sol"`})
	if dedicated.model != "gpt-5.6-terra" || dedicated.source != "argv" {
		t.Fatalf("Codex config override displaced dedicated selector: %+v", dedicated)
	}
	configOnly := modelDispatchForTmux("codex-tmux", Realization{},
		[]string{"--config", `model="gpt-5.6-sol"`})
	if configOnly.model != "gpt-5.6-sol" || configOnly.source != "argv" {
		t.Fatalf("Codex config-only selector = %+v", configOnly)
	}
	repeated := modelDispatchForTmux("codex-tmux", Realization{
		modelDispatchEffect: modelDispatchFromArgs("codex-tmux", modelDispatch{},
			[]string{"-m", "gpt-5.6-terra"}),
	}, []string{"--model=gpt-5.6-sol"})
	if repeated.model != "" || repeated.source != modelDispatchUnknown {
		t.Fatalf("repeated Codex dedicated selectors = %+v, want unknown", repeated)
	}
}

func TestModelDispatchFromExtraArgsLeavesUnknownOptionArityAmbiguous(t *testing.T) {
	base := modelDispatchFromArgs("codex", defaultModelDispatch(),
		[]string{"-c", "model=gpt-5.6-sol"})
	got := modelDispatchFromExtraArgs("codex", base, []string{"--prompt", "--model=private prompt text"})
	if got.model != "" || got.source != "unknown" {
		t.Fatalf("ambiguous pass-through persisted model-looking text: %+v", got)
	}
	explicit := modelDispatchFromExtraArgs("codex", base, []string{"-m", "gpt-5.6-terra"})
	if explicit.model != "gpt-5.6-terra" || explicit.source != modelDispatchArgv {
		t.Fatalf("explicit extra selector lost: %+v", explicit)
	}
}

func TestModelDispatchFromExtraArgsUsesProviderGrammar(t *testing.T) {
	base := modelDispatch{model: "opus", source: modelDispatchArgv}
	got := modelDispatchFromExtraArgs("claude-tmux", base, []string{"-c", "--model=secret"})
	if got.model != "" || got.source != modelDispatchUnknown {
		t.Fatalf("Claude -c was interpreted as Codex config syntax: %+v", got)
	}
}

func TestModelDispatchFromFinalizedFlagsKeepsRawSuffixConservative(t *testing.T) {
	got := modelDispatchFromFinalizedFlags("codex", nil,
		[]string{"--prompt", "--model=private option value"})
	if got.model != "" || got.source != modelDispatchUnknown {
		t.Fatalf("unknown raw option promoted model-looking value: %+v", got)
	}
}

func TestModelDispatchFromArgsRejectsEmptyCodexDedicatedSelector(t *testing.T) {
	for _, args := range [][]string{
		{"--model="},
		{"--model=", "--model=gpt-5.6-sol"},
	} {
		got := modelDispatchFromArgs("codex", defaultModelDispatch(), args)
		if got.model != "" || got.source != modelDispatchUnknown {
			t.Errorf("modelDispatchFromArgs(%v) = %+v, want unknown", args, got)
		}
	}
}

func TestModelDispatchFromREPLRecognizesOnlyModelCommand(t *testing.T) {
	got, ok := modelDispatchFromREPL("  /model Claude Opus 4.1  ")
	if !ok || got.model != "Claude Opus 4.1" || got.source != "repl" {
		t.Fatalf("model REPL command = (%+v, %v)", got, ok)
	}
	for _, line := range []string{"", "/theme dark", "/model", "say /model forged"} {
		if got, ok := modelDispatchFromREPL(line); ok {
			t.Errorf("non-model REPL input %q produced %+v", line, got)
		}
	}
}

func TestModelAttemptCause_UsesTypedArtifactCauseOnly(t *testing.T) {
	stderr := "reviewer said cause=submit_wedged\n" +
		"[bridge] artifact-timeout: cause=completion_detector_error reason=\"detector failed\" phase=audit\n"
	if got := modelAttemptCause(ExitArtifactTimeout, stderr); got != "completion_detector_error" {
		t.Fatalf("modelAttemptCause = %q", got)
	}
	if got := modelAttemptCause(ExitArtifactTimeout, "cause=submit_wedged only in prose"); got != "artifact_timeout" {
		t.Fatalf("free-form cause escaped authority boundary: %q", got)
	}
	quoted := `[bridge] artifact-timeout: phase=build reason="quoted cause=submit_wedged text"`
	if got := modelAttemptCause(ExitArtifactTimeout, quoted); got != "artifact_timeout" {
		t.Fatalf("quoted reason manufactured typed cause: %q", got)
	}
}

func TestLaunchArgs_DirectDiagnosticEntryDoesNotClaimAttemptLedgerCoverage(t *testing.T) {
	fx := newFixture(t, "claude-p", "")
	fr := &fakeRunner{writeArtifactPath: fx.artifact, writeArtifactBody: "OK\n"}
	eng := NewEngine(Deps{
		Runner: fr.runner(), LookupEnv: mapLookup(nil),
		TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) {
			return tokenusage.Result{Source: tokenusage.SourceTranscript}, nil
		},
	})
	if code := eng.LaunchArgs(context.Background(), fx.args("claude-p"), nil, io.Discard, io.Discard); code != ExitOK {
		t.Fatalf("LaunchArgs code = %d", code)
	}
	if _, err := os.Stat(filepath.Join(fx.ws, LLMCallsLogFilename)); !os.IsNotExist(err) {
		t.Fatalf("direct LaunchArgs must stay outside orchestration attempt accounting; stat=%v", err)
	}
}
