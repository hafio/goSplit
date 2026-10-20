package httpapp

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/hafio/gosplit/internal/money"
	"github.com/hafio/gosplit/internal/store"
)

// feedLimit caps expense feed pages so they stay fast as history grows; a
// "Show all" link (see showAllHref below) re-requests without the cap.
const feedLimit = 50

// applyFeedLimit sets f.Limit (fetching one extra row to detect truncation)
// unless the request asked to see everything (?all=1).
func applyFeedLimit(r *http.Request, f *store.ExpenseFilter) bool {
	showAll := r.URL.Query().Get("all") == "1"
	if !showAll {
		f.Limit = feedLimit + 1
	}
	return showAll
}

// showAllHref truncates expenses to feedLimit and returns a re-request link
// with all=1 when the feed was truncated, or "" when everything already fits.
func showAllHref(r *http.Request, showAll bool, expenses *[]*store.Expense) string {
	if showAll || len(*expenses) <= feedLimit {
		return ""
	}
	*expenses = (*expenses)[:feedLimit]
	q := r.URL.Query()
	q.Set("all", "1")
	// v is the freshness poller's fingerprint of the previous render. Echoed
	// into the link it would make every render differ from the last, so a feed
	// over the limit could never be answered 204.
	q.Del("v")
	return r.URL.Path + "?" + q.Encode()
}

// expenseRow is the presentation model for the shared expense_table partial.
type expenseRow struct {
	ID            string
	Name          string
	Category      string
	Date          string
	DateDay       string // day-of-month for the feed date block, e.g. "08"
	DateMonthAbbr string // upper 3-letter month, e.g. "MAR"
	Amount        int64
	Currency      string
	PaidByName    string
	GroupName     string
	Deleted       bool
	Moved         bool
	SplitType     string
	MyNet         int64 // viewer's signed share (valid only when HasNet)
	HasNet        bool  // viewer participates in this expense
}

// monthGroup buckets feed rows under a month heading (e.g. "March 2026").
type monthGroup struct {
	Label string
	Rows  []expenseRow
}

// groupByMonth splits already date-sorted (newest-first) rows into consecutive
// month buckets for the feed.
func groupByMonth(rows []expenseRow) []monthGroup {
	var out []monthGroup
	for _, row := range rows {
		label := monthLabel(row.Date)
		if n := len(out); n > 0 && out[n-1].Label == label {
			out[n-1].Rows = append(out[n-1].Rows, row)
			continue
		}
		out = append(out, monthGroup{Label: label, Rows: []expenseRow{row}})
	}
	return out
}

func monthLabel(date string) string {
	if t, err := time.Parse("2006-01-02", dateOnly(date)); err == nil {
		return t.Format("January 2006")
	}
	return date // cov:ignore defensive: a stored date that does not parse
}

// nameCache resolves user/group display names within a single request without
// repeating lookups (fine at self-host scale).
type nameCache struct {
	s      *store.Store
	users  map[int64]string
	groups map[int64]string
}

func (s *Server) newNameCache() *nameCache {
	return &nameCache{s: s.Store, users: map[int64]string{}, groups: map[int64]string{}}
}

func (c *nameCache) user(ctx context.Context, id int64) string {
	if v, ok := c.users[id]; ok {
		return v
	}
	name := "user"
	if u, err := c.s.GetUser(ctx, id); err == nil {
		if u.Name != "" {
			name = u.Name
		} else {
			name = u.Email
		}
	}
	c.users[id] = name
	return name
}

func (c *nameCache) group(ctx context.Context, id int64) string {
	if v, ok := c.groups[id]; ok {
		return v
	}
	name := ""
	if g, err := c.s.GetGroup(ctx, id); err == nil {
		name = g.Name
	}
	c.groups[id] = name
	return name
}

// privateHeader marks a response the service worker must neither cache nor
// serve offline (sw.js checks it): admin pages, and other people's data seen
// through the admin override, would otherwise persist on the admin's device.
const privateHeader = "X-GoSplit-Private"

