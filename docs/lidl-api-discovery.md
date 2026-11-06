# lidl.es: how the site works, as far as the CLI needs it

Recorded 2026-11-06 by reading pages and live responses with plain `curl`, no cookies, no login. Nothing here is documented by the site; every claim has a captured response in `internal/api/testdata/` and a test.

## 1. Hosts and protection

| host | role |
|---|---|
| `www.lidl.es` | pages, the search API, sitemaps |
| `www.lidl.es/assets/…` | images |

The site is a Nuxt app. Its page config names the endpoints it uses: `searchPath: /q/api/search`, `gridboxesPath: /q/api/gridboxes`, `productScriptURL: /p/fragment/ProductGridbox.html`, and a suggest API. No bot wall, captcha or challenge was seen. A request with a tool User-Agent (`lidl-cli/…`) and no cookies works.

Two behaviours to know:

- `GET /q/api/search` with `Accept: application/json` answers **401**. `Accept: */*` (curl's default) answers 200. The CLI sends `*/*`.
- `robots.txt` disallows `*search?q=*`, `*?offset=*` and `*sort=*` on the site's pages. The API path `/q/api/search` is not in the list. The CLI keeps to a few requests, 500 ms apart, and has no bulk mode.

## 2. Search API

```
GET /q/api/search?q=<q>&offset=<n>&fetchsize=48&locale=es_ES&assortment=ES&version=2.1.0
```

Capture: `testdata/search_atornillador.json` (`q=atornillador`, 137 results).

Top-level keys: `items`, `numFound`, `offset`, `fetchsize`, `maxfetchsize` (1000), `facets`, `sorts`, `sort` (an object, the active one: `{code,label,…}`), `breadcrumbs`, `resultType`, `q`, `masterQuery`, `selections` (when filters are applied).

**Paging.** `offset` moves in steps of `fetchsize`; the CLI uses 48 and computes `pages = ceil(numFound / 48)`. Offset 96 returns the last 41 of 137.

**Total flutters.** The same request answered `numFound` 137 and 139 within a minute. Do not assert on an exact total in anything live.

**Items.** Each is `{code, resultClass, type, label, url, xPayload, gridbox: {country, sequence, data}}`. `gridbox.data` is the product: `productId`, `erpNumber`, `ians`, `fullTitle`, `title`, `brand{name,id,url}`, `canonicalPath` (`/p/<slug>/p<id>`), `category` (a `/`-joined path), `categorySecondaryPath`, `image`, `imageList`, `keyfacts{description,supplementalDescription,…}`, `price`, `ratings{average,count,…}`, `stockAvailability`, `ribbons`, `seals`, `lidlPlus`, `online`, `preventSelling`, `zones`, `store`, `regionsPrices`.

**Price.** `price.price` is the current price; `currencyCode`, `tax` (VAT rate, "21") and `startDate` ride along. A marked-down product also has:

- `oldPrice`: the struck-through price (`discount.deletedPrice` repeats it),
- `discount{absoluteDiscount, percentageDiscount, bargainHintText ("Megadescuento"), showDiscount, …}`,
- `endDate` for a fixed-length promotion (absent or null otherwise).

Captured in `testdata/search_discounted.json` (`category=Bricolaje y jardín`, `sort=percentageDiscount-desc`).

**Availability.** `stockAvailability.onlineAvailable` is the flag; `badgeInfoV2[].badges[].text` is the label ("Disponible online"); `availabilityIndicator` is 3 when available (47 of 48 in the capture) and 1 otherwise. The CLI treats a product as available when `onlineAvailable && online && !preventSelling`. `store: false` on every item seen; no store is named anywhere ([ADR 0003](adr/0003-online-stock-only.md)).

**Facets.** `facets[]` is `{code, label, type, selector, values|topvalues}`. Seen for the atornillador search: `brand` (TEXT, OR), `Tipo de Batería`, `store`, `price` (RANGE), `ratingAverage` (NUMERIC), `availabilityIndicator` (TEXT), `category` (TEXT), `Inalámbrico`, `Capacidad`, `color`, `newArticleFlag` (SWITCH). The set depends on the search.

**Filters are query parameters named after the facet code.** Verified live:

| filter | parameter | notes |
|---|---|---|
| brand | `brand=PARKSIDE` | repeat for several (`PARKSIDE` + `LUPILU` = 101 = 98 + 3); case-insensitive |
| price | `price=0 - 40` | "min - max", both ends; `price=0-40` (no spaces) answers 400 with an empty body |
| rating | `ratingAverage=4.0 - 5.01` | "Más de 4 estrellas" |
| available online | `availabilityIndicator=3` | 107 of 110 with `price=0 - 40` |
| category | `category=Bricolaje y jardín` | the facet's label; the numeric id (`10067761`) matches nothing |
| other facets | `<code>=<value>` | same pattern |

An empty `q` with a `category` returns the whole category (1065 products for "Bricolaje y jardín"). `q=%20` alone returns the whole catalogue (6596).

**Sorts** (`sorts[].code`, passed as `sort=`): `relevancy`, `price` (ascending), `price-desc`, `firstOnlineDate-desc` (Novedades), `ratingScore-desc`, `sh_carts-desc` (Más vendidos), `percentageDiscount-desc` (Ofertas y mayores descuentos primero). The CLI maps them to `relevance`, `price_asc`, `price_desc`, `newest`, `rating`, `bestselling`, `discount`. Note: `sort=price` once showed `numFound` 139 against 137 for relevance; see "Total flutters".

**Redirect result.** `q=<product number>` answers `{"resultType":"redirect","redirectURL":"/p/…/p100405593"}` with no items. The CLI turns that into an empty listing. `/q/api/gridboxes` is not defined for GET (`404 "The current request is not defined by this API."`).

## 3. Product pages

`GET /p/<slug>/p<id>`; `/p/x/p<id>` answers 301 to the real slug, so a bare number needs no search. Capture: `testdata/product_100405593.html` (488 KB).

The page is server-rendered Nuxt HTML with three `application/ld+json` blocks. The `Product` one has `sku`, `gtin13` (list), `name`, `description` (HTML), `image` (list), `url`, `brand.name`, `offers[]` (`price`, `priceCurrency`, `availability` such as `OnlineOnly`, `url`) and `aggregateRating{ratingCount, reviewCount, ratingValue}`. That is all `product show` reads. A missing page answers 404.

Not read: the spec table and the Nuxt payload (`window.__NUXT__`, an index-compressed array that changes per build). The page carries no old price.

## 4. Categories

`/sitemap.xml` (`/static/sitemap.xml`) is an index of `/p/export/ES/es/product_sitemap.xml.gz`, `/explore/assets/s/pages_es-ES_es.xml.gz` and the store pages. The pages sitemap (gzip) lists 642 `/c/<slug>/s<id>` URLs: departments (`bricolaje-y-jardin` is `s10067761`), brands (`crivit`), and editorial pages, some with German slugs (`retail-baumarkt-garten`) or test names. No display names. Sample: `testdata/sitemap_pages_sample.xml` (the first 30 entries).

## 5. Not found (yet)

- No per-store stock endpoint.
- No old price on the product page.
- The suggest API and `ProductGridbox.html` fragment were not explored.
