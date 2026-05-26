package bridge

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// LaunchArgs is the argv-faithful launch entry point: it parses the same
// flag surface as `tools/agent-bridge/bin/bridge launch` (with BRIDGE_*
// env fallbacks; flags win), runs the launch pipeline, and returns a
// bridge exit code (one of the Exit* constants). It is what the
// `evolve bridge launch` CLI shim calls, and the surface the BATS parity
// tests target.
//
// Pipeline (Template Method): parse → load profile → validate required →
// prompt guards → resolve effective config (model/permission-mode/
// stream-output/session-name precedence: flag > env > profile) →
// dispatch to the registered Driver. Launch (the core.Bridge entry)
// shares this by mapping a core.BridgeRequest onto the same Config.
//
// Not yet ported (each lands with its own test slice): --validate-only,
// --dry-run, --require-full tier check, stale-workspace WARN, tmux
// orphan-sweep, --json report, and the per-driver credential-isolation
// guards (those live in the drivers).
func (e *Engine) LaunchArgs(ctx context.Context, args []string, env map[string]string, stdout, stderr io.Writer) int {
	raw, err := parseLaunchArgs(args, env)
	if err != nil {
		fmt.Fprintf(stderr, "[bridge] launch: %v\n", err)
		return ExitBadFlags
	}

	// Required-field validation (after flag-vs-env merge), mirroring
	// bin/bridge cmd_launch's `missing` accumulation.
	missing := raw.missingRequired()
	if len(missing) > 0 {
		fmt.Fprintf(stderr, "[bridge] launch: missing required (flag or env): %s\n", strings.Join(missing, " "))
		return ExitBadFlags
	}

	prof, err := LoadProfile(raw.profile)
	if err != nil {
		fmt.Fprintf(stderr, "[bridge] %v\n", err)
		return ExitBadFlags
	}

	// Prompt guards: readable + non-empty (an empty prompt would hang the
	// agent at the artifact timeout — fail fast, bin/bridge F5).
	promptData, err := os.ReadFile(raw.promptFile)
	if err != nil {
		fmt.Fprintf(stderr, "[bridge] launch: prompt file not readable: %s\n", raw.promptFile)
		return ExitBadFlags
	}
	if len(promptData) == 0 {
		fmt.Fprintf(stderr, "[bridge] launch: prompt file is empty: %s\n", raw.promptFile)
		fmt.Fprintln(stderr, "[bridge] (an empty prompt would hang the agent at the artifact timeout)")
		return ExitBadFlags
	}

	// --human-input two-gate: the per-invocation flag also requires the
	// BRIDGE_HUMAN_SIMULATION=1 host opt-in (the keystroke-plausibility
	// layer is double-gated OFF by default).
	if raw.humanInput {
		if v, _ := lookupEnv(e.deps, "BRIDGE_HUMAN_SIMULATION"); v != "1" {
			fmt.Fprintln(stderr, "[bridge] --human-input requires BRIDGE_HUMAN_SIMULATION=1 host opt-in")
			return ExitSafetyGate
		}
	}

	// Resolve effective model: "auto" defers to the profile's model when set.
	effectiveModel := raw.model
	if effectiveModel == "auto" && prof.Model != "" {
		effectiveModel = prof.Model
	}

	// Resolve effective permission mode: flag/env > profile. Re-validate
	// here because an operator-passed flag bypasses LoadProfile's check.
	permMode := raw.permissionMode
	if permMode == "" {
		permMode = prof.PermissionMode
	}
	if !validPermissionModes[permMode] {
		fmt.Fprintf(stderr, "[bridge] invalid --permission-mode value: '%s'\n", permMode)
		fmt.Fprintln(stderr, "[bridge] valid: plan, default, acceptEdits, bypassPermissions, auto, dontAsk")
		return ExitBadFlags
	}

	// Resolve effective stream-output: flag/env > profile > false.
	streamOut := prof.StreamOutput
	switch raw.streamOutput {
	case "":
		// honor profile default
	case "true", "1", "yes", "on":
		streamOut = true
	case "false", "0", "no", "off":
		streamOut = false
	default:
		fmt.Fprintf(stderr, "[bridge] invalid --stream-output value: '%s' — must be boolean\n", raw.streamOutput)
		return ExitBadFlags
	}

	// Resolve effective session name: flag/env > profile.
	sessionName := raw.sessionName
	if sessionName == "" {
		sessionName = prof.SessionName
	}
	if sessionName != "" {
		if len(sessionName) > 32 || !sessionNameRE.MatchString(sessionName) {
			fmt.Fprintf(stderr, "[bridge] invalid --session-name '%s' — must match [a-zA-Z0-9._-]+ (max 32)\n", sessionName)
			return ExitBadFlags
		}
	}

	cycle := 0
	if raw.cycle != "" {
		cycle, _ = strconv.Atoi(raw.cycle) // non-numeric → 0, matching bash's permissive default
	}

	// Realize the launch intent against this CLI's manifest (ADR-0022). The
	// *-tmux drivers build their launch command from this rather than
	// constructing model/permission flags inline, so a claude-origin profile's
	// raw flags realize only for the matching CLI (RawByCLI[agy/codex] = nil).
	// permMode is still carried on Config for the safety gates and the headless
	// claude-p driver; the realizer is what the tmux launch command consumes.
	sessionMode := "ephemeral"
	if sessionName != "" {
		sessionMode = "named:" + sessionName
	}
	intent := LaunchIntent{
		ModelTier:   effectiveModel,
		Permission:  permissionIntent(permMode),
		SessionMode: sessionMode,
		RawByCLI:    prof.ExtraFlagsByCLI,
	}

	cfg := Config{
		CLI:            raw.cli,
		Profile:        raw.profile,
		Model:          effectiveModel,
		PromptFile:     raw.promptFile,
		Workspace:      raw.workspace,
		StdoutLog:      raw.stdoutLog,
		StderrLog:      raw.stderrLog,
		Artifact:       raw.artifact,
		Cycle:          cycle,
		Worktree:       raw.worktree,
		Agent:          raw.agent,
		PermissionMode: permMode,
		StreamOutput:   streamOut,
		SessionName:    sessionName,
		AllowBypass:    raw.allowBypass,
		HumanInput:     raw.humanInput,
		RequireFull:    raw.requireFull,
		AllowedTools:   prof.AllowedTools,
		ExtraFlags:     raw.extra,
		Realization:    RealizeFor(raw.cli, intent),
	}

	// Non-dispatch modes (bin/bridge order: validate-only → dry-run →
	// require-full → driver dispatch).
	if raw.validateOnly {
		printResolvedConfig(stdout, &cfg, prof)
		return ExitOK
	}
	if raw.dryRun {
		return e.runDryRun(&cfg, stdout, stderr)
	}
	if raw.requireFull {
		if rc, blocked := e.requireFullCheck(&cfg, stderr); blocked {
			return rc
		}
	}

	driver, ok := LookupDriver(cfg.CLI)
	if !ok {
		fmt.Fprintf(stderr, "[bridge] no driver for cli=%s\n", cfg.CLI)
		return ExitBadFlags
	}

	// Thread the caller's diagnostic streams into a per-call Deps copy so
	// drivers emit their `[driver] ...` notes to the bridge's stderr.
	d := e.deps
	d.Stdout = stdout
	d.Stderr = stderr

	rc, err := driver.Launch(ctx, &cfg, d)
	if err != nil {
		fmt.Fprintf(stderr, "[bridge] %v\n", err)
		if rc == 0 {
			rc = ExitBadFlags
		}
	}
	return rc
}

