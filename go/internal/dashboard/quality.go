package dashboard

import (
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type stampedReading struct {
	at      time.Time
	reading QualityReading
}

func readQuality(root string) []QualityReading {
	latest := map[string]stampedReading{}
	evolveDir := paths.EvolveDirOf(root)
	takeQualityStream(filepath.Join(evolveDir, signalcenter.StreamFileName), latest)
	ids, _ := workspaceCycles(root)
	for _, id := range ids {
		takeQualityStream(filepath.Join(runsDir(root), "cycle-"+strconv.Itoa(id), signalcenter.StreamFileName), latest)
	}
	takeQualityChannel(evolveDir, latest)
	indexes := qualitypanel.Indexes()
	out := make([]QualityReading, len(indexes))
	for i, ix := range indexes {
		if s, ok := latest[ix.ID]; ok {
			out[i] = s.reading
			continue
		}
		out[i] = QualityReading{Index: ix.ID, Stage: string(ix.DefaultStage())}
	}
	return out
}

func takeQualityStream(path string, latest map[string]stampedReading) {
	chunk, err := signalcenter.ReadStream(path, 0)
	if err != nil {
		return
	}
	for _, ev := range chunk.Events {
		recordQualityEvent(ev, latest)
	}
}

func takeQualityChannel(evolveDir string, latest map[string]stampedReading) {
	log, _ := channel.New(paths.EventChannelsRoot(evolveDir), "quality.reading", channel.Config{})
	info, err := os.Stat(log.Dir())
	if err != nil || !info.IsDir() {
		return
	}
	if batch, err := log.Read(0); err == nil {
		for _, rec := range batch.Records {
			if rec.Signal != nil {
				recordQualityEvent(*rec.Signal, latest)
			}
		}
	}
}

func recordQualityEvent(ev signalcenter.Event, latest map[string]stampedReading) {
	if ev.Kind != signalcenter.KindQualityReading {
		return
	}
	at, err := time.Parse(time.RFC3339Nano, ev.TS)
	if err != nil {
		return
	}
	qr, ok := parseQualityReading(ev.Fields)
	if !ok {
		return
	}
	if prev, exists := latest[qr.Index]; exists && at.Before(prev.at) {
		return
	}
	latest[qr.Index] = stampedReading{at: at, reading: qr}
}

func parseQualityReading(fields map[string]string) (QualityReading, bool) {
	id := fields["index"]
	ix, ok := qualitypanel.Lookup(id)
	if !ok {
		return QualityReading{}, false
	}
	val, err := strconv.ParseFloat(fields["value"], 64)
	if err != nil {
		return QualityReading{}, false
	}
	n, _ := strconv.Atoi(fields["n"])
	stage := fields["stage"]
	if stage == "" {
		stage = string(ix.DefaultStage())
	}
	qr := QualityReading{
		Index:      id,
		HasReading: true,
		Value:      &val,
		N:          n,
		Window:     fields["window"],
		Stage:      stage,
		Status:     fields["status"],
	}
	if fields["method"] != string(qualitypanel.MethodCensus) {
		qr.Lo, qr.Hi = parseBounds(fields["lo"], fields["hi"])
	}
	return qr, true
}

func parseBounds(loRaw, hiRaw string) (*float64, *float64) {
	lo, errLo := strconv.ParseFloat(loRaw, 64)
	hi, errHi := strconv.ParseFloat(hiRaw, 64)
	if errLo != nil || errHi != nil {
		return nil, nil
	}
	return &lo, &hi
}
