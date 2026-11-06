package cli

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/n0nuser/lidl-cli/internal/api"
	"github.com/n0nuser/lidl-cli/internal/output"
)

// searchFlags are shared by `search`, `search filters` and `watch add search`.
type searchFlags struct {
	category  string
	brands    []string
	filters   []string
	minPrice  float64
	maxPrice  float64
	minRating float64
	sort      string
	inStock   bool
}

func bindSearchFlags(cmd *cobra.Command, f *searchFlags) {
	fl := cmd.Flags()
	fl.StringVar(&f.category, "category", "", "the site's category facet value, for example \"Bricolaje y jardín\" (see `lidl search filters`)")
	fl.StringArrayVar(&f.brands, "brand", nil, "brand, repeatable (the site's Marca facet; case does not matter)")
	fl.StringArrayVar(&f.filters, "filter", nil, "facet CODE=VALUE, repeatable (see `lidl search filters`)")
	fl.Float64Var(&f.minPrice, "min-price", 0, "minimum price in EUR")
	fl.Float64Var(&f.maxPrice, "max-price", 0, "maximum price in EUR")
	fl.Float64Var(&f.minRating, "min-rating", 0, "minimum average rating, 1 to 5")
	fl.StringVar(&f.sort, "sort", "relevance", "order: "+strings.Join(api.SortNames, ", "))
	fl.BoolVar(&f.inStock, "in-stock", false, "only products available online")
	_ = cmd.RegisterFlagCompletionFunc("sort", cobra.FixedCompletions(api.SortNames, cobra.ShellCompDirectiveNoFileComp))
}

func (a *App) listParams(cmd *cobra.Command, keywords []string, f searchFlags) (api.ListParams, error) {
	p := api.ListParams{Query: strings.TrimSpace(strings.Join(keywords, " ")), Filters: map[string][]string{},
		Category: f.category, Brands: f.brands, InStock: f.inStock}
	s, ok := api.Sorts[f.sort]
	if !ok {
		return p, output.Usagef("--sort must be one of %s", strings.Join(api.SortNames, ", "))
	}
	p.Sort = s
	if cmd.Flags().Changed("min-price") {
		v := f.minPrice
		p.MinPrice = &v
	}
	if cmd.Flags().Changed("max-price") {
		v := f.maxPrice
		p.MaxPrice = &v
	}
	if p.MinPrice != nil && p.MaxPrice != nil && *p.MinPrice > *p.MaxPrice {
		return p, output.Usagef("--min-price is above --max-price")
	}
	if cmd.Flags().Changed("min-rating") {
		if f.minRating < 1 || f.minRating > 5 {
			return p, output.Usagef("--min-rating is between 1 and 5")
		}
		v := f.minRating
		p.MinRating = &v
	}
	for _, kv := range f.filters {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" || v == "" {
			return p, output.Usagef("--filter takes CODE=VALUE, for example --filter color=negro. See `lidl search filters`")
		}
		switch k {
		case "price", "brand", "category", "ratingAverage", "availabilityIndicator":
			return p, output.Usagef("use --min-price/--max-price, --brand, --category, --min-rating or --in-stock instead of --filter %s", k)
		}
		p.Filters[k] = append(p.Filters[k], v)
	}
	if p.Query == "" && p.Category == "" && len(p.Brands) == 0 && len(p.Filters) == 0 {
		return p, output.Usagef("say what to search for, or narrow with --category, --brand or --filter")
	}
	return p, nil
}

// inStock keeps products that can be bought online. The site's own facet
// already asks for them; this drops any card it still returned unavailable,
// and filtering each page before counting lets --limit fill up from later
// pages.
func inStock(ps []api.Product) []api.Product {
	kept := ps[:0]
	for _, p := range ps {
		if p.Stock != nil && p.Stock.Available {
			kept = append(kept, p)
		}
	}
	return kept
}

type listingView struct{ *api.Listing }

func (v listingView) Pretty(w io.Writer, color bool) {
	rows := make([][]string, 0, len(v.Products))
	for _, p := range v.Products {
		rows = append(rows, productRow(p, color))
	}
	output.Table(w, color, []string{"REF", "PRICE", "WAS", "RATING", "ONLINE", "PRODUCT", "URL"}, rows)
	fmt.Fprintln(w, output.Dim(fmt.Sprintf("%d of %d products, page %d/%d", len(v.Products), v.Total, v.Page, max(v.Pages, 1)), color))
}

