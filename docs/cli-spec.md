# lidl-cli spec

The interface contract of the `lidl` binary. Vocabulary in [`../AGENTS.md`](../AGENTS.md). Site facts come from read-only probes on 2026-11-06 and are recorded in [lidl-api-discovery.md](lidl-api-discovery.md); the site publishes no documented API and may change.

## 1. Name

Binary `lidl`. Module `github.com/n0nuser/lidl-cli`. Go 1.26, one static binary (`CGO_ENABLED=0`).

## 2. One-liner

Search lidl.es prices, offers and online availability from the terminal.

## 3. Usage

```
lidl [global flags] <noun> <verb> [args] [flags]
```

Noun-verb, except `search` (a verb on the catalogue) and `doctor`.

## 4. Command tree

```
search     [keywords...] | filters [keywords...]
product    show PRODUCT | stock PRODUCT      (alias: item)
category   list
watch      add (search|product) | list | remove | check | run | events | service (install|uninstall|status)
sink       list | test
mcp        (no verb: serves) | install claude-code|codex|cursor
config     path | list | get | set
doctor
skills     list | get
update
completion bash|zsh|fish|powershell
```

No account commands: the CLI is read-only and the site's public endpoints carry everything it reads. There is no login or session of any kind. See [ADR 0002](adr/0002-read-only.md).

### Arguments

- **PRODUCT**: the number at the end of a product URL (`/p/<slug>/p100405593`, 6 to 12 digits), a product URL, or a `/p/<slug>/p<number>` path. A bare number costs no extra request: the site redirects `/p/x/p<number>` to the product's real URL.

### search

`search [keywords...]` calls the site's search API, 48 products per page.

| flag | meaning |
|---|---|
| `--category LABEL` | the site's category facet value, for example `Bricolaje y jardín` (`search filters` lists them) |
| `--brand NAME` | repeatable; the Marca facet, case-insensitive |
| `--filter CODE=VALUE` | repeatable; any other facet from `search filters` |
| `--min-price`, `--max-price` | EUR; the site's range needs both ends, so a missing one becomes 0 or 100000 |
| `--min-rating N` | average rating from 1 to 5 |
| `--sort` | `relevance` (default), `price_asc`, `price_desc`, `newest`, `rating`, `bestselling`, `discount` |
| `--in-stock` | only products available online |
| `--page N`, `--pages N`, `--limit N` | start page, how many pages, stop after N products |

Keywords are optional when `--category`, `--brand` or `--filter` narrows the search; an empty query with none of them is a usage error. `--filter` refuses the codes that have their own flag (`price`, `brand`, `category`, `ratingAverage`, `availabilityIndicator`).

Output (`json`): `{query, page, pages, total, sort, products: [Product]}`. With `jsonl`, one `Product` per line. `Product`:

```json
{"ref": "100405593", "title": "…", "url": "https://www.lidl.es/p/…/p100405593", "image": "…", "brand": "PARKSIDE®",
 "category": "Categorías/Tienda de bricolaje/…",
 "price": {"amount": 36.99, "old_price": 40.99, "discount_percent": 9, "currency": "EUR", "tax": "21", "start": "…", "end": "…"},
 "rating": {"average": 4.7, "count": 259}, "stock": {"available": true, "label": "Disponible online"}, "badges": ["Top valorados"]}
```

`old_price` and `discount_percent` appear only for a marked-down product; `end` only when the promotion has a fixed end. `total` can differ by a couple of products between two identical requests: the site itself is not stable there.

`search filters` prints the facets (`code`, `label`, `type`, `values` with counts) for the same search.

### product

- `product show PRODUCT`: reads the product page's schema.org `Product` JSON-LD: title, brand, EAN, images, price, rating, availability, and the description turned into plain text. The page has no struck-through price; use `search` for `old_price` and the promotion dates.
- `product stock PRODUCT`: `{ref, title, url, stock: {available, label}}`. Lidl publishes availability per product for online orders. There is no per-store stock and no quantity ([ADR 0003](adr/0003-online-stock-only.md)).

### category

`category list [--match TEXT]` lists the `/c/<slug>/s<id>` pages from the pages sitemap: `{id, slug, name, url}`. Names are the slug made readable. The sitemap includes brand and editorial pages.

### watch

