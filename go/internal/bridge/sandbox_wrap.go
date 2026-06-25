package bridge

// sandbox_wrap.go — default SandboxWrap implementation (Workstream B).
//
// CLI-agnostic confinement: every driver (claude/codex/agy/ollama) running a
// source-writing phase gets wrapped in the host's OS sandbox
// (sandbox-exec on macOS, bwrap on Linux). The non-Claude drivers historically
// bypassed the trust kernel entirely (Issue 2 from cycle 119).
//
// This file owns ONLY the decision + prefix-argv synthesis. Drivers (tmux +
// headless) call deps.SandboxWrap at their launch site and either prepend the
// returned argv or, when available=false, run unwrapped (degraded — a soft
// fallback so a missing sandbox binary or nested-claude doesn't kill cycles).
//
// Probe is cached behind sync.Once because it shells to LookPath; the cached
// result is captured in the closure that withDefaults returns.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// defaultSandboxWrap returns the production SandboxWrapper closure. The probe —
// including the capability measurement — is cached process-wide inside
// sandbox.Probe itself, so production passes it directly; tests should call
// defaultSandboxWrapWithProbe with an injected probe func to bypass the cache.
func defaultSandboxWrap(deps Deps) SandboxWrapper {
	return defaultSandboxWrapWithProbe(deps, sandbox.Probe)
}

