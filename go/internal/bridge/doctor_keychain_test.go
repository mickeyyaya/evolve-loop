package bridge

import (
	"context"
	"errors"
	"io"
	"reflect"
	"runtime"
	"testing"
)

func TestKeychainProbe_DecidesFromThePlatformAndTheExit(t *testing.T) {
	t.Parallel()
	exits := func(rc int, err error) CmdRunner {
		return func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
			return rc, err
		}
	}
	for name, row := range map[string]struct {
		goos    string
		runner  CmdRunner
		service string
		want    bool
	}{
		"another platform has no keychain": {"linux", exits(0, nil), ClaudeKeychainService, false},
		"no runner":                        {"darwin", nil, ClaudeKeychainService, false},
		"no service":                       {"darwin", exits(0, nil), "", false},
		"the item exists":                  {"darwin", exits(0, nil), ClaudeKeychainService, true},
		"the item is absent":               {"darwin", exits(44, nil), ClaudeKeychainService, false},
		"the command cannot start":         {"darwin", exits(0, errors.New("no security")), ClaudeKeychainService, false},
	} {
		if got := keychainProbe(row.goos, row.runner)(row.service); got != row.want {
			t.Errorf("%s: got %v, want %v", name, got, row.want)
		}
	}
}

func TestKeychainProbe_AsksSecurityForTheAttributesOnly(t *testing.T) {
	t.Parallel()
	var gotName string
	var gotArgs []string
	run := func(_ context.Context, name, _ string, args, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
		gotName, gotArgs = name, args
		return 0, nil
	}
	keychainProbe("darwin", run)("a named service")
	want := []string{"find-generic-password", "-s", "a named service"}
	if gotName != "security" || !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("got %s %v, want security %v", gotName, gotArgs, want)
	}
}

func TestDefaultKeychainProbe_UsesTheRunnerOfItsDeps(t *testing.T) {
	t.Parallel()
	calls := 0
	run := func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		calls++
		return 0, nil
	}
	want := keychainProbe(runtime.GOOS, run)(ClaudeKeychainService)
	calls = 0
	got := defaultKeychainProbe(Deps{Runner: run})(ClaudeKeychainService)
	if got != want {
		t.Fatalf("got %v, want %v, the answer for this host", got, want)
	}
	if want && calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}
