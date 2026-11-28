package inboxcmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentrequest"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxcurator"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxfile"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxtest"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/verifylock"
)

const agentItem = `{"id":"agent-finding","kind":"fix","priority_class":"correctness","weight":0.5,` +
	`"title":"A finding of the build","files":["go/internal/fleet/pool.go"],` +
	`"summary":"The build saw a defect.","fix":"Fix the defect.",` +
	`"acceptance":["TestPool_TheDefectIsFixed passes"]}`

func causeOf(t *testing.T, err error) string {
	t.Helper()
	var refused *RefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("err = %v, want a *RefusedError", err)
	}
	return refused.Cause
}

func fileAs(t *testing.T, root, raw, source string, deps Deps) (agentrequest.Filed, error) {
	t.Helper()
	return NewFiler(deps, io.Discard)(context.Background(), root, []byte(raw), source)
}

func protectedDeps(protected string) Deps {
	deps := testDeps()
	deps.LaneForbidden = func(string, io.Writer) func(string) bool {
		return func(path string) bool { return path == protected }
	}
	return deps
}

func TestFiler_FilesTheItemWithTheOriginCycleAndTheSource(t *testing.T) {
	t.Parallel()
	root := inboxtest.EmptyRoot(t)

	filed, err := fileAs(t, root, agentItem, "request:7/inbox-add-1", testDeps())
	if err != nil {
		t.Fatal(err)
	}

	doc := filedItem(t, root, "agent-finding")
	if filed.ID != "agent-finding" || doc["origin"] != "cycle" || doc["source"] != "request:7/inbox-add-1" {
		t.Fatalf("filed = %+v doc = %v, want the origin cycle and the request source", filed, doc)
	}
	if _, err := os.Stat(filed.Path); err != nil {
		t.Fatalf("the path of the filed item must exist: %v", err)
	}
}

