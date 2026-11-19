package evidencex_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evidencex"
)

func TestExtract_NamesEveryExportInOneRealRun(t *testing.T) {
	t.Parallel()
	src := []byte("a\nb\nERROR boom\nc\nd\nd\ne\n")
	var res evidencex.Result
	res, err := evidencex.Extract(src, evidencex.Options{Mode: evidencex.ModeExtract, Budget: 4096})
	if err != nil {
		t.Fatal(err)
	}
	if res.Size != len(src) || res.Digest == "" {
		t.Errorf("Size=%d Digest=%q", res.Size, res.Digest)
	}
	if want := (evidencex.Range{Start: 1, End: 7}); len(res.Ranges) != 1 || res.Ranges[0] != want {
		t.Errorf("Ranges = %v, want [%v]", res.Ranges, want)
	}
	if !strings.Contains(res.Text, "ERROR boom") {
		t.Errorf("Text = %q", res.Text)
	}
	var tpl evidencex.Template
	long := []byte(strings.Repeat("x\n", 60) + "ERROR e\n")
	res, err = evidencex.Extract(long, evidencex.Options{Mode: evidencex.ModeExtract, Budget: 4096})
	if err != nil || len(res.Templates) == 0 {
		t.Fatalf("templates = %v err=%v", res.Templates, err)
	}
	tpl = res.Templates[0]
	if tpl.Pattern != "x" || tpl.Count < 2 || tpl.First != "x" || tpl.Last != "x" {
		t.Errorf("template = %+v", tpl)
	}
	ptr, err := evidencex.Extract(src, evidencex.Options{Mode: evidencex.ModePointer})
	if err != nil || ptr.Text != "" || ptr.Size != len(src) {
		t.Errorf("pointer = %+v err=%v", ptr, err)
	}
	if _, err := evidencex.Extract(src, evidencex.Options{Mode: evidencex.ModeExtract}); err == nil {
		t.Errorf("a zero budget was accepted")
	}
	if _, err := evidencex.Extract(src, evidencex.Options{Mode: "nope", Budget: 1}); err == nil {
		t.Errorf("an unknown mode was accepted")
	}
}

func TestArchive_NamesEveryExportInOneRealRun(t *testing.T) {
	t.Parallel()
	var archive evidencex.EvidenceArchive
	archive, err := evidencex.NewLocalArchive(t.TempDir(), 64)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("archived log line\n")
	sum := sha256.Sum256(payload)
	key := hex.EncodeToString(sum[:])
	if err := archive.Put(key, payload); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := archive.Get(key)
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("Get = (%q, %v), want %q", got, err, payload)
	}
	oversized := bytes.Repeat([]byte("a"), 65)
	overSum := sha256.Sum256(oversized)
	if err := archive.Put(hex.EncodeToString(overSum[:]), oversized); !errors.Is(err, evidencex.ErrArchiveTooLarge) {
		t.Errorf("oversized Put err = %v, want ErrArchiveTooLarge", err)
	}
	if _, err := archive.Get(strings.Repeat("0", 64)); !errors.Is(err, evidencex.ErrArchiveNotFound) {
		t.Errorf("missing Get err = %v, want ErrArchiveNotFound", err)
	}
}
