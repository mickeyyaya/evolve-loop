package defectledger

import (
	"testing"
)

func TestAncestorOwes_ReadsWhetherTheAncestorLedgerHoldsAnOwedRow(t *testing.T) {
	cases := []struct {
		name, ledger string
		owes, fault  bool
	}{
		{name: "no ledger"},
		{name: "one open row", ledger: ancestorOneOpen, owes: true},
		{name: "only settled rows", ledger: `{"origin_cycle":1255,"entries":[{"id":"d1","text":"x","status":"FIXED","source":"cycle-1254","evidence":"go/x.go"}]}`},
		{name: "unreadable", ledger: `{not json`, fault: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			if tc.ledger != "" {
				f.ancestorLedger(t, tc.ledger)
			}
			owes, err := AncestorOwes(f.root, 1255)
			if owes != tc.owes || (err != nil) != tc.fault {
				t.Fatalf("AncestorOwes = %v, %v; want owes %v, fault %v", owes, err, tc.owes, tc.fault)
			}
		})
	}
}

func TestReconcile_TheMissingManifestBlockCarriesItsCodeAsTheFailureClass(t *testing.T) {
	f := newFixture(t)
	f.bindRegistry(t, 1255)
	f.removeManifest(t)
	v := New(scopeOf("lane-scope-id"), resolveNever).Reconcile(f.req)
	if !v.Blocked || len(v.Diagnostics) == 0 || v.Diagnostics[0].Code != string(CodeManifestMissing) {
		t.Fatalf("verdict = %+v; want a block whose first diagnostic has code %s", v, CodeManifestMissing)
	}
}
