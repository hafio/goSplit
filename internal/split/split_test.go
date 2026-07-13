package split

import (
	"errors"
	"testing"
)

// sumAmounts returns the total of all participant amounts (must be zero).
func sumAmounts(ps []Participant) int64 {
	var s int64
	for _, p := range ps {
		s += p.Amount
	}
	return s
}

// byUser indexes participant rows by user id.
func byUser(ps []Participant) map[int64]int64 {
	m := make(map[int64]int64, len(ps))
	for _, p := range ps {
		m[p.UserID] = p.Amount
	}
	return m
}

const date = "2025-01-02"

func TestCompute_TableDriven(t *testing.T) {
	tests := []struct {
		name   string
		method Method
		total  int64
		payer  int64
		lines  []Line
		want   map[int64]int64
	}{
		{
			name:   "equal 5-way even",
			method: EQUAL,
			total:  5000,
			payer:  1,
			lines:  []Line{{UserID: 1}, {UserID: 2}, {UserID: 3}, {UserID: 4}, {UserID: 5}},
			want:   map[int64]int64{1: 4000, 2: -1000, 3: -1000, 4: -1000, 5: -1000},
		},
		{
			name:   "exact matches golden seq3",
			method: EXACT,
			total:  15000,
			payer:  3,
			lines: []Line{
				{UserID: 1, Exact: 500}, {UserID: 2, Exact: 1500}, {UserID: 3, Exact: 2500},
				{UserID: 4, Exact: 3500}, {UserID: 5, Exact: 7000},
			},
			want: map[int64]int64{1: -500, 2: -1500, 3: 12500, 4: -3500, 5: -7000},
		},
		{
			name:   "percentage 70/30",
			method: PERCENTAGE,
			total:  10000,
			payer:  1,
			lines:  []Line{{UserID: 1, BasisPoints: 7000}, {UserID: 2, BasisPoints: 3000}},
			want:   map[int64]int64{1: 3000, 2: -3000},
		},
		{
			name:   "share 2:3",
			method: SHARE,
			total:  5000,
			payer:  2,
			lines:  []Line{{UserID: 1, ShareUnits: 2}, {UserID: 2, ShareUnits: 3}},
			want:   map[int64]int64{1: -2000, 2: 2000},
		},
		{
			name:   "adjustment",
			method: ADJUSTMENT,
			total:  3000,
			payer:  1,
			// base = (3000-500)/2 = 1250; shares: u1=1750, u2=1250
			lines: []Line{{UserID: 1, Adjustment: 500}, {UserID: 2, Adjustment: 0}},
			want:  map[int64]int64{1: 1250, 2: -1250},
		},
		{
			name:   "negative total refund equal",
			method: EQUAL,
			total:  -3000,
			payer:  1,
			lines:  []Line{{UserID: 1}, {UserID: 2}, {UserID: 3}},
			want:   map[int64]int64{1: -2000, 2: 1000, 3: 1000},
		},
		{
			name:   "payer not among splitters",
			method: EQUAL,
			total:  900,
			payer:  9,
			lines:  []Line{{UserID: 1}, {UserID: 2}, {UserID: 3}},
			want:   map[int64]int64{9: 900, 1: -300, 2: -300, 3: -300},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Compute(tc.method, tc.total, tc.payer, date, tc.lines)
			if err != nil {
				t.Fatalf("Compute error: %v", err)
			}
			if s := sumAmounts(got); s != 0 {
				t.Fatalf("amounts must sum to zero, got %d", s)
			}
			gm := byUser(got)
			if len(gm) != len(tc.want) {
				t.Fatalf("row count = %d, want %d (%v)", len(gm), len(tc.want), gm)
			}
			for u, want := range tc.want {
				if gm[u] != want {
					t.Errorf("user %d amount = %d, want %d", u, gm[u], want)
				}
			}
		})
	}
}

// TestRemainderDistribution verifies leftover minor units are distributed so
// the rows sum exactly to total, and deterministically (repeatable).
func TestRemainderDistribution(t *testing.T) {
	// 10 / 3 = 3 each, remainder 1.
	got, err := Compute(EQUAL, 10, 1, date, []Line{{UserID: 1}, {UserID: 2}, {UserID: 3}})
	if err != nil {
		t.Fatal(err)
	}
	if s := sumAmounts(got); s != 0 {
		t.Fatalf("sum = %d, want 0", s)
	}
	// Payer's own share is either 3 or 4; total-share must equal payer amount.
	// Re-run to confirm determinism.
	got2, _ := Compute(EQUAL, 10, 1, date, []Line{{UserID: 1}, {UserID: 2}, {UserID: 3}})
	if byUser(got)[1] != byUser(got2)[1] {
		t.Fatalf("non-deterministic remainder: %v vs %v", byUser(got), byUser(got2))
	}
}

