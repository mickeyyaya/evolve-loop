package modelquery

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

// fakeLister returns canned ids per CLI, or an error if the CLI is in errOn.
type fakeLister struct {
	ids   map[string][]string
	errOn map[string]bool
}

func (f fakeLister) List(_ context.Context, cli string) ([]string, error) {
	if f.errOn[cli] {
		return nil, errors.New("boom")
	}
	return f.ids[cli], nil
}

// fakeClassifier maps each id list deterministically: fast=first, deep=last,
// balanced=middle. Errors for CLIs in errOn.
type fakeClassifier struct{ errOn map[string]bool }

func (f fakeClassifier) Classify(_ context.Context, cli string, ids []string) (map[string]string, error) {
	if f.errOn[cli] {
		return nil, errors.New("classify boom")
	}
	out := map[string]string{"fast": ids[0], "deep": ids[len(ids)-1]}
	out["balanced"] = ids[len(ids)/2]
	return out, nil
}

func fixedNow() time.Time { return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC) }

func TestRefreshHappyPath(t *testing.T) {
	deps := RefreshDeps{
		CLIs:       []string{"codex", "ollama"},
		Lister:     fakeLister{ids: map[string][]string{"codex": {"gpt-5.4-mini", "gpt-5.4", "gpt-5.5"}, "ollama": {"a", "b", "c"}}},
		Classifier: fakeClassifier{},
		Now:        fixedNow,
	}
	cat, err := Refresh(context.Background(), deps)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if !cat.FetchedAt.Equal(fixedNow()) {
		t.Fatalf("FetchedAt = %v", cat.FetchedAt)
	}
	if m, ok := cat.Lookup("codex", "deep"); !ok || m != "gpt-5.5" {
		t.Fatalf("codex deep = (%q,%v)", m, ok)
	}
	if m, ok := cat.Lookup("codex", "fast"); !ok || m != "gpt-5.4-mini" {
		t.Fatalf("codex fast = (%q,%v)", m, ok)
	}
	// Available audit trail preserved.
	if got := cat.CLIs["ollama"].Available; len(got) != 3 {
		t.Fatalf("ollama Available = %v, want 3 ids", got)
	}
	// Live-queried entries MUST be dispatch-authoritative (regression guard:
	// a missing Source on the live path would silently make this fail).
	if m, ok := cat.DispatchModel("codex", "deep"); !ok || m != "gpt-5.5" {
		t.Fatalf("live entry must drive dispatch; DispatchModel = (%q,%v)", m, ok)
	}
}

func TestRefreshListFailureFallsBack(t *testing.T) {
	deps := RefreshDeps{
		CLIs:       []string{"codex"},
		Lister:     fakeLister{errOn: map[string]bool{"codex": true}},
		Classifier: fakeClassifier{},
		Fallback:   map[string]map[string]string{"codex": {"balanced": "gpt-5.4-detect"}},
		Now:        fixedNow,
		Log:        io.Discard,
	}
	cat, err := Refresh(context.Background(), deps)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if m, ok := cat.Lookup("codex", "balanced"); !ok || m != "gpt-5.4-detect" {
		t.Fatalf("expected detect fallback, got (%q,%v)", m, ok)
	}
	// Detect fallback is informational, NOT dispatch-authoritative.
	if _, ok := cat.DispatchModel("codex", "balanced"); ok {
		t.Fatal("detect-fallback entry must NOT drive dispatch")
	}
}

func TestRefreshClassifyFailureFallsBack(t *testing.T) {
	deps := RefreshDeps{
		CLIs:       []string{"agy"},
		Lister:     fakeLister{ids: map[string][]string{"agy": {"gemini-x"}}},
		Classifier: fakeClassifier{errOn: map[string]bool{"agy": true}},
		Fallback:   map[string]map[string]string{"agy": {"deep": "gemini-detect"}},
		Now:        fixedNow,
	}
	cat, err := Refresh(context.Background(), deps)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if m, ok := cat.Lookup("agy", "deep"); !ok || m != "gemini-detect" {
		t.Fatalf("expected classify-failure fallback, got (%q,%v)", m, ok)
	}
}

