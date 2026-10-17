package inboxsplit

import (
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func isProtected(path string) bool { return strings.Contains(path, "guard/") }

func mkItem(t *testing.T, fields map[string]any) map[string]json.RawMessage {
	t.Helper()
	out := map[string]json.RawMessage{}
	for k, v := range fields {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		out[k] = b
	}
	return out
}

func mixedItem(t *testing.T) map[string]json.RawMessage {
	return mkItem(t, map[string]any{
		"id": "fix-it", "title": "Fix it", "kind": "bug", "summary": "s", "fix": "f",
		"weight": 0.5, "priority_class": "P1", "route": "console-manual",
		"acceptance": []string{"grader judges this"},
		"files":      []string{"guard/a.go", "lib/b.go", "guard/c.go", "lib/d.go"},
		"deps":       []string{"other"},
	})
}

func strs(t *testing.T, item map[string]json.RawMessage, key string) []string {
	t.Helper()
	var out []string
	if raw, ok := item[key]; ok {
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
	}
	return out
}

func text(t *testing.T, item map[string]json.RawMessage, key string) string {
	t.Helper()
	var out string
	if raw, ok := item[key]; ok {
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
	}
	return out
}

func TestInboxSplit_TheConsolePartKeepsEveryProtectedFile(t *testing.T) {
	var p Proposal
	p, err := Propose(mixedItem(t), isProtected, false)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strs(t, p.Console, "files"), []string{"guard/a.go", "guard/c.go"}; !reflect.DeepEqual(got, want) {
		t.Errorf("console files = %v, want every protected file %v", got, want)
	}
	for _, f := range strs(t, p.Lane, "files") {
		if isProtected(f) {
			t.Errorf("the lane part holds the protected file %s", f)
		}
	}
	if got, want := strs(t, p.Lane, "files"), []string{"lib/b.go", "lib/d.go"}; !reflect.DeepEqual(got, want) {
		t.Errorf("lane files = %v, want %v", got, want)
	}
}

func TestInboxSplit_TheConsolePartKeepsTheRouteAndTheAcceptance(t *testing.T) {
	p, err := Propose(mixedItem(t), isProtected, false)
	if err != nil {
		t.Fatal(err)
	}
	if text(t, p.Console, "route") != "console-manual" {
		t.Errorf("console route = %q", text(t, p.Console, "route"))
	}
	if !reflect.DeepEqual(strs(t, p.Console, "acceptance"), []string{"grader judges this"}) {
		t.Errorf("console acceptance = %v", strs(t, p.Console, "acceptance"))
	}
	if _, has := p.Lane["route"]; has {
		t.Error("the lane part carries a route before route-lane writes it")
	}
	if len(strs(t, p.Lane, "acceptance")) == 0 {
		t.Error("the lane part has no acceptance, so it cannot be filed")
	}
}

func TestInboxSplit_BothPartsLinkBackToTheOriginal(t *testing.T) {
	p, err := Propose(mixedItem(t), isProtected, false)
	if err != nil {
		t.Fatal(err)
	}
	if p.OriginalID != "fix-it" || p.ConsoleID != "fix-it-console" || p.LaneID != "fix-it-lane" {
		t.Errorf("ids = %q %q %q", p.OriginalID, p.ConsoleID, p.LaneID)
	}
	for name, part := range map[string]map[string]json.RawMessage{"console": p.Console, "lane": p.Lane} {
		if text(t, part, "split_from") != "fix-it" {
			t.Errorf("%s split_from = %q", name, text(t, part, "split_from"))
		}
		if text(t, part, "id") != map[string]string{"console": p.ConsoleID, "lane": p.LaneID}[name] {
			t.Errorf("%s id = %q", name, text(t, part, "id"))
		}
	}
}

func TestInboxSplit_TheLanePartDependsOnTheConsolePartOnlyWhenAsked(t *testing.T) {
	plain, _ := Propose(mixedItem(t), isProtected, false)
	if !reflect.DeepEqual(strs(t, plain.Lane, "deps"), []string{"other"}) {
		t.Errorf("lane deps = %v, want the original deps only", strs(t, plain.Lane, "deps"))
	}
	after, _ := Propose(mixedItem(t), isProtected, true)
	if !reflect.DeepEqual(strs(t, after.Lane, "deps"), []string{"other", "fix-it-console"}) {
		t.Errorf("lane deps = %v, want the console part added", strs(t, after.Lane, "deps"))
	}
	if !reflect.DeepEqual(strs(t, after.Console, "deps"), []string{"other"}) {
		t.Errorf("console deps = %v", strs(t, after.Console, "deps"))
	}
}

func TestInboxSplit_ProposeNeverChangesTheItemItReads(t *testing.T) {
	item := mixedItem(t)
	before, _ := json.Marshal(item)
	if _, err := Propose(item, isProtected, true); err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(item)
	if string(before) != string(after) {
		t.Errorf("the original changed:\n%s\n%s", before, after)
	}
}

func TestInboxSplit_AnItemWithOnlyProtectedFilesGetsNoSplit(t *testing.T) {
	item := mkItem(t, map[string]any{"id": "x", "files": []string{"guard/a.go", "guard/b.go"}})
	if _, err := Propose(item, isProtected, false); !errors.Is(err, ErrAllProtected) {
		t.Errorf("err = %v, want ErrAllProtected", err)
	}
}

func TestInboxSplit_AnItemWithOnlyLaneSafeFilesGetsNoSplit(t *testing.T) {
	item := mkItem(t, map[string]any{"id": "x", "files": []string{"lib/a.go"}})
	if _, err := Propose(item, isProtected, false); !errors.Is(err, ErrNoneProtected) {
		t.Errorf("err = %v, want ErrNoneProtected", err)
	}
}

func TestInboxSplit_AnItemThatDeclaresNoFilesGetsNoSplit(t *testing.T) {
	item := mkItem(t, map[string]any{"id": "x"})
	if _, err := Propose(item, isProtected, false); !errors.Is(err, ErrNoFiles) {
		t.Errorf("err = %v, want ErrNoFiles", err)
	}
}

func TestInboxSplit_AnItemThatIsNotWellFormedGetsNoSplit(t *testing.T) {
	for name, item := range map[string]map[string]json.RawMessage{
		"no id":       mkItem(t, map[string]any{"files": []string{"guard/a.go", "lib/b.go"}}),
		"files shape": {"id": json.RawMessage(`"x"`), "files": json.RawMessage(`"not-a-list"`)},
		"id shape":    {"id": json.RawMessage(`5`), "files": json.RawMessage(`["a"]`)},
		"deps shape":  {"id": json.RawMessage(`"x"`), "files": json.RawMessage(`["guard/a","b"]`), "deps": json.RawMessage(`"d"`)},
		"title shape": {"id": json.RawMessage(`"x"`), "files": json.RawMessage(`["guard/a","b"]`), "title": json.RawMessage(`7`)},
		"route shape": {"id": json.RawMessage(`"x"`), "files": json.RawMessage(`["guard/a","b"]`), "route": json.RawMessage(`7`)},
	} {
		if _, err := Propose(item, isProtected, false); !errors.Is(err, ErrMalformed) {
			t.Errorf("%s: err = %v, want ErrMalformed", name, err)
		}
	}
}

func TestInboxSplit_AnItemThatCarriesALifecycleStampGetsNoSplit(t *testing.T) {
	item := mixedItem(t)
	item["failure_count"] = json.RawMessage(`2`)
	if _, err := Propose(item, isProtected, false); !errors.Is(err, ErrStamped) {
		t.Errorf("err = %v, want ErrStamped", err)
	}
}

func TestInboxSplit_ALaneRouteIsNotCopiedToTheConsolePart(t *testing.T) {
	item := mixedItem(t)
	item["route"] = json.RawMessage(`"lane"`)
	item["routed_reason"] = json.RawMessage(`"x"`)
	item["routed_at"] = json.RawMessage(`"t"`)
	p, err := Propose(item, isProtected, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"route", "routed_reason", "routed_at"} {
		if _, has := p.Console[k]; has {
			t.Errorf("console part carries %s", k)
		}
		if _, has := p.Lane[k]; has {
			t.Errorf("lane part carries %s", k)
		}
	}
}

func TestSupply_ProposesASplitOfAMixedItemWhenTheSupplyIsLow(t *testing.T) {
	mixed := mixedItem(t)
	onlyProtected := mkItem(t, map[string]any{"id": "p", "files": []string{"guard/a.go"}})
	onlyLane := mkItem(t, map[string]any{"id": "l", "files": []string{"lib/a.go"}})
	noFiles := mkItem(t, map[string]any{"id": "n"})
	items := []map[string]json.RawMessage{onlyProtected, mixed, onlyLane, noFiles}

	got := Proposals(items, 5, 3, isProtected)
	if len(got) != 1 || got[0].OriginalID != "fix-it" {
		t.Fatalf("proposals = %+v, want one for fix-it", got)
	}
	if got := Proposals(items, 6, 3, isProtected); len(got) != 0 {
		t.Errorf("admissible == 2*window still proposed %d splits", len(got))
	}
	if got := Proposals(items, 0, 0, isProtected); len(got) != 0 {
		t.Errorf("a zero window proposed %d splits", len(got))
	}
}

func TestInboxSplit_AnItemWithoutATitleNamesItsPartsByTheOriginalID(t *testing.T) {
	item := mixedItem(t)
	delete(item, "title")
	p, err := Propose(item, isProtected, false)
	if err != nil {
		t.Fatal(err)
	}
	if text(t, p.Console, "title") != "fix-it (console part)" || text(t, p.Lane, "title") != "fix-it (lane part)" {
		t.Errorf("titles = %q, %q", text(t, p.Console, "title"), text(t, p.Lane, "title"))
	}
}

func TestInboxSplit_AnItemWhoseTextNamesAProtectedPathGetsNoSplit(t *testing.T) {
	for _, field := range []string{"fix", "summary", "description"} {
		item := mixedItem(t)
		item[field] = json.RawMessage(`"Edit go/internal/guard/role.go, then run the tests."`)
		_, err := Propose(item, isProtected, false)
		if !errors.Is(err, ErrFixNamesProtected) || !strings.Contains(err.Error(), "go/internal/guard/role.go") {
			t.Errorf("%s: err = %v, want ErrFixNamesProtected naming the path", field, err)
		}
	}
	item := mixedItem(t)
	item["acceptance"] = json.RawMessage(`["fine", "guard/a.go still builds"]`)
	if _, err := Propose(item, isProtected, false); !errors.Is(err, ErrFixNamesProtected) {
		t.Errorf("acceptance: err = %v", err)
	}
}

func TestInboxSplit_ADirectoryInFilesWithAFixThatNamesAProtectedChildGetsNoSplit(t *testing.T) {
	item := mixedItem(t)
	item["files"] = json.RawMessage(`["lib/", "guard/a.go"]`)
	item["fix"] = json.RawMessage(`"Change lib/ and also guard/child.go."`)
	if _, err := Propose(item, isProtected, false); !errors.Is(err, ErrFixNamesProtected) {
		t.Errorf("err = %v, want ErrFixNamesProtected", err)
	}
}

func TestInboxSplit_TextThatNamesOnlyCleanPathsSplitsAndTheLanePartIsScoped(t *testing.T) {
	item := mixedItem(t)
	item["fix"] = json.RawMessage(`"Edit lib/b.go and lib/d.go (see docs/x.md)."`)
	item["summary"] = json.RawMessage(`"and/or the plain words"`)
	item["description"] = json.RawMessage(`7`)
	p, err := Propose(item, isProtected, false)
	if err != nil {
		t.Fatal(err)
	}
	lane := text(t, p.Lane, "fix") + strings.Join(strs(t, p.Lane, "acceptance"), " ")
	if strings.Contains(lane, "guard/") {
		t.Errorf("the lane part names a protected path: %s", lane)
	}
	want := "The change that fix-it describes is made in lib/b.go, lib/d.go only."
	if got := strs(t, p.Lane, "acceptance"); !reflect.DeepEqual(got, []string{want}) {
		t.Errorf("lane acceptance = %v, want %q", got, want)
	}
}

func TestInboxSplit_FieldsOutsideTheAllowlistNeverReachTheLanePart(t *testing.T) {
	item := mixedItem(t)
	item["notes"] = json.RawMessage(`"see go/internal/guard/role.go"`)
	item["root_cause"] = json.RawMessage(`"go/internal/guard/role.go is wrong"`)
	item["zzz_future"] = json.RawMessage(`"go/internal/guard/x.go"`)
	item["merge_notes"] = json.RawMessage(`["guard/y.go"]`)
	p, err := Propose(item, isProtected, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"notes", "root_cause", "zzz_future", "merge_notes"} {
		if _, has := p.Lane[key]; has {
			t.Errorf("the lane part carries %s", key)
		}
		if _, has := p.Console[key]; !has {
			t.Errorf("the console part lost %s", key)
		}
	}
	for key, raw := range p.Lane {
		for _, tok := range pathToken.FindAllString(string(raw), -1) {
			if isProtected(tok) && key != "files" {
				t.Errorf("lane field %s holds the protected path %s", key, tok)
			}
		}
	}
	for key := range p.Lane {
		if !slices.Contains(laneFields, key) && !slices.Contains([]string{"id", "title", "files", "deps", "acceptance", splitFromKey}, key) {
			t.Errorf("the lane part carries %s, which is not allowlisted", key)
		}
	}
}

func TestInboxSplit_TheConsolePartHoldsEveryOriginalField(t *testing.T) {
	item := mixedItem(t)
	item["notes"] = json.RawMessage(`"n"`)
	item["zzz_future"] = json.RawMessage(`1`)
	p, err := Propose(item, isProtected, false)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range item {
		if key == "id" || key == "files" || key == "title" {
			continue
		}
		if got, has := p.Console[key]; !has || string(got) != string(want) {
			t.Errorf("console %s = %s, want %s", key, got, want)
		}
	}
}

func TestInboxSplit_TheLanePartKeepsTheFieldsAClaimNeeds(t *testing.T) {
	item := mixedItem(t)
	for k, v := range map[string]string{"class": `"operator-state"`, "kind": `"bug"`, "deliverable_kind": `"doc"`,
		"injected_by": `"agent"`, "connects_to": `["x"]`, "campaign": `"c"`, "source": `"s"`, "created_at": `"t"`, "priority": `"p"`} {
		item[k] = json.RawMessage(v)
	}
	p, err := Propose(item, isProtected, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"class", "kind", "deliverable_kind", "injected_by", "connects_to", "campaign", "source", "created_at", "priority", "priority_class", "weight", "summary", "fix"} {
		if _, has := p.Lane[key]; !has {
			t.Errorf("the lane part lost %s", key)
		}
	}
}

func TestInboxSplit_TheTitleAndSourceAreScannedForProtectedPaths(t *testing.T) {
	for _, field := range []string{"title", "source", "connects_to"} {
		item := mixedItem(t)
		item[field] = json.RawMessage(`["go/internal/guard/role.go"]`)
		if field != "connects_to" {
			item[field] = json.RawMessage(`"go/internal/guard/role.go"`)
		}
		if _, err := Propose(item, isProtected, false); !errors.Is(err, ErrFixNamesProtected) {
			t.Errorf("%s: err = %v", field, err)
		}
	}
}
