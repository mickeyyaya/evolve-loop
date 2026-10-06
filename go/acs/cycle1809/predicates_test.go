//go:build acs

package cycle1809

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

const (
	fieldByteLimit      = 160
	acceptanceByteLimit = 600
	loadWarningCode     = "INBOX_LOAD_WARNING"
	operatorStateClass  = "pipeline-architecture"
)

func writeRecord(t *testing.T, dir, name string, body map[string]any) string {
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

func defaultRuleEdges(items []inboxbatch.Item) []inboxbatch.Edge {
	var edges []inboxbatch.Edge
	for _, r := range inboxbatch.DefaultRules() {
		edges = append(edges, r.Edges(items)...)
	}
	return edges
}

func bound(edges []inboxbatch.Edge, a, b int) bool {
	for _, e := range edges {
		if (e.A == a && e.B == b) || (e.A == b && e.B == a) {
			return true
		}
	}
	return false
}

func assertBoundedPrefix(t *testing.T, field, original, got string, limit int) {
	t.Helper()
	if !utf8.ValidString(got) {
		t.Errorf("%s: truncation produced invalid UTF-8: %q", field, got)
	}
	if len(got) > limit {
		t.Errorf("%s: %d bytes exceeds the %d-byte limit", field, len(got), limit)
	}
	if len(got) <= limit-utf8.UTFMax {
		t.Errorf("%s: kept %d bytes, more than one partial rune short of the %d-byte limit", field, len(got), limit)
	}
	if !strings.HasPrefix(original, got) {
		t.Errorf("%s: result is not a prefix of the original value", field)
	}
}

func TestC1809_001_TruncationAtTheLimitNeverSplitsAMultiByteRune(t *testing.T) {
	dir := t.TempDir()
	threeByteIDStraddlingTheLimit := strings.Repeat("世", 60)
	threeByteTitleStraddlingTheLimit := strings.Repeat("界", 61)
	twoByteFileStraddlingTheLimit := strings.Repeat("é", 79) + "/" + strings.Repeat("é", 2)
	fourByteAcceptanceStraddlingTheLimit := "a" + strings.Repeat("𝄞", 150)
	p := writeRecord(t, dir, "utf8.json", map[string]any{
		"id":         threeByteIDStraddlingTheLimit,
		"title":      threeByteTitleStraddlingTheLimit,
		"files":      []string{twoByteFileStraddlingTheLimit},
		"acceptance": []string{fourByteAcceptanceStraddlingTheLimit},
	})
	it, warnings, err := inboxbatch.LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) == 0 {
		t.Errorf("an overlength record must report that it was sanitized")
	}
	assertBoundedPrefix(t, "id", threeByteIDStraddlingTheLimit, it.ID, fieldByteLimit)
	assertBoundedPrefix(t, "title", threeByteTitleStraddlingTheLimit, it.Title, fieldByteLimit)
	if len(it.Files) != 1 || len(it.Acceptance) != 1 {
		t.Fatalf("files/acceptance arity changed: %d/%d", len(it.Files), len(it.Acceptance))
	}
	assertBoundedPrefix(t, "files[0]", twoByteFileStraddlingTheLimit, it.Files[0], fieldByteLimit)
	assertBoundedPrefix(t, "acceptance[0]", fourByteAcceptanceStraddlingTheLimit, it.Acceptance[0], acceptanceByteLimit)
}

func TestC1809_002_ValuesAtOrUnderTheLimitAreKeptWhole(t *testing.T) {
	dir := t.TempDir()
	multiByteTitleExactlyAtTheLimit := strings.Repeat("é", fieldByteLimit/2)
	asciiAcceptanceExactlyAtTheLimit := strings.Repeat("a", acceptanceByteLimit)
	p := writeRecord(t, dir, "exact.json", map[string]any{
		"id":         "exact-limit",
		"title":      multiByteTitleExactlyAtTheLimit,
		"acceptance": []string{asciiAcceptanceExactlyAtTheLimit},
	})
	it, warnings, err := inboxbatch.LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if it.Title != multiByteTitleExactlyAtTheLimit {
		t.Errorf("a title of exactly %d bytes must be kept whole, got %d bytes", fieldByteLimit, len(it.Title))
	}
	if len(it.Acceptance) != 1 || it.Acceptance[0] != asciiAcceptanceExactlyAtTheLimit {
		t.Errorf("an acceptance entry of exactly %d bytes must be kept whole", acceptanceByteLimit)
	}
	if len(warnings) != 0 {
		t.Errorf("a record at the limit is not malformed and must not warn: %v", warnings)
	}
	asciiOver := writeRecord(t, dir, "ascii.json", map[string]any{"id": "ascii-over", "title": strings.Repeat("a", 500)})
	over, _, err := inboxbatch.LoadFile(asciiOver)
	if err != nil {
		t.Fatal(err)
	}
	if len(over.Title) != fieldByteLimit {
		t.Errorf("an ASCII title must still truncate to exactly %d bytes, got %d", fieldByteLimit, len(over.Title))
	}
}

