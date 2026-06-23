package publishmirror

import "testing"

func TestScan_CleanTree_NoViolations(t *testing.T) {
	files := map[string]string{
		"README.md": "# evolveloop\n\nRun `go install`.\n",
		"docs/x.md": "Path is `~/ai/claude/evolve-loop/go`; email user@example.com.\n",
		"go/go.mod": "module github.com/mickeyyaya/evolveloop/go\n",
		"notes.txt": "A host prompt: user@host evolve-loop %\n",
	}
	if v := Scan(files, nil); len(v) != 0 {
		t.Fatalf("clean tree should have 0 violations, got %d: %+v", len(v), v)
	}
}

func TestScan_MacHomePath_Flagged(t *testing.T) {
	files := map[string]string{"a.md": "see /Users/danleemh/ai/claude/evolve-loop for detail"}
	v := Scan(files, nil)
	if len(v) != 1 {
		t.Fatalf("want 1 violation, got %d: %+v", len(v), v)
	}
	if v[0].Rule != "macos-home-path" {
		t.Errorf("rule = %q, want macos-home-path", v[0].Rule)
	}
	if v[0].File != "a.md" || v[0].Line != 1 {
		t.Errorf("loc = %s:%d, want a.md:1", v[0].File, v[0].Line)
	}
	if v[0].Match != "/Users/danleemh" {
		t.Errorf("match = %q, want /Users/danleemh", v[0].Match)
	}
}

func TestScan_PlaceholderHomePath_NotFlagged(t *testing.T) {
	// The literal placeholder "/Users/<user>" (with angle brackets) is documentation,
	// not a real home path — it must NOT trip the structural rule.
	files := map[string]string{"runbook.md": "scrub /Users/<user> paths to ~ before publish"}
	if v := Scan(files, nil); len(v) != 0 {
		t.Fatalf("placeholder /Users/<user> should not be flagged, got %+v", v)
	}
}

func TestScan_Denylist_CaseInsensitiveSubstring(t *testing.T) {
	files := map[string]string{
		"c.md": "author DanLeemh wrote this",
		"d.md": "ping me at Me@Gmail.com please",
	}
	v := Scan(files, []string{"danleemh", "me@gmail.com"})
	if len(v) != 2 {
		t.Fatalf("want 2 denylist violations, got %d: %+v", len(v), v)
	}
	for _, got := range v {
		if got.Rule != "denylist" {
			t.Errorf("rule = %q, want denylist", got.Rule)
		}
	}
}

func TestScan_Secrets_Flagged(t *testing.T) {
	files := map[string]string{
		"e.md": "key sk-ABCDEFGHIJKLMNOP0123 leaked",
		"f.md": "aws AKIAIOSFODNN7EXAMPLE here",
	}
	v := Scan(files, nil)
	if len(v) != 2 {
		t.Fatalf("want 2 secret violations, got %d: %+v", len(v), v)
	}
}

func TestScan_ScrubbedForms_NotFlagged(t *testing.T) {
	// The canonical post-scrub forms must be treated as clean.
	files := map[string]string{
		"g.md": "~/ai/claude/evolve-loop and ~/.claude/plans/x.md",
		"h.md": "Author: Mickey Yaya <user@example.com>",
		"i.md": "user@host evolve-loop %",
	}
	if v := Scan(files, nil); len(v) != 0 {
		t.Fatalf("scrubbed forms should be clean, got %+v", v)
	}
}

func TestScan_Deterministic_SortedByFileThenLine(t *testing.T) {
	files := map[string]string{
		"b.md": "line1\n/Users/danleemh/x\n/Users/danleemh/y",
		"a.md": "/Users/danleemh/z",
	}
	v := Scan(files, nil)
	if len(v) != 3 {
		t.Fatalf("want 3, got %d: %+v", len(v), v)
	}
	// a.md:1, b.md:2, b.md:3
	if v[0].File != "a.md" || v[1].File != "b.md" || v[1].Line != 2 || v[2].Line != 3 {
		t.Errorf("not sorted by file then line: %+v", v)
	}
}
