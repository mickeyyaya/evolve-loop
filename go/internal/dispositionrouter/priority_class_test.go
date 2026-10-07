package dispositionrouter_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func TestStageIntent_RefusesAnAutofileWithNoPriorityClass(t *testing.T) {
	dir := t.TempDir()
	_, err := dispositionrouter.StageIntent(dir, dispositionrouter.Intent{Action: dispositionrouter.ActionAutofile, ItemID: "x", Pattern: "p", Weight: 0.75})
	if !errors.Is(err, inboxbatch.ErrNoPriorityClass) {
		t.Fatalf("StageIntent = %v, want inboxbatch.ErrNoPriorityClass", err)
	}
	if _, serr := os.Stat(dispositionrouter.PendingActionsPath(dir)); !os.IsNotExist(serr) {
		t.Errorf("a refused intent stages nothing: %v", serr)
	}
	if _, err := dispositionrouter.StageIntent(filepath.Join(dir, "esc"), dispositionrouter.Intent{Action: dispositionrouter.ActionEscalate, ItemID: "x", Weight: 0.8}); err != nil {
		t.Errorf("an escalate intent bumps a filed item and needs no class: %v", err)
	}
}
