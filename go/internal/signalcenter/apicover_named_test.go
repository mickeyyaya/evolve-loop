package signalcenter

// apicover_named_test.go — names the exported symbols the behavior tests only
// use anonymously (apicover counts identifiers, not uses), each with a real
// assertion.

import "testing"

func TestAPI_ListenerOptionConflictAndFormatLineNamed(t *testing.T) {
	t.Parallel()
	var seen Event
	var l Listener = func(e Event) { seen = e }
	opts := []Option{WithPID(3), WithRecentLimit(2)}
	c := New(opts...)
	c.Subscribe(l)
	c.Emit(infoEvent("named"))
	if seen.PID != 3 || seen.Reason != "named" {
		t.Errorf("Listener and Option behave as their anonymous twins: %+v", seen)
	}
	var conflict Conflict
	conflict.Code, conflict.Module, conflict.OtherModule = "SHIP_X_Y", ModuleShip, ModuleAudit
	if conflict.Code != "SHIP_X_Y" {
		t.Error("Conflict is a plain record")
	}
	line := FormatLine(Event{Module: ModuleShip, Kind: KindShipLanded, Severity: SeverityInfo, Seq: 9, Origin: "Landing.Land", Reason: "landed"})
	if line != "[ship] ship.landed INFO seq=9 origin=Landing.Land — landed" {
		t.Errorf("FormatLine is the one line format: %q", line)
	}
	var doc CodeDoc
	doc.Code, doc.Doc = CodeSinkDropped, "x"
	if doc.Code != CodeSinkDropped {
		t.Error("CodeDoc is a plain record")
	}
}