func productRow(p api.Product, color bool) []string {
	price, was, rating, online := "-", "", "", ""
	if p.Price != nil {
		price = fmt.Sprintf("%.2f €", p.Price.Amount)
		if p.Price.OldPrice != nil {
			was = fmt.Sprintf("%.2f €", *p.Price.OldPrice)
		}
	}
	if p.Rating != nil {
		rating = fmt.Sprintf("%.1f (%d)", p.Rating.Average, p.Rating.Count)
	}
	if p.Stock != nil {
		online = "yes"
		if !p.Stock.Available {
			online = "no"
		}
	}
	return []string{p.Ref, price, was, rating, online, output.Truncate(p.Title, 52), output.Dim(p.URL, color)}
}

func (a *App) searchCmd() *cobra.Command {
	var f searchFlags
	var page, pages, limit int
	cmd := &cobra.Command{
		Use:   "search [keywords...]",
		Short: "Search products with the site's filters",
		Long: `Search products using the filters the site offers: brand, price range,
rating, availability, sort order and any facet from ` + "`lidl search filters`" + `.

Keywords are optional when --category, --brand or --filter narrows the
search. Lidl sells limited-time offers, so the list changes: use --sort
newest or discount to see what is new or marked down, and read price.old_price
and price.end for the promotion.

Examples:
  lidl search atornillador --format pretty
  lidl search atornillador --max-price 40 --sort price_asc --in-stock
  lidl search --category "Bricolaje y jardín" --sort discount --limit 10
  lidl search taladro --brand parkside --min-rating 4 --pages 2 --format jsonl`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			p, err := a.listParams(cmd, args, f)
			if err != nil {
				return err
			}
			if page < 1 || pages < 1 {
				return output.Usagef("--page and --pages start at 1")
			}
			p.Page = page
			var out *api.Listing
			for i := 0; i < pages; i++ {
				l, err := a.Client.List(ctx, p)
				if err != nil {
					return err
				}
				if f.inStock {
					l.Products = inStock(l.Products)
				}
				if out == nil {
					out = l
				} else {
					out.Products = append(out.Products, l.Products...)
					out.Page = l.Page
				}
				if p.Page >= l.Pages || (limit > 0 && len(out.Products) >= limit) {
					break
				}
				p.Page++
			}
			if limit > 0 && len(out.Products) > limit {
				out.Products = out.Products[:limit]
			}
			if a.Printer.Format == output.JSONL {
				return a.Printer.Print(out.Products)
			}
			return a.Printer.Print(listingView{out})
		},
	}
	bindSearchFlags(cmd, &f)
	cmd.Flags().IntVar(&page, "page", 1, "result page to start at (48 products per page)")
	cmd.Flags().IntVar(&pages, "pages", 1, "how many pages to fetch")
	cmd.Flags().IntVar(&limit, "limit", 0, "stop after this many products (0: whole pages)")
	cmd.AddCommand(a.searchFiltersCmd())
	return cmd
}

type filtersView []api.Filter

func (v filtersView) Pretty(w io.Writer, color bool) {
	rows := make([][]string, 0, len(v))
	for _, f := range v {
		vals := make([]string, 0, len(f.Values))
		for _, x := range f.Values {
			vals = append(vals, fmt.Sprintf("%s (%d)", x.Value, x.Count))
		}
		rows = append(rows, []string{f.Code, f.Label, output.Truncate(strings.Join(vals, ", "), 70)})
	}
	output.Table(w, color, []string{"CODE", "FILTER", "VALUES (COUNT)"}, rows)
}

