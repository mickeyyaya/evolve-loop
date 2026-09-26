package ship

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// readSeededAuditEntry returns the single ledger row seedAudit wrote.
func readSeededAuditEntry(t *testing.T, repo string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repo, ".evolve", "ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var entry map[string]any
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatalf("seeded ledger row: %v", err)
	}
	if tree, _ := entry["worktree_tree_sha"].(string); tree == "" {
		t.Fatal("seedAudit wrote no worktree_tree_sha")
	}
	return entry
}

// rewriteAuditWithReportComment rewrites the seeded report to carry an
// `audit_bound_tree_sha:` line naming commentTree and rebinds the ledger row to
// ledgerTree. Evidence is re-sealed against the seeded tree and the artifact
// re-hashed, so only the ledger binding differs from a clean seeded audit.
func rewriteAuditWithReportComment(t *testing.T, repo, commentTree, ledgerTree string) {
	t.Helper()
	seeded := readSeededAuditEntry(t, repo)
	auditPath, _ := seeded["artifact_path"].(string)
	cycle, _ := seeded["cycle"].(float64)
	runID, _ := seeded["run_id"].(string)
	seededTree, _ := seeded["worktree_tree_sha"].(string)

	mustWrite(t, auditPath, "<!-- challenge-token: testtoken123 -->\n# Audit Report — Cycle 1\n\nVerdict: PASS\n\n"+
		"audit_bound_tree_sha: `"+commentTree+"`\n")
	sealTestPredicateEvidence(t, acssuite.EvidenceIdentity{Cycle: int(cycle), RunID: runID, Round: 1, TreeSHA: seededTree}, auditPath)

	rebound := make(map[string]any, len(seeded))
	for k, v := range seeded {
		rebound[k] = v
	}
	rebound["artifact_sha256"] = mustHashFile(t, auditPath)
	rebound["worktree_tree_sha"] = ledgerTree
	line, err := json.Marshal(rebound)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(repo, ".evolve", "ledger.jsonl"), string(line)+"\n")
}

// The report comment carries the correct tree here — the only shape in which
// a report-sourced binding could look trustworthy.
func TestVerifyAuditBinding_ReportCommentNeverBindsTree(t *testing.T) {
	repo := makeRepo(t)
	seedAudit(t, repo, "PASS")
	seededTree, _ := readSeededAuditEntry(t, repo)["worktree_tree_sha"].(string)
	rewriteAuditWithReportComment(t, repo, seededTree, "")
	opts := auditOpts(t, repo)

	_ = verifyAuditBinding(context.Background(), opts, &RunResult{})

	if opts.internalAuditBoundTreeSHA != "" {
		t.Fatalf("audit-report comment bound the tree to %q; only the ledger's worktree_tree_sha may bind it",
			opts.internalAuditBoundTreeSHA)
	}
}

func TestVerifyAuditBinding_EmptyLedgerTreeRefusedBeforeConsumption(t *testing.T) {
	repo := makeRepo(t)
	seedAudit(t, repo, "PASS")
	seededTree, _ := readSeededAuditEntry(t, repo)["worktree_tree_sha"].(string)
	rewriteAuditWithReportComment(t, repo, seededTree, "")

	err := verifyAuditBinding(context.Background(), auditOpts(t, repo), &RunResult{})

	wantShipErr(t, err, core.CodeAuditBindingMalformed, core.ShipClassPrecondition, "predicate evidence invalid")
}

func TestVerifyAuditBinding_LedgerTreeWinsOverReportComment(t *testing.T) {
	repo := makeRepo(t)
	seedAudit(t, repo, "PASS")
	seededTree, _ := readSeededAuditEntry(t, repo)["worktree_tree_sha"].(string)
	staleTree := strings.Repeat("0", 40)
	rewriteAuditWithReportComment(t, repo, staleTree, seededTree)
	opts := auditOpts(t, repo)

	if err := verifyAuditBinding(context.Background(), opts, &RunResult{}); err != nil {
		t.Fatalf("a ledger-bound audit must pass regardless of the report comment: %v", err)
	}
	if opts.internalAuditBoundTreeSHA != seededTree {
		t.Fatalf("internalAuditBoundTreeSHA = %q, want the ledger's worktree_tree_sha %q",
			opts.internalAuditBoundTreeSHA, seededTree)
	}
}

// The scan targets the report-comment form (`audit_bound_tree_sha:`), not the
// ship-binding.json key "audit_bound_tree_sha", which production legitimately reads.
func TestShipSource_NoAuditReportTreeCommentParser(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	fset := token.NewFileSet()
	var offenders []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING && strings.Contains(lit.Value, "audit_bound_tree_sha:") {
				offenders = append(offenders, fset.Position(lit.Pos()).String())
			}
			return true
		})
	}
	if len(offenders) > 0 {
		t.Errorf("ship source still parses the audit-report `audit_bound_tree_sha:` comment at %v; the ledger's worktree_tree_sha is the only binding source",
			offenders)
	}
}
