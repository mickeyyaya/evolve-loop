package evidencex_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evidencex"
)

func extractWith(t *testing.T, src string, budget int) evidencex.Result {
	t.Helper()
	res, err := evidencex.Extract([]byte(src), evidencex.Options{Mode: evidencex.ModeExtract, Budget: budget})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestExtract_EmptySourceKeepsNothing(t *testing.T) {
	t.Parallel()
	res := extractWith(t, "", 64)
	if res.Size != 0 || res.Text != "" || len(res.Ranges) != 0 || len(res.Templates) != 0 {
		t.Errorf("empty source gave %+v", res)
	}
}

func TestExtract_ErrorWindowCoversTheTailWithoutDoubleKeeping(t *testing.T) {
	t.Parallel()
	res := extractWith(t, "ok\nERROR e\nok\n", 4096)
	if len(res.Ranges) != 1 || res.Ranges[0] != (evidencex.Range{Start: 1, End: 3}) {
		t.Errorf("Ranges = %v, want [1-3]", res.Ranges)
	}
}

func TestExtract_BudgetTooSmallForTheWindowDropsItAndStopsTheTail(t *testing.T) {
	t.Parallel()
	res := extractWith(t, "ok\nERROR e\nok\n", 5)
	if len(res.Ranges) != 0 || res.Text != "" {
		t.Errorf("a budget of 5 bytes kept %+v", res)
	}
}

func TestExtract_TemplatesAreOrderedByCountAndDroppedWhenTheyDoNotFit(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	b.WriteString("ERROR first\n")
	for i := 0; i < 30; i++ {
		b.WriteString("alpha 1\n")
	}
	for i := 0; i < 50; i++ {
		b.WriteString("beta 2\n")
	}
	for i := 0; i < 45; i++ {
		b.WriteString("tail line\n")
	}
	full := extractWith(t, b.String(), 1<<20)
	if len(full.Templates) < 2 || full.Templates[0].Count < full.Templates[1].Count {
		t.Fatalf("templates not ordered by count: %+v", full.Templates)
	}
	head := full.Text[:strings.Index(full.Text, "## repeats")]
	tight := extractWith(t, b.String(), len(head))
	if len(tight.Templates) != 0 || len(tight.Text) > len(head) {
		t.Errorf("templates should not fit: %d templates, %d bytes", len(tight.Templates), len(tight.Text))
	}
}
