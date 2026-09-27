package bridge

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

func TestHasTokenResolver_TrueWhenDepsFieldSet(t *testing.T) {
	eng := NewEngine(Deps{
		TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) {
			return tokenusage.Result{}, nil
		},
	})
	if !eng.HasTokenResolver() {
		t.Error("HasTokenResolver() = false, want true for a Deps with TokenResolver set")
	}
}

func TestHasTokenResolver_FalseWhenDepsFieldNil(t *testing.T) {
	eng := NewEngine(Deps{})
	if eng.HasTokenResolver() {
		t.Error("HasTokenResolver() = true, want false for a Deps with no TokenResolver set")
	}
}
