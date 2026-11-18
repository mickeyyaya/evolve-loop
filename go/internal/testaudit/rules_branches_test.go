package testaudit

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func rulesOf(found []Finding) string {
	ids := make([]string, 0, len(found))
	for _, f := range found {
		ids = append(ids, f.Rule)
	}
	sort.Strings(ids)
	return strings.Join(ids, ",")
}

type branchCase struct {
	name  string
	files map[string]string
	cfg   Config
	want  string
}

const hdr = "package fx\n\nimport (\n\t\"errors\"\n\t\"flag\"\n\t\"os\"\n\t\"path/filepath\"\n\t\"reflect\"\n\t\"strings\"\n\t\"testing\"\n\t\"time\"\n\n\t\"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest\"\n\t\"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert\"\n)\n\nvar (\n\t_ = errors.New\n\t_ = flag.Bool\n\t_ = os.ReadFile\n\t_ = filepath.Join\n\t_ = reflect.DeepEqual\n\t_ = strings.Contains\n\t_ = time.Sleep\n\t_ = fakeclitest.Install\n\t_ = acsassert.FileExists\n)\n\n"

func one(body string) map[string]string {
	return map[string]string{"x_test.go": hdr + body, "lib.go": "package fx\n"}
}

var gitFake = Config{Fakes: []Fake{{Tool: "git", RealSide: "none", Reason: "r"}}}

