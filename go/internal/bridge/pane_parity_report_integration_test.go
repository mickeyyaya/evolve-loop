//go:build integration && darwin

package bridge

import (
	"strings"
	"testing"
)

const (
	jsonPass        = `{"Action":"pass","Test":"TestA"}` + "\n" + `{"Action":"pass"}` + "\n"
	jsonFail        = `{"Action":"fail","Test":"TestA"}` + "\n" + `{"Action":"fail"}` + "\n"
	jsonSkipPackage = `{"Action":"skip","Test":"TestA"}` + "\n" + `{"Action":"pass"}` + "\n"
)

func TestGoReport_ValidateRejectsAReportThatDoesNotProveItsExitStatus(t *testing.T) {
	for _, tc := range []struct {
		name, out string
		rc        int
		wantErr   string
	}{
		{"pass with rc 0", jsonPass, 0, ""},
		{"a failing test with rc 1", jsonFail, 1, ""},
		{"rc 1 and no failing test", jsonPass, 1, "exit status 1 but no test failed"},
		{"rc 0 and a failing test", jsonFail, 0, "exit status 0 but"},
		{"a truncated report", `{"Action":"pass","Test":"TestA"}` + "\n" + `{"Action":"pa`, 0, "1 line(s) that are not test2json"},
		{"no package result", `{"Action":"pass","Test":"TestA"}` + "\n", 0, "no package result"},
		{"a build failure line", "# pkg\n./x.go:1: undefined: y\n" + `{"Action":"fail"}` + "\n", 1, "2 line(s) that are not test2json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := newGoReport(tc.out, tc.rc).validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("validate() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("validate() = %v, want an error with %q", err, tc.wantErr)
			}
		})
	}
}

func TestParityTarget_HostEvidenceNeedsEveryNamedTestToPass(t *testing.T) {
	target := parityTarget{pkg: "./p", named: []string{"TestA", "TestB"}, filtered: true}
	both := newGoReport(`{"Action":"pass","Test":"TestA"}`+"\n"+`{"Action":"pass","Test":"TestB"}`+"\n"+`{"Action":"pass"}`+"\n", 0)
	if err := target.hostEvidence(both); err != nil {
		t.Fatalf("both named tests pass: %v", err)
	}
	for name, out := range map[string]string{
		"a named test skips":       `{"Action":"pass","Test":"TestA"}` + "\n" + `{"Action":"skip","Test":"TestB"}` + "\n" + `{"Action":"pass"}` + "\n",
		"the filter misses a test": `{"Action":"pass","Test":"TestA"}` + "\n" + `{"Action":"pass"}` + "\n",
		"only the package passes":  `{"Action":"pass"}` + "\n",
		"every test skips":         jsonSkipPackage,
	} {
		if err := target.hostEvidence(newGoReport(out, 0)); err == nil || !strings.Contains(err.Error(), "TestB") {
			t.Fatalf("%s: hostEvidence() = %v, want an error that names TestB", name, err)
		}
	}
}

func TestParityTarget_GoTestArgsBuildTheRunFilterFromTheNamedTests(t *testing.T) {
	filtered := parityTarget{pkg: "./p", tags: "integration", named: []string{"TestA", "TestB"}, filtered: true}
	if got, want := strings.Join(filtered.goTestArgs(), " "), "test -json -count=1 -tags integration -run ^(TestA|TestB)$ ./p"; got != want {
		t.Fatalf("filtered args = %q, want %q", got, want)
	}
	whole := parityTarget{pkg: "./q", named: []string{"TestA"}}
	if got, want := strings.Join(whole.goTestArgs(), " "), "test -json -count=1 ./q"; got != want {
		t.Fatalf("whole-package args = %q, want %q", got, want)
	}
}
