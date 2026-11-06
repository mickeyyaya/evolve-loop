# The site check

This check loads each page of the built site at a desktop width and a phone width. It finds console errors, broken images, sideways overflow, stale claims and WCAG A and AA faults. It also plays each demo, and it compares the page with `shared/content.json`.

```bash
cd landing && go run ./cmd/build
printf 'check-%s\n' "$(date +%s)" > dist/build-marker.txt
(cd dist && python3 -m http.server 8391 --bind 127.0.0.1) &
cd e2e && npm install && npx playwright install chromium
SITE_BASE=http://127.0.0.1:8391 EXPECT_MARKER="$(cat ../dist/build-marker.txt)" node verify.mjs ../.. site-check-result.json
```

The check exits 1 when a check fails. It also fails when `EXPECT_MARKER` differs from the marker that the server gives. A server from an earlier run can hold a port, and a check of that server proves nothing.

To check the live site, set `SITE_BASE` to the site URL and leave out `EXPECT_MARKER`.

An automated check finds many faults, and it cannot replace a person with a screen reader. It does not prove that each sentence is true. The review step in `skills/publish/SKILL.md` covers that.