// defaultSandboxWrapWithProbe is the test seam: the probeFunc lets tests
// drive the darwin/linux/unavailable branches deterministically without
// shelling to the real LookPath or hitting the package-level sync.Once.
// The mode + nested-claude signals still come from deps.Env per call.
func defaultSandboxWrapWithProbe(deps Deps, probeFunc func() sandbox.ProbeResult) SandboxWrapper {
	return func(req SandboxWrapRequest) ([]string, bool) {
		// Read mode from request-local Env (envchain pattern). Default auto.
		mode := strings.TrimSpace(deps.Env[envSandboxMode])
		if mode == "" {
			mode = config.SandboxModeAuto
		}
		// Normalize any UNRECOGNIZED value to auto, mirroring config.applyEnv's
		// validation contract. Pre-fix, an unknown value (operator typo like
		// "1") was neither "off" nor "auto", so it slipped past the
		// nested-claude skip below and forced a sandbox-exec wrap — which hangs
		// claude's REPL boot on nested macOS (exit=80 ExitREPLBootTimeout; the
		// 2026-06-13 soak burned cycles 324-326 on exactly this). Treat an
		// unknown value as auto (the safe default) and WARN so it's observable.
		switch mode {
		case config.SandboxModeOff, config.SandboxModeOn, config.SandboxModeAuto:
			// recognized — leave as-is
		default:
			if deps.Stderr != nil {
				fmt.Fprintf(deps.Stderr, "[bridge] WARN: EVOLVE_SANDBOX=%q unrecognized (want auto|on|off); treating as auto\n", mode)
			}
			mode = config.SandboxModeAuto
		}
		if mode == config.SandboxModeOff {
			return nil, false
		}

		// Single-source confinement decision — the SAME DetectNested +
		// ShouldWrap that preflight consumes (internal/adapters/sandbox). The
		// wrap is skipped for ALL modes when we are nested-Claude (the outer
		// session already imposes OS sandbox + Tier-1 hooks, and on macOS the
		// inner sandbox-exec returns sandbox_apply() EPERM and hangs the REPL
		// boot, exit=80) or when no usable sandbox binary is present. `on`
		// declares mandatory confinement, so its bypass is surfaced loudly
		// rather than silently honoured-as-unconfined.
		probe := probeFunc()
		wrap, reason := sandbox.ShouldWrap(sandbox.DetectNested(depEnvGetter(deps)), probe)
		if !wrap {
			if mode == config.SandboxModeOn && deps.Stderr != nil {
				fmt.Fprintf(deps.Stderr, "[bridge] WARN: EVOLVE_SANDBOX=on but inner sandbox not applied; phase runs UNCONFINED at the inner layer. Reason: %s\n", reason)
			}
			return nil, false
		}

		// Build the sandbox.Config for this phase. WritePaths covers the
		// worktree (the only place source writes are permitted) plus the
		// workspace (for artifact/log files the agent must write) plus /tmp
		// (the bridge's scratch space).
		cfg := sandbox.Config{
			RepoRoot:     req.RepoRoot,
			ReadOnlyRepo: true,
			WritePaths:   sandboxWritePaths(req),
		}

		switch probe.OS {
		case "darwin":
			// Materialize the SBPL to a per-phase file so the prefix argv stays
			// short + shell-quote-safe under tmux SendKeys. sandbox-exec(1)
			// distinguishes -p (inline SBPL string) from -f (file path) —
			// passing a path with -p makes sandbox-exec parse the path AS the
			// profile, which would silently leave the phase unconfined. Always
			// -f here; the in-memory adapter at adapters/sandbox.Sandbox.Exec
			// stays on -p because it holds the SBPL string, not a file.
			sbpl := sandbox.GenerateSBPL(cfg)
			// ADR-0049 S0 / gap G6: write the SBPL to a PER-INVOCATION profile
			// dir, not a shared <workspace>/sandbox-<phase>.sb. Two same-phase
			// dispatches sharing a workspace (a re-dispatch, two fan-out workers,
			// or two runs reusing a cycle number) otherwise write the same file —
			// and if their WritePaths differ, B's profile landing between A's
			// write and A's sandbox-exec read confines A to B's allow-list (A's
			// legit source writes EPERM-denied). A mktemp -d (0o700) per
			// invocation isolates them — the per-invocation sandbox-profile
			// pattern (CERT FIO21-C; Codex generates a profile per launch). A
			// mkdir failure degrades to the shared workspace profile (confinement
			// preserved, isolation lost) rather than running unconfined. No-op for
			// the live sequential loop: a lone dispatch just gets its own subdir.
			sbplDir := req.Workspace
			if req.Workspace != "" {
				mk := deps.MkScratchDir
				if mk == nil {
					mk = os.MkdirTemp
				}
				if d, err := mk(req.Workspace, "sbprofile-"); err == nil {
					sbplDir = d
				} else if deps.Stderr != nil {
					fmt.Fprintf(deps.Stderr, "[bridge] WARN: per-invocation sandbox profile dir failed (%v); using shared workspace profile (isolation lost)\n", err)
				}
			}
			sbplPath := filepath.Join(sbplDir, "sandbox-"+req.Phase+".sb")
			if err := os.WriteFile(sbplPath, []byte(sbpl), 0o644); err != nil {
				// Can't write the profile → can't wrap. Caller degrades.
				return nil, false
			}
			return []string{"sandbox-exec", "-f", sbplPath}, true
		case "linux":
			// bwrap takes the inner argv inline. We don't have it here, so we
			// return just the prefix portion via the dedicated helper.
			return sandbox.BwrapPrefix(cfg), true
		default:
			return nil, false
		}
	}
}

// sandboxWritePaths returns the absolute write-allowlist for a source-writing
// phase: worktree (source) + workspace (artifacts) + /tmp (scratch). Empty
// req.Worktree means the orchestrator didn't designate one — return only the
// workspace so non-worktree code paths don't silently land in the main tree.
func sandboxWritePaths(req SandboxWrapRequest) []string {
	out := []string{}
	if req.Worktree != "" {
		out = append(out, req.Worktree)
	}
	if req.Workspace != "" {
		out = append(out, req.Workspace)
	}
	out = append(out, "/tmp")
	return out
}

// depEnvGetter adapts a Deps to the getenv func sandbox.DetectNested expects,
// preserving the bridge's request-local env-chain precedence: the explicit
// Env map first, then the LookupEnv seam. Centralizing nested detection in
// adapters/sandbox.DetectNested removed the bridge-local heuristic that used
// to live here (and diverged from preflight's).
func depEnvGetter(deps Deps) func(string) string {
	return func(k string) string {
		if deps.Env != nil {
			if v := deps.Env[k]; v != "" {
				return v
			}
		}
		if deps.LookupEnv != nil {
			if v, ok := deps.LookupEnv(k); ok {
				return v
			}
		}
		return ""
	}
}

