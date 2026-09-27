package ship

// consume_unified_test.go — a VALIDATED unified_commitment closes all-or-nothing
// at the in-commit consumption seam (triage-unified-solution-synthesis: members
// close TRANSACTIONALLY with landing). The triage phase stamps
// unified_projection only beside a claim it validated (triage/unified.go), so the
// projection — never the raw claim — marks the member set as one atomic unit.
// Every other id keeps consume.go's per-item fail-open contract.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

const (
	unifiedMemberA   = "unified-member-a"
	unifiedMemberB   = "unified-member-b"
	independentSolo  = "independent-solo"
	corruptItemBytes = "{not valid json"
)

// unifiedDecision renders a triage-decision.json committing topN. A non-empty
// members list is declared as a unified_commitment; projected adds the
// unified_projection the triage phase writes only for a claim it validated.
func unifiedDecision(t *testing.T, topN, members []string, projected bool) string {
	t.Helper()
	top := make([]map[string]string, 0, len(topN))
	for _, id := range topN {
		top = append(top, map[string]string{"id": id})
	}
	doc := map[string]any{"schema_version": 1, "top_n": top, "deferred": []any{}, "dropped": []any{}}
	if len(members) > 0 {
		declared := make([]map[string]string, 0, len(members))
		for _, id := range members {
			declared = append(declared, map[string]string{"id": id, "evidence": id + " is one copy of the shared seam gap"})
		}
		doc["unified_commitment"] = map[string]any{
			"root_cause_hypothesis": "every member is one copy of the same seam gap",
			"shared_seam":           "go/internal/phases/ship/consume.go",
			"design_requirements":   []string{"members close transactionally"},
			"members":               declared,
		}
		if projected {
			doc["unified_projection"] = map[string]any{"size": "small", "member_count": len(members)}
		}
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// unifiedConsumeFixture lays out a project root whose inbox holds one file per
// id — present but unparseable for every id in corrupt, the shape the lookup
// can never resolve — and a workspace carrying decision plus a PASS verdict.
// It returns each id's inbox basename and original bytes.
func unifiedConsumeFixture(t *testing.T, decision string, ids []string, corrupt ...string) (root, workspace string, base, orig map[string]string) {
	t.Helper()
	root = t.TempDir()
	workspace = filepath.Join(root, ".evolve", "runs", "cycle-1720")
	bad := map[string]bool{}
	for _, id := range corrupt {
		bad[id] = true
	}
	base, orig = map[string]string{}, map[string]string{}
	for i, id := range ids {
		base[id] = fmt.Sprintf("2026-09-27T00-00-%02dZ-%s.json", i, id)
		orig[id] = fmt.Sprintf(`{"id":%q,"title":"fixture %s","weight":0.5}`, id, id)
		if bad[id] {
			orig[id] = corruptItemBytes
		}
		mustWrite(t, filepath.Join(root, ".evolve", "inbox", base[id]), orig[id])
	}
	mustWrite(t, filepath.Join(workspace, "triage-decision.json"), decision)
	mustWrite(t, filepath.Join(workspace, "acs-verdict.json"), `{"verdict":"PASS","red_count":0}`)
	return root, workspace, base, orig
}

// failStageFor accepts every command except the `git add` that stages
// itemBase — a stage fault (index lock, permission) that lands only after an
// earlier sibling has already been moved and staged.
func failStageFor(itemBase string) CmdRunner {
	return func(_ context.Context, name, _ string, args []string, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) (int, error) {
		if name != "git" || !containsArg(args, "add") {
			return 0, nil
		}
		for _, a := range args {
			if strings.HasSuffix(a, itemBase) {
				return 1, nil
			}
		}
		return 0, nil
	}
}

func containsArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func runUnifiedConsume(root, workspace string, runner CmdRunner) (*Options, *RunResult) {
	opts := &Options{Class: ClassCycle, ProjectRoot: root, WorkspacePath: workspace, Runner: runner}
	res := &RunResult{}
	consumeCommittedItems(context.Background(), opts, res, "")
	return opts, res
}

// assertPickable fails unless id's item is back at the inbox root with its
// original bytes, has no consumed/ record, and is not in the ship's drift
// sanction set.
func assertPickable(t *testing.T, opts *Options, res *RunResult, root, id, base, orig string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, ".evolve", "inbox", base))
	if err != nil || string(raw) != orig {
		t.Errorf("%s must stay pickable at the inbox root with its original bytes (err=%v got=%q want=%q)\nlogs:\n%s",
			id, err, raw, orig, strings.Join(res.Logs, "\n"))
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "inbox", "consumed", base)); !os.IsNotExist(err) {
		t.Errorf("%s must have no consumed/ record — a split closeout of the unified commitment (stat err=%v)\nlogs:\n%s",
			id, err, strings.Join(res.Logs, "\n"))
	}
	if sanctionsBase(opts, base) {
		t.Errorf("%s was rolled back but its paths are still in the drift sanction set %v", id, opts.internalConsumedPaths)
	}
}

