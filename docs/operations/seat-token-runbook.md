# Runbook: move the claude seats to a long-lived token

- **Type:** operations guide.
- **Owner:** the operator. Run each step in your own terminal.
- **Design:** [seat-login](../architecture/seat-login.md), [ADR-0163](../architecture/adr/0163-a-seat-holds-a-narrow-token-in-its-own-directory-and-the-token-file-is-the-stage.md).
- **Plan:** [seat-login-isolation-2026-11](../plans/seat-login-isolation-2026-11.md), components L2, L3 and L7.

## Rules

- Never paste the token into a chat, an agent session or a `!` command. A session records it.
- The pipeline never stores, prints, logs or copies the token. It reads the holder once for each launch, and it gives the value to the environment of a `claude` seat only.
- `<root>` is the root of the loop (the directory that has `.evolve`).

## Steps

1. Run `claude setup-token`. Complete the login in the browser. Copy the token. It starts with `sk-ant-oat01-`.
2. Write the token to a private temporary file. Nothing shows on the screen.
   ```
   umask 077; TMPF=$(mktemp); read -rs SEAT_TOK; printf %s "$SEAT_TOK" > "$TMPF"; unset SEAT_TOK
   ```
3. Prove one seat first (L7). The holder does not exist yet, so the fleet stays on the stored login.
   ```
   evolve agent run <profile> --seat-token-file "$TMPF" --input <file> --project-root <root>
   ```
   The profile is the first argument. `evolve agent list --project-root <root>` shows the names. Add `--json` and check that `seat_login` reads `isolated`.
4. Stop the loop lanes at a boundary. The verb in step 5 refuses while a lane is live.
   ```
   evolve loop-stop --wait --project-root <root>
   ```
5. Move the fleet. The token comes on stdin from the file, never from a terminal. A terminal on stdin is refused with exit 10.
   ```
   evolve auth seat-token set --project-root <root> < "$TMPF"
   rm -- "$TMPF"
   ```
   Exit 11 means the value is not a seat token. The holder stays as it was.
6. Check the result, then release the brake.
   ```
   evolve doctor seat
   evolve loop-stop --release --project-root <root>
   ```
   Expect an `OK` line that says the claude seats are isolated, and a holder age of 0 days.
7. Start the fleet as you do now.
8. Prove the isolation (L2). Let one wave run for more than 24 hours. Run `evolve auth store-times` before the start and after the wave. The times must not change because of a seat. No seat may show an auth error.

## Renewal

The token lasts one year. Before it ends, `evolve doctor seat` and the loop preflight warn. The age for the warning is the constant `seatlogin.TokenWarnAgeDays`. Repeat steps 1 to 6.

## Go back to the stored login

```
evolve loop-stop --wait --project-root <root>
evolve auth seat-token clear --project-root <root>
evolve loop-stop --release --project-root <root>
```

## Limits

- An agent in a seat can run `env` and print the token to its own transcript. The token can only make model requests, and you can revoke it.
- The age counts from the file time of the holder. A copied file can show a wrong age.
