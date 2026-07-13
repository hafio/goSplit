package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/hafio/gosplit/internal/balance"
	"github.com/hafio/gosplit/internal/config"
)

// openTestStore opens a fresh SQLite store backed by a temp file (migrations run
// on open). Users/groups get ids 1..N in insertion order.
func openTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := Open(context.Background(), &config.Config{
		DatabaseURL: "file:" + dbPath, Engine: config.EngineSQLite,
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// --- Golden Scenario construction (ports the reference generator, §9) ------

var (
	goldenUsers  = []string{"Alice", "Bob", "Carol", "Dave", "Erin"} // ids 1..5
	goldenGroups = [][]int64{
		{1, 2, 3, 4, 5}, {1, 2, 3}, {3, 4, 5}, {1, 3, 5}, {2, 4}, {1, 2, 4, 5},
	}
	currencies = []string{"USD", "EUR"}
	perHead    = []int64{1000, 1500, 2000, 1200, 3000}
	pct        = map[int][][]int64{
		2: {{70, 30}, {60, 40}, {50, 50}, {80, 20}, {55, 45}},
		3: {{50, 30, 20}, {40, 30, 30}, {20, 50, 30}, {45, 35, 20}, {60, 25, 15}},
		4: {{40, 30, 20, 10}, {25, 25, 25, 25}, {50, 20, 20, 10}, {10, 20, 30, 40}, {35, 25, 25, 15}},
		5: {{30, 25, 20, 15, 10}, {20, 20, 20, 20, 20}, {40, 15, 15, 15, 15}, {10, 20, 30, 20, 20}, {25, 25, 20, 20, 10}},
	}
	pctUnit = []int64{100, 150, 200}
	exact   = map[int][][]int64{
		2: {{2500, 1500}, {3000, 2000}, {1234, 5766}, {4000, 1000}, {2222, 3778}},
		3: {{1000, 2000, 3000}, {1500, 2500, 2000}, {3333, 3333, 3334}, {500, 1500, 4000}, {2000, 2000, 2000}},
		4: {{1000, 2000, 3000, 4000}, {2500, 2500, 2500, 2500}, {1000, 1000, 1000, 7000}, {1500, 2500, 3500, 2500}, {3000, 1000, 2000, 4000}},
		5: {{1000, 2000, 3000, 4000, 5000}, {3000, 3000, 3000, 3000, 3000}, {500, 1500, 2500, 3500, 7000}, {2000, 2000, 2000, 2000, 7000}, {1111, 2222, 3333, 4444, 4890}},
	}
	shareW = map[int][][]int64{
		2: {{2, 3}, {1, 1}, {3, 1}, {4, 1}, {5, 2}},
		3: {{1, 2, 3}, {2, 2, 1}, {1, 1, 1}, {3, 1, 2}, {4, 3, 2}},
		4: {{1, 1, 2, 2}, {3, 2, 2, 1}, {1, 1, 1, 1}, {2, 3, 1, 4}, {5, 1, 2, 2}},
		5: {{1, 2, 3, 4, 5}, {2, 2, 2, 2, 2}, {1, 1, 1, 1, 1}, {3, 1, 2, 1, 3}, {4, 3, 2, 1, 5}},
	}
	shareUnit = []int64{500, 1000}
	adj       = map[int][][]int64{
		2: {{500, 0}, {0, 300}, {700, 200}},
		3: {{500, 0, 0}, {300, 300, 0}, {0, 0, 600}},
		4: {{500, 0, 0, 0}, {200, 200, 200, 0}, {0, 300, 0, 300}},
		5: {{500, 0, 0, 0, 0}, {100, 100, 100, 100, 100}, {0, 0, 500, 0, 0}},
	}
	adjBase    = []int64{1000, 1500}
	splitCycle = []string{"EQUAL", "PERCENTAGE", "EXACT", "SHARE", "ADJUSTMENT"}
)

func sharesFor(st string, members []int64, k int) map[int64]int64 {
	n := len(members)
	out := map[int64]int64{}
	switch st {
	case "EQUAL":
		ph := perHead[mod(k, len(perHead))]
		for _, u := range members {
			out[u] = ph
		}
	case "PERCENTAGE":
		row := pct[n][mod(k, len(pct[n]))]
		unit := pctUnit[mod(k, len(pctUnit))]
		for i, u := range members {
			out[u] = row[i] * unit
		}
	case "EXACT":
		row := exact[n][mod(k, len(exact[n]))]
		for i, u := range members {
			out[u] = row[i]
		}
	case "SHARE":
		row := shareW[n][mod(k, len(shareW[n]))]
		unit := shareUnit[mod(k, len(shareUnit))]
		for i, u := range members {
			out[u] = row[i] * unit
		}
	case "ADJUSTMENT":
		row := adj[n][mod(k, len(adj[n]))]
		base := adjBase[mod(k, len(adjBase))]
		for i, u := range members {
			out[u] = base + row[i]
		}
	}
	return out
}

func mod(a, b int) int { return ((a % b) + b) % b }

type goldenTxn struct {
	splitType string
	groupID   *int64
	currency  string
	paidBy    int64
	total     int64
	parts     []ExpenseParticipant
}

func buildAmounts(shares map[int64]int64, members []int64, payer int64) (int64, []ExpenseParticipant) {
	var total int64
	for _, s := range shares {
		total += s
	}
	parts := make([]ExpenseParticipant, 0, len(members))
	for _, u := range members {
		amt := -shares[u]
		if u == payer {
			amt = total - shares[u]
		}
		parts = append(parts, ExpenseParticipant{UserID: u, Amount: amt})
	}
	return total, parts
}

// generateGolden mirrors the reference generator to produce the 230 txns.
func generateGolden() []goldenTxn {
	var txns []goldenTxn
	gid := func(i int) *int64 { v := int64(i); return &v }

	// 1) group expenses.
	for gi, members := range goldenGroups {
		g := gid(gi + 1)
		for i := 0; i < 25; i++ {
			st := splitCycle[mod(i, 5)]
			payer := members[mod(i, len(members))]
			cur := currencies[mod(i, 2)]
			sh := sharesFor(st, members, i)
			total, parts := buildAmounts(sh, members, payer)
			txns = append(txns, goldenTxn{st, g, cur, payer, total, parts})
		}
	}

	// 2) direct expenses: 10 pairs, 6 each.
	pairs := [][2]int64{}
	for a := int64(1); a <= 5; a++ {
		for b := a + 1; b <= 5; b++ {
			pairs = append(pairs, [2]int64{a, b})
		}
	}
	for pi, p := range pairs {
		a, b := p[0], p[1]
		for i := 0; i < 6; i++ {
			st := splitCycle[mod(i, 5)]
			members := []int64{a, b}
			payer := a
			if i%2 != 0 {
				payer = b
			}
			cur := currencies[mod(pi+i, 2)]
			sh := sharesFor(st, members, pi+i)
			total, parts := buildAmounts(sh, members, payer)
			txns = append(txns, goldenTxn{st, nil, cur, payer, total, parts})
		}
	}

	// 3) settlements.
	settlements := []struct {
		sender, receiver, amount int64
		cur                      string
		group                    int
	}{
		{2, 1, 4000, "USD", 2}, {3, 1, 2500, "USD", 1}, {4, 3, 3000, "EUR", 3},
		{5, 3, 1500, "USD", 4}, {2, 4, 2000, "EUR", 5}, {1, 2, 3500, "USD", 0},
		{5, 1, 1000, "EUR", 0}, {4, 5, 2500, "USD", 3}, {1, 3, 1800, "EUR", 0},
		{2, 3, 2200, "USD", 2}, {4, 1, 1600, "USD", 6}, {5, 2, 1900, "EUR", 0},
	}
	for _, s := range settlements {
		var g *int64
		if s.group != 0 {
			v := int64(s.group)
			g = &v
		}
		parts := []ExpenseParticipant{{UserID: s.sender, Amount: s.amount}, {UserID: s.receiver, Amount: -s.amount}}
		txns = append(txns, goldenTxn{"SETTLEMENT", g, s.cur, s.sender, s.amount, parts})
	}

	// 4) currency conversions (each = 2 linked rows; 9/10 fx).
	conversions := []struct {
		sender, receiver, amountFrom int64
		curFrom, curTo               string
		group                        int
	}{
		{1, 2, 10000, "USD", "EUR", 0}, {3, 4, 5000, "USD", "EUR", 3},
		{5, 1, 8000, "USD", "EUR", 0}, {2, 3, 6000, "USD", "EUR", 2},
	}
	for _, c := range conversions {
		var g *int64
		if c.group != 0 {
			v := int64(c.group)
			g = &v
		}
		amountTo := c.amountFrom * 9 / 10
		txns = append(txns,
			goldenTxn{"CURRENCY_CONVERSION", g, c.curFrom, c.sender, c.amountFrom,
				[]ExpenseParticipant{{UserID: c.sender, Amount: c.amountFrom}, {UserID: c.receiver, Amount: -c.amountFrom}}},
			goldenTxn{"CURRENCY_CONVERSION", g, c.curTo, c.receiver, amountTo,
				[]ExpenseParticipant{{UserID: c.sender, Amount: -amountTo}, {UserID: c.receiver, Amount: amountTo}}},
		)
	}
	return txns
}

func TestGoldenScenario(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)

	// Seed users 1..5 and groups 1..6 in order (ids assigned sequentially).
	for i, name := range goldenUsers {
		u, err := st.CreateUser(ctx, &User{Name: name, Email: name + "@example.com"})
		if err != nil {
			t.Fatalf("create user: %v", err)
		}
		if u.ID != int64(i+1) {
			t.Fatalf("expected user id %d, got %d", i+1, u.ID)
		}
	}
	for gi := range goldenGroups {
		g, err := st.CreateGroup(ctx, &Group{Name: "G", CreatedBy: 1, PublicID: "g" + string(rune('a'+gi))})
		if err != nil {
			t.Fatalf("create group: %v", err)
		}
		if g.ID != int64(gi+1) {
			t.Fatalf("expected group id %d, got %d", gi+1, g.ID)
		}
	}

	txns := generateGolden()
	if len(txns) != 230 {
		t.Fatalf("expected 230 transactions, got %d", len(txns))
	}
	for _, tx := range txns {
		e := &Expense{
			Name: "golden", Category: "test", Amount: tx.total, SplitType: tx.splitType,
			ExpenseDate: "2025-01-01", Currency: tx.currency, PaidBy: tx.paidBy, AddedBy: tx.paidBy,
		}
		if tx.groupID != nil {
			e.GroupID = sql.NullInt64{Int64: *tx.groupID, Valid: true}
		}
		if _, err := st.CreateExpense(ctx, e, tx.parts); err != nil {
			t.Fatalf("create expense: %v", err)
		}
	}

	// netByUser[currency][user] from the balance view (sum of cumulated rows).
	net := map[string]map[int64]int64{"USD": {}, "EUR": {}}
	for u := int64(1); u <= 5; u++ {
		cum, err := st.CumulatedBalances(ctx, u)
		if err != nil {
			t.Fatalf("cumulated: %v", err)
		}
		for _, c := range cum {
			net[c.Currency][u] += c.Amount
		}
	}

	want := map[string]map[int64]int64{
		"USD": {1: 15746, 2: 22883, 3: 14200, 4: -8125, 5: -44704},
		"EUR": {1: -88782, 2: 5582, 3: 16578, 4: 47608, 5: 19014},
	}
	for cur, users := range want {
		for u, exp := range users {
			if net[cur][u] != exp {
				t.Errorf("netByUser[%s][U%d] = %d, want %d", cur, u, net[cur][u], exp)
			}
		}
	}

	// Conservation: per currency, sum of all balances == 0.
	for _, cur := range currencies {
		var sum int64
		for _, v := range net[cur] {
			sum += v
		}
		if sum != 0 {
			t.Errorf("conservation broken for %s: sum = %d", cur, sum)
		}
	}

	// Debt simplification preserves each node's net per currency.
	for _, cur := range currencies {
		ts := balance.Simplify(net[cur])
		after := map[int64]int64{}
		for u, v := range net[cur] {
			after[u] = v
		}
		for _, tr := range ts {
			after[tr.From] += tr.Amount
			after[tr.To] -= tr.Amount
		}
		for u, v := range after {
			if v != 0 {
				t.Errorf("%s: simplify left U%d at %d", cur, u, v)
			}
		}
	}
}
