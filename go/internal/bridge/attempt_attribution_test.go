package bridge

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

const codeTokensRowUnattributed signalcenter.Code = "TOKENS_ROW_UNATTRIBUTED"

func attributionEngine(t *testing.T, stage string) (*Engine, *signalcenter.Center) {
	t.Helper()
	signals := signalcenter.New()
	engine := NewEngine(Deps{
		Now:                   func() time.Time { return time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC) },
		Stderr:                io.Discard,
		Signals:               signals,
		TokenAttributionStage: stage,
		TokenResolver:         tokenusage.DefaultResolver(t.TempDir()),
	})
	return engine, signals
}

func attributedRow(t *testing.T, engine *Engine, req core.BridgeRequest) llmcalls.Record {
	t.Helper()
	var resp core.BridgeResponse
	start := time.Date(2026, 10, 27, 8, 58, 0, 0, time.UTC)
	engine.recordTokenUsage(req, "sonnet", 0, start, &resp)
	read, err := llmcalls.ReadWorkspace(req.Workspace)
	if err != nil || len(read.Records) != 1 {
		t.Fatalf("ledger read: records=%d err=%v", len(read.Records), err)
	}
	return read.Records[0]
}

func unattributedSignals(signals *signalcenter.Center) []signalcenter.Event {
	var out []signalcenter.Event
	for _, event := range signals.Recent() {
		if event.Code == codeTokensRowUnattributed {
			out = append(out, event)
		}
	}
	return out
}

func TestRecordModelAttempt_RowCarriesCycleRunProfileAndRetryPurpose(t *testing.T) {
	t.Parallel()
	engine, _ := attributionEngine(t, "")
	row := attributedRow(t, engine, core.BridgeRequest{
		CLI: "claude-tmux", Agent: "build", Workspace: t.TempDir(),
		Cycle: 2040, RunID: "r01M5Y4BP", Profile: "builder", Attempt: 2,
	})
	if row.Cycle != 2040 || row.RunID != "r01M5Y4BP" || row.Profile != "builder" {
		t.Errorf("attribution = cycle %d run %q profile %q, want 2040 r01M5Y4BP builder", row.Cycle, row.RunID, row.Profile)
	}
	if row.Purpose != llmcalls.PurposeRetry {
		t.Errorf("purpose = %q, want %q for attempt 2", row.Purpose, llmcalls.PurposeRetry)
	}
}

func TestRecordModelAttempt_PurposeDerivation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		req  core.BridgeRequest
		want string
	}{
		{"first attempt", core.BridgeRequest{Attempt: 1}, llmcalls.PurposeFirst},
		{"unset attempt counts as first", core.BridgeRequest{}, llmcalls.PurposeFirst},
		{"later attempt is a retry", core.BridgeRequest{Attempt: 3}, llmcalls.PurposeRetry},
		{"correction directive on a first attempt", core.BridgeRequest{Attempt: 1, CorrectionDirective: "fix the report"}, llmcalls.PurposeContractCorrection},
		{"correction directive outranks retry", core.BridgeRequest{Attempt: 2, CorrectionDirective: "fix the report"}, llmcalls.PurposeContractCorrection},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			engine, _ := attributionEngine(t, "")
			req := tc.req
			req.CLI, req.Agent, req.Workspace, req.Cycle = "claude-tmux", "build", t.TempDir(), 2040
			if got := attributedRow(t, engine, req).Purpose; got != tc.want {
				t.Errorf("purpose = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEngineLaunch_RowCarriesTheRequestAttribution(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	prof := writeProfile(t, ws, "eng-attribution", "")
	artifact := filepath.Join(ws, "artifact.md")
	fr := &fakeRunner{writeArtifactPath: artifact, writeArtifactBody: "OK\n"}
	engine := NewEngine(Deps{Runner: fr.runner(), LookupEnv: mapLookup(nil)})

	if _, err := engine.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: prof, Model: "auto", Prompt: "work",
		Workspace: ws, ArtifactPath: artifact, Agent: "build",
		Cycle: 2040, RunID: "r01M5Y4BP", Attempt: 2,
	}); err != nil {
		t.Fatal(err)
	}
	read, err := llmcalls.ReadWorkspace(ws)
	if err != nil || len(read.Records) == 0 {
		t.Fatalf("ledger read: records=%d err=%v", len(read.Records), err)
	}
	row := read.Records[0]
	if row.Cycle != 2040 || row.RunID != "r01M5Y4BP" || row.Profile != prof || row.Purpose != llmcalls.PurposeRetry {
		t.Errorf("launch row = cycle %d run %q profile %q purpose %q", row.Cycle, row.RunID, row.Profile, row.Purpose)
	}
}

