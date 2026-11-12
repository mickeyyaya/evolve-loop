package rawgitratchet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_ReturnsTheSitesTheBaselineAndTheFallbackNote(t *testing.T) {
	root := t.TempDir()
	for rel, text := range map[string]string{"p/raw_test.go": "package p\n\nimport \"os/exec\"\n\nfunc raw() { exec.Command(\"git\", \"init\").Run() }\n", BaselineRelPath: `{"p/raw_test.go": 1}`} {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sites, baseline, note, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if sites["p/raw_test.go"] != 1 || baseline["p/raw_test.go"] != 1 || !strings.Contains(note, "git cannot list tracked files") {
		t.Errorf("Load = %v, %v, %q, want one site, its baseline row and the fallback note", sites, baseline, note)
	}
	if sites, _, _, err := Load(filepath.Join(root, "absent")); err == nil || sites != nil {
		t.Errorf("Load over a missing root = %v, %v, want only an error", sites, err)
	}
}

func TestScan_BindsFilesSitesAndBaselineFromOneRoot(t *testing.T) {
	root := t.TempDir()
	write := func(rel, text string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("p/raw_test.go", "package p\n\nimport \"os/exec\"\n\nfunc raw() { exec.Command(\"git\", \"init\").Run() }\n")
	write(BaselineRelPath, "{}")
	if _, err := Scan(root); err == nil || !strings.Contains(err.Error(), "p/raw_test.go") {
		t.Fatalf("Scan over an unlisted raw init = %v, want an error naming p/raw_test.go", err)
	}
	write(BaselineRelPath, `{"p/raw_test.go": 1}`)
	if _, err := Scan(root); err != nil {
		t.Errorf("Scan with the site baselined = %v, want nil", err)
	}
	if err := os.Remove(filepath.Join(root, BaselineRelPath)); err != nil {
		t.Fatal(err)
	}
	if _, err := Scan(root); err == nil {
		t.Error("Scan without a baseline must fail loudly")
	}
}

func TestScanFile_CommitMessageNamedInitIsNotARepoInit(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"commit message", `r.Git("commit", "-q", "-m", "init")`, 0},
		{"raw init", `exec.Command("git", "init")`, 1},
		{"both", `exec.Command("git", "init"); r.Git("commit", "-m", "init")`, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "x_test.go")
			src := "package p\n\nfunc f() { " + tc.body + " }\n"
			if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			_, got, err := scanFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("init sites = %d, want %d", got, tc.want)
			}
		})
	}
}
