package acsassert

import "testing"

func TestGoTestRun_RefusesADuplicateStartASkippedPackageAndAPassWithoutARun(t *testing.T) {
	t.Parallel()
	const pkg = "example.com/p"
	cases := []struct {
		name   string
		events []goTestEvent
		want   string
	}{
		{"duplicate start", []goTestEvent{{Package: pkg, Action: "start"}, {Package: pkg, Action: "start"}}, "duplicate Go package start: " + pkg},
		{"package skip", []goTestEvent{{Package: pkg, Action: "start"}, {Package: pkg, Action: "skip"}}, "go package skipped: " + pkg},
		{"run before start", []goTestEvent{{Package: pkg, Test: "TestA", Action: "run"}}, "unexpected Go test start: TestA"},
		{"pass without run", []goTestEvent{{Package: pkg, Action: "start"}, {Package: pkg, Test: "TestA", Action: "pass"}}, "go test passed without one active execution: TestA"},
	}
	for _, tc := range cases {
		run, err := newGoTestRun(pkg, []string{"TestA"})
		if err != nil {
			t.Fatal(err)
		}
		var got error
		for _, e := range tc.events {
			if got = run.consume(e); got != nil {
				break
			}
		}
		if got == nil || got.Error() != tc.want {
			t.Errorf("%s: consume error = %v, want %q", tc.name, got, tc.want)
		}
	}
}
