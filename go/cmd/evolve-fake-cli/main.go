// evolve-fake-cli is a stand-in for the claude/codex/agy binaries used
// in offline E2E tests. The agent-bridge driver scripts invoke it via
// the BRIDGE_*_BINARY test seam when BRIDGE_TESTING=1.
//
// Behaviour:
//   - Parses the three real invocation styles (-p prompt, exec
//     --output-last-message <path>, --dangerously-skip-permissions).
//   - Detects which evolve-loop phase is being played from the artifact
//     filename (scout-report.md → scout, audit-report.md → audit, …).
//   - Writes the phase's expected artifact(s) — audit emits two
//     (audit-report.md + acs-verdict.json), retro emits two
//     (retrospective.md + failure-lesson-*.yaml).
//   - Exits 0 on the happy path; non-zero on contract failures the test
//     should surface (missing artifact path, unknown phase).
//
// Reusable as a generic fake-bridge backend for any Go test that needs
// to drive the in-process evolve orchestrator without touching a real
// LLM.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Invocation is the post-parse shape: enough to write the right
// artifact, regardless of which CLI style was used.
type Invocation struct {
	ArtifactPath string
	Phase        string // resolved from agent heading or artifact filename
	Prompt       string
	VersionOnly  bool
	// Style is the CLI family inferred from the invocation flags:
	// "claude" (-p), "codex" (exec + stdin), or "agy" (-p +
	// --dangerously-skip-permissions). Drives per-CLI exit injection so
	// a fallback test can make the primary CLI fail while the fallback
	// CLI — pointed at the same fake binary — succeeds.
	Style string
	// Interactive is true for a tmux/REPL launch: no -p prompt and no
	// codex `exec` subcommand. In that mode the fake serves a persistent
	// REPL (see repl.go) instead of doing a one-shot artifact write.
	Interactive bool
}

// phaseToBasename maps the canonical phase name to the filename the
// phase runner expects under the workspace dir. Authoritative.
var phaseToBasename = map[string]string{
	"intent": "intent.md",
	"scout":  "scout-report.md",
	"triage": "triage-report.md",
	// tdd's artifact is test-report.md (NOT the stale pre-rewrite
	// "team-context.md"); the tmux drivers poll this exact path, so a
	// mismatch makes every tdd phase time out (exit 81). See the rename
	// note in internal/phases/tdd/tdd.go.
	"tdd":   "test-report.md",
	"build": "build-report.md",
	"audit": "audit-report.md",
	"retro": "retrospective.md",
}

// agentHeadingToPhase maps the first-line heading of each agent .md
// file to the canonical phase name. The composed prompt body always
// includes this heading, so it's the most reliable phase signal
// regardless of which CLI invocation style fed the binary.
var agentHeadingToPhase = map[string]string{
	"# Evolve Intent":        "intent",
	"# Evolve Scout":         "scout",
	"# Evolve Triage":        "triage",
	"# Evolve TDD Engineer":  "tdd",
	"# Evolve Builder":       "build",
	"# Evolve Auditor":       "audit",
	"# Evolve Retrospective": "retro",
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run is the testable entrypoint. Parses args, writes artifacts, returns
// an exit code. Side-effects are limited to filesystem writes under the
// caller-supplied artifact path.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	inv, err := parseArgs(args, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "fake-cli: parse args: %v\n", err)
		return 2
	}
	if inv.VersionOnly {
		fmt.Fprintln(stdout, "fake-cli 0.1.0 (evolve-loop test stub)")
		return 0
	}
	// Interactive (tmux/REPL) launch — serve a persistent REPL. Detected
	// from argv alone (no env), because env vars do not reliably propagate
	// into a tmux session; the boot marker is a fixed constant the test
	// manifest pins as its prompt_marker.
	if inv.Interactive {
		return runREPL(stdin, stdout, stderr, auditVerdict())
	}
	// Per-CLI exit injection (headless path). Lets a fallback-chain test
	// fail the primary CLI (e.g. exit 81) while the fallback CLI, pointed
	// at this same fake, returns 0. Fires BEFORE any artifact write so the
	// run looks like a genuine failed invocation (no artifact produced).
	if code := injectedExitCode(inv.Style); code != 0 {
		fmt.Fprintf(stderr, "fake-cli: injected exit %d for style=%s\n", code, inv.Style)
		return code
	}
	if inv.ArtifactPath == "" {
		fmt.Fprintln(stderr, "fake-cli: artifact path missing — neither --output-last-message nor a path in -p prompt was found")
		return 3
	}
	phase := inv.Phase
	if phase == "" {
		phase = detectPhase(inv.ArtifactPath)
	}
	files, err := artifactsFor(phase, inv.ArtifactPath, auditVerdict())
	if err != nil {
		fmt.Fprintf(stderr, "fake-cli: artifactsFor(%s): %v\n", phase, err)
		return 4
	}
	if err := writeArtifacts(files); err != nil {
		fmt.Fprintf(stderr, "fake-cli: %v\n", err)
		return 5
	}
	fmt.Fprintf(stdout, "fake-cli: wrote %d artifact(s) for phase=%s\n", len(files), phase)
	return 0
}

