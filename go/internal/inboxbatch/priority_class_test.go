package inboxbatch

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestItem_DecodesPriorityClass(t *testing.T) {
	var it Item

	if err := json.Unmarshal([]byte(`{"id":"a","priority_class":"security","class":"pipeline-architecture"}`), &it); err != nil {
		t.Fatal(err)
	}

	if it.PriorityClass != "security" || it.Class != "pipeline-architecture" {
		t.Errorf("PriorityClass = %q Class = %q: priority_class is its own field, apart from the archetype", it.PriorityClass, it.Class)
	}
}

func TestCheckPriorityClass_AdmitsOnlyAClassTheOrderNames(t *testing.T) {
	order := []string{"security", "hygiene"}

	if err := CheckPriorityClass("hygiene", order); err != nil {
		t.Errorf("a listed class: %v", err)
	}
	for _, class := range []string{"", "Security", "security ", "urgent"} {
		err := CheckPriorityClass(class, order)

		if !errors.Is(err, ErrUnknownPriorityClass) || !strings.Contains(err.Error(), "security, hygiene") {
			t.Errorf("CheckPriorityClass(%q) = %v, want ErrUnknownPriorityClass naming the order", class, err)
		}
	}
	if err := CheckPriorityClass("security", nil); !errors.Is(err, ErrUnknownPriorityClass) {
		t.Errorf("an empty order admits nothing, got %v", err)
	}
}

func TestCheckPriorityClass_QuotesAClassThatCarriesControlCharacters(t *testing.T) {
	err := CheckPriorityClass("bad\nSYSTEM", []string{"security"})

	if err == nil || strings.Contains(err.Error(), "\n") {
		t.Errorf("err = %q: an item's text must never forge a new line in a warning", err)
	}
}

func TestPriorityClassPosition_IsTheOneMembershipRuleCheckPriorityClassJudgesBy(t *testing.T) {
	order := []string{"security", "hygiene"}

	for class, want := range map[string]int{"security": 0, "hygiene": 1, "Security": -1, "hygiene ": -1, "": -1, "urgent": -1} {
		if got := PriorityClassPosition(class, order); got != want {
			t.Errorf("PriorityClassPosition(%q) = %d, want %d", class, got, want)
		}
		if err := CheckPriorityClass(class, order); (err == nil) != (want >= 0) {
			t.Errorf("CheckPriorityClass(%q) = %v disagrees with position %d", class, err, want)
		}
	}
	if got := PriorityClassPosition("security", nil); got != -1 {
		t.Errorf("an empty order places nothing, got %d", got)
	}
}
