package inboxcurator

import (
	"reflect"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func item(id, title string, files []string, acceptance ...string) inboxbatch.Item {
	return inboxbatch.Item{ID: id, Title: title, Files: files, Acceptance: acceptance}
}

func TestSimilarity_IsTheTokenJaccardOfTitleAndAcceptance(t *testing.T) {
	a := item("a", "Wave runs four parallel lanes", nil, "TestQuotaAwareCount passes")
	b := item("b", "A wave runs four lanes in parallel", nil, "TestQuotaAwareCount passes")
	c := item("c", "Comment stripping for the docs tree", nil, "ste-lint is clean")

	if got := Similarity(a, a); got != 1 {
		t.Errorf("Similarity(a, a) = %v, want 1", got)
	}
	if got := Similarity(a, b); got != 1 {
		t.Errorf("Similarity of a reworded title = %v, want 1 (case, order and stop words do not count)", got)
	}
	if got := Similarity(a, c); got != 0 {
		t.Errorf("Similarity of unrelated items = %v, want 0", got)
	}
	half := item("h", "wave lanes", nil)
	if got := Similarity(item("x", "wave lanes four parallel", nil), half); got != 0.5 {
		t.Errorf("Similarity of a half overlap = %v, want 0.5", got)
	}
	if got := Similarity(item("e", "", nil), item("f", "", nil)); got != 0 {
		t.Errorf("Similarity of two items with no tokens = %v, want 0", got)
	}
}

func TestSharedSurface_MatchesEqualPathsAndADirectoryThatHoldsAPath(t *testing.T) {
	a := item("a", "", []string{"go/internal/fleet/pool.go", "go/internal/loopwave/"})
	b := item("b", "", []string{"go/internal/fleet/pool.go:12", "go/internal/loopwave/plan.go", "docs/x.md"})
	c := item("c", "", []string{"go/internal/fleetx/pool.go"})

	got := SharedSurface(a, b)
	if want := []string{"go/internal/fleet/pool.go", "go/internal/loopwave/plan.go"}; !slices.Equal(got, want) {
		t.Errorf("SharedSurface = %v, want %v", got, want)
	}
	if got := SharedSurface(b, a); !slices.Equal(got, []string{"go/internal/fleet/pool.go", "go/internal/loopwave/plan.go"}) {
		t.Errorf("SharedSurface is symmetric, got %v", got)
	}
	if got := SharedSurface(a, c); len(got) != 0 {
		t.Errorf("a sibling directory with a common prefix is not shared, got %v", got)
	}
	if got := SharedSurface(item("n", "", nil), b); len(got) != 0 {
		t.Errorf("an item with no declared surface shares nothing, got %v", got)
	}
}

func TestJudgeOverlap_RefusesAboveTheRefuseThresholdAndWarnsBetween(t *testing.T) {
	files := []string{"go/internal/fleet/pool.go"}
	candidate := item("new", "A free lane takes the next ready task at once", files, "TestOrchestrator_AFreeSlot passes")
	open := []inboxbatch.Item{
		item("dup", "A free lane takes the next ready task", files, "TestOrchestrator_AFreeSlot passes"),
		item("near", "Lane pool records the ready task", files, "TestPool passes"),
		item("other-surface", "A free lane takes the next ready task at once", []string{"docs/a.md"}, "TestOrchestrator_AFreeSlot passes"),
		item("far", "Rename the pool type", files, "go vet is clean"),
		item("new", "A free lane takes the next ready task at once", files, "TestOrchestrator_AFreeSlot passes"),
	}

	v := JudgeOverlap(candidate, open, Thresholds{Refuse: 0.6, Warn: 0.15})

	if ids := overlapIDs(v.Refused); !slices.Equal(ids, []string{"dup"}) {
		t.Errorf("refused = %v, want [dup]", ids)
	}
	if ids := overlapIDs(v.Warned); !slices.Equal(ids, []string{"near"}) {
		t.Errorf("warned = %v, want [near]", ids)
	}
	if v.Refused[0].Similarity < 0.6 || !slices.Equal(v.Refused[0].Surface, files) {
		t.Errorf("the refusal names its similarity and surface, got %+v", v.Refused[0])
	}
}

func TestJudgeOverlap_OrdersTheMostSimilarFirst(t *testing.T) {
	files := []string{"go/a/b.go"}
	candidate := item("new", "alpha beta gamma delta", files)
	open := []inboxbatch.Item{
		item("half", "alpha beta", files),
		item("most", "alpha beta gamma", files),
	}

	v := JudgeOverlap(candidate, open, Thresholds{Refuse: 0.4, Warn: 0.1})

	if ids := overlapIDs(v.Refused); !slices.Equal(ids, []string{"most", "half"}) {
		t.Errorf("refused = %v, want the most similar first", ids)
	}
}

func overlapIDs(overlaps []Overlap) []string {
	ids := make([]string, 0, len(overlaps))
	for _, o := range overlaps {
		ids = append(ids, o.ID)
	}
	return ids
}

func TestJudgeOverlap_ADeclaredRelationIsNotAnOverlap(t *testing.T) {
	files := []string{"go/a/b.go"}
	candidate := item("new", "alpha beta gamma", files)
	candidate.Deps = []string{"dep"}
	candidate.ConnectsTo = []string{"linked"}
	open := []inboxbatch.Item{item("dep", "alpha beta gamma", files), item("linked", "alpha beta gamma", files), item("dup", "alpha beta gamma", files)}

	v := JudgeOverlap(candidate, open, Thresholds{Refuse: 0.5, Warn: 0.1})

	if ids := overlapIDs(v.Refused); !slices.Equal(ids, []string{"dup"}) {
		t.Errorf("refused = %v, want only the undeclared dup", ids)
	}
}

func TestJudgeOverlap_NoSharedSurfaceIsAnEmptyVerdict(t *testing.T) {
	got := JudgeOverlap(item("new", "alpha", []string{"go/a.go"}), []inboxbatch.Item{item("o", "alpha", []string{"go/b.go"})}, Thresholds{Refuse: 0.5, Warn: 0.1})

	if got.Refused != nil || got.Warned != nil || !reflect.DeepEqual(got, OverlapVerdict{}) {
		t.Errorf("JudgeOverlap = %+v, want an empty OverlapVerdict", got)
	}
}

func TestSharedSurface_CleansEachPathBeforeItCompares(t *testing.T) {
	a := item("a", "", []string{"./go/x/a.go"})
	b := item("b", "", []string{"go/x/./a.go"})

	if got := SharedSurface(a, b); !slices.Equal(got, []string{"go/x/a.go"}) {
		t.Errorf("SharedSurface = %v, want [go/x/a.go]", got)
	}
}

func TestJudgeOverlap_ASimilarityEqualToTheThresholdIsRefused(t *testing.T) {
	files := []string{"go/a.go"}
	v := JudgeOverlap(item("new", "alpha beta", files), []inboxbatch.Item{item("at", "alpha", files), item("below", "gamma", files)}, Thresholds{Refuse: 0.5, Warn: 0.5})

	if ids := overlapIDs(v.Refused); !slices.Equal(ids, []string{"at"}) || len(v.Warned) != 0 {
		t.Errorf("refused = %v warned = %v, want [at] refused at exactly 0.5", ids, overlapIDs(v.Warned))
	}
}
