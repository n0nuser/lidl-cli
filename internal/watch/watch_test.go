package watch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/n0nuser/lidl-cli/internal/api"
	"github.com/n0nuser/lidl-cli/internal/state"
)

func ptr[T any](v T) *T { return &v }

func TestDiffReportsPriceAndAvailability(t *testing.T) {
	w := state.Watch{Name: "w"}
	p := api.Product{Ref: "1", Stock: &api.Stock{}}
	prev := state.Seen{Price: ptr(10.0), Available: true}
	cur := state.Seen{Price: ptr(12.0), Available: false}
	evs := diff(w, prev, cur, p)
	if len(evs) != 2 || evs[0].Type != state.PriceChanged || evs[1].Type != state.OutOfStock {
		t.Fatalf("%+v", evs)
	}
	var ch struct{ From, To float64 }
	_ = json.Unmarshal(evs[0].Change, &ch)
	if ch.From != 10 || ch.To != 12 {
		t.Fatalf("change %s", evs[0].Change)
	}
	back := diff(w, cur, prev, p)
	if len(back) != 2 || back[1].Type != state.BackInStock {
		t.Fatalf("%+v", back)
	}
}

// A product without a stock block never produces stock events.
func TestNoStockNoStockEvents(t *testing.T) {
	evs := diff(state.Watch{}, state.Seen{Available: true}, state.Seen{}, api.Product{Ref: "1"})
	if len(evs) != 0 {
		t.Fatalf("%+v", evs)
	}
}

// End to end against the captured search page: the first check baselines
// silently, a second one with the same page reports nothing.
func TestSearchWatchBaselinesThenStaysQuiet(t *testing.T) {
	page := readFixture(t, "search_atornillador.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(page) }))
	defer srv.Close()
	c := api.New("test")
	c.Base, c.Gap = srv.URL, 0
	db, err := state.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	target, _ := json.Marshal(SearchTarget{Params: api.ListParams{Query: "atornillador"}})
	w, err := db.AddWatch(ctx, state.Watch{Name: "w", Kind: state.KindSearch, Target: target, Interval: time.Hour, Pages: 1})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		res, err := Check(ctx, c, db, w, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Events) != 0 || len(res.Upserts) != 48 {
			t.Fatalf("check %d: %d events, %d seen", i, len(res.Events), len(res.Upserts))
		}
		if err := db.CommitCheck(ctx, w.ID, res.Upserts, nil, res.Events); err != nil {
			t.Fatal(err)
		}
		w, _ = db.Watch(ctx, "w")
	}
	seen, _ := db.SeenFor(ctx, w.ID)
	if s := seen["100405593"]; s.Price == nil || *s.Price != 36.99 || !strings.Contains(string(s.Snapshot), "Taladro") {
		t.Fatalf("%+v", s)
	}
}
