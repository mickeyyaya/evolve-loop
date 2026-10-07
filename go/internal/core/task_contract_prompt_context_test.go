package core

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestSeedTaskContract_AUserPhaseThatAsksForItInPromptContextGetsTheBlock(t *testing.T) {
	dir := t.TempDir()
	writeItem(t, dir, "alpha", `{"id":"alpha","title":"Alpha","acceptance":["the refusal path is pinned"]}`)
	ws := contractParityFixture{decision: `{"top_n":[{"id":"alpha"}]}`}.workspace(t)
	asks := phasespec.PhaseSpec{Name: "code-review", Role: "evaluate", Optional: true, PromptContext: []string{CtxKeyTaskContract}}
	silent := phasespec.PhaseSpec{Name: "smell-scan", Role: "evaluate", Optional: true, PromptContext: []string{"goal"}}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithCatalog(mustCatalog(t, asks, silent)),
		WithScopePathResolver(func(_, id string) string { return filepath.Join(dir, id+".json") }))
	o.acsPredicates = func(context.Context, string, int) acsPredicates {
		return acsPredicates{names: []string{"TestC1_001_Fake"}}
	}

	got := o.seedTaskContract(context.Background(), map[string]string{}, Phase("code-review"), CycleState{WorkspacePath: ws}, dir)[CtxKeyTaskContract]
	if !strings.HasPrefix(got, taskContractPreamble) || !strings.Contains(got, "the refusal path is pinned") || !strings.Contains(got, "TestC1_001_Fake") {
		t.Errorf("code-review's block = %q, want the preamble, the acceptance and the predicate inventory", got)
	}
	if other := o.seedTaskContract(context.Background(), map[string]string{}, Phase("smell-scan"), CycleState{WorkspacePath: ws}, dir); other[CtxKeyTaskContract] != "" {
		t.Errorf("a user phase that does not ask got a Task Contract: %q", other[CtxKeyTaskContract])
	}
}
