package selfheal

import (
	"strings"
	"testing"
	"time"
)

func rerunGreen(id int64, failures ...CIFailure) CIRun {
	return CIRun{ID: id, Branch: "feature", Status: "completed", Conclusion: "success", Attempt: 2, Created: t0, Updated: t0.Add(time.Hour), Failures: failures}
}

func TestFromCIRuns_EachFailingTestOfARerunGreenRunHasItsOwnFingerprintAndRepro(t *testing.T) {
	t.Parallel()
	a := CIFailure{Jobs: []string{"go-test"}, Package: "go/internal/x", Test: "TestA", Repro: "go/internal/x/x_test.go:TestA"}
	b := CIFailure{Jobs: []string{"go-test"}, Package: "go/internal/x", Test: "TestB", Repro: "go/internal/x/x_test.go:TestB"}
	got := FromCIRuns([]CIRun{rerunGreen(1, a), rerunGreen(2, b)}, t0)
	if len(got) != 2 {
		t.Fatalf("want 2 observations, got %+v", got)
	}
	if got[0].Fingerprint() == got[1].Fingerprint() {
		t.Fatalf("two tests share the fingerprint %q", got[0].Fingerprint())
	}
	want := []string{"go/internal/x/x_test.go:TestA", "go/internal/x/x_test.go:TestB"}
	for i, o := range got {
		if o.Class != ClassFlakyTest || o.ReproTest != want[i] || !strings.Contains(o.Summary, "go/internal/x.Test") {
			t.Fatalf("observation %d = %+v, want repro %q", i, o, want[i])
		}
	}
}

func TestFromCIRuns_TheSameFlakyTestInTwoRunsIsOneRecurringFingerprint(t *testing.T) {
	t.Parallel()
	a := CIFailure{Jobs: []string{"go-test"}, Package: "go/internal/x", Test: "TestA"}
	got := FromCIRuns([]CIRun{rerunGreen(1, a), rerunGreen(2, a)}, t0)
	if len(got) != 1 || len(got[0].Evidence) != 2 {
		t.Fatalf("want one observation with two evidence rows, got %+v", got)
	}
}

func TestFromCIRuns_OneTestFailingInTwoJobsIsOneObservationThatNamesTheJobs(t *testing.T) {
	t.Parallel()
	onLinux := CIFailure{Jobs: []string{"go-test (linux)"}, Package: "p", Test: "TestA"}
	onMac := CIFailure{Jobs: []string{"go-test (macos)"}, Package: "p", Test: "TestA"}
	both := CIFailure{Jobs: []string{"go-test (linux)", "go-test (macos)"}, Package: "p", Test: "TestA"}
	got := FromCIRuns([]CIRun{rerunGreen(1, onLinux), rerunGreen(2, onMac), rerunGreen(3, both)}, t0)
	if len(got) != 1 || got[0].Subject != "rerun-green:p.TestA" {
		t.Fatalf("want one observation, got %+v", got)
	}
	if !strings.Contains(got[0].Summary, "(go-test (linux))") {
		t.Fatalf("summary does not name the jobs: %q", got[0].Summary)
	}
}

func TestFromCIRuns_ARunWhoseEarlierTriesWereNotReadMakesNoFlakyObservation(t *testing.T) {
	t.Parallel()
	run := rerunGreen(5)
	run.Unread = true
	if got := FromCIRuns([]CIRun{run}, t0); len(got) != 0 {
		t.Fatalf("an unread run must not mint an identity: %+v", got)
	}
}

func TestFromCIRuns_ARerunGreenRunWithNoFailureNamesTheRunInTheSubject(t *testing.T) {
	t.Parallel()
	got := FromCIRuns([]CIRun{rerunGreen(11), rerunGreen(12)}, t0)
	if len(got) != 2 || got[0].Subject == got[1].Subject || got[0].ReproTest != "" {
		t.Fatalf("two unknown reds must not merge: %+v", got)
	}
	if got[0].Subject != "rerun-green:run-11" {
		t.Fatalf("subject = %q", got[0].Subject)
	}
}

func TestCIFailure_SubjectAndReproByShape(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		f             CIFailure
		subject, repr string
	}{
		{"test", CIFailure{Jobs: []string{"j"}, Package: "p", Test: "TestA", Repro: "p/a_test.go:TestA"}, "rerun-green:p.TestA", "p/a_test.go:TestA"},
		{"test without package", CIFailure{Jobs: []string{"j"}, Test: "TestA"}, "rerun-green:TestA", ""},
		{"package only", CIFailure{Jobs: []string{"j"}, Package: "p"}, "rerun-green:p", ""},
		{"step", CIFailure{Jobs: []string{"j"}, Step: "make lint", Package: "p", Test: "TestA"}, "rerun-green:make lint", ""},
		{"job only", CIFailure{Jobs: []string{"j", "k"}}, "rerun-green:j,k", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.f.subject(); got != tc.subject {
				t.Errorf("subject = %q, want %q", got, tc.subject)
			}
			if got := tc.f.Repro; got != tc.repr {
				t.Errorf("repro = %q, want %q", got, tc.repr)
			}
		})
	}
}

func TestDetect_TheReproTestOfAFlakyObservationReachesTheIncident(t *testing.T) {
	t.Parallel()
	a := CIFailure{Jobs: []string{"go-test"}, Package: "go/internal/x", Test: "TestA", Repro: "go/internal/x/x_test.go:TestA"}
	got, _ := Detect(nil, FromCIRuns([]CIRun{rerunGreen(1, a)}, t0), fakeVerifier{}, t0)
	if len(got) != 1 || got[0].ReproTest != "go/internal/x/x_test.go:TestA" {
		t.Fatalf("incident = %+v", got)
	}
	kept := got[0]
	kept.ReproTest = "go/internal/x/x_test.go:TestKept"
	again, _ := Detect([]Incident{kept}, FromCIRuns([]CIRun{rerunGreen(2, a)}, t0.Add(time.Hour)), fakeVerifier{}, t0.Add(2*time.Hour))
	if again[0].ReproTest != "go/internal/x/x_test.go:TestKept" {
		t.Fatalf("an existing repro was replaced: %+v", again[0])
	}
}

func TestCIRun_IsRerunGreenOnlyForACompletedSuccessfulLaterTry(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		run  CIRun
		want bool
	}{
		{"later try green", CIRun{Attempt: 2, Status: "completed", Conclusion: "success"}, true},
		{"first try green", CIRun{Attempt: 1, Status: "completed", Conclusion: "success"}, false},
		{"later try red", CIRun{Attempt: 2, Status: "completed", Conclusion: "failure"}, false},
		{"later try open", CIRun{Attempt: 2, Status: "in_progress", Conclusion: "success"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.run.IsRerunGreen(); got != tc.want {
				t.Fatalf("IsRerunGreen() = %v", got)
			}
		})
	}
}

func TestDefinesTest_MatchesOnlyTheTopLevelDeclarationOfTheNamedTest(t *testing.T) {
	t.Parallel()
	src := []byte("package x\nfunc TestFlaky(t *testing.T) {}\nfunc helperTestOther() {}\n")
	cases := []struct {
		name string
		want bool
	}{
		{"TestFlaky", true},
		{"TestOther", false},
		{"TestFla", false},
		{"", false},
	}
	for _, c := range cases {
		if got := DefinesTest(src, c.name); got != c.want {
			t.Errorf("DefinesTest(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}
