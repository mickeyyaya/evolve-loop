package dossier

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestCorpusPair_NamesTheFilesAndTheMessageThatACommittedWriteUses(t *testing.T) {
	t.Parallel()
	r := gittest.Fixture(t)
	r.Git("commit", "--allow-empty", "-m", "seed")
	if err := Write(validWriteDossier(), CyclesDir(r.Dir), true); err != nil {
		t.Fatalf("Write: %v", err)
	}
	paths, message := CorpusPair(7)
	want := []string{"knowledge-base/cycles/cycle-7.json", "knowledge-base/cycles/cycle-7.md"}
	if strings.Join(paths, ",") != strings.Join(want, ",") {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
	if subject := strings.TrimSpace(r.Git("log", "-1", "--format=%s")); subject != message {
		t.Fatalf("commit subject %q, CorpusPair message %q; the two must agree", subject, message)
	}
	if tracked := strings.Fields(r.Git("ls-files", "knowledge-base/cycles")); strings.Join(tracked, ",") != strings.Join(want, ",") {
		t.Fatalf("tracked = %v, want %v", tracked, want)
	}
}
