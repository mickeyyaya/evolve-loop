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
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// probeOnce caches the (process-lifetime) sandbox.Probe() result so each
// driver call doesn't re-exec LookPath. Tests using a custom SandboxWrap
// bypass this entirely.
var (
	probeOnce sync.Once
	probedRes sandbox.ProbeResult
)

func cachedProbe() sandbox.ProbeResult {
	probeOnce.Do(func() { probedRes = sandbox.Probe() })
	return probedRes
}

// defaultSandboxWrap returns the production SandboxWrapper closure with the
// real cached probe. Production callers use this; tests should call
// defaultSandboxWrapWithProbe with an injected probe func to avoid coupling
// to the package-level sync.Once.
func defaultSandboxWrap(deps Deps) SandboxWrapper {
	return defaultSandboxWrapWithProbe(deps, cachedProbe)
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
		if mode == config.SandboxModeOff {
			return nil, false
		}

		// Nested-claude detection (mirrors preflight): the outer Claude Code
		// session already imposes OS sandbox + Tier-1 hooks, so wrapping again
		// causes EPERM noise without adding safety. Only honored in auto mode;
		// "on" forces wrap regardless.
		if mode == config.SandboxModeAuto && isNestedClaude(deps) {
			return nil, false
		}

		probe := probeFunc()
		if !probe.Available {
			// EVOLVE_SANDBOX=on declares mandatory confinement, so the silent
			// degrade `auto` accepts would silently violate operator intent.
			// Emit a WARN to deps.Stderr (when available) so the bypass is
			// observable even though we still return false — the driver
			// boundary doesn't have a hard-fail path today (raising
			// available=true with no prefix would be worse: an abort with
			// nothing to abort against). Future work: surface as an error.
			if mode == config.SandboxModeOn && deps.Stderr != nil {
				fmt.Fprintf(deps.Stderr, "[bridge] WARN: EVOLVE_SANDBOX=on but no host sandbox binary (%s); phase will run UNCONFINED (operator intent violated). Reason: %s\n",
					probe.OS, probe.Reason)
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
			sbplPath := filepath.Join(req.Workspace, "sandbox-"+req.Phase+".sb")
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

// isNestedClaude reports whether the bridge is running inside an outer Claude
// Code session. The signal is the same one preflight uses
// (CLAUDE_CODE_ENTRYPOINT or CLAUDE_CODE_SESSION_ID).
func isNestedClaude(deps Deps) bool {
	for _, k := range []string{"CLAUDE_CODE_ENTRYPOINT", "CLAUDE_CODE_SESSION_ID"} {
		if v := deps.Env[k]; v != "" {
			return true
		}
		if deps.LookupEnv != nil {
			if v, ok := deps.LookupEnv(k); ok && v != "" {
				return true
			}
		}
	}
	return false
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
