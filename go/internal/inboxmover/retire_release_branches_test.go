package inboxmover

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

func retireOptions(root string, stderr io.Writer) Options {
	return Options{ProjectRoot: root, Stderr: stderr, Now: time.Now}
}

func lockDown(t *testing.T, dir string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

func readOnly(t *testing.T, dir string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

func TestReleaseContinuationOnRetire_AnIdThatNamesNoItemReleasesNothing(t *testing.T) {
	t.Parallel()
	root, _ := reconcileFixture(t)
	bindScope(t, root, "unknown", 3)
	for _, id := range []string{"", "unknown"} {
		releaseContinuationOnRetire(retireOptions(root, &bytes.Buffer{}), filepath.Join(root, "x.json"), id, "r")
	}
	if !stillBound(t, root, "unknown") {
		t.Error("a retirement with no item id released a binding")
	}
}

func TestReleaseContinuationOnRetire_AnUnreadableRegistryIsReportedAndKept(t *testing.T) {
	t.Parallel()
	root, _ := reconcileFixture(t)
	if err := os.WriteFile(continuation.RegistryPath(root), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	releaseContinuationOnRetire(retireOptions(root, &stderr), filepath.Join(root, "x.json"), "a", "r")
	if !strings.Contains(stderr.String(), "registry unreadable") {
		t.Errorf("stderr = %q, want the unreadable registry reported", stderr.String())
	}
}

func TestReleaseContinuationOnRetire_APointerThatCannotBeWrittenStillReleasesTheBinding(t *testing.T) {
	t.Parallel()
	root, _ := reconcileFixture(t)
	bindScope(t, root, "a", 3)
	var stderr bytes.Buffer
	releaseContinuationOnRetire(retireOptions(root, &stderr), filepath.Join(root, "no-such-dir", "x.json"), "a", "r")
	if !strings.Contains(stderr.String(), "NOT written") || stillBound(t, root, "a") {
		t.Errorf("stderr = %q bound = %v, want the pointer fault reported and the binding released", stderr.String(), stillBound(t, root, "a"))
	}
}

func TestReleaseContinuationOnRetire_AFailedRegistryDeleteIsReportedAndKept(t *testing.T) {
	root, _ := reconcileFixture(t)
	bindScope(t, root, "a", 3)
	item := seedItemIn(t, filepath.Join(t.TempDir(), "inbox"), "a")
	readOnly(t, filepath.Join(root, ".evolve"))
	var stderr bytes.Buffer
	releaseContinuationOnRetire(retireOptions(root, &stderr), item, "a", "r")
	if !strings.Contains(stderr.String(), "binding release failed") || !stillBound(t, root, "a") {
		t.Errorf("stderr = %q, want the failed release reported and the binding kept", stderr.String())
	}
}

func TestReleaseContinuationOnRetire_ABindingReboundByAnotherLaneIsLeftIntact(t *testing.T) {
	root, _ := reconcileFixture(t)
	bindScope(t, root, "a", 3)
	item := seedItemIn(t, filepath.Join(t.TempDir(), "inbox"), "a")
	orig := marshalReleasedContinuation
	t.Cleanup(func() { marshalReleasedContinuation = orig })
	marshalReleasedContinuation = func(v releasedContinuation) ([]byte, error) {
		bindScope(t, root, "a", 4)
		return orig(v)
	}
	var stderr bytes.Buffer
	releaseContinuationOnRetire(retireOptions(root, &stderr), item, "a", "r")
	if !strings.Contains(stderr.String(), "rebound by another lane") || !stillBound(t, root, "a") {
		t.Errorf("stderr = %q, want the rebound binding left intact and reported", stderr.String())
	}
}

func TestResolveContinuationForScope_ADeadBindingThatCannotBeReleasedIsReported(t *testing.T) {
	root, _ := reconcileFixture(t)
	seedItemIn(t, filepath.Join(root, ".evolve", "inbox", "quarantine"), "ghost")
	bindScope(t, root, "ghost", 5)
	readOnly(t, filepath.Join(root, ".evolve"))
	var stderr bytes.Buffer
	got, err := ResolveContinuationForScope(Options{ProjectRoot: root, Stderr: &stderr}, 9, []string{"ghost"})
	if err != nil || got != nil || !strings.Contains(stderr.String(), "could not release dead binding") {
		t.Errorf("got=%v err=%v stderr=%q, want no continuation and the failed release reported", got, err, stderr.String())
	}
}

func TestClaimListFaults_AreReturnedByTheSurveyAndTheStaleSweep(t *testing.T) {
	root, _ := reconcileFixture(t)
	lockDown(t, filepath.Join(root, ".evolve", "inbox"))
	opts := Options{ProjectRoot: root, Stderr: &bytes.Buffer{}}
	if _, err := SurveyClaims(opts); err == nil {
		t.Error("SurveyClaims must report an inbox that cannot be listed")
	}
	if _, err := ReleaseStaleClaims(opts, "stale"); err == nil {
		t.Error("ReleaseStaleClaims must report an inbox that cannot be listed")
	}
}

func TestReconcileConsumedBindings_AnEntryThatIsNoJSONItemIsSkipped(t *testing.T) {
	t.Parallel()
	root, consumed := reconcileFixture(t)
	writeConsumed(t, consumed, "junk.json", "{not json")
	if got := mustReconcile(t, Options{ProjectRoot: root, Stderr: &bytes.Buffer{}}); len(got) != 0 {
		t.Fatalf("released = %v, want none for an entry with no readable id", got)
	}
}
