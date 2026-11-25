// Package diskfloor is the single policy resolver for the host free-disk floor and warn level.
package diskfloor

import (
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
)

const (
	BytesPerGB                 = uint64(1_000_000_000)
	BytesPerGiB                = uint64(1 << 30)
	DefaultGCFloorBytes        = 10 * BytesPerGB
	DefaultGCWarnBytes         = 20 * BytesPerGB
	DefaultPreflightFloorBytes = BytesPerGiB

	KeyFloorGB                = "disk.floor_gb"
	KeyWarnGB                 = "disk.warn_gb"
	OldKeyGCFloorGB           = "gc.disk_floor_gb"
	OldKeyGCWarnGB            = "gc.disk_warn_gb"
	OldKeyPreflightMinFreeGiB = "preflight.min_free_gib"
)

var ErrConflict = errors.New("conflicting disk floor policy keys")

type Policy struct {
	FloorGB float64 `json:"floor_gb,omitempty"`
	WarnGB  float64 `json:"warn_gb,omitempty"`
}

type Resolution struct {
	FloorBytes uint64
	WarnBytes  uint64
	FloorSet   bool
	WarnSet    bool
	Warnings   []string
}

type sourceValue struct {
	key   string
	bytes uint64
}

func Resolve(disk *Policy, legacyGC any, legacyPreflight any, defaultFloorBytes uint64, warn io.Writer) (Resolution, error) {
	var newFloor, newWarn float64
	if disk != nil {
		newFloor, newWarn = disk.FloorGB, disk.WarnGB
	}
	gcFloor := readLegacyFloat(legacyGC, "DiskFloorGB")
	gcWarn := readLegacyFloat(legacyGC, "DiskWarnGB")
	preflightGiB := readLegacyFloat(legacyPreflight, "MinFreeGiB")

	floors := collectSources([]sourceValue{
		{KeyFloorGB, scaleBytes(newFloor, BytesPerGB)},
		{OldKeyGCFloorGB, scaleBytes(gcFloor, BytesPerGB)},
		{OldKeyPreflightMinFreeGiB, scaleBytes(preflightGiB, BytesPerGiB)},
	})
	if err := checkConflict(floors, KeyFloorGB); err != nil {
		return Resolution{}, err
	}
	warns := collectSources([]sourceValue{
		{KeyWarnGB, scaleBytes(newWarn, BytesPerGB)},
		{OldKeyGCWarnGB, scaleBytes(gcWarn, BytesPerGB)},
	})
	if err := checkConflict(warns, KeyWarnGB); err != nil {
		return Resolution{}, err
	}
	return buildResolution(floors, warns, gcFloor, gcWarn, preflightGiB, defaultFloorBytes, warn), nil
}

func PreflightBytes(cfg any) uint64 {
	if gib := readLegacyFloat(cfg, "MinFreeGiB"); gib > 0 {
		return scaleBytes(gib, BytesPerGiB)
	}
	return DefaultPreflightFloorBytes
}

func ApplyToPreflight(disk *Policy, legacyGC any, legacyPreflight any, dst any, warn io.Writer) (uint64, error) {
	res, err := Resolve(disk, legacyGC, legacyPreflight, DefaultPreflightFloorBytes, warn)
	if err != nil {
		return 0, err
	}
	gib := float64(res.FloorBytes) / float64(BytesPerGiB)
	if raw := readLegacyFloat(legacyPreflight, "MinFreeGiB"); raw > 0 && (disk == nil || disk.FloorGB <= 0) && readLegacyFloat(legacyGC, "DiskFloorGB") <= 0 {
		gib = raw
	}
	setLegacyFloat(dst, "MinFreeGiB", gib)
	return res.FloorBytes, nil
}

func buildResolution(floors, warns []sourceValue, gcFloor, gcWarn, preflightGiB float64, defaultFloorBytes uint64, warn io.Writer) Resolution {
	res := Resolution{FloorBytes: defaultFloorBytes}
	if res.FloorBytes == 0 {
		res.FloorBytes = DefaultGCFloorBytes
	}
	if len(floors) > 0 {
		res.FloorBytes = floors[0].bytes
		res.FloorSet = true
	}
	res.WarnBytes = 2 * res.FloorBytes
	if len(warns) > 0 {
		res.WarnBytes = max(res.FloorBytes, warns[0].bytes)
		res.WarnSet = true
	}
	res.Warnings = deprecationWarnings(gcFloor, gcWarn, preflightGiB, warn)
	return res
}

func deprecationWarnings(gcFloor, gcWarn, preflightGiB float64, warn io.Writer) []string {
	var out []string
	for _, item := range []struct {
		set    bool
		oldKey string
		newKey string
	}{
		{gcFloor > 0, OldKeyGCFloorGB, KeyFloorGB},
		{gcWarn > 0, OldKeyGCWarnGB, KeyWarnGB},
		{preflightGiB > 0, OldKeyPreflightMinFreeGiB, KeyFloorGB},
	} {
		if !item.set {
			continue
		}
		msg := fmt.Sprintf("policy key %s is deprecated; use %s", item.oldKey, item.newKey)
		out = append(out, msg)
		if warn != nil {
			fmt.Fprintf(warn, "WARN: %s\n", msg)
		}
	}
	return out
}

func collectSources(candidates []sourceValue) []sourceValue {
	var out []sourceValue
	for _, c := range candidates {
		if c.bytes > 0 {
			out = append(out, c)
		}
	}
	return out
}

func checkConflict(sources []sourceValue, canonical string) error {
	if len(sources) < 2 {
		return nil
	}
	first := sources[0]
	for _, s := range sources[1:] {
		if s.bytes != first.bytes {
			return fmt.Errorf("%w: %s (%d bytes) conflicts with %s (%d bytes); set only %s", ErrConflict, first.key, first.bytes, s.key, s.bytes, canonical)
		}
	}
	return nil
}

func scaleBytes(v float64, unit uint64) uint64 {
	if v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	scaled := math.Round(v * float64(unit))
	if scaled >= float64(math.MaxUint64) {
		return math.MaxUint64
	}
	return uint64(scaled)
}

func readLegacyFloat(v any, field string) float64 {
	if v == nil {
		return 0
	}
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return 0
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return 0
	}
	f := rv.FieldByName(field)
	if !f.IsValid() {
		return 0
	}
	switch f.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(f.Int())
	case reflect.Float32, reflect.Float64:
		return f.Float()
	}
	return 0
}

func setLegacyFloat(dst any, field string, val float64) {
	if dst == nil {
		return
	}
	rv := reflect.ValueOf(dst)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return
	}
	f := rv.FieldByName(field)
	if f.IsValid() && f.CanSet() && (f.Kind() == reflect.Float32 || f.Kind() == reflect.Float64) {
		f.SetFloat(val)
	}
}
