package triagedecision

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDerive_AnUnreadableDroppedSectionFailsTheStrictReadingButNotTheLenientOne(t *testing.T) {
	report := "# Triage\n\nphase_skip: []\n\n## top_n (commit to THIS cycle)\n- good-task: do the thing — files=a.go\n\n" +
		"## deferred (carry)\n(none)\n\n## dropped (rejected with reason)\nThis section holds prose instead of cards.\n"

	out, err := Derive([]byte(report), 7, nil)

	if err == nil || !strings.Contains(err.Error(), "`## dropped` holds prose") || out != nil {
		t.Fatalf("strict: out=%s err=%v", out, err)
	}
	lenient, err := Project(report, 7)
	if err != nil || !strings.Contains(string(lenient), `"good-task"`) {
		t.Fatalf("lenient: %s err=%v", lenient, err)
	}
}

func TestRawField_NonObjectsAndNonStringValuesYieldEmpty(t *testing.T) {
	t.Parallel()
	cases := []struct {
		obj  string
		want string
	}{
		{`{"id":"abc"}`, "abc"},
		{`{"id":7}`, ""},
		{`{"other":"x"}`, ""},
		{`"just a string"`, ""},
		{`[1,2]`, ""},
		{`not json`, ""},
		{``, ""},
	}
	for _, tc := range cases {
		if got := rawField([]byte(tc.obj), "id"); got != tc.want {
			t.Errorf("rawField(%q,id) = %q, want %q", tc.obj, got, tc.want)
		}
	}
}

func writeDecisionFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "triage-decision.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func allRoutes(ids ...string) func(Edit) ([]Route, error) {
	return func(Edit) ([]Route, error) {
		var out []Route
		for _, id := range ids {
			out = append(out, Route{ID: id, Reason: ConsoleRouteReason("route:console-only")})
		}
		return out, nil
	}
}

