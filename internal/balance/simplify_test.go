package balance

import "testing"

// netAfter applies transfers to the starting net and returns the resulting net.
func netAfter(start map[int64]int64, ts []Transfer) map[int64]int64 {
	out := map[int64]int64{}
	for k, v := range start {
		out[k] = v
	}
	for _, t := range ts {
		out[t.From] += t.Amount // debtor moves toward zero (net was negative)
		out[t.To] -= t.Amount   // creditor moves toward zero (net was positive)
	}
	return out
}

func TestSimplify_PreservesNetAndSettles(t *testing.T) {
	cases := []map[int64]int64{
		{1: 100, 2: -100},
		{1: 300, 2: -100, 3: -200},
		{1: 15746, 2: 22883, 3: 14200, 4: -8125, 5: -44704}, // golden USD net
		{1: -88782, 2: 5582, 3: 16578, 4: 47608, 5: 19014},  // golden EUR net
		{},
		{1: 0, 2: 0},
	}
	for _, net := range cases {
		ts := Simplify(net)
		after := netAfter(net, ts)
		for id, v := range after {
			if v != 0 {
				t.Fatalf("net not settled for %d: %d (transfers %v)", id, v, ts)
			}
		}
		// No participant both pays and receives.
		pays, recvs := map[int64]bool{}, map[int64]bool{}
		for _, tr := range ts {
			pays[tr.From] = true
			recvs[tr.To] = true
			if tr.Amount <= 0 {
				t.Fatalf("non-positive transfer amount: %v", tr)
			}
		}
		for id := range pays {
			if recvs[id] {
				t.Fatalf("participant %d both pays and receives", id)
			}
		}
	}
}

func TestSimplify_GoldenUSDTransactionCount(t *testing.T) {
	// The committed reference is 4 transactions for the 5-party USD net.
	net := map[int64]int64{1: 15746, 2: 22883, 3: 14200, 4: -8125, 5: -44704}
	ts := Simplify(net)
	if len(ts) != 4 {
		t.Fatalf("want 4 transfers, got %d: %v", len(ts), ts)
	}
	// Naive per-pair upper bound: never exceed nodes-1 transfers here.
	if len(ts) > len(net)-1 {
		t.Fatalf("more transfers than n-1: %d", len(ts))
	}
}
