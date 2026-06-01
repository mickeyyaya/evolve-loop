package modelquery

import (
	"strings"
	"testing"
)

// The fixtures below are verbatim captures of each CLI's /model picker pane
// (tmux capture-pane), collected live on 2026-06-01. They are the ground-truth
// regression corpus for the per-CLI parsers.

const codexPickerPane = `╭──────────────────────────────────────────────╮
│ >_ OpenAI Codex (v0.135.0)                   │
│ model:     gpt-5.5 medium   /model to change │
╰──────────────────────────────────────────────╯
› /model
  Select Model and Effort
  Access legacy models by running codex -m <model_name> or in your config.toml
› 1. gpt-5.5 (current)  Frontier model for complex coding, research, and real-world work.
  2. gpt-5.4            Strong model for everyday coding.
  3. gpt-5.4-mini       Small, fast, and cost-efficient model for simpler coding tasks.
  4. gpt-5.3-codex      Coding-optimized model.
  5. gpt-5.2            Optimized for professional work and long-running agents.
  Press enter to confirm or esc to go back`

const agyPickerPane = `      ▄▀▀▄        Antigravity CLI 1.0.3
     ▀▀▀▀▀▀       mickeyyaya@gmail.com (Google AI Pro)
>
Switch Model
> Gemini 3.5 Flash (Medium)    (current)
  Gemini 3.5 Flash (High)
  Gemini 3.5 Flash (Low)
  Gemini 3.1 Pro (Low)
  Gemini 3.1 Pro (High)
  Claude Sonnet 4.6 (Thinking)
  Claude Opus 4.6 (Thinking)
  GPT-OSS 120B (Medium)
Keyboard: ↑/↓ Navigate  enter Select  esc Go Back
esc to cancel                                  Gemini 3.5 Flash (Medium)`

const claudePickerPane = ` ▐▛███▜▌   Claude Code v2.1.159
❯ /model
  Select model
  Switch between Claude models. Your pick becomes the default for new sessions. For other/previous model names, specify with --model.
  ❯ 1. Default (recommended) ✔  Opus 4.8 with 1M context · Most capable for complex work
    2. Sonnet                   Sonnet 4.6 · Best for everyday tasks
    3. Haiku                    Haiku 4.5 · Fastest for quick answers
  ● High effort (default) ←/→ to adjust
  Enter to set as default · s to use this session only · Esc to cancel`

func assertIDs(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("parsed ids mismatch:\n got=%v\nwant=%v", got, want)
	}
}

func TestParseCodexPicker(t *testing.T) {
	got := parseCodexPicker(codexPickerPane)
	assertIDs(t, got, []string{"gpt-5.5", "gpt-5.4", "gpt-5.4-mini", "gpt-5.3-codex", "gpt-5.2"})
}

func TestParseAgyPicker(t *testing.T) {
	got := parseAgyPicker(agyPickerPane)
	assertIDs(t, got, []string{
		"Gemini 3.5 Flash (Medium)",
		"Gemini 3.5 Flash (High)",
		"Gemini 3.5 Flash (Low)",
		"Gemini 3.1 Pro (Low)",
		"Gemini 3.1 Pro (High)",
		"Claude Sonnet 4.6 (Thinking)",
		"Claude Opus 4.6 (Thinking)",
		"GPT-OSS 120B (Medium)",
	})
}

func TestParseClaudePicker(t *testing.T) {
	// claude's picker labels are aliases + model versions; the dispatch-usable
	// identifier is the model family (claude --model accepts opus|sonnet|haiku).
	got := parseClaudePicker(claudePickerPane)
	assertIDs(t, got, []string{"opus", "sonnet", "haiku"})
}

func TestParsersIgnoreChromeAndEmpty(t *testing.T) {
	if got := parseCodexPicker("just the banner\nno picker here"); len(got) != 0 {
		t.Fatalf("codex parser should return nothing for non-picker pane, got %v", got)
	}
	if got := parseAgyPicker("no switch model header"); len(got) != 0 {
		t.Fatalf("agy parser should return nothing without the Switch Model region, got %v", got)
	}
	if got := parseClaudePicker(""); len(got) != 0 {
		t.Fatalf("claude parser should return nothing for empty pane, got %v", got)
	}
}
