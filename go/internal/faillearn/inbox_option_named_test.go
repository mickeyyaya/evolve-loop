package faillearn

import "testing"

// TestOption_AppliesInboxDestination names the exported `Option` type in a real
// executing assertion. Required by the repo-wide apicover -enforce gate
// (go/.apicover-enforce:81): WithInbox's return type is otherwise never spelled
// anywhere, so the symbol would read as an unnamed export and FAIL the gate.
//
// It is not a naming stub. It pins the one thing the behavioral tests cannot
// see from outside: that applying an Option actually MUTATES the write config.
// An Option that silently no-opped would still write a retrospective and still
// pass every artifact assertion — while quietly restoring the exact 1255 state
// where remediation exists only in the report.
func TestOption_AppliesInboxDestination(t *testing.T) {
	items := []InboxItem{{ID: "retro-1279-example", Title: "example remediation"}}

	var opt Option = WithInbox("/nonexistent/inbox", items)
	var cfg writeConfig
	opt(&cfg)

	if cfg.inboxDir != "/nonexistent/inbox" {
		t.Errorf("inboxDir = %q, want the destination the option carried", cfg.inboxDir)
	}
	if len(cfg.inboxItems) != len(items) || cfg.inboxItems[0].ID != items[0].ID {
		t.Errorf("inboxItems = %+v, want %+v", cfg.inboxItems, items)
	}
}
