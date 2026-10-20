package balance

import (
	"fmt"
	"math/rand"
	"testing"
)

// apply returns the pair balances after the shifts are recorded: a shift is a
// two-row entry, Payer +Amount and Other -Amount, which raises what Other owes
// Payer by Amount.
func apply(pairs []Pair, shifts []Shift) map[pairKey]int64 {
	out := map[pairKey]int64{}
	for _, p := range pairs {
		out[pairKey{p.Currency, p.A, p.B}] += p.Owed
	}
	for _, s := range shifts {
		if s.Payer < s.Other {
			out[pairKey{s.Currency, s.Payer, s.Other}] += s.Amount
		} else {
			out[pairKey{s.Currency, s.Other, s.Payer}] -= s.Amount
		}
	}
	return out
}

// toPairs turns an applied map back into pairs (zeros dropped).
func toPairs(m map[pairKey]int64) []Pair {
	var out []Pair
	for k, v := range m {
		if v != 0 {
			out = append(out, Pair{A: k.a, B: k.b, Currency: k.cur, Owed: v})
		}
	}
	return out
}

// target is the simplified debts of pairs, in the same pair form.
func target(pairs []Pair) map[pairKey]int64 {
	out := map[pairKey]int64{}
	for cur, net := range Nets(pairs) {
		for _, t := range Simplify(net) {
			if t.To < t.From {
				out[pairKey{cur, t.To, t.From}] += t.Amount
			} else {
				out[pairKey{cur, t.From, t.To}] -= t.Amount
			}
		}
	}
	return out
}

// nonZeroNets is Nets without the zero entries, so two pair sets with the same
// nets compare equal even when one of them no longer mentions a settled person.
func nonZeroNets(pairs []Pair) string {
	out := map[string]map[int64]int64{}
	for cur, m := range Nets(pairs) {
		for id, v := range m {
			if v == 0 {
				continue
			}
			if out[cur] == nil {
				out[cur] = map[int64]int64{}
			}
			out[cur][id] = v
		}
	}
	return fmt.Sprint(out)
}

func sameNonZero(a, b map[pairKey]int64) bool {
	for k, v := range a {
		if v != b[k] {
			return false
		}
	}
	for k, v := range b {
		if v != a[k] {
			return false
		}
	}
	return true
}

func shiftsEqual(t *testing.T, got, want []Shift) {
	t.Helper()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("shifts = %v, want %v", got, want)
	}
}

// TestRewireChainCycle is the case netting exists for: A owes B 50 and B owes C
// 50, the group pays the simplified A -> C 50, and the pairs are left as a cycle
// (A owes B, B owes C, C owes A) although every net is zero. Rewire clears all
// three pairs and nothing else.
func TestRewireChainCycle(t *testing.T) {
	const a, b, c = 1, 2, 3
	pairs := []Pair{
		{A: a, B: b, Currency: "USD", Owed: -50}, // A owes B
		{A: b, B: c, Currency: "USD", Owed: -50}, // B owes C
		{A: a, B: c, Currency: "USD", Owed: 50},  // C owes A (A paid C)
	}
	if !NeedsTidy(pairs) {
		t.Error("a settled cycle should need tidying")
	}
	shiftsEqual(t, Rewire(pairs), []Shift{
		{Payer: a, Other: b, Currency: "USD", Amount: 50},
		{Payer: c, Other: a, Currency: "USD", Amount: 50},
		{Payer: b, Other: c, Currency: "USD", Amount: 50},
	})
	if after := toPairs(apply(pairs, Rewire(pairs))); len(after) != 0 {
		t.Errorf("pairs after rewiring = %v, want all settled", after)
	}
}

// TestRewireSignFlip: A paid C only 20 of the 50 the simplified view asked for.
// The pair A-C must flip from "C owes A 20" to "A owes C 30", which takes one
// shift of 50 the other way, while A-B and B-C clear.
func TestRewireSignFlip(t *testing.T) {
	const a, b, c = 1, 2, 3
	pairs := []Pair{
		{A: a, B: b, Currency: "USD", Owed: -50},
		{A: b, B: c, Currency: "USD", Owed: -50},
		{A: a, B: c, Currency: "USD", Owed: 20},
	}
	if NeedsTidy(pairs) {
		t.Error("a group with debts outstanding does not need tidying")
	}
	shiftsEqual(t, Rewire(pairs), []Shift{
		{Payer: a, Other: b, Currency: "USD", Amount: 50},
		{Payer: c, Other: a, Currency: "USD", Amount: 50},
		{Payer: b, Other: c, Currency: "USD", Amount: 50},
	})
	after := apply(pairs, Rewire(pairs))
	if want := (map[pairKey]int64{{"USD", a, c}: -30}); !sameNonZero(after, want) {
		t.Errorf("pairs after rewiring = %v, want only A owes C 30", after)
	}
}

