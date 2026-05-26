package bridge

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"
)

// human_input.go — the Go port of lib/human-input.sh: a behavioral-
// plausibility timing layer for the tmux drivers. It changes only the
// CADENCE of input (Gaussian inter-key delays, boot/review/reading
// pauses) — never the tmux operations or the produced artifact. All
// delays go through the Deps.Sleep seam, so unit tests (no-op Sleep) are
// deterministic and the layer is a pure no-op-for-correctness.
//
// Double-gated OFF by default, mirroring the bash two-gate:
//   1. BRIDGE_HUMAN_SIMULATION=1 (host opt-in)
//   2. --human-input on the launch (per-invocation choice)

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

// humanPasteWithReview pastes the prompt, then pauses proportionally to its
// length (a human glancing over it) before pressing Enter.
func humanPasteWithReview(ctx context.Context, deps Deps, session, promptFile string) {
	_ = deps.Tmux.LoadBuffer(ctx, session, promptFile)
	_ = deps.Tmux.PasteBuffer(ctx, session)
	lines := 1
	if data, err := os.ReadFile(promptFile); err == nil {
		lines = strings.Count(string(data), "\n") + 1
	}
	mean := lines * 80
	if mean < 200 {
		mean = 200
	}
	fmt.Fprintf(deps.Stderr, "[human-input] paste review (%d lines)\n", lines)
	deps.Sleep(humanSampleMS(mean, mean/4))
	_ = deps.Tmux.SendKeys(ctx, session, "", true)
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
