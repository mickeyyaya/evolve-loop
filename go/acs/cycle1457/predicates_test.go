//go:build acs

// Package cycle1457 materialises the cycle-1457 acceptance criteria for the one
// fleet-scoped task triage committed to this lane:
//
//   - tokenopt-attribution-marker-anchored → tokenusage.attributes() must match
//     the assembler's literal "Artifact path: <path>" marker, not a bare
//     ArtifactPath substring anywhere in the first user message.
//
// The defect. go/internal/tokenusage/scanner.go:209 attributes a transcript to a
// launch with `strings.Contains(firstUserText(lines), w.ArtifactPath)`. Every
// production launch DOES carry the path — but so does any transcript that merely
// MENTIONS it in prose. `.evolve/profiles/retrospective.json:118` instructs a
// phase to "Read .evolve/runs/cycle-{cycle}/build-report.md and
// audit-report.md": under the bare-substring rule that transcript attributes to
// the BUILDER's Window and its tokens are billed to the builder. Anchoring the
// match to the literal marker both assemblers stamp closes the vector.
//
// Predicate strategy — every predicate drives the REAL production entry point
// `tokenusage.ScanConfigRoot` over a real on-disk transcript fixture and asserts
// on the returned Usage/Source (the cycle-85 degenerate-predicate ban: no
// predicate here is load-bearing on a source grep). Source greps appear only as
// auxiliary coupling checks in 003 and 004, never as the sole assertion.
//
//   - 001 both real assembler forms still attribute (anti-regression: the fix
//     must not narrow attribution for genuine launches). Expected pre-existing
//     GREEN — it guards the fix from over-correcting.
//   - 002 is the crux: a prose-only mention of the ArtifactPath, with a
//     non-matching cwd so the fallback cannot rescue it, must NOT attribute.
//     RED today — the bare substring matches.
//   - 003 label drift: a near-miss marker label ("Artifact-path:",
//     "artifact path:") must NOT attribute, and both assemblers must still emit
//     the canonical "Artifact path: " label. RED today for the same reason as
//     002.
//   - 004 runs the tokenusage unit suite's two contract tests as a subprocess
//     (ONE named package, -run-narrowed per the flaky-shape rules) and requires
//     both to PASS: the updated S1 concurrent-sessions test and the new
//     TestAttributes_MarkerAnchored. RED today — the latter does not exist.
package cycle1457

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	// fixtureWorktree is the cwd every fixture transcript records unless a test
	// deliberately diverges from it.
	fixtureWorktree = "/repo/worktrees/cycle-1457"
	// fixtureArtifact is the launch's unique deliverable reference — the
	// attribution key under test.
	fixtureArtifact = ".evolve/runs/cycle-1457/build-report.md"
	// canonicalMarker is the literal label BOTH assemblers stamp ahead of the
	// artifact path (subagent.go composePrompt, run.go assembleV2Prompt). The
	// anchored match must key on exactly this.
	canonicalMarker = "Artifact path: "

	windowStart = "2026-08-14T10:00:00Z"
	windowEnd   = "2026-08-14T11:00:00Z"
	turnStamp   = "2026-08-14T10:00:02Z"
)

// fixtureUsage is the assistant turn every fixture emits; a transcript that
// attributes contributes exactly this, one that does not contributes nothing.
var fixtureUsage = cyclestate.TokenUsage{Input: 40, Output: 4}