// TestRewireTargetOnlyPair: the simplified debts can link two people who share
// no balance yet (A owes B, C owes B, C owes D simplifies to C -> B and A -> D),
// and that pair needs a shift although it has no row to start from.
func TestRewireTargetOnlyPair(t *testing.T) {
	const a, b, c, d = 1, 2, 3, 4
	pairs := []Pair{
		{A: a, B: b, Currency: "EUR", Owed: -50},  // A owes B
		{A: b, B: c, Currency: "EUR", Owed: 50},   // C owes B
		{A: c, B: d, Currency: "EUR", Owed: -50},  // C owes D
	}
	after := apply(pairs, Rewire(pairs))
	want := map[pairKey]int64{{"EUR", b, c}: 100, {"EUR", a, d}: -50}
	if !sameNonZero(after, want) {
		t.Fatalf("pairs after rewiring = %v, want C owes B 100 and A owes D 50", after)
	}
}

// TestRewireIsIdempotentAndKeepsNets is the property the ledger relies on, over
// random multi-currency groups (including a zero-decimal currency): after the
// shifts every net is unchanged, every pair equals its simplified target, each
// person's shifts sum to zero, and rewiring the result asks for nothing more.
func TestRewireIsIdempotentAndKeepsNets(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for round := 0; round < 300; round++ {
		var pairs []Pair
		for _, cur := range []string{"USD", "JPY"} {
			n := 2 + rng.Intn(5)
			for a := int64(1); a <= int64(n); a++ {
				for b := a + 1; b <= int64(n); b++ {
					if rng.Intn(3) == 0 {
						continue
					}
					if v := int64(rng.Intn(20001) - 10000); v != 0 {
						pairs = append(pairs, Pair{A: a, B: b, Currency: cur, Owed: v})
					}
				}
			}
		}
		shifts := Rewire(pairs)

		perPerson := map[string]map[int64]int64{}
		for _, s := range shifts {
			if s.Amount <= 0 {
				t.Fatalf("round %d: non-positive shift %v", round, s)
			}
			if perPerson[s.Currency] == nil {
				perPerson[s.Currency] = map[int64]int64{}
			}
			perPerson[s.Currency][s.Payer] += s.Amount
			perPerson[s.Currency][s.Other] -= s.Amount
		}
		for cur, m := range perPerson {
			for id, v := range m {
				if v != 0 {
					t.Fatalf("round %d: %s shifts for %d sum to %d, want 0", round, cur, id, v)
				}
			}
		}

		after := toPairs(apply(pairs, shifts))
		if nonZeroNets(after) != nonZeroNets(pairs) {
			t.Fatalf("round %d: nets changed: %s -> %s", round, nonZeroNets(pairs), nonZeroNets(after))
		}
		if !sameNonZero(apply(after, nil), target(pairs)) {
			t.Fatalf("round %d: pairs are not the simplified debts", round)
		}
		if again := Rewire(after); len(again) != 0 {
			t.Fatalf("round %d: second rewire asked for %v", round, again)
		}
		if fmt.Sprint(Rewire(pairs)) != fmt.Sprint(shifts) {
			t.Fatalf("round %d: Rewire is not deterministic", round)
		}
	}
}

// TestPairsFromRowsFoldsBothDirections: the view reports every pair twice, once
// from each side; folding keeps one entry with A < B and drops zero pairs.
func TestPairsFromRowsFoldsBothDirections(t *testing.T) {
	rows := []Row{
		{UserID: 2, FriendID: 1, Currency: "USD", Amount: 30}, // 1 owes 2 30
		{UserID: 1, FriendID: 2, Currency: "USD", Amount: -30},
		{UserID: 3, FriendID: 1, Currency: "USD", Amount: 0},
		{UserID: 1, FriendID: 3, Currency: "EUR", Amount: 5},
		{UserID: 3, FriendID: 1, Currency: "EUR", Amount: -5},
	}
	got := PairsFromRows(rows)
	want := []Pair{{A: 1, B: 3, Currency: "EUR", Owed: 5}, {A: 1, B: 2, Currency: "USD", Owed: -30}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("PairsFromRows = %v, want %v", got, want)
	}
	if NeedsTidy(nil) {
		t.Error("no pairs: nothing to tidy")
	}
}
