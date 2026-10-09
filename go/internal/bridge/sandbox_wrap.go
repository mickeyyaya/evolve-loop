package bridge

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
		// Normalize any unrecognized value to auto, mirroring config.applyEnv's
		// validation contract, and WARN so the fallback is observable.
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

		// HomeDir is read-allowed so tmux CLIs can load their own config/auth
		// state; repo writes remain confined below.
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
		// The profile's declared write surface (sandbox.write_subpaths) is
		// resolved with the same retarget defense; a grant that cannot be
		// resolved safely refuses the launch rather than confining it wrong.
		grants, err := resolveSandboxWriteGrants(req.WriteSubpaths, req.RepoRoot, req.Worktree)
		if err != nil {
			if deps.Stderr != nil {
				fmt.Fprintf(deps.Stderr, "[bridge] sandbox write grant unavailable: %v\n", err)
			}
			return nil, false
		}
		cfg.WritePaths = append(cfg.WritePaths, grants...)
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
		socketDir, sockets, err := tmuxSocketGuard()
		if err != nil {
			if deps.Stderr != nil {
				fmt.Fprintf(deps.Stderr, "[bridge] sandbox tmux socket guard unavailable: %v\n", err)
			}
			return nil, false
		}
		cfg.DenySockets, cfg.DenyLiterals = sockets, append([]string{socketDir}, sockets...)

		return osSandboxPrefix(probe.OS, deps, req, cfg)
	}
}

func osSandboxPrefix(goos string, deps Deps, req SandboxWrapRequest, cfg sandbox.Config) ([]string, bool) {
	switch goos {
	case "darwin":
		sbpl := sandbox.GenerateSBPL(cfg)
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
		return append([]string{"bwrap"}, sandbox.BwrapPrefix(cfg)...), true
	default:
		return nil, false
	}
}

// sandboxWritePaths returns the FLOOR of the write-allowlist for a sandboxed
// phase: worktree (source) + workspace (artifacts) + /tmp (scratch). It is
// the orchestrator's designation, not the profile's — sandbox.write_subpaths
// can only ADD to it. The whole worktree stays writable at the OS layer
// regardless of what a profile declares; any narrower enforcement is the
// tool-layer hooks' job. An empty req.Worktree means the orchestrator
// designated none, so only the workspace is returned.
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
// Env map first, then the LookupEnv seam.
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
	// Every driver that reaches this wrapper needs the network to reach its
	// model API, and "allow the model host, deny the rest" is not expressible —
	// SBPL/bwrap cannot filter per host, and proxying the model endpoint breaks
	// subscription billing. Force network true structurally: the real
	// confinement boundary here is filesystem, not network.
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
		WriteSubpaths: cfg.SandboxWriteSubpaths,
	})
}

// shellQuotePOSIX wraps s in POSIX single quotes, escaping any embedded
// single quotes. Used to splice the sandbox prefix into the *-tmux driver's
// launchCmd string (SendKeys gets a single shell line, not an argv slice).
//
// The safe character set is intentionally narrow (a-z, A-Z, 0-9, and
// - _ / . , + : @ %); anything else triggers quoting, so an unrecognized
// character is quoted by default rather than passed through.
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
	// The three-cell decision projects from its single home
	// (sandbox.ConfinementSatisfied); preflight's host-capabilities check
	// displays the same predicate, so the two can no longer diverge.
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
