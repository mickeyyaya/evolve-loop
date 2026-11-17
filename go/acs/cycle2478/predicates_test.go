//go:build acs

package cycle2478

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover/lifecycle"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const criterionWithIDToken = "evolve inbox show <id> prints the item & its <id> stays literal"

const filedByInboxAdd = `{"id":"format-probe","kind":"feature","priority_class":"feature","weight":0.3,` +
	`"title":"probe the filed format","files":["go/internal/inboxmover/lifecycle/item.go"],` +
	`"summary":"A rewrite must keep the filed format.","fix":"Keep the filed format.",` +
	`"acceptance":["` + criterionWithIDToken + `","a second criterion"],"source":"console"}`

var classOrder = []string{"correctness", "stability", "performance", "debuggability", "feature", "maintainability", "hygiene", "security"}

func fileThenEdit(t *testing.T, edits []lifecycle.FieldEdit) (before, after string) {
	t.Helper()
	inbox := filepath.Join(t.TempDir(), ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	filedAt := func() time.Time { return time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC) }
	mover := lifecycle.New(inbox, nil, lifecycle.WithNow(filedAt), lifecycle.WithPriorityClasses(classOrder), lifecycle.WithOrigin("console"))
	res, err := mover.File([]byte(filedByInboxAdd))
	if err != nil {
		t.Fatalf("evolve inbox add path (Mover.File): %v", err)
	}
	filed, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mover.Edit("format-probe", edits); err != nil {
		t.Fatalf("evolve inbox edit path (Mover.Edit): %v", err)
	}
	edited, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	return string(filed), string(edited)
}

func changedLines(before, after string) (removed, added []string) {
	b := strings.Split(before, "\n")
	a := strings.Split(after, "\n")
	if len(a) != len(b) {
		return b, a
	}
	for i := range b {
		if b[i] != a[i] {
			removed = append(removed, b[i])
			added = append(added, a[i])
		}
	}
	return removed, added
}

func TestC2478_001_SetWeightChangesOnlyTheWeightLine(t *testing.T) {
	before, after := fileThenEdit(t, []lifecycle.FieldEdit{{Op: lifecycle.EditSet, Field: "weight", Value: "0.4"}})
	removed, added := changedLines(before, after)
	if len(removed) != 1 || len(added) != 1 {
		t.Fatalf("a one-field edit must change one line, changed %d -> %d\nbefore:\n%s\nafter:\n%s", len(removed), len(added), before, after)
	}
	if !strings.Contains(removed[0], `"weight": 0.3`) || !strings.Contains(added[0], `"weight": 0.4`) {
		t.Errorf("the changed line is not the weight line: %q -> %q", removed[0], added[0])
	}
	if want := strings.Replace(before, `"weight": 0.3`, `"weight": 0.4`, 1); after != want {
		t.Errorf("edited file differs from the filed file beyond the weight value\nwant:\n%s\ngot:\n%s", want, after)
	}
}

func TestC2478_002_CriterionWithIDTokenSurvivesAnEditByteForByte(t *testing.T) {
	addedCriterion := "evolve inbox edit <id> --add acceptance keeps <id> & <tags>"
	for name, tc := range map[string]struct {
		edit lifecycle.FieldEdit
		want string
	}{
		"weight set":     {lifecycle.FieldEdit{Op: lifecycle.EditSet, Field: "weight", Value: "0.4"}, `"` + criterionWithIDToken + `",`},
		"title set":      {lifecycle.FieldEdit{Op: lifecycle.EditSet, Field: "title", Value: "a new title"}, `"` + criterionWithIDToken + `",`},
		"files add":      {lifecycle.FieldEdit{Op: lifecycle.EditAdd, Field: "files", Value: "go/internal/inboxmover/lifecycle/edit.go"}, `"` + criterionWithIDToken + `",`},
		"acceptance add": {lifecycle.FieldEdit{Op: lifecycle.EditAdd, Field: "acceptance", Value: addedCriterion}, `"` + addedCriterion + `"`},
		"acceptance set": {lifecycle.FieldEdit{Op: lifecycle.EditSet, Field: "acceptance", Value: `["` + addedCriterion + `"]`}, `"` + addedCriterion + `"`},
		"summary set":    {lifecycle.FieldEdit{Op: lifecycle.EditSet, Field: "summary", Value: "mentions <id> & more"}, `"summary": "mentions <id> & more"`},
	} {
		t.Run(name, func(t *testing.T) {
			_, after := fileThenEdit(t, []lifecycle.FieldEdit{tc.edit})
			if !strings.Contains(after, tc.want) {
				t.Errorf("%s did not survive the rewrite byte-for-byte\n%s", tc.want, after)
			}
		})
	}
}

func TestC2478_004_RewriteNeverHTMLEscapesTheItem(t *testing.T) {
	_, after := fileThenEdit(t, []lifecycle.FieldEdit{{Op: lifecycle.EditSet, Field: "fix", Value: "keep <id> & <b> literal"}})
	for _, escaped := range []string{`\u003c`, `\u003e`, `\u0026`} {
		if strings.Contains(after, escaped) {
			t.Errorf("the rewrite HTML-escaped the item (%s)\n%s", escaped, after)
		}
	}
}

func TestC2478_003_RuntimeReferenceDescribesTheFiledFormatRewrite(t *testing.T) {
	doc := filepath.Join(acsassert.RepoRoot(t), "docs", "operations", "runtime-reference.md")
	body, err := os.ReadFile(doc)
	if err != nil {
		t.Fatal(err)
	}
	var entry string
	for _, line := range strings.Split(string(body), "\n") {
		if strings.Contains(line, "evolve inbox edit <id|item-path>") {
			entry = line
			break
		}
	}
	if entry == "" {
		t.Fatal("runtime-reference.md has no `evolve inbox edit <id|item-path>` entry")
	}
	for _, stale := range []string{"compact JSON with sorted keys", "compare fields, not lines"} {
		if strings.Contains(entry, stale) {
			t.Errorf("the inbox edit entry still says %q, but the item writer keeps the filed format", stale)
		}
	}
}