func (a *App) searchFiltersCmd() *cobra.Command {
	var f searchFlags
	cmd := &cobra.Command{
		Use:   "filters [keywords...]",
		Short: "List the filters the site offers for a search",
		Long: `List the facets the site offers for a search, with the values and how many
products each has. Pass CODE=VALUE to --filter.

Examples:
  lidl search filters atornillador --format pretty
  lidl search filters --category "Bricolaje y jardín"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := a.listParams(cmd, args, f)
			if err != nil {
				return err
			}
			l, err := a.Client.List(cmd.Context(), p)
			if err != nil {
				return err
			}
			if l.Filters == nil {
				l.Filters = []api.Filter{}
			}
			return a.Printer.Print(filtersView(l.Filters))
		},
	}
	bindSearchFlags(cmd, &f)
	return cmd
}

type productView struct{ *api.ProductDetail }

func (v productView) Pretty(w io.Writer, color bool) {
	d := v.ProductDetail
	rows := [][]string{{"ref", d.Ref}, {"product", d.Title}}
	if d.Brand != "" {
		rows = append(rows, []string{"brand", d.Brand})
	}
	price := "-"
	if d.Price != nil {
		price = fmt.Sprintf("%.2f €", d.Price.Amount)
	}
	rows = append(rows, []string{"price", price})
	if d.Rating != nil {
		rows = append(rows, []string{"rating", fmt.Sprintf("%.1f (%d reviews)", d.Rating.Average, d.Rating.Count)})
	}
	if d.Stock != nil {
		online := "no"
		if d.Stock.Available {
			online = "yes"
		}
		rows = append(rows, []string{"online", online})
	}
	if d.EAN != "" {
		rows = append(rows, []string{"ean", d.EAN})
	}
	rows = append(rows, []string{"url", d.URL})
	output.Table(w, color, nil, rows)
	if d.Description != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, d.Description)
	}
}

func (a *App) productCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "product",
		Aliases: []string{"item"},
		Short:   "Inspect one product",
	}
	show := &cobra.Command{
		Use:   "show PRODUCT",
		Short: "A product's price, rating, online availability and description",
		Long: `Show one product. PRODUCT is its number (the digits after "p" in
/p/<slug>/p100405593) or its URL.

The page has no struck-through price; use ` + "`lidl search`" + ` for old_price and the
promotion dates.

Examples:
  lidl product show 100405593 --format pretty
  lidl product show https://www.lidl.es/p/parkside-taladro/p100405593`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := a.Client.Product(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return a.Printer.Print(productView{d})
		},
	}
	stock := &cobra.Command{
		Use:   "stock PRODUCT",
		Short: "Whether a product can be bought online",
		Long: `Report online availability for one product. Lidl publishes stock per
product, not per store, and no quantity.

Examples:
  lidl product stock 100405593`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := a.Client.Product(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return a.Printer.Print(stockView{Ref: d.Ref, Title: d.Title, URL: d.URL, Stock: *d.Stock})
		},
	}
	cmd.AddCommand(show, stock)
	return cmd
}

type stockView struct {
	Ref   string    `json:"ref"`
	Title string    `json:"title"`
	URL   string    `json:"url"`
	Stock api.Stock `json:"stock"`
}

func (v stockView) Pretty(w io.Writer, color bool) {
	online := "no"
	if v.Stock.Available {
		online = "yes"
	}
	output.Table(w, color, nil, [][]string{{"ref", v.Ref}, {"product", v.Title}, {"online", online}, {"site says", v.Stock.Label}, {"url", v.URL}})
}

type categoryList []api.Category

func (l categoryList) Pretty(w io.Writer, color bool) {
	rows := make([][]string, 0, len(l))
	for _, c := range l {
		rows = append(rows, []string{c.ID, c.Name, output.Dim(c.URL, color)})
	}
	output.Table(w, color, []string{"ID", "NAME", "URL"}, rows)
}

func (a *App) categoryCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "category", Short: "Browse the site's category pages"}
	var match string
	list := &cobra.Command{
		Use:   "list",
		Short: "List the /c/<slug>/s<id> category pages",
		Long: `List the category pages from the site's sitemap. The sitemap also holds
brand and editorial pages, and carries no display names (Name is the slug).

To search inside a category, pass the site's category label to
` + "`lidl search --category`" + `; ` + "`lidl search filters`" + ` shows the labels.

Examples:
  lidl category list --format pretty
  lidl category list --match jardin`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cats, err := a.Client.Categories(cmd.Context())
			if err != nil {
				return err
			}
			out := categoryList{}
			for _, c := range cats {
				if match == "" || strings.Contains(strings.ToLower(c.Slug), strings.ToLower(match)) {
					out = append(out, c)
				}
			}
			sort.Slice(out, func(i, j int) bool {
				x, _ := strconv.Atoi(out[i].ID)
				y, _ := strconv.Atoi(out[j].ID)
				return x < y
			})
			return a.Printer.Print(out)
		},
	}
	list.Flags().StringVar(&match, "match", "", "only categories whose slug contains this text")
	cmd.AddCommand(list)
	return cmd
}
