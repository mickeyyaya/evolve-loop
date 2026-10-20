package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAmplify_RoutingPlan_InvalidCycleArgumentsExitTen(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args []string
	}{
		{"missing cycle", []string{"plan"}},
		{"zero cycle", []string{"plan", "--cycle", "0"}},
		{"negative cycle", []string{"plan", "--cycle", "-3"}},
		{"non numeric cycle", []string{"plan", "--cycle", "abc"}},
		{"empty cycle value", []string{"plan", "--cycle", ""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var out, errb bytes.Buffer
			if code := runRouting(tc.args, nil, &out, &errb); code != 10 {
				t.Errorf("exit = %d, want 10; stderr=%q", code, errb.String())
			}
			if out.Len() != 0 {
				t.Errorf("stdout = %q, want empty on usage error", out.String())
			}
		})
	}
}

func TestAmplify_RoutingPlan_UnknownCycleNamesTheCycleAndCreatesNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var out, errb bytes.Buffer
	code := runRouting([]string{"plan", "--cycle", "424242", "--project-root", root}, nil, &out, &errb)
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr=%q", code, errb.String())
	}
	if !strings.Contains(errb.String(), "424242") {
		t.Errorf("stderr = %q, want it to name the cycle", errb.String())
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("project root gained %d entries, want none", len(entries))
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "runs", "cycle-424242")); !os.IsNotExist(err) {
		t.Errorf("run dir for unknown cycle exists or stat failed: %v", err)
	}
}

func TestAmplify_RoutingPlan_UnknownSubcommandHintNamesPlan(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	code := runRouting([]string{"nonesuch"}, nil, &out, &errb)
	if code != 10 {
		t.Fatalf("exit = %d, want 10", code)
	}
	if !strings.Contains(errb.String(), "plan") {
		t.Errorf("stderr = %q, want hint naming plan", errb.String())
	}
}
