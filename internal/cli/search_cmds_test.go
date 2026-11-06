package cli

import (
	"net/http"
	"strings"
	"testing"

	"github.com/n0nuser/lidl-cli/internal/api"
)

const (
	searchPath  = "/q/api/search"
	productPath = "/p/x/p100405593"
)

func TestSearchJSONAndPretty(t *testing.T) {
	isolate(t, site(t, map[string]string{searchPath: "search_atornillador.json"}))
	out, errOut, code := run("search", "atornillador", "--limit", "5")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	l := decode[api.Listing](t, out)
	if l.Total != 137 || len(l.Products) != 5 || l.Products[0].Ref != "100405593" {
		t.Fatalf("%d %d %+v", l.Total, len(l.Products), l.Products[0])
	}
	pretty, _, code := run("search", "atornillador", "--limit", "5", "--format", "pretty", "--no-color")
	if code != 0 || !strings.Contains(pretty, "36.99 €") || !strings.Contains(pretty, "5 of 137 products") {
		t.Fatalf("%s", pretty)
	}
	jsonl, _, code := run("search", "atornillador", "--limit", "3", "--format", "jsonl")
	if lines := strings.Split(strings.TrimSpace(jsonl), "\n"); code != 0 || len(lines) != 3 {
		t.Fatalf("%d lines", len(lines))
	}
}

func TestSearchSendsTheFilters(t *testing.T) {
	var q map[string][]string
	isolate(t, siteFunc(t, func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query()
		w.Write(fixture(t, "search_atornillador.json"))
	}))
	_, errOut, code := run("search", "taladro", "--brand", "parkside", "--brand", "lupilu", "--min-price", "5", "--max-price", "40",
		"--min-rating", "4", "--sort", "price_asc", "--in-stock", "--filter", "color=negro", "--category", "Bricolaje y jardín")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	for k, want := range map[string]string{"q": "taladro", "price": "5 - 40", "ratingAverage": "4 - 5.01", "sort": "price",
		"availabilityIndicator": "3", "color": "negro", "category": "Bricolaje y jardín"} {
		if got := q[k]; len(got) != 1 || got[0] != want {
			t.Errorf("%s = %v, want %s", k, got, want)
		}
	}
	if len(q["brand"]) != 2 {
		t.Errorf("brand = %v", q["brand"])
	}
}

func TestSearchPagesAndLimit(t *testing.T) {
	var offsets []string
	isolate(t, siteFunc(t, func(w http.ResponseWriter, r *http.Request) {
		offsets = append(offsets, r.URL.Query().Get("offset"))
		w.Write(fixture(t, "search_atornillador.json"))
	}))
	out, _, code := run("search", "x", "--pages", "2", "--limit", "60")
	l := decode[api.Listing](t, out)
	if code != 0 || len(l.Products) != 60 || strings.Join(offsets, ",") != "0,48" {
		t.Fatalf("%d products, offsets %v", len(l.Products), offsets)
	}
	offsets = nil
	run("search", "x", "--page", "3")
	if strings.Join(offsets, ",") != "96" {
		t.Fatalf("offsets %v", offsets)
	}
}

func TestSearchUsageErrors(t *testing.T) {
	isolate(t, "http://127.0.0.1:1")
	for _, args := range [][]string{
		{"search"},
		{"search", "x", "--sort", "cheapest"},
		{"search", "x", "--min-price", "9", "--max-price", "3"},
		{"search", "x", "--min-rating", "9"},
		{"search", "x", "--filter", "nope"},
		{"search", "x", "--filter", "price=1-2"},
		{"search", "x", "--page", "0"},
		{"search", "filters"},
	} {
		if _, _, code := run(args...); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
}

func TestSearchFiltersCommand(t *testing.T) {
	isolate(t, site(t, map[string]string{searchPath: "search_atornillador.json"}))
	out, errOut, code := run("search", "filters", "atornillador")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	var codes []string
	for _, f := range decode[[]api.Filter](t, out) {
		codes = append(codes, f.Code)
	}
	if !strings.Contains(strings.Join(codes, ","), "brand") || !strings.Contains(strings.Join(codes, ","), "price") {
		t.Fatalf("%v", codes)
	}
	if pretty, _, code := run("search", "filters", "atornillador", "--format", "pretty"); code != 0 || !strings.Contains(pretty, "PARKSIDE (98)") {
		t.Fatalf("%s", pretty)
	}
}

func TestProductShowAndStock(t *testing.T) {
	isolate(t, site(t, map[string]string{productPath: "product_100405593.html"}))
	out, errOut, code := run("product", "show", "100405593")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	d := decode[api.ProductDetail](t, out)
	if d.Ref != "100405593" || d.Price.Amount != 36.99 || d.Rating.Count != 259 || !strings.Contains(d.Description, "45 Nm") {
		t.Fatalf("%+v", d)
	}
	pretty, _, _ := run("product", "show", "100405593", "--format", "pretty", "--no-color")
	for _, want := range []string{"36.99 €", "4.7 (259 reviews)", "online", "45 Nm"} {
		if !strings.Contains(pretty, want) {
			t.Errorf("pretty lacks %q:\n%s", want, pretty)
		}
	}
	out, _, code = run("product", "stock", "100405593")
	if st := decode[stockView](t, out); code != 0 || !st.Stock.Available || st.Ref != "100405593" {
		t.Fatalf("%d %+v", code, st)
	}
	if pretty, _, code := run("product", "stock", "100405593", "--format", "pretty"); code != 0 || !strings.Contains(pretty, "yes") {
		t.Fatalf("%s", pretty)
	}
}

func TestProductErrors(t *testing.T) {
	isolate(t, site(t, map[string]string{productPath: "404"}))
	if _, _, code := run("product", "show", "100405593"); code != 4 {
		t.Fatalf("missing product: exit %d, want 4", code)
	}
	if _, _, code := run("product", "show", "abc"); code != 2 {
		t.Fatalf("bad argument: exit %d, want 2", code)
	}
}

func TestCategoryList(t *testing.T) {
	isolate(t, site(t, map[string]string{"/explore/assets/s/pages_es-ES_es.xml.gz": "sitemap_pages_sample.xml"}))
	out, errOut, code := run("category", "list")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if cats := decode[[]api.Category](t, out); len(cats) != 30 || cats[0].ID != "10067761" {
		t.Fatalf("%d %+v", len(cats), cats[0])
	}
	out, _, _ = run("category", "list", "--match", "bricolaje")
	if cats := decode[[]api.Category](t, out); len(cats) != 1 {
		t.Fatalf("%+v", cats)
	}
	if pretty, _, code := run("category", "list", "--format", "pretty"); code != 0 || !strings.Contains(pretty, "bricolaje y jardin") {
		t.Fatalf("%s", pretty)
	}
}
