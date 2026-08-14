package tokenusage

// scanner_marker_test.go — regression contract for the over-attribution vector
// closed in cycle-1457. attributes() used to key on a BARE ArtifactPath
// substring anywhere in the first user message. Every production launch carries
// the path, but so does any prompt that merely cites it in prose:
// .evolve/profiles/retrospective.json instructs a retrospective launch to "Read
// .evolve/runs/cycle-{cycle}/build-report.md", which under the bare rule billed
// the whole retrospective launch to the BUILDER's Window. Both assemblers stamp
// a literal label — subagent.go:358 ("Artifact path: %s\n") and run.go:442
// ("- Artifact path: %s\n") — so the match anchors on artifactMarker+path.

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// TestAttributes_MarkerAnchored drives the real ScanConfigRoot entry point over
// one fixture per prompt shape and asserts the attribution verdict. The two
// assembler forms must still attribute (the anchor must not narrow genuine
// launches out); a prose citation and every near-miss label must not. Each
// fixture records a cwd DIFFERENT from Window.Worktree so the ArtifactPath-less
// cwd fallback can never rescue a match and mask the anchor.
func TestAttributes_MarkerAnchored(t *testing.T) {
	const (
		worktree = "/repo/worktrees/cycle-1457"
		foreign  = "/repo/worktrees/cycle-other-lane"
		artifact = ".evolve/runs/cycle-1457/build-report.md"
	)
	attributed := cyclestate.TokenUsage{Input: 40, Output: 4}

	tests := []struct {
		name          string
		firstUserText string
		wantSource    Source
		wantUsage     cyclestate.TokenUsage
		why           string
	}{
		{
			name:          "subagent composePrompt form attributes",
			firstUserText: "## INVOCATION CONTEXT ##\nAgent: builder\nCycle: 1457\nArtifact path: " + artifact + "\n",
			wantSource:    SourceTranscript,
			wantUsage:     attributed,
			why:           "the marker form subagent.go:358 stamps is the genuine launch shape",
		},
		{
			name:          "run.go assembleV2Prompt list form attributes",
			firstUserText: "## INVOCATION CONTEXT\n\n- Agent: builder\n- Artifact path: " + artifact + "\n",
			wantSource:    SourceTranscript,
			wantUsage:     attributed,
			why:           "run.go:442's leading list bullet is prose decoration OUTSIDE the key",
		},
		{
			// The shape a REAL loop-phase prompt carries: the bridge's contract
			// footer, not the subagent assembler's marker. Verified against the
			// cycle-1457 build prompt itself, whose only path disclosure is
			// "DELIVERABLE PATH: <abs>" (phasecontract/render.go:86) — an anchor
			// set that omitted this would silently zero every loop launch's
			// transcript-tier token telemetry.
			name:          "bridge contract footer form attributes",
			firstUserText: "…END OF PROMPT\n\nDELIVERABLE PATH: " + artifact + "\n",
			wantSource:    SourceTranscript,
			wantUsage:     attributed,
			why:           "the contract footer is the loop dispatch path's path disclosure",
		},
		{
			name:          "contract tail artifact-path element attributes",
			firstUserText: "<deliverable-contract phase=\"build\">  <artifact-path>" + artifact + "</artifact-path>\n",
			wantSource:    SourceTranscript,
			wantUsage:     attributed,
			why:           "render.go:117 stamps the path inside the contract tail element",
		},
		{
			name: "the footer's own instruction line does not attribute by itself",
			firstUserText: "- Write it to the EXACT absolute path shown under \"DELIVERABLE PATH:\" at the END of this prompt.\n" +
				"Read " + artifact + " for context.\n",
			wantSource: SourceNone,
			why:        "the instruction line names the label but not this launch's path; only anchor+path attributes",
		},
		{
			name:          "prose citation of a foreign artifact does not attribute",
			firstUserText: "## INVOCATION CONTEXT ##\nAgent: retrospective\nArtifact path: .evolve/runs/cycle-1457/retro-report.md\n\nRead " + artifact + " and audit-report.md, then summarise.\n",
			wantSource:    SourceNone,
			why:           "the retrospective launch's tokens belong to the retrospective, not to the Window it cites",
		},
		{
			name:          "hyphenated near-miss label does not attribute",
			firstUserText: "Artifact-path: " + artifact + "\n",
			wantSource:    SourceNone,
			why:           "matching a lookalike label means the match is still a bare substring",
		},
		{
			name:          "lowercased near-miss label does not attribute",
			firstUserText: "artifact path: " + artifact + "\n",
			wantSource:    SourceNone,
			why:           "the anchor is the assemblers' exact literal, case included",
		},
		{
			name:          "key-value near-miss label does not attribute",
			firstUserText: "ArtifactPath=" + artifact + "\n",
			wantSource:    SourceNone,
			why:           "no assembler emits this shape; matching it reopens the vector",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			sessionDir := filepath.Join(root, "projects", "-repo-worktrees-cycle-1457")
			body := `{"type":"user","cwd":"` + foreign + `","timestamp":"2026-07-07T10:00:01Z","message":{"id":"u1","content":[{"type":"text","text":` + jsonQuote(tc.firstUserText) + `}]}}
{"type":"assistant","cwd":"` + foreign + `","timestamp":"2026-07-07T10:00:02Z","message":{"id":"m1","usage":{"input_tokens":40,"output_tokens":4,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
`
			writeTranscript(t, sessionDir, "sess.jsonl", body)

			res, err := ScanConfigRoot(root, Window{
				Worktree:     worktree,
				ArtifactPath: artifact,
				Start:        mustParse(t, launchWindowStart),
				End:          mustParse(t, launchWindowEnd),
			})
			if err != nil {
				t.Fatalf("ScanConfigRoot: %v", err)
			}
			if res.Source != tc.wantSource {
				t.Errorf("Source = %q, want %q — %s", res.Source, tc.wantSource, tc.why)
			}
			if res.Usage != tc.wantUsage {
				t.Errorf("Usage = %+v, want %+v — %s", res.Usage, tc.wantUsage, tc.why)
			}
		})
	}
}

