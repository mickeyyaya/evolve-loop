package dispositionrouter

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStageIntent_EachStagingFaultIsNamedAndNothingIsStaged(t *testing.T) {
	boom := errors.New("disk full")
	cases := []struct {
		name    string
		setup   func(t *testing.T, dir string)
		intent  Intent
		wantErr string
	}{
		{"an unencodable weight", func(*testing.T, string) {}, Intent{Action: ActionEscalate, Weight: math.NaN()}, "dispositionrouter: encode intent"},
		{"the lock sidecar cannot be taken", func(t *testing.T, dir string) {
			if err := os.Mkdir(filepath.Join(dir, PendingActionsFile+".lock"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, Intent{Action: ActionEscalate}, ""},
		{"the staging file is a directory", func(t *testing.T, dir string) {
			if err := os.Mkdir(filepath.Join(dir, PendingActionsFile), 0o755); err != nil {
				t.Fatal(err)
			}
		}, Intent{Action: ActionEscalate}, "dispositionrouter: open staging file"},
		{"the append fails", func(t *testing.T, dir string) {
			orig := stagingWrite
			t.Cleanup(func() { stagingWrite = orig })
			stagingWrite = func(*os.File, []byte) (int, error) { return 0, boom }
		}, Intent{Action: ActionEscalate}, "dispositionrouter: append intent"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "escalations")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			tc.setup(t, dir)

			path, err := StageIntent(dir, tc.intent)

			if err == nil || path != "" || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("path=%q err=%v, want %q", path, err, tc.wantErr)
			}
			if info, statErr := os.Stat(filepath.Join(dir, PendingActionsFile)); statErr == nil && !info.IsDir() && info.Size() != 0 {
				t.Errorf("a failed stage must not leave a partial record: size=%d", info.Size())
			}
		})
	}
}

func TestStageIntent_AppendsOneRecordPerCallAndReadsBack(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "escalations")
	for _, id := range []string{"a", "b"} {
		if _, err := StageIntent(dir, Intent{Cycle: 4, Pattern: "p", ItemID: id, Action: ActionEscalate, Route: "console", Recurrence: 3, Weight: 1.5}); err != nil {
			t.Fatal(err)
		}
	}

	got, err := LoadIntents(PendingActionsPath(dir))

	if err != nil || len(got) != 2 || got[0].ItemID != "a" || got[1].ItemID != "b" || got[1].Weight != 1.5 {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}
