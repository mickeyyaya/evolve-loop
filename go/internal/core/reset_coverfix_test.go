package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type hookedLedger struct{ onAppend func() error }

func (h hookedLedger) Append(context.Context, LedgerEntry) error { return h.onAppend() }

func sealErrContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("err = %v, want it to contain %q", err, want)
	}
}

func TestSealCycleCoverfix_DefaultClockAndHeadStillSealACycle(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	sealFixture(t, evolveDir, 7)
	res, err := SealCycle(context.Background(), nil, SealOptions{EvolveDir: evolveDir, ProjectRoot: evolveDir})
	if err != nil {
		t.Fatalf("SealCycle: %v", err)
	}
	if res.SealedCycleID != 7 {
		t.Fatalf("SealedCycleID = %d, want 7", res.SealedCycleID)
	}
	if _, statErr := os.Stat(res.ArchiveDir); statErr != nil {
		t.Fatalf("archive dir missing: %v", statErr)
	}
}

func TestSealCycleCoverfix_CycleStateFailuresAreNamed(t *testing.T) {
	t.Parallel()
	t.Run("unreadable", func(t *testing.T) {
		t.Parallel()
		evolveDir := t.TempDir()
		if err := os.Mkdir(filepath.Join(evolveDir, "cycle-state.json"), 0o755); err != nil {
			t.Fatal(err)
		}
		_, err := SealCycle(context.Background(), nil, sealOpts(evolveDir))
		sealErrContains(t, err, "reset: read cycle-state")
		if errors.Is(err, ErrNothingToReset) {
			t.Fatalf("a read failure must not look like nothing-to-reset: %v", err)
		}
	})
	t.Run("unparsable", func(t *testing.T) {
		t.Parallel()
		evolveDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(evolveDir, "cycle-state.json"), []byte("{not json"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := SealCycle(context.Background(), nil, sealOpts(evolveDir))
		sealErrContains(t, err, "reset: parse cycle-state")
	})
}

func TestSealCycleCoverfix_MissingWorkspacePathDefaultsUnderRuns(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	writeJSONFixture(t, filepath.Join(evolveDir, "cycle-state.json"), map[string]any{"cycle_id": 9, "phase": "build"})
	writeJSONFixture(t, filepath.Join(evolveDir, "state.json"), map[string]any{"lastCycleNumber": 8})
	res, err := SealCycle(context.Background(), nil, sealOpts(evolveDir))
	if err != nil {
		t.Fatalf("SealCycle: %v", err)
	}
	want := filepath.Join(evolveDir, "runs", "cycle-9")
	if res.Workspace != want {
		t.Fatalf("Workspace = %q, want %q", res.Workspace, want)
	}
	if _, statErr := os.Stat(filepath.Join(res.ArchiveDir, "reset-manifest.json")); statErr != nil {
		t.Fatalf("manifest must be written into the archive of a never-created workspace: %v", statErr)
	}
}

func TestSealCycleCoverfix_ManifestMarshalFailureAbortsBeforeAnyMutation(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	ws := sealFixture(t, evolveDir, 3)
	opts := sealOpts(evolveDir)
	opts.seams.marshalManifest = func(any, string, string) ([]byte, error) { return nil, errors.New("boom") }
	_, err := SealCycle(context.Background(), nil, opts)
	sealErrContains(t, err, "reset: marshal manifest")
	if _, statErr := os.Stat(ws); statErr != nil {
		t.Fatalf("workspace must be untouched: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(evolveDir, "cycle-state.json")); statErr != nil {
		t.Fatalf("cycle-state.json must survive an aborted seal: %v", statErr)
	}
}

func TestSealCycleCoverfix_MetadataWriteFailuresLeaveTheWorkspaceInPlace(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, blocker, want string
	}{
		{"snapshot", "cycle-state.snapshot.json", "reset: write snapshot"},
		{"manifest", "reset-manifest.json", "reset: write manifest"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			evolveDir := t.TempDir()
			ws := sealFixture(t, evolveDir, 4)
			if err := os.Mkdir(filepath.Join(ws, tc.blocker), 0o755); err != nil {
				t.Fatal(err)
			}
			_, err := SealCycle(context.Background(), nil, sealOpts(evolveDir))
			sealErrContains(t, err, tc.want)
			if _, statErr := os.Stat(filepath.Join(ws, "scout-report.md")); statErr != nil {
				t.Fatalf("workspace must stay intact for a clean retry: %v", statErr)
			}
		})
	}
}

func TestSealCycleCoverfix_LearningWriteFailureIsHardBeforeTheRename(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		createWorkdir bool
	}{
		{"live workspace", true},
		{"never-created workspace", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			evolveDir := t.TempDir()
			ws := sealFixture(t, evolveDir, 5)
			if !tc.createWorkdir {
				if err := os.RemoveAll(ws); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(evolveDir, "instincts"), []byte("a file, not a directory"), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := SealCycle(context.Background(), nil, sealOpts(evolveDir))
			sealErrContains(t, err, "reset: write learning artifacts")
			if tc.createWorkdir {
				if _, statErr := os.Stat(ws); statErr != nil {
					t.Fatalf("workspace must not be renamed when learning fails: %v", statErr)
				}
			}
			if _, statErr := os.Stat(filepath.Join(evolveDir, "cycle-state.json")); statErr != nil {
				t.Fatalf("cycle-state.json must survive: %v", statErr)
			}
		})
	}
}

