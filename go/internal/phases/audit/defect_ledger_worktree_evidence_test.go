package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func worktreeContinuationFixture(t *testing.T, ancestorCycle, thisCycle int, openDefects []string) (string, string, core.PhaseRequest) {
	t.Helper()
	ws, req := continuationFixture(t, ancestorCycle, thisCycle, openDefects)
	wt := t.TempDir()
	req.Worktree = wt
	return ws, wt, req
}

func TestClassify_WorktreeResidentEvidenceClosesADefect(t *testing.T) {
	ws, wt, req := worktreeContinuationFixture(t, 1330, 1340, []string{"boundary refresh does not repin the short sha"})
	cite := evidenceFile(t, wt, "go/cmd/evolve/cmd_loop_chain_boundaryrefresh_shortsha_test.go")
	if _, err := os.Stat(filepath.Join(req.ProjectRoot, "go/cmd/evolve/cmd_loop_chain_boundaryrefresh_shortsha_test.go")); err == nil {
		t.Fatalf("fixture is wrong: the citation must NOT exist under the project root — that is the whole deadlock")
	}
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED", "evidence": cite, "reason": "fix landed in this lane's worktree"},
		},
	})

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})

	if verdict != core.VerdictPASS {
		t.Errorf("a closure claim citing a file that exists in this lane's OWN worktree must be accepted; verdict = %q. This is the 1320→1330 deadlock: the gate demands evidence under the project root that only merging can put there, and it is this gate that blocks the merge.\ndiagnostics:\n%s", verdict, diagsText(diags))
	}
	doc := readLedger(t, ws)
	if len(doc.Entries) != 1 {
		t.Fatalf("written-back ledger has %d entries, want 1", len(doc.Entries))
	}
	if doc.Entries[0].Status != "FIXED" {
		t.Errorf("entry d1 status = %q, want FIXED — a gate-accepted closure must be visible in the written-back ledger, not merely implied by the verdict", doc.Entries[0].Status)
	}
	if strings.TrimSpace(doc.Entries[0].Evidence) == "" {
		t.Errorf("entry d1 closed with empty evidence — the accepted citation must be recorded")
	}
}

func TestClassify_EvidenceAbsentFromBothRootsStillBlocks(t *testing.T) {
	ws, _, req := worktreeContinuationFixture(t, 1330, 1340, []string{"boundary refresh does not repin the short sha"})
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED", "evidence": "go/internal/nowhere/imaginary.go:12", "reason": "claims a fix that does not exist"},
		},
	})

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})

	if verdict == core.VerdictPASS {
		t.Errorf("a closure claim whose evidence resolves under NEITHER the project root NOR the worktree must still block PASS — the fallback widens where a real file may live, never whether one must exist")
	}
	if text := diagsText(diags); !strings.Contains(text, "d1") {
		t.Errorf("the unresolvable closure must be named by id; diagnostics:\n%s", text)
	}
	doc := readLedger(t, ws)
	if len(doc.Entries) == 1 && doc.Entries[0].Status != "OPEN" {
		t.Errorf("entry d1 status = %q, want OPEN — a rejected closure must not be written back as FIXED", doc.Entries[0].Status)
	}
}

func TestClassify_WorktreeSelfCitationStillRejected(t *testing.T) {
	ws, wt, req := worktreeContinuationFixture(t, 1330, 1340, []string{"boundary refresh does not repin the short sha"})
	cite := evidenceFile(t, wt, "go/internal/phases/audit/"+ledgerFile)
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED", "evidence": cite, "reason": "cites the mechanism's own record"},
		},
	})

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})

	if verdict == core.VerdictPASS {
		t.Errorf("a closure citing the defect-ledger mechanism's own bookkeeping must be rejected under the WORKTREE root exactly as under the project root — a claim may not vouch for itself from the tree the graded agent writes")
	}
	if text := diagsText(diags); !strings.Contains(text, "d1") {
		t.Errorf("the self-citing closure must be named by id; diagnostics:\n%s", text)
	}
}

