package ship

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
)

func TestMainCIWired(t *testing.T) {
	t.Parallel()
	if New(Config{}).MainCIWired() {
		t.Fatal("no MainCI configured must report unwired")
	}
	f := func(context.Context, string) (ciwatch.RunStatus, error) { return ciwatch.RunStatus{}, nil }
	if !New(Config{MainCI: f}).MainCIWired() {
		t.Fatal("a configured MainCI must report wired")
	}
}