func assertConsumed(t *testing.T, opts *Options, res *RunResult, root, id, base string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, ".evolve", "inbox", base)); !os.IsNotExist(err) {
		t.Errorf("%s must leave the inbox root (stat err=%v)\nlogs:\n%s", id, err, strings.Join(res.Logs, "\n"))
	}
	raw, err := os.ReadFile(filepath.Join(root, ".evolve", "inbox", "consumed", base))
	if err != nil {
		t.Fatalf("%s must have a consumed/ record: %v\nlogs:\n%s", id, err, strings.Join(res.Logs, "\n"))
	}
	var doc map[string]any
	if json.Unmarshal(raw, &doc) != nil || doc["consumed"] == nil {
		t.Errorf("%s consumed record must carry the consumption annotation: %s", id, raw)
	}
	if !sanctionsBase(opts, base) {
		t.Errorf("%s was consumed but its move is missing from the drift sanction set %v", id, opts.internalConsumedPaths)
	}
}

func sanctionsBase(opts *Options, base string) bool {
	for _, p := range opts.internalConsumedPaths {
		if strings.HasSuffix(p, "/"+base) {
			return true
		}
	}
	return false
}

// unifiedWarn reports whether one ship WARN names the unified commitment and
// the member that could not close.
func unifiedWarn(res *RunResult, failing string) bool {
	for _, l := range res.Logs {
		if strings.Contains(l, "WARN") && strings.Contains(l, "unified_commitment") && strings.Contains(l, failing) {
			return true
		}
	}
	return false
}

func TestConsumeCommittedItems_UnifiedRollsBackAllOnPartialFailure(t *testing.T) {
	members := []string{unifiedMemberA, unifiedMemberB}

	t.Run("stage failure after a sibling staged rolls the sibling back", func(t *testing.T) {
		root, ws, base, orig := unifiedConsumeFixture(t, unifiedDecision(t, members, members, true), members)
		bound := continuation.Continuation{
			Worktree:    "/tmp/evolve/worktrees/cycle-1719",
			Branch:      "cycle-1719",
			SnapshotSHA: "1719aaaabbbbccccddddeeeeffff000011112222",
			BaseSHA:     "4b3fcead00112233445566778899aabbccddeeff",
			Cycle:       1719,
		}
		if err := continuation.WriteRegistryEntry(root, unifiedMemberA, bound); err != nil {
			t.Fatalf("fixture bind: %v", err)
		}
		opts, res := runUnifiedConsume(root, ws, failStageFor(base[unifiedMemberB]))
		for _, id := range members {
			assertPickable(t, opts, res, root, id, base[id], orig[id])
		}
		// A rolled-back member leaves BOTH stores as they were: releasing its
		// continuation binding would strand the salvage pointer of an item that
		// is still pickable.
		if _, ok, err := continuation.ReadRegistryEntry(root, unifiedMemberA); err != nil || !ok {
			t.Errorf("rolled-back member %q lost its continuation binding (ok=%v err=%v)", unifiedMemberA, ok, err)
		}
		if !unifiedWarn(res, unifiedMemberB) {
			t.Errorf("expected one WARN naming unified_commitment and the failing member %q; logs:\n%s", unifiedMemberB, strings.Join(res.Logs, "\n"))
		}
	})

	t.Run("unresolvable member rolls every sibling back", func(t *testing.T) {
		root, ws, base, orig := unifiedConsumeFixture(t, unifiedDecision(t, members, members, true), members, unifiedMemberB)
		opts, res := runUnifiedConsume(root, ws, stubRunner)
		for _, id := range members {
			assertPickable(t, opts, res, root, id, base[id], orig[id])
		}
		if !unifiedWarn(res, unifiedMemberB) {
			t.Errorf("an unresolvable member must be LOUD, not the silent not-found no-op: expected a WARN naming unified_commitment and %q; logs:\n%s", unifiedMemberB, strings.Join(res.Logs, "\n"))
		}
	})

	t.Run("failing member first leaves later siblings unconsumed", func(t *testing.T) {
		order := []string{unifiedMemberB, unifiedMemberA}
		root, ws, base, orig := unifiedConsumeFixture(t, unifiedDecision(t, order, order, true), order, unifiedMemberB)
		opts, res := runUnifiedConsume(root, ws, stubRunner)
		for _, id := range order {
			assertPickable(t, opts, res, root, id, base[id], orig[id])
		}
		if !unifiedWarn(res, unifiedMemberB) {
			t.Errorf("expected a WARN naming unified_commitment and %q; logs:\n%s", unifiedMemberB, strings.Join(res.Logs, "\n"))
		}
	})
}

