package main

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestUsageGuardsMatchDispatcher(t *testing.T) {
	help, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	dispatcher, err := os.ReadFile("../../internal/cli/guardcmd/guard.go")
	if err != nil {
		t.Fatal(err)
	}
	known := regexp.MustCompile(`unknown guard %q \(known: ([^)]+)\)`).FindSubmatch(dispatcher)
	if known == nil {
		t.Fatal("missing known guards in dispatcher diagnostic")
	}
	want := strings.Fields(string(known[1]))
	for _, line := range strings.Split(string(help), "\n") {
		if guards, ok := strings.CutPrefix(strings.TrimSpace(line), "Guards:"); ok {
			got := strings.Fields(strings.ReplaceAll(guards, "|", " "))
			if !slices.Equal(got, want) {
				t.Fatalf("help guards = %v, dispatcher guards = %v", got, want)
			}
			return
		}
	}
	t.Fatal("missing Guards line in top-level help")
}
