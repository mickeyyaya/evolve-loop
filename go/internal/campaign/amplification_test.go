package campaign_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/campaign"
)

func TestAmplificationVerifyRejectsCycleOverflowWithSingleWidthWaves(t *testing.T) {
	atLimit := amplificationPlan(t, campaign.MaxCycles, true)
	if err := atLimit.Verify(); err != nil {
		t.Fatalf("invalid at-limit chain fixture: %v", err)
	}

	overLimit := amplificationPlan(t, campaign.MaxCycles+1, true)
	err := overLimit.Verify()
	if err == nil {
		t.Fatalf("Verify accepted %d cycles with wave width 1; max is %d", campaign.MaxCycles+1, campaign.MaxCycles)
	}
	if text := strings.ToLower(err.Error()); !strings.Contains(text, "cycle") || !strings.Contains(text, fmt.Sprint(campaign.MaxCycles)) {
		t.Fatalf("cycle overflow error %q does not identify the %d-cycle limit", err, campaign.MaxCycles)
	}
}

func TestAmplificationVerifyRejectsWaveOverflowBelowCycleLimit(t *testing.T) {
	atLimit := amplificationPlan(t, campaign.MaxWaveWidth, false)
	if err := atLimit.Verify(); err != nil {
		t.Fatalf("invalid at-limit independent fixture: %v", err)
	}

	overLimit := amplificationPlan(t, campaign.MaxWaveWidth+1, false)
	err := overLimit.Verify()
	if err == nil {
		t.Fatalf("Verify accepted wave width %d; max is %d", campaign.MaxWaveWidth+1, campaign.MaxWaveWidth)
	}
	if text := strings.ToLower(err.Error()); !strings.Contains(text, "wave") || !strings.Contains(text, fmt.Sprint(campaign.MaxWaveWidth)) {
		t.Fatalf("wave overflow error %q does not identify the width-%d limit", err, campaign.MaxWaveWidth)
	}
}

func amplificationPlan(t *testing.T, count int, chain bool) campaign.Plan {
	t.Helper()

	var plan campaign.Plan
	value := reflect.ValueOf(&plan).Elem()
	seedAmplificationStruct(t, value, "plan", 0)

	cycles := value.FieldByName("Cycles")
	if !cycles.IsValid() || cycles.Kind() != reflect.Slice {
		t.Fatal("campaign.Plan must expose a Cycles slice")
	}

	items := reflect.MakeSlice(cycles.Type(), count, count)
	for i := 0; i < count; i++ {
		item := items.Index(i)
		target := item
		if item.Kind() == reflect.Pointer {
			item.Set(reflect.New(item.Type().Elem()))
			target = item.Elem()
		}
		seedAmplificationStruct(t, target, fmt.Sprintf("cycle-%03d", i), i)
		if chain && i > 0 {
			setAmplificationDependencies(t, target, fmt.Sprintf("cycle-%03d", i-1))
		}
	}
	cycles.Set(items)
	return plan
}

func seedAmplificationStruct(t *testing.T, value reflect.Value, identity string, index int) {
	t.Helper()
	if value.Kind() != reflect.Struct {
		t.Fatalf("expected struct fixture target, got %s", value.Kind())
	}

	for i := 0; i < value.NumField(); i++ {
		fieldInfo := value.Type().Field(i)
		field := value.Field(i)
		if !field.CanSet() || strings.Contains(strings.ToLower(fieldInfo.Name), "depend") {
			continue
		}

		name := strings.ToLower(fieldInfo.Name)
		switch field.Kind() {
		case reflect.String:
			switch {
			case name == "id", strings.HasSuffix(name, "id"):
				field.SetString(identity)
			case strings.Contains(name, "hash"):
				field.SetString(fmt.Sprintf("%064x", index+1))
			default:
				field.SetString(identity)
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			field.SetInt(1)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			field.SetUint(1)
		case reflect.Bool:
			field.SetBool(true)
		case reflect.Slice:
			if field.Type().Elem().Kind() == reflect.String {
				field.Set(reflect.ValueOf([]string{fmt.Sprintf("%s-%s", identity, name)}).Convert(field.Type()))
			}
		case reflect.Struct:
			seedAmplificationStruct(t, field, identity, index)
		case reflect.Pointer:
			if field.Type().Elem().Kind() == reflect.Struct {
				field.Set(reflect.New(field.Type().Elem()))
				seedAmplificationStruct(t, field.Elem(), identity, index)
			}
		}
	}
}

func setAmplificationDependencies(t *testing.T, cycle reflect.Value, dependency string) {
	t.Helper()
	for i := 0; i < cycle.NumField(); i++ {
		fieldInfo := cycle.Type().Field(i)
		if !strings.Contains(strings.ToLower(fieldInfo.Name), "depend") {
			continue
		}
		field := cycle.Field(i)
		if field.CanSet() && field.Kind() == reflect.Slice && field.Type().Elem().Kind() == reflect.String {
			field.Set(reflect.ValueOf([]string{dependency}).Convert(field.Type()))
			return
		}
	}
	t.Fatal("campaign cycle type must expose a string dependency slice")
}
