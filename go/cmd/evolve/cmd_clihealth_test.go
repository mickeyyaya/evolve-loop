package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

func runDispatch(args ...string) (string, string, int) {
	var out, errOut bytes.Buffer
	code := dispatch(args, strings.NewReader(""), &out, &errOut)
	return out.String(), errOut.String(), code
}

func TestClihealth_ListShowsActiveAndClearRemovesOnlyTheNamedBench(t *testing.T) {
	root := t.TempDir()
	store := clihealth.NewStore(root, time.Now)
	for _, family := range []string{"codex", "agy"} {
		if err := store.Bench(clihealth.Entry{Family: family, Reason: "rate_limit", BenchedUntil: time.Now().Add(time.Hour)}); err != nil {
			t.Fatal(err)
		}
	}
	if out, _, code := runDispatch("clihealth", "list", "--project-root", root); code != 0 || !strings.Contains(out, "codex") || !strings.Contains(out, "agy") {
		t.Errorf("list: exit=%d out=%q", code, out)
	}
	if _, _, code := runDispatch("clihealth", "clear", "codex", "--project-root", root); code != 0 {
		t.Fatalf("clear codex exit=%d", code)
	}
	if active := store.Active(); len(active) != 1 || active["agy"].Family != "agy" {
		t.Errorf("only agy must remain benched, got %v", active)
	}
	if _, errOut, code := runDispatch("clihealth", "clear", "codex", "--project-root", root); code != 1 || !strings.Contains(errOut, "codex") {
		t.Errorf("clear of an unbenched family: exit=%d stderr=%q", code, errOut)
	}
}

func TestClihealth_RejectsUnknownVerbAndMissingFamily(t *testing.T) {
	for _, args := range [][]string{{"clihealth"}, {"clihealth", "frobnicate"}, {"clihealth", "clear"}} {
		if _, _, code := runDispatch(args...); code != 2 {
			t.Errorf("%v exit=%d, want 2", args, code)
		}
	}
}

func TestClihealth_ClearOfAnExpiredBenchExitsOneAndHyphenAliasRoutes(t *testing.T) {
	root := t.TempDir()
	store := clihealth.NewStore(root, time.Now)
	if err := store.Bench(clihealth.Entry{Family: "agy", Reason: "rate_limit", BenchedUntil: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := store.Bench(clihealth.Entry{Family: "codex", Reason: "rate_limit", BenchedUntil: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if out, _, code := runDispatch("cli-health", "list", "--json", "--project-root", root); code != 0 || strings.Contains(out, "agy") || !strings.Contains(out, "codex") {
		t.Errorf("cli-health list --json: exit=%d out=%q", code, out)
	}
	if _, errOut, code := runDispatch("clihealth", "clear", "agy", "--project-root", root); code != 1 || !strings.Contains(errOut, "agy") {
		t.Errorf("clear of an expired bench: exit=%d stderr=%q, want 1 naming agy", code, errOut)
	}
	if _, errOut, code := runDispatch("cli-health", "clear", "codex", "--project-root", root); code != 0 {
		t.Errorf("cli-health clear codex: exit=%d stderr=%q", code, errOut)
	}
	if _, ok := store.Active()["codex"]; ok {
		t.Error("cli-health clear codex left codex benched")
	}
}