// writeTranscript materialises a two-line transcript (one user message carrying
// firstUserText, one assistant turn carrying fixtureUsage) under a config root
// laid out the way ScanConfigRoot walks it, and returns that root.
func writeTranscript(t *testing.T, cwd, firstUserText string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "projects", "-repo-worktrees-cycle-1457")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir transcript dir: %v", err)
	}
	body := `{"type":"user","cwd":"` + cwd + `","timestamp":"` + windowStart + `","message":{"id":"u1","content":` + jsonString(t, firstUserText) + `}}
{"type":"assistant","cwd":"` + cwd + `","timestamp":"` + turnStamp + `","message":{"id":"m1","usage":{"input_tokens":40,"output_tokens":4,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
`
	if err := os.WriteFile(filepath.Join(dir, "sess.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}
	return root
}

// jsonString encodes s as a JSON string literal so multi-line prompt bodies
// survive embedding in the fixture.
func jsonString(t *testing.T, s string) string {
	t.Helper()
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// scanFixture runs the REAL production entry point over the fixture.
func scanFixture(t *testing.T, cwd, firstUserText string) tokenusage.Result {
	t.Helper()
	root := writeTranscript(t, cwd, firstUserText)
	start, err := time.Parse(time.RFC3339, windowStart)
	if err != nil {
		t.Fatalf("parse window start: %v", err)
	}
	end, err := time.Parse(time.RFC3339, windowEnd)
	if err != nil {
		t.Fatalf("parse window end: %v", err)
	}
	res, err := tokenusage.ScanConfigRoot(root, tokenusage.Window{
		Worktree:     fixtureWorktree,
		ArtifactPath: fixtureArtifact,
		Start:        start,
		End:          end,
	})
	if err != nil {
		t.Fatalf("ScanConfigRoot: %v", err)
	}
	return res
}

// TestC1457_001_RealAssemblerFormsStillAttribute pins the anti-regression half of
// the fix: both prompt forms a production launch actually carries MUST still
// attribute after the match is anchored. The bodies are the real assembler
// output shapes — subagent.go's composePrompt stamps "Artifact path: %s" inside
// the "## INVOCATION CONTEXT ##" block; run.go's assembleV2Prompt stamps the
// same label as a markdown list item, "- Artifact path: %s". The leading "- " is
// prose decoration OUTSIDE the key, so an implementation that anchors on
// "Artifact path: "+path (not on the list bullet) satisfies both.
func TestC1457_001_RealAssemblerFormsStillAttribute(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{
			name: "subagent composePrompt form",
			body: "## INVOCATION CONTEXT ##\nAgent: builder\nCycle: 1457\nChallenge token: a82ddd0923c5d22a\n" +
				canonicalMarker + fixtureArtifact + "\n\n## BEGIN TASK PROMPT ##\nbuild it\n",
		},
		{
			name: "run.go assembleV2Prompt list form",
			body: "## INVOCATION CONTEXT\n\n- Agent: builder\n- Cycle: 1457\n- Workspace: .evolve/runs/cycle-1457\n" +
				"- " + canonicalMarker + fixtureArtifact + "\n- Challenge token: a82ddd0923c5d22a\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := scanFixture(t, fixtureWorktree, tc.body)
			if res.Source != tokenusage.SourceTranscript {
				t.Errorf("Source = %q, want %q — a genuine launch stamped by %s must still attribute; anchoring must not narrow real launches out",
					res.Source, tokenusage.SourceTranscript, tc.name)
			}
			if res.Usage != fixtureUsage {
				t.Errorf("Usage = %+v, want %+v — the attributed transcript's only in-window turn must be counted", res.Usage, fixtureUsage)
			}
		})
	}
}

// TestC1457_002_ProseMentionDoesNotAttribute is the crux regression guard for the
// over-attribution vector. The transcript's first user message MENTIONS the
// builder's artifact path in prose — exactly what
// .evolve/profiles/retrospective.json:118 tells a retrospective launch to do —
// but carries no "Artifact path: " marker of its own. Its cwd is deliberately a
// DIFFERENT worktree so the ArtifactPath-less cwd fallback cannot rescue the
// match and mask the defect. It must not attribute: those tokens belong to the
// retrospective launch, not the builder's Window.
//
// RED today: scanner.go:209's bare strings.Contains(firstUserText, ArtifactPath)
// matches the prose mention and bills a whole foreign launch to the builder.
func TestC1457_002_ProseMentionDoesNotAttribute(t *testing.T) {
	body := "## INVOCATION CONTEXT ##\nAgent: retrospective\nCycle: 1457\n" +
		canonicalMarker + ".evolve/runs/cycle-1457/retro-report.md\n\n" +
		"## BEGIN TASK PROMPT ##\nRead " + fixtureArtifact + " and audit-report.md, then summarise.\n"

	res := scanFixture(t, "/repo/worktrees/cycle-other-lane", body)

	if res.Source != tokenusage.SourceNone {
		t.Errorf("Source = %q, want %q — a transcript that only MENTIONS %q in prose (its own marker names a different artifact) must not attribute to that launch",
			res.Source, tokenusage.SourceNone, fixtureArtifact)
	}
	if res.Usage != (cyclestate.TokenUsage{}) {
		t.Errorf("Usage = %+v, want zero — a foreign launch's tokens must never be billed to the Window it merely cites (bare-substring over-attribution)", res.Usage)
	}
}