// rawLaunch holds the parsed launch flags before profile-aware
// resolution. Mirrors the local variables in bin/bridge cmd_launch.
type rawLaunch struct {
	cli, profile, model, promptFile, workspace, stdoutLog, stderrLog, artifact string
	cycle, worktree, agent                                                     string
	permissionMode, sessionName, streamOutput                                  string
	validateOnly, dryRun, requireFull, allowBypass, humanInput                 bool
	extra                                                                      []string // args after `--`, forwarded to the inner CLI
}

// missingRequired returns the names of unset required fields, in the
// same flag/ENV label form bin/bridge prints.
func (r rawLaunch) missingRequired() []string {
	var m []string
	check := func(v, label string) {
		if v == "" {
			m = append(m, label)
		}
	}
	check(r.cli, "--cli/BRIDGE_CLI")
	check(r.profile, "--profile/PROFILE_PATH")
	check(r.model, "--model/RESOLVED_MODEL")
	check(r.promptFile, "--prompt-file/PROMPT_FILE")
	check(r.workspace, "--workspace/WORKSPACE_PATH")
	check(r.stdoutLog, "--stdout-log/STDOUT_LOG")
	check(r.stderrLog, "--stderr-log/STDERR_LOG")
	check(r.artifact, "--artifact/ARTIFACT_PATH")
	return m
}

