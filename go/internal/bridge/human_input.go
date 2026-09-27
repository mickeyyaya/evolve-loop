package bridge

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"
)

// humanActive reports whether both gates are satisfied.
func humanActive(deps Deps, humanInput bool) bool {
	if !humanInput {
		return false
	}
	v, _ := lookupEnv(deps, "BRIDGE_HUMAN_SIMULATION")
	return v == "1"
}

// humanSampleMS returns a truncated-Gaussian delay (mean±sd, floor 10ms).
// The value feeds Sleep only, so its exact magnitude is irrelevant to tests.
func humanSampleMS(meanMS, sdMS int) time.Duration {
	v := rand.NormFloat64()*float64(sdMS) + float64(meanMS)
	if v < 10 {
		v = 10
	}
	return time.Duration(int(v)) * time.Millisecond
}

// humanBootPause simulates a 1.5–3.5s human reaction to a freshly-booted REPL.
func humanBootPause(deps Deps) {
	ms := 1500 + rand.Intn(2001)
	fmt.Fprintf(deps.Stderr, "[human-input] boot pause %dms\n", ms)
	deps.Sleep(time.Duration(ms) * time.Millisecond)
}

// pastePrompt delivers one complete prompt. Human mode changes only the review
// pause; both cadences stop on the first failed transport operation.
func pastePrompt(ctx context.Context, deps Deps, pfx, session, promptFile string, human bool) (pasteOutcome, error) {
	if err := deps.Tmux.LoadBuffer(ctx, session, promptFile); err != nil {
		return pasteOutcome{}, fmt.Errorf("prompt load-buffer: %w", err)
	}
	if err := deps.Tmux.PasteBuffer(ctx, session); err != nil {
		return pasteOutcome{}, fmt.Errorf("prompt paste-buffer: %w", err)
	}
	data, readErr := os.ReadFile(promptFile)
	size := len(data)
	if readErr != nil {
		fmt.Fprintf(deps.Stderr, "%s paste settle: prompt file unreadable (%v) — assuming a large paste\n", pfx, readErr)
		size = pasteSettleMaxBytes
	}
	settle := pasteSettleFor(size)
	if human {
		lines := strings.Count(string(data), "\n") + 1
		mean := lines * 80
		if mean < 200 {
			mean = 200
		}
		fmt.Fprintf(deps.Stderr, "[human-input] paste review (%d lines)\n", lines)
		settle = humanSampleMS(mean, mean/4)
	}
	out, err := settlePasteThenEnter(ctx, deps, pfx, session, settle, size)
	if err != nil {
		return out, fmt.Errorf("prompt submit: %w", err)
	}
	return out, nil
}

// humanSendKeysCSV sends each CSV key token with a human-shaped inter-key
// delay (vs the default bulk send).
func humanSendKeysCSV(ctx context.Context, deps Deps, session, csv string) {
	for _, tok := range strings.Split(csv, ",") {
		if tok == "Enter" {
			_ = deps.Tmux.SendKeys(ctx, session, "", true)
		} else if tok != "" {
			_ = deps.Tmux.SendKeys(ctx, session, tok, false)
		}
		deps.Sleep(humanSampleMS(65, 20))
	}
	fmt.Fprintf(deps.Stderr, "[human-input] sent keys: %s\n", csv)
}

// humanReadingPause pauses ~ words/wpm before responding to a prompt.
func humanReadingPause(deps Deps, text string) {
	words := len(strings.Fields(text))
	if words < 3 {
		words = 3
	}
	ms := 60000 * words / 220
	fmt.Fprintf(deps.Stderr, "[human-input] reading pause (~%d words)\n", words)
	deps.Sleep(humanSampleMS(ms, ms/4))
}
