package api

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
)

// PageSize is the fetchsize the CLI asks for. The API accepts more, but 48 is
// what the site's own listing uses.
const PageSize = 48

// Price is what the site charges now. OldPrice is the struck-through price of
// a discount, nil when there is none. Start and End bound the promo window as
// the site words them (RFC 3339); End is empty for a price with no end date.
type Price struct {
	Amount          float64  `json:"amount"`
	OldPrice        *float64 `json:"old_price,omitempty"`
	DiscountPercent *int     `json:"discount_percent,omitempty"`
	Currency        string   `json:"currency"`
	Tax             string   `json:"tax,omitempty"` // VAT rate in percent, "21"
	Start           string   `json:"start,omitempty"`
	End             string   `json:"end,omitempty"`
}

// Rating is the customer rating, nil when nobody rated the product.
type Rating struct {
	Average float64 `json:"average"`
	Count   int     `json:"count"`
}

// Stock is online availability. Lidl reports it per product, not per store.
type Stock struct {
	Available bool   `json:"available"`
	Label     string `json:"label,omitempty"` // the site's badge, "Disponible online"
}

// Product is one product as a listing card shows it.
type Product struct {
	Ref      string   `json:"ref"`
	Title    string   `json:"title"`
	URL      string   `json:"url"`
	Image    string   `json:"image,omitempty"`
	Brand    string   `json:"brand,omitempty"`
	Category string   `json:"category,omitempty"`
	Price    *Price   `json:"price"`
	Rating   *Rating  `json:"rating,omitempty"`
	Stock    *Stock   `json:"stock,omitempty"`
	Badges   []string `json:"badges,omitempty"`
}

type FilterValue struct {
	Value   string `json:"value"`
	Label   string `json:"label,omitempty"`
	Count   int    `json:"count"`
	Applied bool   `json:"applied,omitempty"`
}

// Filter is one facet the site offers for a listing. Code is what goes into
// --filter CODE=VALUE.
type Filter struct {
	Code    string        `json:"code"`
	Label   string        `json:"label"`
	Type    string        `json:"type"` // TEXT, RANGE, NUMERIC or SWITCH
	Values  []FilterValue `json:"values,omitempty"`
	Applied bool          `json:"applied,omitempty"`
}

type Listing struct {
	Query    string    `json:"query,omitempty"`
	Page     int       `json:"page"`
	Pages    int       `json:"pages"`
	Total    int       `json:"total"`
	Sort     string    `json:"sort"`
	Products []Product `json:"products"`
	Filters  []Filter  `json:"-"`
}

// Sorts maps the CLI's sort names to the site's sort codes.
var Sorts = map[string]string{
	"relevance":   "relevancy",
	"price_asc":   "price",
	"price_desc":  "price-desc",
	"newest":      "firstOnlineDate-desc",
	"rating":      "ratingScore-desc",
	"bestselling": "sh_carts-desc",
	"discount":    "percentageDiscount-desc",
}

// SortNames lists the CLI's sort names in help order.
var SortNames = []string{"relevance", "price_asc", "price_desc", "newest", "rating", "bestselling", "discount"}

// ListParams is one search page.
type ListParams struct {
	Query     string
	Category  string // the site's category facet value: a label or numeric id, see `lidl search filters`
	Brands    []string
	Filters   map[string][]string // raw facet CODE to values
	MinPrice  *float64
	MaxPrice  *float64
	MinRating *float64
	Sort      string // site code, one of Sorts' values
	Page      int    // 1-based
	// InStock asks for the "Mostrar sólo productos disponibles" facet.
	InStock bool
}

// searchBase is the query every search call carries.
func searchBase(q string, offset int) url.Values {
	v := url.Values{}
	v.Set("q", q)
	v.Set("locale", "es_ES")
	v.Set("assortment", "ES")
	v.Set("version", "2.1.0")
	v.Set("fetchsize", strconv.Itoa(PageSize))
	v.Set("offset", strconv.Itoa(offset))
	return v
}

