package inboxsplit

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func itemWith(t *testing.T, overrides map[string]any) map[string]json.RawMessage {
	t.Helper()
	item := mixedItem(t)
	for k, v := range overrides {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		item[k] = b
	}
	return item
}

func TestInboxSplit_TheLanePartKeepsTheCriteriaThatNameOnlyLaneFiles(t *testing.T) {
	item := itemWith(t, map[string]any{"acceptance": []string{
		"lib/b.go returns the sorted list",
		"guard/a.go rejects an empty id",
		"lib/b.go and guard/c.go agree on the order",
	}})

	p, err := Propose(item, isProtected, false)

	if err != nil {
		t.Fatal(err)
	}
	if got, want := strs(t, p.Lane, "acceptance"), []string{"lib/b.go returns the sorted list"}; !reflect.DeepEqual(got, want) {
		t.Errorf("lane acceptance = %v, want %v", got, want)
	}
	if got, want := strs(t, p.Console, "acceptance"), []string{"guard/a.go rejects an empty id", "lib/b.go and guard/c.go agree on the order"}; !reflect.DeepEqual(got, want) {
		t.Errorf("console acceptance = %v, want %v", got, want)
	}
}

func TestInboxSplit_TheLanePartNeverCarriesAGeneratedCriterion(t *testing.T) {
	p, err := Propose(mixedItem(t), isProtected, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range strs(t, p.Lane, "acceptance") {
		if strings.Contains(c, "is made in") || strings.Contains(c, "only.") {
			t.Errorf("the lane part holds the generated criterion %q, which any edit would pass", c)
		}
	}
}

func TestInboxSplit_ACriterionThatNamesNoFileGoesToBothParts(t *testing.T) {
	item := itemWith(t, map[string]any{"acceptance": []string{
		"go vet and the suite stay green",
		"lib/b.go returns the sorted list",
	}})

	p, err := Propose(item, isProtected, false)

	if err != nil {
		t.Fatal(err)
	}
	if got, want := strs(t, p.Console, "acceptance"), []string{"go vet and the suite stay green"}; !reflect.DeepEqual(got, want) {
		t.Errorf("console acceptance = %v, want %v", got, want)
	}
	if got, want := strs(t, p.Lane, "acceptance"), []string{"go vet and the suite stay green", "lib/b.go returns the sorted list"}; !reflect.DeepEqual(got, want) {
		t.Errorf("lane acceptance = %v, want %v", got, want)
	}
}

func TestInboxSplit_ASlashInProseDoesNotMakeACriterionNameAFile(t *testing.T) {
	criteria := []string{
		"Intake refuses/stamps an id-less item; go vet ./... stays green",
		"docs/x.md lists the split",
		"lib/b.go returns the sorted list",
	}
	item := itemWith(t, map[string]any{"acceptance": criteria})

	p, err := Propose(item, isProtected, false)

	if err != nil {
		t.Fatal(err)
	}
	if got, want := strs(t, p.Console, "acceptance"), criteria[:2]; !reflect.DeepEqual(got, want) {
		t.Errorf("console acceptance = %v, want the two criteria that name no listed file %v", got, want)
	}
	if got := strs(t, p.Lane, "acceptance"); !reflect.DeepEqual(got, criteria) {
		t.Errorf("lane acceptance = %v, want every criterion %v", got, criteria)
	}
}

func TestInboxSplit_ABaseNameThatBothPartsHoldBelongsToTheConsolePart(t *testing.T) {
	item := itemWith(t, map[string]any{
		"files": []string{"guard/doc.go", "lib/doc.go"},
		"fix":   "Edit doc.go.",
	})

	if _, err := Propose(item, isProtected, false); !errors.Is(err, ErrFixNamesProtected) {
		t.Errorf("err = %v, want ErrFixNamesProtected: the console part holds a doc.go too", err)
	}
}

func TestInboxSplit_AWordWithoutASlashIsNeverJudgedAsAPath(t *testing.T) {
	judgesWords := func(path string) bool { return isProtected(path) || path == "skills" || path == "agents" }
	item := itemWith(t, map[string]any{
		"fix":        "Sync the skills and agents lists.",
		"acceptance": []string{"the agents lists match the skills lists"},
	})

	if _, err := Propose(item, judgesWords, false); err != nil {
		t.Errorf("err = %v, want a split: a word is not a path", err)
	}
}

func TestInboxSplit_ADirectoryOfTheLanePartIsNamedWithOrWithoutItsSlash(t *testing.T) {
	item := itemWith(t, map[string]any{
		"files":      []string{"guard/a.go", "lib/dir/"},
		"acceptance": []string{"lib/dir is removed", "lib/dir/ is empty", "the suite stays green"},
	})

	p, err := Propose(item, isProtected, false)

	if err != nil {
		t.Fatal(err)
	}
	if got, want := strs(t, p.Console, "acceptance"), []string{"the suite stays green"}; !reflect.DeepEqual(got, want) {
		t.Errorf("console acceptance = %v, want %v", got, want)
	}
	if got, want := strs(t, p.Lane, "acceptance"), []string{"lib/dir is removed", "lib/dir/ is empty", "the suite stays green"}; !reflect.DeepEqual(got, want) {
		t.Errorf("lane acceptance = %v, want %v", got, want)
	}
}

func TestInboxSplit_ACriterionThatNamesAConsoleFileByItsBasenameStaysOnTheConsolePart(t *testing.T) {
	item := itemWith(t, map[string]any{"acceptance": []string{"a.go rejects an empty id", "b.go keeps its order"}})

	p, err := Propose(item, isProtected, false)

	if err != nil {
		t.Fatal(err)
	}
	if got, want := strs(t, p.Lane, "acceptance"), []string{"b.go keeps its order"}; !reflect.DeepEqual(got, want) {
		t.Errorf("lane acceptance = %v, want only the criterion about the lane file %v", got, want)
	}
	if got, want := strs(t, p.Console, "acceptance"), []string{"a.go rejects an empty id"}; !reflect.DeepEqual(got, want) {
		t.Errorf("console acceptance = %v, want %v", got, want)
	}
}

func TestInboxSplit_AnItemWhoseLanePartWouldHaveNoCriterionGetsNoSplit(t *testing.T) {
	for name, acceptance := range map[string]any{
		"every criterion names a console file": []string{"guard/a.go rejects an empty id"},
		"no acceptance at all":                 nil,
		"an empty list":                        []string{},
	} {
		item := mixedItem(t)
		if acceptance == nil {
			delete(item, "acceptance")
		} else {
			item["acceptance"], _ = json.Marshal(acceptance)
		}

		_, err := Propose(item, isProtected, false)

		if !errors.Is(err, ErrNoCriterion) || !strings.Contains(err.Error(), "lane") {
			t.Errorf("%s: err = %v, want ErrNoCriterion naming the lane part", name, err)
		}
	}
}

func TestInboxSplit_AnItemWhoseConsolePartWouldHaveNoCriterionGetsNoSplit(t *testing.T) {
	item := itemWith(t, map[string]any{"acceptance": []string{"lib/b.go returns the sorted list"}})

	_, err := Propose(item, isProtected, false)

	if !errors.Is(err, ErrNoCriterion) || !strings.Contains(err.Error(), "console") {
		t.Errorf("err = %v, want ErrNoCriterion naming the console part", err)
	}
}

func TestInboxSplit_AnAcceptanceThatIsNotAListOfTextGetsNoSplit(t *testing.T) {
	for _, bad := range []string{`"one criterion"`, `[1,2]`, `{"a":1}`} {
		item := mixedItem(t)
		item["acceptance"] = json.RawMessage(bad)
		if _, err := Propose(item, isProtected, false); !errors.Is(err, ErrMalformed) {
			t.Errorf("acceptance %s: err = %v, want ErrMalformed", bad, err)
		}
	}
}

func TestInboxSplit_ALanePartWithoutDepsCarriesNoDepsKey(t *testing.T) {
	item := mixedItem(t)
	delete(item, "deps")

	p, err := Propose(item, isProtected, false)

	if err != nil {
		t.Fatal(err)
	}
	if raw, has := p.Lane["deps"]; has {
		t.Errorf("the lane part carries deps = %s, want no key", raw)
	}
}

func TestInboxSplit_AnItemWhoseTextNamesAConsoleFileByItsBasenameGetsNoSplit(t *testing.T) {
	for _, field := range []string{"fix", "summary", "description", "title", "source"} {
		item := itemWith(t, map[string]any{field: "Tighten a.go first, then b.go."})

		_, err := Propose(item, isProtected, false)

		if !errors.Is(err, ErrFixNamesProtected) || !strings.Contains(err.Error(), "a.go") {
			t.Errorf("%s: err = %v, want ErrFixNamesProtected naming a.go", field, err)
		}
	}
}

func TestInboxSplit_ATextThatMeetsAConsoleBasenameOnlyInsideALongerNameStillSplits(t *testing.T) {
	item := itemWith(t, map[string]any{
		"fix":     "Edit data.go and lib/b.go; the old a.gone note stays; see ultra.go.bak.",
		"summary": "Keep b.go and d.go in step.",
	})

	if _, err := Propose(item, isProtected, false); err != nil {
		t.Errorf("err = %v, want a split: no console file is named", err)
	}
}

func TestInboxSplit_AConsoleBasenameNamedAtTheEndOfASentenceIsStillNamed(t *testing.T) {
	item := itemWith(t, map[string]any{"fix": "The bug sits in a.go."})

	if _, err := Propose(item, isProtected, false); !errors.Is(err, ErrFixNamesProtected) {
		t.Errorf("err = %v, want ErrFixNamesProtected", err)
	}
}

func TestInboxSplit_AFilesEntryIsJudgedByItsPathTokens(t *testing.T) {
	holds := func(path string) bool { return isProtected(path) || strings.TrimSuffix(path, "/") == "lib/holds" }
	item := itemWith(t, map[string]any{"files": []string{"lib/holds/ (the held surface)", "lib/b.go (new)", "guard/a.go:42"}})

	p, err := Propose(item, holds, false)

	if err != nil {
		t.Fatal(err)
	}
	if got, want := strs(t, p.Console, "files"), []string{"lib/holds/ (the held surface)", "guard/a.go:42"}; !reflect.DeepEqual(got, want) {
		t.Errorf("console files = %v, want %v", got, want)
	}
	if got, want := strs(t, p.Lane, "files"), []string{"lib/b.go (new)"}; !reflect.DeepEqual(got, want) {
		t.Errorf("lane files = %v, want %v", got, want)
	}
}

func laneDepsOf(t *testing.T, laneFiles ...string) []string {
	t.Helper()
	item := itemWith(t, map[string]any{"files": append([]string{"guard/a.go"}, laneFiles...)})
	p, err := Propose(item, isProtected, false)
	if err != nil {
		t.Fatal(err)
	}
	return strs(t, p.Lane, "deps")
}

func TestInboxSplit_ALanePartThatOnlyRegistersARatchetWaitsForTheConsolePart(t *testing.T) {
	for _, files := range [][]string{
		{"go/.cover-strict"},
		{"go/.apicover-enforce (new line)"},
		{"go/.cover-strict", "go/.apicover-enforce"},
	} {
		if got, want := laneDepsOf(t, files...), []string{"other", "fix-it-console"}; !reflect.DeepEqual(got, want) {
			t.Errorf("lane files %v: deps = %v, want %v", files, got, want)
		}
	}
}

func TestInboxSplit_ALanePartThatOnlyHoldsTestsWaitsForTheConsolePart(t *testing.T) {
	for _, files := range [][]string{
		{"go/cmd/evolve/cmd_x_test.go"},
		{"go/acs/cycle1452/predicates_test.go (new)", "go/cmd/evolve/cmd_x_test.go:42"},
		{"go/.cover-strict", "go/internal/lib/b_test.go"},
	} {
		if got, want := laneDepsOf(t, files...), []string{"other", "fix-it-console"}; !reflect.DeepEqual(got, want) {
			t.Errorf("lane files %v: deps = %v, want %v", files, got, want)
		}
	}
}

func TestInboxSplit_ALanePartWithProductionCodeDoesNotWaitForTheConsolePart(t *testing.T) {
	for _, files := range [][]string{
		{"lib/b.go"},
		{"lib/b.go", "lib/b_test.go"},
		{"go/.cover-strict", "lib/b.go"},
		{"docs/x.md"},
		{"lib/"},
		{"cmd_x_test.go"},
	} {
		if got, want := laneDepsOf(t, files...), []string{"other"}; !reflect.DeepEqual(got, want) {
			t.Errorf("lane files %v: deps = %v, want only the original deps %v", files, got, want)
		}
	}
}

func TestInboxSplit_AConsoleDependencyThatIsAskedForAndNeededIsListedOnce(t *testing.T) {
	item := itemWith(t, map[string]any{"files": []string{"guard/a.go", "go/.cover-strict"}})

	p, err := Propose(item, isProtected, true)

	if err != nil {
		t.Fatal(err)
	}
	if got, want := strs(t, p.Lane, "deps"), []string{"other", "fix-it-console"}; !reflect.DeepEqual(got, want) {
		t.Errorf("deps = %v, want the console part once: %v", got, want)
	}
}

func TestInboxSplit_TheProposalSaysWhenTheLanePartWaits(t *testing.T) {
	waiting, err := Propose(itemWith(t, map[string]any{"files": []string{"guard/a.go", "go/.cover-strict"}}), isProtected, false)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := Propose(mixedItem(t), isProtected, false)
	if err != nil {
		t.Fatal(err)
	}
	forced, err := Propose(mixedItem(t), isProtected, true)
	if err != nil {
		t.Fatal(err)
	}
	if !waiting.LaneWaits || plain.LaneWaits || !forced.LaneWaits {
		t.Errorf("LaneWaits = needed:%v plain:%v asked:%v, want true false true", waiting.LaneWaits, plain.LaneWaits, forced.LaneWaits)
	}
}
