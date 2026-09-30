//go:build acs

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
	fixtureWorktree = "/repo/worktrees/cycle-1457"
	fixtureArtifact = ".evolve/runs/cycle-1457/build-report.md"
	canonicalMarker = "Artifact path: "

	windowStart = "2026-08-14T10:00:00Z"
	windowEnd   = "2026-08-14T11:00:00Z"
	turnStamp   = "2026-08-14T10:00:02Z"
)

var fixtureUsage = cyclestate.TokenUsage{Input: 40, Output: 4}

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

	root := acsassert.RepoRoot(t)
	if !acsassert.LineContainsAll(filepath.Join(root, "go/internal/subagent/subagent.go"), `"Artifact path: %s\n"`) {
		t.Errorf("subagent.go no longer stamps %q — the scanner's anchor and the assembler have drifted apart", canonicalMarker)
	}
	if !acsassert.LineContainsAll(filepath.Join(root, "go/internal/subagent/subagentrun/prompt.go"), `"- Artifact path: %s\n"`) {
		t.Errorf("subagentrun/prompt.go no longer stamps %q — the scanner's anchor and the assembler have drifted apart", canonicalMarker)
	}
}

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

	scannerTest := filepath.Join(root, "go/internal/tokenusage/scanner_test.go")
	if !acsassert.FileContains(t, scannerTest, canonicalMarker+".evolve/runs/cycle-997/launch-token-abc123") {
		t.Errorf("scanner_test.go's S1 fixture still uses the bare-path form — it must carry the %q marker the assemblers stamp", canonicalMarker)
	}
}