// parseArgs handles three invocation styles. The bridge driver script
// chooses the style; we accept all three and normalise to a single
// Invocation shape.
func parseArgs(args []string, stdin io.Reader) (Invocation, error) {
	// Quick scans for early-exit flags.
	for _, a := range args {
		if a == "--version" {
			return Invocation{VersionOnly: true}, nil
		}
	}

	var (
		prompt        string
		outputLastMsg string
		isCodexExec   bool
		hasPromptFlag bool
		skipPerms     bool
	)

	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "-p" && i+1 < len(args):
			prompt = args[i+1]
			hasPromptFlag = true
			i += 2
		case a == "--output-last-message" && i+1 < len(args):
			outputLastMsg = args[i+1]
			i += 2
		case a == "exec":
			isCodexExec = true
			i++
		case a == "-m" && i+1 < len(args):
			// codex model flag; we don't use it but must consume the value.
			i += 2
		case a == "--model" && i+1 < len(args):
			i += 2
		case a == "--allowedTools":
			// claude consumes a single value here; consume the next non-flag.
			i++
			if i < len(args) && !strings.HasPrefix(args[i], "--") {
				i++
			}
		case a == "--dangerously-skip-permissions":
			skipPerms = true
			i++
		case strings.HasPrefix(a, "--"):
			// Unknown flag with `=value` form, or boolean. Best-effort consume.
			i++
		default:
			i++
		}
	}

	// Codex pipes the prompt on stdin.
	if isCodexExec && stdin != nil {
		buf, err := io.ReadAll(stdin)
		if err != nil {
			return Invocation{}, fmt.Errorf("read stdin: %w", err)
		}
		prompt = string(buf)
	}

	// Phase resolution: heading-from-prompt is authoritative because the
	// composed prompt body always begins with the agent heading. Filename-
	// based detection is left to run() as a fallback so we don't leak
	// the "unknown" sentinel into the parse layer.
	phase := detectPhaseFromPrompt(prompt)

	artifactPath := outputLastMsg
	if artifactPath == "" {
		artifactPath = resolveArtifactPath(prompt, phase)
	}

	// Infer CLI family for per-CLI exit injection: codex `exec`, then agy
	// (-p + skip-permissions), else claude (-p). agy is checked before the
	// plain -p case because agy also passes -p.
	style := "claude"
	switch {
	case isCodexExec:
		style = "codex"
	case skipPerms:
		style = "agy"
	}

	// A tmux/REPL launch carries neither -p nor codex `exec`. (--version is
	// handled by the early-exit scan above and never reaches here.)
	interactive := !hasPromptFlag && !isCodexExec

	return Invocation{
		ArtifactPath: artifactPath,
		Phase:        phase,
		Prompt:       prompt,
		Style:        style,
		Interactive:  interactive,
	}, nil
}

// detectPhaseFromPrompt scans the prompt for the agent heading line.
// Returns "" if no known heading is present.
func detectPhaseFromPrompt(prompt string) string {
	for heading, phase := range agentHeadingToPhase {
		if strings.Contains(prompt, heading) {
			return phase
		}
	}
	return ""
}

// resolveArtifactPath builds workspace + per-phase basename when the
// prompt has a Cycle Context workspace line. Falls back to whatever
// absolute artifact-path mention exists in the prompt (production path
// with $ARTIFACT_PATH substituted).
func resolveArtifactPath(prompt, phase string) string {
	if phase != "" && phase != "unknown" {
		if base, ok := phaseToBasename[phase]; ok {
			if m := workspaceLineRE.FindStringSubmatch(prompt); len(m) >= 2 {
				return filepath.Join(m[1], base)
			}
		}
	}
	return absPathRE.FindString(prompt)
}

// absPathRE matches absolute paths to known artifact basenames. The
// dir prefix MUST be non-empty so a relative mention like
// "workspace/scout-report.md" in the agent body doesn't accidentally
// match as "/scout-report.md" — that quirk of greedy backtracking bit
// us early in development.
var absPathRE = regexp.MustCompile(
	`/[A-Za-z0-9._-][A-Za-z0-9._/-]*/(?:intent\.md|intent-delta\.md|scout-report\.md|triage-report\.md|test-report\.md|build-report\.md|audit-report\.md|retrospective\.md)\b`,
)

