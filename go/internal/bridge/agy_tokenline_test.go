package bridge

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

func TestRecordTokenUsage_AgyThoughtLineMeasuresTheAttempt(t *testing.T) {
	start := time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC)
	end := start.Add(90 * time.Second)
	ws := filepath.Join(t.TempDir(), ".evolve", "runs", "cycle-1806")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "tmux-final-scrollback.txt"), []byte(agy1217Frame(t, "answer.txt")), 0o644); err != nil {
		t.Fatal(err)
	}
	var errBuf bytes.Buffer
	signals := signalcenter.New()
	signals.Subscribe(signalcenter.Filter(signalcenter.StderrSink(&errBuf), signalcenter.SeverityWarn))
	e := NewEngine(Deps{Now: func() time.Time { return end }, Stderr: io.Discard, Signals: signals,
		TokenResolver: tokenusage.DefaultResolver(t.TempDir())})
	req := core.BridgeRequest{CLI: "agy-tmux", Agent: "router", Workspace: ws, Cycle: 1806}
	var resp core.BridgeResponse
	e.recordTokenUsage(req, "Gemini 3.8 Flash (High)", 0, start, &resp)

	stderr := errBuf.String()
	if strings.Contains(stderr, string(CodeTokenUsageWarning)) || strings.Contains(stderr, "TRIPWIRE") {
		t.Errorf("an agy pane that printed its thought tokens must be measured, not uncovered:\n%s", stderr)
	}
	record := readLLMCalls(t, ws)
	if !strings.Contains(record, `"source":"scrollback_peak"`) || !strings.Contains(record, `"output":1500`) {
		t.Errorf("llm-calls record = %s, want source scrollback_peak with the 1.5k thought tokens", record)
	}
}

func TestAgyTmuxManifest_TokenLineRegexNamesCountAndScale(t *testing.T) {
	m, err := LoadManifest("agy-tmux")
	if err != nil {
		t.Fatal(err)
	}
	re, err := regexp.Compile(m.TokenLineRegex)
	if err != nil || m.TokenLineRegex == "" {
		t.Fatalf("agy-tmux token_line_regex %q must be a non-empty valid pattern: %v", m.TokenLineRegex, err)
	}
	if re.SubexpIndex("count") < 0 || re.SubexpIndex("scale") < 0 {
		t.Fatalf("token_line_regex %q must name the count and scale groups", m.TokenLineRegex)
	}
	if got := panestream.TokenLinePeak(agy1217Frame(t, "answer.txt"), m.TokenLineRegex); got != 1500 {
		t.Errorf("1.2.17 answer frame: peak %d, want 1500", got)
	}
	if paneProfileFor(agyLaunchForTest()).TokenLineRegex != m.TokenLineRegex {
		t.Error("the agy pane profile must carry the manifest's token line pattern")
	}
}
