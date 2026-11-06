# go/internal/seatlogin

Decision: [ADR-0163](../adr/0163-a-seat-holds-a-narrow-token-in-its-own-directory-and-the-token-file-is-the-stage.md). Spec: [seat-login](../seat-login.md).

## Purpose

The package decides how a claude seat logs in. `Resolve(home, param, deps)` reads one token file once, and it returns a `Decision`. When `param` names a file (`evolve agent run --seat-token-file`), that file wins and the holder is not read. `ParseToken` is the one rule of a seat token: it starts with `sk-ant-oat01-`, holds no whitespace or control byte, and has at most 4096 bytes.

| Holder file | Decision |
|---|---|
| Absent | The seat uses the stored login, as before. |
| Valid, mode 0600 | The host is isolated, and the seat gets the token. |
| Empty, wrong mode, a symlink or other non-regular file, not a seat token, or unreadable | A refusal with a named text. It never falls back to the stored login. |

The bridge calls `Resolve` at two doors: `launchSwept` and `launchPane` for a smoke. Only a manifest with `seat_login: isolated` acts on a refusal or joins the token. The launch records `seat_login` as `isolated` or `stored`. A refusal ends the launch with the exit class `seat_token_refused` (exit 88).

`Deps` holds the file reads, so that a test injects a failure. No text and no error holds the value of a token.

## Inspect

`Inspect(home, deps)` names the state of the host for `evolve doctor seat` and for the `seat-login` check of the loop preflight. A bad holder is a halt in the preflight. A seat directory that holds `settings.json` but no holder is a warning, because only an isolated launch writes that file. A seat directory with no `settings.json` reads as no token. A seat directory that is not a directory is a warning. The verb `seat-token clear` removes `settings.json`, so that a cleared host reads as no token and no message holds a token.
