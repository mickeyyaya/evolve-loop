// Package detectcli identifies which AI coding CLI is driving the session from explicit override, environment and PATH.
// See docs/architecture/packages/internal-detectcli.md.
package detectcli

import (
	"os/exec"
)

type Result struct {
	CLI    string `json:"cli"`
	Reason string `json:"reason"`
}

type Options struct {
	Env      func(name string) string
	LookPath func(name string) (string, error)
	Platform string
}

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

func Canonical(cli string) string {
	if cli == "antigravity" {
		return "agy"
	}
	return cli
}

func osGetenv(name string) string {
	return osGetenvImpl(name)
}
