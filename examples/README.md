# examples

Ready-to-run recipes. Everything is read-only and needs no setup.

## search

```bash
# human-readable
lidl search atornillador --format pretty

# cheap, orderable, best rated first
lidl search taladro --max-price 40 --in-stock --sort rating --limit 10 --format pretty

# several brands, rated 4 or more
lidl search taladro --brand parkside --brand silvercrest --min-rating 4

# the facets the site offers for a search
lidl search filters atornillador --format pretty

# a whole category
lidl search --category "Bricolaje y jardín" --sort newest --limit 20 --format pretty
```

## offers

```bash
# biggest markdowns first, with the end date
lidl search --category "Bricolaje y jardín" --sort discount --pages 2 \
  | jq -r '.products[] | select(.price.old_price) | "\(.price.discount_percent)%\t\(.price.amount) €\t\(.price.end // "-")\t\(.title)"'

# offers ending in the next two days
lidl search --category "Bricolaje y jardín" --sort discount --pages 3 \
  | jq -r --arg until "$(date -u -d '+2 days' +%FT%TZ)" '.products[] | select(.price.end and .price.end < $until) | "\(.price.end)\t\(.title)"'
```

## products

```bash
lidl product show 100405593 --format pretty
lidl product stock 100405593
lidl product show https://www.lidl.es/p/parkside-taladro-atornillador-recargable-de-20-v-pabs-20-li-g8-con-bateria-y-cargador/p100405593
```

## watch price and availability

```bash
# one product
lidl watch add product 100405593 --name taladro --notify phone

# new offers in a search
lidl watch add search taladro --brand parkside --sort newest --name parkside-new

lidl watch check --format jsonl
lidl watch events --since 24h | jq -r 'select(.type == "product.back_in_stock") | .product.url'
lidl watch service install --interval 1h
```

a watch is stored in the local SQLite database, not in a file; `watch add` is the only way to create one. `config.toml` holds the interval and the sinks: see `config.toml` here.

## check the install

```bash
lidl doctor --format pretty
```
