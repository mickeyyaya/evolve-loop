package bridge

import (
	"reflect"
	"testing"
)

func TestAdoptFamilyEffort_AFamilyThatDoesNotLoadLeavesTheManifestAsItIs(t *testing.T) {
	t.Parallel()
	own := map[string]ParamSpec{"model_tier": {Flag: "--model"}}
	headless := Manifest{CLI: "headless", ModelTierMapFrom: "no-such-family-manifest", Params: own}

	got := adoptFamilyEffort(headless)

	if !reflect.DeepEqual(got, headless) {
		t.Errorf("adoptFamilyEffort = %#v, want the manifest unchanged when its family does not load", got)
	}
}

func TestAdoptFamilyEffort_AFamilyWithNoEffortTableAddsNoEffortParam(t *testing.T) {
	t.Parallel()
	family, err := loadManifestRaw("agy")
	if err != nil {
		t.Fatalf("load agy: %v", err)
	}
	if _, declares := family.Params["effort"]; declares {
		t.Fatal("agy declares an effort param: this test needs a family manifest with none")
	}
	own := map[string]ParamSpec{"model_tier": {Flag: "--model"}}
	headless := Manifest{CLI: "headless", ModelTierMapFrom: "agy", Params: own}

	got := adoptFamilyEffort(headless)

	if _, invented := got.Params["effort"]; invented {
		t.Errorf("adoptFamilyEffort invented an effort param from a family that declares none: %#v", got.Params)
	}
	if !reflect.DeepEqual(got, headless) {
		t.Errorf("adoptFamilyEffort = %#v, want the manifest unchanged", got)
	}
}
