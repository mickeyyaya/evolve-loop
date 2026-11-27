package inboxmover

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
)

type failingAppender struct{}

func (failingAppender) AppendLifecycle(context.Context, ledger.LifecycleRecord) error {
	return errors.New("ledger write failed")
}

func TestRequeuer_Requeue_MovesDeliveredItemWithBriefAndBumpsFailureCount(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inboxDir := filepath.Join(root, ".evolve", "inbox")
	deliveredDir := filepath.Join(inboxDir, StateDelivered)
	if err := os.MkdirAll(deliveredDir, 0o755); err != nil {
		t.Fatal(err)
	}
	deliveredPath := filepath.Join(deliveredDir, "2026-11-26T00-00-00Z-item-a.json")
	if err := os.WriteFile(deliveredPath, []byte(`{"id":"item-a","title":"keep title","failure_count":2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := committedset.WriteRecord(root, committedset.Record{
		Cycle:   41,
		ItemIDs: []string{"item-a", "not-in-delivered"},
	}); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	recLedger := &recordingAppender{}
	fixedNow := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	var port integration.Requeuer = Requeuer{
		Options: Options{
			ProjectRoot: root,
			Stderr:      &stderr,
			Ledger:      recLedger,
			Now:         func() time.Time { return fixedNow },
		},
	}

	err := port.Requeue(context.Background(), integration.DeliverableRecord{Cycle: 41}, "/waves/integration-7/brief-41.txt")
	if err != nil {
		t.Fatalf("Requeue = %v", err)
	}
	if _, statErr := os.Stat(deliveredPath); !os.IsNotExist(statErr) {
		t.Errorf("delivered copy still exists: %v", statErr)
	}
	rootCopy := filepath.Join(inboxDir, "2026-11-26T00-00-00Z-item-a.json")
	raw, err := os.ReadFile(rootCopy)
	if err != nil {
		t.Fatal(err)
	}
	var item map[string]any
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatal(err)
	}
	if item["brief_path"] != "/waves/integration-7/brief-41.txt" {
		t.Errorf("brief_path = %v", item["brief_path"])
	}
	if item["failure_count"] != float64(3) {
		t.Errorf("failure_count = %v, want 3", item["failure_count"])
	}
	if item["title"] != "keep title" {
		t.Errorf("title = %v, want original title kept", item["title"])
	}
	if len(recLedger.records) != 1 || recLedger.records[0].Action != "requeue" || recLedger.records[0].TaskID != "item-a" || recLedger.records[0].Cycle != 41 {
		t.Errorf("ledger = %+v", recLedger.records)
	}
	if !strings.Contains(stderr.String(), "requeued: 2026-11-26T00-00-00Z-item-a.json ← delivered/") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestRequeuer_Requeue_NoOpsAndErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	if err := (Requeuer{}).Requeue(ctx, integration.DeliverableRecord{Cycle: 41}, "b"); !errors.Is(err, errNoProjectRoot) {
		t.Errorf("empty Options = %v, want errNoProjectRoot", err)
	}

	root := t.TempDir()
	inboxDir := filepath.Join(root, ".evolve", "inbox")
	deliveredDir := filepath.Join(inboxDir, StateDelivered)
	if err := os.MkdirAll(deliveredDir, 0o755); err != nil {
		t.Fatal(err)
	}

	req := Requeuer{Options: Options{ProjectRoot: root}}
	if err := req.Requeue(ctx, integration.DeliverableRecord{Cycle: 0}, ""); err != nil {
		t.Errorf("cycle 0 = %v, want nil", err)
	}
	if err := req.Requeue(ctx, integration.DeliverableRecord{Cycle: 99}, ""); err != nil {
		t.Errorf("missing lineage record = %v, want nil", err)
	}

	t.Run("empty brief path and ledger append warn", func(t *testing.T) {
		t.Parallel()
		r := t.TempDir()
		dDir := filepath.Join(r, ".evolve", "inbox", StateDelivered)
		if err := os.MkdirAll(dDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dDir, "item-b.json"), []byte(`{"id":"item-b"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := committedset.WriteRecord(r, committedset.Record{Cycle: 12, ItemIDs: []string{"item-b"}}); err != nil {
			t.Fatal(err)
		}
		var stderr bytes.Buffer
		err := (Requeuer{Options: Options{ProjectRoot: r, Ledger: failingAppender{}, Stderr: &stderr}}).
			Requeue(ctx, integration.DeliverableRecord{Cycle: 12}, "")
		if err != nil {
			t.Fatalf("Requeue = %v", err)
		}
		if !strings.Contains(stderr.String(), "ledger append") {
			t.Errorf("stderr = %q, want ledger warn", stderr.String())
		}
		raw, err := os.ReadFile(filepath.Join(r, ".evolve", "inbox", "item-b.json"))
		if err != nil {
			t.Fatal(err)
		}
		var item map[string]any
		if err := json.Unmarshal(raw, &item); err != nil {
			t.Fatal(err)
		}
		if _, hasBrief := item["brief_path"]; hasBrief {
			t.Errorf("brief_path present when empty: %v", item)
		}
		if item["failure_count"] != float64(1) {
			t.Errorf("failure_count = %v, want 1", item["failure_count"])
		}
	})

	t.Run("existing root copy refuses", func(t *testing.T) {
		t.Parallel()
		r := t.TempDir()
		inbox := filepath.Join(r, ".evolve", "inbox")
		dDir := filepath.Join(inbox, StateDelivered)
		if err := os.MkdirAll(dDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dDir, "dup.json"), []byte(`{"id":"dup"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(inbox, "dup.json"), []byte(`{"id":"dup"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := committedset.WriteRecord(r, committedset.Record{Cycle: 13, ItemIDs: []string{"dup"}}); err != nil {
			t.Fatal(err)
		}
		err := (Requeuer{Options: Options{ProjectRoot: r}}).Requeue(ctx, integration.DeliverableRecord{Cycle: 13}, "b")
		if !errors.Is(err, ErrMvFailed) {
			t.Errorf("existing root copy = %v, want ErrMvFailed", err)
		}
	})

	t.Run("unwritable delivered dir fails UpdateItemJSON", func(t *testing.T) {
		t.Parallel()
		r := t.TempDir()
		dDir := filepath.Join(r, ".evolve", "inbox", StateDelivered)
		if err := os.MkdirAll(dDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dDir, "bad.json"), []byte(`{"id":"bad"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := committedset.WriteRecord(r, committedset.Record{Cycle: 14, ItemIDs: []string{"bad"}}); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(dDir, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dDir, 0o755) })
		if err := (Requeuer{Options: Options{ProjectRoot: r}}).Requeue(ctx, integration.DeliverableRecord{Cycle: 14}, "b"); err == nil {
			t.Error("Requeue in read-only delivered dir = nil, want error")
		}
	})

	t.Run("unwritable inbox root fails Rename", func(t *testing.T) {
		t.Parallel()
		r := t.TempDir()
		inbox := filepath.Join(r, ".evolve", "inbox")
		dDir := filepath.Join(inbox, StateDelivered)
		if err := os.MkdirAll(dDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dDir, "mvfail.json"), []byte(`{"id":"mvfail"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := committedset.WriteRecord(r, committedset.Record{Cycle: 15, ItemIDs: []string{"mvfail"}}); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(inbox, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(inbox, 0o755) })
		err := (Requeuer{Options: Options{ProjectRoot: r}}).Requeue(ctx, integration.DeliverableRecord{Cycle: 15}, "b")
		if !errors.Is(err, ErrMvFailed) {
			t.Errorf("Rename in read-only inbox = %v, want ErrMvFailed", err)
		}
	})
}
