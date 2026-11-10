package lifecycle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateItemJSON_KeepsTheFiledFormat(t *testing.T) {
	inbox := newInbox(t)
	path := filepath.Join(inbox, "a.json")
	filed := "{\n  \"criteria\": \"ids stay as <id>\",\n  \"id\": \"a\",\n  \"weight\": 0.3\n}\n"
	writeItem(t, path, filed)
	if err := UpdateItemJSON(path, func(m map[string]json.RawMessage) { m["weight"] = json.RawMessage("0.4") }); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(filed, "0.3", "0.4", 1)
	if string(body) != want {
		t.Errorf("one field edit must change one line\nwant:\n%s\ngot:\n%s", want, body)
	}
}
