# internal/events/filter

> The spec of record: [event-channels.md](../event-channels.md) §2 (the selectors) and §8 (the filter grammar). The plan (component E4, decision D29): [event-notification-protocol-2026-10.md](../../plans/event-notification-protocol-2026-10.md). The decision: [ADR-0127](../adr/0127-push-only-event-channels.md). The vocabulary: [signal-center-design.md](../signal-center-design.md).

## Purpose

`internal/events/filter` is the one filter grammar of the event channels. The channel routes, `--filter`, `--until` and the subscriptions use it (the Specification pattern). It has four parts:

- the grammar and the parser (`Parse`), which returns a `Filter`, the warnings (`[]Warning`) and an error;
- the matcher (`Filter.Match`) and the end test of a watch (`Filter.Until`), over a `Record`;
- the channel selectors (`ResolveChannels`) and the channel name check (`ValidChannelName`);
- the vocabulary (`Catalog`, `RegisteredCatalog`).

The package is a leaf. It imports only `internal/signalcenter`. Nothing in production calls it yet: component E10 wires it.

## Design

- **Terms are ANDed, values are ORed.** `Parse` splits the expression on white space. Each term is `KEY OP VALUE{,VALUE}`. With `=`, any value matches. With `!=`, no value matches. An empty expression gives a filter that selects every record.
- **One table of keys** (`keys`, data and not branches). Each key has a class (text, glob, number or severity), a reader and, for `kind`, `module` and `code`, a vocabulary rule. `fields.NAME` reads one entry of `signal.fields`.
- **Order operators.** `>=`, `>`, `<=` and `<` apply only to `severity` and to the number keys `cycle`, `attempt`, `pid` and `seq`. `severity` orders `INFO`, `WARN` and `INCIDENT` by `signalcenter.Severity.Level`. Numbers compare as numbers, so `cycle=01841` matches cycle 1841.
- **Absent keys.** The rules are in the spec, [event-channels.md](../event-channels.md) §8. Empty text values, a severity that is not a tier, a missing `fields.NAME` and a `cycle` or `attempt` of 0 are absent. `pid` and `seq` are always present. An absent key matches only `!=`.
- **Globs.** Only `kind` and `code` take `*`. A `*` matches any run of characters, also an empty run and a dot. A `*` on another key is a usage error.
- **Two error classes.** The CLI maps them to exit 10 and exit 1.
  - `ErrUsage` marks an expression that does not obey the grammar. Examples: an unknown key, a term without an operator, an empty value or a glob on another key.
  - `ErrUsage` also marks an order operator on a key without an order, or with more than one value. A number that does not parse and an unknown severity are usage errors too. The spec states these rules in §8.
  - `ErrRefused` marks a grammatical expression that names no registered value: an unknown kind or module, or a `kind` glob that matches no registered kind.
- **An unknown code only warns.** The code registry is open: a newer build can add a code that a stored filter names. `Parse` returns one `Warning{Key, Value}` for each `code` value that matches no registered code. The filter still matches that code. `Warning.String` gives the text that the CLI prints. The type is fixed now, so the CLI (E9) can depend on its fields.
- **Gap records.** A `Record` with no `Signal` is a gap record. It passes every filter, so a gap is never silent. `Until` is true only for a signal record that the filter selects, so a gap never ends a watch (D29). A synthetic `loop.lost` record is a signal record and can end a watch.
- **The catalog is injected.** `Parse` takes a `Catalog`. `RegisteredCatalog` reads `signalcenter.Kinds`, `signalcenter.Modules` and `signalcenter.RegisteredCodes` at the call. The codes are those that the linked packages registered, sorted. Tests give their own catalog, so they do not depend on the producers of components E11.
- **Channel selectors.** A selector is a dotted name. A token is `[a-z0-9_-]+`, `*` (one token) or `>` (one or more tokens, at the end only), as for NATS subjects.
  - `ResolveChannels` takes selectors that are already split. The caller splits the comma list of `--channel`.
  - It returns the selected channels in the order of the catalog, with no duplicate.
  - A malformed selector or an empty selection is `ErrUsage`. A selector that matches no channel is `ErrRefused`.
  - `ValidChannelName` checks a channel name with the same token rule, so the policy loader (E7) does not keep a second copy of it.

## Invariants

- **`!=` is the complement of `=` for present keys.** `TestMatch_NotEqualIsTheComplementOfEqualForPresentKeys` is a `rapid` property over every key, with globs, and with records where each key is present.
- **An absent key matches only `!=`.** `TestMatch_AnAbsentKeyMatchesOnlyNotEqual` covers each order operator on `cycle` and `attempt`, each text key, `code=*` and an empty severity. `TestMatch_AnInvalidSeverityIsAbsent` covers a severity that is not a tier.
- **A gap passes every filter and never satisfies `--until`.** `TestMatch_GapRecordsPassEveryFilter` and `TestUntil_AGapRecordNeverMatches`.
- **The vocabulary rules.** `TestParse_AKindGlobThatMatchesNoKindIsRefused`, `TestParse_AnUnknownKindOrModuleIsRefused`, `TestParse_AnUnknownCodeOnlyWarns`, `TestParse_AnUnknownCodeWarningNamesTheKeyAndTheValue` and `TestParse_AGlobOnAnotherKeyIsAUsageError`.
- **The catalog is the registries.** `TestRegisteredCatalog_IsTheSignalCenterVocabulary` registers probe codes for four modules, so the sort of the codes across modules is tested.

## Findings

- **Three mutants are equivalent.**
  - An empty `source` made present. A value is never empty, and `source` takes no glob, so `=` and `!=` give the same result.
  - A parser guard (`i > 0` against `i >= 0`). A term with no key is refused in both cases, with a different text.
  - The key of each warning set to `code`. Only `code` has the warning rule, so the key is always `code`.
