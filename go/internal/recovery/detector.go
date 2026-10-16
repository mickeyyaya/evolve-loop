package recovery

import "strings"

type TerminalCause string

const (
	CauseModelInvalid   TerminalCause = "model_invalid"
	CauseCLISelfUpdated TerminalCause = "cli_self_updated"
	CauseDeadShell      TerminalCause = "dead_shell"
	CauseUnknown        TerminalCause = "unknown"
)

func (c TerminalCause) SessionRecoverable() bool {
	return c == CauseDeadShell || c == CauseCLISelfUpdated
}

type FatalSignature struct {
	Substr string
	Cause  TerminalCause
	Note   string
}

type FatalPaneDetector struct {
	sigs []FatalSignature
}

func NewFatalPaneDetector(sigs []FatalSignature) *FatalPaneDetector {
	return &FatalPaneDetector{sigs: sigs}
}

func SeedDetector() *FatalPaneDetector {
	return NewFatalPaneDetector([]FatalSignature{
		{
			Substr: "There's an issue with the selected model",
			Cause:  CauseModelInvalid,
			Note:   "claude boot error (cycle-262 retro: --model auto)",
		},
		{
			Substr: "Update ran successfully! Please restart",
			Cause:  CauseCLISelfUpdated,
			Note:   "codex self-upgrade mid-launch (cycle-262 build)",
		},
		{
			Substr: ": command not found",
			Cause:  CauseDeadShell,
			Note:   "shell rejecting agent-directed input — the REPL is gone (cycle-262: nudged bare zsh)",
		},
		{
			Substr: "\nquote>",
			Cause:  CauseDeadShell,
			Note:   "zsh continuation prompt after prompt spill — the REPL is gone (cycle-274 codex update-menu wedge)",
		},
		{
			Substr: "\nbquote>",
			Cause:  CauseDeadShell,
			Note:   "zsh backquote continuation prompt after prompt spill — the REPL is gone (cycle-274 codex update-menu wedge)",
		},
		{
			Substr: "\ndquote>",
			Cause:  CauseDeadShell,
			Note:   "zsh double-quote continuation prompt after prompt spill — the REPL is gone (cycle-274/277 transcript variant, R3.3)",
		},
		{
			Substr: "\nheredoc>",
			Cause:  CauseDeadShell,
			Note:   "zsh heredoc continuation prompt after prompt spill — the REPL is gone (cycle-274/277 transcript variant, R3.3)",
		},
	})
}

func (d *FatalPaneDetector) Signatures() []string {
	if d == nil {
		return nil
	}
	out := make([]string, 0, len(d.sigs))
	for _, sig := range d.sigs {
		if sig.Substr != "" {
			out = append(out, sig.Substr)
		}
	}
	return out
}

func (d *FatalPaneDetector) Detect(pane string) (TerminalCause, string, bool) {
	if d == nil || pane == "" {
		return CauseUnknown, "", false
	}
	for _, sig := range d.sigs {
		if sig.Substr != "" && strings.Contains(pane, sig.Substr) {
			return sig.Cause, sig.Substr, true
		}
	}
	return CauseUnknown, "", false
}