func TestSealCycleCoverfix_ArchiveCannotBeCreatedOrRenamedOnto(t *testing.T) {
	t.Parallel()
	t.Run("rename onto a populated archive", func(t *testing.T) {
		t.Parallel()
		evolveDir := t.TempDir()
		sealFixture(t, evolveDir, 6)
		opts := sealOpts(evolveDir)
		probe, err := SealCycle(context.Background(), nil, SealOptions{EvolveDir: evolveDir, ProjectRoot: evolveDir, Now: opts.Now, GitHead: opts.GitHead, DryRun: true})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(probe.ArchiveDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(probe.ArchiveDir, "occupant"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err = SealCycle(context.Background(), nil, opts)
		sealErrContains(t, err, "reset: archive workspace")
	})
	t.Run("archive path is a file", func(t *testing.T) {
		t.Parallel()
		evolveDir := t.TempDir()
		ws := sealFixture(t, evolveDir, 6)
		if err := os.RemoveAll(ws); err != nil {
			t.Fatal(err)
		}
		opts := sealOpts(evolveDir)
		probe, err := SealCycle(context.Background(), nil, SealOptions{EvolveDir: evolveDir, ProjectRoot: evolveDir, Now: opts.Now, GitHead: opts.GitHead, DryRun: true})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(probe.ArchiveDir, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err = SealCycle(context.Background(), nil, opts)
		sealErrContains(t, err, "reset: create archive")
	})
}

func TestSealCycleCoverfix_LedgerFailureSurfacesAndKeepsState(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	sealFixture(t, evolveDir, 8)
	led := hookedLedger{onAppend: func() error { return errors.New("ledger down") }}
	_, err := SealCycle(context.Background(), led, sealOpts(evolveDir))
	sealErrContains(t, err, "reset: append ledger")
	if _, statErr := os.Stat(filepath.Join(evolveDir, "cycle-state.json")); statErr != nil {
		t.Fatalf("cycle-state.json must survive a ledger failure: %v", statErr)
	}
}

func TestSealCycleCoverfix_StateFailuresAreNamedAndKeepCycleState(t *testing.T) {
	t.Parallel()
	t.Run("failed-approach record rejects a corrupt state.json", func(t *testing.T) {
		t.Parallel()
		evolveDir := t.TempDir()
		sealFixture(t, evolveDir, 10)
		if err := os.WriteFile(filepath.Join(evolveDir, "state.json"), []byte("{corrupt"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := SealCycle(context.Background(), nil, sealOpts(evolveDir))
		sealErrContains(t, err, "reset: record failed approach")
		if _, statErr := os.Stat(filepath.Join(evolveDir, "cycle-state.json")); statErr != nil {
			t.Fatalf("cycle-state.json must survive: %v", statErr)
		}
	})
	t.Run("state read", func(t *testing.T) {
		t.Parallel()
		evolveDir := t.TempDir()
		sealFixture(t, evolveDir, 10)
		opts := sealOpts(evolveDir)
		opts.seams.readState = func(string) (map[string]any, error) { return nil, errors.New("read boom") }
		_, err := SealCycle(context.Background(), nil, opts)
		sealErrContains(t, err, "reset: read state.json")
	})
	t.Run("state write", func(t *testing.T) {
		t.Parallel()
		evolveDir := t.TempDir()
		sealFixture(t, evolveDir, 10)
		opts := sealOpts(evolveDir)
		opts.seams.writeState = func(string, map[string]any) error { return errors.New("write boom") }
		_, err := SealCycle(context.Background(), nil, opts)
		sealErrContains(t, err, "reset: write state.json")
		if _, statErr := os.Stat(filepath.Join(evolveDir, "cycle-state.json")); statErr != nil {
			t.Fatalf("cycle-state.json is the abandon commit point and must survive a state write failure: %v", statErr)
		}
	})
}

func TestSealCycleCoverfix_ClearingCycleStateFailureIsReported(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	sealFixture(t, evolveDir, 11)
	csPath := filepath.Join(evolveDir, "cycle-state.json")
	led := hookedLedger{onAppend: func() error {
		if err := os.Remove(csPath); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(csPath, "occupant"), 0o755); err != nil {
			return err
		}
		return nil
	}}
	_, err := SealCycle(context.Background(), led, sealOpts(evolveDir))
	sealErrContains(t, err, "reset: clear cycle-state")
}

func TestPathWithin_RelativeTargetAgainstAbsoluteRootIsOutside(t *testing.T) {
	t.Parallel()
	if pathWithin("relative/dir", "/abs/root") {
		t.Fatal("a target that cannot be made relative to the root must not count as within it")
	}
	if !pathWithin("/abs/root/a/b", "/abs/root") {
		t.Fatal("nested path must be within root")
	}
	if pathWithin("/abs/rootless", "/abs/root") {
		t.Fatal("a sibling sharing a name prefix must not be within root")
	}
}
