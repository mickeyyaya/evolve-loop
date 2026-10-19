package evidence_test

import (
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/evidence"
)

func TestExportsAreNamed(t *testing.T) {
	src := evidence.Source{Root: t.TempDir(), Cycle: 1, Commands: []string{"go vet ./..."}}
	if err := evidence.Resolve(src, handoff.Evidence{Kind: "command", Ref: "go vet ./..."}); err != nil {
		t.Fatalf("recorded command: %v", err)
	}
	if err := evidence.Resolve(src, handoff.Evidence{Kind: "command", Ref: "absent"}); !errors.Is(err, evidence.ErrUnresolved) {
		t.Fatalf("absent command = %v, want ErrUnresolved", err)
	}
}
