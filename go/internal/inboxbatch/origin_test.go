package inboxbatch

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestItem_DecodesOrigin(t *testing.T) {
	var it Item
	if err := json.Unmarshal([]byte(`{"id":"a","origin":"retro"}`), &it); err != nil {
		t.Fatal(err)
	}
	if it.Origin != OriginRetro {
		t.Errorf("Origin = %q, want %q", it.Origin, OriginRetro)
	}
}

func TestCheckOrigin_AdmitsOnlyTheFourOrigins(t *testing.T) {
	for _, origin := range Origins() {
		if err := CheckOrigin(origin); err != nil {
			t.Errorf("CheckOrigin(%q) = %v, want nil", origin, err)
		}
	}
	if got := Origins(); len(got) != 4 || got[0] != OriginOperator || got[1] != OriginConsole || got[2] != OriginCycle || got[3] != OriginRetro {
		t.Errorf("Origins() = %v, want operator, console, cycle, retro", got)
	}
	for _, origin := range []string{"", "Operator", "lane", "human"} {
		if err := CheckOrigin(origin); !errors.Is(err, ErrUnknownOrigin) {
			t.Errorf("CheckOrigin(%q) = %v, want ErrUnknownOrigin", origin, err)
		}
	}
}