func TestClassify_WorktreeEvidenceCannotEscapeRoot(t *testing.T) {
	ws, wt, req := worktreeContinuationFixture(t, 1330, 1340, []string{"boundary refresh does not repin the short sha"})
	evidenceFile(t, filepath.Dir(wt), "outside-the-worktree.go")
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED", "evidence": "../outside-the-worktree.go", "reason": "reaches outside the tree"},
		},
	})

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})

	if verdict == core.VerdictPASS {
		t.Errorf("an escaping citation ('../') must stay rejected even when a worktree root is present; a real file sits at that path and the gate must refuse it on the RULE, not on absence")
	}
	if text := diagsText(diags); !strings.Contains(text, "d1") {
		t.Errorf("the escaping closure must be named by id; diagnostics:\n%s", text)
	}
}

func TestClassify_LineRangeCitationResolves(t *testing.T) {
	for _, suffix := range []string{":570-588", ":570", ":570:12"} {
		t.Run(suffix, func(t *testing.T) {
			ws, _, req := worktreeContinuationFixture(t, 1330, 1340, []string{"brake resolved twice"})
			evidenceFile(t, req.ProjectRoot, "go/cmd/evolve/cmd_loop_chain.go")
			writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
				"dispositions": []any{
					map[string]any{"id": "d1", "status": "FIXED", "evidence": "go/cmd/evolve/cmd_loop_chain.go" + suffix, "reason": "brake resolved once into a local"},
				},
			})

			verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})

			if verdict != core.VerdictPASS {
				t.Errorf("a citation carrying a %q locator must resolve to the file it names; verdict = %q. A line range is the house citation style and names a REAL file — rejecting it is the same false-negative as the worktree miss.\ndiagnostics:\n%s", suffix, verdict, diagsText(diags))
			}
		})
	}
}

func TestClassify_NonLocatorSuffixIsPartOfThePath(t *testing.T) {
	ws, _, req := worktreeContinuationFixture(t, 1330, 1340, []string{"brake resolved twice"})
	evidenceFile(t, req.ProjectRoot, "go/cmd/evolve/cmd_loop_chain.go")
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED", "evidence": "go/cmd/evolve/cmd_loop_chain.go:notaline", "reason": "cites a file that does not exist"},
		},
	})

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})

	if verdict == core.VerdictPASS {
		t.Errorf("a non-numeric ':suffix' is part of the path, not a locator — stripping it would let a claim be satisfied by a DIFFERENT file than the one it names")
	}
	if text := diagsText(diags); !strings.Contains(text, "d1") {
		t.Errorf("the unresolvable closure must be named by id; diagnostics:\n%s", text)
	}
}

func TestClassify_ProjectRootEvidencePathUnchanged(t *testing.T) {
	t.Run("project-root evidence still closes with a worktree set", func(t *testing.T) {
		ws, _, req := worktreeContinuationFixture(t, 1330, 1340, []string{"boundary refresh does not repin the short sha"})
		cite := evidenceFile(t, req.ProjectRoot, "go/internal/core/fleet.go")
		writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
			"dispositions": []any{
				map[string]any{"id": "d1", "status": "FIXED", "evidence": cite, "reason": "already landed"},
			},
		})
		verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})
		if verdict != core.VerdictPASS {
			t.Errorf("project-root-resident evidence must still close a defect; verdict = %q\ndiagnostics:\n%s", verdict, diagsText(diags))
		}
	})

	t.Run("empty worktree root still blocks an unresolvable citation", func(t *testing.T) {
		ws, req := continuationFixture(t, 1330, 1340, []string{"boundary refresh does not repin the short sha"})
		if req.Worktree != "" {
			t.Fatalf("fixture is wrong: Worktree = %q, want empty (the provisioning-failed shape)", req.Worktree)
		}
		writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
			"dispositions": []any{
				map[string]any{"id": "d1", "status": "FIXED", "evidence": "go/internal/nowhere/imaginary.go", "reason": "no such file anywhere"},
			},
		})
		verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})
		if verdict == core.VerdictPASS {
			t.Errorf("with no worktree on the request, an unresolvable citation must still block PASS")
		}
		if text := diagsText(diags); !strings.Contains(text, "d1") {
			t.Errorf("the unresolvable closure must be named by id; diagnostics:\n%s", text)
		}
	})
}