func TestConsumeCommittedItems_UnifiedAllSucceed(t *testing.T) {
	members := []string{unifiedMemberA, unifiedMemberB}
	root, ws, base, _ := unifiedConsumeFixture(t, unifiedDecision(t, members, members, true), members)
	opts, res := runUnifiedConsume(root, ws, stubRunner)
	for _, id := range members {
		assertConsumed(t, opts, res, root, id, base[id])
	}
	for _, l := range res.Logs {
		if strings.Contains(l, "WARN") && strings.Contains(l, "unified_commitment") {
			t.Errorf("a commitment whose members all close must not warn: %q", l)
		}
	}
}

// The atomic unit is exactly the validated member set: an ordinary batch, a
// claim the triage phase rejected, and an independent item beside a commitment
// all keep the per-item fail-open contract.
func TestConsumeCommittedItems_UnifiedScopeIsOnlyTheValidatedMembers(t *testing.T) {
	pair := []string{unifiedMemberA, unifiedMemberB}

	t.Run("an ordinary top_n batch stays per-item fail-open", func(t *testing.T) {
		root, ws, base, orig := unifiedConsumeFixture(t, unifiedDecision(t, pair, nil, false), pair, unifiedMemberB)
		opts, res := runUnifiedConsume(root, ws, stubRunner)
		assertConsumed(t, opts, res, root, unifiedMemberA, base[unifiedMemberA])
		assertPickable(t, opts, res, root, unifiedMemberB, base[unifiedMemberB], orig[unifiedMemberB])
	})

	t.Run("a claim triage rejected carries no projection and stays per-item", func(t *testing.T) {
		root, ws, base, orig := unifiedConsumeFixture(t, unifiedDecision(t, pair, pair, false), pair, unifiedMemberB)
		opts, res := runUnifiedConsume(root, ws, stubRunner)
		assertConsumed(t, opts, res, root, unifiedMemberA, base[unifiedMemberA])
		assertPickable(t, opts, res, root, unifiedMemberB, base[unifiedMemberB], orig[unifiedMemberB])
	})

	t.Run("an independent item still consumes beside a rolled-back commitment", func(t *testing.T) {
		all := []string{unifiedMemberA, unifiedMemberB, independentSolo}
		root, ws, base, orig := unifiedConsumeFixture(t, unifiedDecision(t, all, pair, true), all, unifiedMemberB)
		opts, res := runUnifiedConsume(root, ws, stubRunner)
		for _, id := range pair {
			assertPickable(t, opts, res, root, id, base[id], orig[id])
		}
		assertConsumed(t, opts, res, root, independentSolo, base[independentSolo])
	})
}
