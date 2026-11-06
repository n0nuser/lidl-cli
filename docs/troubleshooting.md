# Troubleshooting

lidl.es needs no login and showed no bot protection when this was written, so most problems are "the site changed" or "the address is rate limited". One command answers almost every one of them.

## Start here

```bash
lidl doctor --format pretty
```

`doctor` checks, without changing anything: the config parses, the sinks are valid, one probe search answers **and parses**, the watch database opens, and the timer is installed. A `FAIL` line names the piece and the reason; when the site is the problem it is the `search` line that fails.

## "lidl.es refused the request" (HTTP 403 or 429)

Exit 5. A 429 with a short `Retry-After` is retried once on its own; a longer one is reported with the wait. Wait a few minutes and run the command once more. Do not loop: lower `watch` frequency instead (the floor is 5 minutes, the default 30).

## "does not understand" (exit 7)

The search API or a product page no longer has the shape the CLI expects. The message ends with a prefilled issue link; open it. Add `--debug` to see the requests.

One known trap: the search API answers `Accept: application/json` with HTTP 401. The CLI sends `Accept: */*`; if you replay a request by hand with curl and set the header, you get the 401 too.

## The total changes between runs

`total` can be 137 on one request and 139 on the next. The site does that on its own; treat it as approximate.

## `product show` has no old price

The product page does not carry it. `lidl search "<title>"` (or the category) returns it in `price.old_price`, with `price.start` and `price.end`.

## A product I know exists is missing

Offers expire and some items are not sold online. `lidl product show NUMBER` answers 404 (exit 4) once a page is gone. `lidl search ... --in-stock` hides everything not orderable online.

## `--category` finds nothing

`--category` takes the site's facet *label* ("Bricolaje y jardín"), not the id from `category list`. `lidl search filters <keywords> --format pretty` lists the labels with counts.

## The watch timer does not run

```bash
lidl watch service status
loginctl enable-linger     # systemd user timers stop at logout otherwise
```

Logs are in `~/.local/state/lidl-cli/lidl-watch.log`.
