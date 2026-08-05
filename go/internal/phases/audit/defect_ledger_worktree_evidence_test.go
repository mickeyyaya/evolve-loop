package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// defect_ledger_worktree_evidence_test.go — RED contract for cycle-1340
// `defect-ledger-worktree-evidence-fallback` (the sole top_n card).
//
// The defect this pins (scout Finding 1): evidenceResolves
// (defect_ledger.go:253-300) resolves every closure citation with a single
// os.Lstat under req.ProjectRoot. A continuation LANE's own fix lives in its
// own still-open worktree and reaches the project root only when the lane
// merges — which is precisely what this gate blocks. Cycles 1320 → 1323 →
// 1325 → 1330 each cited the same two real, worktree-resident files and each
// was rejected with the identical "resolves to no file under the project
// root" message: the gate demands evidence it structurally prevents from
// existing. P0, cycles_unpicked=5+.
//
// The fix (Task 1): when the project-root Lstat misses AND req.Worktree != "",
// retry under req.Worktree — the SHIPPED-TREE root already threaded to every
// phase (core/phase.go:81-91). Every existing rejection stays: absolute paths,
// escapes, non-regular files, and the self-citation guard (rule 4) must reject
// worktree-resident citations exactly as they reject project-root ones, or the
// fallback reopens the self-vouching hole cycle-1285 F3 closed.
//
// Every assertion below reaches its subject through the REAL production seam,
// hooks{}.Classify — the audit phase's verdict path. evidenceResolves is
// unexported and calling it directly would pass on dead code.
//
// Adversarial diversity: positive (worktree-only evidence closes a defect),
// negative (absent from BOTH roots still blocks; self-citation still rejected
// under the new root), edge (empty Worktree unchanged; escape/absolute still
// rejected with a worktree set), semantic (four distinct gate behaviors, not
// one restated).

// worktreeContinuationFixture extends continuationFixture with a populated
// req.Worktree — the lane's own SHIPPED-TREE root, distinct from ProjectRoot,
// which is exactly the 1320→1330 shape (fix committed in the lane's worktree,
// not yet merged to the project root).
func worktreeContinuationFixture(t *testing.T, ancestorCycle, thisCycle int, openDefects []string) (string, string, core.PhaseRequest) {
	t.Helper()
	ws, req := continuationFixture(t, ancestorCycle, thisCycle, openDefects)
	wt := t.TempDir()
	req.Worktree = wt
	return ws, wt, req
}

// TestClassify_WorktreeResidentEvidenceClosesADefect — POSITIVE, the P0 repro.
// The lane cites go/cmd/evolve/cmd_loop_chain_boundaryrefresh_shortsha_test.go
// (cycle-1323's actual citation). The file exists in the lane's worktree and
// NOT under the project root, because the merge that would put it there is
// what this gate is blocking. Today: rejected, cycle cannot PASS, forever.
func TestClassify_WorktreeResidentEvidenceClosesADefect(t *testing.T) {
	ws, wt, req := worktreeContinuationFixture(t, 1330, 1340, []string{"boundary refresh does not repin the short sha"})
	// Materialize the citation in the WORKTREE only — never under ProjectRoot.
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

// TestClassify_EvidenceAbsentFromBothRootsStillBlocks — NEGATIVE, the
// anti-no-op. The fallback widens WHERE a real file may live; it must never
// weaken the requirement that one exist. Deleting the Lstat entirely would
// pass the positive test above and fail this one.
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

// TestClassify_WorktreeSelfCitationStillRejected — NEGATIVE, the hole the
// fallback could reopen. The gate's own bookkeeping is rejected by basename
// (rule 4, EqualFold, cycle-1285 F3). A lane may not evade that by planting
// defect-ledger.json in its worktree instead of the project root. The graded
// agent WRITES its own worktree, so this is the cheapest bypass of the fix.
func TestClassify_WorktreeSelfCitationStillRejected(t *testing.T) {
	ws, wt, req := worktreeContinuationFixture(t, 1330, 1340, []string{"boundary refresh does not repin the short sha"})
	// A real, regular, worktree-resident file — it fails ONLY on rule 4.
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

// TestClassify_WorktreeEvidenceCannotEscapeRoot — EDGE / security. The
// absolute-path and ".." rejections run BEFORE either Lstat and must stay
// there: a worktree root must not become a new pivot for reaching outside the
// tree. The escape target is materialized so the test fails for the right
// reason (rejected by rule, not by absence).
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

// TestClassify_LineRangeCitationResolves — the SECOND live instance of the
// same deadlock, found while reproducing it. Defect ddda7857a (inherited by
// this lane from 1325) cites "go/cmd/evolve/cmd_loop_chain.go:570-588": a real
// file, present under BOTH roots, rejected anyway. The suffix stripper takes
// only ":<digits>", so a ":<line>-<line>" RANGE stays glued to the path and no
// Lstat can ever succeed. The worktree fallback alone does not close the P0 —
// this citation misses under both roots for a reason the fallback cannot fix.
//
// Ranges are the house citation style (build/audit reports cite them
// everywhere), so this is the common case, not an exotic one.
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

// TestClassify_NonLocatorSuffixIsPartOfThePath — the anti-over-strip guard for
// the test above. Only a numeric locator may be shaved off; a colon followed by
// anything else is part of the filename and must NOT be discarded to make a
// different, existing file satisfy the claim.
func TestClassify_NonLocatorSuffixIsPartOfThePath(t *testing.T) {
	ws, _, req := worktreeContinuationFixture(t, 1330, 1340, []string{"brake resolved twice"})
	// The would-be over-strip target EXISTS; only the cited name does not.
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

// TestClassify_ProjectRootEvidencePathUnchanged — REGRESSION. Two shapes that
// must behave exactly as they do today: (a) evidence under the project root
// still closes a defect when a worktree is also set (the fallback must be a
// FALLBACK, not a replacement), and (b) with req.Worktree empty — the
// provisioning-failed case — a missing citation still blocks rather than
// panicking or degrading open on the empty root.
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
