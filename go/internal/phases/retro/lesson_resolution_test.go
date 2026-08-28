package retro

// lesson_resolution_test.go — RED contract for where a failure lesson LIVES.
//
// retro graded itself FAIL unless hasFailureLesson found a file named
// `failure-lesson*.yaml` in the CYCLE WORKSPACE. The retro persona is instructed
// to write `.evolve/instincts/lessons/inst-LXXX-<slug>.yaml` — a different
// directory AND a different filename. The phase was graded FAIL for not producing
// an artifact its own persona is instructed never to produce.
//
// Measured on the runtime plane (where the loop actually writes; a git worktree's
// copy is empty because .evolve/instincts/lessons/* is gitignored): 600 lesson
// files, of which 135 are inst-L* and 464 are an older cycle-<N>-* convention.
// Every recent FAILING cycle produced inst-L* and no cycle-<N>-*: 1572→1, 1574→3,
// 1576→3, 1577→4. Exactly ONE file named failure-lesson* exists anywhere, and 220
// of 238 retro FAILs (92%) had a substantial retrospective and no workspace lesson.
//
// The persona's convention is the one that is load-bearing — those files are read
// back into future agents' instinctSummary — so the GATE moves to meet it, not the
// other way round. The legacy workspace shape is still accepted so cycle-1571-era
// artifacts keep passing.

import (
	"os"
	"path/filepath"
	"testing"
)

// lessonsFixture builds a project root with the canonical layout:
//
//	<root>/.evolve/instincts/lessons/   — where the persona writes
//	<root>/.evolve/runs/cycle-N/        — the phase workspace
func lessonsFixture(t *testing.T, cycle int, lessonNames ...string) (root, workspace string) {
	t.Helper()
	root = t.TempDir()
	workspace = filepath.Join(root, ".evolve", "runs", "cycle-"+itoa(cycle))
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	lessons := filepath.Join(root, ".evolve", "instincts", "lessons")
	if err := os.MkdirAll(lessons, 0o755); err != nil {
		t.Fatalf("mkdir lessons: %v", err)
	}
	for _, n := range lessonNames {
		if err := os.WriteFile(filepath.Join(lessons, n), []byte("type: failure-lesson\n"), 0o644); err != nil {
			t.Fatalf("write lesson %s: %v", n, err)
		}
	}
	return root, workspace
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func TestResolveFailureLesson(t *testing.T) {
	tests := []struct {
		name    string
		cycle   int
		lessons []string
		legacy  bool // also drop a legacy failure-lesson*.yaml in the workspace
		want    bool
	}{
		{
			// The shape that has been failing for months.
			name:    "a lesson named for THIS cycle, where the persona writes it",
			cycle:   1574,
			lessons: []string{"inst-L1574a-continuation-deferral-must-be-reconciled.yaml"},
			want:    true,
		},
		{
			name:    "multiple lessons for this cycle",
			cycle:   1574,
			lessons: []string{"inst-L1574a-one.yaml", "inst-L1574b-two.yaml", "inst-L1574c-three.yaml"},
			want:    true,
		},
		{
			// CYCLE-SCOPED, not "any lesson anywhere". 600 lessons exist; a gate
			// that accepts any of them would pass every cycle unconditionally,
			// turning a broken gate into a rubber stamp — the opposite failure.
			name:    "lessons exist but none name this cycle",
			cycle:   1577,
			lessons: []string{"inst-L1572a-other.yaml", "inst-L1574a-other.yaml"},
			want:    false,
		},
		{
			// DIGIT BOUNDARY (review L2). A plain prefix match lets cycle 157 be
			// satisfied by cycle 1574's lesson.
			name:    "a longer cycle number does not satisfy a shorter one's gate",
			cycle:   157,
			lessons: []string{"inst-L1574a-continuation-deferral.yaml"},
			want:    false,
		},
		{
			name:    "cycle 1 is not satisfied by every lesson starting with 1",
			cycle:   1,
			lessons: []string{"inst-L1572a-other.yaml"},
			want:    false,
		},
		{
			name:  "no lessons at all",
			cycle: 1577,
			want:  false,
		},
		{
			// cycle-1571 wrote failure-lesson-cycle1571.yaml into its workspace.
			// That corpus must keep passing.
			name:   "legacy workspace failure-lesson*.yaml still counts",
			cycle:  1571,
			legacy: true,
			want:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, ws := lessonsFixture(t, tc.cycle, tc.lessons...)
			if tc.legacy {
				if err := os.WriteFile(filepath.Join(ws, "failure-lesson-cycle"+itoa(tc.cycle)+".yaml"), []byte("x"), 0o644); err != nil {
					t.Fatalf("write legacy lesson: %v", err)
				}
			}

			if got := hasFailureLesson(root, ws, tc.cycle); got != tc.want {
				t.Errorf("hasFailureLesson = %v, want %v", got, tc.want)
			}
		})
	}
}

// The drift that caused this. The gate's search location and the persona's
// documented output path must be the same string — checked against the persona
// file itself, so a future edit to either side fails here rather than silently
// re-breaking the verdict for another 238 cycles.
func TestLessonPathIsSingleSourcedWithThePersona(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "agents", "evolve-retrospective.md"))
	if err != nil {
		t.Skipf("persona not readable from this worktree: %v", err)
	}
	if !containsPath(string(body), lessonsDirRel) {
		t.Errorf("the retro persona does not document %q as its lesson output path; gate and persona have drifted apart again", lessonsDirRel)
	}
}

func containsPath(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
