package split

import "testing"

// TestLineInputRoundTrip: every user-selectable method stores and restores its
// one relevant Line field, and touches no other.
func TestLineInputRoundTrip(t *testing.T) {
	cases := []struct {
		method Method
		value  int64
		field  func(Line) int64
	}{
		{PERCENTAGE, 3333, func(l Line) int64 { return l.BasisPoints }},
		{EXACT, 1250, func(l Line) int64 { return l.Exact }},
		{SHARE, 2, func(l Line) int64 { return l.ShareUnits }},
		{ADJUSTMENT, -500, func(l Line) int64 { return l.Adjustment }},
	}
	for _, c := range cases {
		l := LineFromInput(c.method, 7, c.value)
		if l.UserID != 7 {
			t.Errorf("%s: UserID = %d, want 7", c.method, l.UserID)
		}
		if got := c.field(l); got != c.value {
			t.Errorf("%s: stored field = %d, want %d", c.method, got, c.value)
		}
		if got := l.Input(c.method); got != c.value {
			t.Errorf("%s: Input round-trip = %d, want %d", c.method, got, c.value)
		}
		// Only the method's own field is populated.
		var sum int64
		for _, f := range []int64{l.BasisPoints, l.Exact, l.ShareUnits, l.Adjustment} {
			if f != 0 {
				sum++
			}
		}
		if sum != 1 {
			t.Errorf("%s: %d fields set, want exactly 1 (%+v)", c.method, sum, l)
		}
	}
}

// TestLineInputEqualAndUnknown: EQUAL carries no per-participant value, and an
// unrecognized method is inert rather than mis-assigning one.
func TestLineInputEqualAndUnknown(t *testing.T) {
	bare := Line{UserID: 3}
	full := Line{BasisPoints: 1, Exact: 2, ShareUnits: 3, Adjustment: 4}
	for _, m := range []Method{EQUAL, SETTLEMENT, Method("NOPE")} {
		l := LineFromInput(m, 3, 99)
		if l != bare {
			t.Errorf("%s: LineFromInput set a field: %+v", m, l)
		}
		if got := full.Input(m); got != 0 {
			t.Errorf("%s: Input = %d, want 0", m, got)
		}
	}
}

func TestUserSelectable(t *testing.T) {
	for _, m := range []Method{EQUAL, PERCENTAGE, EXACT, SHARE, ADJUSTMENT} {
		if !UserSelectable(m) {
			t.Errorf("%s should be user-selectable", m)
		}
	}
	for _, m := range []Method{SETTLEMENT, CURRENCY_CONVERSION, ARCHIVE, Method("")} {
		if UserSelectable(m) {
			t.Errorf("%s should not be user-selectable", m)
		}
	}
}

// TestEncodeDecodeInputs: a payload written for a method reads back as the same
// per-user values, keyed by user id.
func TestEncodeDecodeInputs(t *testing.T) {
	lines := []Line{
		LineFromInput(SHARE, 1, 2),
		LineFromInput(SHARE, 2, 1),
	}
	payload, err := EncodeInputs(SHARE, lines)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, ok := DecodeInputs(payload, SHARE)
	if !ok {
		t.Fatalf("decode reported unusable payload %q", payload)
	}
	if len(got) != 2 || got[1] != 2 || got[2] != 1 {
		t.Errorf("decoded = %v, want {1:2, 2:1}", got)
	}
}

// TestEncodeInputsSkipsSystemMethods: SETTLEMENT and friends never run through
// Compute, so there is nothing to record for them.
func TestEncodeInputsSkipsSystemMethods(t *testing.T) {
	for _, m := range []Method{SETTLEMENT, CURRENCY_CONVERSION, ARCHIVE} {
		if p, err := EncodeInputs(m, []Line{{UserID: 1}}); err != nil || p != "" {
			t.Errorf("%s: payload = %q, err = %v; want empty", m, p, err)
		}
	}
	if p, err := EncodeInputs(EQUAL, nil); err != nil || p != "" {
		t.Errorf("no lines: payload = %q, err = %v; want empty", p, err)
	}
}

// TestDecodeInputsRejects covers every path that must fall back to
// reconstructing values from the amounts rather than restoring a bad payload.
func TestDecodeInputsRejects(t *testing.T) {
	good, err := EncodeInputs(PERCENTAGE, []Line{LineFromInput(PERCENTAGE, 1, 10000)})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	cases := []struct {
		name    string
		payload string
		method  Method
	}{
		{"empty", "", PERCENTAGE},
		{"malformed", "{not json", PERCENTAGE},
		{"wrong version", `{"v":99,"method":"PERCENTAGE","values":{"1":10000}}`, PERCENTAGE},
		{"method mismatch", good, EXACT},
		{"no values", `{"v":1,"method":"PERCENTAGE","values":{}}`, PERCENTAGE},
		{"non-numeric key", `{"v":1,"method":"PERCENTAGE","values":{"alice":10000}}`, PERCENTAGE},
	}
	for _, c := range cases {
		if _, ok := DecodeInputs(c.payload, c.method); ok {
			t.Errorf("%s: payload accepted, want rejected", c.name)
		}
	}
	if _, ok := DecodeInputs(good, PERCENTAGE); !ok {
		t.Error("the control payload should decode")
	}
}
