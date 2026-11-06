# 0003. Online availability only

Status: accepted, 2026-11-06.

## Context

The author's other shop CLIs report stock per store. Lidl's search items carry `stockAvailability` (`onlineAvailable`, a badge, `availabilityIndicator`) and a `zones` object for regional prices and delivery, plus a `store` flag. None of it names a physical store, and no store-stock endpoint turned up in the page configuration (`searchPath`, `gridboxesPath`, `productScriptURL`, a suggest API).

## Decision

`product stock` reports whether a product can be bought online, from the product page (`offers.availability`). There is no `store` command and no quantity. A store lookup would be invented, so it is left out until a real endpoint is found.

## Consequences

- `--in-stock` means "available online" (the site's `availabilityIndicator=3` facet).
- A product that is not sold online shows as unavailable here, even if a store may have it.
