package baseattr

import (
	"context"
	"errors"
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type goCall struct {
	name, dir string
	args, env []string
}

func fakeGo(calls *[]goCall, out string, code int, err error) sysexec.RunFunc {
	return func(_ context.Context, name, dir string, args, env []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
		*calls = append(*calls, goCall{name, dir, args, env})
		_, _ = io.WriteString(stdout, out)
		return code, err
	}
}

const failOut = "--- FAIL: TestA (0s)\nFAIL\nFAIL\texample.com/m/p\t0.1s\n"

func twoKeys() Reds {
	return Reds{Keys: []Key{{"example.com/m/p", "TestA"}, {"example.com/m/p", "TestB"}, {"example.com/m/q", "TestC.x"}}, Flags: []string{"-tags", "integration"}}
}

func TestGoTestRunner_RunsOnlyTheNamedKeysInTheModule(t *testing.T) {
	t.Parallel()
	var calls []goCall
	env := []string{"A=1"}
	got, err := goTestRunner(fakeGo(&calls, failOut, 1, nil), env)(context.Background(), "/mod", twoKeys())
	if err != nil {
		t.Fatal(err)
	}
	if want := []Key{{"example.com/m/p", "TestA"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("red keys = %v, want %v", got, want)
	}
	want := goCall{"go", "/mod", []string{"test", "-count=1", "-tags", "integration", "-run", `^(TestA|TestB|TestC\.x)$`, "example.com/m/p", "example.com/m/q"}, env}
	if len(calls) != 1 || !reflect.DeepEqual(calls[0], want) {
		t.Errorf("call = %+v, want %+v", calls, want)
	}
}

func TestGoTestRunner_PassesTheRemainingDeadlineAsTheTestTimeout(t *testing.T) {
	t.Parallel()
	var calls []goCall
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if _, err := goTestRunner(fakeGo(&calls, "", 0, nil), nil)(ctx, "/mod", twoKeys()); err != nil {
		t.Fatal(err)
	}
	args := calls[0].args
	i := slices.Index(args, "-timeout")
	if i < 0 || i+1 >= len(args) {
		t.Fatalf("args = %v: no -timeout", args)
	}
	d, err := time.ParseDuration(args[i+1])
	if err != nil || d < time.Minute || d > 5*time.Minute {
		t.Errorf("-timeout %q = %v (%v), want between 1m and 5m", args[i+1], d, err)
	}
}

func TestGoTestRunner_ANearDeadlineStillSetsAtLeastOneSecond(t *testing.T) {
	t.Parallel()
	var calls []goCall
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, _ = goTestRunner(fakeGo(&calls, "", 0, nil), nil)(ctx, "/mod", twoKeys())
	args := calls[0].args
	if i := slices.Index(args, "-timeout"); i < 0 || args[i+1] != "1s" {
		t.Errorf("args = %v, want -timeout 1s", args)
	}
}

func TestGoTestRunner_AGreenRunReturnsNoKeys(t *testing.T) {
	t.Parallel()
	var calls []goCall
	got, err := goTestRunner(fakeGo(&calls, "ok  \texample.com/m/p\t0.1s\n", 0, nil), nil)(context.Background(), "/mod", twoKeys())
	if err != nil || len(got) != 0 {
		t.Errorf("got %v, %v, want no keys and no error", got, err)
	}
}

func TestGoTestRunner_AFailureOutsideTheAskedKeysIsDropped(t *testing.T) {
	t.Parallel()
	var calls []goCall
	out := "--- FAIL: TestOther (0s)\nFAIL\texample.com/m/p\t0.1s\n"
	got, err := goTestRunner(fakeGo(&calls, out, 1, nil), nil)(context.Background(), "/mod", twoKeys())
	if err != nil || len(got) != 0 {
		t.Errorf("got %v, %v, want no keys: -run could match a test that the lane did not ask for", got, err)
	}
}

func TestGoTestRunner_ErrorsInsteadOfAnEmptyResult(t *testing.T) {
	t.Parallel()
	start := errors.New("exec: go: not found")
	for _, tc := range []struct {
		name string
		out  string
		code int
		err  error
		want string
	}{
		{"cannot start", "", -1, start, "exec: go: not found"},
		{"unnamed failure", "panic: boom\nFAIL\texample.com/m/p\t0.1s\n", 2, nil, "unnamed"},
		{"nonzero exit with no failure line", "no Go files\n", 1, nil, "exit 1"},
	} {
		var calls []goCall
		got, err := goTestRunner(fakeGo(&calls, tc.out, tc.code, tc.err), nil)(context.Background(), "/mod", twoKeys())
		if err == nil || !strings.Contains(err.Error(), tc.want) || got != nil {
			t.Errorf("%s: got %v, %v, want an error with %q", tc.name, got, err, tc.want)
		}
	}
}