Watches, checks, events and sinks are local (`$XDG_DATA_HOME/lidl-cli/state.db`, SQLite).

- `watch add search [search flags] --name NAME [--notify SINK]... [--interval D] [--pages N] [--emit-initial]`
- `watch add product PRODUCT --name NAME [--notify SINK]... [--interval D] [--emit-initial]`

  `add` runs the first check at once as a silent baseline (reported as `product.new` with `--emit-initial`). A target that cannot be read is not saved.
- `watch check [NAME...] [--all]`: due watches (or the named ones, or all) once. Events are printed and delivered to the watch's sinks, then committed: a crash between the two repeats events, never loses them. Exit 0 when nothing changed. A block (exit 5) stops the run.
- `watch run [--interval D]`: the same in a loop, with up to 10% jitter.
- `watch events [NAME] [--since D] [--limit N]`, `watch list`, `watch remove NAME [--yes]`.
- `watch service install [--interval D] | uninstall | status`: a systemd user timer (Linux) or launchd agent (macOS) running `lidl watch check --format jsonl`, output appended to `$XDG_STATE_HOME/lidl-cli/lidl-watch.log` (rotated at 1 MiB). The units pin `HOME`, the XDG variables and `LIDL_CONFIG` as they were at install. Windows gets the `schtasks` line to run.

Interval: default 30m, floor 5m.

Events:

| type | when | `change` |
|---|---|---|
| `product.new` | a search lists a product it had not listed | |
| `product.price_changed` | price differs from the last check | `{from, to}` |
| `product.out_of_stock` | it was available online, now is not | |
| `product.back_in_stock` | it was not available online, now is | |
| `product.removed` | product watches: the page is gone (404) | |

Lidl's offers are short, so a search watch sorted by `newest` or `discount` is the way to hear about a new offer. A product leaving a search is not reported (offers expire); a product watch reports `product.removed` when its page 404s.
Event: `{type, watch, at, product: Product, change}`.

### sink

Declared in config.toml:

```toml
[sinks.phone]
type = "ntfy"            # url, topic, token_file (optional)
url = "https://ntfy.sh"
topic = "my-lidl"

[sinks.chat]
type = "webhook"         # url; template = "discord" | "slack" sends text, otherwise the event JSON
url = "https://discord.com/api/webhooks/..."
template = "discord"

[sinks.script]
type = "exec"            # command: event JSON on stdin, LIDL_EVENT_TYPE and LIDL_WATCH set
command = ["/home/me/bin/on-lidl-event"]
```

`sink list` shows each and what is wrong with it; `sink test NAME` sends a made-up event.

### mcp

`lidl mcp` serves MCP over stdio (JSON-RPC 2.0, one message per line; protocol revisions 2024-11-05, 2025-03-26, 2025-06-18). Tools are command lines run in-process, so a tool returns exactly what the command prints; a failure returns the `--error-format json` envelope with `isError: true`. Tools: `search`, `product_show`, `product_stock`, `category_list`, `watch_check`. Unknown arguments are refused (JSON-RPC -32602). `mcp install` writes `command` and `args` for this binary into the harness config, keeps every other entry, and reports `added`, `updated` or `unchanged`.

### config / doctor / skills / update

- `config path | list | get KEY | set KEY VALUE` on config.toml. Keys: `watch.interval`, `sinks.NAME.*`. Values are checked by decoding the whole file strictly before it is written.
- `doctor`: config parses, sinks are valid, a probe search answers and parses, the state database opens, the schedule state. Exit 1 if anything fails.
- `skills list | get NAME`: embedded usage notes for agents.
- `update [--check] [--yes]`: replaces the binary with the latest GitHub release after checking it against `checksums.txt`. Development builds are not replaced. Interactive commands check at most once a day and ask; scripts, `mcp` and `watch` never do; `LIDL_NO_UPDATE_CHECK=1` turns it off.

## 5. Global flags

| flag | default | meaning |
|---|---|---|
| `--format json\|jsonl\|pretty\|toon` | `json` | stdout rendering |
| `--no-color` | off | also `NO_COLOR`, `TERM=dumb`, non-TTY |
| `--error-format text\|json` | `text` | stderr error rendering (also `LIDL_ERROR_FORMAT`) |
| `--no-input` | off | never prompt |
| `--debug` | off | one stderr line per request and response |
| `-h, --help`, `-v, --version` | | |

