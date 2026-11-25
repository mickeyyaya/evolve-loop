package covchanged

import "testing"

func TestDeclaredFuncs_NamesEachFunctionWithABodyByItsMeasuredName(t *testing.T) {
	t.Parallel()
	src := "package p\n\ntype T struct{}\n\nfunc F() {}\n\nfunc (t *T) M() {}\n\nfunc (T) V() {}\n\nfunc Ext()\n\nvar x = 1\n"

	got, err := DeclaredFuncs("p/a.go", []byte(src))

	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"F", "T.M", "T.V"} {
		if !got[name] {
			t.Errorf("DeclaredFuncs = %v, want %s", got, name)
		}
	}
	if len(got) != 3 {
		t.Errorf("DeclaredFuncs = %v, want only the three functions with a body", got)
	}
}

func TestDeclaredFuncs_ASourceThatDoesNotParseIsAnError(t *testing.T) {
	t.Parallel()

	if got, err := DeclaredFuncs("p/a.go", []byte("package p\nfunc (")); err == nil || got != nil {
		t.Errorf("DeclaredFuncs = %v, %v, want an error", got, err)
	}
}

func TestChangedFuncNames_NamesOnlyFunctionsWhoseSpanHoldsAChangedLine(t *testing.T) {
	t.Parallel()
	src := "package p\n\ntype T struct{}\n\nfunc F() {\n\treturn\n}\n\nfunc (t *T) M() {\n\treturn\n}\n\nfunc Ext()\n\nvar x = 1\n"

	got, err := ChangedFuncNames("p/a.go", []byte(src), []int{10, 13, 15})

	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "T.M" {
		t.Errorf("ChangedFuncNames = %v, want only T.M (line 15 is in no function, Ext has no body)", got)
	}
}

func TestChangedFuncNames_ASourceThatDoesNotParseIsAnError(t *testing.T) {
	t.Parallel()

	if got, err := ChangedFuncNames("p/a.go", []byte("package p\nfunc ("), []int{2}); err == nil || got != nil {
		t.Errorf("ChangedFuncNames = %v, %v, want an error", got, err)
	}
}
