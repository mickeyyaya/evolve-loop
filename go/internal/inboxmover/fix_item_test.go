package inboxmover

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConsumeFixItem_APendingItemMovesToProcessedOfTheCycle(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	pending := seedItemIn(t, inbox, "fix-main-red-abc")

	if err := ConsumeFixItem(Options{ProjectRoot: root}, "fix-main-red-abc", PromoteOpts{Cycle: "9"}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(pending); !os.IsNotExist(err) {
		t.Errorf("the fix item is pending after the consume: %v", err)
	}
	if _, err := os.Stat(filepath.Join(inbox, "processed", "cycle-9", filepath.Base(pending))); err != nil {
		t.Errorf("the fix item is not in processed/cycle-9: %v", err)
	}
}

func TestConsumeFixItem_AnItemThatIsNotInTheInboxIsNoError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	other := seedItemIn(t, inbox, "other-item")

	if err := ConsumeFixItem(Options{ProjectRoot: root}, "fix-main-red-abc", PromoteOpts{Cycle: "9"}); err != nil {
		t.Fatalf("a fix item that is not in the inbox: %v", err)
	}

	if _, err := os.Stat(other); err != nil {
		t.Errorf("the consume moved an item that is not the fix item: %v", err)
	}
	if _, err := os.Stat(filepath.Join(inbox, "processed")); !os.IsNotExist(err) {
		t.Errorf("the consume made a processed directory for no item: %v", err)
	}
}

func TestConsumeFixItem_AFailureNamesTheFixItem(t *testing.T) {
	t.Parallel()
	cases := map[string]func(t *testing.T, inbox string){
		"the inbox cannot be read": func(t *testing.T, inbox string) {
			if err := os.Chmod(inbox, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(inbox, 0o755) })
		},
		"the processed directory is a file": func(t *testing.T, inbox string) {
			if err := os.WriteFile(filepath.Join(inbox, "processed"), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, breakInbox := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			inbox := filepath.Join(root, ".evolve", "inbox")
			seedItemIn(t, inbox, "fix-main-red-abc")
			breakInbox(t, inbox)

			err := ConsumeFixItem(Options{ProjectRoot: root}, "fix-main-red-abc", PromoteOpts{Cycle: "9"})

			if err == nil || !strings.HasPrefix(err.Error(), "fix item fix-main-red-abc: ") {
				t.Errorf("err = %v, want an error that names the fix item", err)
			}
		})
	}
}

func TestConsumeFixItem_ReadsTheInboxOfTheOptions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inbox := filepath.Join(t.TempDir(), "inbox")
	pending := seedItemIn(t, inbox, "fix-main-red-abc")

	if err := ConsumeFixItem(Options{ProjectRoot: root, InboxDir: inbox}, "fix-main-red-abc", PromoteOpts{Cycle: "9"}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(inbox, "processed", "cycle-9", filepath.Base(pending))); err != nil {
		t.Errorf("the fix item is not in processed/cycle-9 of the inbox of the options: %v", err)
	}
}
