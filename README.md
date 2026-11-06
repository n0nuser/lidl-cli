<p align="center">
  <img src=".github/assets/lidl-cli-logo.svg" width="640" alt="lidl cli logo">
</p>

<p align="center">
  <a href="https://github.com/n0nuser/lidl-cli/releases"><img src="https://img.shields.io/github/v/release/n0nuser/lidl-cli?display_name=tag&style=flat-square&label=release&color=000000" alt="release badge"></a>
  <a href="https://github.com/n0nuser/lidl-cli/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/n0nuser/lidl-cli/ci.yml?branch=main&style=flat-square&label=ci&color=000000" alt="ci badge"></a>
  <a href="https://pkg.go.dev/github.com/n0nuser/lidl-cli"><img src="https://img.shields.io/badge/go-reference-000000?style=flat-square" alt="go reference badge"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-mit-000000?style=flat-square" alt="license badge"></a>
</p>

---

`lidl` is an unofficial terminal cli for lidl.es. it searches the catalogue with the same filters the site has, shows what a product costs, what it cost before and when the offer ends, whether it can still be ordered online, and watches products and searches for price and availability changes. it is read-only: no account, no cart, no orders, and no login or browser session is needed. it compiles to one static Go binary. output is json by default so everything pipes into `jq`; `--format pretty` is for humans.