func TestC1809_003_InvalidUTF8BytesNearTheLimitNeverSurviveLoading(t *testing.T) {
	dir := t.TempDir()
	invalidByteJustBeforeTheLimit := strings.Repeat("a", fieldByteLimit-2) + "\xff\xfe" + strings.Repeat("b", 10)
	raw := []byte(`{"id":"invalid-bytes","title":"` + invalidByteJustBeforeTheLimit + `"}`)
	p := filepath.Join(dir, "invalid.json")
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	it, _, err := inboxbatch.LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !utf8.ValidString(it.Title) {
		t.Errorf("a title carrying invalid bytes at the limit must load as valid UTF-8: %q", it.Title)
	}
	if len(it.Title) > fieldByteLimit {
		t.Errorf("title is %d bytes, over the %d-byte limit", len(it.Title), fieldByteLimit)
	}
}

func TestC1809_004_ConnectsBindsADuplicatedIDToTheFirstHolderOnly(t *testing.T) {
	items := []inboxbatch.Item{
		{ID: "dup", Path: "2026-01-01T00-00-00Z-dup.json"},
		{ID: "dup", Path: "2026-01-02T00-00-00Z-dup.json"},
		{ID: "ref", Path: "2026-01-03T00-00-00Z-ref.json", ConnectsTo: []string{"dup (shares the seam)"}},
	}
	edges := inboxbatch.ConnectsRule{}.Edges(items)
	if !bound(edges, 2, 0) {
		t.Errorf("connects_to a duplicated id must bind the first holder: %+v", edges)
	}
	if bound(edges, 2, 1) {
		t.Errorf("connects_to a duplicated id must not bind a later holder: %+v", edges)
	}
}

func unifiedOver(memberIDs ...string) inboxbatch.UnifiedCommitment {
	members := make([]inboxbatch.UnifiedMember, 0, len(memberIDs))
	for _, id := range memberIDs {
		members = append(members, inboxbatch.UnifiedMember{ID: id, Evidence: "evidence for " + id})
	}
	return inboxbatch.UnifiedCommitment{
		RootCauseHypothesis: "shared cause",
		SharedSeam:          "seam",
		DesignRequirements:  []string{"one fix"},
		Members:             members,
	}
}

func TestC1809_005_ValidateResolvesADuplicatedIDToTheFirstHolder(t *testing.T) {
	firstHolderSharesThePeersCampaign := []inboxbatch.Item{
		{ID: "dup", Path: "a.json", Campaign: "alpha"},
		{ID: "dup", Path: "b.json", Campaign: "beta"},
		{ID: "peer", Path: "c.json", Campaign: "alpha"},
	}
	if err := unifiedOver("dup", "peer").Validate(firstHolderSharesThePeersCampaign); err != nil {
		t.Errorf("Validate must judge a duplicated id by its first holder (same campaign as peer): %v", err)
	}
	firstHolderSplitsFromThePeersCampaign := []inboxbatch.Item{
		{ID: "dup", Path: "a.json", Campaign: "beta"},
		{ID: "dup", Path: "b.json", Campaign: "alpha"},
		{ID: "peer", Path: "c.json", Campaign: "alpha"},
	}
	if err := unifiedOver("dup", "peer").Validate(firstHolderSplitsFromThePeersCampaign); err == nil {
		t.Errorf("Validate must judge a duplicated id by its first holder (a different campaign from peer) and reject")
	}
}

func firstIndexOf(items []inboxbatch.Item, id string) int {
	for i, it := range items {
		if it.ID == id {
			return i
		}
	}
	return -1
}

