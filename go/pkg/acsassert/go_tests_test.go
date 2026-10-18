package acsassert

import (
	"encoding/json"
	"slices"
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

func TestExactNames_ReadsOnlyFullyAnchoredLists(t *testing.T) {
	for _, tc := range []struct {
		name, pattern string
		want          []string
	}{
		{"one name", "^TestA$", []string{"TestA"}},
		{"a group of names", "^(TestA|TestB)$", []string{"TestA", "TestB"}},
		{"a group of one name", "^(TestA)$", []string{"TestA"}},
		{"names joined by a pipe", "^TestA$|^TestB$", []string{"TestA", "TestB"}},
		{"a name and a group joined by a pipe", "^TestA$|^(TestB|TestC)$", []string{"TestA", "TestB", "TestC"}},
		{"an exact term beside a bare term", "^TestA$|TestB", nil},
		{"a pipe with nothing after it", "^TestA$|", nil},
		{"head anchor only is a prefix", "^TestA_", nil},
		{"tail anchor only is a suffix", "TestA$", nil},
		{"no anchor is a substring", "TestA", nil},
		{"a pipe without a group splits the anchors", "^TestA|TestB$", nil},
		{"an unbalanced group", "^(TestA|TestB$", nil},
		{"an empty group", "^()$", nil},
		{"a regex metacharacter", "^TestA.*$", nil},
		{"a subtest filter", "^TestParent$/^sub$", nil},
		{"empty", "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := exactNames(tc.pattern); !slices.Equal(got, tc.want) {
				t.Fatalf("exactNames(%q) = %q, want %q", tc.pattern, got, tc.want)
			}
		})
	}
}

func TestUnlistedExactNames_ReportsEachExactNameThatNamesLacks(t *testing.T) {
	for _, tc := range []struct {
		name, pattern string
		names, want   []string
	}{
		{"all listed", "^(TestA|TestB)$", []string{"TestB", "TestA"}, nil},
		{"one missing", "^(TestA|TestB)$", []string{"TestA"}, []string{"TestB"}},
		{"all missing", "^(TestA|TestB)$", nil, []string{"TestA", "TestB"}},
		{"a pipe list of terms", "^TestA$|^TestB$", []string{"TestA"}, []string{"TestB"}},
		{"a prefix pattern asks for nothing", "^TestA_", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := unlistedExactNames(tc.pattern, tc.names); !slices.Equal(got, tc.want) {
				t.Fatalf("unlistedExactNames(%q, %q) = %q, want %q", tc.pattern, tc.names, got, tc.want)
			}
		})
	}
}

func TestGoTests_RefusesAnExactPatternNameThatNamesDoNotList(t *testing.T) {
	ft := &fakeT{}
	spec := GoTestSpec{Package: "example.test/contract", Pattern: "^(TestA|TestB)$", Names: []string{"TestA"}}
	if GoTests(ft, spec) {
		t.Fatal("GoTests accepted a pattern whose exact name Names does not list")
	}
	if len(ft.errs) != 1 || !strings.Contains(ft.errs[0], "which Names does not list") {
		t.Fatalf("errors = %q, want one that says Names does not list the pattern name", ft.errs)
	}
}
