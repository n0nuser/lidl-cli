# lidl-cli

Read-only terminal client for lidl.es, the Spanish Lidl web shop: search the catalogue, see a product's price, offer and online availability, and get told when either changes.

## Working on it

- `go build ./... && go test ./... && go vet ./...` must stay clean; `make check` adds staticcheck and gofmt.
- After changing a command's help or flags, run `make docs`; CI fails on a stale reference.
- Everything that knows the site lives in `internal/api`. A site change is fixed there, with a new capture in `internal/api/testdata/` and a test that pins it.
- Every bug fix gets a regression test whose comment says what broke.
- Tests never call lidl.es. Live checks are manual: low rate, a few requests, the tool's own User-Agent.
- Spec: `docs/cli-spec.md`. Site notes: `docs/lidl-api-discovery.md`. Decisions: `docs/adr/`.
- Layering: `cli` orchestrates; `api` is network and parsing; `config` is disk; `state` is the local sqlite; `watch` diffs two checks; `sink` delivers events. `api` never touches the disk and `config` never touches the network.

## Distribution

- Releases are goreleaser archives named `lidl_<version>_<os>_<arch>.tar.gz` (`.zip` on Windows),
  with `checksums.txt`. `scripts/install.sh` is the `curl | sh` installer; it downloads, verifies the
  checksum, and only then installs.
- `npm/` is the `npx` wrapper. Publishing runs in `.github/workflows/npm-publish.yml` on a `v*` tag
  with npm Trusted Publishing (OIDC): no npm token exists in the repo, and provenance is automatic. The
  workflow overwrites the package version from the tag. One npm-side step is left: on npmjs.com, open the
  package → Settings → Trusted Publishers → add a GitHub Actions publisher with owner `n0nuser`,
  repository `lidl-cli`, workflow `npm-publish.yml`, environment empty. Until then that workflow fails
  on its own; it never touches `release.yml`.
- Homebrew, Scoop and AUR are written but commented out in `.goreleaser.yaml`, so `release.yml` stays
  green. To enable them: **homebrew** needs a `homebrew-tap` repository and a `HOMEBREW_TAP_GITHUB_TOKEN`
  secret with write access; **scoop** needs a `scoop-bucket` repository and a `SCOOP_BUCKET_GITHUB_TOKEN`
  secret with write access; **aur** needs an AUR account and an `AUR_SSH_PRIVATE_KEY` secret whose public
  key is registered on it.

## Language

**Product**:
One article the site sells, identified by its number (the digits after `p` in `/p/<slug>/p100405593`).
_Avoid_: item, SKU, article

**Listing**:
One page of search results (48 products), in the site's ranking order.

**Facet**:
A filter the site offers on a search (brand, price range, category, rating, availability...), addressed by its code. The CLI has flags for the common ones and `--filter CODE=VALUE` for the rest.

**Offer**:
A product with a limited-time price: `price.old_price` is the struck-through price and `price.start` / `price.end` bound the promotion. Offers come and go.
_Avoid_: deal, promo (in output names)

**Availability**:
Whether the product can be bought online, as the site reports it (`stock.available`). There is no quantity and no per-store stock.
_Avoid_: inventory, stock level

**Category**:
A `/c/<slug>/s<id>` page. For searching, a category is the site's category facet value (a label such as "Bricolaje y jardín").

### Tracking

**Watch**:
A saved instruction to check a search or a product and report changes. Local only.

**Check**:
One run of a watch: fetch, compare with the last stored state, emit events, store the new state.

**Event**:
One observed change: new product, price changed, out of stock, back in stock, removed.

**Sink**:
Where events go besides stdout: ntfy, a webhook, or a program.
