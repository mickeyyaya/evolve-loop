# internal/phaseregistrar

## Purpose

`internal/phaseregistrar` turns a unified `phaseconfig.PhaseConfig` into a registered, validated phase runner. Advisor-minted phases pass through it. It validates the spec against the user-phase floor. It clamps the dispatch to the envelope and the allowed CLIs. It persists the spec and the dispatch profile, so the runner reads them from disk. It builds a spec-backed `core.PhaseRunner`.

## Design

- **Pure factory.** `Register` returns the runner and the normalized spec. The caller splices them into the runner map, the routing config and the phase catalog, because those live in different composition roots.
- **Normalize, then validate.** A minted phase is always optional, because it cannot satisfy the build, audit and ship floor. A source writer always runs sandboxed, with a writable repo. The code forces both rather than rejecting the config.
- **Metadata defaults at the seam.** A minted phase with neither a description nor a when-to-use gets a stub description and a stub when-to-use. The catalog test will otherwise fail once ship tracks the file. A phase whose persona does not resolve gets `catalog: "on-demand"`, so it stays off the menu. An advisor's own text, or an explicit catalog word, is kept as written.
- **Bare CLI names.** The registrar resolves a bare family name to its driver before it builds the profile. A name that is not a registered driver is rejected at mint time, not at the next launch.
- **Name guard.** The profile name derives from the advisor-set agent field. It must be lowercase kebab-case, because it becomes a file name and a lookup key.
- **Driverless guard.** When the registrar persists profiles, it rejects a profile with no dispatch CLI. With persistence off, the registrar is a pure factory, and checked-in phase files may omit the dispatch block.
- **Envelope clamp.** An unclassifiable tier, rank 0, is rejected when an envelope is declared. The pin validator will otherwise skip it.
- **Register before persist.** The minted name goes into the shared active-mints registry first. Each lane's tree-diff guard can then exempt the new files. If the registry write fails, nothing is persisted.
- **Persist.** The profile is written before the spec. Each write is skipped when its directory is empty.

## Invariants

- A minted phase is never a trust-kernel escape: it is optional, sandboxed when it writes source, and clamped to its envelope. Pinned by `TestRegister_UnclassifiableTierWithEnvelope_Rejected` and `TestRegister_CLIWithDriverSuffix_ClampedByBase`.
- The registry write precedes persistence. Pinned by `TestRegister_RegistryAppendFails_RejectsBeforePersist`.
- A persisted profile has a driver. Pinned by `TestRegister_DriverlessCLI_Rejected`, and the opt-out by `TestRegister_DriverlessCLI_AllowedWithoutPersistence`.
- An advisor's metadata is never overwritten. Pinned by `TestRegister_AdvisorMetadata_NotClobbered`.
- A minted stub stays catalog-green after the round trip through disk. Pinned by `TestRegister_UnknownPhaseNameStaysCatalogGreen`, `TestRegister_MetadataLessMint_DefaultsSelectMetadata` and the tests in `registrar_mint_ondemand_test.go`.
- A derived profile name is kebab-case. Pinned by `TestRegister_NonKebabDerivedProfileName_Rejected`.
