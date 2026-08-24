package runner

// agent_doc_missing_wiring_test.go — the runner's REAL load path must wrap a
// nonexistent persona in core.ErrAgentDocMissing, or the optional-skip
// admission one layer up never fires (the week's tenth candidate for a correct
// component nothing calls). Driven through Run with a loader whose FS simply
// lacks the doc — the exact cycle-1551 shape.

import (
	"context"
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

func TestRun_MissingAgentDocCarriesTheTypedSentinel(t *testing.T) {
	b := New(Options{
		Hooks:   &fakeHooks{phase: "defect-disposition-preflight", agent: "evolve-defect-disposition-preflight"},
		Bridge:  &fakeBridge{},
		Prompts: prompts.NewFromFS(fstest.MapFS{}), // no docs at all
	})
	_, err := b.Run(context.Background(), core.PhaseRequest{Cycle: 1551, Workspace: t.TempDir()})
	if err == nil {
		t.Fatalf("a missing persona must still be an error — the SKIP decision belongs to the orchestrator, never the runner")
	}
	if !errors.Is(err, core.ErrAgentDocMissing) {
		t.Fatalf("the error chain must carry core.ErrAgentDocMissing so optionalInfraSkip can admit it; got %v", err)
	}
}

// An unreadable-but-EXISTING doc must NOT carry the sentinel — only genuine
// absence has the take-it-off-the-menu remedy. The path exists here (as a
// directory, so ReadFile fails with a non-NotExist error): classifying THAT as
// "missing" would let optionalInfraSkip absorb corrupt personas and I/O faults.
func TestRun_UnreadableAgentDocDoesNotCarryTheSentinel(t *testing.T) {
	b := New(Options{
		Hooks:  &fakeHooks{phase: "p", agent: "evolve-p"},
		Bridge: &fakeBridge{},
		Prompts: prompts.NewFromFS(fstest.MapFS{
			"agents/evolve-p.md": &fstest.MapFile{Mode: fs.ModeDir},
		}),
	})
	_, err := b.Run(context.Background(), core.PhaseRequest{Cycle: 1, Workspace: t.TempDir()})
	if err == nil {
		t.Fatalf("reading a directory as the agent doc must fail")
	}
	if errors.Is(err, core.ErrAgentDocMissing) {
		t.Fatalf("a present-but-unreadable doc is a different defect and must not classify as missing: %v", err)
	}
}

// A NIL prompts source fails every load with fs.ErrNotExist (the documented
// zero-loader contract) — but that is a WIRING defect, not a missing doc, and
// must NOT classify as skippable: otherwise a misresolved prompts root turns
// every optional phase in the cycle into a silent WARN-skip.
func TestRun_NilPromptsSourceDoesNotCarryTheSentinel(t *testing.T) {
	b := New(Options{
		Hooks:   &fakeHooks{phase: "p", agent: "evolve-p"},
		Bridge:  &fakeBridge{},
		Prompts: prompts.NewFromFS(nil),
	})
	_, err := b.Run(context.Background(), core.PhaseRequest{Cycle: 1, Workspace: t.TempDir()})
	if err == nil {
		t.Fatalf("nil prompts source must still be an error")
	}
	if errors.Is(err, core.ErrAgentDocMissing) {
		t.Fatalf("a nil prompts source is a wiring defect, not a missing doc — must not classify as skippable: %v", err)
	}
}