func num(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

// rangeBound stands in for a missing end of a range. The site's range filter
// needs both ends.
const rangeBound = 100000

func (p ListParams) request() request {
	page := max(p.Page, 1)
	q := searchBase(p.Query, (page-1)*PageSize)
	if p.Sort != "" && p.Sort != "relevancy" {
		q.Set("sort", p.Sort)
	}
	for _, b := range p.Brands {
		q.Add("brand", b)
	}
	if p.Category != "" {
		q.Set("category", p.Category)
	}
	if p.MinPrice != nil || p.MaxPrice != nil {
		lo, hi := 0.0, float64(rangeBound)
		if p.MinPrice != nil {
			lo = *p.MinPrice
		}
		if p.MaxPrice != nil {
			hi = *p.MaxPrice
		}
		q.Set("price", num(lo)+" - "+num(hi))
	}
	if p.MinRating != nil {
		// 5.01 is the upper bound the site itself sends.
		q.Set("ratingAverage", num(*p.MinRating)+" - 5.01")
	}
	if p.InStock {
		q.Set("availabilityIndicator", "3")
	}
	for code, vals := range p.Filters {
		for _, v := range vals {
			q.Add(code, v)
		}
	}
	return request{path: "/q/api/search", query: q, json: true, endpoint: "GET /q/api/search"}
}

// The subset of the search response this package reads.
type searchResponse struct {
	NumFound   int    `json:"numFound"`
	ResultType string `json:"resultType"`
	Sort       struct {
		Code string `json:"code"`
	} `json:"sort"`
	Items []struct {
		Code    string `json:"code"`
		Gridbox struct {
			Data gridData `json:"data"`
		} `json:"gridbox"`
	} `json:"items"`
	Facets []rawFacet `json:"facets"`
}

type gridData struct {
	ProductID      json.Number `json:"productId"`
	FullTitle      string      `json:"fullTitle"`
	Title          string      `json:"title"`
	CanonicalPath  string      `json:"canonicalPath"`
	Image          string      `json:"image"`
	Category       string      `json:"category"`
	Online         bool        `json:"online"`
	PreventSelling bool        `json:"preventSelling"`
	Brand          struct {
		Name string `json:"name"`
	} `json:"brand"`
	Price struct {
		Price        float64  `json:"price"`
		OldPrice     *float64 `json:"oldPrice"`
		CurrencyCode string   `json:"currencyCode"`
		Tax          string   `json:"tax"`
		StartDate    string   `json:"startDate"`
		EndDate      string   `json:"endDate"`
		Discount     *struct {
			Percentage int `json:"percentageDiscount"`
		} `json:"discount"`
	} `json:"price"`
	Ratings *struct {
		Average float64 `json:"average"`
		Count   int     `json:"count"`
	} `json:"ratings"`
	HavingPrice bool `json:"havingPrice"`
	Stock       struct {
		OnlineAvailable bool `json:"onlineAvailable"`
		Badges          []struct {
			Badges []struct {
				Text string `json:"text"`
			} `json:"badges"`
		} `json:"badgeInfoV2"`
	} `json:"stockAvailability"`
	Ribbons []struct {
		Text string `json:"text"`
	} `json:"ribbons"`
}

type rawFacet struct {
	Code      string          `json:"code"`
	Label     string          `json:"label"`
	Type      string          `json:"type"`
	Values    []rawFacetValue `json:"values"`
	TopValues []rawFacetValue `json:"topvalues"`
}

type rawFacetValue struct {
	Label    string          `json:"label"`
	Value    json.RawMessage `json:"value"`
	Count    int             `json:"count"`
	Selected bool            `json:"selected"`
	Link     struct {
		Filter map[string][]string `json:"filter"`
	} `json:"link"`
}

// List fetches one page of search results.
func (c *Client) List(ctx context.Context, p ListParams) (*Listing, error) {
	if strings.TrimSpace(p.Query) == "" && p.Category == "" && len(p.Brands) == 0 && len(p.Filters) == 0 {
		return nil, usagef("say what to search for, or narrow with --category, --brand or --filter")
	}
	r := p.request()
	resp, err := c.get(ctx, r)
	if err != nil {
		return nil, err
	}
	if resp.status != 200 {
		return nil, changed(r.endpoint, fmt.Sprintf("HTTP %d", resp.status), resp.body)
	}
	var sr searchResponse
	if err := json.Unmarshal(resp.body, &sr); err != nil {
		return nil, changed(r.endpoint, "not JSON", resp.body)
	}
	// A query that is a product number, or a name the site maps to a page,
	// answers with a redirect and no items.
	if sr.ResultType == "redirect" {
		return &Listing{Query: p.Query, Page: max(p.Page, 1), Sort: p.Sort, Products: []Product{}}, nil
	}
	if sr.Items == nil && sr.NumFound > 0 {
		return nil, changed(r.endpoint, "numFound without items", resp.body)
	}
	l := &Listing{
		Query:    p.Query,
		Page:     max(p.Page, 1),
		Total:    sr.NumFound,
		Pages:    int(math.Ceil(float64(sr.NumFound) / PageSize)),
		Sort:     sr.Sort.Code,
		Products: make([]Product, 0, len(sr.Items)),
	}
	if l.Sort == "" {
		l.Sort = p.Sort
	}
	for _, it := range sr.Items {
		l.Products = append(l.Products, it.Gridbox.Data.product(it.Code))
	}
	for _, f := range sr.Facets {
		l.Filters = append(l.Filters, f.filter())
	}
	return l, nil
}

func (g gridData) product(code string) Product {
	ref := g.ProductID.String()
	if ref == "" || ref == "0" {
		ref = code
	}
	title := g.FullTitle
	if title == "" {
		title = g.Title
	}
	p := Product{
		Ref: ref, Title: title, URL: abs(g.CanonicalPath), Image: g.Image,
		Brand: g.Brand.Name, Category: g.Category,
	}
	if g.HavingPrice || g.Price.Price > 0 {
		pr := &Price{Amount: g.Price.Price, OldPrice: g.Price.OldPrice, Currency: g.Price.CurrencyCode,
			Tax: g.Price.Tax, Start: g.Price.StartDate, End: g.Price.EndDate}
		if g.Price.Discount != nil && g.Price.OldPrice != nil {
			v := g.Price.Discount.Percentage
			pr.DiscountPercent = &v
		}
		p.Price = pr
	}
	if g.Ratings != nil && g.Ratings.Count > 0 {
		p.Rating = &Rating{Average: g.Ratings.Average, Count: g.Ratings.Count}
	}
	s := &Stock{Available: g.Stock.OnlineAvailable && g.Online && !g.PreventSelling}
	for _, grp := range g.Stock.Badges {
		for _, b := range grp.Badges {
			if s.Label == "" {
				s.Label = b.Text
			}
		}
	}
	p.Stock = s
	for _, r := range g.Ribbons {
		if r.Text != "" {
			p.Badges = append(p.Badges, r.Text)
		}
	}
	return p
}

// scalar renders a facet value, which is a string or a {"value": n} object.
func scalar(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var o struct {
		Value json.Number `json:"value"`
	}
	if json.Unmarshal(raw, &o) == nil {
		return o.Value.String()
	}
	return strings.Trim(string(raw), `"`)
}

func (f rawFacet) filter() Filter {
	out := Filter{Code: f.Code, Label: f.Label, Type: f.Type}
	vals := f.Values
	if len(vals) == 0 {
		vals = f.TopValues
	}
	for _, v := range vals {
		fv := FilterValue{Value: scalar(v.Value), Label: v.Label, Count: v.Count, Applied: v.Selected}
		// What goes after CODE= is what the site's own link for the value
		// carries: the category facet's value is an id, but its filter takes
		// the label (regression test TestFilterValueIsWhatTheLinkFilters).
		if lf := v.Link.Filter[f.Code]; len(lf) > 0 {
			fv.Value = lf[0]
		}
		if fv.Value == "" {
			fv.Value = v.Label
		}
		out.Values = append(out.Values, fv)
		out.Applied = out.Applied || v.Selected
	}
	return out
}