func TestIntakeItem_UsesTheHostFunctionsOfItsDepsToRouteAProtectedPath(t *testing.T) {
	t.Parallel()
	root := inboxtest.EmptyRoot(t)
	item := strings.Replace(agentItem, "go/internal/fleet/pool.go", "go/internal/guarded/guard.go", 1)
	deps := protectedDeps("go/internal/guarded/guard.go")
	pol, err := policy.Load(filepath.Join(root, ".evolve", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	signals, closeSignals := deps.NewSignalCenter("inbox", root, filepath.Join(root, ".evolve"), io.Discard)
	defer closeSignals()

	res, err := intakeItem(context.Background(), intakeInput{root: root, raw: []byte(item), source: "request:7/inbox-add-1", pol: pol,
		isProtected: deps.LaneForbidden(root, io.Discard), signals: signals, stderr: io.Discard, req: inboxAddRequest{origin: inboxbatch.OriginCycle}})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(res.ConsoleReason, "go/internal/guarded/guard.go") {
		t.Fatalf("a protected path must file console-routed from the deps of the filer, got %+v", res)
	}
}

func TestFiler_RefusesWhenTheDepsLackAHostFunction(t *testing.T) {
	t.Parallel()
	root := inboxtest.EmptyRoot(t)
	for name, deps := range map[string]Deps{
		"no lane check":    {NewSignalCenter: testDeps().NewSignalCenter},
		"no signal center": {LaneForbidden: testDeps().LaneForbidden},
	} {
		if _, err := fileAs(t, root, agentItem, "request:7/inbox-add-1", deps); err == nil || errors.Is(err, agentrequest.ErrItemRefused) {
			t.Errorf("%s: err = %v, want a failure of the pipeline, not a refusal of the request", name, err)
		}
	}
	if matches, _ := filepath.Glob(filepath.Join(root, ".evolve", "inbox", "*.json")); len(matches) != 0 {
		t.Errorf("a filer with no host function files nothing, got %v", matches)
	}
}

func TestFiler_RefusesAnItemTheIntakeRefusesWithTheCause(t *testing.T) {
	t.Parallel()
	root := inboxtest.EmptyRoot(t)
	for name, tc := range map[string]struct{ body, why string }{
		"no fix surface":  {strings.Replace(agentItem, `"files":["go/internal/fleet/pool.go"]`, `"files":[]`, 1), "fix surface"},
		"an unknown kind": {strings.Replace(agentItem, `"kind":"fix"`, `"kind":"wish"`, 1), `kind "wish"`},
		"a zero weight":   {strings.Replace(agentItem, `"weight":0.5`, `"weight":0`, 1), "weight"},
	} {
		_, err := fileAs(t, root, tc.body, "request:7/inbox-add-1", testDeps())
		if !errors.Is(err, agentrequest.ErrItemRefused) || !strings.Contains(causeOf(t, err), tc.why) {
			t.Errorf("%s: err = %v, want agentrequest.ErrItemRefused naming %q", name, err, tc.why)
		}
	}
}

func TestFiler_AnItemThatIsNotJSONIsRefused(t *testing.T) {
	t.Parallel()
	root := inboxtest.EmptyRoot(t)

	_, err := fileAs(t, root, "not json", "request:7/inbox-add-1", testDeps())

	if !errors.Is(err, agentrequest.ErrItemRefused) {
		t.Fatalf("err = %v, want agentrequest.ErrItemRefused", err)
	}
}

func TestFiler_AnOverlapRefusalCarriesTheTypedOverlapAndNoVerbAdvice(t *testing.T) {
	t.Parallel()
	root := inboxtest.EmptyRoot(t)
	if _, err := fileAs(t, root, poolItem, "request:7/inbox-add-1", testDeps()); err != nil {
		t.Fatal(err)
	}
	dup := strings.Replace(poolItem, `"id":"rolling-pool"`, `"id":"pool-backfill"`, 1)

	_, err := fileAs(t, root, dup, "request:7/inbox-add-2", testDeps())

	var overlap *inboxcurator.OverlapError
	if !errors.As(err, &overlap) || strings.Contains(causeOf(t, err), "--allow-overlap") || strings.Contains(causeOf(t, err), "--reason") {
		t.Fatalf("err = %v, want the typed overlap and a cause with no flag advice of the verb", err)
	}
}

func TestFiler_AnOverlapIsARefusalThatNamesTheOpenItem(t *testing.T) {
	t.Parallel()
	root := inboxtest.EmptyRoot(t)
	if _, err := fileAs(t, root, poolItem, "request:7/inbox-add-1", testDeps()); err != nil {
		t.Fatal(err)
	}
	dup := strings.Replace(poolItem, `"id":"rolling-pool"`, `"id":"pool-backfill"`, 1)

	_, err := fileAs(t, root, dup, "request:7/inbox-add-2", testDeps())

	if !errors.Is(err, agentrequest.ErrItemRefused) || !strings.Contains(causeOf(t, err), "rolling-pool") {
		t.Fatalf("err = %v, want agentrequest.ErrItemRefused that names rolling-pool", err)
	}
}

func TestFiler_AReplayOfTheSameRequestReturnsTheItemAlreadyFiled(t *testing.T) {
	t.Parallel()
	root := inboxtest.EmptyRoot(t)
	first, err := fileAs(t, root, agentItem, "request:7/inbox-add-1", testDeps())
	if err != nil {
		t.Fatal(err)
	}

	again, err := fileAs(t, root, agentItem, "request:7/inbox-add-1", testDeps())

	if err != nil || again.ID != first.ID || again.Path != first.Path {
		t.Fatalf("replay = %+v, %v; want the item of the first filing %+v", again, err, first)
	}
	if matches, _ := filepath.Glob(filepath.Join(root, ".evolve", "inbox", "*-agent-finding.json")); len(matches) != 1 {
		t.Fatalf("a replay files no second item, got %v", matches)
	}
	_, err = fileAs(t, root, agentItem, "request:8/inbox-add-1", testDeps())
	if !errors.Is(err, agentrequest.ErrItemRefused) || !strings.Contains(causeOf(t, err), "already filed") {
		t.Fatalf("the same id from another request: err = %v, want agentrequest.ErrItemRefused naming already filed", err)
	}
}

func TestFiler_TwoFilersOfTheSameIDFileOneItem(t *testing.T) {
	t.Parallel()
	root := inboxtest.EmptyRoot(t)
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = fileAs(t, root, agentItem, "request:"+string(rune('a'+i))+"/inbox-add-1", testDeps())
		}()
	}
	wg.Wait()
	var filed, refused int
	for _, err := range errs {
		switch {
		case err == nil:
			filed++
		case errors.Is(err, agentrequest.ErrItemRefused):
			refused++
		default:
			t.Errorf("unexpected failure: %v", err)
		}
	}
	if matches, _ := filepath.Glob(filepath.Join(root, ".evolve", "inbox", "*-agent-finding.json")); len(matches) != 1 || filed != 1 || refused != 7 {
		t.Fatalf("filed %d, refused %d, files %v; want one item and seven refusals", filed, refused, matches)
	}
}