func TestC1809_006_RoutingConnectsAndValidateResolveADuplicatedIDToTheSameItem(t *testing.T) {
	for _, tc := range []struct {
		name               string
		earlierRoute       string
		laterRoute         string
		wantGateRefusesDup bool
	}{
		{name: "earlier-filed holder is console-routed", earlierRoute: "console-manual", laterRoute: "lane", wantGateRefusesDup: true},
		{name: "earlier-filed holder is lane-routed", earlierRoute: "lane", laterRoute: "console-manual", wantGateRefusesDup: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeRecord(t, dir, "2026-01-01T00-00-00Z-dup.json", map[string]any{"id": "dup", "route": tc.earlierRoute, "campaign": "alpha"})
			writeRecord(t, dir, "2026-01-02T00-00-00Z-dup.json", map[string]any{"id": "dup", "route": tc.laterRoute, "campaign": "beta"})
			writeRecord(t, dir, "2026-01-03T00-00-00Z-peer.json", map[string]any{"id": "peer", "route": "lane", "campaign": "alpha", "connects_to": []string{"dup"}})
			items, _, err := inboxbatch.LoadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			first := firstIndexOf(items, "dup")
			if first < 0 || items[first].Path != "2026-01-01T00-00-00Z-dup.json" {
				t.Fatalf("the first-resolved holder of a duplicated id must be the earliest-filed record, got index %d", first)
			}
			routed, reason := inboxbatch.RoutedResolver(dir, nil)("dup")
			if routed != tc.wantGateRefusesDup {
				t.Errorf("RoutedResolver resolved dup to the later holder: routed=%v (%s), want %v", routed, reason, tc.wantGateRefusesDup)
			}
			peer := firstIndexOf(items, "peer")
			if !bound(inboxbatch.ConnectsRule{}.Edges(items), peer, first) {
				t.Errorf("ConnectsRule must resolve dup to the same earliest-filed holder RoutedResolver used")
			}
			if err := unifiedOver("dup", "peer").Validate(items); err != nil {
				t.Errorf("Validate must resolve dup to the same earliest-filed holder (campaign alpha): %v", err)
			}
		})
	}
}

func TestC1809_007_LoadDirKeepsDuplicatedIDsInFilingOrderWhileSorting(t *testing.T) {
	dir := t.TempDir()
	const records = 48
	for i := 0; i < records; i++ {
		id := "dup"
		if i%2 == 1 {
			id = fmt.Sprintf("other-%02d", records-i)
		}
		if i%3 == 0 {
			id = fmt.Sprintf("aaa-%02d", records-i)
		}
		writeRecord(t, dir, fmt.Sprintf("2026-01-01T00-00-%02dZ-x.json", i), map[string]any{"id": id})
	}
	items, _, err := inboxbatch.LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != records {
		t.Fatalf("loaded %d of %d records", len(items), records)
	}
	previous := ""
	for _, it := range items {
		if it.ID != "dup" {
			continue
		}
		if previous != "" && previous > it.Path {
			t.Fatalf("duplicated ids must keep filing order so first-wins is the earliest record: %s after %s", it.Path, previous)
		}
		previous = it.Path
	}
}

func TestC1809_008_MalformedConsoleRoutedItemResolvesWithACodedLoadWarning(t *testing.T) {
	for _, tc := range []struct {
		name   string
		file   string
		record map[string]any
	}{
		{name: "explicit console route with a control character", file: "ops.json", record: map[string]any{"id": "ops-item", "route": "console-manual", "title": "bad\u0007title"}},
		{name: "derived console route with an overlength title", file: "repair.json", record: map[string]any{"id": "ops-item", "kind": inboxbatch.KindPipelineRepair, "title": strings.Repeat("世", 61)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeRecord(t, dir, tc.file, tc.record)
			writeRecord(t, dir, "lane.json", map[string]any{"id": "lane-item", "files": []string{"go/pkg/x.go"}})
			routed, reason := inboxbatch.RoutedResolver(dir, nil)("ops-item")
			if !routed {
				t.Fatalf("a sanitized console-routed item must stay routed, reason=%q", reason)
			}
			if !strings.Contains(reason, loadWarningCode) {
				t.Errorf("the plan-time gate reason for a malformed console-routed item must carry %s, got %q", loadWarningCode, reason)
			}
			if !strings.Contains(reason, tc.file) {
				t.Errorf("the coded warning must name the malformed record %s, got %q", tc.file, reason)
			}
		})
	}
}

func TestC1809_009_HealthyConsoleRoutedItemCarriesNoLoadWarningCode(t *testing.T) {
	dir := t.TempDir()
	writeRecord(t, dir, "ops.json", map[string]any{"id": "ops-item", "route": "console-manual", "title": "clean title"})
	routed, reason := inboxbatch.RoutedResolver(dir, nil)("ops-item")
	if !routed {
		t.Fatalf("a console-manual item must be routed")
	}
	if !strings.Contains(reason, "route:console-manual") {
		t.Errorf("the routing reason must still state the route, got %q", reason)
	}
	if strings.Contains(reason, loadWarningCode) {
		t.Errorf("a healthy record must not carry %s, got %q", loadWarningCode, reason)
	}
}

