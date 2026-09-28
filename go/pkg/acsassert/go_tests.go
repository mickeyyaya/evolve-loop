package acsassert

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type goTestEvent struct {
	Action  string
	Package string
	Test    string
}

type GoTestSpec struct {
	Dir, Package, Pattern string
	Names                 []string
	Race                  bool
	Tags                  string
}

func GoTests(tb TB, spec GoTestSpec) bool {
	tb.Helper()
	args := []string{"test", "-json", "-count=1", "-v"}
	if spec.Race {
		args = append(args, "-race")
	}
	if spec.Tags != "" {
		args = append(args, "-tags", spec.Tags)
	}
	args = append(args, "-run", spec.Pattern, spec.Package)
	ctx := context.Background()
	if caller, ok := tb.(interface{ Deadline() (time.Time, bool) }); ok {
		if deadline, set := caller.Deadline(); set {
			var cancel context.CancelFunc
			ctx, cancel = context.WithDeadline(ctx, deadline)
			defer cancel()
		}
	}
	out, stderr, code, err := sysexec.Capture(ctx, sysexec.DefaultRunner, spec.Dir, "go", args...)
	if err != nil || code != 0 {
		tb.Errorf("go %v exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s", args, code, err, out, stderr)
		return false
	}
	if err := validateGoTests(out, spec.Package, spec.Names); err != nil {
		tb.Errorf("go %v: %v\nstdout:\n%s\nstderr:\n%s", args, err, out, stderr)
		return false
	}
	return true
}

type goTestRun struct {
	packageName       string
	states            map[string]string
	started, finished bool
}

func newGoTestRun(pkg string, names []string) (*goTestRun, error) {
	if pkg == "" || len(names) == 0 {
		return nil, fmt.Errorf("Go test package and expected cases must be nonempty")
	}
	run := &goTestRun{packageName: pkg, states: make(map[string]string, len(names))}
	for _, name := range names {
		if _, duplicate := run.states[name]; name == "" || duplicate {
			return nil, fmt.Errorf("empty or duplicate Go test identity %q", name)
		}
		run.states[name] = ""
	}
	return run, nil
}

func validateGoTests(output, pkg string, names []string) error {
	run, err := newGoTestRun(pkg, names)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(output))
	for {
		var event goTestEvent
		if err := decoder.Decode(&event); err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("decode Go test event: %w", err)
		}
		if err := run.consume(event); err != nil {
			return err
		}
	}
	if !run.finished {
		return fmt.Errorf("Go package did not complete successfully: %s", pkg)
	}
	for _, name := range names {
		if run.states[name] != "pass" {
			return fmt.Errorf("required Go test did not execute and pass: %s", name)
		}
	}
	return nil
}

func (r *goTestRun) consume(event goTestEvent) error {
	if event.Package != r.packageName {
		return nil
	}
	if event.Action == "fail" {
		return fmt.Errorf("Go test failure: %s/%s", event.Package, event.Test)
	}
	if r.finished {
		return fmt.Errorf("Go test event after package completion: %s", event.Action)
	}
	if event.Test == "" {
		return r.packageEvent(event.Action)
	}
	state, required := r.states[event.Test]
	if !required {
		return nil
	}
	switch event.Action {
	case "run":
		if !r.started || state != "" {
			return fmt.Errorf("unexpected Go test start: %s", event.Test)
		}
		r.states[event.Test] = "run"
	case "pass":
		if state != "run" {
			return fmt.Errorf("Go test passed without one active execution: %s", event.Test)
		}
		r.states[event.Test] = "pass"
	case "skip":
		return fmt.Errorf("required Go test skipped: %s", event.Test)
	}
	return nil
}

func (r *goTestRun) packageEvent(action string) error {
	switch action {
	case "start":
		if r.started {
			return fmt.Errorf("duplicate Go package start: %s", r.packageName)
		}
		r.started = true
	case "pass":
		if !r.started {
			return fmt.Errorf("Go package passed before starting: %s", r.packageName)
		}
		r.finished = true
	case "skip":
		return fmt.Errorf("Go package skipped: %s", r.packageName)
	}
	return nil
}
