//go:build acs

package cycle2358

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const facadeFile = "go/internal/inboxmover/closesmarker.go"

func goDir(t *testing.T) string { return filepath.Join(acsassert.RepoRoot(t), "go") }

func TestC2358_001_ClosesInboxNoneIsAnExplicitEmptySet(t *testing.T) {
	for name, body := range map[string]string{
		"alone":      "Closes-Inbox: none\n",
		"upper case": "CLOSES-INBOX: NONE\n",
		"backticked": "- Closes-Inbox: `none`\n",
	} {
		if got := inboxmover.ClosesInboxIDs([]byte(body)); len(got) != 0 {
			t.Errorf("%s: ClosesInboxIDs = %#v, want none", name, got)
		}
	}
	if got, want := inboxmover.ClosesInboxIDs([]byte("Closes-Inbox: none, real-item\n")), []string{"real-item"}; !reflect.DeepEqual(got, want) {
		t.Errorf("none in a list: ClosesInboxIDs = %#v, want %#v", got, want)
	}
	if got, want := inboxmover.ClosesInboxIDs([]byte("Closes-Inbox: none-such-item\n")), []string{"none-such-item"}; !reflect.DeepEqual(got, want) {
		t.Errorf("an id that starts with none must stay an id: got %#v, want %#v", got, want)
	}
}

func TestC2358_002_RefsInboxIDsReadsOnlyTheRefsMarker(t *testing.T) {
	body := []byte("Closes-Inbox: closed-item\nRefs-Inbox: partial-item, other-item\n  Refs-Inbox: indented-item\n```\nRefs-Inbox: fenced-item\n```\n")
	if got, want := inboxmover.RefsInboxIDs(body), []string{"partial-item", "other-item"}; !reflect.DeepEqual(got, want) {
		t.Errorf("RefsInboxIDs = %#v, want %#v", got, want)
	}
	if got, want := inboxmover.ClosesInboxIDs(body), []string{"closed-item"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ClosesInboxIDs = %#v, want %#v: a Refs-Inbox line must close nothing", got, want)
	}
}

func TestC2358_003_RefsInboxNoneAndAbsentMarkerGiveNoIDs(t *testing.T) {
	for name, body := range map[string]string{
		"none":      "Refs-Inbox: none\n",
		"mixed":     "Refs-Inbox: None\n",
		"no marker": "# Build Report\nCloses-Inbox: only-a-close\n",
		"empty":     "",
	} {
		if got := inboxmover.RefsInboxIDs([]byte(body)); len(got) != 0 {
			t.Errorf("%s: RefsInboxIDs = %#v, want none", name, got)
		}
	}
	if got, want := inboxmover.RefsInboxIDs([]byte("Refs-Inbox: none, real-item\n")), []string{"real-item"}; !reflect.DeepEqual(got, want) {
		t.Errorf("none in a list: RefsInboxIDs = %#v, want %#v", got, want)
	}
}

func TestC2358_004_TheNamedRedTestsExistAndPass(t *testing.T) {
	names := []string{
		"TestClosesInboxIDs_NoneIsAnExplicitEmptySet",
		"TestRefsInboxIDs_ReadsItsOwnMarkerOnly",
		"TestRefsInboxIDs_NoneIsNotAnID",
	}
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", goDir(t), "test", "-count=1", "-run", "^("+strings.Join(names, "|")+")$", "-v", "./internal/inboxmover")
	if err != nil || code != 0 {
		t.Errorf("the named tests are not green (exit=%d err=%v)\n%s\n%s",
			code, err, acsassert.OutputTail(stdout, 40), acsassert.OutputTail(stderr, 40))
	}
	for _, name := range names {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("%s did not run and pass", name)
		}
	}
}

func TestC2358_005_EachChangedFunctionHasFullStatementCoverage(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "cover.out")
	_, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", goDir(t), "test", "-count=1", "-run", "^(TestClosesInboxIDs|TestRefsInboxIDs)", "-coverprofile="+profile, "./internal/inboxmover")
	if err != nil || code != 0 {
		t.Fatalf("coverage run failed (exit=%d err=%v)\n%s", code, err, acsassert.OutputTail(stderr, 40))
	}
	funcs, _, code, err := acsassert.SubprocessOutput("go", "-C", goDir(t), "tool", "cover", "-func="+profile)
	if err != nil || code != 0 {
		t.Fatalf("cover -func failed (exit=%d err=%v)", code, err)
	}
	for _, fn := range []string{"ClosesInboxIDs", "RefsInboxIDs"} {
		pct, err := acsassert.FuncCoverage(funcs, fn)
		if err != nil {
			t.Errorf("%s: %v", fn, err)
			continue
		}
		if pct != 100 {
			t.Errorf("%s coverage = %.1f%%, want 100%%", fn, pct)
		}
	}
}

func TestC2358_006_TheFacadeFilesCarryNoComments(t *testing.T) {
	root := acsassert.RepoRoot(t)
	fset := token.NewFileSet()
	for _, rel := range []string{facadeFile, "go/internal/inboxmover/closesmarker_test.go"} {
		f, err := parser.ParseFile(fset, filepath.Join(root, rel), nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		if len(f.Comments) != 0 {
			t.Errorf("%s carries %d comment group(s); code has no comments", rel, len(f.Comments))
		}
	}
}

func TestC2358_007_ThePackageNoteAndTheSpecStateTheChange(t *testing.T) {
	root := acsassert.RepoRoot(t)
	note := filepath.Join(root, "docs/architecture/packages/internal-inboxmover.md")
	spec := filepath.Join(root, "docs/architecture/inbox-injection-protocol.md")
	// acs-predicate: config-check
	if !acsassert.AllOf(t,
		func(tb acsassert.TB) bool { return acsassert.FileContains(tb, note, "RefsInboxIDs") },
		func(tb acsassert.TB) bool { return acsassert.FileContains(tb, note, "Closes-Inbox: none") },
		func(tb acsassert.TB) bool { return acsassert.FileContains(tb, spec, "RefsInboxIDs") },
		func(tb acsassert.TB) bool { return acsassert.FileContains(tb, spec, "Closes-Inbox: none") },
	) {
		t.Errorf("the package note and the spec must both name RefsInboxIDs and Closes-Inbox: none")
	}
}
