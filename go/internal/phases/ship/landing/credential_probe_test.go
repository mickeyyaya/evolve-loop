package landing

import (
	"context"
	"testing"
)

func TestWithCredentialProbe_LogsOnceBeforeThePushAndKeepsTodaysPush(t *testing.T) {
	for _, tc := range []struct {
		name string
		line string
		want []string
	}{
		{name: "the probe reports a source", line: "[credential] shadow: source file answered", want: []string{"[credential] shadow: source file answered"}},
		{name: "the stage is off", line: "", want: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeGit()
			var probes, gitBefore []int
			probe := func(context.Context) string {
				probes = append(probes, len(probes))
				gitBefore = append(gitBefore, len(f.calls))
				return tc.line
			}
			l := New(f.git, f.streams, WithCredentialProbe(probe))
			var logs []string
			req := pushReq(SiteDirect)
			req.Log = func(line string) { logs = append(logs, line) }
			if _, err := l.Push(context.Background(), req); err != nil {
				t.Fatal(err)
			}
			if len(probes) != 1 || gitBefore[0] != 0 {
				t.Fatalf("probe ran %d times, first after %v git calls; want once before the push", len(probes), gitBefore)
			}
			if len(f.calls) == 0 || f.calls[0] != "push origin "+testBranch+" [streams]" {
				t.Fatalf("git calls = %v, want today's push first", f.calls)
			}
			if len(logs) != len(tc.want) || (len(tc.want) == 1 && logs[0] != tc.want[0]) {
				t.Fatalf("logs = %q, want %q", logs, tc.want)
			}
		})
	}
}