// envSandboxMode is the env var that overrides cfg.SandboxMode on the
// subprocess hot path (the bridge doesn't hold a *config.RoutingConfig; it
// reads via the same envchain pattern every other phase flag uses).
const envSandboxMode = "EVOLVE_SANDBOX"

// sandboxPrefixForLaunch is the shared adapter that turns the SandboxWrap
// decision into a per-driver consumable. Returns (prefix []string, true) only
// when this is a source-writing phase (cfg.Worktree non-empty) AND the wrap
// is available; otherwise (nil, false) for "run unwrapped". Centralizing the
// gate here keeps every driver call site identical. Takes *Config to match
// the driver-call convention (cfg is passed by pointer throughout this pkg).
func sandboxPrefixForLaunch(deps Deps, cfg *Config) ([]string, bool) {
	if cfg == nil || cfg.Worktree == "" {
		return nil, false // not a source-writing phase
	}
	if deps.SandboxWrap == nil {
		return nil, false
	}
	return deps.SandboxWrap(SandboxWrapRequest{
		Phase:     cfg.Agent,
		Workspace: cfg.Workspace,
		Worktree:  cfg.Worktree,
		RepoRoot:  cfg.ProjectRoot,
	})
}

// shellQuotePOSIX wraps s in POSIX single quotes, escaping any embedded
// single quotes. Used to splice the sandbox prefix into the *-tmux driver's
// launchCmd string (SendKeys gets a single shell line, not an argv slice).
//
// The "safe" character set is intentionally narrow: a-z, A-Z, 0-9, and the
// path/option chars - _ / . , + : @ %. Anything else (including !, (, ),
// &, |, ;, <, >, ~, #, =, {, }, [, ], spaces, quotes, $, backtick, etc.)
// triggers single-quoting. POSIX single-quoting is always correct, so when
// in doubt we quote. The previous narrow blocklist missed several active
// shell metacharacters; this allow-list flips the safety bias.
func shellQuotePOSIX(s string) string {
	if s == "" {
		return "''"
	}
	safe := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z',
			c >= 'A' && c <= 'Z',
			c >= '0' && c <= '9',
			c == '-', c == '_', c == '/', c == '.',
			c == ',', c == '+', c == ':', c == '@', c == '%':
			continue
		default:
			safe = false
		}
		if !safe {
			break
		}
	}
	if safe {
		return s
	}
	var b strings.Builder
	b.WriteByte('\'')
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			b.WriteString(`'\''`)
			continue
		}
		b.WriteByte(s[i])
	}
	b.WriteByte('\'')
	return b.String()
}

// joinPrefixForTmux turns a sandbox prefix argv into a single shell-safe
// string for prepending to a *-tmux driver's launchCmd line.
func joinPrefixForTmux(prefix []string) string {
	parts := make([]string, len(prefix))
	for i, p := range prefix {
		parts[i] = shellQuotePOSIX(p)
	}
	return strings.Join(parts, " ")
}

// wrapHeadlessInvocation transforms a (name, args) pair into the sandboxed
// equivalent: when the sandbox prefix is available, name becomes prefix[0]
// (e.g. "sandbox-exec") and args becomes prefix[1:]+[name]+oldArgs. Otherwise
// returns the inputs unchanged. Centralizes the rewrite so claude-p and codex
// drivers (and any future headless driver) share one path.
func wrapHeadlessInvocation(deps Deps, cfg *Config, name string, args []string) (string, []string) {
	prefix, ok := sandboxPrefixForLaunch(deps, cfg)
	if !ok {
		return name, args
	}
	newArgs := make([]string, 0, len(prefix)-1+1+len(args))
	newArgs = append(newArgs, prefix[1:]...)
	newArgs = append(newArgs, name)
	newArgs = append(newArgs, args...)
	return prefix[0], newArgs
}
