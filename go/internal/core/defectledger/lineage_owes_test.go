package defectledger

import (
	"os"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

func TestAncestorStanding_ReadsWhatTheAncestorLedgerOwes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, ledger string
		want         Standing
		fault        bool
	}{
		{name: "no ledger", want: StandingAbsent},
		{name: "one open row", ledger: ancestorOneOpen, want: StandingOwed},
		{name: "only settled rows", ledger: `{"origin_cycle":1255,"entries":[{"id":"d1","text":"x","status":"FIXED","source":"cycle-1254","evidence":"go/x.go"}]}`, want: StandingSettled},
		{name: "unreadable", ledger: `{not json`, fault: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			if tc.ledger != "" {
				f.ancestorLedger(t, tc.ledger)
			}
			got, err := AncestorStanding(f.root, 1255)
			if got != tc.want || (err != nil) != tc.fault {
				t.Fatalf("AncestorStanding = %v, %v; want %v, fault %v", got, err, tc.want, tc.fault)
			}
		})
	}
}

func TestFirstBinding_ReturnsTheFirstBoundIdOfTheGivenOrder(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.bindRegistry(t, 1255)
	cases := []struct {
		name   string
		ids    []string
		wantID string
		ok     bool
	}{
		{name: "bound first", ids: []string{"lane-scope-id", "other"}, wantID: "lane-scope-id", ok: true},
		{name: "unbound first", ids: []string{"other", "lane-scope-id"}, wantID: "lane-scope-id", ok: true},
		{name: "none bound", ids: []string{"other"}},
		{name: "empty", ids: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, c, ok := FirstBinding(f.root, tc.ids)
			if ok != tc.ok || id != tc.wantID || (ok && c.Cycle != 1255) {
				t.Fatalf("FirstBinding = %q, %+v, %v; want %q, ok %v", id, c, ok, tc.wantID, tc.ok)
			}
		})
	}
}

func TestFirstBinding_AnUnreadableRegistryIsNoBinding(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	if err := os.WriteFile(continuation.RegistryPath(f.root), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if id, _, ok := FirstBinding(f.root, []string{"lane-scope-id"}); ok {
		t.Fatalf("FirstBinding = %q, true; want no binding from an unreadable registry", id)
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
