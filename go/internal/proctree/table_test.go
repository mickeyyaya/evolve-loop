package proctree

import (
	"reflect"
	"testing"
	"time"
)

func TestParsePS_KeepsOnlyTheRowsOfTheGivenUser(t *testing.T) {
	t.Parallel()
	out := "    1     0     1     0 Wed Aug 19 01:20:16 2026     /sbin/launchd\n" +
		"30466 30465 30466   501 Thu Oct  8 14:13:01 2026     /opt/homebrew/bin/agy\n" +
		"30469 30466 30466   501 Thu Oct  8 14:13:02 2026     /opt/homebrew/Cellar/node/22/bin/node\n" +
		"  777     1   777   502 Thu Oct  8 14:13:02 2026     /usr/bin/tail\n" +
		"  901     1   901   501 Thu Oct  8 09:00:00 2026     Google Chrome Helper (Renderer)\n" +
		"garbage line\n"

	got := parsePS(out, 501)

	want := []Process{
		{Pid: 30466, Ppid: 30465, Pgid: 30466, Started: time.Date(2026, 10, 8, 14, 13, 1, 0, time.Local), Comm: "/opt/homebrew/bin/agy"},
		{Pid: 30469, Ppid: 30466, Pgid: 30466, Started: time.Date(2026, 10, 8, 14, 13, 2, 0, time.Local), Comm: "/opt/homebrew/Cellar/node/22/bin/node"},
		{Pid: 901, Ppid: 1, Pgid: 901, Started: time.Date(2026, 10, 8, 9, 0, 0, 0, time.Local), Comm: "Google Chrome Helper (Renderer)"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsePS =\n%+v\nwant\n%+v\n(rows of uid 0 and uid 502 are foreign; a row that does not parse is dropped)", got, want)
	}
}