func TestRecordModelAttempt_ShadowStageFiresNoSignalForACycleRowWithoutCycle(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"", "shadow", "off"} {
		engine, signals := attributionEngine(t, stage)
		attributedRow(t, engine, core.BridgeRequest{
			CLI: "claude-tmux", Agent: "build", Attempt: 1,
			Workspace: filepath.Join(t.TempDir(), ".evolve", "runs", "cycle-2056"),
		})
		if got := unattributedSignals(signals); len(got) != 0 {
			t.Errorf("stage %q fired %d %s signals, want none", stage, len(got), codeTokensRowUnattributed)
		}
	}
}

func TestRecordModelAttempt_EnforceFiresOnceForACycleRowWithoutCycle(t *testing.T) {
	t.Parallel()
	engine, signals := attributionEngine(t, "enforce")
	attributedRow(t, engine, core.BridgeRequest{
		CLI: "claude-tmux", Agent: "build", Attempt: 1, RunID: "r01M5Y4BP",
		Workspace: filepath.Join(t.TempDir(), ".evolve", "runs", "cycle-2056"),
	})
	fired := unattributedSignals(signals)
	if len(fired) != 1 {
		t.Fatalf("fired %d %s signals, want exactly 1", len(fired), codeTokensRowUnattributed)
	}
	if fired[0].Module != signalcenter.ModuleTokens || !strings.Contains(fired[0].Reason, "cycle") {
		t.Errorf("signal = module %q reason %q, want the tokens module and a reason naming the missing cycle", fired[0].Module, fired[0].Reason)
	}
}

func TestRecordModelAttempt_EnforceStaysQuietWhenTheRowIsAttributedOrNotACycleRow(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		req  core.BridgeRequest
	}{
		{"cycle set", core.BridgeRequest{Cycle: 2056, Workspace: filepath.Join(t.TempDir(), "cycle-2056")}},
		{"standalone agent run with no cycle", core.BridgeRequest{RunID: "review-run", Workspace: filepath.Join(t.TempDir(), "agent-run-1")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			engine, signals := attributionEngine(t, "enforce")
			req := tc.req
			req.CLI, req.Agent, req.Attempt = "claude-tmux", "build", 1
			attributedRow(t, engine, req)
			if got := unattributedSignals(signals); len(got) != 0 {
				t.Errorf("fired %d %s signals, want none", len(got), codeTokensRowUnattributed)
			}
		})
	}
}

func TestTokensRowUnattributed_CodeIsRegisteredOnTheTokensModule(t *testing.T) {
	t.Parallel()
	signals := signalcenter.New()
	signals.Emit(signalcenter.Event{
		Module: signalcenter.ModuleTokens, Origin: "test", Kind: signalcenter.KindBridgeWarning,
		Severity: signalcenter.SeverityWarn, Code: CodeTokensRowUnattributed, Reason: "cycle missing",
	})
	for _, event := range signals.Recent() {
		if event.Code != CodeTokensRowUnattributed {
			t.Errorf("the code %q was rewritten to %q: it is not registered", CodeTokensRowUnattributed, event.Code)
		}
	}
	if CodeTokensRowUnattributed != codeTokensRowUnattributed {
		t.Errorf("CodeTokensRowUnattributed = %q", CodeTokensRowUnattributed)
	}
}
