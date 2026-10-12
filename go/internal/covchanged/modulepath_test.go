package covchanged

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModulePath_ReadsTheModuleLineAndRefusesAMissingOrEmptyGoMod(t *testing.T) {
	for name, tc := range map[string]struct {
		goMod string
		want  string
		fail  string
	}{
		"a plain path":  {"module example.com/m\n\ngo 1.23\n", "example.com/m", ""},
		"a quoted path": {"go 1.23\nmodule \"example.com/q\"\n", "example.com/q", ""},
		"no module":     {"go 1.23\n", "", "declares no module"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(tc.goMod), 0o644); err != nil {
				t.Fatal(err)
			}

			got, err := ModulePath(dir)

			if tc.fail != "" {
				if err == nil || !strings.Contains(err.Error(), tc.fail) {
					t.Errorf("err = %v; want %q", err, tc.fail)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Errorf("ModulePath = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	if _, err := ModulePath(t.TempDir()); err == nil || !strings.Contains(err.Error(), "read go.mod") {
		t.Errorf("err = %v; want a refusal that names go.mod", err)
	}
}
