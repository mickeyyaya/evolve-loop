// Package detectcli ports legacy/scripts/dispatch/detect-cli.sh.
//
// Identifies which AI coding CLI is currently driving the skill, used
// by platform-detect.md + CLAUDE.md overlay selection. Probe order:
//
//  1. Options.Platform          — explicit operator override (DI seam)
//  2. CLAUDE_CODE_*             → claude
//  3. GEMINI_CLI / GEMINI_API_KEY → gemini
//  4. CODEX_HOME / CODEX_API_KEY  → codex
//  5. command -v agy            → antigravity
//  6. unknown
package detectcli

import (
	"os/exec"
)

// Result captures the detected CLI + the probe that matched.
type Result struct {
	CLI    string `json:"cli"`
	Reason string `json:"reason"`
}

// Options exposes seams for testing. Env returns the value of an env
// var (defaults to os.Getenv). LookPath checks PATH for a binary name
// (defaults to exec.LookPath). Platform is an explicit CLI override
// (replaces EVOLVE_PLATFORM env read). Both Env and LookPath must be
// non-nil if supplied; the zero-value Options uses production defaults.
type Options struct {
	Env      func(name string) string
	LookPath func(name string) (string, error)
	// Platform, when non-empty, is returned as the detected CLI without
	// consulting env or PATH. Set via --platform in cmd_detect_cli.go.
	Platform string
}

// Detect runs the probe chain and returns a Result. Pass zero-value
// Options for production behavior; tests can inject stubs.
func Detect(opts Options) Result {
	getEnv := opts.Env
	if getEnv == nil {
		getEnv = osGetenv
	}
	lookPath := opts.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}

	switch {
	case opts.Platform != "":
		return Result{CLI: opts.Platform, Reason: "explicit override via Options.Platform"}
	case getEnv("CLAUDE_CODE_INTERACTIVE") != "" || getEnv("CLAUDE_CODE_SESSION_ID") != "":
		return Result{CLI: "claude", Reason: "CLAUDE_CODE_* env detected"}
	case getEnv("GEMINI_CLI") != "" || getEnv("GEMINI_API_KEY") != "":
		return Result{CLI: "gemini", Reason: "GEMINI_* env detected"}
	case getEnv("CODEX_HOME") != "" || getEnv("CODEX_API_KEY") != "":
		return Result{CLI: "codex", Reason: "CODEX_* env detected"}
	}

	if _, err := lookPath("agy"); err == nil {
		return Result{CLI: "antigravity", Reason: "agy binary on PATH detected"}
	}

	return Result{CLI: "unknown", Reason: "no probe matched"}
}

// osGetenv is a thin wrapper for the default Env seam.
func osGetenv(name string) string {
	return osGetenvImpl(name)
}
