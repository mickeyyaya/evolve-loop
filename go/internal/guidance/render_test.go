package guidance

import (
	"encoding/json"
	"strings"
	"testing"
)

func renderRules() []Rule {
	return []Rule{
		{ID: "gate", Statement: "Run a | b unpiped.", Layer: "L2", Status: StatusActive, Audience: []string{"console", "brief"}, Families: []string{"claude"}, Triggers: Triggers{Paths: []string{"go/**"}, Actions: []string{"git-commit"}, Tasks: []string{"review"}, Failures: []string{"gate-red"}}, EnforcedBy: []string{"go/a.go", "go/b.go"}},
		{ID: "safe", Statement: "Never kill all.", Bytes: 42, Layer: "L0", Order: 1, Status: StatusActive, Audience: []string{"all"}, EnforcedBy: []string{"go/k.go"}},
		{ID: "plain", Statement: "Read first.", Layer: "L0", Order: 2, Status: StatusActive, Audience: []string{"all"}},
		{ID: "old", Statement: "Old rule.", Layer: "L0", Status: "retired", Audience: []string{"all"}},
	}
}

func TestIndexMarkdown_GroupsTheActiveRowsByLayerAndEscapesEachCell(t *testing.T) {
	got := IndexMarkdown(renderRules())

	for _, want := range []string{
		"## L0 Invariants\n",
		"| [safe](rules/safe.md) | Never kill all. | all | all | — | `go/k.go` |\n",
		"| [plain](rules/plain.md) | Read first. | all | all | — | — |\n",
		"## L2 Situation\n",
		"| [gate](rules/gate.md) | Run a \\| b unpiped. | console, brief | claude | paths: go/**; actions: git-commit; tasks: review; failures: gate-red | `go/a.go`, `go/b.go` |\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("IndexMarkdown lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "old") {
		t.Errorf("IndexMarkdown lists the retired rule:\n%s", got)
	}
	if strings.Index(got, "[safe]") > strings.Index(got, "[plain]") {
		t.Errorf("IndexMarkdown puts order 2 before order 1:\n%s", got)
	}
}

func TestIndexJSON_HoldsTheSchemaAndNeverANullList(t *testing.T) {
	got := IndexJSON(renderRules())

	var f struct {
		Schema  string           `json:"schema"`
		Entries []map[string]any `json:"entries"`
	}
	if err := json.Unmarshal([]byte(got), &f); err != nil {
		t.Fatal(err)
	}
	if f.Schema != IndexFileSchema || len(f.Entries) != 3 || f.Entries[0]["path"] != "guidance/rules/safe.md" || f.Entries[0]["kind"] != EntryKindRule || f.Entries[0]["summary"] != "Never kill all." || f.Entries[0]["bytes"] != float64(42) {
		t.Fatalf("IndexJSON = %s", got)
	}
	if strings.Contains(got, "null") || !strings.HasSuffix(got, "}\n") {
		t.Fatalf("IndexJSON has a null list or no final newline: %s", got)
	}
}

func TestRows_AnEmptySetIsAnEmptyList(t *testing.T) {
	if rows := Rows(nil); rows == nil || len(rows) != 0 {
		t.Fatalf("Rows(nil) = %#v, want an empty non-nil list", rows)
	}
}

func TestL0Block_HoldsOnlyTheL0StatementsAndThePointer(t *testing.T) {
	got := L0Block(renderRules())

	want := L0Begin + " from guidance/rules by `evolve guidance build`; edit the registry, not this region -->\n### Hard rules\n\n" +
		"- Never kill all. (enforced by `go/k.go`)\n- Read first.\n\n" + PointerLine + "\n" + L0End + "\n"
	if got != want {
		t.Fatalf("L0Block =\n%s\nwant\n%s", got, want)
	}
}

func TestRowsMarkdown_LinksEachRowFromTheGivenBase(t *testing.T) {
	got := RowsMarkdown(Active(renderRules()), "guidance/rules/")

	if !strings.Contains(got, "| [safe](guidance/rules/safe.md) |") || !strings.HasPrefix(got, "\n## L0 Invariants\n") {
		t.Fatalf("RowsMarkdown =\n%s", got)
	}
	if RowsMarkdown(nil, "x/") != "" {
		t.Fatal("RowsMarkdown(nil) is not empty")
	}
}

func TestIndexRow_CarriesTheBodyPathOfItsRule(t *testing.T) {
	var row IndexRow = Rows(Active(renderRules()))[0]

	if row.ID != "safe" || row.Path != BodyPath("safe") || row.Kind != EntryKindRule || row.Summary != "Never kill all." || row.Bytes != 42 || row.Layer != "L0" || len(row.EnforcedBy) != 1 {
		t.Fatalf("IndexRow = %+v", row)
	}
}
