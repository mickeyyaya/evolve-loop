package opscmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestDoctorBoot_Usage(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := runDoctorBoot(nil, &out, &errb); rc != 10 {
		t.Errorf("rc=%d, want 10 (missing driver arg); stderr=%s", rc, errb.String())
	}
}

func TestDoctorBoot_UnknownDriver(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := runDoctorBoot([]string{"no-such-driver"}, &out, &errb); rc != 10 {
		t.Errorf("rc=%d, want 10 (unknown driver); stderr=%s", rc, errb.String())
	}
}

func TestDoctorBoot_JSONShape(t *testing.T) {
	var out, errb bytes.Buffer
	runDoctorBoot([]string{"no-such-driver", "--json"}, &out, &errb)
	for _, key := range []string{`"driver"`, `"exit_code"`, `"booted"`} {
		if !strings.Contains(out.String(), key) {
			t.Errorf("--json output missing %s; got %s", key, out.String())
		}
	}
}
