package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// site serves the captures under testdata the way lidl.es does.
func site(t *testing.T, h http.HandlerFunc) *Client {
	return testClient(t, h)
}

func TestListRoundTrip(t *testing.T) {
	var got string
	c := site(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.RawQuery
		w.Write(fixture(t, "search_atornillador.json"))
	})
	l, err := c.List(context.Background(), ListParams{Query: "atornillador"})
	if err != nil {
		t.Fatal(err)
	}
	if l.Total != 137 || l.Pages != 3 || len(l.Products) != 48 {
		t.Fatalf("total %d pages %d products %d", l.Total, l.Pages, len(l.Products))
	}
	for _, want := range []string{"q=atornillador", "offset=0", "fetchsize=48", "assortment=ES", "locale=es_ES"} {
		if !strings.Contains(got, want) {
			t.Errorf("query %q lacks %s", got, want)
		}
	}
	p := l.Products[0]
	if p.Ref != "100405593" || p.Price == nil || p.Price.Amount != 36.99 || p.Price.Currency != "EUR" || p.Price.Tax != "21" {
		t.Fatalf("%+v", p)
	}
	if p.Rating == nil || p.Rating.Average != 4.7 || p.Rating.Count != 259 {
		t.Fatalf("rating %+v", p.Rating)
	}
	if p.Stock == nil || !p.Stock.Available || p.Stock.Label != "Disponible online" {
		t.Fatalf("stock %+v", p.Stock)
	}
	if p.Brand != "PARKSIDE®" || !strings.HasPrefix(p.URL, "https://www.lidl.es/p/parkside-") || p.Image == "" {
		t.Fatalf("%+v", p)
	}
	codes := map[string]bool{}
	for _, f := range l.Filters {
		codes[f.Code] = true
	}
	for _, c := range []string{"brand", "price", "category", "availabilityIndicator"} {
		if !codes[c] {
			t.Errorf("no %s facet in %v", c, codes)
		}
	}
}

// The struck-through price lives in price.oldPrice and the percentage in
// price.discount; neither is in the keys a first read of the card suggests.
func TestListDiscountedCard(t *testing.T) {
	c := site(t, func(w http.ResponseWriter, r *http.Request) { w.Write(fixture(t, "search_discounted.json")) })
	l, err := c.List(context.Background(), ListParams{Query: "atornillador", Sort: Sorts["discount"]})
	if err != nil {
		t.Fatal(err)
	}
	p := l.Products[0].Price
	if p.OldPrice == nil || *p.OldPrice != 9.99 || p.Amount != 7.49 || p.DiscountPercent == nil || *p.DiscountPercent != 25 {
		t.Fatalf("%+v", p)
	}
	if p.Start == "" || p.End == "" {
		t.Fatalf("promo window lost: %+v", p)
	}
}

func TestListParamsQuery(t *testing.T) {
	lo, hi, rt := 5.0, 40.5, 4.0
	q := ListParams{Query: "x", Brands: []string{"A", "B"}, MinPrice: &lo, MaxPrice: &hi, MinRating: &rt,
		Sort: "price", Page: 3, InStock: true, Category: "Bricolaje y jardín",
		Filters: map[string][]string{"color": {"rojo"}}}.request().query
	for k, want := range map[string]string{"offset": "96", "sort": "price", "price": "5 - 40.5",
		"ratingAverage": "4 - 5.01", "availabilityIndicator": "3", "category": "Bricolaje y jardín", "color": "rojo"} {
		if q.Get(k) != want {
			t.Errorf("%s = %q, want %q", k, q.Get(k), want)
		}
	}
	if len(q["brand"]) != 2 {
		t.Errorf("brands %v", q["brand"])
	}
	// Only a ceiling: the site's range needs both ends.
	if got := (ListParams{Query: "x", MaxPrice: &hi}).request().query.Get("price"); got != "0 - 40.5" {
		t.Errorf("price = %q", got)
	}
}

// An empty query with a category browses it: the site answers with the whole
// category.
func TestListCategoryWithoutQuery(t *testing.T) {
	var got string
	c := site(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query().Get("category")
		w.Write(fixture(t, "search_discounted.json"))
	})
	if _, err := c.List(context.Background(), ListParams{Category: "Bricolaje y jardín"}); err != nil || got != "Bricolaje y jardín" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestListEmptyQueryIsUsage(t *testing.T) {
	_, err := New("t").List(context.Background(), ListParams{})
	if e, ok := err.(*Error); !ok || e.Kind != KindUsage {
		t.Fatalf("%v", err)
	}
}