func TestRefreshNoLiveNoFallbackSkips(t *testing.T) {
	deps := RefreshDeps{
		CLIs:       []string{"broken"},
		Lister:     fakeLister{errOn: map[string]bool{"broken": true}},
		Classifier: fakeClassifier{},
		Now:        fixedNow,
	}
	cat, err := Refresh(context.Background(), deps)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if !cat.Empty() {
		t.Fatalf("expected empty catalog (CLI skipped), got %+v", cat.CLIs)
	}
}

func TestRefreshRequiresSeams(t *testing.T) {
	if _, err := Refresh(context.Background(), RefreshDeps{CLIs: []string{"x"}}); err == nil {
		t.Fatal("expected error when Lister/Classifier nil")
	}
}

// TestRefreshEmptyModelListFallsBack covers liveTiers' len(ids)==0 branch: the
// lister succeeds but returns zero models, so the CLI falls back to its detect
// tier map rather than being treated as live.
func TestRefreshEmptyModelListFallsBack(t *testing.T) {
	deps := RefreshDeps{
		CLIs:       []string{"codex"},
		Lister:     fakeLister{ids: map[string][]string{"codex": {}}}, // empty, no error
		Classifier: fakeClassifier{},
		Fallback:   map[string]map[string]string{"codex": {"balanced": "gpt-detect"}},
		Now:        fixedNow,
	}
	cat, err := Refresh(context.Background(), deps)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if m, ok := cat.Lookup("codex", "balanced"); !ok || m != "gpt-detect" {
		t.Fatalf("empty-list CLI must fall back to detect, got (%q,%v)", m, ok)
	}
	// Detect fallback is informational, NOT dispatch-authoritative.
	if _, ok := cat.DispatchModel("codex", "balanced"); ok {
		t.Fatal("empty-list fallback entry must NOT drive dispatch")
	}
}

// TestRefreshDefaultClockWhenNowNil covers the `now = time.Now` default branch
// (Now seam left nil). Asserts FetchedAt is stamped within the invocation
// window — deterministic without coupling to an exact wall-clock value.
func TestRefreshDefaultClockWhenNowNil(t *testing.T) {
	before := time.Now().UTC()
	deps := RefreshDeps{
		CLIs:       []string{"codex"},
		Lister:     fakeLister{ids: map[string][]string{"codex": {"m1", "m2", "m3"}}},
		Classifier: fakeClassifier{},
		// Now intentionally nil → defaults to time.Now.
	}
	cat, err := Refresh(context.Background(), deps)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	after := time.Now().UTC()
	if cat.FetchedAt.Before(before) || cat.FetchedAt.After(after) {
		t.Errorf("FetchedAt=%v not within [%v,%v]", cat.FetchedAt, before, after)
	}
}

func TestRouterRoutes(t *testing.T) {
	ollama := fakeLister{ids: map[string][]string{"ollama": {"phi4"}}}
	deflt := fakeLister{ids: map[string][]string{"codex": {"gpt-5.5"}}}
	r := Router{ByCLI: map[string]Lister{"ollama": ollama}, Default: deflt}

	if ids, _ := r.List(context.Background(), "ollama"); len(ids) != 1 || ids[0] != "phi4" {
		t.Fatalf("ollama route = %v", ids)
	}
	if ids, _ := r.List(context.Background(), "codex"); len(ids) != 1 || ids[0] != "gpt-5.5" {
		t.Fatalf("default route = %v", ids)
	}
	r2 := Router{ByCLI: map[string]Lister{}}
	if _, err := r2.List(context.Background(), "unknown"); err == nil {
		t.Fatal("expected error with no default lister")
	}
}
