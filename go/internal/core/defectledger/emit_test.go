package defectledger

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestEmit_AppendsOpenRowsDedupedByText(t *testing.T) {
	ws := t.TempDir()
	l, got := observed(scopeOf(), resolveNever)
	r := Rejection{Defects: []string{"alpha defect", "beta defect", strings.Repeat("x", 2100), "alpha defect"}, Prescriptions: []string{"add a regression test"}}
	mustEmit(t, l, Request{Cycle: 1255, Workspace: ws}, r)
	golden := string(goldenBytes(t, "emit_ledger.golden.json"))
	if body := readFile(t, filepath.Join(ws, LedgerFile)); body != golden {
		t.Fatalf("G1 bytes:\n%s", body)
	}
	mustEmit(t, l, Request{Cycle: 1256, Workspace: ws}, r)
	if body := readFile(t, filepath.Join(ws, LedgerFile)); body != golden {
		t.Fatalf("a retry is byte-idempotent:\n%s", body)
	}
	doc, _, _ := Read(ws)
	for _, e := range doc.Entries {
		if e.Status != StatusOpen || e.ID != ID(e.Text) {
			t.Fatalf("every emitted row is OPEN with a content id: %+v", e)
		}
	}
	if len(*got) != 0 {
		t.Fatalf("a clean emit is silent: %v", codesOf(*got))
	}
}

func TestEmit_PrescriptionPrefixAndOriginCycleOnlyOnFirstWrite(t *testing.T) {
	ws := t.TempDir()
	l, _ := observed(scopeOf(), resolveNever)
	mustEmit(t, l, Request{Cycle: 1255, Workspace: ws}, Rejection{Prescriptions: []string{"p"}})
	doc, _, _ := Read(ws)
	if doc.OriginCycle != 1255 || len(doc.Entries) != 1 || doc.Entries[0].Text != "PRESCRIPTION: p" {
		t.Fatalf("prefixed, origin stamped: %+v", doc)
	}
	mustEmit(t, l, Request{Cycle: 1290, Workspace: ws}, Rejection{Defects: []string{"later"}})
	doc, _, _ = Read(ws)
	if doc.OriginCycle != 1255 || len(doc.Entries) != 2 {
		t.Fatalf("an existing ledger keeps its origin: %+v", doc)
	}
}

func TestEmit_OverflowRowIsRecordedOnceAndSignalled(t *testing.T) {
	ws := t.TempDir()
	l, got := observed(scopeOf(), resolveNever)
	defects := make([]string, 65)
	for i := range defects {
		defects[i] = "defect " + strconv.Itoa(i)
	}
	req := Request{Cycle: 1285, Workspace: ws}
	mustEmit(t, l, req, Rejection{Defects: defects})
	golden := string(goldenBytes(t, "overflow_ledger.golden.json"))
	if body := readFile(t, filepath.Join(ws, LedgerFile)); body != golden {
		t.Fatalf("G2 bytes:\n%s", body)
	}
	e := only(t, *got, CodeOverflow)
	if e.Origin != "Ledger.Emit" || e.Cycle != 1285 || e.Fields["step"] != "emit" || e.Fields["overflow"] != "1" || e.Fields["cap"] != "64" || e.Fields["blocked"] != "false" || e.Fields["path"] != filepath.Join(ws, LedgerFile) {
		t.Fatalf("overflow event: %+v", e)
	}
	mustEmit(t, l, req, Rejection{Defects: defects})
	if body := readFile(t, filepath.Join(ws, LedgerFile)); body != golden {
		t.Fatalf("the retry adds nothing:\n%s", body)
	}
	if n := len(*got); n != 2 {
		t.Fatalf("the retry overflowed again and said so: %d events", n)
	}
}

func TestEmit_NothingToAdd_WritesNoFile(t *testing.T) {
	l, got := observed(scopeOf(), resolveNever)
	mustEmit(t, l, Request{Cycle: 1, Workspace: ""}, Rejection{Defects: []string{"x"}})
	ws := t.TempDir()
	mustEmit(t, l, Request{Cycle: 1, Workspace: ws}, Rejection{})
	if _, err := os.Stat(filepath.Join(ws, LedgerFile)); err == nil {
		t.Fatal("an empty rejection mints nothing")
	}
	mustEmit(t, l, Request{Cycle: 1, Workspace: ws}, Rejection{Defects: []string{"x"}})
	before := readFile(t, filepath.Join(ws, LedgerFile))
	if err := os.Chmod(ws, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ws, 0o755) })
	if v := l.Emit(Request{Cycle: 2, Workspace: ws}, Rejection{Defects: []string{"x", "x"}}); len(v.Diagnostics) != 0 {
		t.Fatalf("all-duplicate: no write is attempted, so a read-only workspace is fine: %+v", v)
	}
	if after := readFile(t, filepath.Join(ws, LedgerFile)); after != before || len(*got) != 0 {
		t.Fatalf("untouched and silent: %v", codesOf(*got))
	}
}

func TestEmit_ReadOrWriteFault_AuthorsTheWarningAndSignals(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, LedgerFile), 0o755); err != nil {
		t.Fatal(err)
	}
	l, got := observed(scopeOf(), resolveNever)
	v := l.Emit(Request{Cycle: 1, Workspace: ws}, Rejection{Defects: []string{"x"}})
	f := fixture{ws: ws, root: "/no-root-in-this-scenario"}
	assertVerdict(t, "emit_read_fault", v, goldenScenario(t, "emit_read_fault", f))
	e := only(t, *got, CodeEmitFailed)
	if e.Reason != v.Diagnostics[0].Message || e.Fields["op"] != "read" || e.Fields["step"] != "emit" || e.Fields["blocked"] != "false" || e.Fields["path"] != filepath.Join(ws, LedgerFile) || e.Origin != "Ledger.Emit" {
		t.Fatalf("read fault event carries the diagnostic as its reason: %+v", e)
	}

	ws = t.TempDir()
	mustEmit(t, l, Request{Cycle: 1, Workspace: ws}, Rejection{Defects: []string{"first"}})
	if err := os.Chmod(ws, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ws, 0o755) })
	v = l.Emit(Request{Cycle: 2, Workspace: ws}, Rejection{Defects: []string{"second"}})
	if v.Blocked || len(v.Diagnostics) != 1 || v.Diagnostics[0].Severity != "warning" {
		t.Fatalf("the write must have failed on the read-only workspace (never skip), as one warning: %+v", v)
	}
	msg := v.Diagnostics[0].Message
	if !strings.HasPrefix(msg, "defect ledger: could not record this cycle's defects (atomicwrite:") || !strings.HasSuffix(msg, ") — a later continuation will have nothing to reconcile against") {
		t.Fatalf("the host wire around the write error: %s", msg)
	}
	if e := (*got)[len(*got)-1]; e.Code != CodeEmitFailed || e.Fields["op"] != "write" || e.Cycle != 2 || e.Reason != msg {
		t.Fatalf("write fault event: %+v", e)
	}
}
