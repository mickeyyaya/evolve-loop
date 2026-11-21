package ship

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/derived"
)

var productionDerivedGeneratorFor = derivedGeneratorFor

func init() {
	derivedGeneratorFor = func(string) derived.Generator {
		return func(context.Context, ...string) error { return nil }
	}
}

func TestShip_TheProductionDefaultOfTheGeneratorSeamRunsTheEvolveVerbInTheWorktree(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "no-such-worktree")

	err := productionDerivedGeneratorFor(missing)(context.Background(), "docs", "index", "check")

	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err = %v, want the go run failure of the missing worktree", err)
	}
}

func TestShip_AnOptionsWithoutAnInjectionUsesTheGeneratorSeam(t *testing.T) {
	t.Parallel()
	opts := &Options{}

	if err := opts.derivedGenerator(t.TempDir())(context.Background(), "docs", "index", "check"); err != nil {
		t.Fatalf("the stubbed seam must answer for an Options without an injection, got %v", err)
	}
}
