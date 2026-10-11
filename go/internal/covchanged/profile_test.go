package covchanged

import (
	"fmt"
	"reflect"
	"testing"
)

const testModule = "example.com/m"

func TestParseProfileLine_ReadsTheBlockTheStatementsAndTheCount(t *testing.T) {
	key, stmts, count, err := ParseProfileLine("example.com/m/p/a.go:12.34,15.2 3 7")

	if err != nil {
		t.Fatalf("ParseProfileLine: %v", err)
	}
	want := BlockKey{File: "example.com/m/p/a.go", StartLine: 12, StartCol: 34, EndLine: 15, EndCol: 2}
	if key != want || stmts != 3 || count != 7 {
		t.Errorf("got %+v %d %d, want %+v 3 7", key, stmts, count, want)
	}
}

func TestParseProfileLine_RefusesEveryMalformedShape(t *testing.T) {
	for _, line := range []string{
		"",
		"no colon here",
		"a.go:1.1,2.1 3",
		"a.go:1.1,2.1 3 4 5",
		"a.go:1.1-2.1 3 4",
		"a.go:1,2.1 3 4",
		"a.go:1.1,2 3 4",
		"a.go:x.1,2.1 3 4",
		"a.go:1.x,2.1 3 4",
		"a.go:1.1,x.1 3 4",
		"a.go:1.1,2.x 3 4",
		"a.go:1.1,2.1 x 4",
		"a.go:1.1,2.1 3 x",
		"a.go:0.1,2.1 3 4",
		"a.go:3.1,2.1 3 4",
		"a.go:1.1,2.1 -3 4",
		"a.go:1.1,2.1 3 -4",
		":1.1,2.1 3 4",
	} {
		if key, _, _, err := ParseProfileLine(line); err == nil {
			t.Errorf("ParseProfileLine(%q) = %+v, nil; want an error", line, key)
		}
	}
}

func TestParseProfile_MergesDuplicateBlocksByKeyWithAnOrOfTheHit(t *testing.T) {
	data := "mode: atomic\n" +
		"example.com/m/p/a.go:3.10,5.2 2 4\n" +
		"example.com/m/p/a.go:3.10,5.2 2 0\n" +
		"example.com/m/p/a.go:7.10,8.2 1 0\n" +
		"example.com/m/p/a.go:7.10,8.2 1 0\n" +
		"\n" +
		"example.com/other/x.go:1.1,2.1 1 1\n" +
		"example.com/mod/y.go:1.1,2.1 1 1\n"

	got, err := ParseProfile([]byte(data), testModule)

	if err != nil {
		t.Fatalf("ParseProfile: %v", err)
	}
	want := Profile{"p/a.go": []Block{
		{BlockKey: BlockKey{File: "p/a.go", StartLine: 3, StartCol: 10, EndLine: 5, EndCol: 2}, Statements: 2, Hit: true},
		{BlockKey: BlockKey{File: "p/a.go", StartLine: 7, StartCol: 10, EndLine: 8, EndCol: 2}, Statements: 1, Hit: false},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseProfile = %+v, want %+v", got, want)
	}
}

func TestParseProfile_RefusesAProfileWithoutAModeOrWithABadLine(t *testing.T) {
	for name, data := range map[string]string{
		"no mode line":    "example.com/m/a.go:1.1,2.1 1 1\n",
		"an empty file":   "",
		"a malformed row": "mode: set\nexample.com/m/a.go:1.1,2.1 1\n",
	} {
		if got, err := ParseProfile([]byte(data), testModule); err == nil {
			t.Errorf("%s: ParseProfile = %v, nil; want an error", name, got)
		}
	}
}

func FuzzParseProfileLine(f *testing.F) {
	f.Add("example.com/m/p/a.go:12.34,15.2 3 7")
	f.Add("a.go:1.1,1.1 0 0")
	f.Add("C:/x/a.go:3.1,4.1 1 1")
	f.Add("a.go:+1.01,2.1 3 4")
	f.Fuzz(func(t *testing.T, line string) {
		key, stmts, count, err := ParseProfileLine(line)
		if err != nil {
			return
		}
		if key.StartLine < 1 || key.EndLine < key.StartLine || stmts < 0 || count < 0 {
			t.Fatalf("ParseProfileLine(%q) accepted %+v %d %d", line, key, stmts, count)
		}
		again := fmt.Sprintf("%s:%d.%d,%d.%d %d %d", key.File, key.StartLine, key.StartCol, key.EndLine, key.EndCol, stmts, count)
		k2, s2, c2, err := ParseProfileLine(again)
		if err != nil || k2 != key || s2 != stmts || c2 != count {
			t.Fatalf("the round trip of %q through %q = %+v %d %d %v", line, again, k2, s2, c2, err)
		}
	})
}
