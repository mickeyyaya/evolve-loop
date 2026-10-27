package kerneltest

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

type fatalRecorder struct {
	testing.TB
	fatal string
}

func (r *fatalRecorder) Helper() {}

func (r *fatalRecorder) Fatalf(format string, args ...any) { r.fatal = fmt.Sprintf(format, args...) }

func TestLoadFrom_AnUnreadableRegistryFailsTheTest(t *testing.T) {
	t.Parallel()
	rec := &fatalRecorder{TB: t}
	if f := loadFrom(rec, filepath.Join(t.TempDir(), "absent.json")); f != nil || !strings.Contains(rec.fatal, "kerneltest: load registry") {
		t.Errorf("loadFrom = %+v, fatal = %q, want no fixture and a load-registry failure", f, rec.fatal)
	}
}
