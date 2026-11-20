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
