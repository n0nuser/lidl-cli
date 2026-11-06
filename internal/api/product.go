package api

import (
	"context"
	"encoding/json"
	"html"
	"net/url"
	"regexp"
	"strings"
)

// ProductDetail is a product page. The embedded Product carries what the page
// shows: price (no struck-through price: only listings carry it), online
// availability and rating.
type ProductDetail struct {
	Product
	EAN         string   `json:"ean,omitempty"`
	Description string   `json:"description,omitempty"`
	Images      []string `json:"images,omitempty"`
}

var reRef = regexp.MustCompile(`^\d{6,12}$`)

// IsRef reports whether s looks like a product number.
func IsRef(s string) bool { return reRef.MatchString(s) }

var reProductURL = regexp.MustCompile(`/p/[^/?#]+/p(\d+)`)

// ProductPath resolves a product argument: a number, a product URL or a
// /p/<slug>/p<number> path. The site redirects /p/x/p<number> to the real
// slug, so a bare number needs no search round trip.
func ProductPath(arg string) (path, ref string, err error) {
	arg = strings.TrimSpace(arg)
	if IsRef(arg) {
		return "/p/x/p" + arg, arg, nil
	}
	if m := reProductURL.FindStringSubmatch(arg); m != nil {
		u, perr := url.Parse(arg)
		if perr != nil || (u.Host != "" && !strings.HasSuffix(u.Host, "lidl.es")) {
			return "", "", usagef("%q is not a lidl.es product", arg)
		}
		return u.Path, m[1], nil
	}
	return "", "", usagef("%q is not a product number or a lidl.es /p/<slug>/p<number> URL", arg)
}

// Product reads a product page. arg is a product number, URL or path.
func (c *Client) Product(ctx context.Context, arg string) (*ProductDetail, error) {
	path, ref, err := ProductPath(arg)
	if err != nil {
		return nil, err
	}
	r := request{path: path, endpoint: "GET /p/{slug}/p{id}"}
	resp, err := c.get(ctx, r)
	if err != nil {
		return nil, err
	}
	if resp.status == 404 {
		return nil, notFoundf("no product %s on lidl.es", ref)
	}
	return parseProduct(string(resp.body), ref, resp.url.Path, r.endpoint)
}

var reLDJSON = regexp.MustCompile(`(?s)<script type="application/ld\+json"[^>]*>(.*?)</script>`)

type ldProduct struct {
	Type        string          `json:"@type"`
	SKU         string          `json:"sku"`
	GTIN13      json.RawMessage `json:"gtin13"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	URL         string          `json:"url"`
	Image       json.RawMessage `json:"image"`
	Brand       struct {
		Name string `json:"name"`
	} `json:"brand"`
	Offers json.RawMessage `json:"offers"`
	Rating *struct {
		Count json.Number `json:"ratingCount"`
		Value float64     `json:"ratingValue"`
	} `json:"aggregateRating"`
}

type ldOffer struct {
	Price        json.Number `json:"price"`
	Currency     string      `json:"priceCurrency"`
	Availability string      `json:"availability"`
	URL          string      `json:"url"`
}

// oneOrMany decodes a JSON-LD value that is a string or a list of strings.
func oneOrMany(raw json.RawMessage) []string {
	var many []string
	if json.Unmarshal(raw, &many) == nil {
		return many
	}
	var one string
	if json.Unmarshal(raw, &one) == nil && one != "" {
		return []string{one}
	}
	return nil
}

func parseProduct(page, ref, path, endpoint string) (*ProductDetail, error) {
	var ld *ldProduct
	for _, m := range reLDJSON.FindAllStringSubmatch(page, -1) {
		var p ldProduct
		if json.Unmarshal([]byte(m[1]), &p) == nil && p.Type == "Product" {
			ld = &p
			break
		}
	}
	if ld == nil {
		return nil, changed(endpoint, "no schema.org Product on the page", []byte(page))
	}
	d := &ProductDetail{Product: Product{
		Ref: ld.SKU, Title: strings.TrimSpace(ld.Name), URL: abs(ld.URL), Brand: ld.Brand.Name,
	}}
	if d.Ref == "" {
		d.Ref = ref
	}
	if d.URL == "" {
		d.URL = abs(path)
	}
	if g := oneOrMany(ld.GTIN13); len(g) > 0 {
		d.EAN = g[0]
	}
	d.Images = oneOrMany(ld.Image)
	if len(d.Images) > 0 {
		d.Image = d.Images[0]
	}
	d.Description = htmlText(ld.Description)
	if ld.Rating != nil {
		if n, err := ld.Rating.Count.Int64(); err == nil && n > 0 {
			d.Rating = &Rating{Average: ld.Rating.Value, Count: int(n)}
		}
	}
	var offers []ldOffer
	if json.Unmarshal(ld.Offers, &offers) != nil {
		var one ldOffer
		if json.Unmarshal(ld.Offers, &one) == nil {
			offers = []ldOffer{one}
		}
	}
	if len(offers) == 0 {
		return nil, changed(endpoint, "no offer in the schema.org Product", []byte(page))
	}
	o := offers[0]
	if amount, err := o.Price.Float64(); err == nil {
		d.Price = &Price{Amount: amount, Currency: o.Currency}
	}
	d.Stock = &Stock{Available: availableLD(o.Availability), Label: o.Availability}
	return d, nil
}

// availableLD reads a schema.org availability value, with or without the
// schema.org/ prefix: these sell; the rest, InStoreOnly included, do not (online).
func availableLD(a string) bool {
	a = a[strings.LastIndex(a, "/")+1:]
	switch a {
	case "InStock", "OnlineOnly", "LimitedAvailability", "PreOrder", "PreSale", "BackOrder":
		return true
	}
	return false
}

var (
	reBlockEnd = regexp.MustCompile(`(?i)</(p|li|ul|ol|div|h\d)>|<br\s*/?>`)
	reTags     = regexp.MustCompile(`(?s)<[^>]*>`)
	reInline   = regexp.MustCompile(`[ \t\x{a0}]+`)
	reBlank    = regexp.MustCompile(`\n\s*\n+`)
)

// htmlText turns the description's HTML into plain text, one block per line.
func htmlText(s string) string {
	s = strings.ReplaceAll(s, "<li>", "- ")
	s = reBlockEnd.ReplaceAllString(s, "\n")
	s = reTags.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = reInline.ReplaceAllString(s, " ")
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	return strings.TrimSpace(reBlank.ReplaceAllString(strings.Join(lines, "\n"), "\n"))
}
