package looppreflight

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCheckResult_MarshalJSON_LevelIsString(t *testing.T) {
	c := CheckResult{Name: "bridge-boot", Level: LevelHalt, Message: "1 driver failed", Detail: "rc=80"}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `"level":"halt"`) {
		t.Fatalf("level must serialize as the string \"halt\"; got %s", s)
	}
	if !strings.Contains(s, `"name":"bridge-boot"`) {
		t.Fatalf("name missing; got %s", s)
	}
}

// A pass-level check omits the empty detail.
func TestCheckResult_MarshalJSON_OmitsEmptyDetail(t *testing.T) {
	b, _ := json.Marshal(CheckResult{Name: "x", Level: LevelPass, Message: "ok"})
	if strings.Contains(string(b), "detail") {
		t.Fatalf("empty detail should be omitted; got %s", b)
	}
}

func TestResult_PrettyJSON_Shape(t *testing.T) {
	r, err := Run(goodPipelineOptions(t))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := string(r.PrettyJSON())
	for _, want := range []string{`"overall_level"`, `"checks"`, `"checks_passed"`, `"checks_total"`, `"generated_at"`, `"pipeline-structure"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("PrettyJSON missing %q; got:\n%s", want, out)
		}
	}
	// overall_level must be a string token, never a raw int.
	if strings.Contains(out, `"overall_level": 0`) {
		t.Fatalf("overall_level must serialize as a string, not an int; got:\n%s", out)
	}
}

func TestResult_Summary_ListsChecksAndLevels(t *testing.T) {
	r, err := Run(goodPipelineOptions(t))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	s := r.Summary()
	for _, want := range []string{"pipeline-structure", "llm-cli-status", "host-capabilities", "bridge-boot"} {
		if !strings.Contains(s, want) {
			t.Fatalf("Summary missing check %q; got:\n%s", want, s)
		}
	}
}
