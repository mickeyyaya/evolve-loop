package retro

import (
	"os"
	"path/filepath"
	"testing"
)

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
		legacy  bool
		want    bool
	}{
		{
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
			name:    "lessons exist but none name this cycle",
			cycle:   1577,
			lessons: []string{"inst-L1572a-other.yaml", "inst-L1574a-other.yaml"},
			want:    false,
		},
		{
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
