# 0001. Read the search API and the product page's JSON-LD

Status: accepted, 2026-11-06.

## Context

lidl.es is a Nuxt app. Its own listing calls `GET /q/api/search`, which answers JSON with the products, facets, sorts and the total. Product pages are server-rendered and carry a schema.org `Product` block. Neither needs cookies, a login or a browser User-Agent.

## Decision

`internal/api` calls the search API for listings and filters (price, brand, category, rating, availability and any other facet are plain query parameters). A product is read from its page's JSON-LD (`/p/x/p<number>` redirects to the real slug). Categories come from the pages sitemap.

The search response is large (600 KB for 48 products) but stable, and it is the only place with the struck-through price and the promotion dates, so `search` is the source for offers.

## Consequences

- No HTML scraping beyond one `<script type="application/ld+json">` block.
- `product show` has no `old_price`. The page's Nuxt payload has more, but it is an index-compressed array that changes with each build; not worth parsing.
- If the API version changes, requests keep `version=2.1.0`; a shape change surfaces as exit 7.
