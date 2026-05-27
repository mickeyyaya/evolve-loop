package evalqualitycheck

import (
	"os"
	"path/filepath"
	"testing"
)

// writeEvalDir builds a temp dir of eval files from name→body and returns it.
func writeEvalDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func fence(cmds ...string) string {
	body := "```bash\n"
	for _, c := range cmds {
		body += c + "\n"
	}
	return body + "```\n"
}

// TestCheckDiversity_Levels is the data-driven matrix over suite shapes.
func TestCheckDiversity_Levels(t *testing.T) {
	posEval := fence(`grep -q "Feature initialized" src/f.txt`)
	negEval := fence(`! grep -q "removed" src/f.txt`)

	cases := []struct {
		name      string
		files     map[string]string
		wantLevel DiversityLevel
		wantEvals int
	}{
		{
			name:      "empty dir → PASS (nothing to assess)",
			files:     map[string]string{},
			wantLevel: DiversityPass,
			wantEvals: 0,
		},
		{
			name: "three all-positive evals → HALT (cohesive, zero negatives)",
			files: map[string]string{
				"a.md": posEval, "b.md": posEval, "c.md": posEval,
			},
			wantLevel: DiversityHalt,
			wantEvals: 3,
		},
		{
			name: "mixed suite with a negative case → PASS",
			files: map[string]string{
				"a.md": posEval, "b.md": negEval, "c.md": posEval,
			},
			wantLevel: DiversityPass,
			wantEvals: 3,
		},
		{
			name:      "single positive eval → WARN (>50% positive-only, but <3 so not HALT)",
			files:     map[string]string{"a.md": posEval},
			wantLevel: DiversityWarn,
			wantEvals: 1,
		},
		{
			name:      "two positive evals → WARN (<3 so not HALT)",
			files:     map[string]string{"a.md": posEval, "b.md": posEval},
			wantLevel: DiversityWarn,
			wantEvals: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeEvalDir(t, tc.files)
			res, err := CheckDiversity(DiversityOptions{EvalDir: dir})
			if err != nil {
				t.Fatal(err)
			}
			if res.Level != tc.wantLevel {
				t.Errorf("Level=%d, want %d (reasons=%v)", res.Level, tc.wantLevel, res.Reasons)
			}
			if res.EvalCount != tc.wantEvals {
				t.Errorf("EvalCount=%d, want %d", res.EvalCount, tc.wantEvals)
			}
		})
	}
}

