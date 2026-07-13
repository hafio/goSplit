// Package split implements GoSplit's expense split engine.
//
// Given an expense total (int64 minor units), a payer, and the selected
// participants, it computes each participant's *share*, then converts shares
// into the zero-sum signed ExpenseParticipant amounts:
//
//	payer     amount = +(total - payerShare)   // creditor
//	everyone  amount = -(share)                // debtor
//
// so the rows always sum to exactly zero (the storage convention balances rely
// on, spec §4.1). Leftover minor units from integer division are distributed
// deterministically (a seeded shuffle keyed on the sorted base shares + the
// expense date) so results are reproducible and unit-testable (spec §5.1).
package split

import (
	"errors"
	"fmt"
	"hash/fnv"
	"sort"
)

// Method enumerates the split types. EQUAL, PERCENTAGE, EXACT, SHARE and
// ADJUSTMENT are user-selectable; SETTLEMENT and CURRENCY_CONVERSION are system
// methods handled elsewhere (they do not go through Compute).
type Method string

const (
	EQUAL              Method = "EQUAL"
	PERCENTAGE         Method = "PERCENTAGE"
	EXACT              Method = "EXACT"
	SHARE              Method = "SHARE"
	ADJUSTMENT         Method = "ADJUSTMENT"
	SETTLEMENT         Method = "SETTLEMENT"
	CURRENCY_CONVERSION Method = "CURRENCY_CONVERSION"
	// ARCHIVE marks a synthetic "Historical Transactions" expense created by the
	// collapse feature; its participant rows carry the net of the rows it
	// replaced. System method, not user-selectable, does not go through Compute.
	ARCHIVE Method = "ARCHIVE"
)

// BasisPointsFull is 100% expressed in basis points (used by PERCENTAGE).
const BasisPointsFull = 10000

// Line is one participant's method-specific input. Only the field relevant to
// the chosen method is read.
type Line struct {
	UserID      int64
	BasisPoints int64 // PERCENTAGE: hundredths of a percent; must sum to 10000
	Exact       int64 // EXACT: exact share in minor units; must sum to total
	ShareUnits  int64 // SHARE: integer weight (>= 0), at least one > 0
	Adjustment  int64 // ADJUSTMENT: fixed per-person adjustment in minor units
}

// Participant is one output row: a signed net amount in minor units.
type Participant struct {
	UserID int64
	Amount int64
}

var (
	ErrNoParticipants   = errors.New("split: at least one participant is required")
	ErrDuplicateUser    = errors.New("split: duplicate participant")
	ErrPercentSum       = errors.New("split: percentages must total 100% (10000 basis points)")
	ErrExactSum         = errors.New("split: exact shares must sum to the total")
	ErrShareUnits       = errors.New("split: share units must be non-negative with a positive sum")
	ErrUnknownMethod    = errors.New("split: unknown split method")
	ErrNegativeBP       = errors.New("split: percentages must be non-negative")
)

// DefaultSplitAllowed reports whether a method is permitted for friend/group
// default splits (restricted to EQUAL, PERCENTAGE, SHARE — spec §5.1).
func DefaultSplitAllowed(m Method) bool {
	switch m {
	case EQUAL, PERCENTAGE, SHARE:
		return true
	default:
		return false
	}
}

// Compute produces the signed, zero-sum participant rows for an expense.
//
//	total  — expense total in minor units (may be negative for refunds).
//	payer  — the user id who paid; always emitted as a row (share 0 if absent
//	         from lines, i.e. they paid but do not owe a share).
//	date   — the expense date (ISO string) mixed into the remainder seed.
//	lines  — the splitting participants and their method inputs.
func Compute(method Method, total, payer int64, date string, lines []Line) ([]Participant, error) {
	if len(lines) == 0 {
		return nil, ErrNoParticipants
	}
	seen := make(map[int64]bool, len(lines))
	for _, l := range lines {
		if seen[l.UserID] {
			return nil, ErrDuplicateUser
		}
		seen[l.UserID] = true
	}

	shares, err := computeShares(method, total, date, lines)
	if err != nil {
		return nil, err
	}
	return finalize(total, payer, lines, shares), nil
}

