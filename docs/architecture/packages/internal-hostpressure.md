# internal/hostpressure

## Purpose

`internal/hostpressure` reads the load, the free memory and the used swap of the host. It also decides from them how many lanes the fleet may admit. It starts no goroutine and holds no clock of its own. `cmd/evolve` owns the sampling loop. The plan is [fleet-host-adaptive-window-2026-11](../../plans/fleet-host-adaptive-window-2026-11.md).

## Design

- **Parsers.** `parse.go` holds the text parsers that `cmd/hostsampler` used to hold: `ParseMeminfo`, `ParseVMStat` and `ParseSwapUsage`. The sampler now calls them. There is one copy. `ParseMemoryLevel` reads `kern.memorystatus_level`. Each parser fails with `ErrMissingField` or `ErrMacParse`, never with a zero.
- **`Reader`.** `Reader{ReadFile, Run, CPUs}` takes its three seams as function values, so a test needs no host. `Read` tries `/proc/loadavg` first. If that file exists, it reads `/proc/meminfo` too (Linux). If not, it takes the macOS path.
  - The macOS path runs `sysctl -n vm.loadavg` and `sysctl -n vm.swapusage`.
  - For the free memory it prefers `sysctl -n kern.memorystatus_level`, which gives a percent. If that call fails or gives junk, it runs `vm_stat` and `sysctl -n hw.memsize` and counts the used pages as active, wired and compressor pages.
  - A failed step returns an error. It never returns a zero reading, because a zero reads as no pressure.
- **`Decide`.** `Decide(state, reading, ok, config, now)` is a pure function. It returns a new `State` and a `Decision`. It never changes its input.
  - `Score` is the highest of the load for each core over `LoadHigh`, `MemMinPct` over the available memory, and the used swap over `SwapMaxMB`.
  - The decision uses the mean of the last `SmoothSamples` scores. A mean above 1 halves the cap (`DecreaseFactor`) down to `Floor`, but only after `Cooldown` since the last change. A mean under `LoadLow` over `LoadHigh` adds `Step` up to `Ceiling`, but only after `IncreaseCooldown`. Else the cap holds.
  - The increase cooldown is longer than the decrease cooldown. This stops a sawtooth.
  - A failed read (`ok` false) holds the cap and counts a blind streak. `Decision.Blind` is true once, when the streak reaches `SmoothSamples`.

## Invariants

- **A failed read never lowers or raises the cap.** Pinned by `TestDecide_FailedReadHoldsTheCapAndBlindFiresOnceAtTheStreak`.
- **The cap stays between `Floor` and `Ceiling`.** Pinned by `TestDecide_HighLoadHalvesTheCapDownToTheFloor` and `TestDecide_LowPressureRaisesOneStepPerIncreaseCooldown`.
- **Samples inside the cooldown change nothing.** Pinned by `TestDecide_AlternatingSamplesInsideTheCooldownCauseNoChange`.
- **`Decide` does not change its input.** Pinned by `TestDecide_DoesNotMutateItsInputState`.
- **Each reader path fails loudly.** Pinned by `TestReader_LinuxFailuresReturnAnError` and `TestReader_DarwinFailuresReturnAnError`.

## Findings

- On the macOS host of 2026-11-20, `sysctl -n kern.memorystatus_level` printed `93`. The fallback to `vm_stat` stays for a host where the call fails.
- The calibration data does not use the signal bus. `cmd/evolve` appends each reading and decision to `host-pressure.ndjson`. The bus carries only a change of the cap, so a wave publishes few events.
- The idle load of that host is 23.6 on 14 cores. The default thresholds of `fleet.orchestrator.window.adaptive` are a start. Set them from shadow data.
