// Package balance implements debt simplification (min-cash-flow) over the
// per-currency net positions derived from the balance view. It preserves each
// participant's net exactly while minimizing the number of transfers, and never
// makes a participant both pay and receive (spec §9).
package balance

import "sort"

// Transfer is one settlement payment: From pays To the amount (minor units).
type Transfer struct {
	From   int64
	To     int64
	Amount int64
}

// Simplify greedily reduces a set of per-user net positions (in one currency)
// to a minimal set of transfers. Positive net = the user is owed; negative =
// the user owes. The result is deterministic: creditors and debtors are ordered
// by descending magnitude then ascending id, matching the reference generator.
func Simplify(net map[int64]int64) []Transfer {
	type node struct {
		id  int64
		amt int64
	}
	var creditors, debtors []node
	for id, v := range net {
		switch {
		case v > 0:
			creditors = append(creditors, node{id, v})
		case v < 0:
			debtors = append(debtors, node{id, -v})
		}
	}
	less := func(s []node) func(i, j int) bool {
		return func(i, j int) bool {
			if s[i].amt != s[j].amt {
				return s[i].amt > s[j].amt
			}
			return s[i].id < s[j].id
		}
	}
	sort.Slice(creditors, less(creditors))
	sort.Slice(debtors, less(debtors))

	var res []Transfer
	ci, di := 0, 0
	for ci < len(creditors) && di < len(debtors) {
		pay := creditors[ci].amt
		if debtors[di].amt < pay {
			pay = debtors[di].amt
		}
		res = append(res, Transfer{From: debtors[di].id, To: creditors[ci].id, Amount: pay})
		creditors[ci].amt -= pay
		debtors[di].amt -= pay
		if creditors[ci].amt == 0 {
			ci++
		}
		if debtors[di].amt == 0 {
			di++
		}
	}
	return res
}