// workspaceLineRE matches the "- workspace: /path" line that
// composePrompt appends to every phase prompt's Cycle Context section.
var workspaceLineRE = regexp.MustCompile(`(?m)^[-*]\s*workspace:\s*(\S+)\s*$`)

// detectPhase classifies an artifact path by its basename. Each phase
// has a unique filename in evolve-loop's contract.
func detectPhase(artifactPath string) string {
	base := filepath.Base(artifactPath)
	switch base {
	case "intent.md", "intent-delta.md":
		return "intent"
	case "scout-report.md":
		return "scout"
	case "triage-report.md":
		return "triage"
	case "test-report.md":
		return "tdd"
	case "build-report.md":
		return "build"
	case "audit-report.md":
		return "audit"
	case "retrospective.md":
		return "retro"
	default:
		return "unknown"
	}
}

// artifactsFor returns the file map a phase must emit to satisfy its
// classifier. Audit emits TWO files (the report + acs-verdict.json next
// to it); retro emits TWO (report + failure-lesson YAML).
//
// verdict ∈ {PASS,WARN,FAIL} steers ONLY the audit phase: it shapes both
// the audit-report.md verdict line and the fused acs-verdict.json red_count
// (FAIL → red_count 1, so the EGPS gate blocks). Other phases ignore it.
// An empty/unknown verdict is treated as PASS by the caller (auditVerdict).
func artifactsFor(phase, mainPath, verdict string) (map[string]string, error) {
	out := map[string]string{}
	switch phase {
	case "intent":
		out[mainPath] = "goal: synthetic e2e\nacceptance_checks:\n  - cycle completes\n"
	case "scout":
		out[mainPath] = "# Scout Report\n\n## Proposed Tasks\n- task-1: synthetic\n- task-2: also synthetic\n"
	case "triage":
		out[mainPath] = "# Triage\n\n## top_n\n- task-1\n\n## deferred\n- task-2\n"
	case "tdd":
		out[mainPath] = "# Team Context\n\n## Acceptance\n- cycle ships clean\n\n## RED Tests\n- tests/synthetic_test.go\n"
	case "build":
		out[mainPath] = "# Build Report\n\n## Files Modified\n- file.go (synthetic)\n"
	case "audit":
		redCount := 0
		if verdict == "FAIL" {
			redCount = 1
		}
		// Emit BOTH the prose heading and the machine-readable sentinel: at
		// EVOLVE_PHASE_IO=enforce (the default since the 3.10 cutover) the sentinel
		// is mandatory for the audit verdict parse, so a prose-only fake report
		// would fail audit and the happy-path pipeline would never reach ship.
		out[mainPath] = fmt.Sprintf("# Audit Report\n\n## Verdict\n**%s**\n\nSynthetic %s verdict.\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"%s\",\"schema_version\":1} -->\n", verdict, verdict, verdict)
		acsPath := filepath.Join(filepath.Dir(mainPath), "acs-verdict.json")
		out[acsPath] = fmt.Sprintf(`{"red_count": %d, "yellow_count": 0, "green_count": 1}`, redCount) + "\n"
	case "retro":
		out[mainPath] = "# Retrospective\n\n## Lessons\n- synthetic lesson learned\n"
		lessonPath := filepath.Join(filepath.Dir(mainPath), "failure-lesson-1.yaml")
		out[lessonPath] = "id: synthetic-lesson-1\nseverity: low\nsummary: synthetic e2e lesson\n"
	default:
		return nil, fmt.Errorf("unknown phase %q (no known artifact contract for %s)", phase, mainPath)
	}
	return out, nil
}

// auditVerdict reads FAKE_CLI_AUDIT_VERDICT and normalises it to one of
// {PASS,WARN,FAIL}; anything else (including unset) is PASS. Read from the
// process env, so it only takes effect on the headless path where env
// propagates to the fake subprocess (tmux launches default to PASS).
func auditVerdict() string {
	switch v := strings.ToUpper(strings.TrimSpace(os.Getenv("FAKE_CLI_AUDIT_VERDICT"))); v {
	case "WARN", "FAIL":
		return v
	default:
		return "PASS"
	}
}

// injectedExitCode returns the exit code a test wants this CLI family to
// fail with, via FAKE_CLI_{CLAUDE,CODEX,AGY}_EXIT. 0 (or unset/garbage)
// means "no injection — behave normally".
func injectedExitCode(style string) int {
	raw := os.Getenv("FAKE_CLI_" + strings.ToUpper(style) + "_EXIT")
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 0 {
		return 0
	}
	return n
}
