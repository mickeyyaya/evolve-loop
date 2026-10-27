package main

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
)

func wallStream(pkg string, wall float64) string {
	return fmt.Sprintf(`{"Action":"pass","Package":%q,"Elapsed":%v}`+"\n", pkg, wall)
}

func writeRatchetFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ratchet.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func runWith(stream string, args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := run(args, strings.NewReader(stream), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestRatchet_FailsOnlyAboveTwiceClassCeilingCappedByTimeout(t *testing.T) {
	path := writeRatchetFile(t, `{"packages":{"p":{"class":"small","p90":10}}}`)
	cases := []struct {
		wall    float64
		timeout string
		want    int
	}{
		{119, "20m", 0},
		{121, "20m", 1},
		{100, "90s", 1},
	}
	for _, c := range cases {
		code, _, errOut := runWith(wallStream("p", c.wall), "-ratchet", path, "-test-timeout", c.timeout)
		if code != c.want {
			t.Errorf("wall %v timeout %s: exit %d, want %d\n%s", c.wall, c.timeout, code, c.want, errOut)
		}
	}
}

func TestRatchet_UnreadableFileExitsTwoNamingIt(t *testing.T) {
	code, _, errOut := runWith(wallStream("p", 1), "-ratchet", filepath.Join(t.TempDir(), "absent.json"))
	if code != 2 || !strings.Contains(errOut, "absent.json") {
		t.Errorf("exit %d stderr %q, want 2 naming absent.json", code, errOut)
	}
}

func TestRatchet_WarnsOnlyPastBothMargins(t *testing.T) {
	path := writeRatchetFile(t, `{"packages":{"slow":{"class":"large","p90":100},"near":{"class":"large","p90":100}}}`)
	code, out, _ := runWith(wallStream("slow", 151)+wallStream("near", 140), "-ratchet", path)
	if code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	section := out[strings.Index(out, warnHeading):]
	if !strings.Contains(section, "| slow | 100.00 | 151.00 |") || strings.Contains(section, "near") {
		t.Errorf("warning section wrong:\n%s", section)
	}
}

func TestTighten_LowersNeverRaises(t *testing.T) {
	path := writeRatchetFile(t, `{"packages":{"fast":{"class":"medium","p90":100},"slow":{"class":"small","p90":30}}}`)
	if code, _, errOut := runWith(wallStream("fast", 40)+wallStream("slow", 80), "tighten", "-ratchet", path); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	rf, err := loadRatchet(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := rf.Packages["fast"]; got != (record{Class: "medium", P90: 40}) {
		t.Errorf("fast = %+v, want p90 lowered to 40", got)
	}
	if got := rf.Packages["slow"]; got.P90 != 30 {
		t.Errorf("slow = %+v, want p90 kept at 30", got)
	}
}

func TestTighten_NeedsReadableRatchet(t *testing.T) {
	if code, _, _ := runWith("", "tighten"); code != 2 {
		t.Errorf("no -ratchet: exit %d, want 2", code)
	}
	if code, _, _ := runWith("", "tighten", "-ratchet", filepath.Join(t.TempDir(), "absent.json")); code != 2 {
		t.Errorf("missing file: exit %d, want 2", code)
	}
}

func TestLoadRatchet_RejectsMalformedRecords(t *testing.T) {
	cases := map[string]string{
		"not json":      `{`,
		"unknown class": `{"packages":{"p":{"class":"huge","p90":1}}}`,
		"zero p90":      `{"packages":{"p":{"class":"small","p90":0}}}`,
	}
	for name, body := range cases {
		if _, err := loadRatchet(writeRatchetFile(t, body)); err == nil {
			t.Errorf("%s: loadRatchet accepted %s", name, body)
		}
	}
}

func TestRatchet_UnknownPackageIsCappedByTimeoutOnly(t *testing.T) {
	rf := &ratchetFile{Packages: map[string]record{}}
	if got := rf.limit("example.com/mod/other", 90); got != 90 {
		t.Errorf("limit = %v, want the timeout 90", got)
	}
}

func TestRatchet_NoRegressionPrintsNone(t *testing.T) {
	path := writeRatchetFile(t, `{"packages":{"p":{"class":"small","p90":10}}}`)
	_, out, _ := runWith(wallStream("p", 11), "-ratchet", path)
	if !strings.Contains(out, warnHeading+"\n\n_(none)_") {
		t.Errorf("want the none marker under the warning heading:\n%s", out)
	}
}

func TestRatchet_BadTimeoutExitsTwo(t *testing.T) {
	path := writeRatchetFile(t, `{"packages":{"p":{"class":"small","p90":10}}}`)
	code, _, errOut := runWith(wallStream("p", 1), "-ratchet", path, "-test-timeout", "soon")
	if code != 2 || !strings.Contains(errOut, "-test-timeout") {
		t.Errorf("exit %d stderr %q, want 2 naming -test-timeout", code, errOut)
	}
}

func TestRatchet_ScanFailureExitsOne(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run(nil, iotest.ErrReader(errors.New("boom")), &out, &errOut); code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	if code := runTighten([]string{"-ratchet", writeRatchetFile(t, `{"packages":{}}`)}, iotest.ErrReader(errors.New("boom")), &errOut); code != 1 {
		t.Errorf("tighten exit %d, want 1", code)
	}
}

func TestTighten_BadFlagExitsTwo(t *testing.T) {
	if code, _, _ := runWith("", "tighten", "-nope"); code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
}

func TestRatchetSave_ReportsEveryWriteFailure(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]struct {
		rf   *ratchetFile
		path string
	}{
		"marshal": {&ratchetFile{Packages: map[string]record{"p": {Class: "small", P90: math.NaN()}}}, filepath.Join(dir, "a.json")},
		"write":   {&ratchetFile{}, filepath.Join(dir, "missing", "a.json")},
		"rename":  {&ratchetFile{}, dir},
	}
	for name, c := range cases {
		if err := c.rf.save(c.path); err == nil {
			t.Errorf("%s: save returned nil", name)
		}
	}
}

func TestTighten_SaveFailureExitsTwo(t *testing.T) {
	path := writeRatchetFile(t, `{"packages":{"p":{"class":"small","p90":50}}}`)
	if err := os.Mkdir(path+".tmp", 0o755); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := runWith(wallStream("p", 10), "tighten", "-ratchet", path); code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
}

func TestResolveBudget_RejectsABadWallBudget(t *testing.T) {
	if _, _, err := resolveBudget("bogus", true, false, 0); err == nil {
		t.Error("want an error for an unparsable package wall budget")
	}
}
