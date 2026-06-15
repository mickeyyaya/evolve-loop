package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func writeTokenUsage(workspace string, peakTokens int) {
	if peakTokens < 0 {
		peakTokens = 0
	}
	data, err := json.MarshalIndent(struct {
		PeakTokens int `json:"peak_tokens"`
	}{PeakTokens: peakTokens}, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(workspace, "token-usage.json"), append(data, '\n'), 0o644)
}

// tmuxNonClaudePreflight runs the rejections shared by codex-tmux and
// agy-tmux: permission_mode is claude-only, named sessions are
// claude-tmux-only, and --allow-bypass is mandatory (these drivers run
// the inner CLI with bypass-like semantics). Returns (exitCode, handled);
// when handled, the driver returns immediately.
func tmuxNonClaudePreflight(name string, cfg *Config, deps Deps) (int, bool) {
	pfx := "[" + name + "]"
	if cfg.PermissionMode != "" {
		fmt.Fprintf(deps.Stderr, "%s permission_mode='%s' is not supported on this CLI\n", pfx, cfg.PermissionMode)
		fmt.Fprintf(deps.Stderr, "%s Only claude-p and claude-tmux drivers support --permission-mode.\n", pfx)
		return ExitBadFlags, true
	}
	if cfg.StreamOutput {
		fmt.Fprintf(deps.Stderr, "%s NOTE: stream_output=true is not supported on this CLI — no-op\n", pfx)
	}
	if cfg.SessionName != "" {
		fmt.Fprintf(deps.Stderr, "%s --session-name='%s' is not supported on this CLI in v0.5\n", pfx, cfg.SessionName)
		fmt.Fprintf(deps.Stderr, "%s Only claude-tmux supports named/resumable sessions; use --cli=claude-tmux or omit --session-name.\n", pfx)
		return ExitBadFlags, true
	}
	if !cfg.AllowBypass {
		fmt.Fprintf(deps.Stderr, "%s safety gate: --allow-bypass is required\n", pfx)
		return ExitSafetyGate, true
	}
	return 0, false
}

type escalationReport struct {
	Phase     string `json:"phase"`
	Cycle     int    `json:"cycle"`
	ElapsedS  int    `json:"elapsed_s"`
	IntervalS int    `json:"interval_s"`
	Attempt   int    `json:"attempt"`
	StopKind  string `json:"stop_kind"`
	Action    string `json:"action"`
	Reason    string `json:"reason"`
	FinalPane string `json:"final_pane"`
}

func writeEscalationReport(workspace, phase string, cycle int, ev StopEvent, verdict ReviewVerdict) error {
	report := escalationReport{
		Phase:     phase,
		Cycle:     cycle,
		ElapsedS:  ev.ElapsedS,
		IntervalS: ev.IntervalS,
		Attempt:   ev.Attempt,
		StopKind:  string(ev.Kind),
		Action:    string(verdict.Action),
		Reason:    verdict.Reason,
		FinalPane: ev.StdoutTail,
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(workspace, fmt.Sprintf("%s-escalation-report.json", phase))
	return os.WriteFile(path, data, 0o644)
}
