package lifecycle

import (
	"errors"
	"strings"
	"testing"
)

var filingClassOrder = []string{"correctness", "stability", "performance", "debuggability", "feature", "maintainability", "hygiene", "security"}

func newFiler(inbox string, appender LedgerAppender, opts ...Option) *Mover {
	return New(inbox, appender, append([]Option{WithNow(filingClock), WithPriorityClasses(filingClassOrder)}, opts...)...)
}

func TestMover_File_RefusesAPriorityClassTheOrderDoesNotName(t *testing.T) {
	for name, body := range map[string]string{
		"an unknown class": strings.Replace(validItem, `"priority_class":"debuggability"`, `"priority_class":"urgent"`, 1),
		"no class":         strings.Replace(validItem, `"priority_class":"debuggability",`, ``, 1),
		"a mis-cased one":  strings.Replace(validItem, `"priority_class":"debuggability"`, `"priority_class":"Security"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			inbox := newInbox(t)
			rec := &recordingAppender{}

			_, err := newFiler(inbox, rec).File([]byte(body))

			if !errors.Is(err, ErrInvalidItem) || !strings.Contains(err.Error(), "priority_class") || !strings.Contains(err.Error(), "correctness, stability") {
				t.Errorf("err = %v, want ErrInvalidItem naming priority_class and the class order", err)
			}
			if files := jsonFiles(t, inbox); len(files) != 0 || len(rec.records) != 0 {
				t.Errorf("a refused class left files %v or ledger %+v", files, rec.records)
			}
		})
	}
}

func TestMover_File_JudgesTheClassAgainstTheWiredOrder(t *testing.T) {
	body := strings.Replace(validItem, `"priority_class":"debuggability"`, `"priority_class":"pipeline-health"`, 1)

	if _, err := newFiler(newInbox(t), nil).File([]byte(body)); !errors.Is(err, ErrInvalidItem) {
		t.Fatalf("a class outside the default order: err = %v, want ErrInvalidItem", err)
	}
	if _, err := newFiler(newInbox(t), nil, WithPriorityClasses([]string{"pipeline-health"})).File([]byte(body)); err != nil {
		t.Errorf("the operator's own class order admits its class: %v", err)
	}
}

func TestMover_File_WithNoClassOrderWiredIsAFaultNotAnInvalidItem(t *testing.T) {
	inbox := newInbox(t)
	rec := &recordingAppender{}

	_, err := New(inbox, rec, WithNow(filingClock)).File([]byte(validItem))

	if err == nil || errors.Is(err, ErrInvalidItem) || !strings.Contains(err.Error(), "class order") {
		t.Errorf("err = %v, want a wiring fault naming the class order", err)
	}
	if files := jsonFiles(t, inbox); len(files) != 0 || len(rec.records) != 0 {
		t.Errorf("an unwired filer left files %v or ledger %+v", files, rec.records)
	}
}
