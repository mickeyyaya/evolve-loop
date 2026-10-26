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

func defaultSandboxWrap(deps Deps) SandboxWrapper {
	return defaultSandboxWrapWithProbe(deps, sandbox.Probe)
}

func defaultSandboxWrapWithProbe(deps Deps, probeFunc func() sandbox.ProbeResult) SandboxWrapper {
	return func(req SandboxWrapRequest) ([]string, bool) {
		mode := strings.TrimSpace(deps.Env[envSandboxMode])
		if mode == "" {
			mode = config.SandboxModeAuto
		}
		switch mode {
		case config.SandboxModeOff, config.SandboxModeOn, config.SandboxModeAuto:
		default:
			if deps.Stderr != nil {
				fmt.Fprintf(deps.Stderr, "[bridge] WARN: EVOLVE_SANDBOX=%q unrecognized (want auto|on|off); treating as auto\n", mode)
			}
			mode = config.SandboxModeAuto
		}
		if mode == config.SandboxModeOff {
			return nil, false
		}

		probe := probeFunc()
		wrap, reason := sandbox.ShouldWrap(sandbox.DetectNested(depEnvGetter(deps)), probe)
		if !wrap {
			if mode == config.SandboxModeOn && deps.Stderr != nil {
				fmt.Fprintf(deps.Stderr, "[bridge] WARN: EVOLVE_SANDBOX=on but inner sandbox not applied; phase runs UNCONFINED at the inner layer. Reason: %s\n", reason)
			}
			return nil, false
		}

		if probe.OS == "darwin" && req.TerminalPath != "" {
			if err := validateSandboxTerminal(req.TerminalPath); err != nil {
				if deps.Stderr != nil {
					fmt.Fprintf(deps.Stderr, "[bridge] sandbox terminal unavailable: %v\n", err)
				}
				return nil, false
			}
		}

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
		cred := deps.dispatchCredentialDeny(deps.execStage())
		cfg.DenyReadPaths = append(slices.Clone(cfg.DenyReadPaths), cred.read...)
		cfg.DenyPaths = append(slices.Clone(cfg.DenyPaths), cred.write...)
		cfg.ExecDenyPaths, cfg.ExecDenyRoots, cfg.ExecDenyEnforce = deps.execDeny(req.CLI, probe.OS)
		cfg.ReadIsolation = deps.readIsolation(req, home)
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
		grants, err := resolveSandboxWriteGrants(req.WriteSubpaths, req.RepoRoot, req.Worktree, req.Workspace)
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
		cfg.DenySockets, cfg.DenyLiterals = sockets, append(append([]string{socketDir}, sockets...), cred.literals...)

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

const envSandboxMode = "EVOLVE_SANDBOX"

const modelNetworkRequired = true

func sandboxPrefixForLaunch(deps Deps, cfg *Config, terminalPath string) ([]string, bool) {
	if cfg == nil || cfg.Worktree == "" && !cfg.RequireSandbox {
		return nil, false
	}
	if deps.SandboxWrap == nil {
		return nil, false
	}
	if !cfg.AllowNetwork && deps.Stderr != nil {
		fmt.Fprintf(deps.Stderr, "[bridge] WARN: source-writing phase %q has sandbox.allow_network=false; forcing true (a sandboxed model-reaching CLI cannot boot with network denied)\n", cfg.Agent)
	}
	return deps.SandboxWrap(SandboxWrapRequest{
		CLI:           cfg.CLI,
		Tag:           cfg.DispatchID,
		TerminalPath:  terminalPath,
		Phase:         cfg.Agent,
		Workspace:     cfg.Workspace,
		Worktree:      cfg.Worktree,
		RepoRoot:      cfg.ProjectRoot,
		AllowNetwork:  modelNetworkRequired,
		DenyPaths:     cfg.DenyPaths,
		DenyReadPaths: cfg.DenyReadPaths,
		WriteSubpaths: cfg.SandboxWriteSubpaths,
	})
}

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

func joinPrefixForTmux(prefix []string) string {
	parts := make([]string, len(prefix))
	for i, p := range prefix {
		parts[i] = shellQuotePOSIX(p)
	}
	return strings.Join(parts, " ")
}

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

func sandboxRequiredButUnavailable(deps Deps, cfg *Config, wrapped bool) bool {
	if cfg == nil || !cfg.RequireSandbox || wrapped {
		return false
	}
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