// TestSeedIndependentOfOrder confirms the seed uses sorted shares, so input
// ordering does not change the result.
func TestSeedIndependentOfOrder(t *testing.T) {
	a, _ := Compute(EQUAL, 100, 1, date, []Line{{UserID: 1}, {UserID: 2}, {UserID: 3}})
	b, _ := Compute(EQUAL, 100, 1, date, []Line{{UserID: 3}, {UserID: 2}, {UserID: 1}})
	if byUser(a)[2] != byUser(b)[2] || byUser(a)[3] != byUser(b)[3] {
		t.Fatalf("order changed result: %v vs %v", byUser(a), byUser(b))
	}
}

func TestShareGCDNormalization(t *testing.T) {
	// units 500:1000 normalize to 1:2; total 3000 -> shares 1000 / 2000.
	// Payer(1) share 1000 -> amount = 3000-1000 = 2000; user2 share 2000 -> -2000.
	got, err := Compute(SHARE, 3000, 1, date, []Line{{UserID: 1, ShareUnits: 500}, {UserID: 2, ShareUnits: 1000}})
	if err != nil {
		t.Fatal(err)
	}
	m := byUser(got)
	if m[1] != 2000 || m[2] != -2000 {
		t.Fatalf("got %v, want payer(1)=2000 u2=-2000", m)
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name   string
		method Method
		total  int64
		lines  []Line
		want   error
	}{
		{"no participants", EQUAL, 100, nil, ErrNoParticipants},
		{"duplicate", EQUAL, 100, []Line{{UserID: 1}, {UserID: 1}}, ErrDuplicateUser},
		{"percent sum wrong", PERCENTAGE, 100, []Line{{UserID: 1, BasisPoints: 6000}, {UserID: 2, BasisPoints: 3000}}, ErrPercentSum},
		{"percent negative", PERCENTAGE, 100, []Line{{UserID: 1, BasisPoints: -100}, {UserID: 2, BasisPoints: 10100}}, ErrNegativeBP},
		{"exact sum wrong", EXACT, 100, []Line{{UserID: 1, Exact: 40}, {UserID: 2, Exact: 40}}, ErrExactSum},
		{"share zero sum", SHARE, 100, []Line{{UserID: 1, ShareUnits: 0}, {UserID: 2, ShareUnits: 0}}, ErrShareUnits},
		{"share negative", SHARE, 100, []Line{{UserID: 1, ShareUnits: -1}, {UserID: 2, ShareUnits: 2}}, ErrShareUnits},
		{"unknown method", Method("BOGUS"), 100, []Line{{UserID: 1}}, ErrUnknownMethod},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compute(tc.method, tc.total, 1, date, tc.lines)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

// TestProperty_ZeroSum is a lightweight property test: across many generated
// inputs and methods, participant amounts always sum to zero.
func TestProperty_ZeroSum(t *testing.T) {
	totals := []int64{0, 1, 7, 100, 999, 10000, -333, -1, 123456}
	sizes := []int{1, 2, 3, 5, 7}
	for _, total := range totals {
		for _, n := range sizes {
			lines := make([]Line, n)
			for i := range lines {
				lines[i] = Line{UserID: int64(i + 1), ShareUnits: int64(i + 1), BasisPoints: 0, Adjustment: int64(i * 10)}
			}
			// EQUAL
			assertZeroSum(t, EQUAL, total, lines)
			// SHARE
			assertZeroSum(t, SHARE, total, lines)
			// ADJUSTMENT
			assertZeroSum(t, ADJUSTMENT, total, lines)
			// PERCENTAGE with an even-ish split summing to 10000
			pl := make([]Line, n)
			each := int64(BasisPointsFull / n)
			var acc int64
			for i := range pl {
				bp := each
				if i == n-1 {
					bp = BasisPointsFull - acc
				}
				acc += each
				pl[i] = Line{UserID: int64(i + 1), BasisPoints: bp}
			}
			assertZeroSum(t, PERCENTAGE, total, pl)
			// EXACT: distribute total across lines exactly
			el := make([]Line, n)
			var used int64
			for i := range el {
				share := total / int64(n)
				if i == n-1 {
					share = total - used
				}
				used += total / int64(n)
				el[i] = Line{UserID: int64(i + 1), Exact: share}
			}
			assertZeroSum(t, EXACT, total, el)
		}
	}
}

func assertZeroSum(t *testing.T, m Method, total int64, lines []Line) {
	t.Helper()
	got, err := Compute(m, total, lines[0].UserID, date, lines)
	if err != nil {
		t.Fatalf("%s total=%d n=%d: %v", m, total, len(lines), err)
	}
	if s := sumAmounts(got); s != 0 {
		t.Fatalf("%s total=%d n=%d: sum=%d want 0 (%v)", m, total, len(lines), s, byUser(got))
	}
}

func TestDefaultSplitAllowed(t *testing.T) {
	allowed := []Method{EQUAL, PERCENTAGE, SHARE}
	denied := []Method{EXACT, ADJUSTMENT, SETTLEMENT, CURRENCY_CONVERSION}
	for _, m := range allowed {
		if !DefaultSplitAllowed(m) {
			t.Errorf("%s should be allowed for default splits", m)
		}
	}
	for _, m := range denied {
		if DefaultSplitAllowed(m) {
			t.Errorf("%s should NOT be allowed for default splits", m)
		}
	}
}