func TestC1809_010_LoadWarningsNeverChangeTheRoutingDecision(t *testing.T) {
	dir := t.TempDir()
	writeRecord(t, dir, "lane.json", map[string]any{"id": "lane-item", "title": "lane\u0007item", "files": []string{"go/pkg/x.go"}})
	if err := os.WriteFile(filepath.Join(dir, "broken-item.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	resolve := inboxbatch.RoutedResolver(dir, nil)
	if routed, reason := resolve("lane-item"); routed {
		t.Errorf("a sanitized lane item must stay dispatchable, got routed (%s)", reason)
	}
	if routed, reason := resolve("broken-item"); routed {
		t.Errorf("an unreadable record must fail open (dispatchable), got routed (%s)", reason)
	}
	if routed, reason := resolve("never-filed"); routed {
		t.Errorf("an unknown id must fail open (dispatchable), got routed (%s)", reason)
	}
}

func routingTokens(entry string) []string {
	return inboxbatch.Item{Files: []string{entry}}.DeclaredPaths()
}

func TestC1809_011_FileAreaBindsExactlyWhenRoutingTokensShareAnArea(t *testing.T) {
	const area = "go/internal/foo/"
	anchor := inboxbatch.Item{ID: "anchor", Files: []string{"go/internal/foo/b.go"}}
	for _, entry := range []string{
		"(go/internal/foo/a.go)",
		"go/internal/foo/a.go:178",
		"go/internal/foo/a.go:189,205,221",
		"go/internal/foo/a.go#L10-L20",
		"see go/internal/foo/a.go",
		"go/internal/bar/a.go go/internal/foo/c.go",
		"`go/internal/foo/a.go`",
		"<go/internal/foo/a.go>",
		"go/internal/bar/a.go",
		"w/o go/internal/bar/a.go",
	} {
		wantBound := false
		for _, tok := range routingTokens(entry) {
			if strings.HasPrefix(tok, area) {
				wantBound = true
			}
		}
		items := []inboxbatch.Item{{ID: "probe", Files: []string{entry}}, anchor}
		if got := bound(defaultRuleEdges(items), 0, 1); got != wantBound {
			t.Errorf("entry %q: file-area bound=%v, but routing's tokens %q say bound=%v", entry, got, routingTokens(entry), wantBound)
		}
	}
}

func TestC1809_012_OperatorStateReadsRoutingsDeclaredTokens(t *testing.T) {
	for _, files := range [][]string{
		{"(.evolve/state.json)"},
		{".evolve/state.json", ".evolve/inbox/;"},
		{".evolve/state.json#L3-L9"},
		{".evolve/state.json w/o lock"},
		{".evolve/state.json N/A"},
		{"`.evolve/state.json`"},
		{".evolve/state.json (see go/internal/core/loop.go)"},
		{".evolve/state.json", "(go/internal/core/loop.go)"},
		{"(.evolvex/state.json)"},
		{".evolve/state.json (see .evolve/inbox/)"},
	} {
		want := true
		for _, entry := range files {
			tokens := routingTokens(entry)
			if len(tokens) == 0 {
				want = false
			}
			for _, tok := range tokens {
				if !strings.HasPrefix(tok, ".evolve/") {
					want = false
				}
			}
		}
		it := inboxbatch.Item{ID: "state", Class: operatorStateClass, Files: files}
		if got := inboxbatch.IsOperatorState(it); got != want {
			t.Errorf("files %q: IsOperatorState=%v, but routing's tokens say %v", files, got, want)
		}
	}
	notPipelineArchitecture := inboxbatch.Item{ID: "state", Class: "hygiene", Files: []string{".evolve/state.json"}}
	if inboxbatch.IsOperatorState(notPipelineArchitecture) {
		t.Errorf("only a pipeline-architecture item can be operator state")
	}
}

func TestC1809_013_TokenizedRulesKeepTheWrappedAndMultiPathCases(t *testing.T) {
	for _, tc := range []struct {
		name      string
		probe     string
		anchor    string
		wantBound bool
	}{
		{name: "wrapped and located entries share an area", probe: "(go/internal/foo/a.go)", anchor: "go/internal/foo/b.go:178", wantBound: true},
		{name: "prose-prefixed entry binds by the area it names", probe: "(see go/internal/foo/a.go)", anchor: "go/internal/foo/b.go", wantBound: true},
		{name: "second path of a multi-path entry contributes its area", probe: "go/internal/foo/a.go go/internal/bar/b.go", anchor: "go/internal/bar/c.go", wantBound: true},
		{name: "different areas stay apart", probe: "(go/internal/foo/a.go)", anchor: "go/internal/bar/b.go:178", wantBound: false},
	} {
		items := []inboxbatch.Item{{ID: "probe", Files: []string{tc.probe}}, {ID: "anchor", Files: []string{tc.anchor}}}
		if got := bound(defaultRuleEdges(items), 0, 1); got != tc.wantBound {
			t.Errorf("%s: bound=%v, want %v", tc.name, got, tc.wantBound)
		}
	}
	mixedInOneEntry := inboxbatch.Item{Class: operatorStateClass, Files: []string{".evolve/state.json (see go/internal/core/loop.go)"}}
	if inboxbatch.IsOperatorState(mixedInOneEntry) {
		t.Errorf("a source path later in the same entry must disqualify operator state")
	}
}
