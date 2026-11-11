package gatehermetic

import (
	"slices"
	"testing"
)

func TestConversions_ARunFuncBoxedIntoAnInterfaceIsExec(t *testing.T) {
	t.Parallel()
	files := gatesFile(`"context"
	"io"
	`+sx, `func takeAny(v any) { _ = v }

func Boxed() { takeAny(sysexec.DefaultRunner) }

func BoxedLocal(r sysexec.RunFunc) any { return r }

func BoxedLiteral() any {
	return func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return 0, nil
	}
}

func get() (sysexec.RunFunc, error) { return nil, nil }

func pair() (any, error) { return get() }

func BoxedTuple() { _, _ = pair() }

func Named() { takeAny(takeAny) }

func Plain() { takeAny(1) }

func Direct(r sysexec.RunFunc) sysexec.RunFunc { return r }
`)

	by := byRoot(refuseFixture(t, files, rootsOf("Boxed", "BoxedLocal", "BoxedLiteral", "BoxedTuple", "Named", "Plain", "Direct")...))

	for _, name := range []string{"Boxed", "BoxedLocal", "BoxedLiteral", "BoxedTuple"} {
		if got := classes(by[name]); !slices.Contains(got, "exec") {
			t.Errorf("%s = %v, want exec", name, got)
		}
	}
	for _, name := range []string{"Named", "Plain", "Direct"} {
		if len(by[name]) != 0 {
			t.Errorf("%s = %+v, want none", name, by[name])
		}
	}
}

func TestConversions_AMapKeyConvertsThroughAnIndexExpression(t *testing.T) {
	t.Parallel()
	files := gatesFile(`"fmt"
	"os/exec"`, `type T struct{}

func (T) M() { _ = exec.Command("git", "log") }

func Assign() {
	m := map[any]int{}
	m[T{}] = 1
	fmt.Println(m)
}

func IncDec(m map[any]int) { m[T{}]++ }

func Read(m map[any]int) int { return m[T{}] }

func Slice(s []int) int { return s[0] }
`)

	by := byRoot(refuseFixture(t, files, rootsOf("Assign", "IncDec", "Read", "Slice")...))

	for _, name := range []string{"Assign", "IncDec", "Read"} {
		if len(by[name]) != 1 || !slices.Contains(edges(by[name][0]), "live-type") {
			t.Errorf("%s = %+v, want one git refusal through a live-type edge", name, by[name])
		}
	}
	if len(by["Slice"]) != 0 {
		t.Errorf("Slice = %+v", by["Slice"])
	}
}

func TestConversions_ATupleFlowsEachElementIntoItsTarget(t *testing.T) {
	t.Parallel()
	files := gatesFile(`"os/exec"`, `type T struct{}

func (T) M() { _ = exec.Command("git", "log") }

func pair() (T, int) { return T{}, 1 }

func take2(a any, b int) { _, _ = a, b }

func Take() { take2(pair()) }

func Ret() (any, int) { return pair() }

func Assign() {
	var x any
	x, _ = pair()
	_ = x
}

func Decl() {
	var a, b any = pair()
	_, _ = a, b
}

func Short() {
	x, y := pair()
	_, _ = x, y
}
`)

	files["internal/tupvar/tupvar.go"] = "package tupvar\n\nimport \"" + modulePath + "/internal/gates\"\n\nvar PA, PB any = gates.Pair()\n\nfunc Pkg() any { return PA }\n"
	files["internal/gates/gates.go"] += "\nfunc Pair() (T, int) { return pair() }\n"
	roots := append(rootsOf("Take", "Ret", "Assign", "Decl", "Short"), gateRoot("tupvar", "Pkg"))

	by := byRoot(refuseFixture(t, files, roots...))

	for _, name := range []string{"Take", "Ret", "Assign", "Decl", "Pkg"} {
		if len(by[name]) != 1 || !slices.Contains(edges(by[name][0]), "live-type") {
			t.Errorf("%s = %+v, want one git refusal through a live-type edge", name, by[name])
		}
	}
	if len(by["Short"]) != 0 {
		t.Errorf("Short = %+v", by["Short"])
	}
}

