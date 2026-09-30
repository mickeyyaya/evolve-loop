// evolve-fake-cli stands in for the claude, codex and agy binaries in offline
// E2E tests, writing each phase's expected artifacts without a real LLM.
// See docs/architecture/packages/cmd-evolve-fake-cli.md.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

type Invocation struct {
	ArtifactPath string
	Phase        string
	Prompt       string
	VersionOnly  bool
	Style        string
	Interactive  bool
}

var phaseToBasename = map[string]string{
	"intent": "intent.md",
	"scout":  "scout-report.md",
	"triage": "triage-report.md",
	"tdd":    "test-report.md",
	"build":  "build-report.md",
	"audit":  "audit-report.md",
	"retro":  "retrospective.md",
}

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
	if inv.Interactive {
		return runREPL(stdin, stdout, stderr, auditVerdict())
	}
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

func parseArgs(args []string, stdin io.Reader) (Invocation, error) {
	for _, a := range args {
		if a == "--version" {
			return Invocation{VersionOnly: true}, nil
		}
	}

	flags := scanFlags(args)
	prompt := flags.prompt

	if flags.isCodexExec && stdin != nil {
		buf, err := io.ReadAll(stdin)
		if err != nil {
			return Invocation{}, fmt.Errorf("read stdin: %w", err)
		}
		prompt = string(buf)
	}

	phase := detectPhaseFromPrompt(prompt)

	artifactPath := flags.outputLastMsg
	if artifactPath == "" {
		artifactPath = resolveArtifactPath(prompt, phase)
	}

	interactive := !flags.hasPromptFlag && !flags.isCodexExec

	return Invocation{
		ArtifactPath: artifactPath,
		Phase:        phase,
		Prompt:       prompt,
		Style:        flags.style(),
		Interactive:  interactive,
	}, nil
}

type argFlags struct {
	prompt        string
	outputLastMsg string
	isCodexExec   bool
	hasPromptFlag bool
	skipPerms     bool
}

func scanFlags(args []string) argFlags {
	var f argFlags
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "-p" && i+1 < len(args):
			f.prompt = args[i+1]
			f.hasPromptFlag = true
			i += 2
		case a == "--output-last-message" && i+1 < len(args):
			f.outputLastMsg = args[i+1]
			i += 2
		case a == "exec":
			f.isCodexExec = true
			i++
		case (a == "-m" || a == "--model") && i+1 < len(args):
			i += 2
		case a == "--allowedTools":
			i++
			if i < len(args) && !strings.HasPrefix(args[i], "--") {
				i++
			}
		case a == "--dangerously-skip-permissions":
			f.skipPerms = true
			i++
		default:
			i++
		}
	}
	return f
}

func (f argFlags) style() string {
	switch {
	case f.isCodexExec:
		return "codex"
	case f.skipPerms:
		return "agy"
	}
	return "claude"
}

func detectPhaseFromPrompt(prompt string) string {
	for heading, phase := range agentHeadingToPhase {
		if strings.Contains(prompt, heading) {
			return phase
		}
	}
	return ""
}

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

var absPathRE = regexp.MustCompile(
	`/[A-Za-z0-9._-][A-Za-z0-9._/-]*/(?:intent\.md|intent-delta\.md|scout-report\.md|triage-report\.md|test-report\.md|build-report\.md|audit-report\.md|retrospective\.md)\b`,
)

var workspaceLineRE = regexp.MustCompile(`(?m)^[-*]\s*workspace:\s*(\S+)\s*$`)

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

func artifactsFor(phase, mainPath, verdict string) (map[string]string, error) {
	out := map[string]string{}
	token := challengeTokenLine(mainPath)

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
		out[mainPath] = "# Build Report\n\n## Files Modified\n- file.go (synthetic)\n\n" +
			explanationdocs.RenderNotApplicableDeclaration("synthetic e2e build; the base-bound diff contains no material changes")
	case "audit":
		out = auditArtifacts(mainPath, verdict)
	case "retro":
		out[mainPath] = "# Retrospective\n\n## Lessons\n- synthetic lesson learned\n"
		lessonPath := filepath.Join(filepath.Dir(mainPath), "failure-lesson-1.yaml")
		out[lessonPath] = "id: synthetic-lesson-1\nseverity: low\nsummary: synthetic e2e lesson\n"
	default:
		return nil, fmt.Errorf("unknown phase %q (no known artifact contract for %s)", phase, mainPath)
	}
	if token != "" && out[mainPath] != "" {
		out[mainPath] = token + out[mainPath]
	}
	return out, nil
}

func challengeTokenLine(mainPath string) string {
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(mainPath), "challenge-token.txt"))
	if err != nil {
		return ""
	}
	if t := strings.TrimSpace(string(raw)); t != "" {
		return "<!-- challenge-token: " + t + " -->\n"
	}
	return ""
}

func auditArtifacts(mainPath, verdict string) map[string]string {
	redCount := 0
	if verdict == "FAIL" {
		redCount = 1
	}
	var failure *phasecontract.FailureBlock
	if verdict == "WARN" || verdict == "FAIL" {
		failure = &phasecontract.FailureBlock{
			Class:         "code-audit-fail",
			Defects:       []string{"synthetic e2e " + verdict + " finding"},
			EvidencePaths: []string{"audit-report.md"},
		}
	}
	out := map[string]string{}
	out[mainPath] = fmt.Sprintf("# Audit Report\n\n## Verdict\n**%s**\n\nSynthetic %s verdict.\n"+
		"## Explanation Documentation\n- Status: VERIFIED\n- Build status: not_applicable\n"+
		"- Evidence: reviewed the NOT_APPLICABLE declaration in build-report.md against the empty base-bound diff\n"+
		"%s\n", verdict, verdict, phasecontract.RenderVerdictSentinelWithFailure("audit", verdict, failure))
	acsPath := filepath.Join(filepath.Dir(mainPath), "acs-verdict.json")
	out[acsPath] = fmt.Sprintf(`{"schema_version":"1.0","verdict":%q,"ship_eligible":%t,"red_count":%d,"yellow_count":0,"green_count":1}`,
		verdict, verdict != "FAIL", redCount) + "\n"
	return out
}

func auditVerdict() string {
	switch v := strings.ToUpper(strings.TrimSpace(os.Getenv("FAKE_CLI_AUDIT_VERDICT"))); v {
	case "WARN", "FAIL":
		return v
	default:
		return "PASS"
	}
}

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
