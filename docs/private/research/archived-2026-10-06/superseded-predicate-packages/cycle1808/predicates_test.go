//go:build acs

package cycle1808

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

func TestC1808_001_TruncationNeverSplitsARune(t *testing.T) {
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

func TestC1808_002_AsciiStillTruncatedToTheLimit(t *testing.T) {
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

func TestC1808_003_DuplicateIDConnectsToTheFirstItem(t *testing.T) {
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

func TestC1808_004_RoutedResolverAndConnectsAgreeOnDuplicateID(t *testing.T) {
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

func TestC1808_005_FileAreaTokenizesWrappedAndLocatedEntries(t *testing.T) {
	items := []inboxbatch.Item{
		{ID: "one", Files: []string{"(go/internal/foo/a.go)"}},
		{ID: "two", Files: []string{"go/internal/foo/b.go:178"}},
	}
	if !linked(allDefaultEdges(items), 0, 1) {
		t.Errorf("items sharing go/internal/foo through wrapped/located entries must bind by file area")
	}
}

func TestC1808_006_FileAreaStillSeparatesUnrelatedAreas(t *testing.T) {
	items := []inboxbatch.Item{
		{ID: "one", Files: []string{"(go/internal/foo/a.go)"}},
		{ID: "two", Files: []string{"go/internal/bar/b.go:178"}},
	}
	if linked(allDefaultEdges(items), 0, 1) {
		t.Errorf("different areas must not bind")
	}
}

func TestC1808_007_OperatorStateTokenizesEntries(t *testing.T) {
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

func TestC1808_008_ValidateResolvesDuplicateIDToTheFirstItem(t *testing.T) {
	items := []inboxbatch.Item{
		{ID: "dup", Path: "a.json", Campaign: "alpha"},
		{ID: "dup", Path: "b.json", Campaign: "beta"},
		{ID: "peer", Path: "c.json", Campaign: "alpha"},
	}
	c := inboxbatch.UnifiedCommitment{
		RootCauseHypothesis: "shared cause",
		SharedSeam:          "seam",
		DesignRequirements:  []string{"req"},
		Members: []inboxbatch.UnifiedMember{
			{ID: "dup", Evidence: "e1"},
			{ID: "peer", Evidence: "e2"},
		},
	}
	if err := c.Validate(items); err != nil {
		t.Errorf("Validate must resolve a duplicated id to the first holder like connects and routing do: %v", err)
	}
}

func TestC1808_009_FileAreaTokenizesEveryEntryToken(t *testing.T) {
	items := []inboxbatch.Item{
		{ID: "one", Files: []string{"(see go/internal/foo/a.go)"}},
		{ID: "two", Files: []string{"go/internal/foo/b.go"}},
	}
	if !linked(allDefaultEdges(items), 0, 1) {
		t.Errorf("a prose-prefixed entry must still bind by the package area it names")
	}
}

func TestC1808_010_FileAreaCountsEveryPathInOneEntry(t *testing.T) {
	items := []inboxbatch.Item{
		{ID: "one", Files: []string{"go/internal/foo/a.go go/internal/bar/b.go"}},
		{ID: "two", Files: []string{"go/internal/bar/c.go"}},
	}
	if !linked(allDefaultEdges(items), 0, 1) {
		t.Errorf("the second path of a multi-path entry must contribute its area")
	}
}

func TestC1808_011_OperatorStateRejectsSourcePathAfterStatePath(t *testing.T) {
	mixedInOneEntry := inboxbatch.Item{Class: "pipeline-architecture", Files: []string{".evolve/state.json (see go/internal/core/loop.go)"}}
	if inboxbatch.IsOperatorState(mixedInOneEntry) {
		t.Errorf("a source path later in the same entry must disqualify operator state")
	}
	stateOnly := inboxbatch.Item{Class: "pipeline-architecture", Files: []string{".evolve/state.json (see .evolve/inbox/)"}}
	if !inboxbatch.IsOperatorState(stateOnly) {
		t.Errorf("an entry naming only .evolve/ paths stays operator state")
	}
}

func TestC1808_012_LoadDirOrdersDuplicateIDsByPath(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 40; i++ {
		writeItem(t, dir, "2026-01-01T00-00-"+twoDigits(i)+"Z-x.json", map[string]any{"id": "dup", "title": "t" + twoDigits(i)})
	}
	items, _, err := inboxbatch.LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(items); i++ {
		if items[i-1].Path > items[i].Path {
			t.Fatalf("duplicate ids must keep path order so first-wins is deterministic: %s before %s", items[i-1].Path, items[i].Path)
		}
	}
}

func twoDigits(i int) string {
	return string([]byte{byte('0' + i/10), byte('0' + i%10)})
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

const inboxWarnCode = "INBOX_LOAD_WARNING"

func TestC1808_013_MalformedInboxFileEmitsCodedWarn(t *testing.T) {
	e, inbox, stderr := newEngine(t)
	writeItem(t, inbox, "lane.json", map[string]any{"id": "lane-item", "files": []string{"go/pkg/x.go"}})
	if err := os.WriteFile(filepath.Join(inbox, "broken-item.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, _ := e.RoutedResolver()("lane-item"); ok {
		t.Fatalf("a lane item stays dispatchable")
	}
	out := stderr.String()
	if !strings.Contains(out, "WARN") || !strings.Contains(out, inboxWarnCode) || !strings.Contains(out, "broken-item.json") {
		t.Errorf("a malformed inbox file must surface a WARN carrying %s and naming it, stderr=%q", inboxWarnCode, out)
	}
}

func TestC1808_014_ConsoleRoutedItemWithControlCharsWarnsCodedAndStaysRouted(t *testing.T) {
	e, inbox, stderr := newEngine(t)
	writeItem(t, inbox, "ops.json", map[string]any{"id": "ops-item", "route": "console-manual", "title": "bad\u0007title"})
	routed, _ := e.RoutedResolver()("ops-item")
	if !routed {
		t.Errorf("a sanitized console-routed item must stay routed")
	}
	out := stderr.String()
	if !strings.Contains(out, inboxWarnCode) || !strings.Contains(out, "ops.json") {
		t.Errorf("sanitization of a console-routed item must emit coded WARN, stderr=%q", out)
	}
}

func TestC1808_015_HealthyInboxStaysQuietAndWarnsOncePerResolver(t *testing.T) {
	e, inbox, stderr := newEngine(t)
	writeItem(t, inbox, "lane.json", map[string]any{"id": "lane-item", "files": []string{"go/pkg/x.go"}})
	routed := e.RoutedResolver()
	routed("lane-item")
	routed("unknown")
	if stderr.Len() != 0 {
		t.Errorf("a healthy inbox must emit nothing, stderr=%q", stderr.String())
	}
	if err := os.WriteFile(filepath.Join(inbox, "broken-item.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	routed = e.RoutedResolver()
	routed("lane-item")
	routed("lane-item")
	if n := strings.Count(stderr.String(), "broken-item.json"); n != 1 {
		t.Errorf("one resolver build must warn once per broken file, got %d in %q", n, stderr.String())
	}
}
