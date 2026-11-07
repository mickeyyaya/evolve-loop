//go:build acs

package cycle2218

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func bridgeTests(t *testing.T, pattern string, names ...string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(root, "go"),
		Package: "./internal/bridge",
		Pattern: pattern,
		Names:   names,
	})
}

func TestC2218_001_TheRecipeAdapterProbeTagsItsPaneAndPersistsTheTreeBeforeKillSession(t *testing.T) {
	bridgeTests(t,
		"^(TestProbeTag_CaptureHelpTagsItsPaneAndPersistsTheTreeBeforeKillSession|TestProbeTag_TheProbeStillStopsTheRecordedPaneChildAtTheEndOfTheProbe)$",
		"TestProbeTag_CaptureHelpTagsItsPaneAndPersistsTheTreeBeforeKillSession",
		"TestProbeTag_TheProbeStillStopsTheRecordedPaneChildAtTheEndOfTheProbe")
}

func TestC2218_002_TheCaptureModelsProbeTagsItsPaneAndPersistsTheTreeBeforeKillSession(t *testing.T) {
	bridgeTests(t,
		"^TestProbeTag_CaptureModelPickerTagsItsPaneAndPersistsTheTreeBeforeKillSession$",
		"TestProbeTag_CaptureModelPickerTagsItsPaneAndPersistsTheTreeBeforeKillSession")
}

func TestC2218_003_TheLaunchModelVerificationKillTagsItsPaneAndPersistsTheTreeBeforeKillSession(t *testing.T) {
	bridgeTests(t,
		"^TestProbeTag_AnUnnamedLaunchModelMismatchKillTagsItsPaneAndPersistsTheTreeBeforeKillSession$",
		"TestProbeTag_AnUnnamedLaunchModelMismatchKillTagsItsPaneAndPersistsTheTreeBeforeKillSession")
}

func TestC2218_004_GCReapsALeakedProbeSessionOfEachPathThroughAnInjectedSender(t *testing.T) {
	bridgeTests(t,
		"^(TestProbeGC_ALeakedHelpProbeSessionIsReapedByGCThroughAnInjectedSender|TestProbeGC_ALeakedModelPickerProbeSessionIsReapedByGCThroughAnInjectedSender|TestProbeGC_ALeakedLaunchModelMismatchProbeSessionIsReapedByGCThroughAnInjectedSender)$",
		"TestProbeGC_ALeakedHelpProbeSessionIsReapedByGCThroughAnInjectedSender",
		"TestProbeGC_ALeakedModelPickerProbeSessionIsReapedByGCThroughAnInjectedSender",
		"TestProbeGC_ALeakedLaunchModelMismatchProbeSessionIsReapedByGCThroughAnInjectedSender")
}

func TestC2218_005_GCSparesALiveDispatchAnotherProjectAndUntaggedProcesses(t *testing.T) {
	bridgeTests(t,
		"^TestProbeGC_ALeakedProbeTagIsNoWarrantToStopALiveDispatchAnotherProjectOrAnUntaggedProcess$",
		"TestProbeGC_ALeakedProbeTagIsNoWarrantToStopALiveDispatchAnotherProjectOrAnUntaggedProcess")
}