func TestIntakeItem_VerbAndFilerShareOneIntake(t *testing.T) {
	verbRoot, filerRoot := inboxtest.EmptyRoot(t), inboxtest.EmptyRoot(t)
	for _, root := range []string{verbRoot, filerRoot} {
		if _, err := fileAs(t, root, poolItem, "request:1/inbox-add-1", testDeps()); err != nil {
			t.Fatal(err)
		}
	}
	dup := strings.Replace(poolItem, `"id":"rolling-pool"`, `"id":"pool-backfill"`, 1)
	t.Setenv("EVOLVE_PROJECT_ROOT", verbRoot)

	var stdout, stderr bytes.Buffer
	rc := runInboxCmd([]string{"add", "--file", "-", "--origin", "cycle"}, strings.NewReader(dup), &stdout, &stderr)
	_, err := fileAs(t, filerRoot, dup, "request:7/inbox-add-1", testDeps())

	if rc != exitRefused || !errors.Is(err, agentrequest.ErrItemRefused) {
		t.Fatalf("verb rc = %d, filer err = %v; both must refuse the overlap", rc, err)
	}
	for _, line := range strings.Split(strings.TrimSpace(stderr.String()), "\n") {
		if strings.Contains(line, "overlaps open item") && !strings.Contains(causeOf(t, err), strings.TrimPrefix(line, "inbox add: ")) {
			t.Errorf("the filer cause %q lacks the verb line %q", causeOf(t, err), line)
		}
	}
}

func TestFiler_FilesOnlyUnderTheLockOfTheInbox(t *testing.T) {
	t.Parallel()
	root := inboxtest.EmptyRoot(t)
	release, err := verifylock.AcquireAt(context.Background(), filepath.Join(root, ".evolve", "inbox", inboxfile.LockName), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	_, err = NewFiler(testDeps(), io.Discard)(ctx, root, []byte(agentItem), "request:7/inbox-add-1")

	release()
	if err == nil || errors.Is(err, agentrequest.ErrItemRefused) {
		t.Fatalf("err = %v, want a failure of the lock, not a refusal of the request", err)
	}
	if matches, _ := filepath.Glob(filepath.Join(root, ".evolve", "inbox", "*-agent-finding.json")); len(matches) != 0 {
		t.Fatalf("a filer that does not hold the lock files nothing, got %v", matches)
	}
	if _, err := fileAs(t, root, agentItem, "request:7/inbox-add-1", testDeps()); err != nil {
		t.Fatalf("after the release the filer files: %v", err)
	}
}

func inboxWithMode(t *testing.T, mode os.FileMode) string {
	t.Helper()
	root := inboxtest.EmptyRoot(t)
	dir := filepath.Join(root, ".evolve", "inbox")
	release, err := verifylock.AcquireAt(context.Background(), filepath.Join(dir, inboxfile.LockName), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	release()
	if err := os.Chmod(dir, mode); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	return root
}

func TestFiler_AnInboxItCannotReadIsAFailureNotARefusal(t *testing.T) {
	t.Parallel()
	root := inboxWithMode(t, 0o300)

	_, err := fileAs(t, root, agentItem, "request:7/inbox-add-1", testDeps())

	if err == nil || errors.Is(err, agentrequest.ErrItemRefused) {
		t.Fatalf("err = %v, want a failure of the pipeline", err)
	}
}

func TestFiler_AnInboxItCannotWriteIsAFailureNotARefusal(t *testing.T) {
	t.Parallel()
	root := inboxWithMode(t, 0o500)

	_, err := fileAs(t, root, agentItem, "request:7/inbox-add-1", testDeps())

	if err == nil || errors.Is(err, agentrequest.ErrItemRefused) {
		t.Fatalf("err = %v, want a failure of the pipeline", err)
	}
}

func TestFiler_APolicyThatDoesNotParseIsAFailureNotARefusal(t *testing.T) {
	t.Parallel()
	root := inboxtest.EmptyRoot(t)
	inboxtest.WritePolicy(t, root, `{"isolation":`)

	_, err := fileAs(t, root, agentItem, "request:7/inbox-add-1", testDeps())

	if err == nil || errors.Is(err, agentrequest.ErrItemRefused) {
		t.Fatalf("err = %v, want a failure of the pipeline", err)
	}
}

func TestInboxAdd_AnInboxTheVerbCannotReadFailsTheOverlapCheck(t *testing.T) {
	root := inboxWithMode(t, 0o300)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)

	if rc, _, stderr := addItem(t, poolItem); rc != exitIO || !strings.Contains(stderr, "overlap check") {
		t.Errorf("rc = %d stderr = %q, want %d naming the overlap check", rc, stderr, exitIO)
	}
}

func TestRefusedError_NamesTheSentinelAndTheCause(t *testing.T) {
	t.Parallel()
	cause := errors.New("kind not listed")
	err := error(&RefusedError{Cause: "kind \"wish\" is not listed", Err: cause})

	if unwrapped := err.(*RefusedError).Unwrap(); len(unwrapped) != 2 || unwrapped[0] != agentrequest.ErrItemRefused || unwrapped[1] != cause {
		t.Fatalf("Unwrap() = %v, want the refusal sentinel and the cause", unwrapped)
	}
	if got, want := err.Error(), "the inbox intake refused the item: kind \"wish\" is not listed"; got != want || !errors.Is(err, agentrequest.ErrItemRefused) || !errors.Is(err, cause) {
		t.Fatalf("Error() = %q, want %q wrapping the sentinel", got, want)
	}
}
