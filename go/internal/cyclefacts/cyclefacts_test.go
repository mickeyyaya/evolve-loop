package cyclefacts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	fixtureWorkspace = "testdata/runs/cycle-2157"
	fixtureEvolveDir = "testdata"
)

type reader[T any] struct {
	name string
	file func(evolveDir string) string
	read func(evolveDir string) Read[T]
}

func workspaceReader[T any](file string, read func(string) Read[T]) reader[T] {
	return reader[T]{
		name: file,
		file: func(dir string) string { return filepath.Join(dir, file) },
		read: read,
	}
}

func assertStates[T any](t *testing.T, r reader[T], presentDir string, check func(T) string) {
	t.Helper()
	present := r.read(presentDir)
	if present.State != StatePresent || present.Err != nil || present.Path != r.file(presentDir) {
		t.Errorf("%s present: %+v, want state present, no error, path %s", r.name, present, r.file(presentDir))
	}
	if msg := check(present.Value); msg != "" {
		t.Errorf("%s present: %s", r.name, msg)
	}

	absent := r.read(t.TempDir())
	if absent.State != StateAbsent || absent.Err != nil {
		t.Errorf("%s absent: %+v, want state absent and no error", r.name, absent)
	}

	corruptDir := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(r.file(corruptDir)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(r.file(corruptDir), []byte(`{"torn`), 0o644); err != nil {
		t.Fatal(err)
	}
	corrupt := r.read(corruptDir)
	if corrupt.State != StateCorrupt || corrupt.Err == nil || !strings.Contains(corrupt.Err.Error(), filepath.Base(r.file(corruptDir))) {
		t.Errorf("%s corrupt: %+v, want state corrupt and an error that names the file", r.name, corrupt)
	}

	unreadableDir := t.TempDir()
	if err := os.MkdirAll(r.file(unreadableDir), 0o755); err != nil {
		t.Fatal(err)
	}
	unreadable := r.read(unreadableDir)
	if unreadable.State != StateUnreadable || unreadable.Err == nil {
		t.Errorf("%s unreadable: %+v, want state unreadable and an error", r.name, unreadable)
	}
}

func TestReadDecision_ReturnsEachState(t *testing.T) {
	t.Parallel()
	assertStates(t, workspaceReader(DecisionFile, ReadDecision), fixtureWorkspace, func(d Decision) string {
		if d.Category != "code-build-fail" || d.Level != "task" || d.Action != "defer-or-quarantine" || d.FixType != "pipeline-repair" {
			return "decision = " + d.Category + "/" + d.Level + "/" + d.Action + "/" + d.FixType
		}
		return ""
	})
}

func TestReadDisposition_ReturnsEachState(t *testing.T) {
	t.Parallel()
	assertStates(t, workspaceReader(DispositionFile, ReadDisposition), fixtureWorkspace, func(d Disposition) string {
		if d.Routing != "console" || d.RootCause.Layer != "pipeline-code" || !d.Salvage.WorktreeHasValue ||
			!strings.HasPrefix(d.Salvage.Pointer, ".evolve/worktrees/cycle-cd3ae73e-2157 ") || d.Urgency != "P1" || d.Legitimacy != "legit-rejection" {
			return "disposition not read verbatim"
		}
		return ""
	})
}

func TestReadFailReason_ReturnsEachState(t *testing.T) {
	t.Parallel()
	assertStates(t, workspaceReader(FailReasonFile, ReadFailReason), fixtureWorkspace, func(f FailReason) string {
		if f.Phase != "build" || len(f.Reasons) != 1 || !strings.HasPrefix(f.Reasons[0], "resume review gate: phase \"build\" deliverable rejected after 2 correction(s)") {
			return "reasons not read verbatim"
		}
		return ""
	})
}

func TestReadDigestRecord_ReturnsEachState(t *testing.T) {
	t.Parallel()
	const cycle = 2157
	r := reader[DigestRecord]{
		name: "digest record",
		file: func(dir string) string { return filepath.Join(dir, "failure-digests", "cycle-2157.json") },
		read: func(dir string) Read[DigestRecord] { return ReadDigestRecord(dir, cycle) },
	}
	assertStates(t, r, fixtureEvolveDir, func(d DigestRecord) string {
		if d.PreClass != "infra-error" || d.ClassRecurrence != 2 || d.ClassRoute != "console" || d.ClassLayer != "pipeline-code" ||
			len(d.Checks) != 1 || d.Checks[0] != "composed-floor" {
			return "digest record not read verbatim"
		}
		return ""
	})
}

func TestReadDecision_ACorruptFileKeepsNoPartialValue(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, DecisionFile), []byte(`{"level":"task","category":5}`), 0o644); err != nil {
		t.Fatal(err)
	}

	r := ReadDecision(ws)

	if r.State != StateCorrupt || r.Value != (Decision{}) || r.Err == nil {
		t.Errorf("read = %+v, want corrupt state, zero value and an error", r)
	}
}
