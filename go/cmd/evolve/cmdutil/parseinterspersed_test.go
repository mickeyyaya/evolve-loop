package cmdutil

import (
	"flag"
	"io"
	"reflect"
	"testing"
)

func TestParseInterspersed_ValueFlagsKeepTheirValueWhereverTheyStand(t *testing.T) {
	cases := []struct {
		in        []string
		wantPos   []string
		wantModel string
		wantJSON  bool
	}{
		{[]string{"agy-claude-tmux", "--model", "Claude Opus 5.5 (High)"}, []string{"agy-claude-tmux"}, "Claude Opus 5.5 (High)", false},
		{[]string{"--model", "Claude Opus 5.5 (High)", "agy-claude-tmux", "--json"}, []string{"agy-claude-tmux"}, "Claude Opus 5.5 (High)", true},
		{[]string{"--model=deep", "a", "b"}, []string{"a", "b"}, "deep", false},
		{[]string{"a", "--", "--model"}, []string{"a", "--model"}, "", false},
		{[]string{"a", "--", "x", "--json"}, []string{"a", "x", "--json"}, "", false},
		{[]string{"--model", "--", "pos1", "--json"}, []string{"pos1"}, "--", true},
		{[]string{"--json", "--", "--model", "x"}, []string{"--model", "x"}, "", true},
		{[]string{"--model", "--model", "--", "x", "--json"}, []string{"x", "--json"}, "--model", false},
		{[]string{"--model=--", "x", "--json"}, []string{"x"}, "--", true},
		{[]string{"-json=false", "--", "-"}, []string{"-"}, "", false},
		{nil, nil, "", false},
	}
	for _, tc := range cases {
		fs := flag.NewFlagSet("t", flag.ContinueOnError)
		model := fs.String("model", "", "")
		asJSON := fs.Bool("json", false, "")
		pos, err := ParseInterspersed(fs, tc.in)
		if err != nil {
			t.Fatalf("ParseInterspersed(%q): %v", tc.in, err)
		}
		if !reflect.DeepEqual(pos, tc.wantPos) || *model != tc.wantModel || *asJSON != tc.wantJSON {
			t.Errorf("ParseInterspersed(%q) = %q model=%q json=%v, want %q model=%q json=%v", tc.in, pos, *model, *asJSON, tc.wantPos, tc.wantModel, tc.wantJSON)
		}
	}
}

func TestParseInterspersed_AnUnknownOrValuelessFlagIsAnError(t *testing.T) {
	for _, in := range [][]string{{"a", "--bogus"}, {"a", "--model"}} {
		fs := flag.NewFlagSet("t", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		fs.String("model", "", "")
		if _, err := ParseInterspersed(fs, in); err == nil {
			t.Errorf("ParseInterspersed(%q) accepted a malformed flag", in)
		}
	}
}
