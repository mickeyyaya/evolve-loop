package inboxbatch

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeInboxFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func noticeOnlyInbox() map[string]string {
	return map[string]string{
		"long.json":  `{"id":"long","title":"` + strings.Repeat("x", maxFieldLen+1) + `"}`,
		"dup-a.json": `{"id":"dup"}`,
		"dup-b.json": `{"id":"dup"}`,
	}
}

func TestDirScan_ANoticeAboutAReadableItemIsNotUnreadable(t *testing.T) {
	scan, err := ScanDir(writeInboxFiles(t, noticeOnlyInbox()))
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.Items) != 3 || len(scan.Warnings) != 2 {
		t.Fatalf("a sanitized field and a duplicate id are warnings about readable items: %d items, %v", len(scan.Items), scan.Warnings)
	}
	if scan.HasUnreadable() {
		t.Errorf("no file is unreadable: %v", scan.Warnings)
	}
	if !(DirScan{Warnings: []LoadWarning{{Text: "notice"}, {Text: "broken", Unreadable: true}}}).HasUnreadable() {
		t.Error("one unreadable file among notices makes the scan unreadable")
	}
}

func TestLoadWarning_AFileItCannotParseIsUnreadable(t *testing.T) {
	files := noticeOnlyInbox()
	files["broken.json"] = `{not json`
	scan, err := ScanDir(writeInboxFiles(t, files))
	if err != nil {
		t.Fatal(err)
	}
	var unreadable []LoadWarning
	for _, w := range scan.Warnings {
		if w.Unreadable {
			unreadable = append(unreadable, w)
		}
	}
	if len(unreadable) != 1 || !strings.HasPrefix(unreadable[0].Text, "broken.json: ") || !scan.HasUnreadable() {
		t.Errorf("only the file it could not parse is unreadable, got %v", unreadable)
	}
}

func TestLoadDir_WarnsExactlyAsScanDirDoes(t *testing.T) {
	files := noticeOnlyInbox()
	files["broken.json"] = `{not json`
	dir := writeInboxFiles(t, files)
	scan, err := ScanDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	items, warnings, err := LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, w := range scan.Warnings {
		texts = append(texts, w.Text)
	}
	if !reflect.DeepEqual(items, scan.Items) || !reflect.DeepEqual(warnings, texts) {
		t.Errorf("LoadDir projects ScanDir: items %v vs %v, warnings %v vs %v", items, scan.Items, warnings, texts)
	}
}

func TestScanDir_ADirectoryItCannotListIsAnError(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-dir.json")
	if err := os.WriteFile(file, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ScanDir(file); err == nil || !strings.HasPrefix(err.Error(), "inboxbatch: read dir: ") {
		t.Errorf("a path it cannot list is an error, got %v", err)
	}
}