// markPrivate sets privateHeader on w; call it before the response is written.
func markPrivate(w http.ResponseWriter) { w.Header().Set(privateHeader, "1") }

// privateResponses is middleware that marks every response of a route group
// private (the admin area).
func privateResponses(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		markPrivate(w)
		next.ServeHTTP(w, r)
	})
}

// buildRows converts store expenses to presentation rows with resolved names,
// feed date parts, and the viewer's net share per row (positive = lent). Viewer
// 0 means a neutral feed (the admin's): no per-viewer net is looked up at all,
// which also keeps an all-users list from binding every row id into one query.
func (s *Server) buildRows(ctx context.Context, nc *nameCache, viewer int64, expenses []*store.Expense) []expenseRow {
	var nets map[string]int64
	if viewer != 0 {
		ids := make([]string, len(expenses))
		for i, e := range expenses {
			ids[i] = e.ID
		}
		nets, _ = s.Store.UserNetByExpense(ctx, viewer, ids)
	}

	rows := make([]expenseRow, 0, len(expenses))
	for _, e := range expenses {
		date := dateOnly(e.ExpenseDate)
		day, mon := feedDateParts(date)
		row := expenseRow{
			ID: e.ID, Name: e.Name, Category: e.Category, Date: date,
			DateDay: day, DateMonthAbbr: mon,
			Amount: e.Amount, Currency: e.Currency, PaidByName: nc.user(ctx, e.PaidBy),
			Deleted: e.DeletedAt.Valid, Moved: e.MovedFromID.Valid, SplitType: e.SplitType,
		}
		if net, ok := nets[e.ID]; ok {
			row.MyNet, row.HasNet = net, true
		}
		if e.GroupID.Valid {
			row.GroupName = nc.group(ctx, e.GroupID.Int64)
		}
		rows = append(rows, row)
	}
	return rows
}

// feedDateParts returns the day-of-month and an upper 3-letter month for the
// feed date block.
func feedDateParts(date string) (day, monthAbbr string) {
	if t, err := time.Parse("2006-01-02", date); err == nil {
		return t.Format("02"), strings.ToUpper(t.Format("Jan"))
	}
	return date, ""
}

// parseFilter builds a store.ExpenseFilter from query params plus a string map
// for re-rendering the filter bar. Amount min/max are parsed as 2-decimal
// values into minor units (spec §5.2).
func parseFilter(r *http.Request) (store.ExpenseFilter, map[string]string) {
	q := r.URL.Query()
	f := store.ExpenseFilter{}
	view := map[string]string{
		"q": q.Get("q"), "min": q.Get("min"), "max": q.Get("max"),
		"from": q.Get("from"), "to": q.Get("to"), "scope": q.Get("scope"),
		"archived": q.Get("archived"),
	}
	f.Descr = q.Get("q")
	f.IncludeArchived = q.Get("archived") == "1"
	if v := q.Get("min"); v != "" {
		if m, err := money.Parse(v, "USD"); err == nil {
			f.AmountMin = &m
		}
	}
	if v := q.Get("max"); v != "" { // cov:ignore glue: query plumbing; the filters are tested in internal/store
		if m, err := money.Parse(v, "USD"); err == nil {
			f.AmountMax = &m
		}
	}
	f.DateFrom = q.Get("from")
	f.DateTo = q.Get("to")
	switch q.Get("scope") {
	case "group": // cov:ignore glue: query plumbing; the filters are tested in internal/store
		f.Scope = store.ScopeOnlyGroup
	case "nongroup": // cov:ignore glue: query plumbing; the filters are tested in internal/store
		f.Scope = store.ScopeOnlyNonGroup
	default:
		f.Scope = store.ScopeAll
		view["scope"] = "all"
	}
	return f, view
}

// dateOnly trims an ISO timestamp to its date component.
func dateOnly(iso string) string {
	if len(iso) >= 10 {
		return iso[:10]
	}
	return iso // cov:ignore defensive: a date shorter than YYYY-MM-DD
}