// computeShares returns the per-line share (aligned to lines) for a method.
// Every branch guarantees sum(shares) == total via deterministic remainder
// distribution, so finalize yields zero-sum rows.
func computeShares(method Method, total int64, date string, lines []Line) ([]int64, error) {
	n := len(lines)
	shares := make([]int64, n)

	switch method {
	case EQUAL:
		base := total / int64(n)
		for i := range shares {
			shares[i] = base
		}
		distributeRemainder(shares, total, date)

	case PERCENTAGE:
		var sum int64
		for _, l := range lines {
			if l.BasisPoints < 0 {
				return nil, ErrNegativeBP
			}
			sum += l.BasisPoints
		}
		if sum != BasisPointsFull {
			return nil, fmt.Errorf("%w (got %d)", ErrPercentSum, sum)
		}
		for i, l := range lines {
			shares[i] = l.BasisPoints * total / BasisPointsFull
		}
		distributeRemainder(shares, total, date)

	case EXACT:
		var sum int64
		for i, l := range lines {
			shares[i] = l.Exact
			sum += l.Exact
		}
		if sum != total {
			return nil, fmt.Errorf("%w (got %d, want %d)", ErrExactSum, sum, total)
		}

	case SHARE:
		units := make([]int64, n)
		var sumUnits int64
		for i, l := range lines {
			if l.ShareUnits < 0 {
				return nil, ErrShareUnits
			}
			units[i] = l.ShareUnits
			sumUnits += l.ShareUnits
		}
		if sumUnits <= 0 {
			return nil, ErrShareUnits
		}
		// Normalize by GCD (does not change the ratio, keeps numbers small).
		g := int64(0)
		for _, u := range units {
			g = gcd(g, u)
		}
		if g > 1 {
			for i := range units {
				units[i] /= g
			}
			sumUnits /= g
		}
		for i := range shares {
			shares[i] = units[i] * total / sumUnits
		}
		distributeRemainder(shares, total, date)

	case ADJUSTMENT:
		var sumAdj int64
		for _, l := range lines {
			sumAdj += l.Adjustment
		}
		base := (total - sumAdj) / int64(n)
		for i, l := range lines {
			shares[i] = base + l.Adjustment
		}
		distributeRemainder(shares, total, date)

	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownMethod, method)
	}

	return shares, nil
}

// finalize converts shares into zero-sum signed participant rows, ensuring the
// payer is always represented (share 0 if not among the splitters).
func finalize(total, payer int64, lines []Line, shares []int64) []Participant {
	out := make([]Participant, 0, len(lines)+1)
	var payerShare int64
	payerPresent := false
	for i, l := range lines {
		if l.UserID == payer {
			payerShare = shares[i]
			payerPresent = true
			continue
		}
		out = append(out, Participant{UserID: l.UserID, Amount: -shares[i]})
	}
	// Payer row (creditor): total minus their own share.
	payerRow := Participant{UserID: payer, Amount: total - payerShare}
	_ = payerPresent
	// Keep deterministic ordering: payer first, then the others in input order.
	return append([]Participant{payerRow}, out...)
}

// distributeRemainder adjusts shares in place so they sum exactly to total.
// The leftover (total - sum(shares)) is applied one minor unit at a time in a
// deterministic, seeded order (seed = hash of the sorted base shares + date),
// making the outcome reproducible regardless of participant ordering.
func distributeRemainder(shares []int64, total int64, date string) {
	var sum int64
	for _, s := range shares {
		sum += s
	}
	remainder := total - sum
	if remainder == 0 {
		return
	}
	order := seededOrder(shares, date)
	i := 0
	for remainder > 0 {
		shares[order[i%len(order)]]++
		remainder--
		i++
	}
	for remainder < 0 {
		shares[order[i%len(order)]]--
		remainder++
		i++
	}
}

// seededOrder returns a deterministic permutation of participant indices. The
// seed is derived from the sorted base shares plus the expense date, so the
// order does not depend on the participants' input ordering.
func seededOrder(shares []int64, date string) []int {
	sorted := make([]int64, len(shares))
	copy(sorted, shares)
	sort.Slice(sorted, func(a, b int) bool { return sorted[a] < sorted[b] })

	h := fnv.New64a()
	var buf [8]byte
	for _, s := range sorted {
		u := uint64(s)
		for b := 0; b < 8; b++ {
			buf[b] = byte(u >> (8 * b))
		}
		_, _ = h.Write(buf[:])
	}
	_, _ = h.Write([]byte(date))

	rng := splitmix64(h.Sum64())
	order := make([]int, len(shares))
	for i := range order {
		order[i] = i
	}
	// Fisher-Yates with the deterministic PRNG.
	for i := len(order) - 1; i > 0; i-- {
		j := int(rng() % uint64(i+1))
		order[i], order[j] = order[j], order[i]
	}
	return order
}

// splitmix64 returns a deterministic PRNG closure seeded by s.
func splitmix64(s uint64) func() uint64 {
	return func() uint64 {
		s += 0x9E3779B97F4A7C15
		z := s
		z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
		z = (z ^ (z >> 27)) * 0x94D049BB133111EB
		return z ^ (z >> 31)
	}
}

func gcd(a, b int64) int64 {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
