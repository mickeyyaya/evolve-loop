package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelquery"
)

func TestShouldRefreshCatalog(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	stale := modelcatalog.Catalog{FetchedAt: now.Add(-48 * time.Hour)}
	fresh := modelcatalog.Catalog{FetchedAt: now.Add(-1 * time.Hour)}
	empty := modelcatalog.Catalog{}

	tests := []struct {
		name        string
		cat         modelcatalog.Catalog
		autoRefresh bool
		want        bool
	}{
		{"stale → refresh", stale, true, true},
		{"empty (never fetched) → refresh", empty, true, true},
		{"fresh within TTL → skip", fresh, true, false},
		{"disabled overrides stale", stale, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRefreshCatalog(tt.cat, now, tt.autoRefresh); got != tt.want {
				t.Fatalf("shouldRefreshCatalog = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPickClassifierCLI(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		ready []string
		want  []string
	}{
		{"preference order, skipping a CLI with no classifier driver", []string{"agy", "ollama", "codex"}, []string{"codex-tmux", "agy-tmux"}},
		{"claude leads when codex is not ready", []string{"agy", "claude"}, []string{"claude-tmux", "agy-tmux"}},
		{"every preferred ready CLI is in the chain", []string{"agy", "claude", "codex"}, []string{"codex-tmux", "claude-tmux", "agy-tmux"}},
		{"ollama alone yields no classifier", []string{"ollama"}, []string{}},
		{"empty when none ready", nil, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pickClassifierCLI(tt.ready, legacyClassifierPreference(t)); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("pickClassifierCLI(%v) = %v, want %v", tt.ready, got, tt.want)
			}
		})
	}
}

type classifierScript struct {
	replies map[string]string
	errs    map[string]error
	clis    []string
}

func (s *classifierScript) DispatchPrompt(_ context.Context, cli, _ string) (string, error) {
	s.clis = append(s.clis, cli)
	return s.replies[cli], s.errs[cli]
}

type offeringLister map[string][]string

func (l offeringLister) List(_ context.Context, cli string) ([]string, error) { return l[cli], nil }

func TestTierClassifier_FirstPreferredCLIFailsTheNextClassifiesLive(t *testing.T) {
	t.Parallel()
	script := &classifierScript{
		errs:    map[string]error{"codex-tmux": errors.New("bridgePromptDispatcher: launch codex-tmux: bridge: launch exit=1")},
		replies: map[string]string{"claude-tmux": `{"fast":"Gemini 3.8 Flash (Low)","balanced":"Gemini 3.8 Flash (High)","deep":"Gemini 3.1 Pro (High)","top":"Gemini 3.1 Pro (High)"}`},
	}
	var log bytes.Buffer
	cat, err := modelquery.Refresh(context.Background(), modelquery.RefreshDeps{
		CLIs:       []string{"agy"},
		Lister:     offeringLister{"agy": {"Gemini 3.8 Flash (Low)", "Gemini 3.8 Flash (High)", "Gemini 3.7 Flash (High)", "Gemini 3.1 Pro (High)"}},
		Classifier: modelquery.ChainClassifier{CLIs: pickClassifierCLI([]string{"agy", "claude", "codex"}, legacyClassifierPreference(t)), Dispatcher: script, Log: &log},
		Fallback:   map[string]map[string]string{"agy": {"balanced": "Gemini 3.7 Flash (High)"}},
		Now:        stageNow,
	})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if m, ok := cat.DispatchModel("agy", "balanced"); !ok || m != "Gemini 3.8 Flash (High)" {
		t.Fatalf("agy balanced = (%q,%v) source=%q reason=%q, want live Gemini 3.8 Flash (High)",
			m, ok, cat.CLIs["agy"].Source, cat.CLIs["agy"].FallbackReason)
	}
	if !reflect.DeepEqual(script.clis, []string{"codex-tmux", "claude-tmux"}) {
		t.Errorf("classifier launches = %v, want codex then claude", script.clis)
	}
	if !strings.Contains(log.String(), "cli=codex-tmux") {
		t.Errorf("the codex classifier failure must be logged, got %q", log.String())
	}
}

func TestBridgePromptDispatcher_NamesTheArtifactTheBridgeReadsTheReplyFrom(t *testing.T) {
	t.Parallel()
	d := bridgePromptDispatcher{workspace: t.TempDir(), projectRoot: t.TempDir()}
	for _, driver := range []string{"claude-p", "claude-tmux", "agy", "codex-tmux"} {
		req := d.request(driver, "PROMPT")
		if req.CLI != driver {
			t.Errorf("%s launches driver %q, want %q", driver, req.CLI, driver)
		}
		if req.Completion != core.CompletionArtifact || req.ArtifactPath == "" {
			t.Fatalf("%s: completion %q artifact %q, want the artifact contract with a path", driver, req.Completion, req.ArtifactPath)
		}
		if !strings.HasPrefix(req.Prompt, "PROMPT") || !strings.Contains(req.Prompt, req.ArtifactPath) {
			t.Errorf("%s: the prompt never names the artifact the bridge reads the reply from, so a CLI that only prints its answer returns empty: %q", driver, req.Prompt)
		}
	}
}

func TestBridgePromptDispatcher_ALinkThatWritesNothingNeverReadsTheLastLinksReply(t *testing.T) {
	t.Parallel()
	const staleReply = `{"fast":"Gemini 9 Imaginary (Low)"}`
	read := map[string]string{}
	d := bridgePromptDispatcher{
		workspace:   t.TempDir(),
		projectRoot: t.TempDir(),
		launch: func(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
			if req.CLI == "codex-tmux" {
				if err := os.WriteFile(req.ArtifactPath, []byte(staleReply), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			reply, _ := os.ReadFile(req.ArtifactPath)
			read[req.CLI] = string(reply)
			return core.BridgeResponse{Stdout: string(reply)}, nil
		},
	}
	cat, err := modelquery.Refresh(context.Background(), modelquery.RefreshDeps{
		CLIs:       []string{"agy"},
		Lister:     offeringLister{"agy": {"Gemini 3.8 Flash (Low)", "Gemini 3.8 Flash (High)"}},
		Classifier: modelquery.ChainClassifier{CLIs: pickClassifierCLI([]string{"codex", "claude"}, legacyClassifierPreference(t)), Dispatcher: d, Log: io.Discard},
		Fallback:   map[string]map[string]string{"agy": {"balanced": "Gemini 3.8 Flash (High)"}},
		Now:        stageNow,
	})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if read["claude-tmux"] != "" {
		t.Errorf("claude wrote nothing yet its reply was %q, the previous link's", read["claude-tmux"])
	}
	reason := cat.CLIs["agy"].FallbackReason
	claudePart := reason[strings.Index(reason, "classifier claude-tmux:"):]
	if !strings.HasPrefix(claudePart, "classifier claude-tmux: no JSON object in reply") || strings.Contains(claudePart, "Imaginary") {
		t.Errorf("claude's fallback reason must be its own empty reply, not codex's: %q", reason)
	}
}

func TestBridgePromptDispatcher_AnUnclearableArtifactFailsBeforeLaunching(t *testing.T) {
	t.Parallel()
	launched := false
	d := bridgePromptDispatcher{workspace: t.TempDir(), projectRoot: t.TempDir(),
		launch: func(context.Context, core.BridgeRequest) (core.BridgeResponse, error) {
			launched = true
			return core.BridgeResponse{}, nil
		},
	}
	blocker := filepath.Join(d.request("claude", "").ArtifactPath, "keep")
	if err := os.MkdirAll(blocker, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := d.DispatchPrompt(context.Background(), "claude", "PROMPT")

	if err == nil || !strings.Contains(err.Error(), "previous reply") {
		t.Fatalf("err = %v, want a refusal naming the uncleared previous reply", err)
	}
	if launched {
		t.Error("the classifier launched although the previous reply could not be cleared")
	}
}
