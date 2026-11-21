package bridge

import (
	"errors"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestOpenRecordFailure_NamesTheMissingRecordWhenTheLogHoldsTheFenceAndNoGrantReport(t *testing.T) {
	t.Parallel()
	tag, path := "run1/7/build/p12n9", "/wt/.evolve/runs/cycle-6/retrospective-report.md"
	cases := map[string]struct {
		poll     readPoll
		wantKind error
		wantText string
	}{
		"fence without a grant report": {
			readPoll{log: []byte(fenceReport(tag)), tag: tag, path: path, elapsed: time.Second},
			errReadRecordMissing, "record missing after the fence: the log holds the fence of the cell and no grant report of " + path,
		},
		"fence with the grant report of another path": {
			readPoll{log: []byte(grantReport(tag, "/wt/other.md") + fenceReport(tag)), tag: tag, path: path},
			errReadRecordMissing, "record missing after the fence: the log holds the fence of the cell and no grant report of " + path,
		},
		"fence with the grant report": {
			readPoll{log: []byte(grantReport(tag, path) + fenceReport(tag)), tag: tag, path: path, elapsed: 1500 * time.Millisecond, stderr: "warn"},
			errReadNoRecord, `no access record names the read of ` + path + ` after 1.5s of polling for the fence (stderr "warn")`,
		},
		"no fence": {
			readPoll{log: nil, tag: tag, path: path, elapsed: time.Minute},
			errReadNoRecord, `no access record names the read of ` + path + ` after 1m0s of polling for the fence (stderr "")`,
		},
		"unmeasured with elapsed": {
			readPoll{tag: tag, path: path, events: []signalcenter.Event{{Code: isolation.CodeReadUnmeasured, Fields: map[string]string{"cause": "fence-deadline", "elapsed": "1m0s"}}}},
			errReadUnmeasured, "unmeasured: cause=fence-deadline after 1m0s (ISOLATION_READ_UNMEASURED)",
		},
		"unmeasured without elapsed beats a fence": {
			readPoll{log: []byte(fenceReport(tag)), tag: tag, path: path, events: []signalcenter.Event{{Code: isolation.CodeReadUnmeasured, Fields: map[string]string{"cause": "fence-write"}}}},
			errReadUnmeasured, "unmeasured: cause=fence-write (ISOLATION_READ_UNMEASURED)",
		},
	}
	for name, tc := range cases {
		err := openRecordFailure(tc.poll)
		if !errors.Is(err, tc.wantKind) {
			t.Errorf("%s: failure %v is not of kind %v", name, err, tc.wantKind)
		}
		if err == nil || err.Error() != tc.wantText {
			t.Errorf("%s: failure = %v, want %q", name, err, tc.wantText)
		}
	}
}
