package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// parseArgs must handle three real-world invocation styles:
//  1. claude-p:  fake -p "<prompt>" --model M --allowedTools Bash,Edit
//  2. agy:       fake -p "<prompt>" --dangerously-skip-permissions --model M
//  3. codex:     echo "<prompt>" | fake exec --output-last-message <path>
//
// Output: an Invocation carrying ArtifactPath (resolved from the right
// source per style) and Prompt (read from -p or stdin).
// When the agent prompt does NOT contain an absolute artifact path
// (production agents like evolve-scout.md only mention the filename),
// fake-cli must derive workspace+basename from the Cycle Context.
func TestParseArgs_ClaudeStyle_WorkspaceFallback(t *testing.T) {
	prompt := strings.Join([]string{
		"# Evolve Scout",
		"You will write scout-report.md.",
		"",
		"## Cycle Context",
		"- cycle: 1",
		"- workspace: /var/folders/x/cycle-1",
		"- goal_hash: g",
	}, "\n")
	args := []string{"-p", prompt, "--model", "auto"}
	inv, err := parseArgs(args, bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	want := "/var/folders/x/cycle-1/scout-report.md"
	if inv.ArtifactPath != want {
		t.Errorf("ArtifactPath=%q, want %q", inv.ArtifactPath, want)
	}
}

func TestParseArgs_ClaudeStyle(t *testing.T) {
	args := []string{"-p", "write to /tmp/ws/scout-report.md please", "--model", "sonnet", "--allowedTools", "Bash"}
	inv, err := parseArgs(args, bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if inv.Prompt == "" || !strings.Contains(inv.Prompt, "scout-report.md") {
		t.Errorf("Prompt=%q, want substring scout-report.md", inv.Prompt)
	}
	if inv.ArtifactPath != "/tmp/ws/scout-report.md" {
		t.Errorf("ArtifactPath=%q, want /tmp/ws/scout-report.md", inv.ArtifactPath)
	}
}

func TestParseArgs_AgyStyle(t *testing.T) {
	args := []string{"-p", "see /tmp/ws/intent.md", "--dangerously-skip-permissions"}
	inv, err := parseArgs(args, bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if inv.ArtifactPath != "/tmp/ws/intent.md" {
		t.Errorf("ArtifactPath=%q, want /tmp/ws/intent.md", inv.ArtifactPath)
	}
}

func TestParseArgs_CodexStyle(t *testing.T) {
	stdin := bytes.NewReader([]byte("emit the build report for /tmp/ws/build-report.md"))
	args := []string{"exec", "--output-last-message", "/tmp/ws/build-report.md"}
	inv, err := parseArgs(args, stdin)
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if inv.ArtifactPath != "/tmp/ws/build-report.md" {
		t.Errorf("ArtifactPath=%q, want /tmp/ws/build-report.md", inv.ArtifactPath)
	}
	if !strings.Contains(inv.Prompt, "build report") {
		t.Errorf("Prompt did not capture stdin: %q", inv.Prompt)
	}
}

func TestParseArgs_CodexStyle_WithMFlag(t *testing.T) {
	// Codex driver may pass `exec -m <model> --output-last-message <path>`.
	stdin := bytes.NewReader([]byte("audit /tmp/audit-report.md"))
	args := []string{"exec", "-m", "gpt-4", "--output-last-message", "/tmp/audit-report.md"}
	inv, err := parseArgs(args, stdin)
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if inv.ArtifactPath != "/tmp/audit-report.md" {
		t.Errorf("ArtifactPath=%q", inv.ArtifactPath)
	}
}

func TestParseArgs_VersionFlag(t *testing.T) {
	inv, err := parseArgs([]string{"--version"}, bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if !inv.VersionOnly {
		t.Error("VersionOnly must be true for --version")
	}
}

// detectPhase identifies which evolve-loop phase the prompt belongs to
// from the artifact path the bridge handed us. Filename suffix is the
// most reliable signal — every phase has a unique artifact filename.
func TestDetectPhase(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/tmp/ws/intent.md", "intent"},
		{"/tmp/ws/intent-delta.md", "intent"},
		{"/tmp/ws/scout-report.md", "scout"},
		{"/tmp/ws/triage-report.md", "triage"},
		{"/tmp/ws/test-report.md", "tdd"},
		{"/tmp/ws/build-report.md", "build"},
		{"/tmp/ws/audit-report.md", "audit"},
		{"/tmp/ws/retrospective.md", "retro"},
		{"/tmp/ws/mystery.md", "unknown"},
	}
	for _, tc := range cases {
		if got := detectPhase(tc.path); got != tc.want {
			t.Errorf("detectPhase(%q)=%q, want %q", tc.path, got, tc.want)
		}
	}
}

// artifactsFor returns the file map a phase should emit. Most phases
// emit one file; audit uniquely emits BOTH audit-report.md AND
// acs-verdict.json (the EGPS gate fusion).
func TestArtifactsFor_PerPhaseShape(t *testing.T) {
	dir := t.TempDir()

	cases := []struct {
		phase             string
		mainPath          string
		wantMainMarkers   []string
		wantExtraFile     string // empty if none
		wantExtraJSONKeys []string
	}{
		{
			phase:           "intent",
			mainPath:        filepath.Join(dir, "intent.md"),
			wantMainMarkers: []string{"goal:", "acceptance_checks:"},
		},
		{
			phase:           "scout",
			mainPath:        filepath.Join(dir, "scout-report.md"),
			wantMainMarkers: []string{"## Proposed Tasks", "- "},
		},
		{
			phase:           "triage",
			mainPath:        filepath.Join(dir, "triage-report.md"),
			wantMainMarkers: []string{"## top_n", "- "},
		},
		{
			phase:           "tdd",
			mainPath:        filepath.Join(dir, "test-report.md"),
			wantMainMarkers: []string{"## Acceptance", "## RED Tests"},
		},
		{
			phase:           "build",
			mainPath:        filepath.Join(dir, "build-report.md"),
			wantMainMarkers: []string{"## Files Modified"},
		},
		{
			phase:             "audit",
			mainPath:          filepath.Join(dir, "audit-report.md"),
			wantMainMarkers:   []string{"## Verdict", "**PASS**"},
			wantExtraFile:     filepath.Join(dir, "acs-verdict.json"),
			wantExtraJSONKeys: []string{"red_count"},
		},
		{
			phase:           "retro",
			mainPath:        filepath.Join(dir, "retrospective.md"),
			wantMainMarkers: []string{"## Lessons"},
			wantExtraFile:   filepath.Join(dir, "failure-lesson-1.yaml"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.phase, func(t *testing.T) {
			files, err := artifactsFor(tc.phase, tc.mainPath, "PASS")
			if err != nil {
				t.Fatalf("artifactsFor: %v", err)
			}
			content, ok := files[tc.mainPath]
			if !ok {
				t.Fatalf("main artifact %q not produced; keys=%v", tc.mainPath, mapKeys(files))
			}
			for _, m := range tc.wantMainMarkers {
				if !strings.Contains(content, m) {
					t.Errorf("main artifact missing marker %q; content=%q", m, content)
				}
			}
			if tc.wantExtraFile != "" {
				extra, ok := files[tc.wantExtraFile]
				if !ok {
					t.Fatalf("extra artifact %q not produced; keys=%v", tc.wantExtraFile, mapKeys(files))
				}
				for _, k := range tc.wantExtraJSONKeys {
					if !strings.Contains(extra, k) {
						t.Errorf("extra artifact missing JSON key %q; content=%q", k, extra)
					}
				}
				if len(tc.wantExtraJSONKeys) > 0 {
					var probe map[string]any
					if err := json.Unmarshal([]byte(extra), &probe); err != nil {
						t.Errorf("extra artifact not valid JSON: %v\n%s", err, extra)
					}
				}
			}
		})
	}
}

func TestArtifactsFor_UnknownPhase(t *testing.T) {
	files, err := artifactsFor("nonesuch", "/tmp/x.md", "PASS")
	if err == nil && len(files) == 0 {
		t.Error("unknown phase must error OR emit empty artifact, but got both no error and no files")
	}
}

// run executes the end-to-end fake-cli flow: parse args, write the
// right artifact(s), exit 0. Returns the exit code.
func TestRun_HappyPathClaudeScout(t *testing.T) {
	dir := t.TempDir()
	artifact := filepath.Join(dir, "scout-report.md")
	args := []string{"-p", "write to " + artifact + " please", "--model", "sonnet"}

	var stdout, stderr bytes.Buffer
	rc := run(args, bytes.NewReader(nil), &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d, stderr=%s", rc, stderr.String())
	}

	body, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatalf("artifact not written: %v", err)
	}
	if !strings.Contains(string(body), "## Proposed Tasks") {
		t.Errorf("scout artifact missing marker: %s", string(body))
	}
}

func TestRun_HappyPathCodexBuild(t *testing.T) {
	dir := t.TempDir()
	artifact := filepath.Join(dir, "build-report.md")
	prompt := "synthesize build for " + artifact
	args := []string{"exec", "--output-last-message", artifact}

	var stdout, stderr bytes.Buffer
	rc := run(args, bytes.NewReader([]byte(prompt)), &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d, stderr=%s", rc, stderr.String())
	}

	body, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatalf("artifact not written: %v", err)
	}
	if !strings.Contains(string(body), "## Files Modified") {
		t.Errorf("build artifact missing marker: %s", string(body))
	}
}

func TestRun_HappyPathAuditEmitsBothFiles(t *testing.T) {
	dir := t.TempDir()
	artifact := filepath.Join(dir, "audit-report.md")
	args := []string{"-p", "audit verdict to " + artifact, "--model", "opus"}

	var stdout, stderr bytes.Buffer
	rc := run(args, bytes.NewReader(nil), &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d, stderr=%s", rc, stderr.String())
	}

	if _, err := os.Stat(artifact); err != nil {
		t.Fatalf("audit-report.md not written: %v", err)
	}
	acsPath := filepath.Join(dir, "acs-verdict.json")
	body, err := os.ReadFile(acsPath)
	if err != nil {
		t.Fatalf("acs-verdict.json not written next to audit-report.md: %v", err)
	}
	var probe struct {
		RedCount int `json:"red_count"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		t.Fatalf("acs-verdict.json invalid: %v", err)
	}
	if probe.RedCount != 0 {
		t.Errorf("red_count=%d, want 0 (happy path)", probe.RedCount)
	}
}

func TestRun_NoArtifactPath_FailsCleanly(t *testing.T) {
	args := []string{"-p", "no artifact mentioned at all"}
	var stdout, stderr bytes.Buffer
	rc := run(args, bytes.NewReader(nil), &stdout, &stderr)
	if rc == 0 {
		t.Error("missing artifact path must produce non-zero rc")
	}
	if !strings.Contains(stderr.String(), "artifact") {
		t.Errorf("stderr must mention artifact; got %q", stderr.String())
	}
}

func TestRun_VersionFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := run([]string{"--version"}, bytes.NewReader(nil), &stdout, &stderr)
	if rc != 0 {
		t.Errorf("--version rc=%d, want 0", rc)
	}
	if !strings.Contains(stdout.String(), "fake") {
		t.Errorf("stdout=%q must mention 'fake'", stdout.String())
	}
}

func mapKeys[K comparable, V any](m map[K]V) []K {
	out := make([]K, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

type errReader struct{}

func (errReader) Read(_ []byte) (int, error) { return 0, errBrokenPipe }

var errBrokenPipe = &myErr{msg: "broken pipe"}

type myErr struct{ msg string }

func (e *myErr) Error() string { return e.msg }

func TestParseArgs_CodexStdinError(t *testing.T) {
	args := []string{"exec", "--output-last-message", "/tmp/x.md"}
	if _, err := parseArgs(args, errReader{}); err == nil {
		t.Error("parseArgs must propagate stdin read failure")
	}
}

func TestRun_UnknownPhase(t *testing.T) {
	// detectPhase returns "unknown" → artifactsFor errors. We need an artifact
	// path whose basename isn't a known phase file; a "-p" prompt mentioning
	// e.g. /tmp/intent.md would match the regex and resolve to the intent phase,
	// so hand-craft an unknown basename via codex's --output-last-message.
	mystery := "/tmp/mystery-output.md"
	args := []string{"exec", "--output-last-message", mystery}

	var stdout, stderr bytes.Buffer
	rc := run(args, bytes.NewReader([]byte("prompt")), &stdout, &stderr)
	if rc == 0 {
		t.Error("unknown phase must produce non-zero rc")
	}
	if !strings.Contains(stderr.String(), "unknown phase") {
		t.Errorf("stderr must mention unknown phase; got %q", stderr.String())
	}
}

func TestRun_UnwritableDir(t *testing.T) {
	// Point artifact at an unwritable path; mkdir/write fails.
	args := []string{"-p", "write to /this/path/cannot/be/created/by/test/scout-report.md", "--model", "x"}
	// On most systems /proc, /sys etc are not writable. /this/path/... will fail.
	var stdout, stderr bytes.Buffer
	rc := run(args, bytes.NewReader(nil), &stdout, &stderr)
	if rc == 0 {
		t.Error("unwritable artifact path must produce non-zero rc")
	}
}

// Regression: an early version of absPathRE matched
// "/scout-report.md" by backtracking inside `workspace/scout-report.md`,
// causing the fake to write to the filesystem root.
func TestParseArgs_RelativePathInPromptDoesNotMatchAbs(t *testing.T) {
	// Real production prompts begin with the agent heading; we keep that
	// shape here so the heading-based phase resolver fires.
	prompt := "# Evolve Scout\n\nbody talks about workspace/scout-report.md as a Workspace File reference.\n\n## Cycle Context\n- workspace: /tmp/ws/cycle-7\n"
	args := []string{"-p", prompt, "--model", "auto"}
	inv, err := parseArgs(args, bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if inv.ArtifactPath == "/scout-report.md" {
		t.Fatalf("regression: absPathRE matched relative mention as root path")
	}
	if inv.ArtifactPath != "/tmp/ws/cycle-7/scout-report.md" {
		t.Errorf("ArtifactPath=%q, want /tmp/ws/cycle-7/scout-report.md", inv.ArtifactPath)
	}
}

// Multi-artifact prompt (e.g. builder mentions scout-report.md as
// upstream input) must NOT misroute to scout when the heading is
// "# Evolve Builder". This is the bug that caused the matrix to fail
// at the Build phase.
func TestParseArgs_HeadingResolvesBuilderEvenIfPromptMentionsScout(t *testing.T) {
	prompt := strings.Join([]string{
		"# Evolve Builder",
		"You will read scout-report.md and write build-report.md.",
		"",
		"## Cycle Context",
		"- workspace: /tmp/ws/cycle-2",
	}, "\n")
	args := []string{"-p", prompt, "--model", "auto"}
	inv, err := parseArgs(args, bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if inv.Phase != "build" {
		t.Errorf("Phase=%q, want build", inv.Phase)
	}
	if inv.ArtifactPath != "/tmp/ws/cycle-2/build-report.md" {
		t.Errorf("ArtifactPath=%q, want /tmp/ws/cycle-2/build-report.md", inv.ArtifactPath)
	}
}

func TestParseArgs_BooleanFlagWithEquals(t *testing.T) {
	// Some bridge driver permutations may use --flag=value form; ensure we
	// don't blow up consuming them.
	args := []string{"-p", "write /tmp/scout-report.md", "--allowedTools=Bash", "--something-else"}
	inv, err := parseArgs(args, bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if inv.ArtifactPath != "/tmp/scout-report.md" {
		t.Errorf("ArtifactPath=%q", inv.ArtifactPath)
	}
}
