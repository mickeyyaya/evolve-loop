package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// doctor.go — per-CLI auth + binary preflight (Go port of lib/doctor.sh's
// shallow path + an optional deep live-noop via the runner seam).
//
// Simplification vs bash: auth detection is file-based (the bash Keychain
// branch is macOS-specific and not portably testable); the file fallback
// covers the common case. Verdict + exit-code contract is preserved.

// BinaryInfo describes a CLI binary's presence on PATH.
type BinaryInfo struct {
	Present bool   `json:"present"`
	Path    string `json:"path"`
	Version string `json:"version"`
}

// AuthInfo describes a CLI's configured auth (file signals).
type AuthInfo struct {
	Configured       bool   `json:"configured"`
	Source           string `json:"source"`
	SubscriptionType string `json:"subscription_type,omitempty"`
	Hint             string `json:"hint,omitempty"`
	// AuthOptional flips the doctor verdict so a CLI that runs unauthenticated
	// (e.g. local-only ollama) is NOT marked "blocked" when Configured=false.
	// Default false preserves the previous fail-loud posture for every other
	// driver where missing auth genuinely blocks the launch.
	AuthOptional bool `json:"auth_optional,omitempty"`
}

// DeepProbe is the optional live-noop result.
type DeepProbe struct {
	Ran        bool  `json:"ran"`
	Passed     bool  `json:"passed"`
	DurationMS int64 `json:"duration_ms"`
}

// DoctorResult is one CLI's preflight outcome.
type DoctorResult struct {
	CLI         string     `json:"cli"`
	Binary      BinaryInfo `json:"binary"`
	Auth        AuthInfo   `json:"auth"`
	EnvWarnings []string   `json:"env_warnings"`
	DeepProbe   DeepProbe  `json:"deep_probe"`
	Verdict     string     `json:"verdict"` // ready | warning | blocked
}

// DoctorReport is the full preflight summary.
type DoctorReport struct {
	ScannedAt string         `json:"scanned_at"`
	Host      string         `json:"host"`
	Deep      bool           `json:"deep"`
	Results   []DoctorResult `json:"results"`
	Summary   struct {
		Ready   int `json:"ready"`
		Warning int `json:"warning"`
		Blocked int `json:"blocked"`
	} `json:"summary"`
}

// doctorBinaryFor maps a cli to its underlying binary (claude-p/claude-tmux
// → claude, codex* → codex, agy* → agy, ollama-tmux → ollama).
func doctorBinaryFor(cli string) string {
	switch cli {
	case "claude-p", "claude-tmux":
		return "claude"
	case "codex", "codex-tmux":
		return "codex"
	case "agy", "agy-tmux":
		return "agy"
	case "ollama-tmux":
		return "ollama"
	}
	return strings.TrimSuffix(cli, "-tmux")
}

func (e *Engine) doctorHome() string {
	if h, ok := lookupEnv(e.deps, "HOME"); ok && h != "" {
		return h
	}
	return os.Getenv("HOME")
}

// doctorAuth probes file-based auth signals for cli.
func (e *Engine) doctorAuth(cli string) AuthInfo {
	home := e.doctorHome()
	switch doctorBinaryFor(cli) {
	case "claude":
		if fileNonEmpty(filepath.Join(home, ".claude", ".credentials.json")) {
			return AuthInfo{Configured: true, Source: "file:credentials.json"}
		}
		return AuthInfo{Hint: "Run `claude login` or check the macOS Keychain"}
	case "codex":
		p := filepath.Join(home, ".codex", "auth.json")
		if b, err := os.ReadFile(p); err == nil && json.Valid(b) {
			return AuthInfo{Configured: true, Source: "file:~/.codex/auth.json", SubscriptionType: "chatgpt-account"}
		}
		return AuthInfo{Hint: "Run `codex login` or set OPENAI_API_KEY + BRIDGE_ALLOW_OPENAI_API_KEY=1"}
	case "agy":
		for _, d := range []string{filepath.Join(home, ".config", "agy"), filepath.Join(home, ".agy"), filepath.Join(home, "Library", "Application Support", "Antigravity")} {
			if isDir(d) {
				return AuthInfo{Configured: true, Source: "file:" + d, SubscriptionType: "google-ai"}
			}
		}
		return AuthInfo{Hint: "Run `agy` once to trigger OAuth login + directory trust"}
	case "ollama":
		// Local-only ollama needs no auth. Cloud (:cloud-tagged models) needs
		// OLLAMA_API_KEY OR an ollama signin OAuth key at ~/.ollama/id_ed25519.
		if v, ok := lookupEnv(e.deps, "OLLAMA_API_KEY"); ok && v != "" {
			return AuthInfo{Configured: true, Source: "env:OLLAMA_API_KEY", SubscriptionType: "ollama-cloud"}
		}
		if fileNonEmpty(filepath.Join(home, ".ollama", "id_ed25519")) {
			return AuthInfo{Configured: true, Source: "file:~/.ollama/id_ed25519", SubscriptionType: "ollama-cloud"}
		}
		// Local-only: NOT blocked — that's the primary WS-F use case.
		return AuthInfo{AuthOptional: true, Hint: "Local models work without auth; for :cloud models run `ollama signin` or set OLLAMA_API_KEY"}
	}
	return AuthInfo{Hint: "unknown CLI"}
}

