package covchanged

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const percentsSource = `package m

type T struct{}

func Empty() {}

func Half(x int) int {
	if x > 0 {
		return 1
	}
	return 2
}

func (t *T) Method(x int) int {
	for i := 0; i < x; i++ {
		x--
	}
	return x
}

func Third(a, b, c bool) int {
	n := 0
	if a {
		n++
	}
	if b {
		n++
	}
	if c {
		n++
	}
	return n
}

func Untouched() int {
	return 1
}

func Declared()
`

const percentsProfile = `mode: set
example.com/m/m.go:5.15,5.15 0 1
example.com/m/m.go:8.2,8.11 1 1
example.com/m/m.go:9.3,10.1 1 1
example.com/m/m.go:11.2,11.10 1 0
example.com/m/m.go:15.2,15.25 1 1
example.com/m/m.go:16.3,17.1 1 1
example.com/m/m.go:18.2,18.10 1 1
example.com/m/m.go:22.2,23.7 2 1
example.com/m/m.go:24.3,25.1 1 1
example.com/m/m.go:26.2,26.7 1 1
example.com/m/m.go:27.3,28.1 1 0
example.com/m/m.go:29.2,29.7 1 1
example.com/m/m.go:30.3,31.1 1 0
example.com/m/m.go:32.2,32.10 1 1
example.com/m/m.go:36.2,37.1 1 0
`

const percentsToolResult = `example.com/m/m.go:5:	Empty		0.0%
example.com/m/m.go:7:	Half		66.7%
example.com/m/m.go:14:	Method		100.0%
example.com/m/m.go:21:	Third		75.0%
example.com/m/m.go:35:	Untouched	0.0%
`

func readPercentsFixture(rel string) ([]byte, error) {
	if rel != "m.go" {
		return nil, fs.ErrNotExist
	}
	return []byte(percentsSource), nil
}

func TestFuncPercents_EqualTheRecordedGoToolCoverResult(t *testing.T) {
	t.Parallel()
	profile, err := ParseProfile([]byte(percentsProfile), "example.com/m")
	if err != nil {
		t.Fatal(err)
	}

	funcs, err := FuncPercents(profile, readPercentsFixture)

	if err != nil {
		t.Fatal(err)
	}
	recorded := strings.Split(strings.TrimSpace(percentsToolResult), "\n")
	if len(funcs) != len(recorded) {
		t.Fatalf("funcs = %d, recorded = %d", len(funcs), len(recorded))
	}
	for i, line := range recorded {
		fields := strings.Fields(line)
		if !strings.HasSuffix(fields[0], "m.go:"+strconv.Itoa(funcs[i].Line)+":") || percentText(funcs[i]) != fields[len(fields)-1] || !strings.HasSuffix(funcs[i].Name, fields[1]) {
			t.Errorf("func %d = %+v (%s), recorded %q", i, funcs[i], percentText(funcs[i]), line)
		}
	}
}

func percentText(f Func) string {
	return strconv.FormatFloat(f.CoverPercent(), 'f', 1, 64) + "%"
}

func TestFuncPercents_ARoundedPercentHasOneDecimal(t *testing.T) {
	t.Parallel()
	for covered, want := range map[int]float64{1: 33.3, 2: 66.7} {
		if got := (Func{Statements: 3, Covered: covered, Profiled: true}).CoverPercent(); got != want {
			t.Errorf("CoverPercent(%d of 3) = %v, want %v", covered, got, want)
		}
	}
}

func TestFuncPercents_AMissingSourceFileIsAnError(t *testing.T) {
	t.Parallel()
	profile := Profile{"gone.go": {{BlockKey: BlockKey{StartLine: 1}}}}

	funcs, err := FuncPercents(profile, readPercentsFixture)

	if !errors.Is(err, fs.ErrNotExist) || funcs != nil || !strings.Contains(err.Error(), "gone.go") {
		t.Errorf("FuncPercents = %v, %v; want an error naming gone.go", funcs, err)
	}
}

func TestFuncPercents_ASourceFileThatDoesNotParseIsAnError(t *testing.T) {
	t.Parallel()
	profile := Profile{"bad.go": nil}
	read := func(string) ([]byte, error) { return []byte("package"), nil }

	_, err := FuncPercents(profile, read)

	if err == nil || !strings.Contains(err.Error(), "parse bad.go") {
		t.Errorf("FuncPercents = %v, want a parse error naming bad.go", err)
	}
}

func TestFuncPercents_ReadsTheRealFilesWhenGivenOsReadFile(t *testing.T) {
	t.Parallel()
	profile := Profile{"covchanged.go": nil}

	funcs, err := FuncPercents(profile, os.ReadFile)

	if err != nil || len(funcs) == 0 || funcs[0].Profiled {
		t.Errorf("FuncPercents = %v, %v; want unprofiled functions", funcs, err)
	}
}

func TestFuncPercents_ATieRoundsAsGoToolCoverDoes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		covered, statements int
		want                float64
	}{{1, 16, 6.2}, {3, 16, 18.8}, {1, 8, 12.5}} {
		if got := (Func{Statements: tc.statements, Covered: tc.covered}).CoverPercent(); got != tc.want {
			t.Errorf("CoverPercent(%d of %d) = %v, want %v", tc.covered, tc.statements, got, tc.want)
		}
		if got, want := percentText(Func{Statements: tc.statements, Covered: tc.covered}), fmt.Sprintf("%.1f%%", 100*float64(tc.covered)/float64(tc.statements)); got != want {
			t.Errorf("percentText = %s, want %s", got, want)
		}
	}
}

func TestFuncPercents_FilesComeInSortedOrder(t *testing.T) {
	t.Parallel()
	profile := Profile{"b.go": nil, "a.go": nil, "c.go": nil, "d.go": nil, "e.go": nil, "f.go": nil, "g.go": nil, "h.go": nil}
	read := func(rel string) ([]byte, error) { return []byte("package m\n\nfunc F() {}\n"), nil }

	funcs, err := FuncPercents(profile, read)

	var files []string
	for _, f := range funcs {
		files = append(files, f.File)
	}
	if err != nil || !slices.Equal(files, []string{"a.go", "b.go", "c.go", "d.go", "e.go", "f.go", "g.go", "h.go"}) {
		t.Errorf("files = %v, %v; want a.go to h.go in order", files, err)
	}
}