// TestC1457_003_MarkerLabelDriftDoesNotAttribute pins the anchor to the exact
// label rather than to any lookalike, and pins the assemblers to that same
// label. Two halves:
//
//   - behavioural (load-bearing): near-miss labels — a hyphenated
//     "Artifact-path:", a lowercased "artifact path:" — must NOT attribute.
//     Under the bare-substring rule they all match, so this is RED today. Under
//     a correctly anchored match they cannot.
//   - auxiliary coupling: both assemblers must still emit the canonical label.
//     If either format string drifts, the anchored match silently stops
//     attributing real launches and this predicate says which side moved.
func TestC1457_003_MarkerLabelDriftDoesNotAttribute(t *testing.T) {
	for _, label := range []string{"Artifact-path: ", "artifact path: ", "ArtifactPath="} {
		t.Run(strings.TrimSpace(label), func(t *testing.T) {
			body := "## INVOCATION CONTEXT ##\nAgent: scout\nCycle: 1457\n" + label + fixtureArtifact + "\n"
			res := scanFixture(t, "/repo/worktrees/cycle-other-lane", body)
			if res.Source != tokenusage.SourceNone {
				t.Errorf("Source = %q, want %q — %q is not the canonical %q marker; matching it means the match is still a bare substring",
					res.Source, tokenusage.SourceNone, label, canonicalMarker)
			}
		})
	}

	// Auxiliary: the canonical label must remain what the assemblers stamp.
	root := acsassert.RepoRoot(t)
	if !acsassert.LineContainsAll(filepath.Join(root, "go/internal/subagent/subagent.go"), `"Artifact path: %s\n"`) {
		t.Errorf("subagent.go no longer stamps %q — the scanner's anchor and the assembler have drifted apart", canonicalMarker)
	}
	if !acsassert.LineContainsAll(filepath.Join(root, "go/internal/subagent/run.go"), `"- Artifact path: %s\n"`) {
		t.Errorf("run.go no longer stamps %q — the scanner's anchor and the assembler have drifted apart", canonicalMarker)
	}
}

// TestC1457_004_TokenusageContractTestsPass requires the package's own contract
// tests to be present AND green: the new TestAttributes_MarkerAnchored, and the
// S1 concurrent-sessions test whose bare-path fixture must be re-cut to the
// marker form as part of this change (it passes today only because its fixture
// happens to omit the marker — a coincidence, not a contract).
//
// Shape compliance: ONE named package, -run-narrowed to two tests, cmd.Dir set
// explicitly rather than inheriting process cwd (which differs between main
// tree, worktree, and each fleet lane).
//
// RED today: TestAttributes_MarkerAnchored does not exist, so its PASS line
// cannot appear.
func TestC1457_004_TokenusageContractTestsPass(t *testing.T) {
	root := acsassert.RepoRoot(t)

	const runRE = `^(TestAttributes_MarkerAnchored|TestTranscriptScan_ConcurrentSessionsSameDir_OnlyContentVerifiedCounted)$`
	cmd := exec.Command("go", "test", "-count=1", "-v", "-run", runRE, "./internal/tokenusage")
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	got := string(out)
	if err != nil {
		t.Errorf("go test ./internal/tokenusage failed: %v\n%s", err, got)
	}
	for _, name := range []string{
		"TestAttributes_MarkerAnchored",
		"TestTranscriptScan_ConcurrentSessionsSameDir_OnlyContentVerifiedCounted",
	} {
		if !strings.Contains(got, "--- PASS: "+name) {
			t.Errorf("no `--- PASS: %s` in the tokenusage run — the test is missing, skipped, or failing:\n%s", name, got)
		}
	}

	// Auxiliary: the S1 fixture must be re-cut to the marker form. A bare-path
	// fixture would keep passing against an anchored implementation only by
	// accident of the cwd fallback, so the update is part of the contract.
	scannerTest := filepath.Join(root, "go/internal/tokenusage/scanner_test.go")
	if !acsassert.FileContains(t, scannerTest, canonicalMarker+".evolve/runs/cycle-997/launch-token-abc123") {
		t.Errorf("scanner_test.go's S1 fixture still uses the bare-path form — it must carry the %q marker the assemblers stamp", canonicalMarker)
	}
}
