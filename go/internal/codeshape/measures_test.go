package codeshape

import "testing"

func measured(t *testing.T, body string) Func {
	t.Helper()
	funcs, err := ParseFuncs("m.go", []byte("package m\n\n"+body))
	if err != nil || len(funcs) != 1 {
		t.Fatalf("parse: %v %v", funcs, err)
	}
	return funcs[0]
}

func TestNesting_AnElseIfChainStaysAtItsOwnLevel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, body string
		want       int
	}{
		{"flat else-if chain", "func f(x int) int {\n\tif x == 1 {\n\t\treturn 1\n\t} else if x == 2 {\n\t\treturn 2\n\t} else if x == 3 {\n\t\treturn 3\n\t} else {\n\t\treturn 4\n\t}\n}\n", 1},
		{"if inside if", "func f(x int) {\n\tif x > 0 {\n\t\tif x > 1 {\n\t\t}\n\t}\n}\n", 2},
		{"if inside else block", "func f(x int) {\n\tif x > 0 {\n\t} else {\n\t\tif x < -1 {\n\t\t}\n\t}\n}\n", 2},
		{"if inside an else-if body", "func f(x int) {\n\tif x > 0 {\n\t} else if x < 0 {\n\t\tfor {\n\t\t}\n\t}\n}\n", 2},
		{"closure in a condition", "func f(x int) {\n\tif func() bool { if x > 0 { return true }; return false }() {\n\t}\n}\n", 3},
		{"if with init", "func f() {\n\tif x := 1; x > 0 {\n\t}\n}\n", 1},
	}
	for _, c := range cases {
		if got := measured(t, c.body).Nest; got != c.want {
			t.Errorf("%s: nesting = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestCyclomatic_ADefaultBranchIsNotADecision(t *testing.T) {
	t.Parallel()
	cases := map[string]int{
		"func f(x int) {\n\tswitch x {\n\tcase 1:\n\tcase 2:\n\tdefault:\n\t}\n}\n":       3,
		"func f(c chan int) {\n\tselect {\n\tcase <-c:\n\tdefault:\n\t}\n}\n":             2,
		"func f(x int) bool {\n\treturn x > 0 && x < 9 || x == 20\n}\n":                   3,
		"func f(xs []int) {\n\tfor range xs {\n\t}\n\tfor i := 0; i < 2; i++ {\n\t}\n}\n": 3,
	}
	for body, want := range cases {
		if got := measured(t, body).Cyclo; got != want {
			t.Errorf("cyclomatic = %d, want %d for\n%s", got, want, body)
		}
	}
}
