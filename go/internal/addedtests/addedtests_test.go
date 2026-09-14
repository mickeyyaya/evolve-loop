package addedtests_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/addedtests"
	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
)

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestGroups_TagGatedAddedPackagesRunUnderTheirOwnTags — the cycle-1679 shape:
// an added //go:build acs package must be grouped under acs (the default
// context never sees it), an untagged added package stays in the default
// group, a modified test is not a candidate, and a requires_tmux test is
// excluded by name rather than silently dropped.
func TestGroups_TagGatedAddedPackagesRunUnderTheirOwnTags(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go/acs/cycle1676/predicates_test.go", "//go:build acs\n\npackage cycle1676\n")
	write(t, root, "go/internal/widget/widget_test.go", "package widget\n")
	write(t, root, "go/internal/repl/repl_test.go", "//go:build requires_tmux\n\npackage repl\n")
	write(t, root, "go/internal/old/old_test.go", "package old\n")
	files := []changedpkgs.ChangedFile{
		{Path: "go/acs/cycle1676/predicates_test.go", Added: true},
		{Path: "go/internal/widget/widget_test.go", Added: true},
		{Path: "go/internal/repl/repl_test.go", Added: true},
		{Path: "go/internal/old/old_test.go", Added: false},
		{Path: "docs/x.md", Added: true},
	}
	groups, excluded, err := addedtests.Groups(root, files)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 {
		t.Fatalf("groups = %+v", groups)
	}
	if len(groups[0].Tags) != 0 || strings.Join(groups[0].Packages, " ") != "./internal/widget" {
		t.Fatalf("default group = %+v", groups[0])
	}
	if strings.Join(groups[1].Tags, ",") != "acs" || strings.Join(groups[1].Packages, " ") != "./acs/cycle1676" {
		t.Fatalf("acs group = %+v", groups[1])
	}
	if len(excluded) != 1 || excluded[0] != "go/internal/repl/repl_test.go" {
		t.Fatalf("excluded = %v", excluded)
	}
}

// TestBuildTags names the tag resolver: default context, a single tag, and
// the requires_tmux exclusion.
func TestBuildTags(t *testing.T) {
	root := t.TempDir()
	write(t, root, "plain_test.go", "package x\n")
	write(t, root, "acs_test.go", "//go:build acs\n\npackage x\n")
	write(t, root, "tmux_test.go", "//go:build requires_tmux && acs\n\npackage x\n")
	if tags, ok, err := addedtests.BuildTags(filepath.Join(root, "plain_test.go")); err != nil || !ok || len(tags) != 0 {
		t.Fatalf("plain: %v %v %v", tags, ok, err)
	}
	if tags, ok, err := addedtests.BuildTags(filepath.Join(root, "acs_test.go")); err != nil || !ok || strings.Join(tags, ",") != "acs" {
		t.Fatalf("acs: %v %v %v", tags, ok, err)
	}
	if _, ok, err := addedtests.BuildTags(filepath.Join(root, "tmux_test.go")); err != nil || ok {
		t.Fatalf("requires_tmux must be unrunnable: %v %v", ok, err)
	}
}

// TestGroup_NamesTheTagSetAndItsPackages names the Group type for apicover:
// a group is one tag set and the packages that build under it.
func TestGroup_NamesTheTagSetAndItsPackages(t *testing.T) {
	g := addedtests.Group{Tags: []string{"acs"}, Packages: []string{"./acs/cycle1676"}}
	if strings.Join(g.Tags, ",") != "acs" || len(g.Packages) != 1 {
		t.Fatalf("Group = %+v", g)
	}
}
