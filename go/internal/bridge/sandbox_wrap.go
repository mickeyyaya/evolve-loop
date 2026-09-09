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
// returned argv or enforce the profile's confinement requirement when no
// wrapper is available. Mandatory profiles fail closed without an explicit opt-out.
//
// Probe is cached behind sync.Once because it shells to LookPath; the cached
// result is captured in the closure that withDefaults returns.

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
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

		// Share the measured capability decision with preflight. A session hint
		// never substitutes for this profile's actual OS wrapper.
		probe := probeFunc()
		wrap, reason := sandbox.ShouldWrap(sandbox.DetectNested(depEnvGetter(deps)), probe)
		if !wrap {
			if mode == config.SandboxModeOn && deps.Stderr != nil {
				fmt.Fprintf(deps.Stderr, "[bridge] WARN: EVOLVE_SANDBOX=on but inner sandbox not applied; phase runs UNCONFINED at the inner layer. Reason: %s\n", reason)
			}
			return nil, false
		}

		// A terminal grant is tied to a real assigned device, never a caller's
		// arbitrary writable path. Headless and Linux profiles do not need it.
		if probe.OS == "darwin" && req.TerminalPath != "" {
			if err := validateSandboxTerminal(req.TerminalPath); err != nil {
				if deps.Stderr != nil {
					fmt.Fprintf(deps.Stderr, "[bridge] sandbox terminal unavailable: %v\n", err)
				}
				return nil, false
			}
		}

		// Build the sandbox.Config for this phase. WritePaths covers the
		// worktree (the only place source writes are permitted) plus the
		// workspace (for artifact/log files the agent must write) plus /tmp
		// (the bridge's scratch space). HomeDir is read-allowed so tmux CLIs can
		// load their own config/auth state; repo writes remain confined below.
		home, _ := os.UserHomeDir()
		cfg := sandbox.Config{
			TerminalPath:  req.TerminalPath,
			RepoRoot:      req.RepoRoot,
			HomeDir:       home,
			ReadOnlyRepo:  true,
			WritePaths:    sandboxWritePaths(req),
			AllowNetwork:  req.AllowNetwork,
			DenyPaths:     req.DenyPaths,
			DenyReadPaths: req.DenyReadPaths,
		}
		// SBPL matches filesystem paths after symlink resolution. In particular,
		// /var and /tmp aliases on macOS must not bypass the repository deny.
		gitWrites, gitDenies, err := sandboxGitWritePaths(req.Worktree, probe.OS)
		if err == nil {
			gitDenies, err = resolveSandboxDenials(gitDenies, "", "", true)
		}
		if err != nil {
			if deps.Stderr != nil {
				fmt.Fprintf(deps.Stderr, "[bridge] sandbox capability unavailable: %v\n", err)
			}
			return nil, false
		}
		if req.Phase == "retrospective" {
			lessonPath, err := retrospectiveLessonPath(req.RepoRoot)
			if err != nil {
				if deps.Stderr != nil {
					fmt.Fprintf(deps.Stderr, "[bridge] sandbox lesson grant unavailable: %v\n", err)
				}
				return nil, false
			}
			cfg.WritePaths = append(cfg.WritePaths, lessonPath)
		}
		cfg.WritePaths = append(cfg.WritePaths, gitWrites...)
		cfg.DenyPaths = append(append([]string{}, cfg.DenyPaths...), gitDenies...)
		paths := append([]string{cfg.RepoRoot}, cfg.WritePaths...)
		for i, path := range paths {
			if path == "" {
				continue
			}
			real, err := canonicalSandboxPath(path)
			if err != nil {
				if deps.Stderr != nil {
					fmt.Fprintf(deps.Stderr, "[bridge] sandbox path resolution failed: %v\n", err)
				}
				return nil, false
			}
			paths[i] = real
		}
		cfg.RepoRoot, cfg.WritePaths = paths[0], paths[1:]

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
			for _, path := range append(append([]string{}, req.DenyPaths...), req.DenyReadPaths...) {
				info, err := os.Stat(path)
				if err != nil || (!info.IsDir() && slices.Contains(req.DenyReadPaths, path)) {
					if deps.Stderr != nil {
						fmt.Fprintf(deps.Stderr, "[bridge] sandbox policy unavailable: Linux denial target %q must exist; read denials require directories (stat: %v)\n", path, err)
					}
					return nil, false
				}
			}
			// bwrap takes the inner argv inline. We don't have it here, so we
			// return just the prefix portion via the dedicated helper.
			return append([]string{"bwrap"}, sandbox.BwrapPrefix(cfg)...), true
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
// when the launch carries a worktree or explicitly requires confinement and
// the wrap is available; otherwise (nil, false) for "run unwrapped".
func sandboxPrefixForLaunch(deps Deps, cfg *Config, terminalPath string) ([]string, bool) {
	if cfg == nil || cfg.Worktree == "" && !cfg.RequireSandbox {
		return nil, false // not a source-writing phase
	}
	if deps.SandboxWrap == nil {
		return nil, false
	}
	// Reaching here means a cloud model CLI requested confinement. ollama-tmux —
	// the only local driver — never requests this wrapper, so only
	// claude/codex/agy-tmux get here. That covers both source-writing phases
	// (built-in build/tdd or a custom writes_source phase) AND the boot/live-smoke
	// probes, which set a scratch Worktree via applyScratchCwd — all of them need
	// the network to reach the model API. The sandbox's network deny would block
	// that connection, and "allow model, deny the rest" is NOT expressible: SBPL/
	// bwrap can't filter per-host, and the model endpoint can't be proxied without
	// breaking subscription billing (driver_claudetmux.go aborts on a proxy base
	// URL). So network MUST be allowed here regardless of the profile value — a
	// false sandbox.allow_network on a phase that reaches the sandbox is a
	// misconfiguration, not a control. Force it true structurally so nothing —
	// including a future custom writes_source phase — boots network-denied and
	// hangs unable to reach the model. The real confinement boundary here is
	// filesystem (ReadOnlyRepo + WritePaths + DenyPaths) + the kernel PreToolUse
	// hooks, not network.
	if !cfg.AllowNetwork && deps.Stderr != nil {
		fmt.Fprintf(deps.Stderr, "[bridge] WARN: source-writing phase %q has sandbox.allow_network=false; forcing true (a sandboxed model-reaching CLI cannot boot with network denied)\n", cfg.Agent)
	}
	return deps.SandboxWrap(SandboxWrapRequest{
		TerminalPath:  terminalPath,
		Phase:         cfg.Agent,
		Workspace:     cfg.Workspace,
		Worktree:      cfg.Worktree,
		RepoRoot:      cfg.ProjectRoot,
		AllowNetwork:  true, // forced — see above; source-writing ⇒ model network required
		DenyPaths:     cfg.DenyPaths,
		DenyReadPaths: cfg.DenyReadPaths,
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

// wrapHeadlessInvocation transforms a command into its sandboxed equivalent
// and reports whether confinement was installed. Callers that require a
// sandbox fail closed when wrapped is false.
func wrapHeadlessInvocation(deps Deps, cfg *Config, name string, args []string) (wrappedName string, wrappedArgs []string, wrapped bool) {
	prefix, ok := sandboxPrefixForLaunch(deps, cfg, "")
	if !ok {
		return name, args, false
	}
	newArgs := make([]string, 0, len(prefix)-1+1+len(args))
	newArgs = append(newArgs, prefix[1:]...)
	newArgs = append(newArgs, name)
	newArgs = append(newArgs, args...)
	return prefix[0], newArgs, true
}

// sandboxRequiredButUnavailable fails mandatory controls closed when no wrapper
// applied. A nesting marker cannot prove the outer environment enforces this
// profile's denials. Only the explicit host sandbox-off opt-out permits a
// mandatory launch to continue unconfined.
func sandboxRequiredButUnavailable(deps Deps, cfg *Config, wrapped bool) bool {
	if cfg == nil || !cfg.RequireSandbox || wrapped {
		return false
	}
	// The three-cell decision projects from its single home (Specification —
	// sandbox.ConfinementSatisfied); preflight's host-capabilities check
	// displays the same predicate, so the two can no longer diverge (the
	// 2026-09-01 nested-HALT divergence class).
	ok, optOut, reason := sandbox.ConfinementSatisfied(
		sandbox.DetectNested(depEnvGetter(deps)),
		strings.TrimSpace(deps.Env[envSandboxMode]))
	if ok {
		if optOut && deps.Stderr != nil {
			fmt.Fprintf(deps.Stderr, "[bridge] WARN: EVOLVE_SANDBOX=off — host opt-out honoured; phase %q runs UNCONFINED despite its sandbox requirement\n", cfg.Agent)
		}
		return false
	}
	if deps.Stderr != nil {
		fmt.Fprintf(deps.Stderr, "[bridge] sandbox requirement unsatisfied: %s\n", reason)
	}
	return true
}

// retrospectiveLessonPath grants only the role's documented main-repository
// lesson directory. Resolve existing ancestors too: a not-yet-created leaf
// below a retargeted instincts directory must not broaden the grant.
func retrospectiveLessonPath(root string) (string, error) {
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("absolute project root required")
	}
	canonicalRoot, err := canonicalSandboxPath(root)
	if err != nil {
		return "", err
	}
	expected := filepath.Join(canonicalRoot, ".evolve", "instincts", "lessons")
	actual, err := canonicalSandboxPath(expected)
	if err != nil {
		return "", err
	}
	if actual != expected {
		return "", fmt.Errorf("lesson directory resolves outside its declared scope")
	}
	return actual, nil
}