// parseLaunchArgs initializes raw fields from env fallbacks then applies
// flags (flags win). Supports both --key=value and --key value forms,
// the boolean toggles, and the `--` inner-CLI passthrough separator —
// matching bin/bridge cmd_launch's getopts loop.
func parseLaunchArgs(args []string, env map[string]string) (rawLaunch, error) {
	get := func(k string) string {
		if env == nil {
			return ""
		}
		return env[k]
	}
	truthy := func(v string) bool {
		switch v {
		case "1", "true", "yes", "on":
			return true
		}
		return false
	}

	r := rawLaunch{
		cli:            get("BRIDGE_CLI"),
		profile:        get("PROFILE_PATH"),
		model:          get("RESOLVED_MODEL"),
		promptFile:     get("PROMPT_FILE"),
		workspace:      get("WORKSPACE_PATH"),
		stdoutLog:      get("STDOUT_LOG"),
		stderrLog:      get("STDERR_LOG"),
		artifact:       get("ARTIFACT_PATH"),
		cycle:          get("CYCLE"),
		worktree:       get("WORKTREE_PATH"),
		agent:          get("AGENT"),
		permissionMode: get("BRIDGE_PERMISSION_MODE"),
		sessionName:    get("BRIDGE_SESSION_NAME"),
		streamOutput:   get("BRIDGE_STREAM_OUTPUT"),
		requireFull:    truthy(get("BRIDGE_REQUIRE_FULL")),
		allowBypass:    truthy(get("BRIDGE_ALLOW_BYPASS")),
		dryRun:         truthy(get("BRIDGE_DRY_RUN")),
		validateOnly:   truthy(get("VALIDATE_ONLY")),
		humanInput:     truthy(get("BRIDGE_HUMAN_INPUT_FLAG")),
	}

	// stringDest maps a flag name to the field it sets.
	stringDest := map[string]*string{
		"--cli": &r.cli, "--profile": &r.profile, "--model": &r.model,
		"--prompt-file": &r.promptFile, "--workspace": &r.workspace,
		"--stdout-log": &r.stdoutLog, "--stderr-log": &r.stderrLog,
		"--artifact": &r.artifact, "--cycle": &r.cycle, "--worktree": &r.worktree,
		"--agent": &r.agent, "--permission-mode": &r.permissionMode,
		"--session-name": &r.sessionName,
	}

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			r.extra = append(r.extra, args[i+1:]...)
			return r, nil
		case a == "--validate-only":
			r.validateOnly = true
		case a == "--dry-run":
			r.dryRun = true
		case a == "--require-full":
			r.requireFull = true
		case a == "--allow-bypass":
			r.allowBypass = true
		case a == "--human-input":
			r.humanInput = true
		case a == "--stream-output":
			r.streamOutput = "true"
		case a == "--no-stream-output":
			r.streamOutput = "false"
		case strings.HasPrefix(a, "--") && strings.Contains(a, "="):
			key, val, _ := strings.Cut(a, "=")
			dst, ok := stringDest[key]
			if !ok {
				return r, fmt.Errorf("unknown flag: %s", key)
			}
			*dst = val
		case strings.HasPrefix(a, "--"):
			dst, ok := stringDest[a]
			if !ok {
				return r, fmt.Errorf("unknown flag: %s", a)
			}
			if i+1 >= len(args) {
				return r, fmt.Errorf("%s requires a value", a)
			}
			*dst = args[i+1]
			i++
		default:
			return r, fmt.Errorf("unexpected argument: %s", a)
		}
	}
	return r, nil
}