func TestConversions_AGenericInstanceMakesItsTypeArgumentsLive(t *testing.T) {
	t.Parallel()
	files := gatesFile(`"os/exec"`, `type T struct{}

func (T) M() { _ = exec.Command("git", "log") }

func callM[X interface{ M() }](x X) { x.M() }

func Generic() { callM(T{}) }

func toAny[X any](x X) any { return x }

func Converted() { _ = toAny(T{}) }

func Explicit() { _ = toAny[T](T{}) }

type Box[X any] struct{ v X }

func Typed() { _ = Box[T]{} }

func Plain[X any](x X) {}

func NoMethods() { Plain(1) }
`)

	by := byRoot(refuseFixture(t, files, rootsOf("Generic", "Converted", "Explicit", "Typed", "NoMethods")...))

	for _, name := range []string{"Generic", "Converted", "Explicit", "Typed"} {
		if len(by[name]) != 1 || !slices.Contains(edges(by[name][0]), "live-type") {
			t.Errorf("%s = %+v, want one git refusal through a live-type edge", name, by[name])
		}
	}
	if len(by["NoMethods"]) != 0 {
		t.Errorf("NoMethods = %+v", by["NoMethods"])
	}
}

func TestConversions_AnExecEntryUsedAsAValueIsExecUnlessItFlowsIntoRunFunc(t *testing.T) {
	t.Parallel()
	files := gatesFile(`"context"
	"go/build"
	"io"
	"os/exec"
	`+sx, `type D struct{ run `+runnerSig+` }

func (d D) Field(ctx context.Context) {
	_, _ = d.run(ctx, "git", "", nil, nil, nil, nil, nil)
}

func ExecValue() {
	f := exec.Command
	_ = f("git", "log").Run()
}

func RunnerLocal(ctx context.Context) {
	run := sysexec.DefaultRunner
	_, _ = run(ctx, "git", "", nil, nil, nil, nil, nil)
}

func FieldCall(ctx context.Context, d D) { d.Field(ctx) }

func FieldLiteral(ctx context.Context) {
	d := D{run: sysexec.DefaultRunner}
	_, _ = d.run(ctx, "go", "", nil, nil, nil, nil, nil)
}

func Unlisted() {
	f := sysexec.Unlisted
	f()
}

func MethodValue() {
	f := build.Default.Import
	_, _ = f("x", "", 0)
}

func CallResult(ctx context.Context) {
	_, _ = get()(ctx, "git", "", nil, nil, nil, nil, nil)
}

func get() sysexec.RunFunc { return sysexec.DefaultRunner }

func MapElement(ctx context.Context, m map[string]sysexec.RunFunc) {
	_, _ = m["a"](ctx, "git", "", nil, nil, nil, nil, nil)
}

func Typed() {
	var r sysexec.RunFunc = sysexec.DefaultRunner
	_ = r
}

func Converted() { _ = sysexec.RunFunc(sysexec.DefaultRunner) }

func take(r sysexec.RunFunc) { _ = r }

func Argument() { take(sysexec.DefaultRunner) }

func Returned() sysexec.RunFunc { return sysexec.DefaultRunner }

func Assigned() {
	var r sysexec.RunFunc
	r = sysexec.DefaultRunner
	_ = r
}

func Literal() sysexec.RunFunc {
	lit := func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return 0, nil
	}
	return lit
}

func Immediate(ctx context.Context) {
	_, _ = func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return 0, nil
	}(ctx, "git", "", nil, nil, nil, nil, nil)
}
`)
	roots := rootsOf("ExecValue", "RunnerLocal", "FieldCall", "FieldLiteral", "Unlisted", "MethodValue", "CallResult", "MapElement", "Typed", "Converted", "Argument", "Returned", "Assigned", "Literal", "Immediate")

	by := byRoot(refuseFixture(t, files, roots...))

	want := map[string][]string{
		"ExecValue": {"exec"}, "RunnerLocal": {"exec", "git"}, "FieldCall": {"git"}, "FieldLiteral": {"exec", "exec"},
		"Unlisted": {"exec"}, "MethodValue": {"exec"}, "CallResult": {"git"}, "MapElement": {"git"},
	}
	for name, exp := range want {
		if got := classes(by[name]); !slices.Equal(got, exp) {
			t.Errorf("%s = %v, want %v", name, got, exp)
		}
	}
	for _, name := range []string{"Typed", "Converted", "Argument", "Returned", "Assigned", "Literal", "Immediate"} {
		if len(by[name]) != 0 {
			t.Errorf("%s = %+v, want none", name, by[name])
		}
	}
}

