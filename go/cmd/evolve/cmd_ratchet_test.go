package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/rawgitratchet"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
)

func TestRatchet_CheckPassesOnTheRealModuleAndRejectsOtherVerbs(t *testing.T) {
	if _, errOut, code := runDispatch("ratchet", "check", "--root", "../.."); code != 0 {
		t.Errorf("module must satisfy both ratchets: exit=%d stderr=%s", code, errOut)
	}
	if _, errOut, code := runDispatch("ratchet"); code != 2 || !strings.Contains(errOut, "usage") {
		t.Errorf("bare ratchet: exit=%d stderr=%q", code, errOut)
	}
}

func ratchetFixtureRoot(t *testing.T, funcLines int, offenders string) string {
	t.Helper()
	root := t.TempDir()
	var src strings.Builder
	src.WriteString("package fixture\n\nfunc Big() int {\n\ttotal := 0\n")
	for i := 0; i < funcLines; i++ {
		fmt.Fprintf(&src, "\ttotal += %d\n", i)
	}
	src.WriteString("\treturn total\n}\n")
	files := map[string]string{
		"internal/fixture/big.go":      src.String(),
		"internal/fixture/raw_test.go": "package fixture\n\nimport \"os/exec\"\n\nfunc raw() { exec.Command(\"git\", \"init\").Run() }\n",
		sizeratchet.OffendersRelPath:   offenders,
		rawgitratchet.BaselineRelPath:  "{}",
	}
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestRatchet_SelectorScansOnlyTheNamedRatchetUnderRoot(t *testing.T) {
	root := ratchetFixtureRoot(t, 80, `{"internal/fixture.Big": 60}`)
	cases := []struct {
		args     []string
		wantCode int
		wantErr  string
	}{
		{[]string{"ratchet", "check", "size", "--root", root}, 1, "internal/fixture.Big grew to"},
		{[]string{"ratchet", "check", "--root", root, "size"}, 1, "internal/fixture.Big grew to"},
		{[]string{"ratchet", "check", "rawgit", "--root", root}, 1, "internal/fixture/raw_test.go"},
		{[]string{"ratchet", "check", "--root", root}, 1, "raw_test.go"},
		{[]string{"ratchet", "check", "bogus", "--root", root}, 2, "unknown ratchet"},
		{[]string{"ratchet", "check", "--root", root, "junk"}, 2, "unknown ratchet"},
		{[]string{"ratchet", "check", "size", "--root", root, "junk"}, 2, "unexpected arguments"},
		{[]string{"ratchet", "check", "--bogus-flag"}, 2, "bogus-flag"},
	}
	for _, c := range cases {
		out, errOut, code := runDispatch(c.args...)
		if code != c.wantCode || !strings.Contains(errOut, c.wantErr) || strings.Contains(out, "clean") {
			t.Errorf("%v: exit=%d out=%q stderr=%q, want exit %d naming %q", c.args, code, out, errOut, c.wantCode, c.wantErr)
		}
	}
	if _, errOut, code := runDispatch("ratchet", "check", "size", "--root", ratchetFixtureRoot(t, 80, `{"internal/fixture.Big": 200}`)); code != 0 {
		t.Errorf("check size must not run the raw-git ratchet: exit=%d stderr=%q", code, errOut)
	}
}
