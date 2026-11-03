package bridge

import (
	"reflect"
	"testing"
)

func TestCapturePaneArgs(t *testing.T) {
	t.Parallel()
	target := ExactSessionTarget("s")
	cases := []struct {
		name       string
		scrollback int
		joined     bool
		want       []string
	}{
		{"plain with no scrollback", 0, false, []string{"capture-pane", "-p", "-t", target}},
		{"plain with scrollback", 200, false, []string{"capture-pane", "-p", "-S", "-200", "-t", target}},
		{"joined with no scrollback", 0, true, []string{"capture-pane", "-p", "-J", "-t", target}},
		{"joined with scrollback", 200, true, []string{"capture-pane", "-p", "-J", "-S", "-200", "-t", target}},
	}
	for _, c := range cases {
		if got := capturePaneArgs("s", c.scrollback, c.joined); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
