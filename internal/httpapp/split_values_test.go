package httpapp

import (
	"errors"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/hafio/gosplit/internal/service"
	"github.com/hafio/gosplit/internal/split"
	"github.com/hafio/gosplit/internal/store"
)

// TestValueFormatRoundTrip: formatValue is the exact inverse of valueForMethod,
// so a restored edit form re-posts the digits it was rendered with.
func TestValueFormatRoundTrip(t *testing.T) {
	cases := []struct {
		method   split.Method
		typed    string
		currency string
	}{
		{split.PERCENTAGE, "33.33", "USD"},
		{split.PERCENTAGE, "33.33", "JPY"}, // percentages are always 2dp, not the expense currency
		{split.EXACT, "12.50", "USD"},
		{split.EXACT, "1250", "JPY"},
		{split.ADJUSTMENT, "-5.00", "USD"},
		{split.SHARE, "2", "USD"},
	}
	for _, c := range cases {
		v, err := valueForMethod(c.method, c.typed, c.currency)
		if err != nil {
			t.Errorf("%s %q: parse: %v", c.method, c.typed, err)
			continue
		}
		if got := formatValue(c.method, v, c.currency); got != c.typed {
			t.Errorf("%s %q in %s: round-trip = %q", c.method, c.typed, c.currency, got)
		}
	}
}

// TestValueForMethodEqualAndUnknown: EQUAL takes no value, and an unusable
// method is rejected rather than silently parsed.
func TestValueForMethodEqualAndUnknown(t *testing.T) {
	if v, err := valueForMethod(split.EQUAL, "ignored", "USD"); v != 0 || err != nil {
		t.Errorf("EQUAL: got %d, %v; want 0, nil", v, err)
	}
	if _, err := valueForMethod(split.Method("BOGUS"), "1", "USD"); err == nil {
		t.Error("unknown method accepted, want an error")
	}
	if got := formatValue(split.EQUAL, 500, "USD"); got != "" {
		t.Errorf("formatValue(EQUAL) = %q, want empty", got)
	}
	if got := formatValue(split.Method("BOGUS"), 500, "USD"); got != "" {
		t.Errorf("formatValue(unknown) = %q, want empty", got)
	}
}

// TestValueForMethodErrors covers each per-method parse failure.
func TestValueForMethodErrors(t *testing.T) {
	cases := []struct {
		method split.Method
		val    string
		want   string
	}{
		{split.PERCENTAGE, "abc", "invalid percentage"},
		{split.EXACT, "abc", "invalid exact amount"},
		{split.SHARE, "1.5", "invalid share units"},
		{split.ADJUSTMENT, "abc", "invalid adjustment"},
	}
	for _, c := range cases {
		_, err := valueForMethod(c.method, c.val, "USD")
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s %q: err = %v, want one containing %q", c.method, c.val, err, c.want)
		}
	}
}

// TestFieldErrorsFor maps errors onto the field the form should flag them at.
func TestFieldErrorsFor(t *testing.T) {
	if got := fieldErrorsFor(fieldErrorf("amount", "bad")); got["amount"] != "bad" {
		t.Errorf("tagged error = %v, want it on the amount field", got)
	}
	// The split engine's set-level failures belong under the participant list.
	for _, err := range []error{
		split.ErrPercentSum, split.ErrExactSum, split.ErrShareUnits,
		split.ErrNegativeBP, split.ErrNoParticipants, split.ErrDuplicateUser,
		service.ErrNotMember,
	} {
		if got := fieldErrorsFor(err); got[participantsField] == "" {
			t.Errorf("%v: not attached to the participants block", err)
		}
	}
	// A wrapped engine error still resolves.
	if got := fieldErrorsFor(errors.New("something else")); len(got) != 0 {
		t.Errorf("unrecognized error produced field errors: %v", got)
	}
}