[documentation](https://n0nuser.github.io/lidl-cli/) | [command reference](https://n0nuser.github.io/lidl-cli/docs/reference/) | [github](https://github.com/n0nuser/lidl-cli)

```console
$ lidl search taladro --brand parkside --sort discount --limit 4 --format pretty
REF        PRICE    WAS      RATING     ONLINE  PRODUCT                                               URL
100407643  4.49 €   5.49 €   4.9 (45)   yes     PARKSIDE® Juego de brocas helicoidales HSS / brocas…  https://www.lidl.es/p/parkside-juego-d…
100410076  32.99 €  35.99 €  4.3 (11)   yes     PARKSIDE® Martillo perforador y cincelador »PBH 800…  https://www.lidl.es/p/parkside-martill…
100405608  37.99 €  40.99 €  4.8 (158)  yes     PARKSIDE® Taladro atornillador recargable de 12 V »…  https://www.lidl.es/p/parkside-taladro…
100400824  18.99 €  19.99 €  4.7 (84)   yes     PARKSIDE® Tornillo de banco paralelo                  https://www.lidl.es/p/parkside-tornill…
4 of 89 products, page 1/2
```

## for ai agents

two ways to wire `lidl` into an agent, both copy-paste:

```bash
# register the read commands as an MCP server (claude-code, codex or cursor)
lidl mcp install claude-code

# or read the embedded usage notes into the context
lidl skills get lidl-usage

# confirm the wiring with one live search
lidl search atornillador --limit 3 --format pretty
```

the mcp server exposes `search`, `product_show`, `product_stock`, `category_list` and `watch_check`; each tool returns exactly what the matching command prints, json by default. `examples/` has more recipes.

## install

### one line

```sh
curl -fsSL https://raw.githubusercontent.com/n0nuser/lidl-cli/main/scripts/install.sh | sh
```

installs the latest release into `~/.local/bin` (`$PREFIX/bin` when `PREFIX` is set, or `$LIDL_INSTALL_DIR`). the script picks the archive for your OS and architecture, verifies it against the release's `checksums.txt`, and only then installs it. pin a version with `VERSION`:

```sh
curl -fsSL https://raw.githubusercontent.com/n0nuser/lidl-cli/main/scripts/install.sh | VERSION=v1.2.3 sh
```

### go install

```bash
go install github.com/n0nuser/lidl-cli/cmd/lidl@latest
lidl --version
```

### npx

```bash
npx lidl-cli --version
npx lidl-cli search atornillador --format pretty
```

the npm wrapper downloads the matching release binary during install. publishing runs from the `npm-publish.yml` workflow on a `v*` tag via npm Trusted Publishing (OIDC: no token stored anywhere, provenance automatic). one npm-side step remains: on npmjs.com, once the package exists, open it → Settings → Trusted Publishers → add a GitHub Actions publisher with owner `n0nuser`, repository `lidl-cli`, workflow `npm-publish.yml`, and an empty environment. until that exists the workflow fails, which is separate from the binary release.

### release binaries

download the archive for your platform from [releases](https://github.com/n0nuser/lidl-cli/releases) (linux, macos and windows, amd64 and arm64), unpack it and put `lidl` on your `PATH`. every release ships `checksums.txt` and a github provenance attestation; download both files first, then verify:

```bash
gh release download --repo n0nuser/lidl-cli --pattern 'lidl_*_linux_amd64.tar.gz' --pattern checksums.txt
sha256sum --ignore-missing -c checksums.txt
gh attestation verify checksums.txt --repo n0nuser/lidl-cli
```

### from source

```bash
git clone https://github.com/n0nuser/lidl-cli && cd lidl-cli
make build     # bin/lidl
```

homebrew, scoop and aur are written but commented out in `.goreleaser.yaml`; they stay disabled until the account side exists, because a block whose repository or secret is missing breaks the release. their exact prerequisites:

- **homebrew**: a `homebrew-tap` repository, plus a `HOMEBREW_TAP_GITHUB_TOKEN` secret with write access to it.
- **scoop**: a `scoop-bucket` repository, plus a `SCOOP_BUCKET_GITHUB_TOKEN` secret with write access to it.
- **aur**: an AUR account, plus an `AUR_SSH_PRIVATE_KEY` secret whose public key is registered on it.

## why

lidl sells a rotating range: products come and go, and a markdown ends on a set date. the site answers "what is on offer" one page at a time. this cli lists a whole search or category, sorted by discount or by newest, with the old price and the end date in the data, and keeps answering on a timer so you hear when something you want drops or comes back. json output means scripts, agents and shell pipelines all read the same thing.

## quickstart

there is nothing to configure: no account, no login, no store to pick.

```bash
lidl doctor --format pretty
lidl search atornillador --format pretty
```

`doctor` checks that the config parses, sinks are valid, a test search answers and parses, the watch database opens, and whether the watch timer is installed.

stuck? [troubleshooting](docs/troubleshooting.md) starts with `lidl doctor`.

## updates

```bash
lidl update --check
lidl update --yes
```

interactive commands check for a new release at most once a day and ask before updating. scripts, `mcp`, `watch` and `--no-input` never check. `LIDL_NO_UPDATE_CHECK=1` turns it off. the download is verified against the release's `checksums.txt` before it replaces the binary. builds from source are not replaced.

## command surface

| command | what it does |
|---|---|
| `search [keywords] / search filters` | search with brand, price range, rating, category, sort and any facet; `filters` lists the facets |
| `product show` | price, rating, ean, images and description of one product |
| `product stock` | whether it can be ordered online (lidl publishes no per-store stock) |
| `category list` | the `/c/<slug>/s<id>` pages from the sitemap |
| `watch add search\|product` | watch a search or a product |
| `watch check / run / events / list / remove` | one-shot check, foreground loop, history |
| `watch service install / uninstall / status` | systemd user timer or launchd agent running `watch check` |
| `sink list / test` | notification targets from config |
| `mcp / mcp install` | serve the read commands to an agent harness |
| `config path / list / get / set` | config.toml |
| `doctor` | checks config, sinks, the site, the watch state |
| `skills list / get` | embedded usage notes for agents |
| `update` | replace the binary with the latest release |
| `completion bash\|zsh\|fish\|powershell` | shell completion |

global flags: `--format json|jsonl|pretty|toon`, `--no-color`, `--error-format text|json`, `--no-input`, `--debug`.

## examples

```bash
lidl search atornillador --format pretty
lidl search atornillador --max-price 40 --sort price_asc --in-stock
lidl search taladro --brand parkside --min-rating 4 --pages 2
lidl search --category "Bricolaje y jardín" --sort discount --limit 10
lidl search filters atornillador --format pretty
lidl product show 100405593 --format pretty
lidl product stock 100405593

lidl search --category "Bricolaje y jardín" --sort discount | jq -r '.products[] | "\(.price.discount_percent)%\t\(.price.end // "-")\t\(.title)"'
```

## watches

a watch saves a search or a product, and each check reports what changed: `product.new`, `product.price_changed`, `product.out_of_stock`, `product.back_in_stock`, `product.removed`.

```bash
lidl watch add product 100405593 --name taladro --notify phone
lidl watch add search atornillador --max-price 40 --sort newest --name atornillador
lidl watch check --format jsonl | jq -r 'select(.type=="product.back_in_stock") | .product.url'
lidl watch service install --interval 1h
```

## sinks config

```toml
[sinks.phone]
type = "ntfy"
url = "https://ntfy.sh"
topic = "my-lidl-alerts"

[sinks.chat]
type = "webhook"
url = "https://discord.com/api/webhooks/..."
template = "discord"

[sinks.script]
type = "exec"
command = ["~/bin/on-lidl-event"]   # event json on stdin
```

## exit codes

`0` ok, `1` generic, `2` usage, `4` not found, `5` refused or rate limited by the site, `6` network or site down, `7` the site changed shape (stderr carries a prefilled issue link), `130` interrupted. `3` is unused.

## development

after `make build` (see [install](#install)):

```bash
make check     # vet, staticcheck, tests, gofmt
```

`go test` never touches the network: tests serve captures from `internal/api/testdata/`. to check the real site by hand, run a few commands at a low rate, for example `lidl search atornillador --limit 3`.

## documentation

the docs site source is in `docs-site` (fumadocs, exported as static pages); its command reference is generated from the binary with `make docs`.

- [docs/cli-spec.md](docs/cli-spec.md): the interface contract
- [docs/lidl-api-discovery.md](docs/lidl-api-discovery.md): how the site works, endpoint by endpoint
- [docs/troubleshooting.md](docs/troubleshooting.md): the common failures, starting with `doctor`
- [examples/](examples/): ready-to-run search, offer and watch recipes
- [AGENTS.md](AGENTS.md): vocabulary and working rules
- [docs/adr](docs/adr): decisions

## sibling projects

same author, same shape, other shops: [obramat-cli](https://github.com/n0nuser/obramat-cli) and [leroymerlin-cli](https://github.com/n0nuser/leroymerlin-cli).

## contributing

issues and pull requests are welcome. when the site changes, the cli exits `7` and prints a prefilled issue link; that is the most useful report you can file.

## disclaimer

this is an unofficial client, not affiliated with, endorsed by or connected to lidl or its parent company. "lidl" is a trademark of its owner and is used here only to say which site the tool works with. the site publishes no documented api; the cli reads the same public endpoints a browser does, for interoperability. it is read-only, asks for no password and stores no credentials. use it responsibly and keep the request rate low: no bulk scraping, keep watch intervals reasonable. prices and availability shown are what the site showed at that moment; check the site before you buy.

## license

[mit license](LICENSE)
