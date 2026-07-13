package money

import "testing"

func TestFormat(t *testing.T) {
	cases := []struct {
		minor    int64
		currency string
		want     string
	}{
		{1234, "USD", "12.34"},
		{-50, "USD", "-0.50"},
		{0, "USD", "0.00"},
		{5, "USD", "0.05"},
		{100, "JPY", "100"},
		{-2500, "JPY", "-2500"},
		{1234567, "BHD", "1234.567"},
		{1000000, "EUR", "10000.00"},
	}
	for _, c := range cases {
		if got := Format(c.minor, c.currency); got != c.want {
			t.Errorf("Format(%d,%s)=%q want %q", c.minor, c.currency, got, c.want)
		}
	}
}

func TestFormatWithCode(t *testing.T) {
	if got := FormatWithCode(1234, "usd"); got != "12.34 USD" {
		t.Fatalf("got %q", got)
	}
}

func TestParse(t *testing.T) {
	cases := []struct {
		in       string
		currency string
		want     int64
	}{
		{"12.34", "USD", 1234},
		{"12", "USD", 1200},
		{"0.05", "USD", 5},
		{"-1.50", "USD", -150},
		{"+2", "USD", 200},
		{"1,234.50", "USD", 123450},
		{"100", "JPY", 100},
		{".5", "USD", 50},
	}
	for _, c := range cases {
		got, err := Parse(c.in, c.currency)
		if err != nil {
			t.Errorf("Parse(%q,%s) error: %v", c.in, c.currency, err)
			continue
		}
		if got != c.want {
			t.Errorf("Parse(%q,%s)=%d want %d", c.in, c.currency, got, c.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	bad := []struct{ in, cur string }{
		{"", "USD"},
		{"12.345", "USD"}, // too many decimals
		{"abc", "USD"},
		{"1.2.3", "USD"},
	}
	for _, c := range bad {
		if _, err := Parse(c.in, c.cur); err == nil {
			t.Errorf("Parse(%q,%s) should error", c.in, c.cur)
		}
	}
}

func TestParseFormatRoundTrip(t *testing.T) {
	for _, cur := range []string{"USD", "JPY", "BHD"} {
		for _, minor := range []int64{0, 1, 99, 100, 12345, -678} {
			s := Format(minor, cur)
			got, err := Parse(s, cur)
			if err != nil {
				t.Fatalf("round-trip Parse(%q,%s): %v", s, cur, err)
			}
			if got != minor {
				t.Fatalf("round-trip %s %d -> %q -> %d", cur, minor, s, got)
			}
		}
	}
}