// TestPrefillFromForm restores exactly what was submitted, including a
// deliberately unchecked participant.
func TestPrefillFromForm(t *testing.T) {
	cands := []candidate{{ID: 1, Included: true}, {ID: 2, Included: true}, {ID: 3, Included: true}}
	form := url.Values{
		"include_1": {"1"}, "value_1": {"10.00"},
		"value_3": {"5.00"}, // present but not included
	}
	r := httptest.NewRequest("POST", "/expenses", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	prefillFromForm(cands, r)

	if !cands[0].Included || cands[0].Value != "10.00" {
		t.Errorf("candidate 1 = %+v, want included with 10.00", cands[0])
	}
	if cands[1].Included || cands[1].Value != "" {
		t.Errorf("candidate 2 = %+v, want unchecked and empty", cands[1])
	}
	if cands[2].Included {
		t.Errorf("candidate 3 = %+v, want unchecked despite carrying a value", cands[2])
	}
}

// TestFormVersionFailsClosed: a form with no version token cannot match an
// already-edited row, so a stale page cannot overwrite one.
func TestFormVersionFailsClosed(t *testing.T) {
	r := httptest.NewRequest("POST", "/x", strings.NewReader(""))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if got := formVersion(r); got != 0 {
		t.Errorf("missing version = %d, want 0", got)
	}
	r2 := httptest.NewRequest("POST", "/x", strings.NewReader("version=7"))
	r2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if got := formVersion(r2); got != 7 {
		t.Errorf("version = %d, want 7", got)
	}
}

// TestSumTargetPerMethod: only the methods whose values the split engine
// constrains have a sum to reach.
func TestSumTargetPerMethod(t *testing.T) {
	e := &store.Expense{Amount: 2500}
	if got, ok := sumTarget(split.PERCENTAGE, e); !ok || got != split.BasisPointsFull {
		t.Errorf("PERCENTAGE = %d, %v; want %d, true", got, ok, split.BasisPointsFull)
	}
	if got, ok := sumTarget(split.EXACT, e); !ok || got != 2500 {
		t.Errorf("EXACT = %d, %v; want the expense total 2500, true", got, ok)
	}
	for _, m := range []split.Method{split.EQUAL, split.SHARE, split.ADJUSTMENT} {
		if _, ok := sumTarget(m, e); ok {
			t.Errorf("%s claims a fixed sum it does not have", m)
		}
	}
}

// TestRebalanceToTargetEdges: an empty set and an on-target set are both no-ops,
// so the helper is safe to call unconditionally, and an overshoot comes off the
// largest entry just as a shortfall goes onto it.
func TestRebalanceToTargetEdges(t *testing.T) {
	rebalanceToTarget(nil, split.BasisPointsFull) // must not panic

	onTarget := []int64{6000, 4000}
	rebalanceToTarget(onTarget, split.BasisPointsFull)
	if onTarget[0] != 6000 || onTarget[1] != 4000 {
		t.Errorf("an on-target set was adjusted anyway: %v", onTarget)
	}
	over := []int64{7000, 4000}
	rebalanceToTarget(over, split.BasisPointsFull)
	if over[0] != 6000 || over[1] != 4000 {
		t.Errorf("overshoot = %v, want the excess taken off the largest entry", over)
	}
}

// TestPrefillStoredRebalancesDepartedParticipant is the fix for a reachable
// dead end: unfriend someone (or drop them from the group) and they stop being
// selectable, so their stored value cannot be rendered. Replaying the rest
// verbatim left an EXACT form that no longer summed to the total, and saving it
// untouched was rejected with ErrExactSum. Both fixed-sum methods now close the
// gap on the largest remaining entry.
func TestPrefillStoredRebalancesDepartedParticipant(t *testing.T) {
	cases := []struct {
		method split.Method
		values map[int64]int64 // user 3 has since left
		want   map[int64]string
	}{
		// 30.00 entered as 10.00 / 5.00 / 15.00: the departed 15.00 is absorbed.
		{split.EXACT, map[int64]int64{1: 1000, 2: 500, 3: 1500}, map[int64]string{1: "25.00", 2: "5.00"}},
		// 50% / 20% / 30%: the departed 30% is absorbed to re-reach 100%.
		{split.PERCENTAGE, map[int64]int64{1: 5000, 2: 2000, 3: 3000}, map[int64]string{1: "80.00", 2: "20.00"}},
	}
	for _, c := range cases {
		cands := []candidate{{ID: 1}, {ID: 2}}
		e := &store.Expense{SplitType: string(c.method), Amount: 3000, Currency: "USD"}
		if got := prefillStored(cands, e, c.values); got != string(c.method) {
			t.Errorf("%s: preselected method = %q", c.method, got)
		}
		for _, cd := range cands {
			if !cd.Included {
				t.Errorf("%s: user %d came back unchecked despite a stored value", c.method, cd.ID)
			}
			if cd.Value != c.want[cd.ID] {
				t.Errorf("%s: user %d value = %q, want %q", c.method, cd.ID, cd.Value, c.want[cd.ID])
			}
		}
	}
}

// TestPrefillStoredKeepsCompleteSet: nobody left, nothing to close -- the digits
// come back exactly as typed rather than being nudged.
func TestPrefillStoredKeepsCompleteSet(t *testing.T) {
	cands := []candidate{{ID: 1}, {ID: 2}}
	e := &store.Expense{SplitType: string(split.EXACT), Amount: 3000, Currency: "USD"}
	prefillStored(cands, e, map[int64]int64{1: 1000, 2: 2000})
	if cands[0].Value != "10.00" || cands[1].Value != "20.00" {
		t.Errorf("a complete EXACT set was rebalanced anyway: %q / %q", cands[0].Value, cands[1].Value)
	}
}

// TestParseExpenseInputSortsLines pins the fix for a real defect: the parser
// walks r.Form, which is a map, so Go randomizes the order participants come
// out in. The split engine hands leftover minor units out by position, so an
// unsorted set let the odd cent land on a different person from one request to
// the next -- meaning an unchanged re-save could quietly move money.
func TestParseExpenseInputSortsLines(t *testing.T) {
	form := url.Values{
		"name": {"Dinner"}, "amount": {"10.01"}, "currency": {"USD"},
		"date": {"2026-01-02"}, "method": {"SHARE"}, "paid_by": {"1"},
		"include_3": {"1"}, "value_3": {"1"},
		"include_1": {"1"}, "value_1": {"1"},
		"include_2": {"1"}, "value_2": {"1"},
	}
	// Repeat: one pass could be sorted by luck, since the defect was a random
	// map walk.
	for attempt := 0; attempt < 20; attempt++ {
		r := httptest.NewRequest("POST", "/expenses", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		in, err := parseExpenseInput(r, 1)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if len(in.Lines) != 3 {
			t.Fatalf("lines = %d, want 3", len(in.Lines))
		}
		for i := 1; i < len(in.Lines); i++ {
			if in.Lines[i-1].UserID >= in.Lines[i].UserID {
				t.Fatalf("attempt %d: lines not in user-id order: %+v", attempt, in.Lines)
			}
		}
	}
}

// TestSplitRemainderStableAcrossLineOrder is the property that ordering buys:
// the same participants and total put the leftover unit on the same person
// however the caller happened to order them.
func TestSplitRemainderStableAcrossLineOrder(t *testing.T) {
	const total = 1001 // odd, so an equal split leaves one unit over
	ordered := []split.Line{{UserID: 1}, {UserID: 2}}
	reversed := []split.Line{{UserID: 2}, {UserID: 1}}

	sortLines := func(ls []split.Line) []split.Line {
		out := append([]split.Line(nil), ls...)
		sort.Slice(out, func(i, j int) bool { return out[i].UserID < out[j].UserID })
		return out
	}
	a, err := split.Compute(split.EQUAL, total, 1, "2026-01-02", sortLines(ordered))
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	b, err := split.Compute(split.EQUAL, total, 1, "2026-01-02", sortLines(reversed))
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	byUser := func(ps []split.Participant) map[int64]int64 {
		m := map[int64]int64{}
		for _, p := range ps {
			m[p.UserID] = p.Amount
		}
		return m
	}
	got, want := byUser(a), byUser(b)
	for uid, amt := range want {
		if got[uid] != amt {
			t.Errorf("user %d got %d one way and %d the other", uid, got[uid], amt)
		}
	}
}
