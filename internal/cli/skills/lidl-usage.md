# lidl usage notes

`lidl` reads lidl.es: products, prices, offers, ratings and online availability. It is
read-only and needs no account, login or browser session. Output is JSON on stdout;
`--format pretty` for people, `jsonl` for streams, `toon` for compact model context. Errors go
to stderr; `--error-format json` gives an envelope with `code`, `category` and `retryable`.

## Search and browse

```
lidl search atornillador
lidl search atornillador --max-price 40 --sort price_asc --in-stock
lidl search taladro --brand parkside --min-rating 4 --pages 2
lidl search --category "Bricolaje y jardín" --sort discount --limit 10
lidl search filters atornillador        # facet codes and values for --filter CODE=VALUE
```

Result: `{"query","page","pages","total","sort","products":[{ref,title,url,image,brand,category,
price:{amount,old_price,discount_percent,currency,tax,start,end},rating:{average,count},
stock:{available,label},badges}]}`. 48 products per page; `--page`, `--pages` and `--limit` page through.
Sorts: relevance, price_asc, price_desc, newest, rating, bestselling, discount.

Keywords are optional when `--category`, `--brand` or `--filter` narrows the search. `--category`
takes the site's category facet value (a label such as "Bricolaje y jardín"), listed by
`lidl search filters`. `lidl category list` shows the `/c/<slug>/s<id>` pages from the sitemap,
which include brand and editorial pages.

## Offers

Lidl runs limited-time promotions: products come and go. `price.old_price` is the struck-through
price (absent when the product is not marked down), `discount_percent` the badge, and
`price.start` / `price.end` the promotion window (`end` is empty without a fixed end). Find
current bargains with `--sort discount`; find fresh arrivals with `--sort newest`.

## Products and availability

```
lidl product show 100405593        # price, rating, EAN, images, description
lidl product stock 100405593       # online availability
```

`PRODUCT` is the number after `p` in `/p/<slug>/p100405593`, or the URL. Stock is per
product, online only: there is no per-store stock and no quantity. `product show` has no
`old_price`; use `search` for the promotion. A product that left the catalogue exits 4.

## Watches

```
lidl watch add product 100405593 --name taladro
lidl watch add search atornillador --max-price 40 --name atornillador --notify phone
lidl watch check --format jsonl
lidl watch events --since 24h
```

Events: `product.new`, `product.price_changed`, `product.out_of_stock`, `product.back_in_stock`,
`product.removed`. Each carries `watch`, `at`, `product` and, for changes, `change: {from, to}`.
Because promotions are short, a search watch with `--sort newest` reports new offers as
`product.new`.

## Exit codes

0 ok (also "no changes"), 1 generic, 2 usage, 4 not found, 5 refused or rate limited (wait minutes,
do not loop), 6 network or site down, 7 the site changed shape (stderr carries a prefilled issue
link), 130 interrupted.

Keep the request rate low: one command is one to a few requests; `--pages` adds more.
