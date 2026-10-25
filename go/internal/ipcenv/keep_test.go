package ipcenv_test

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func TestKeep_KeepsOnlyTheEntriesWhoseNameTheRuleAllows(t *testing.T) {
	t.Parallel()
	in := []string{"PATH=/usr/bin", "CLAUDECODE=1", "HOME=/Users/lane", "MALFORMED", "=nameless", "FOO=PATH=x"}
	allowed := func(name string) bool { return name == "PATH" || name == "HOME" }
	want := []string{"PATH=/usr/bin", "HOME=/Users/lane"}
	if got := ipcenv.Keep(in, allowed); !reflect.DeepEqual(got, want) {
		t.Fatalf("Keep =\n  %q\nwant\n  %q", got, want)
	}
	if in[1] != "CLAUDECODE=1" {
		t.Fatalf("Keep changed its input: %q", in)
	}
	if got := ipcenv.Keep(nil, allowed); len(got) != 0 {
		t.Fatalf("Keep(nil) = %q, want empty", got)
	}
}