// doctorEnvWarnings mirrors lib/doctor.sh's env-leak warnings.
func (e *Engine) doctorEnvWarnings(cli string) []string {
	var w []string
	set := func(k string) bool { v, ok := lookupEnv(e.deps, k); return ok && v != "" }
	switch cli {
	case "claude-p", "claude-tmux":
		if set("ANTHROPIC_API_KEY") {
			w = append(w, "ANTHROPIC_API_KEY is set; would route through API billing, not subscription")
		}
		if set("ANTHROPIC_BASE_URL") {
			w = append(w, "ANTHROPIC_BASE_URL is set; proxy mode would invalidate subscription billing")
		}
	case "codex", "codex-tmux":
		if set("OPENAI_API_KEY") {
			if v, _ := lookupEnv(e.deps, "BRIDGE_ALLOW_OPENAI_API_KEY"); v != "1" {
				w = append(w, "OPENAI_API_KEY is set; bridge will refuse without BRIDGE_ALLOW_OPENAI_API_KEY=1")
			}
		}
	}
	return w
}

// doctorDeep runs a bounded live-noop for the headless CLIs.
func (e *Engine) doctorDeep(ctx context.Context, cli, binary string) DeepProbe {
	switch cli {
	case "claude-tmux", "codex-tmux", "agy-tmux", "ollama-tmux":
		return DeepProbe{Ran: false} // deep covers the headless backend only
	}
	var probeArgs []string
	switch cli {
	case "claude-p":
		probeArgs = []string{"-p", "Reply only: PROBE_OK", "--model", "haiku"}
	case "codex":
		probeArgs = []string{"exec"}
	case "agy":
		probeArgs = []string{"-p", "Reply only: PROBE_OK"}
	default:
		return DeepProbe{Ran: false}
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	start := e.deps.Now()
	var out bytes.Buffer
	rc, err := e.deps.Runner(cctx, binary, probeArgs, driverEnv(e.deps), nil, &out, &out)
	dur := e.deps.Now().Sub(start).Milliseconds()
	return DeepProbe{Ran: true, Passed: err == nil && rc == 0, DurationMS: dur}
}

func (e *Engine) doctorOne(ctx context.Context, cli string, deep bool) DoctorResult {
	binary := doctorBinaryFor(cli)
	r := DoctorResult{CLI: cli, EnvWarnings: e.doctorEnvWarnings(cli), Auth: e.doctorAuth(cli)}
	if path, err := e.deps.LookPath(binary); err == nil {
		r.Binary = BinaryInfo{Present: true, Path: path, Version: doctorVersion(ctx, e.deps, binary)}
	}
	if deep && r.Binary.Present {
		r.DeepProbe = e.doctorDeep(ctx, cli, binary)
	}
	switch {
	case !r.Binary.Present:
		r.Verdict = "blocked"
	case !r.Auth.Configured && !r.Auth.AuthOptional:
		// AuthOptional=true (local-only ollama) skips the blocked verdict
		// because no auth is actually required to run. Every other driver
		// keeps the historical fail-loud posture.
		r.Verdict = "blocked"
	case r.DeepProbe.Ran && !r.DeepProbe.Passed:
		r.Verdict = "blocked"
	case len(r.EnvWarnings) > 0:
		r.Verdict = "warning"
	default:
		r.Verdict = "ready"
	}
	return r
}

// doctorVersion captures `binary --version` (best-effort, first line).
func doctorVersion(ctx context.Context, deps Deps, binary string) string {
	var out bytes.Buffer
	if _, err := deps.Runner(ctx, binary, []string{"--version"}, driverEnv(deps), nil, &out, &out); err != nil {
		return "unknown"
	}
	line := out.String()
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	return strings.TrimSpace(line)
}

// Doctor runs preflight across the manifest CLIs (or a single filter) and
// returns the report + exit code (0 ready, 1 warning, 2 blocked).
func (e *Engine) Doctor(ctx context.Context, filter string, deep bool) (DoctorReport, int) {
	rep := DoctorReport{
		ScannedAt: e.deps.Now().UTC().Format("2006-01-02T15:04:05Z"),
		Host:      runtime.GOOS,
		Deep:      deep,
	}
	for _, cli := range ManifestNames() {
		if filter != "" && cli != filter {
			continue
		}
		rep.Results = append(rep.Results, e.doctorOne(ctx, cli, deep))
	}
	for _, r := range rep.Results {
		switch r.Verdict {
		case "ready":
			rep.Summary.Ready++
		case "warning":
			rep.Summary.Warning++
		case "blocked":
			rep.Summary.Blocked++
		}
	}
	switch {
	case rep.Summary.Blocked > 0:
		return rep, 2
	case rep.Summary.Warning > 0:
		return rep, 1
	default:
		return rep, ExitOK
	}
}
