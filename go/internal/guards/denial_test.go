package guards

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const deniedDispatch = "run-41/7/build/p4242n1t9"

func TestRecordDenial_EmitsGuardDeniedWithTheRunIDForThatDispatch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	RecordDenial(root, deniedDispatch, Denial{Guard: "role", Path: "/repo/x.go", Reason: "role guard: outside workspace"})

	chunk, err := signalcenter.ReadStream(DenialStream(root, 7), 0)
	if err != nil || len(chunk.Events) != 1 {
		t.Fatalf("events %+v err %v: one GUARD_DENIED in the run stream", chunk.Events, err)
	}
	e := chunk.Events[0]
	if e.Code != CodeDenied || e.RunID != "run-41" || e.Cycle != 7 || e.Phase != "build" || e.Module != signalcenter.ModuleGuard {
		t.Fatalf("event %+v: GUARD_DENIED with the run id, cycle and phase of the dispatch", e)
	}
	if e.Fields["dispatch_id"] != deniedDispatch || e.Fields["guard"] != "role" || e.Fields["path"] != "/repo/x.go" {
		t.Fatalf("fields %v", e.Fields)
	}
	if mod, ok := signalcenter.IsRegistered(CodeDenied); !ok || mod != signalcenter.ModuleGuard {
		t.Fatalf("GUARD_DENIED must be registered to the guard module: %v %v", mod, ok)
	}
}

func TestDenialOf_ReadsBackOnlyTheDenialOfThatDispatch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	RecordDenial(root, "run-41/7/build/p4242n2t9", Denial{Guard: "ship", Path: "git push", Reason: "other dispatch"})
	RecordDenial(root, deniedDispatch, Denial{Guard: "role", Path: "/repo/x.go", Reason: "outside workspace"})

	got, found := DenialOf(root, deniedDispatch)

	if !found || got != (Denial{Guard: "role", Path: "/repo/x.go", Reason: "outside workspace"}) {
		t.Fatalf("got %+v found %v", got, found)
	}
	if got.Summary() != "guard role denied /repo/x.go: outside workspace" {
		t.Fatalf("summary %q", got.Summary())
	}
	if _, found := DenialOf(root, "run-41/7/build/p4242n3t9"); found {
		t.Fatal("a dispatch with no denial must read none")
	}
}

func TestDenialOf_NoCellOrNoStreamReadsNone(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	RecordDenial(root, "", Denial{Guard: "role"})
	if _, err := os.Stat(DenialStream(root, 0)); !os.IsNotExist(err) {
		t.Fatalf("a denial outside a cell records nothing: %v", err)
	}
	if _, found := DenialOf(root, ""); found {
		t.Fatal("no dispatch id reads no denial")
	}
	if _, found := DenialOf(root, deniedDispatch); found {
		t.Fatal("no stream reads no denial")
	}
}

func TestDenialStream_ACycleWritesTheRunStreamAndNoCycleTheRootStream(t *testing.T) {
	t.Parallel()
	if got := DenialStream("/p", 7); got != filepath.Join(paths.RunWorkspace("/p", 7), signalcenter.StreamFileName) {
		t.Fatalf("cycle stream %q", got)
	}
	if got := DenialStream("/p", 0); got != filepath.Join(paths.EvolveDirOf("/p"), signalcenter.StreamFileName) {
		t.Fatalf("root stream %q", got)
	}
}

func TestDeniedPath_NamesTheFileTheCommandOrThePersona(t *testing.T) {
	t.Parallel()
	for want, in := range map[string]core.GuardInput{
		"/repo/x.go":   {ToolName: "Write", ToolInput: map[string]any{"file_path": "/repo/x.go"}},
		"git push":     {ToolName: "Bash", ToolInput: map[string]any{"command": "git push"}},
		"evolve-scout": {ToolName: "Agent", ToolInput: map[string]any{"subagent_type": "evolve-scout"}},
		"":             {ToolName: "Read"},
	} {
		if got := DeniedPath(in); got != want {
			t.Errorf("DeniedPath(%+v) = %q, want %q", in, got, want)
		}
	}
}
