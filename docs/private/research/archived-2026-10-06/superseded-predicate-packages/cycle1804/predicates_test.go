//go:build acs

package cycle1804

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/loopwave"
)

func writeItem(t *testing.T, dir, name string, body map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func allDefaultEdges(items []inboxbatch.Item) []inboxbatch.Edge {
	var edges []inboxbatch.Edge
	for _, r := range inboxbatch.DefaultRules() {
		edges = append(edges, r.Edges(items)...)
	}
	return edges
}

func linked(edges []inboxbatch.Edge, a, b int) bool {
	for _, e := range edges {
		if (e.A == a && e.B == b) || (e.A == b && e.B == a) {
			return true
		}
	}
	return false
}

func TestC1804_001_TruncationNeverSplitsARune(t *testing.T) {
	dir := t.TempDir()
	multibyteTitleStraddlingTheByteLimit := strings.Repeat("世", 60)
	p := writeItem(t, dir, "a.json", map[string]any{
		"id":         "utf8-item",
		"title":      multibyteTitleStraddlingTheByteLimit,
		"files":      []string{strings.Repeat("é", 100)},
		"acceptance": []string{strings.Repeat("世", 400)},
	})
	it, _, err := inboxbatch.LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !utf8.ValidString(it.Title) {
		t.Errorf("title truncated into invalid UTF-8: %q", it.Title)
	}
	if it.Title == "" || len(it.Title) > 160 {
		t.Errorf("title must keep a bounded non-empty prefix, got %d bytes", len(it.Title))
	}
	for _, f := range it.Files {
		if !utf8.ValidString(f) {
			t.Errorf("files entry invalid UTF-8: %q", f)
		}
	}
	for _, a := range it.Acceptance {
		if !utf8.ValidString(a) {
			t.Errorf("acceptance entry invalid UTF-8")
		}
	}
}

func TestC1804_002_AsciiStillTruncatedToTheLimit(t *testing.T) {
	dir := t.TempDir()
	p := writeItem(t, dir, "a.json", map[string]any{"id": "ascii-item", "title": strings.Repeat("a", 500)})
	it, _, err := inboxbatch.LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(it.Title) != 160 {
		t.Errorf("ascii title must truncate to exactly 160 bytes, got %d", len(it.Title))
	}
}

func TestC1804_003_DuplicateIDConnectsToTheFirstItem(t *testing.T) {
	items := []inboxbatch.Item{
		{ID: "dup", Path: "a.json"},
		{ID: "dup", Path: "b.json"},
		{ID: "other", Path: "c.json", ConnectsTo: []string{"dup"}},
	}
	edges := inboxbatch.ConnectsRule{}.Edges(items)
	if !linked(edges, 2, 0) {
		t.Errorf("connects_to a duplicated id must bind the first holder (index 0): %+v", edges)
	}
	if linked(edges, 2, 1) {
		t.Errorf("connects_to must not bind the later duplicate: %+v", edges)
	}
}

func TestC1804_004_RoutedResolverAndConnectsAgreeOnDuplicateID(t *testing.T) {
	dir := t.TempDir()
	writeItem(t, dir, "2026-01-01T00-00-00Z-a.json", map[string]any{"id": "dup", "route": "console-manual"})
	writeItem(t, dir, "2026-01-02T00-00-00Z-b.json", map[string]any{"id": "dup", "route": "lane"})
	items, _, err := inboxbatch.LoadDir(dir)
	if err != nil || len(items) != 2 {
		t.Fatalf("load: %v items=%d", err, len(items))
	}
	first := -1
	for i, it := range items {
		if it.ID == "dup" {
			first = i
			break
		}
	}
	wantRouted, _ := inboxbatch.ConsoleRouted(items[first], nil)
	gotRouted, _ := inboxbatch.RoutedResolver(dir, nil)("dup")
	if gotRouted != wantRouted {
		t.Errorf("RoutedResolver routed=%v, first-wins item routed=%v", gotRouted, wantRouted)
	}
	with := append(items, inboxbatch.Item{ID: "ref", ConnectsTo: []string{"dup"}})
	if !linked(inboxbatch.ConnectsRule{}.Edges(with), len(with)-1, first) {
		t.Errorf("ConnectsRule must resolve dup to the same first item as RoutedResolver")
	}
}

func TestC1804_005_FileAreaTokenizesWrappedAndLocatedEntries(t *testing.T) {
	items := []inboxbatch.Item{
		{ID: "one", Files: []string{"(go/internal/foo/a.go)"}},
		{ID: "two", Files: []string{"go/internal/foo/b.go:178"}},
	}
	if !linked(allDefaultEdges(items), 0, 1) {
		t.Errorf("items sharing go/internal/foo through wrapped/located entries must bind by file area")
	}
}

func TestC1804_006_FileAreaStillSeparatesUnrelatedAreas(t *testing.T) {
	items := []inboxbatch.Item{
		{ID: "one", Files: []string{"(go/internal/foo/a.go)"}},
		{ID: "two", Files: []string{"go/internal/bar/b.go:178"}},
	}
	if linked(allDefaultEdges(items), 0, 1) {
		t.Errorf("different areas must not bind")
	}
}

func TestC1804_007_OperatorStateTokenizesEntries(t *testing.T) {
	stateOnly := inboxbatch.Item{Class: "pipeline-architecture", Files: []string{"(.evolve/state.json)", ".evolve/inbox/;"}}
	if !inboxbatch.IsOperatorState(stateOnly) {
		t.Errorf("wrapped .evolve/ entries are still operator state")
	}
	mixed := inboxbatch.Item{Class: "pipeline-architecture", Files: []string{".evolve/state.json", "(go/internal/core/loop.go)"}}
	if inboxbatch.IsOperatorState(mixed) {
		t.Errorf("a wrapped source path must disqualify operator state")
	}
	lookalike := inboxbatch.Item{Class: "pipeline-architecture", Files: []string{"(.evolvex/state.json)"}}
	if inboxbatch.IsOperatorState(lookalike) {
		t.Errorf(".evolvex/ is not runtime state")
	}
}

func newEngine(t *testing.T) (*loopwave.Engine, string, *bytes.Buffer) {
	t.Helper()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	e := loopwave.New(loopwave.RootsOf(root), loopwave.Ports{}, &stderr)
	return e, inbox, &stderr
}

func TestC1804_008_RoutedResolverWarnsOnMalformedInboxFile(t *testing.T) {
	e, inbox, stderr := newEngine(t)
	writeItem(t, inbox, "lane.json", map[string]any{"id": "lane-item", "files": []string{"go/pkg/x.go"}})
	if err := os.WriteFile(filepath.Join(inbox, "broken-item.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	routed := e.RoutedResolver()
	if ok, _ := routed("lane-item"); ok {
		t.Fatalf("a lane item stays dispatchable")
	}
	out := stderr.String()
	if !strings.Contains(out, "WARN") || !strings.Contains(out, "broken-item.json") {
		t.Errorf("a malformed inbox file must surface a WARN naming it, stderr=%q", out)
	}
}

func TestC1804_009_RoutedResolverStaysQuietOnHealthyInbox(t *testing.T) {
	e, inbox, stderr := newEngine(t)
	writeItem(t, inbox, "lane.json", map[string]any{"id": "lane-item", "files": []string{"go/pkg/x.go"}})
	routed := e.RoutedResolver()
	routed("lane-item")
	routed("unknown")
	if stderr.Len() != 0 {
		t.Errorf("a healthy inbox must emit nothing, stderr=%q", stderr.String())
	}
}
