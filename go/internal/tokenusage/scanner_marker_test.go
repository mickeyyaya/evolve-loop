package tokenusage

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// Every fixture records a foreign cwd, so the cwd fallback can never rescue a match and mask the anchor.
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

func TestArtifactAnchors_MatchRenderedContract(t *testing.T) {
	const artifact = "/repo/.evolve/runs/cycle-1457/build-report.md"
	c := phasecontract.Contract{Phase: "build", AgentName: "build", ArtifactName: "build-report.md"}

	rendered := map[string]string{
		"RenderContractFooter": phasecontract.RenderContractFooter(c, artifact),
		"RenderContractTail":   phasecontract.RenderContractTail(c, artifact, filepath.Dir(artifact)),
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
