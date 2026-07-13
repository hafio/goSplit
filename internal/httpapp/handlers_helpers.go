package httpapp

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/hafio/gosplit/internal/money"
	"github.com/hafio/gosplit/internal/store"
)

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
	return date
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

// buildRows converts store expenses to presentation rows with resolved names,
// feed date parts, and the viewer's net share per row (positive = lent).
func (s *Server) buildRows(ctx context.Context, nc *nameCache, viewer int64, expenses []*store.Expense) []expenseRow {
	ids := make([]string, len(expenses))
	for i, e := range expenses {
		ids[i] = e.ID
	}
	nets, _ := s.Store.UserNetByExpense(ctx, viewer, ids)

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
	}
	f.Descr = q.Get("q")
	if v := q.Get("min"); v != "" {
		if m, err := money.Parse(v, "USD"); err == nil {
			f.AmountMin = &m
		}
	}
	if v := q.Get("max"); v != "" {
		if m, err := money.Parse(v, "USD"); err == nil {
			f.AmountMax = &m
		}
	}
	f.DateFrom = q.Get("from")
	f.DateTo = q.Get("to")
	switch q.Get("scope") {
	case "group":
		f.Scope = store.ScopeOnlyGroup
	case "nongroup":
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
	return iso
}
