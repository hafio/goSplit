package balance

import "sort"

// Row is one directed balance row as the balance view reports it: Amount > 0
// means FriendID owes UserID, in Currency. The view emits both directions of
// every pair, so PairsFromRows folds them back into one entry per pair.
type Row struct {
	UserID, FriendID int64
	Currency         string
	Amount           int64
}

// Pair is one unordered pair's balance in one currency, with A < B: Owed is how
// much B owes A (negative when A owes B).
type Pair struct {
	A, B     int64
	Currency string
	Owed     int64
}

// Shift is one two-row ledger entry that moves a single pair's balance: Payer's
// row is +Amount and Other's -Amount, so what Other owes Payer grows by Amount --
// exactly the effect of a settlement in which Payer pays Other. Amount is
// always positive.
type Shift struct {
	Payer, Other int64
	Currency     string
	Amount       int64
}

type pairKey struct {
	cur  string
	a, b int64
}

// PairsFromRows folds the view's directed rows into one Pair per unordered pair
// and currency (the two directions carry the same amount with opposite signs,
// so either one gives the same Pair). Zero pairs are dropped. The result is
// sorted by currency, then A, then B.
func PairsFromRows(rows []Row) []Pair {
	seen := map[pairKey]bool{}
	var out []Pair
	for _, r := range rows {
		a, b, owed := r.UserID, r.FriendID, r.Amount
		if a > b {
			a, b, owed = b, a, -owed
		}
		k := pairKey{r.Currency, a, b}
		if seen[k] {
			continue
		}
		seen[k] = true
		if owed != 0 {
			out = append(out, Pair{A: a, B: b, Currency: r.Currency, Owed: owed})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return lessKey(pairKey{out[i].Currency, out[i].A, out[i].B}, pairKey{out[j].Currency, out[j].A, out[j].B})
	})
	return out
}

// Nets returns each person's net position per currency (positive: is owed) --
// the input Simplify takes. Every pair adds and subtracts the same amount, so
// each currency's nets sum to zero.
func Nets(pairs []Pair) map[string]map[int64]int64 {
	out := map[string]map[int64]int64{}
	for _, p := range pairs {
		m := out[p.Currency]
		if m == nil {
			m = map[int64]int64{}
			out[p.Currency] = m
		}
		m[p.A] += p.Owed
		m[p.B] -= p.Owed
	}
	return out
}

// NeedsTidy reports whether a group is settled -- every net is zero -- yet some
// pair still shows a balance: the leftovers of paying simplified debts before
// netting existed, which Rewire would clear without changing what anyone owes.
func NeedsTidy(pairs []Pair) bool {
	if len(pairs) == 0 {
		return false
	}
	for _, net := range Nets(pairs) {
		for _, v := range net {
			if v != 0 {
				return false
			}
		}
	}
	return true
}

// Rewire returns the shifts that turn the given pair balances into the
// simplified debts Simplify derives from those same balances. Per currency, every
// pair whose balance differs from its target gets one shift of the difference --
// including a pair with no balance at all that the target links (A owes B and C
// owes D can simplify to A paying D). Simplify preserves every net, so each
// person's shifts sum to zero: applying them changes who owes whom, never how
// much anyone owes or is owed in total. When the pairs already are the
// simplified debts the result is empty, so applying it twice is the same as
// once. Shifts come sorted by currency, then by the pair's lower and higher id.
func Rewire(pairs []Pair) []Shift {
	diff := map[pairKey]int64{} // target minus current, per pair
	for _, p := range pairs {
		diff[pairKey{p.Currency, p.A, p.B}] -= p.Owed
	}
	for cur, net := range Nets(pairs) {
		for _, t := range Simplify(net) {
			// t.From owes t.To: in Pair terms B owes A when the debtor is B.
			a, b, owed := t.To, t.From, t.Amount
			if a > b {
				a, b, owed = b, a, -owed
			}
			diff[pairKey{cur, a, b}] += owed
		}
	}
	var out []Shift
	for k, d := range diff {
		switch {
		case d > 0: // B must owe A more: A "pays" B.
			out = append(out, Shift{Payer: k.a, Other: k.b, Currency: k.cur, Amount: d})
		case d < 0: // A must owe B more: B "pays" A.
			out = append(out, Shift{Payer: k.b, Other: k.a, Currency: k.cur, Amount: -d})
		}
	}
	sort.Slice(out, func(i, j int) bool { return lessKey(shiftKey(out[i]), shiftKey(out[j])) })
	return out
}

// shiftKey is the pair a shift moves, in pairKey order.
func shiftKey(s Shift) pairKey {
	a, b := s.Payer, s.Other
	if a > b {
		a, b = b, a
	}
	return pairKey{s.Currency, a, b}
}

func lessKey(x, y pairKey) bool {
	if x.cur != y.cur {
		return x.cur < y.cur
	}
	if x.a != y.a {
		return x.a < y.a
	}
	return x.b < y.b
}
