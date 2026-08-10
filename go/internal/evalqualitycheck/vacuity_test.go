package evalqualitycheck

// vacuity_test.go — pins the fix for the vacuous quality-gate class found in
// the 2026-08-09 postmortem sweep (docs/incidents/2026-08-09-zero-ship-batch.md,
// ADR-0084 invariant 2): the scout template mandates `- [code] <cmd>` bullet
// graders (agents/evolve-scout-reference.md, eval-format-template anchor) but
// scanBashCommands read only ```bash fences — so 281 of 625 live evals
// (bullet-format) produced ZERO parsed commands and the anti-gaming gate
// returned LevelPass vacuously. Three pins: (1) the bullet format parses,
// (2) the template's own literal example round-trips through the production
// scanner (single-source: template drift breaks this test), (3) zero parsed
// commands is a WARN, never a silent PASS.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func checkContent(t *testing.T, content string) Result {
	t.Helper()
	p := filepath.Join(t.TempDir(), "eval.md")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Check(Options{Path: p})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	return res
}

func TestCheck_CodeBulletFormatIsParsed(t *testing.T) {
	res := checkContent(t, "# Eval: x\n## Code Graders (bash commands that must exit 0)\n- `[code]` `go test ./internal/core/`\n- `[code]` `test -s go/internal/core/fleet.go`\n- `[model]` Rubric: \"clarity\" — threshold: >= 60\n")
	if len(res.Commands) != 2 {
		t.Fatalf("parsed %d commands, want 2 — the template's `[code]` bullet format must be scanned ([model]/[human] lines are not bash)", len(res.Commands))
	}
	for _, c := range res.Commands {
		if strings.Contains(c.Line, "[code]") {
			t.Errorf("command %q still carries the [code] marker — extract only the command span", c.Line)
		}
	}
}

func TestCheck_TemplateExampleRoundTripsThroughScanner(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(thisFile))))
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-scout-reference.md"))
	if err != nil {
		t.Skipf("scout reference not found: %v", err)
	}
	const anchor = "<!-- ANCHOR:eval-format-template -->"
	i := strings.Index(string(raw), anchor)
	if i < 0 {
		t.Fatal("eval-format-template anchor missing from agents/evolve-scout-reference.md — the single-source tie is broken")
	}
	section := string(raw)[i:]
	if j := strings.Index(section[len(anchor):], "<!-- ANCHOR:"); j >= 0 {
		section = section[:len(anchor)+j]
	}
	// The template shows the eval INSIDE a ````markdown illustration fence;
	// an authored eval file carries the bullets at top level. Unwrap the
	// fence to scan what an authored file would actually contain (fenced
	// bullets are deliberately NOT scanned — see the decoy test below).
	fence := "````markdown"
	fi := strings.Index(section, fence)
	if fi < 0 {
		t.Fatalf("template example fence not found — the eval-format-template anchor changed shape")
	}
	inner := section[fi+len(fence):]
	if fj := strings.Index(inner, "````"); fj >= 0 {
		inner = inner[:fj]
	}
	res := checkContent(t, inner)
	if len(res.Commands) < 3 {
		t.Errorf("the template's own literal example yields %d parsed commands, want >= 3 — if the template format and the scanner drift, the quality gate goes vacuous again", len(res.Commands))
	}
}

func TestCheck_ZeroCommandsIsWarnNeverSilentPass(t *testing.T) {
	res := checkContent(t, "# Eval: prose-only\nSome acceptance prose with no graders at all.\n")
	if res.Overall != LevelWarn {
		t.Errorf("Overall = %v, want LevelWarn — zero parsed commands means the gate verified NOTHING; silent PASS is the vacuity that defeated the gate", res.Overall)
	}
	if len(res.Commands) != 1 || res.Commands[0].Reason == "" {
		t.Errorf("the WARN must carry an explanatory entry (operator-visible why), got %+v", res.Commands)
	}
}

func TestCheck_FencedDecoyBulletIsNotACommand(t *testing.T) {
	// The adversarial-review BLOCK scenario: a `[code]`-styled bullet inside
	// a non-bash fence is illustration (or a planted decoy to fake rigor)
	// and must not be extracted as a real command.
	res := checkContent(t, "# Eval: decoy\n```text\n- `[code]` `rm -rf /tmp/should-not-run`\n```\n")
	for _, c := range res.Commands {
		if strings.Contains(c.Line, "should-not-run") {
			t.Fatalf("fenced decoy bullet was extracted as a command: %+v", c)
		}
	}
	if res.Overall != LevelWarn {
		t.Errorf("Overall = %v, want LevelWarn — a decoy-only eval has zero real graders", res.Overall)
	}
}

func TestCheck_ScoreCapGradedEvalPassesWithNote(t *testing.T) {
	// ~82% of the live corpus is score_cap/evidence-graded (consumed by the
	// ACS suite, not this scanner) — zero bash commands there is designed,
	// and a blanket WARN would train operators to ignore real WARNs.
	res := checkContent(t, "# Eval: acs-form\nscore_cap: 0.8\nevidence: \"cd go && go test ./internal/core/\"\n")
	if res.Overall != LevelPass {
		t.Errorf("Overall = %v, want LevelPass — score_cap grading is the ACS suite's jurisdiction", res.Overall)
	}
	if len(res.Commands) != 1 || !strings.Contains(res.Commands[0].Reason, "ACS") {
		t.Errorf("the pass must carry the score_cap note entry, got %+v", res.Commands)
	}
}