func TestConversions_ATypeConvertedInAPackageVariableInitializerIsLive(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"internal/hold/hold.go":   "package hold\n\nimport \"os/exec\"\n\ntype T struct{}\n\nfunc (T) M() { _ = exec.Command(\"git\", \"log\") }\n\nvar Any any = T{}\n",
		"internal/plain/plain.go": "package plain\n\nimport \"os/exec\"\n\ntype T struct{}\n\nfunc (T) M() { _ = exec.Command(\"git\", \"log\") }\n\nvar Plain = T{}\n",
		"internal/gates/gates.go": "package gates\n\nimport (\n\t\"" + modulePath + "/internal/hold\"\n\t\"" + modulePath + "/internal/plain\"\n)\n\nfunc ReadsAny() any { return hold.Any }\n\nfunc ReadsPlain() plain.T { return plain.Plain }\n",
	}

	by := byRoot(refuseFixture(t, files, rootsOf("ReadsAny", "ReadsPlain")...))

	if len(by["ReadsAny"]) != 1 || len(by["ReadsPlain"]) != 0 {
		t.Errorf("ReadsAny = %+v ReadsPlain = %+v", by["ReadsAny"], by["ReadsPlain"])
	}
}

func TestConversions_ATypeThatABodyOnlyMentionsDoesNotReachItsMethodsAndAConversionDoes(t *testing.T) {
	t.Parallel()
	files := gatesFile(`"os/exec"`, `type T struct{}

func (T) M() { _ = exec.Command("git", "log") }

type Holder struct{ t T }

func Mention(h Holder, t T, p *T) {
	var local T
	var copied T = t
	_, _, _, _ = h, local, t, copied
	_ = p
}

func ToAny() { var i any = T{}; _ = i }

func Explicit() { _ = any(T{}) }

func Argument() { take(T{}) }

func take(v any) { _ = v }

func Return() any { return T{} }

func Literal() []any { return []any{T{}} }

func Keyed() Holder2 { return Holder2{v: T{}} }

type Holder2 struct{ v any }

func Positional() Holder2 { return Holder2{T{}} }

func MapElem() map[string]any { return map[string]any{"k": T{}} }

func MapKey() map[any]int { return map[any]int{T{}: 1} }

func Send(ch chan any) { ch <- T{} }

func Assign() { var i any; i = T{}; _ = i }

func Closure() func() any { return func() any { return T{} } }

func Variadic() { all(T{}, T{}) }

func all(v ...any) { _ = v }

func Spread(vs []any) { all(vs...) }

func Pointer() any { return &T{} }

func Multi() { take2(pair()) }

func pair() (T, int) { return T{}, 1 }

func take2(a any, b int) { _, _ = a, b }

func Elided() []*Holder2 { return []*Holder2{{v: T{}}} }

func ArrayLit() [1]any { return [1]any{T{}} }

func Builtins(s []any) int { s = append(s, T{}); panic(len(s)) }

func Declared() { var i, j any = T{}, 1; _, _ = i, j }
`)
	roots := rootsOf("Mention", "ToAny", "Explicit", "Argument", "Return", "Literal", "Keyed", "Positional", "MapElem", "MapKey", "Send", "Assign", "Closure", "Variadic", "Spread", "Pointer", "Declared", "Multi", "Elided", "ArrayLit", "Builtins")

	by := byRoot(refuseFixture(t, files, roots...))

	for _, name := range []string{"Mention", "Spread"} {
		if len(by[name]) != 0 {
			t.Errorf("%s = %+v, want none", name, by[name])
		}
	}
	for _, name := range []string{"ToAny", "Explicit", "Argument", "Return", "Literal", "Keyed", "Positional", "MapElem", "MapKey", "Send", "Assign", "Variadic", "Pointer", "Declared", "Multi", "Elided", "ArrayLit", "Builtins"} {
		if len(by[name]) != 1 || !slices.Contains(edges(by[name][0]), "live-type") {
			t.Errorf("%s = %+v, want one git refusal through a live-type edge", name, by[name])
		}
	}
	if len(by["Closure"]) != 1 {
		t.Errorf("Closure = %+v", by["Closure"])
	}
}

func TestConversions_AConvertedTypeIsLiveOnce(t *testing.T) {
	t.Parallel()
	files := gatesFile(`"os/exec"`, `type T struct{ self *T }

func (T) M() { _ = exec.Command("git", "log") }

func Run() {
	var a any = T{}
	var b any = &T{}
	_, _ = a, b
}
`)

	got := refuseFixture(t, files, gateRoot("gates", "Run"))

	if len(got) != 1 {
		t.Errorf("refusals = %+v", got)
	}
}
