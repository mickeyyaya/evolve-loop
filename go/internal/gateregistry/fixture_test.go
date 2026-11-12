package gateregistry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
)

const dupHelper = "package %s\n\nimport \"os\"\n\nfunc helper(dir string) error {\n\tif err := os.Mkdir(dir, 0o755); err != nil {\n\t\treturn err\n\t}\n\treturn os.Chmod(dir, 0o700)\n}\n"

const rawGitTest = "package p\n\nimport \"os/exec\"\n\nfunc raw() { exec.Command(\"git\", \"init\").Run() }\n"

func oversizeFunction() string {
	return "package p\n\nfunc F() {\n" + strings.Repeat("\t_ = 1\n", sizeratchet.MaxLines+10) + "}\n"
}

func lay(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, text := range files {
		target := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

type seedFixture struct {
	name string
	tree func() map[string]string
	git  bool
}

func (f seedFixture) root(t *testing.T) string {
	t.Helper()
	if f.git {
		return gittest.Fixture(t).Dir
	}
	return t.TempDir()
}

func seedFixtures() []seedFixture {
	return []seedFixture{
		{
			name: "testdup-ratchet",
			tree: func() map[string]string {
				return map[string]string{
					"go/a/a_test.go": strings.Replace(dupHelper, "%s", "a", 1),
					"go/b/b_test.go": strings.Replace(dupHelper, "%s", "b", 1),
				}
			},
			git: true,
		},
		{
			name: "size-ratchet",
			tree: func() map[string]string { return map[string]string{"go/p/p.go": oversizeFunction()} },
		},
		{
			name: "raw-git-ratchet",
			tree: func() map[string]string { return map[string]string{"go/p/raw_test.go": rawGitTest} },
		},
	}
}

var baselineFiles = map[string]map[string]string{
	"testdup-ratchet": {"go/.test-dup-baseline": "count=0\ndate=2026-11-09\n"},
	"size-ratchet":    {"go/internal/sizeratchet/offenders.json": "{}"},
	"raw-git-ratchet": {"go/internal/rawgitratchet/baseline.json": "{}"},
}

var greenFiles = map[string]map[string]string{
	"testdup-ratchet": {"go/.test-dup-baseline": "count=1\ndate=2026-11-09\n"},
	"size-ratchet":    {"go/internal/sizeratchet/offenders.json": `{"p.F": 100}`},
	"raw-git-ratchet": {"go/internal/rawgitratchet/baseline.json": `{"p/raw_test.go": 1}`},
}
