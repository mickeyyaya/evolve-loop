package auditledger

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeLedger(t *testing.T, lines ...string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "ledger.jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const (
	rowOld     = `{"ts":"2026-01-01T00:00:00Z","role":"auditor","kind":"agent_subprocess","run_id":"A","git_head":"shaOld","artifact_path":"/a.md","exit_code":1}`
	rowNew     = `{"role":"auditor","kind":"agent_subprocess","run_id":"B","git_head":"shaNew","worktree_tree_sha":"wt","tree_state_sha":"ts","artifact_sha256":"x"}`
	rowBuilder = `{"role":"builder","kind":"agent_subprocess","run_id":"B","git_head":"shaBuilder"}`
	rowVerdict = `{"role":"auditor","kind":"phase_verdict","run_id":"B","git_head":"shaVerdict"}`
)

func TestAuditorRows_NewestFirstAndOnlyAuditorSubprocessRows(t *testing.T) {
	rows, err := AuditorRows(writeLedger(t, rowOld, "{not json", rowBuilder, rowNew, rowVerdict, ""))
	if err != nil {
		t.Fatalf("AuditorRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want the 2 auditor subprocess rows: %+v", len(rows), rows)
	}
	want := Entry{Role: "auditor", Kind: "agent_subprocess", RunID: "B", GitHEAD: "shaNew",
		WorktreeTreeSHA: "wt", TreeStateSHA: "ts", ArtifactSHA256: "x"}
	if rows[0] != want {
		t.Errorf("rows[0] = %+v, want %+v", rows[0], want)
	}
	if rows[1].TS != "2026-01-01T00:00:00Z" || rows[1].ExitCode != 1 || rows[1].ArtifactPath != "/a.md" {
		t.Errorf("rows[1] = %+v, want the older row's ts, exit_code and artifact_path", rows[1])
	}
}

func TestAuditorRows_ReadErrorsWrapTheOSError(t *testing.T) {
	_, err := AuditorRows(filepath.Join(t.TempDir(), "absent.jsonl"))
	if !errors.Is(err, fs.ErrNotExist) || errors.Is(err, ErrNoAuditorForRun) {
		t.Errorf("absent ledger: err=%v, want fs.ErrNotExist and not the miss sentinel", err)
	}
	_, err = AuditorRows(t.TempDir())
	if err == nil || errors.Is(err, fs.ErrNotExist) || errors.Is(err, ErrNoAuditorForRun) {
		t.Errorf("unreadable ledger: err=%v, want a read error that is neither absence nor a miss", err)
	}
}

func TestBindRun(t *testing.T) {
	older := Entry{RunID: "A", GitHEAD: "shaOld"}
	newer := Entry{RunID: "B", GitHEAD: "shaNew"}
	unstamped := Entry{GitHEAD: "shaBare"}
	cases := []struct {
		name, runID string
		rows        []Entry
		want        Entry
		wantErrText []string
	}{
		{name: "no run binds the newest", rows: []Entry{newer, older}, want: newer},
		{name: "run binds its own row behind a newer sibling", runID: "A", rows: []Entry{newer, older}, want: older},
		{name: "no rows without a run", wantErrText: []string{"no Auditor ledger entry found"}},
		{name: "no rows for a run", runID: "A", wantErrText: []string{"for run A"}},
		{name: "foreign run is refused and named", runID: "Z", rows: []Entry{newer, older},
			wantErrText: []string{"for run Z", "foreign run B", "git_head=shaNew"}},
		{name: "unstamped row never binds a run", runID: "Z", rows: []Entry{unstamped},
			wantErrText: []string{"foreign run <unstamped>", "git_head=shaBare"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := BindRun(c.rows, c.runID)
			if c.wantErrText == nil {
				if err != nil || got != c.want {
					t.Fatalf("BindRun = %+v, %v; want %+v", got, err, c.want)
				}
				return
			}
			if !errors.Is(err, ErrNoAuditorForRun) {
				t.Fatalf("BindRun err=%v, want ErrNoAuditorForRun", err)
			}
			for _, w := range c.wantErrText {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("err %q does not contain %q", err, w)
				}
			}
		})
	}
}

func TestLatestAuditorEntry(t *testing.T) {
	ledger := writeLedger(t, rowOld, rowNew, rowVerdict)
	e, err := LatestAuditorEntry(ledger, "A")
	if err != nil || e.GitHEAD != "shaOld" {
		t.Errorf("run A: got %+v, %v; want shaOld", e, err)
	}
	e, err = LatestAuditorEntry(ledger, "")
	if err != nil || e.GitHEAD != "shaNew" {
		t.Errorf("no run: got %+v, %v; want shaNew (the phase_verdict row is not an auditor row)", e, err)
	}
	if _, err := LatestAuditorEntry(ledger, "Z"); !errors.Is(err, ErrNoAuditorForRun) {
		t.Errorf("foreign-only: err=%v, want ErrNoAuditorForRun", err)
	}
	if _, err := LatestAuditorEntry(filepath.Join(t.TempDir(), "absent.jsonl"), "A"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("absent ledger: err=%v, want fs.ErrNotExist", err)
	}
}
