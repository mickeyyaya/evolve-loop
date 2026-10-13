package main

import (
	"os"
	"testing"
)

func TestMain_ExitsWithMainStatusForArgs(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"no subcommand prints usage and exits 2", []string{"commentaudit"}, 2},
		{"unknown subcommand exits 2", []string{"commentaudit", "nonsense"}, 2},
		{"a subcommand missing its required flag exits 2", []string{"commentaudit", "check"}, 2},
		{"a successful ranking of an empty directory exits 0", []string{"commentaudit", "rank", t.TempDir()}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			origArgs, origExit := os.Args, commentauditExit
			t.Cleanup(func() { os.Args, commentauditExit = origArgs, origExit })
			got := -1
			os.Args = tc.args
			commentauditExit = func(code int) { got = code }
			main()
			if got != tc.want {
				t.Fatalf("exit code = %d, want %d", got, tc.want)
			}
		})
	}
}