func TestApplyRoutes_RemovesTheCardAndListsTheEscalationOnce(t *testing.T) {
	path := writeDecisionFile(t, `{"cycle":1,"top_n":[{"id":"x"},{"id":"y"}],"escalate_block":[]}`)

	for run := 0; run < 2; run++ {
		if err := ApplyRoutes(path, []string{"x"}, allRoutes("x")); err != nil {
			t.Fatal(err)
		}
	}

	raw, _ := os.ReadFile(path)
	var got struct {
		TopN []struct {
			ID string `json:"id"`
		} `json:"top_n"`
		Escalate []struct {
			TaskID string `json:"task_id"`
			Reason string `json:"reason"`
		} `json:"escalate_block"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.TopN) != 1 || got.TopN[0].ID != "y" || len(got.Escalate) != 1 || got.Escalate[0].TaskID != "x" {
		t.Fatalf("x leaves top_n and is escalated once: %s", raw)
	}
	if want := "console-routed: route:console-only — operator-owned work is never built by a lane"; got.Escalate[0].Reason != want {
		t.Fatalf("reason = %q, want %q", got.Escalate[0].Reason, want)
	}
}

func TestApplyRoutes_ANoOpRunLeavesTheFileUntouched(t *testing.T) {
	body := `{"top_n":[{"id":"y"}],"escalate_block":[{"task_id":"x","reason":"r"}]}`
	path := writeDecisionFile(t, body)

	if err := ApplyRoutes(path, []string{"x"}, allRoutes("x")); err != nil {
		t.Fatal(err)
	}

	if raw, _ := os.ReadFile(path); string(raw) != body {
		t.Fatalf("nothing to change, nothing written: %s", raw)
	}
}

func TestApplyRoutes_FailsLoudOnAnUnreadableDecision(t *testing.T) {
	for body, want := range map[string]string{
		"not json":                             "decision:",
		`{"top_n":"none"}`:                     "top_n:",
		`{"top_n":[],"escalate_block":"none"}`: "escalate_block:",
	} {
		if err := ApplyRoutes(writeDecisionFile(t, body), []string{"x"}, allRoutes("x")); err == nil || !strings.HasPrefix(err.Error(), want) {
			t.Errorf("%s: err = %v, want prefix %q", body, err, want)
		}
	}
	if err := ApplyRoutes(filepath.Join(t.TempDir(), "absent.json"), nil, allRoutes()); err == nil {
		t.Error("an absent decision is an error")
	}
}

func TestApplyRoutes_ThePlanSeesWhatTheEditFoundAndCanRefuse(t *testing.T) {
	path := writeDecisionFile(t, `{"top_n":[{"id":"x"}],"escalate_block":[{"task_id":"z","reason":"r"}]}`)
	var seen Edit
	err := ApplyRoutes(path, []string{"x"}, func(e Edit) ([]Route, error) {
		seen = e
		return nil, errors.New("refused")
	})
	if err == nil || err.Error() != "refused" {
		t.Fatalf("err = %v", err)
	}
	if seen.Kept != 0 || !seen.Removed["x"] || !seen.Escalated["z"] || len(seen.Raw) == 0 {
		t.Fatalf("edit = %+v", seen)
	}
	if raw, _ := os.ReadFile(path); !strings.Contains(string(raw), `"x"`) {
		t.Fatal("a refused plan writes nothing")
	}
}

func TestDeriveRouted_ANoneReportIsAcceptedOnlyWhenEveryPinnedItemWasRouted(t *testing.T) {
	none := "# Triage\n\n## top_n\nnone\n"
	cases := map[string]struct {
		pin, routed []string
		wantErr     bool
	}{
		"pin fully routed":        {nil, []string{"x"}, false},
		"an unrouted pin remains": {[]string{"y"}, []string{"x"}, true},
		"no route recorded":       {nil, nil, true},
	}
	for name, c := range cases {
		out, err := DeriveRouted([]byte(none), 3, c.pin, c.routed)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err = %v, out = %s", name, err, out)
		}
	}
}

func TestDeriveRouted_AnyOtherReportDerivesAsBefore(t *testing.T) {
	report := "# Triage\n\n## top_n\n- x: one — files=a.go\n- y: two — files=b.go\n"
	out, err := DeriveRouted([]byte(report), 3, []string{"y"}, []string{"x"})
	if err != nil || !strings.Contains(string(out), `"y"`) {
		t.Fatalf("%s %v", out, err)
	}
	if _, err := DeriveRouted([]byte(report), 3, []string{"w"}, []string{"x"}); err == nil {
		t.Fatal("a missing unrouted pin item still fails")
	}
	prose := "# Triage\n\n## top_n\nnone yet\n"
	if _, err := DeriveRouted([]byte(prose), 3, nil, []string{"x"}); err == nil {
		t.Fatal("prose other than none is refused")
	}
	absent := "# Triage\n\n## deferred\n(none)\n"
	if _, err := DeriveRouted([]byte(absent), 3, nil, []string{"x"}); err == nil {
		t.Fatal("no top_n section is refused")
	}
}

func TestDeriveRouted_ACardReportWithNoUnroutedPinDerivesItsCards(t *testing.T) {
	report := "# Triage\n\n## top_n\n- x: one — files=a.go\n"
	out, err := DeriveRouted([]byte(report), 3, nil, []string{"x"})
	if err != nil || !strings.Contains(string(out), `"x"`) {
		t.Fatalf("%s %v", out, err)
	}
}

func TestApplyRoutes_ANullOrAbsentTopNAndEscalationAreEmptyArrays(t *testing.T) {
	for _, body := range []string{`{"top_n":null,"escalate_block":null}`, `{}`} {
		path := writeDecisionFile(t, body)
		if err := ApplyRoutes(path, []string{"x"}, allRoutes("x")); err != nil {
			t.Fatalf("%s: %v", body, err)
		}
		if raw, _ := os.ReadFile(path); !strings.Contains(string(raw), "task_id") {
			t.Fatalf("%s: %s", body, raw)
		}
	}
}

func TestDeriveRouted_ABulletWithNoSlugIDIsNotAnEmptyCommitment(t *testing.T) {
	t.Parallel()
	report := "# Triage\n\n## top_n\n- ~~x~~: rejected — reason\n"

	out, err := DeriveRouted([]byte(report), 7, nil, []string{"x"})

	if err == nil || !strings.Contains(err.Error(), "bullet has no slug id") {
		t.Fatalf("out=%s err=%v", out, err)
	}
}

func TestDeriveRouted_ACapitalisedNoneIsAnEmptyTopN(t *testing.T) {
	t.Parallel()
	report := "# Triage\n\n## top_n\nNone\n"

	out, err := DeriveRouted([]byte(report), 7, nil, []string{"x"})

	if err != nil {
		t.Fatalf("out=%s err=%v", out, err)
	}
}