// TestCheckDiversity_ArchiveScaleZeroNegativeIsWarnNotHalt — a large accumulated
// archive (>maxCohesiveSuiteSize) with zero negatives downgrades HALT→WARN so
// the legacy .evolve/evals/ corpus is never a hard block.
func TestCheckDiversity_ArchiveScaleZeroNegativeIsWarnNotHalt(t *testing.T) {
	pos := fence(`grep -q "x" src/f.txt`)
	files := map[string]string{}
	for i := 0; i < maxCohesiveSuiteSize+5; i++ {
		files[string(rune('a'+i))+".md"] = pos
	}
	dir := writeEvalDir(t, files)
	res, err := CheckDiversity(DiversityOptions{EvalDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Level != DiversityWarn {
		t.Errorf("Level=%d, want WARN(1) for archive-scale zero-negative suite", res.Level)
	}
}

// TestCheckDiversity_NegativeCasePrecision — the word "failure" inside a grep
// target must NOT count as a negative case (it's a doc-grep, not a rejection
// test); shell negation constructs MUST.
func TestCheckDiversity_NegativeCasePrecision(t *testing.T) {
	cases := []struct {
		cmd         string
		wantNegated bool
	}{
		{`grep -q "failure pattern analysis" docs/x.md`, false}, // English word, not a negative test
		{`grep -q "should not fail" docs/x.md`, false},          // still a doc-grep target
		{`grep -q "result != expected" out.txt`, false},         // != inside a grep string is NOT a negative test
		{`grep -q "!= mismatch" out.txt`, false},                // != inside a grep string is NOT a negative test
		{`! grep -q "removed" src/f.txt`, true},                 // shell negation
		{`test "$x" -ne 0`, true},                               // numeric inequality
		{`assert_fail "rejects bad input" cmd`, true},           // helper naming
		{`run cmd; [ "$?" != 0 ]`, true},                        // != inside a test bracket IS a negative assertion
		{`go test ./...`, false},                                // plain positive
	}
	for _, tc := range cases {
		t.Run(tc.cmd, func(t *testing.T) {
			got := negativeCaseRE.MatchString(tc.cmd)
			if got != tc.wantNegated {
				t.Errorf("negativeCaseRE(%q)=%v, want %v", tc.cmd, got, tc.wantNegated)
			}
		})
	}
}

// TestCheckDiversity_EdgeCaseDetection — boundary/OOD keywords flag edge cases.
func TestCheckDiversity_EdgeCaseDetection(t *testing.T) {
	dir := writeEvalDir(t, map[string]string{
		"edge.md": fence(`run cmd --input "" # empty boundary`, `grep -q "invalid" out.txt`),
	})
	res, err := CheckDiversity(DiversityOptions{EvalDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.EdgeCaseCount != 1 {
		t.Errorf("EdgeCaseCount=%d, want 1", res.EdgeCaseCount)
	}
}

// TestCheckDiversity_SkipsMetaAndCommandlessFiles — underscore-prefixed files
// and files with no bash commands do not count toward the suite.
func TestCheckDiversity_SkipsMetaAndCommandlessFiles(t *testing.T) {
	dir := writeEvalDir(t, map[string]string{
		"_canary.md":    fence(`echo "canary"`),
		"prose-only.md": "# Eval\nNo commands here, just text.\n",
		"real.md":       fence(`! grep -q "x" f.txt`),
		"noteval.txt":   fence(`grep -q "x" f.txt`), // non-.md ignored
	})
	res, err := CheckDiversity(DiversityOptions{EvalDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.EvalCount != 1 {
		t.Errorf("EvalCount=%d, want 1 (only real.md counts)", res.EvalCount)
	}
}

// TestCheckDiversity_SlugFilter — only files whose name contains the slug count.
func TestCheckDiversity_SlugFilter(t *testing.T) {
	dir := writeEvalDir(t, map[string]string{
		"feat-login.md":  fence(`grep -q "x" f.txt`),
		"feat-logout.md": fence(`grep -q "y" f.txt`),
		"other.md":       fence(`grep -q "z" f.txt`),
	})
	res, err := CheckDiversity(DiversityOptions{EvalDir: dir, Slug: "login"})
	if err != nil {
		t.Fatal(err)
	}
	if res.EvalCount != 1 {
		t.Errorf("EvalCount=%d, want 1 (slug=login)", res.EvalCount)
	}
}

func TestCheckDiversity_EmptyDirRequired(t *testing.T) {
	if _, err := CheckDiversity(DiversityOptions{}); err == nil {
		t.Error("want error for empty EvalDir")
	}
}

func TestCheckDiversity_DirNotFound(t *testing.T) {
	if _, err := CheckDiversity(DiversityOptions{EvalDir: "/no/such/dir/xyz"}); err == nil {
		t.Error("want error for missing dir")
	}
}

// TestCheckDiversity_PerFileFingerprint — verifies the per-file Files slice.
func TestCheckDiversity_PerFileFingerprint(t *testing.T) {
	dir := writeEvalDir(t, map[string]string{
		"neg.md": fence(`! grep -q "x" f.txt`),
	})
	res, err := CheckDiversity(DiversityOptions{EvalDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Files) != 1 || !res.Files[0].HasNegative {
		t.Errorf("Files=%+v, want 1 file with HasNegative=true", res.Files)
	}
}