No `--profile`: without accounts or stores there is nothing to switch between.

## 6. I/O contract

- stdout carries data only: one JSON document, or one object per line with `jsonl`.
- stderr carries errors, notices and prompts. Text from the site is stripped of control characters before it reaches a terminal.
- Prompts only when stdin, stdout and stderr are terminals and `--no-input` is absent.

## 7. Exit codes

| code | meaning |
|---|---|
| 0 | success, including "nothing changed" |
| 1 | generic failure |
| 2 | usage: bad flags or arguments, a search with nothing to search for |
| 3 | unused (the CLI needs no account); kept so the other numbers stay put |
| 4 | not found: product, watch |
| 5 | refused: HTTP 403 or 429 from the site |
| 6 | network, timeout, or the site down (5xx) |
| 7 | the site changed: a known page or endpoint no longer has the expected shape |
| 130 | interrupted |

## 8. Errors

Text errors are one line, `lidl: ` prefixed. `--error-format json`: `{code, category, retryable, message, http_status, endpoint, suggested_commands, issue_url}`. Exit 7 carries `issue_url`, a prefilled `api-change.yml` issue with CLI version, OS/arch, command path, endpoint template, HTTP status and the first 300 bytes of the response; never the user's arguments.

## 9. Transport

Requests go to `https://www.lidl.es` (override with `LIDL_BASE_URL` for tests) over Go's standard HTTP client. The `User-Agent` is `lidl-cli/<version> (+https://github.com/n0nuser/lidl-cli)`. The search API takes `Accept: */*`: it answers `application/json` with a 401 (regression test `TestSearchAcceptHeader`). No cookies are sent or kept. Requests from one process are spaced at least 500 ms apart.

Transient failures get a bounded retry, never more than twice: a 5xx or a transport/timeout error is retried after 500 ms, then 1 s (doubling, capped at 5 s), and a 429 whose `Retry-After` is at most 2 minutes is retried once after that wait. A 403, a shape change and a usage error are never retried. A 403 or a surfaced 429 is exit 5.

## 10. Files and environment

| what | path | mode |
|---|---|---|
| config | `$XDG_CONFIG_HOME/lidl-cli/config.toml` (or `LIDL_CONFIG`) | 0600 |
| watch state | `$XDG_DATA_HOME/lidl-cli/state.db` | 0600 |
| update stamp | `$XDG_CACHE_HOME/lidl-cli/` | 0600 |
| scheduler log | `$XDG_STATE_HOME/lidl-cli/lidl-watch.log` | |

Writes are atomic (temp file and rename). Env: `LIDL_CONFIG`, `LIDL_ERROR_FORMAT`, `LIDL_NO_UPDATE_CHECK`, `LIDL_BASE_URL`, `NO_COLOR`. Precedence: flag > env > config.

## 11. Stack

cobra + pflag, `net/http`, `pelletier/go-toml/v2`, `adrg/xdg`, `modernc.org/sqlite`, `toon-go`. No MCP SDK, no HTML parser: the search API is JSON, and the product page is read through its JSON-LD block. Tests use stdlib `testing` and `httptest` serving captured responses from `internal/api/testdata/`, plus a binary-level test with an isolated XDG home. No test calls lidl.es.

## 12. Layout

```
cmd/lidl/          entrypoint; under cmd/ so `go install .../cmd/lidl` names the binary lidl
cmd/docsgen/       writes docs-site/content/docs/reference from the cobra tree
internal/api/      everything that knows lidl.es: client, search, product, categories
internal/cli/      one file per noun; skills/ holds the embedded notes
internal/state/    SQLite watch state
internal/watch/    one check: fetch, diff, events
internal/sink/     ntfy, webhook, exec
internal/mcp/      MCP server, tool table, harness install
internal/output/   json/jsonl/pretty/toon, error envelope, exit codes
internal/config/   paths, config.toml, atomic writes
internal/updater/  release check and checksum-verified replace
```

## 13. Not in v1

Cart, checkout, orders, account (read-only by design). Per-store stock and store lookup: no endpoint found ([ADR 0003](adr/0003-online-stock-only.md)). Search autocomplete (the site has a suggest API). The spec table of a product page. Homebrew, Scoop and AUR packages.
