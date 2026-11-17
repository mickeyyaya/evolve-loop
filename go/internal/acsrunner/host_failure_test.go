package acsrunner

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func killedBySignal(t *testing.T) error {
	t.Helper()
	err := exec.Command("sh", "-c", "kill -9 $$").Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("sh did not exit with an ExitError: %v", err)
	}
	return err
}

func exitedWithStatus(t *testing.T, code string) error {
	t.Helper()
	err := exec.Command("sh", "-c", "exit "+code).Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("sh did not exit with an ExitError: %v", err)
	}
	return err
}

func emptyOutput(wait error) runCommander {
	return func(context.Context, []string, ...string) (io.ReadCloser, func() error, error) {
		return &closingReader{strings.NewReader("")}, func() error { return wait }, nil
	}
}

func TestHostFailure_WrapsTheCauseAndIsFoundThroughAJoin(t *testing.T) {
	cause := errors.New("signal: killed")
	err := HostFailure(cause)
	if !IsHostFailure(err) || !errors.Is(err, cause) {
		t.Fatalf("HostFailure(%v) = %v, want a host failure that unwraps to its cause", cause, err)
	}
	if !IsHostFailure(errors.Join(errors.New("other"), err)) {
		t.Error("a host failure inside a join must still be found")
	}
	if IsHostFailure(cause) || IsHostFailure(nil) {
		t.Error("a plain error and nil are not host failures")
	}
	if HostFailure(nil) != nil {
		t.Error("HostFailure(nil) must be nil")
	}
}

func TestRun_AGoTestKilledBySignalIsAHostFailure(t *testing.T) {
	withCommander(emptyOutput(killedBySignal(t)), func() {
		v, err := Run(context.Background(), 7, "./acs/cycle7")
		if err == nil || !IsHostFailure(err) {
			t.Fatalf("Run = %+v, %v, want a host failure", v, err)
		}
	})
}

func TestRun_ACancelledContextIsAHostFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	withCommander(emptyOutput(errors.New("context canceled")), func() {
		if _, err := Run(ctx, 7, "./acs/cycle7"); !IsHostFailure(err) {
			t.Fatalf("Run on a cancelled context = %v, want a host failure", err)
		}
	})
}

func TestRun_AGoTestThatCannotStartIsAHostFailure(t *testing.T) {
	withCommander(func(context.Context, []string, ...string) (io.ReadCloser, func() error, error) {
		return nil, nil, errors.New("fork/exec: resource temporarily unavailable")
	}, func() {
		if _, err := Run(context.Background(), 7, "./acs/cycle7"); !IsHostFailure(err) {
			t.Fatalf("Run = %v, want a host failure", err)
		}
	})
}

func TestRun_ABuildFailureIsNotAHostFailureAndKeepsItsSyntheticRed(t *testing.T) {
	withCommander(emptyOutput(exitedWithStatus(t, "1")), func() {
		v, err := Run(context.Background(), 7, "./acs/cycle7")
		if err == nil || IsHostFailure(err) {
			t.Fatalf("Run = %+v, %v, want an error that is not a host failure", v, err)
		}
		if v.RedCount != 1 || len(v.RedIDs) != 1 {
			t.Errorf("verdict = %+v, want the one synthetic red of a package with no predicate", v)
		}
	})
}

func TestRun_APredicateRedUnderAKillIsStillAHostFailure(t *testing.T) {
	body := `{"Action":"fail","Package":"p","Test":"TestA","Elapsed":0.001}` + "\n"
	withCommander(func(context.Context, []string, ...string) (io.ReadCloser, func() error, error) {
		return &closingReader{strings.NewReader(body)}, func() error { return killedBySignal(t) }, nil
	}, func() {
		if _, err := Run(context.Background(), 7, "./acs/cycle7"); !IsHostFailure(err) {
			t.Fatalf("Run = %v, want a host failure: a kill decides nothing about the predicates", err)
		}
	})
}

func TestRun_ARealPackageThatDoesNotCompileIsABuildFailureAndARealCancelIsAHostFailure(t *testing.T) {
	mod := t.TempDir()
	files := map[string]string{
		"go.mod":      "module fixture\n\ngo 1.23\n",
		"p/p.go":      "package p\n",
		"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) { undefinedSymbol() }\n",
	}
	for name, body := range files {
		path := filepath.Join(mod, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(mod); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
	v, err := Run(context.Background(), 1, "./p")
	if err == nil || IsHostFailure(err) || v.RedCount == 0 {
		t.Fatalf("Run on a package that does not compile = %+v, %v, want a red and an error that is not a host failure", v, err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Run(cancelled, 1, "./p"); !IsHostFailure(err) {
		t.Errorf("Run on a cancelled context = %v, want a host failure", err)
	}
}

func TestHostFailure_KeepsTheMessageOfItsCauseAndNilStaysNil(t *testing.T) {
	cause := errors.New("signal: killed")
	if got := HostFailure(cause); got.Error() != cause.Error() || !errors.Is(got, cause) {
		t.Errorf("HostFailure(%v) = %v, want the cause message and chain", cause, got)
	}
	if HostFailure(nil) != nil {
		t.Error("HostFailure(nil) must stay nil")
	}
}
