package currency

import "testing"

func TestConvert(t *testing.T) {
	tests := []struct {
		name              string
		amount            int64
		from, to, rate    string
		want              int64
	}{
		{"usd to eur golden 9/10", 10000, "USD", "EUR", "0.9", 9000},
		{"eur to usd", 9000, "EUR", "USD", "1.1111111111", 10000},
		{"same scale round half up", 100, "USD", "EUR", "1.005", 101},
		{"negative amount", -5000, "USD", "EUR", "0.9", -4500},
		{"usd(2dp) to jpy(0dp)", 10000, "USD", "JPY", "150", 15000}, // $100.00 * 150 = ¥15000
		{"jpy(0dp) to usd(2dp)", 15000, "JPY", "USD", "0.0066667", 10000},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Convert(tc.amount, tc.from, tc.to, tc.rate)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("Convert(%d, %s->%s, %s) = %d, want %d", tc.amount, tc.from, tc.to, tc.rate, got, tc.want)
			}
		})
	}
}

func TestConvertErrors(t *testing.T) {
	if _, err := Convert(100, "USD", "EUR", "abc"); err == nil {
		t.Fatal("expected error for invalid rate")
	}
	if _, err := Convert(100, "USD", "EUR", "-1"); err == nil {
		t.Fatal("expected error for negative rate")
	}
}
