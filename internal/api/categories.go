package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Category is one /c/<slug>/s<id> page of the site, taken from its pages
// sitemap. The sitemap carries no names, so Name is the slug made readable.
// It lists editorial and brand pages too, not only the product taxonomy.
type Category struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

const categorySitemap = "/explore/assets/s/pages_es-ES_es.xml.gz"

var reCategoryLoc = regexp.MustCompile(`<loc>https?://[^/<]+(/c/([^/<]+)/s(\d+))</loc>`)

// Categories lists the category pages, sorted by id (the site's top-level
// departments come first).
func (c *Client) Categories(ctx context.Context) ([]Category, error) {
	resp, err := c.get(ctx, request{path: categorySitemap, endpoint: "GET " + categorySitemap})
	if err != nil {
		return nil, err
	}
	return parseCategorySitemap(resp.body)
}

func parseCategorySitemap(body []byte) ([]Category, error) {
	if len(body) > 2 && body[0] == 0x1f && body[1] == 0x8b {
		zr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, changed("GET "+categorySitemap, "bad gzip", body)
		}
		if body, err = io.ReadAll(io.LimitReader(zr, 64<<20)); err != nil {
			return nil, changed("GET "+categorySitemap, "bad gzip", body)
		}
	}
	seen := map[string]bool{}
	var out []Category
	for _, m := range reCategoryLoc.FindAllStringSubmatch(string(body), -1) {
		if seen[m[3]] {
			continue
		}
		seen[m[3]] = true
		out = append(out, Category{ID: m[3], Slug: m[2], Name: strings.ReplaceAll(m[2], "-", " "), URL: DefaultBase + m[1]})
	}
	if len(out) == 0 {
		return nil, changed("GET "+categorySitemap, "no category URLs", body)
	}
	sort.Slice(out, func(i, j int) bool {
		a, _ := strconv.Atoi(out[i].ID)
		b, _ := strconv.Atoi(out[j].ID)
		return a < b
	})
	return out, nil
}