// TestArtifactAnchors_MatchRenderedContract is the drift guard for the bridge
// side of the anchor set. Predicate 003 pins the two subagent format strings by
// grep; the contract footer and tail have no such guard, so this renders them
// from the REAL phasecontract assembler and requires each anchor form to match
// the rendered bytes. If render.go ever restyles its path disclosure, this fails
// loudly instead of letting loop-phase attribution silently go dark.
func TestArtifactAnchors_MatchRenderedContract(t *testing.T) {
	const artifact = "/repo/.evolve/runs/cycle-1457/build-report.md"
	c := phasecontract.Contract{Phase: "build", AgentName: "build", ArtifactName: "build-report.md"}

	rendered := map[string]string{
		"RenderContractFooter": phasecontract.RenderContractFooter(c, artifact),
		"RenderContractTail":   phasecontract.RenderContractTail(c, artifact),
	}
	for name, body := range rendered {
		matched := false
		for _, anchor := range artifactAnchors {
			if strings.Contains(body, anchor+artifact) {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("%s's path disclosure matches no artifactAnchors entry — the assembler and the scanner's anchor set have drifted, so every launch dispatched through it would stop attributing:\n%s", name, body)
		}
	}
}

// jsonQuote encodes s as a JSON string literal so multi-line prompt bodies
// survive embedding in a fixture transcript.
func jsonQuote(s string) string {
	var b []byte
	b = append(b, '"')
	for _, r := range s {
		switch r {
		case '"':
			b = append(b, '\\', '"')
		case '\\':
			b = append(b, '\\', '\\')
		case '\n':
			b = append(b, '\\', 'n')
		default:
			b = append(b, string(r)...)
		}
	}
	return string(append(b, '"'))
}
