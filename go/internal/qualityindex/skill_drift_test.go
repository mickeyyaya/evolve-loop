package qualityindex

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

var tableKey = regexp.MustCompile("^\\| `([a-z-]+)` \\|")

var backticked = regexp.MustCompile("`([a-z-]+)`")

func projection(t *testing.T, name string) (rows, never []string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "skills", "quality-index", name))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if m := tableKey.FindStringSubmatch(line); m != nil {
			rows = append(rows, m[1])
		}
		if strings.HasPrefix(line, "Never N/A: ") {
			for _, m := range backticked.FindAllStringSubmatch(line, -1) {
				never = append(never, m[1])
			}
		}
	}
	return rows, never
}

func TestTheSkillProjectsTheIndexItsCodeDefines(t *testing.T) {
	var wantNever []string
	for _, k := range Keys() {
		if NeverNA(k) {
			wantNever = append(wantNever, k)
		}
	}
	for _, name := range []string{"SKILL.md", "COMPACT.md"} {
		rows, never := projection(t, name)
		if !reflect.DeepEqual(rows, Keys()) {
			t.Errorf("%s dimension table lists %v, want %v in index order", name, rows, Keys())
		}
		if !reflect.DeepEqual(never, wantNever) {
			t.Errorf("%s \"Never N/A:\" line lists %v, want %v", name, never, wantNever)
		}
	}
}

func designDimensions(t *testing.T) (rows, neverColumn, neverLine []string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "architecture", "review-loop-and-quality-index.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, section, found := strings.Cut(string(raw), "#### 2.1 Dimensions\n")
	if !found {
		t.Fatal("the design doc has no #### 2.1 Dimensions section")
	}
	section, _, _ = strings.Cut(section, "\n#### ")
	for _, line := range strings.Split(section, "\n") {
		if m := tableKey.FindStringSubmatch(line); m != nil {
			rows = append(rows, m[1])
			if cells := strings.Split(line, "|"); len(cells) > 5 && strings.TrimSpace(cells[5]) == "never" {
				neverColumn = append(neverColumn, m[1])
			}
		}
		if strings.HasPrefix(line, "- **Never N/A:** ") {
			for _, m := range backticked.FindAllStringSubmatch(line, -1) {
				neverLine = append(neverLine, m[1])
			}
		}
	}
	return rows, neverColumn, neverLine
}

func TestTheDesignDocProjectsTheIndexItsCodeDefines(t *testing.T) {
	var wantNever []string
	for _, k := range Keys() {
		if NeverNA(k) {
			wantNever = append(wantNever, k)
		}
	}

	rows, neverColumn, neverLine := designDimensions(t)

	if !reflect.DeepEqual(rows, Keys()) {
		t.Errorf("design §2.1 dimension table lists %v, want %v in index order", rows, Keys())
	}
	if !reflect.DeepEqual(neverColumn, wantNever) {
		t.Errorf("design §2.1 rows whose N/A column is never: %v, want %v", neverColumn, wantNever)
	}
	if !reflect.DeepEqual(neverLine, wantNever) {
		t.Errorf("design §2.1 Never N/A bullet lists %v, want %v", neverLine, wantNever)
	}
}
