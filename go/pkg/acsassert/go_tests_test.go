package acsassert

import (
	"encoding/json"
	"strings"
	"testing"
)

func goEvents(rows ...[3]string) string {
	var out strings.Builder
	for _, row := range rows {
		raw, _ := json.Marshal(map[string]string{"Action": row[0], "Package": row[1], "Test": row[2]})
		out.Write(raw)
		out.WriteByte('\n')
	}
	return out.String()
}

func TestValidateGoTests_RequiresExactExecutedPasses(t *testing.T) {
	const pkg = "example.test/contract"
	start := goEvents([3]string{"start", pkg, ""})
	run := goEvents([3]string{"run", pkg, "TestExpected"})
	pass := goEvents([3]string{"pass", pkg, "TestExpected"})
	end := goEvents([3]string{"pass", pkg, ""})
	valid := start + run + pass + end
	for _, tc := range []struct {
		name, output string
		wantOK       bool
	}{
		{"complete", valid, true},
		{"parallel_pause_continue", start + run + goEvents([3]string{"pause", pkg, "TestExpected"}, [3]string{"cont", pkg, "TestExpected"}) + pass + end, true},
		{"empty", "", false},
		{"printed_pass", "--- PASS: TestExpected (0.00s)\nPASS\n", false},
		{"printed_json_inside_output", goEvents([3]string{"output", pkg, "TestExpected"}) + end, false},
		{"no_matching_test", start + end, false},
		{"pass_without_run", start + pass + end, false},
		{"run_without_pass", start + run + end, false},
		{"no_package_terminal", start + run + pass, false},
		{"failed_package", start + run + pass + goEvents([3]string{"fail", pkg, ""}), false},
		{"skipped_test", start + run + goEvents([3]string{"skip", pkg, "TestExpected"}) + end, false},
		{"failed_test", start + run + goEvents([3]string{"fail", pkg, "TestExpected"}) + end, false},
		{"prefix_collision", strings.ReplaceAll(valid, "TestExpected", "TestExpectedSuffix"), false},
		{"other_package", strings.ReplaceAll(valid, pkg, "example.test/other"), false},
		{"another_package_fails_beside_a_passing_target", start + run + pass + goEvents([3]string{"fail", "example.test/other", "TestElsewhere"}, [3]string{"fail", "example.test/other", ""}) + end, true},
		{"duplicate_terminal", start + run + pass + pass + end, false},
		{"failure_then_pass", start + run + goEvents([3]string{"fail", pkg, "TestExpected"}) + pass + end, false},
		{"malformed_json", start + "{broken\n" + run + pass + end, false},
		{"event_after_package_pass", valid + run, false},
		{"package_pass_before_start", end + valid, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateGoTests(tc.output, pkg, []string{"TestExpected"})
			if (err == nil) != tc.wantOK {
				t.Fatalf("validateGoTests = %v, want valid=%v", err, tc.wantOK)
			}
		})
	}
}

func TestValidateGoTests_RequiresEveryDeclaredCase(t *testing.T) {
	const pkg = "example.test/contract"
	out := goEvents([3]string{"start", pkg, ""}, [3]string{"run", pkg, "TestParent"},
		[3]string{"run", pkg, "TestParent/edge"}, [3]string{"pass", pkg, "TestParent/edge"},
		[3]string{"pass", pkg, "TestParent"}, [3]string{"pass", pkg, ""})
	for _, tc := range []struct {
		name   string
		tests  []string
		wantOK bool
	}{
		{"parent_and_child", []string{"TestParent", "TestParent/edge"}, true},
		{"missing_second_case", []string{"TestParent", "TestMissing"}, false},
		{"missing_child", []string{"TestParent", "TestParent/absent"}, false},
		{"empty_inventory", nil, false},
		{"empty_identity", []string{""}, false},
		{"duplicate_identity", []string{"TestParent", "TestParent"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateGoTests(out, pkg, tc.tests)
			if (err == nil) != tc.wantOK {
				t.Fatalf("validateGoTests(%v) = %v, want valid=%v", tc.tests, err, tc.wantOK)
			}
		})
	}
}
