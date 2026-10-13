package inboxmover

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

func outcomeOptsF4(root string) Options {
	return Options{ProjectRoot: root, Stderr: io.Discard, IsLandedFn: func(string) (bool, error) { return true, nil }}
}

func seedInboxItemF4(t *testing.T, root, id string) {
	t.Helper()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "item-"+id+".json"), []byte(`{"id":"`+id+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

func blockInboxDirF4(t *testing.T, root, name string) {
	t.Helper()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, name), []byte("a file where a directory belongs"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestApplyCycleOutcome_PassPathReportsEachFailingStageAndStillDrains(t *testing.T) {
	cases := []struct {
		name     string
		setup    func(t *testing.T, root string)
		ids      []string
		wantErr  string
		promoted int
	}{
		{"a promote that cannot create its destination", func(t *testing.T, root string) {
			seedInboxItemF4(t, root, "a")
			blockInboxDirF4(t, root, "processed")
		}, []string{"a"}, `promote "a"`, 0},
		{"an unbacked id whose retirement record cannot be written", func(t *testing.T, root string) {
			blockInboxDirF4(t, root, "processed")
		}, []string{"ghost"}, `retire-unbacked "ghost"`, 0},
		{"a residual drain that cannot open the cycle directory", func(t *testing.T, root string) {
			seedInboxItemF4(t, root, "a")
			blockInboxDirF4(t, root, "processing")
		}, []string{"a"}, "residual drain", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.setup(t, root)

			res, err := ApplyCycleOutcome(outcomeOptsF4(root), CycleOutcome{Cycle: 7, Passed: true, CommittedIDs: tc.ids})

			if err == nil || !strings.Contains(err.Error(), "apply-cycle-outcome:") || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.wantErr)
			}
			if len(res.Promoted) != tc.promoted {
				t.Errorf("promoted = %v, want %d id(s): a failed later stage must not hide earlier moves", res.Promoted, tc.promoted)
			}
		})
	}
}

func TestApplyCycleOutcome_PassPathJoinsEveryFailureNotOnlyTheFirst(t *testing.T) {
	root := t.TempDir()
	seedInboxItemF4(t, root, "a")
	blockInboxDirF4(t, root, "processed")
	blockInboxDirF4(t, root, "processing")

	_, err := ApplyCycleOutcome(outcomeOptsF4(root), CycleOutcome{Cycle: 7, Passed: true, CommittedIDs: []string{"a", "ghost"}})

	if err == nil {
		t.Fatal("want a joined error")
	}
	for _, want := range []string{`promote "a"`, `retire-unbacked "ghost"`, "residual drain"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("joined error lacks %q: %v", want, err)
		}
	}
}

func TestApplyCycleOutcome_FailPathSurfacesClaimAndDrainErrors(t *testing.T) {
	t.Run("drain failure is wrapped", func(t *testing.T) {
		root := t.TempDir()
		blockInboxDirF4(t, root, "processing")

		_, err := ApplyCycleOutcome(outcomeOptsF4(root), CycleOutcome{Cycle: 7})

		if err == nil || !strings.Contains(err.Error(), "apply-cycle-outcome: failure drain:") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestOriginMainHead_UnrunnableGitIsWrappedWithTheRef(t *testing.T) {
	head, err := originMainHead(filepath.Join(t.TempDir(), "absent"))
	if err == nil || head != "" || !strings.Contains(err.Error(), "git rev-parse origin/main") {
		t.Fatalf("head=%q err=%v", head, err)
	}
}

func TestResolveClaim_SkipsUnreadableAndMalformedClaimsThenFindsTheStampedOne(t *testing.T) {
	root := t.TempDir()
	cycleDir := filepath.Join(root, ".evolve", "inbox", "processing", "cycle-9")
	if err := os.MkdirAll(cycleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cycleDir, "a-malformed.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	unreadable := filepath.Join(cycleDir, "b-unreadable.json")
	if err := os.WriteFile(unreadable, []byte(`{"id":"b","continuation":{"snapshot_sha":"x","cycle":1}}`), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o644) })
	if f, err := os.Open(unreadable); err == nil {
		f.Close()
		t.Skip("file permissions are not enforced for this user")
	}
	if err := os.WriteFile(filepath.Join(cycleDir, "c-unstamped.json"), []byte(`{"id":"c"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	stamped, _ := json.Marshal(map[string]any{"id": "d", "continuation": continuation.Continuation{Cycle: 4, SnapshotSHA: "deadbeef"}})
	if err := os.WriteFile(filepath.Join(cycleDir, "d-stamped.json"), stamped, 0o644); err != nil {
		t.Fatal(err)
	}

	got := ResolveContinuation(Options{ProjectRoot: root}, 9)

	if got == nil || got.SnapshotSHA != "deadbeef" || got.Cycle != 4 {
		t.Fatalf("resolved = %+v", got)
	}
}

func TestAppendReleasedContinuation_ExistingListIsExtendedAndMalformedOneReplaced(t *testing.T) {
	at := time.Date(2026, 10, 1, 2, 3, 4, 0, time.UTC)
	c := continuation.Continuation{Cycle: 3, SnapshotSHA: "abc123"}
	cases := []struct {
		name     string
		existing string
		wantLen  int
	}{
		{"no previous list", ``, 1},
		{"valid previous list is extended", `"released_continuations":[{"snapshot_sha":"old"}],`, 2},
		{"malformed previous value is replaced", `"released_continuations":"oops",`, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "item.json")
			if err := os.WriteFile(path, []byte(`{`+tc.existing+`"id":"x"}`), 0o644); err != nil {
				t.Fatal(err)
			}

			if err := appendReleasedContinuation(path, c, "retired", "runtime", at); err != nil {
				t.Fatal(err)
			}

			var doc struct {
				ID       string                   `json:"id"`
				Released []map[string]interface{} `json:"released_continuations"`
			}
			body, _ := os.ReadFile(path)
			if err := json.Unmarshal(body, &doc); err != nil {
				t.Fatal(err)
			}
			if doc.ID != "x" || len(doc.Released) != tc.wantLen {
				t.Fatalf("doc = %+v", doc)
			}
			last := doc.Released[len(doc.Released)-1]
			if last["snapshot_sha"] != "abc123" || last["released_at"] != "2026-10-01T02:03:04Z" || last["reason"] != "retired" || last["released_by"] != "runtime" {
				t.Errorf("last entry = %v", last)
			}
		})
	}
}

func TestAppendReleasedContinuation_FailuresAreReturned(t *testing.T) {
	t.Run("item file absent", func(t *testing.T) {
		err := appendReleasedContinuation(filepath.Join(t.TempDir(), "absent.json"), continuation.Continuation{}, "r", "b", time.Now())
		if err == nil {
			t.Fatal("want a read error")
		}
	})
	t.Run("entry cannot be encoded", func(t *testing.T) {
		orig := marshalReleasedContinuation
		t.Cleanup(func() { marshalReleasedContinuation = orig })
		boom := errors.New("encode failed")
		marshalReleasedContinuation = func(releasedContinuation) ([]byte, error) { return nil, boom }
		path := filepath.Join(t.TempDir(), "item.json")
		if err := os.WriteFile(path, []byte(`{"id":"x"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := appendReleasedContinuation(path, continuation.Continuation{}, "r", "b", time.Now()); !errors.Is(err, boom) {
			t.Fatalf("err = %v", err)
		}
		if body, _ := os.ReadFile(path); string(body) != `{"id":"x"}` {
			t.Errorf("the item must stay untouched: %s", body)
		}
	})
}
