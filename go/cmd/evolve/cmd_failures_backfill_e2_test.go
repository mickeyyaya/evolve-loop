package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func e2StubProcessGetwd(t *testing.T, err error) {
	t.Helper()
	previous := processGetwd
	processGetwd = func() (string, error) { return "", err }
	t.Cleanup(func() { processGetwd = previous })
}

func TestRunFailures_UnknownSubcommandPrintsUsageAndNamesIt(t *testing.T) {
	for _, verb := range []string{"zap", "LIST", "", "清單"} {
		var stdout, stderr bytes.Buffer

		code := runFailures([]string{verb}, nil, &stdout, &stderr)

		want := failuresUsage + "\nevolve failures: unknown subcommand " + e2Quote(verb) + "\n"
		if code != 10 || stderr.String() != want || stdout.Len() != 0 {
			t.Fatalf("verb %q: code=%d stderr=%q, want 10 and %q", verb, code, stderr.String(), want)
		}
	}
}

func e2Quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestParseFailuresFlags_AcceptsFlagsAndRefusesTheRest(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantOK     bool
		wantStderr string
	}{
		{"no arguments", nil, true, ""},
		{"a known flag", []string{"--project-root", "/r"}, true, ""},
		{"an unknown flag", []string{"--nope"}, false, "flag provided but not defined: -nope"},
		{"one positional", []string{"x"}, false, `evolve failures list: unexpected arguments ["x"]`},
		{"positionals after a flag", []string{"--project-root", "/r", "a", "b"}, false, `unexpected arguments ["a" "b"]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer
			fs, _ := failuresFlags("list", &stderr)

			ok := parseFailuresFlags(fs, tc.args, &stderr)

			if ok != tc.wantOK || !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Fatalf("ok=%v stderr=%q, want %v containing %q", ok, stderr.String(), tc.wantOK, tc.wantStderr)
			}
			if !tc.wantOK && tc.name != "an unknown flag" && !strings.HasPrefix(stderr.String(), failuresUsage) {
				t.Fatalf("a refused positional must lead with the usage, got %q", stderr.String())
			}
		})
	}
}

func TestParseFailuresFlags_HelpIsARefusal(t *testing.T) {
	var stderr bytes.Buffer
	fs, _ := failuresFlags("reset", &stderr)

	if parseFailuresFlags(fs, []string{"-h"}, &stderr) {
		t.Fatal("-h must not proceed")
	}
	if !errors.Is(fs.Parse([]string{"-h"}), flag.ErrHelp) {
		t.Fatal("the flag set must treat -h as help")
	}
}

func TestFailuresReadRoot_ResolvesTheRoot(t *testing.T) {
	t.Run("an explicit root is returned as given", func(t *testing.T) {
		var stderr bytes.Buffer
		got, ok := failuresReadRoot("relative/dir", "list", &stderr)
		if !ok || got != "relative/dir" || stderr.Len() != 0 {
			t.Fatalf("got %q ok=%v stderr=%q", got, ok, stderr.String())
		}
	})
	t.Run("an empty root falls back to the working directory", func(t *testing.T) {
		dir := t.TempDir()
		chdirForTest(t, dir)
		var stderr bytes.Buffer
		got, ok := failuresReadRoot("", "list", &stderr)
		if !ok {
			t.Fatalf("not ok: %q", stderr.String())
		}
		if !e2SameDir(t, got, dir) {
			t.Fatalf("got %q, want the working directory %q", got, dir)
		}
	})
	t.Run("an unresolvable working directory is refused", func(t *testing.T) {
		e2StubProcessGetwd(t, errors.New("cwd gone"))
		var stderr bytes.Buffer
		got, ok := failuresReadRoot("", "prune", &stderr)
		if ok || got != "" {
			t.Fatalf("got %q ok=%v, want a refusal", got, ok)
		}
		if stderr.String() != "evolve failures prune: cannot resolve the working directory: cwd gone\n" {
			t.Fatalf("stderr=%q", stderr.String())
		}
	})
}

func e2SameDir(t *testing.T, a, b string) bool {
	t.Helper()
	ai, err := os.Stat(a)
	if err != nil {
		t.Fatal(err)
	}
	bi, err := os.Stat(b)
	if err != nil {
		t.Fatal(err)
	}
	return os.SameFile(ai, bi)
}

func TestFailuresMutatingRoot_DemandsAnExplicitResolvableRoot(t *testing.T) {
	t.Run("empty is refused", func(t *testing.T) {
		var stderr bytes.Buffer
		got, ok := failuresMutatingRoot("", &stderr)
		if ok || got != "" || !strings.Contains(stderr.String(), "--project-root must be explicitly set") {
			t.Fatalf("got %q ok=%v stderr=%q", got, ok, stderr.String())
		}
	})
	t.Run("an absolute root is cleaned", func(t *testing.T) {
		var stderr bytes.Buffer
		got, ok := failuresMutatingRoot("/a/b/../c/", &stderr)
		if !ok || got != "/a/c" {
			t.Fatalf("got %q ok=%v", got, ok)
		}
	})
	t.Run("a relative root is made absolute", func(t *testing.T) {
		dir := t.TempDir()
		chdirForTest(t, dir)
		var stderr bytes.Buffer
		got, ok := failuresMutatingRoot("sub", &stderr)
		if !ok || !filepath.IsAbs(got) || filepath.Base(got) != "sub" {
			t.Fatalf("got %q ok=%v, want an absolute path ending in sub", got, ok)
		}
	})
	t.Run("an unresolvable root is refused", func(t *testing.T) {
		previous := failuresAbs
		failuresAbs = func(string) (string, error) { return "", errors.New("no cwd") }
		t.Cleanup(func() { failuresAbs = previous })
		var stderr bytes.Buffer
		got, ok := failuresMutatingRoot("sub", &stderr)
		if ok || got != "" || stderr.String() != "evolve failures: resolve --project-root: no cwd\n" {
			t.Fatalf("got %q ok=%v stderr=%q", got, ok, stderr.String())
		}
	})
}

func TestLoadFailedApproaches_DecodesOrRefusesTheState(t *testing.T) {
	cases := []struct {
		name    string
		body    *string
		dir     bool
		want    []map[string]any
		wantErr string
	}{
		{name: "an absent file is empty", want: []map[string]any{}},
		{name: "a directory is a read error", dir: true, wantErr: "is a directory"},
		{name: "corrupt json is an error", body: e2Ptr("{corrupt"), wantErr: "invalid character"},
		{name: "an empty object is empty", body: e2Ptr(`{}`), want: []map[string]any{}},
		{name: "a null array is empty", body: e2Ptr(`{"failedApproaches":null}`), want: []map[string]any{}},
		{name: "an empty array is empty", body: e2Ptr(`{"failedApproaches":[]}`), want: []map[string]any{}},
		{name: "one entry", body: e2Ptr(`{"failedApproaches":[{"cycle":7,"classification":"code-build-fail"}]}`), want: []map[string]any{{"cycle": float64(7), "classification": "code-build-fail"}}},
		{name: "a wrong type is an error", body: e2Ptr(`{"failedApproaches":"nope"}`), wantErr: "cannot unmarshal"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.json")
			if tc.dir {
				if err := os.Mkdir(path, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if tc.body != nil {
				writeE2File(t, path, *tc.body)
			}

			got, err := loadFailedApproaches(path)

			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) || got != nil {
					t.Fatalf("got %v err=%v, want nil and an error containing %q", got, err, tc.wantErr)
				}
				return
			}
			if err != nil || got == nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v err=%v, want %#v", got, err, tc.want)
			}
		})
	}
}

func e2Ptr(s string) *string { return &s }

func TestRunFailuresList_JSONEmitsTheFilteredEntries(t *testing.T) {
	root, _ := failuresProject(t, failuresLiveState, "")
	cases := []struct {
		name       string
		args       []string
		wantCycles []float64
	}{
		{"all", nil, []float64{1, 2, 3, 4}},
		{"one class", []string{"--class", "code-build-fail"}, []float64{2}},
		{"a known class with no entries", []string{"--class", "human-abort"}, []float64{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			args := append([]string{"--json", "--project-root", root}, tc.args...)

			code := runFailuresList(args, &stdout, &stderr)

			if code != 0 {
				t.Fatalf("code=%d stderr=%q", code, stderr.String())
			}
			var got []map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
				t.Fatalf("stdout is not a JSON array: %v (%q)", err, stdout.String())
			}
			cycles := make([]float64, 0, len(got))
			for _, e := range got {
				cycles = append(cycles, e["cycle"].(float64))
			}
			if !reflect.DeepEqual(cycles, tc.wantCycles) {
				t.Fatalf("cycles=%v, want %v", cycles, tc.wantCycles)
			}
		})
	}
}

func TestRunFailuresList_AnEmptyStateIsAnEmptyArrayNotNull(t *testing.T) {
	root, _ := failuresProject(t, `{"failedApproaches":[]}`, "")
	var stdout, stderr bytes.Buffer

	code := runFailuresList([]string{"--json", "--project-root", root}, &stdout, &stderr)

	if code != 0 || stdout.String() != "[]\n" {
		t.Fatalf("code=%d stdout=%q, want []", code, stdout.String())
	}
}

func TestRunFailuresList_ErrorPaths(t *testing.T) {
	t.Run("an unknown flag", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if code := runFailuresList([]string{"--frob"}, &stdout, &stderr); code != 10 {
			t.Fatalf("code=%d, want 10", code)
		}
	})
	t.Run("an unresolvable working directory", func(t *testing.T) {
		e2StubProcessGetwd(t, errors.New("gone"))
		var stdout, stderr bytes.Buffer
		code := runFailuresList(nil, &stdout, &stderr)
		if code != 2 || !strings.Contains(stderr.String(), "evolve failures list: cannot resolve the working directory: gone") || stdout.Len() != 0 {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	})
	t.Run("a corrupt state", func(t *testing.T) {
		root, _ := failuresProject(t, "{corrupt", "")
		var stdout, stderr bytes.Buffer
		code := runFailuresList([]string{"--project-root", root}, &stdout, &stderr)
		if code != 2 || !strings.HasPrefix(stderr.String(), "evolve failures list: invalid character") || stdout.Len() != 0 {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	})
	t.Run("an encode failure", func(t *testing.T) {
		root, _ := failuresProject(t, failuresLiveState, "")
		previous := failuresMarshalIndent
		failuresMarshalIndent = func(any, string, string) ([]byte, error) { return nil, errors.New("encode broke") }
		t.Cleanup(func() { failuresMarshalIndent = previous })
		var stdout, stderr bytes.Buffer
		code := runFailuresList([]string{"--json", "--project-root", root}, &stdout, &stderr)
		if code != 2 || stderr.String() != "evolve failures list: encode broke\n" || stdout.Len() != 0 {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	})
}

func TestRunFailuresList_WithoutARootReadsTheWorkingDirectory(t *testing.T) {
	root, _ := failuresProject(t, failuresLiveState, "")
	chdirForTest(t, root)
	var stdout, stderr bytes.Buffer

	code := runFailuresList([]string{"--class", "ship-gate-config"}, &stdout, &stderr)

	if code != 0 || !strings.HasPrefix(stdout.String(), "1 failed approach(es)\n  cycle=3 ") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestPrintFailuresText_ListsEntriesAndSortedAcknowledgements(t *testing.T) {
	_, evolveDir := failuresProject(t, "", `[{"fingerprint":"fp-b"},{"fingerprint":"fp-a"},{"fingerprint":""}]`)
	shown := []map[string]any{
		{"cycle": float64(5), "recordedAt": "2026-10-01T00:00:00Z", "classification": "code-build-fail", "expiresAt": "2099-01-01T00:00:00Z", "summary": "kept"},
		{"cycle": float64(6), "recordedAt": "2026-10-02T00:00:00Z", "classification": "ship-gate-config", "summary": "no expiry"},
	}
	var stdout, stderr bytes.Buffer

	code := printFailuresText(evolveDir, shown, &stdout, &stderr)

	want := "2 failed approach(es)\n" +
		"  cycle=5  2026-10-01T00:00:00Z  code-build-fail  expiresAt=2099-01-01T00:00:00Z  kept\n" +
		"  cycle=6  2026-10-02T00:00:00Z  ship-gate-config  expiresAt=-  no expiry\n" +
		"2 acknowledged fingerprint(s)\n  fp-a\n  fp-b\n"
	if code != 0 || stdout.String() != want {
		t.Fatalf("code=%d stdout=\n%s\nwant\n%s", code, stdout.String(), want)
	}
}

func TestPrintFailuresText_EmptyListAndNoAcknowledgements(t *testing.T) {
	_, evolveDir := failuresProject(t, "", "")
	var stdout, stderr bytes.Buffer

	code := printFailuresText(evolveDir, nil, &stdout, &stderr)

	if code != 0 || stdout.String() != "0 failed approach(es)\n0 acknowledged fingerprint(s)\n" {
		t.Fatalf("code=%d stdout=%q", code, stdout.String())
	}
}

func TestPrintFailuresText_ACorruptAckLedgerExits2BeforePrinting(t *testing.T) {
	_, evolveDir := failuresProject(t, "", failuresCorruptResolved)
	var stdout, stderr bytes.Buffer

	code := printFailuresText(evolveDir, []map[string]any{{"cycle": 1}}, &stdout, &stderr)

	if code != 2 || !strings.HasPrefix(stderr.String(), "evolve failures list: resolved-fingerprints.json:") || stdout.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestFieldOrDash_DashesOnlyAbsentNilOrEmptyStrings(t *testing.T) {
	e := map[string]any{"nil": nil, "empty": "", "text": "x", "zero": float64(0), "false": false, "unicode": "期限"}
	cases := []struct {
		key  string
		want any
	}{
		{"absent", "-"},
		{"nil", "-"},
		{"empty", "-"},
		{"text", "x"},
		{"zero", float64(0)},
		{"false", false},
		{"unicode", "期限"},
	}
	for _, tc := range cases {
		if got := fieldOrDash(e, tc.key); got != tc.want {
			t.Errorf("fieldOrDash(%q) = %#v, want %#v", tc.key, got, tc.want)
		}
	}
	if got := fieldOrDash(nil, "any"); got != "-" {
		t.Errorf("a nil map must dash, got %#v", got)
	}
}

func TestRunFailuresReset_AnUnknownFlagExits10AndTouchesNothing(t *testing.T) {
	root, evolveDir := failuresProject(t, failuresLiveState, "")
	var stdout, stderr bytes.Buffer

	code := runFailuresReset([]string{"--project-root", root, "--bogus"}, &stdout, &stderr)

	if code != 10 || stdout.Len() != 0 {
		t.Fatalf("code=%d stdout=%q", code, stdout.String())
	}
	if got := failedApproachClasses(t, evolveDir); got != "infrastructure-systemic,code-build-fail,ship-gate-config,code-audit-fail" {
		t.Fatalf("a refused reset changed state: %q", got)
	}
}

func TestRunFailuresPrune_ErrorPaths(t *testing.T) {
	t.Run("an unknown flag", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if code := runFailuresPrune([]string{"--wet"}, &stdout, &stderr); code != 10 {
			t.Fatalf("code=%d", code)
		}
	})
	t.Run("a dry run with no resolvable working directory", func(t *testing.T) {
		e2StubProcessGetwd(t, errors.New("gone"))
		var stdout, stderr bytes.Buffer
		code := runFailuresPrune([]string{"--dry-run"}, &stdout, &stderr)
		if code != 2 || stderr.String() != "evolve failures prune: cannot resolve the working directory: gone\n" {
			t.Fatalf("code=%d stderr=%q", code, stderr.String())
		}
	})
}

func TestRunFailuresPrune_ADryRunPreviewsAndKeepsTheState(t *testing.T) {
	root, evolveDir := failuresProject(t, failuresLiveState, "")
	before, err := os.ReadFile(filepath.Join(evolveDir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	code := runFailures([]string{"prune", "--dry-run", "--project-root", root}, nil, &stdout, &stderr)

	want := "evolve failures prune --dry-run: would remove 1 expired failedApproaches (4→3)\n" +
		"  failedApproaches cycle=4 code-audit-fail expiresAt=2020-02-01T00:00:00Z\n" +
		"evolve failures prune --dry-run: would remove 0 expired carryoverTodos (0→0)\n"
	if code != 0 || stdout.String() != want {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	after, err := os.ReadFile(filepath.Join(evolveDir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("a dry run changed state.json")
	}
}

func TestRunFailuresPrune_ADryRunWithoutARootReadsTheWorkingDirectory(t *testing.T) {
	root, _ := failuresProject(t, failuresLiveState, "")
	chdirForTest(t, root)
	var stdout, stderr bytes.Buffer

	code := runFailuresPrune([]string{"--dry-run"}, &stdout, &stderr)

	if code != 0 || !strings.HasPrefix(stdout.String(), "evolve failures prune --dry-run: would remove 1 expired failedApproaches (4→3)\n") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestPreviewFailuresPrune_ListsEveryExpiredEntryAtTheGivenInstant(t *testing.T) {
	now := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	state := `{"failedApproaches":[` +
		`{"cycle":1,"classification":"code-build-fail","expiresAt":"2026-10-11T23:59:59Z"},` +
		`{"cycle":2,"classification":"code-build-fail","expiresAt":"2026-10-12T00:00:00Z"},` +
		`{"cycle":3,"recordedAt":"2026-10-10T00:00:00Z"},` +
		`{"classification":"x","expiresAt":"2000-01-01T00:00:00Z"}],` +
		`"carryoverTodos":[{"id":"t1","expiresAt":"2026-10-01T00:00:00Z"},{"id":"t2","expiresAt":"2027-01-01T00:00:00Z"},{"recordedAt":"2026-01-01T00:00:00Z"}]}`
	path := filepath.Join(t.TempDir(), "state.json")
	writeE2File(t, path, state)
	var stdout, stderr bytes.Buffer

	code := previewFailuresPrune(path, now, &stdout, &stderr)

	want := "evolve failures prune --dry-run: would remove 3 expired failedApproaches (4→1)\n" +
		"  failedApproaches cycle=1 code-build-fail expiresAt=2026-10-11T23:59:59Z\n" +
		"  failedApproaches cycle=3 - expiresAt=-\n" +
		"  failedApproaches cycle=- x expiresAt=2000-01-01T00:00:00Z\n" +
		"evolve failures prune --dry-run: would remove 2 expired carryoverTodos (3→1)\n" +
		"  carryoverTodos id=t1 expiresAt=2026-10-01T00:00:00Z\n" +
		"  carryoverTodos id=- expiresAt=-\n"
	if code != 0 || stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=\n%s\nwant\n%s\nstderr=%q", code, stdout.String(), want, stderr.String())
	}
}

func TestPreviewFailuresPrune_AnAbsentStateWouldRemoveNothing(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := previewFailuresPrune(filepath.Join(t.TempDir(), "state.json"), time.Now(), &stdout, &stderr)

	want := "evolve failures prune --dry-run: would remove 0 expired failedApproaches (0→0)\n" +
		"evolve failures prune --dry-run: would remove 0 expired carryoverTodos (0→0)\n"
	if code != 0 || stdout.String() != want {
		t.Fatalf("code=%d stdout=%q", code, stdout.String())
	}
}

func TestPreviewFailuresPrune_ACorruptStateExits2(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	writeE2File(t, path, "{corrupt")
	var stdout, stderr bytes.Buffer

	code := previewFailuresPrune(path, time.Now(), &stdout, &stderr)

	if code != 2 || !strings.HasPrefix(stderr.String(), "evolve failures prune: failurelog: parse state:") || stdout.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