// A product number as the query answers resultType "redirect" and no items;
// the listing must come back empty, not as an api_changed error.
func TestListRedirectIsEmpty(t *testing.T) {
	c := site(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"type":"redirect","resultType":"redirect","redirectURL":"/p/x/p1"}`))
	})
	l, err := c.List(context.Background(), ListParams{Query: "100405593"})
	if err != nil || len(l.Products) != 0 || l.Total != 0 {
		t.Fatalf("%+v %v", l, err)
	}
}

func TestListNotJSONIsAPIChanged(t *testing.T) {
	c := site(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html>")) })
	_, err := c.List(context.Background(), ListParams{Query: "x"})
	if e, ok := err.(*Error); !ok || e.Kind != KindAPIChanged {
		t.Fatalf("%v", err)
	}
}

func TestProductRoundTrip(t *testing.T) {
	var path string
	c := site(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Write(fixture(t, "product_100405593.html"))
	})
	d, err := c.Product(context.Background(), "100405593")
	if err != nil {
		t.Fatal(err)
	}
	if path != "/p/x/p100405593" {
		t.Fatalf("path %s", path)
	}
	if d.Ref != "100405593" || d.EAN != "4054599217857" || d.Brand != "PARKSIDE®" {
		t.Fatalf("%+v", d)
	}
	if d.Price == nil || d.Price.Amount != 36.99 || d.Rating == nil || d.Rating.Count != 259 || d.Rating.Average != 4.7 {
		t.Fatalf("%+v %+v", d.Price, d.Rating)
	}
	if d.Stock == nil || !d.Stock.Available {
		t.Fatalf("stock %+v", d.Stock)
	}
	if !strings.Contains(d.Description, "Par de apriete máximo de 45 Nm") || strings.Contains(d.Description, "<") || !strings.Contains(d.Description, "\n- Batería") {
		t.Fatalf("description %q", d.Description)
	}
	if len(d.Images) < 2 {
		t.Fatalf("images %v", d.Images)
	}
}

func TestProductNotFound(t *testing.T) {
	c := site(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	_, err := c.Product(context.Background(), "999999999")
	if e, ok := err.(*Error); !ok || e.Kind != KindNotFound {
		t.Fatalf("%v", err)
	}
}

func TestProductArgs(t *testing.T) {
	for _, tc := range []struct{ in, path, ref string }{
		{"100405593", "/p/x/p100405593", "100405593"},
		{"https://www.lidl.es/p/foo-bar/p100408338", "/p/foo-bar/p100408338", "100408338"},
		{"/p/foo/p5555555", "/p/foo/p5555555", "5555555"},
	} {
		path, ref, err := ProductPath(tc.in)
		if err != nil || path != tc.path || ref != tc.ref {
			t.Errorf("%s -> %s %s %v", tc.in, path, ref, err)
		}
	}
	for _, bad := range []string{"", "abc", "https://evil.example/p/x/p100408338", "12"} {
		if _, _, err := ProductPath(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestAvailability(t *testing.T) {
	for in, want := range map[string]bool{"OnlineOnly": true, "https://schema.org/InStock": true, "OutOfStock": false, "SoldOut": false, "InStoreOnly": false, "": false} {
		if availableLD(in) != want {
			t.Errorf("%q", in)
		}
	}
}

func TestProductWithoutJSONLDIsAPIChanged(t *testing.T) {
	c := site(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html></html>")) })
	_, err := c.Product(context.Background(), "100405593")
	if e, ok := err.(*Error); !ok || e.Kind != KindAPIChanged {
		t.Fatalf("%v", err)
	}
}

func TestCategories(t *testing.T) {
	cats, err := parseCategorySitemap(fixture(t, "sitemap_pages_sample.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cats) != 30 || cats[0].ID != "10067761" || cats[0].Slug != "bricolaje-y-jardin" || cats[0].Name != "bricolaje y jardin" {
		t.Fatalf("%d %+v", len(cats), cats[0])
	}
	if _, err := parseCategorySitemap([]byte("<urlset/>")); err == nil {
		t.Fatal("empty sitemap accepted")
	}
}

func TestCategoriesGzip(t *testing.T) {
	c := site(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != categorySitemap {
			t.Errorf("path %s", r.URL.Path)
		}
		w.Write(gz(fixture(t, "sitemap_pages_sample.xml")))
	})
	cats, err := c.Categories(context.Background())
	if err != nil || len(cats) != 30 {
		t.Fatalf("%d %v", len(cats), err)
	}
}

// Reachable must work against a server that is up and fail on a 403.
func TestForbiddenIsBlocked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(403) }))
	defer srv.Close()
	c := New("t")
	c.Base, c.Gap = srv.URL, 0
	err := c.Reachable(context.Background())
	if e, ok := err.(*Error); !ok || e.Kind != KindBlocked {
		t.Fatalf("%v", err)
	}
}

func gz(b []byte) []byte {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	w.Write(b)
	w.Close()
	return buf.Bytes()
}

// The search API answers 401 to "Accept: application/json" and 200 to
// "*/*". The first live run of the CLI broke on it.
func TestSearchAcceptHeader(t *testing.T) {
	c := site(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") == "application/json" {
			w.WriteHeader(401)
			return
		}
		if !strings.HasPrefix(r.UserAgent(), "lidl-cli/") {
			t.Errorf("user agent %q does not name the tool", r.UserAgent())
		}
		w.Write(fixture(t, "search_atornillador.json"))
	})
	if _, err := c.List(context.Background(), ListParams{Query: "x"}); err != nil {
		t.Fatal(err)
	}
}