var branchCases = []branchCase{
	{name: "non-test funcs and shapes are skipped", files: one(`
func helper(t *testing.T) { time.Sleep(1) }
func (r recv) TestMethod(t *testing.T) { time.Sleep(1) }
func TestTwoParams(t *testing.T, x int) { time.Sleep(1) }
func TestWrongType(m *testing.M) { time.Sleep(1) }
func TestNoName(*testing.T) { time.Sleep(1) }
func TestOtherPkg(t *other.T) { time.Sleep(1) }
func TestBare(t testing.T) { t.Fatal("x") }
type recv struct{}
`), want: ""},
	{name: "assertion by passing the handle and by closure handle", files: one(`
func TestSub(t *testing.T) {
	t.Run("a", func(sub *testing.T) { sub.Fatal("x") })
	t.Run("b", func(tb testing.TB, n int) { tb.Error("x") })
}
func TestHelperArg(t *testing.T) { acsassert.FileExists(t, "testdata/a.txt") }
`), want: ""},
	{name: "own setup with address and range and decl scopes", files: one(`
func TestPtr(t *testing.T) {
	r := &struct{ N int }{N: 1}
	var v int
	const c = "s"
	for i, j := range []int{1} { _, _ = i, j }
	if (r.N) != 1 || !true {
		t.Error("n")
	}
	_ = v
}
func TestEscapes(t *testing.T) {
	r := struct{ N int }{N: 1}
	fill(&r)
	if r.N != 1 {
		t.Error("n")
	}
	s := []int{1}
	use(s)
	if s[0] != 1 {
		t.Error("n")
	}
}
func TestCallInCond(t *testing.T) {
	r := struct{ N int }{N: 1}
	if len(r.X) != 1 || *p != 2 || other != 3 || r.X[0] != nil || noWay() {
		t.Error("n")
	}
}
func TestLitWithCall(t *testing.T) {
	r := struct{ N int }{N: f()}
	a, b := 1, 2
	if r.N != 1 {
		t.Error("n")
	}
	_, _ = a, b
}
type localT struct{}

func TestSkipBody(t *testing.T) {
	if err := f(); err != nil {
		t.Skip("x")
	}
	t.Fatal("y")
}
func TestOwnIdents(t *testing.T) {
	s := []int{1}
	p := &struct{ N int }{N: 1}
	if s != nil && *p != 3 && s[0] != 2 && f().N != 1 && other[0] != 1 && (s) != nil && (p).N != 3 {
		t.Error("n")
	}
	type inner struct{}
}
func TestNoFailInBody(t *testing.T) {
	r := struct{ N int }{N: 1}
	if r.N != 1 {
		t.Log("n")
	}
}
`), want: "asserts-own-setup,log-only,skip-hides-error"},
	{name: "incdec and index writes decide whether a container is production filled", files: one(`
func TestIncLiteral(t *testing.T) {
	m := map[string]int{"a": 1}
	m["a"]++
	if m["a"] != 2 {
		t.Error("n")
	}
}
func TestIncProduction(t *testing.T) {
	m := map[string]int{"a": 1}
	m[f()]++
	if m["a"] != 2 {
		t.Error("n")
	}
}
func TestIndexAssignProduction(t *testing.T) {
	m := map[string]int{}
	m[f()] = 1
	if m["a"] != 1 {
		t.Error("n")
	}
}
`), want: "asserts-own-setup"},
	{name: "values derived from production or handed to it are not own setup", files: one(`
func TestRangeDerivedKey(t *testing.T) {
	seen := map[row]bool{}
	for _, r := range f().Rows {
		key := row{Code: r.Code}
		if seen[key] {
			t.Error("dup")
		}
		seen[key] = true
	}
}
func TestHandedOut(t *testing.T) {
	rec := &recorder{}
	h := newHolder()
	h.sink = rec
	if rec.n == 0 {
		t.Error("n")
	}
}
func TestOwnKey(t *testing.T) {
	key := row{Code: 1}
	if key.Code != 2 {
		t.Error("n")
	}
}
`), want: "asserts-own-setup"},
	{name: "an alias of a literal is own setup", files: one(`
func TestAlias(t *testing.T) {
	v := 1
	x := v
	if x != 1 {
		t.Error("n")
	}
}
`), want: "asserts-own-setup"},
	{name: "holders aliases and control flow clear what production can write", files: one(`
func TestHolderRun(t *testing.T) {
	st := &row{}
	st.Code = 1
	h := holder{r: st}
	var g = h
	run(g)
	if st.Code != 2 {
		t.Error("n")
	}
}
func TestShadow(t *testing.T) {
	x := 1
	if x == 1 {
		x := x
		_ = x
	}
	var a, b = 1
	var c int
	if f() != a+b+c {
		t.Error("n")
	}
}
func TestClosureWrite(t *testing.T) {
	n := 0
	fn := func() { n++ }
	fn()
	if n != 1 {
		t.Error("n")
	}
}
func TestBranchWrite(t *testing.T) {
	n := 0
	m := 0
	if f() {
		n++
	} else {
		m--
	}
	if k := 0; f() {
		k++
	}
	if n != 1 || m != 1 {
		t.Error("n")
	}
}
func TestLoopWrites(t *testing.T) {
	n := 0
	m := 0
	for range f().Rows {
		a := 1
		_ = a
		n += 1
		m = 2
		*g()++
		*g() = 1
	}
	for i := 0; i < f(); i++ {
		n++
	}
	for {
		n++
		break
	}
	if n != 1 || m != 2 {
		t.Error("n")
	}
}
func TestLoopCounter(t *testing.T) {
	for i := 0; i < 3; i++ {
		if i == 1 {
			t.Error("n")
		}
	}
	for ; ; {
		break
	}
}
`), want: ""},
	{name: "skip on nil compare in else and not-nil", files: one(`
func TestSkipElse(t *testing.T) {
	err := f()
	if err == nil {
		t.Log("ok")
	} else {
		t.Skip("x")
	}
	if err == nil {
		t.Fatal("y")
	}
	if err != nil && other {
		t.Log("z")
	}
	if nilFirst := err; nil != nilFirst {
		t.Log("z")
	}
	if a == b {
		t.Fatal("c")
	}
}
`), want: "skip-hides-error"},
	{name: "field count shapes", files: one(`
const wantN = 2
const wantS = "s"
func TestFields(t *testing.T) {
	if 2 == reflect.TypeOf(x).NumField() {
		t.Fatal("a")
	}
	if reflect.TypeOf(x).NumField() > wantN {
		t.Fatal("a")
	}
	if reflect.TypeOf(x).NumField() > wantS {
		t.Fatal("a")
	}
	if reflect.TypeOf(x).NumField() > 1.5 {
		t.Fatal("a")
	}
	if reflect.TypeOf(x).Name() > 1 {
		t.Fatal("a")
	}
	if foo > 1 {
		t.Fatal("a")
	}
	if (reflect.TypeOf(x)).NumField() > 1 {
		t.Fatal("a")
	}
	if n() > 1 {
		t.Fatal("a")
	}
}
func TestFieldsPkgConst(t *testing.T) {
	if reflect.TypeOf(x).NumField() > wantN {
		t.Fatal("a")
	}
}
func TestFieldsLocalConst(t *testing.T) {
	const wantLocal = 3
	if reflect.TypeOf(x).NumField() != wantLocal {
		t.Fatal("a")
	}
}
func TestFieldsCall(t *testing.T) {
	if reflect.TypeOf(x).NumField() != f() {
		t.Fatal("a")
	}
}
`), want: "literal-field-count,literal-field-count,literal-field-count"},
	{name: "const resolution shapes", files: one(`
const dir = "."
const base = "lib"
func TestPins(t *testing.T) {
	os.ReadFile(base + ".go")
	os.ReadFile((filepath.Join(dir, base + ".go")))
	os.ReadFile(base * 2)
	os.ReadFile(filepath.Join(dir, name))
	os.ReadFile(load())
	os.ReadFile(` + "`x.go`" + `)
	os.ReadFile(1)
	os.ReadFile()
	os.ReadFile("sub/testdata/x.go")
	os.ReadFile(strconv.Itoa(1))
	acsassert.FileExists(t, "a.go")
	t.Fatal("x")
}
func TestPinsText(t *testing.T) {
	os.ReadFile("a.txt")
	const k = "k.go"
	os.ReadFile(k)
	t.Fatal("x")
}
func TestPinsTwo(t *testing.T) {
	var name = "k.go"
	os.ReadFile(name)
	t.Fatal("x")
}
`), want: "single-file-source-pin,single-file-source-pin"},
	{name: "fake install tail shapes", files: one(`
const shim = "tmux"
func TestInstalls(t *testing.T) {
	fakeclitest.Install(t, dir+"/git", "")
	fakeclitest.Install(t, (filepath.Join(d, shim)), "")
	fakeclitest.Install(t, filepath.Join(), "")
	fakeclitest.Install(t, name, "")
	fakeclitest.Install(t, 5, "")
	fakeclitest.Install(t, "only")
	t.Fatal("x")
}
func TestInstallsListed(t *testing.T) {
	fakeclitest.Install(t, "git", "")
	t.Fatal("x")
}
`), cfg: gitFake, want: "unverified-fake"},
	{name: "call order shapes", files: one(`
func TestOrder(t *testing.T) {
	if !reflect.DeepEqual(calls, want) {
		t.Fatal("a")
	}
	if !reflect.DeepEqual((f.CallOrder), want) {
		t.Fatal("a")
	}
	if !reflect.DeepEqual(f.Names(), want) {
		t.Fatal("a")
	}
	if !other.Equal(calls, want) {
		t.Fatal("a")
	}
}
`), want: "mock-call-order"},
	{name: "golden shapes", files: map[string]string{"x_test.go": hdr + `
var update = flag.Bool("update", false, "")
var other = 3
func TestGuarded(t *testing.T) {
	if *update {
		os.WriteFile("testdata/a", nil, 0o644)
	}
	if other == 3 {
		os.WriteFile(filepath.Join("out", "b"), nil, 0o644)
	}
	os.WriteFile()
	t.Fatal("x")
}
func TestUnguarded(t *testing.T) {
	os.WriteFile(filepath.Join("testdata", "c"), nil, 0o644)
	t.Fatal("x")
}
`, "y_test.go": hdr + `
var flagged bool

func init() { flag.BoolVar(&flagged, "g", false, "") }

func TestViaBoolVar(t *testing.T) {
	if flagged {
		os.WriteFile("testdata/a", nil, 0o644)
	}
	flag.BoolVar(other, "g", false, "")
	flag.BoolVar(&other.x, "g", false, "")
	t.Fatal("x")
}
`}, want: "golden-update-flag"},
	{name: "error text shapes", files: one(`
func TestText(t *testing.T) {
	if err.Error() == "x" || "y" != err.Error() || err.Error() == other || err.Error(1) == "z" {
		t.Fatal("a")
	}
	if strings.Contains(s, "a") || strings.Contains(err.Error(), "b") {
		t.Fatal("a")
	}
	if a + b == "c" {
		t.Fatal("a")
	}
}
func TestTextSel(t *testing.T) {
	if strings.HasPrefix(e.Wrap, "b") || x.y != "z" {
		t.Fatal("a")
	}
}
`), want: "error-text-match"},
	{name: "both branches with log only", files: one(`
func TestB(t *testing.T) {
	if a {
		t.Log("x")
	} else if b {
		_ = 1
	}
}
`), want: "both-branches-pass,log-only"},
	{name: "sleep", files: one(`
func TestSleep(t *testing.T) {
	time.Sleep(1)
	t.Fatal("x")
}
`), want: "sleep-sync"},
	{name: "ast pin variants", files: one(`
func TestAst(t *testing.T) {
	parser.ParseFile(fset, "lib.go", nil, 0)
	parser.ParseFile(fset)
	parser.ParseFile(fset, "testdata/a.go", nil, 0)
	t.Fatal("x")
}
`), want: "single-file-ast-pin"},
}

func TestRulesBranches(t *testing.T) {
	t.Parallel()
	for _, c := range branchCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := tree(t, c.files)
			found, err := Analyze(dir, c.cfg)
			if err != nil {
				t.Fatalf("Analyze: %v", err)
			}
			if got := rulesOf(found); got != c.want {
				t.Errorf("rules = %q, want %q (%v)", got, c.want, found)
			}
		})
	}
}
