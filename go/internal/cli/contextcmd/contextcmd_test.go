package contextcmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inputindex"
)

type cell struct {
	root, ws string
	idx      inputindex.Index
}

func newCell(t *testing.T) cell {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ws := filepath.Join(root, ".evolve", "runs", "cycle-7")
	for name, body := range map[string]string{"scout-report.md": "# Scout: replay\n", "test-report.md": "# TDD: 3 RED\n"} {
		if err := os.MkdirAll(ws, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idx, err := inputindex.Build(inputindex.Request{ProjectRoot: root, Workspace: ws, Cycle: 7, Phase: "build"})
	if err != nil {
		t.Fatal(err)
	}
	if err := inputindex.Write(ws, idx); err != nil {
		t.Fatal(err)
	}
	return cell{root: root, ws: ws, idx: idx}
}

func (c cell) env(key string) string {
	return map[string]string{"EVOLVE_PROJECT_ROOT": c.root, "EVOLVE_DISPATCH_ID": "run1/7/build/p12n3"}[key]
}

func (c cell) run(t *testing.T, env func(string) string, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := New(Deps{Env: env})(args, nil, &out, &errOut)
	return code, out.String(), errOut.String()
}

func decode(t *testing.T, raw string) envelope {
	t.Helper()
	var e envelope
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		t.Fatalf("stdout is not one envelope: %v\n%s", err, raw)
	}
	if e.Warnings == nil {
		t.Fatal("warnings is never absent")
	}
	return e
}

func idOf(t *testing.T, idx inputindex.Index, kind string) string {
	t.Helper()
	for _, e := range idx.Entries {
		if e.Kind == kind {
			return e.ID
		}
	}
	t.Fatalf("no %s entry", kind)
	return ""
}

func TestContextIndex_ListsTheRowsOfTheCellFromThePipelineMarkers(t *testing.T) {
	c := newCell(t)
	code, out, _ := c.run(t, c.env, "index", "--json")
	e := decode(t, out)
	if code != 0 || !e.OK || e.Schema != SchemaIndex || e.Error != nil {
		t.Fatalf("code %d envelope %+v", code, e)
	}
	var data inputindex.Index
	raw, _ := json.Marshal(e.Data)
	if err := json.Unmarshal(raw, &data); err != nil || len(data.Entries) != 2 || data.Cycle != 7 {
		t.Fatalf("data = %+v err %v", data, err)
	}
	code, out, _ = c.run(t, c.env, "index", "--required", "--json")
	raw, _ = json.Marshal(decode(t, out).Data)
	_ = json.Unmarshal(raw, &data)
	if code != 0 || len(data.Entries) != 1 || data.Entries[0].Kind != "scout-report" {
		t.Fatalf("--required keeps only the required rows: %+v", data.Entries)
	}
	code, out, _ = c.run(t, nil, "index", "--layer", "L2", "--workspace", c.ws)
	if code != 0 || strings.Contains(out, "scout-report") || !strings.Contains(out, "0 of 2 rows") {
		t.Fatalf("--layer L2 has no rows on a first attempt; text output = %q code %d", out, code)
	}
	code, out, _ = c.run(t, c.env, "index")
	if code != 0 || !strings.Contains(out, idOf(t, c.idx, "scout-report")+"\tL1\tscout-report\t") || !strings.Contains(out, "required") {
		t.Fatalf("text output = %q", out)
	}
}

func TestContextOpen_RefusesAnUnknownIDAForeignPathAndAChangedBody(t *testing.T) {
	c := newCell(t)
	scout := idOf(t, c.idx, "scout-report")
	code, out, _ := c.run(t, c.env, "open", scout)
	if code != 0 || out != "# Scout: replay\n" {
		t.Fatalf("open %s = %d %q", scout, code, out)
	}
	code, out, _ = c.run(t, c.env, "open", scout, "--json")
	e := decode(t, out)
	if code != 0 || e.Schema != SchemaOpen || !strings.Contains(out, `"body":"# Scout: replay\n"`) {
		t.Fatalf("open --json = %d %s", code, out)
	}
	code, out, _ = c.run(t, c.env, "open", "z9", "--json")
	if e := decode(t, out); code != exitInput || e.OK || e.Error == nil || e.Error.Code != exitInput || e.Error.Kind != "input" {
		t.Fatalf("an unknown id exits %d: %d %s", exitInput, code, out)
	}
	foreign := c.idx
	foreign.Entries = append(foreign.Entries, inputindex.Entry{ID: "x1", Path: "../../etc/passwd"})
	if err := inputindex.Write(c.ws, foreign); err != nil {
		t.Fatal(err)
	}
	if code, _, errOut := c.run(t, c.env, "open", "x1"); code != exitInput || !strings.Contains(errOut, "outside the project root") {
		t.Fatalf("a path outside the root exits %d: %d %q", exitInput, code, errOut)
	}
	if err := os.WriteFile(filepath.Join(c.ws, "scout-report.md"), []byte("# changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, errOut := c.run(t, c.env, "open", scout); code != exitFailed || !strings.Contains(errOut, "sha256") {
		t.Fatalf("a changed body exits %d: %d %q", exitFailed, code, errOut)
	}
	if err := os.Remove(filepath.Join(c.ws, "scout-report.md")); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := c.run(t, c.env, "open", scout); code != exitIO {
		t.Fatalf("a missing body exits %d, got %d", exitIO, code)
	}
}

func TestContext_UsageAndCellErrorsPrintAnEnvelopeOnEachExit(t *testing.T) {
	c := newCell(t)
	none := func(string) string { return "" }
	for name, tc := range map[string]struct {
		env  func(string) string
		args []string
		code int
		want string
	}{
		"no verb":               {c.env, nil, exitUsage, "usage"},
		"an unknown verb":       {c.env, []string{"show", "--json"}, exitUsage, `unknown verb "show"`},
		"an unknown flag":       {c.env, []string{"index", "--bogus", "--json"}, exitUsage, "bogus"},
		"a bad layer":           {c.env, []string{"index", "--layer", "L9", "--json"}, exitUsage, `layer "L9"`},
		"open with no id":       {c.env, []string{"open", "--json"}, exitUsage, "one entry id"},
		"no cell":               {none, []string{"index", "--json"}, exitUsage, "--workspace"},
		"a workspace of no run": {none, []string{"index", "--workspace", c.root, "--json"}, exitUsage, ".evolve/runs/"},
		"no index":              {none, []string{"index", "--workspace", filepath.Join(c.root, ".evolve", "runs", "cycle-8"), "--json"}, exitIO, "index.json"},
	} {
		code, out, errOut := c.run(t, tc.env, tc.args...)
		if code != tc.code {
			t.Errorf("%s: code %d, want %d (%s %s)", name, code, tc.code, out, errOut)
		}
		if strings.Contains(strings.Join(tc.args, " "), "--json") {
			e := decode(t, out)
			if e.OK || e.Error == nil || e.Error.Code != tc.code || !strings.Contains(e.Error.Message, tc.want) {
				t.Errorf("%s: envelope %+v, want an error that names %q", name, e, tc.want)
			}
		} else if !strings.Contains(errOut, tc.want) {
			t.Errorf("%s: stderr %q, want %q", name, errOut, tc.want)
		}
	}
	if err := os.WriteFile(inputindex.Path(c.ws), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := c.run(t, c.env, "open", "d1"); code != exitInput {
		t.Fatalf("an index that does not decode exits %d, got %d", exitInput, code)
	}
}

func TestContextStats_PrintsTheOpenRateOfEachKindFromTheAccessLog(t *testing.T) {
	c := newCell(t)
	scout := idOf(t, c.idx, "scout-report")
	if _, err := inputindex.RecordAccess(c.root, c.idx, "d1", inputindex.AccessSourceSandbox, []string{scout}); err != nil {
		t.Fatal(err)
	}
	code, out, _ := c.run(t, nil, "stats", "--phase", "build", "--project-root", c.root, "--json")
	e := decode(t, out)
	if code != 0 || e.Schema != SchemaStats || !strings.Contains(out, `"kind":"scout-report","layer":"L1","listed":1,"opened":1`) || !strings.Contains(out, `"kind":"test-report","layer":"L1","listed":1,"opened":0`) {
		t.Fatalf("stats --json = %d %s", code, out)
	}
	code, out, _ = c.run(t, nil, "stats", "--project-root", c.root)
	if code != 0 || !strings.Contains(out, "build\tscout-report\tL1\t1/1\t1.00") {
		t.Fatalf("stats text = %d %q", code, out)
	}
	if err := os.MkdirAll(filepath.Join(inputindex.AccessDir(c.root), "cycle-9.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := c.run(t, nil, "stats", "--project-root", c.root); code != exitIO {
		t.Fatalf("an unreadable access log exits %d, got %d", exitIO, code)
	}
	if code, _, errOut := c.run(t, nil, "stats", "--bogus"); code != exitUsage || !strings.Contains(errOut, "bogus") {
		t.Fatalf("a bad flag exits %d: %d %q", exitUsage, code, errOut)
	}
}
