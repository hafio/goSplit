package currency

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// covRoundTripFunc lets a closure stand in for the provider HTTP endpoint, so
// providers are exercised end-to-end without a network call. Mirrors the
// helper in internal/bank/provider_test.go (that one lives in package bank and
// cannot be reused across the package boundary).
type covRoundTripFunc func(*http.Request) (*http.Response, error)

func (f covRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func covJSONResp(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func covClient(fn covRoundTripFunc) *http.Client { return &http.Client{Transport: fn} }

func TestNewProviderSelection(t *testing.T) {
	cases := []struct {
		name     string
		wantName string
		wantType string // "none" | "frankfurter" | "oxr"
	}{
		{"none", "none", "none"},
		{"disabled", "none", "none"},
		{"DISABLED", "none", "none"},
		{"openexchangerates", "openexchangerates", "oxr"},
		{"oxr", "openexchangerates", "oxr"},
		{"OXR", "openexchangerates", "oxr"},
		{"frankfurter", "frankfurter", "frankfurter"},
		{"", "frankfurter", "frankfurter"},
		{"  ", "frankfurter", "frankfurter"},
		{"nonsense", "frankfurter", "frankfurter"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := NewProvider(tc.name, "app-id")
			if p.Name() != tc.wantName {
				t.Fatalf("NewProvider(%q).Name() = %q, want %q", tc.name, p.Name(), tc.wantName)
			}
			switch tc.wantType {
			case "none":
				if _, ok := p.(NoneProvider); !ok {
					t.Fatalf("NewProvider(%q) type = %T, want NoneProvider", tc.name, p)
				}
			case "oxr":
				oxr, ok := p.(*OpenExchangeRatesProvider)
				if !ok {
					t.Fatalf("NewProvider(%q) type = %T, want *OpenExchangeRatesProvider", tc.name, p)
				}
				if oxr.AppID != "app-id" {
					t.Errorf("AppID = %q, want app-id", oxr.AppID)
				}
				if oxr.HTTP == nil {
					t.Error("HTTP client not initialized")
				}
			case "frankfurter":
				fp, ok := p.(*FrankfurterProvider)
				if !ok {
					t.Fatalf("NewProvider(%q) type = %T, want *FrankfurterProvider", tc.name, p)
				}
				if fp.HTTP == nil {
					t.Error("HTTP client not initialized")
				}
			}
		})
	}
}

func TestNoneProviderRate(t *testing.T) {
	var np NoneProvider
	if np.Name() != "none" {
		t.Fatalf("Name() = %q, want none", np.Name())
	}
	_, err := np.Rate(context.Background(), "USD", "EUR", "")
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("Rate() err = %v, want disabled error", err)
	}
}

func TestFrankfurterSameCurrency(t *testing.T) {
	// from == to (case-insensitive) short-circuits before any HTTP call.
	p := &FrankfurterProvider{HTTP: covClient(func(*http.Request) (*http.Response, error) {
		t.Fatal("HTTP should not be called for same-currency conversion")
		return nil, nil
	})}
	got, err := p.Rate(context.Background(), "usd", "USD", "latest")
	if err != nil || got != "1" {
		t.Fatalf("Rate(usd->USD) = %q, %v, want 1, nil", got, err)
	}
}

func TestFrankfurterRateSuccess(t *testing.T) {
	var gotURL string
	p := &FrankfurterProvider{HTTP: covClient(func(r *http.Request) (*http.Response, error) {
		gotURL = r.URL.String()
		return covJSONResp(200, `{"rates":{"EUR":0.9}}`), nil
	})}
	got, err := p.Rate(context.Background(), "usd", "eur", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "0.9" {
		t.Fatalf("Rate = %q, want 0.9", got)
	}
	// Empty date -> "latest"; currencies upper-cased into the query.
	if !strings.Contains(gotURL, "/latest?") || !strings.Contains(gotURL, "from=USD") || !strings.Contains(gotURL, "to=EUR") {
		t.Errorf("url = %q, want latest with from=USD&to=EUR", gotURL)
	}
}

func TestFrankfurterRateWithDate(t *testing.T) {
	var gotURL string
	p := &FrankfurterProvider{HTTP: covClient(func(r *http.Request) (*http.Response, error) {
		gotURL = r.URL.String()
		return covJSONResp(200, `{"rates":{"EUR":0.8}}`), nil
	})}
	if _, err := p.Rate(context.Background(), "USD", "EUR", "2025-01-02"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotURL, "/2025-01-02?") {
		t.Errorf("url = %q, want the historical date path", gotURL)
	}
}

func TestFrankfurterRateStatusError(t *testing.T) {
	p := &FrankfurterProvider{HTTP: covClient(func(*http.Request) (*http.Response, error) {
		return covJSONResp(500, ``), nil
	})}
	_, err := p.Rate(context.Background(), "USD", "EUR", "")
	if err == nil || !strings.Contains(err.Error(), "frankfurter status 500") {
		t.Fatalf("err = %v, want frankfurter status 500", err)
	}
}

func TestFrankfurterRateMalformedJSON(t *testing.T) {
	p := &FrankfurterProvider{HTTP: covClient(func(*http.Request) (*http.Response, error) {
		return covJSONResp(200, `{not json`), nil
	})}
	if _, err := p.Rate(context.Background(), "USD", "EUR", ""); err == nil {
		t.Fatal("expected decode error for malformed JSON")
	}
}

func TestFrankfurterRateMissingRate(t *testing.T) {
	p := &FrankfurterProvider{HTTP: covClient(func(*http.Request) (*http.Response, error) {
		return covJSONResp(200, `{"rates":{"GBP":0.8}}`), nil
	})}
	_, err := p.Rate(context.Background(), "USD", "EUR", "")
	if err == nil || !strings.Contains(err.Error(), "no rate for USD->EUR") {
		t.Fatalf("err = %v, want no rate for USD->EUR", err)
	}
}

func TestFrankfurterRateTransportError(t *testing.T) {
	p := &FrankfurterProvider{HTTP: covClient(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("boom")
	})}
	_, err := p.Rate(context.Background(), "USD", "EUR", "")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v, want transport error", err)
	}
}

func TestOXRSameCurrency(t *testing.T) {
	p := &OpenExchangeRatesProvider{AppID: "id", HTTP: covClient(func(*http.Request) (*http.Response, error) {
		t.Fatal("HTTP should not be called for same-currency conversion")
		return nil, nil
	})}
	got, err := p.Rate(context.Background(), "eur", "EUR", "")
	if err != nil || got != "1" {
		t.Fatalf("Rate(eur->EUR) = %q, %v, want 1, nil", got, err)
	}
	if p.Name() != "openexchangerates" {
		t.Fatalf("Name() = %q, want openexchangerates", p.Name())
	}
}

func TestOXRMissingAppID(t *testing.T) {
	p := &OpenExchangeRatesProvider{AppID: "", HTTP: covClient(func(*http.Request) (*http.Response, error) {
		t.Fatal("HTTP should not be called when AppID is empty")
		return nil, nil
	})}
	_, err := p.Rate(context.Background(), "USD", "EUR", "")
	if err == nil || !strings.Contains(err.Error(), "OPEN_EXCHANGE_RATES_APP_ID not set") {
		t.Fatalf("err = %v, want app id not set error", err)
	}
}

func TestOXRRateSuccess(t *testing.T) {
	var gotURL string
	p := &OpenExchangeRatesProvider{AppID: "secret-id", HTTP: covClient(func(r *http.Request) (*http.Response, error) {
		gotURL = r.URL.String()
		// Cross rate derived from USD legs: toUSD/fromUSD = 4/2 = 2.
		return covJSONResp(200, `{"rates":{"EUR":2,"GBP":4}}`), nil
	})}
	got, err := p.Rate(context.Background(), "eur", "gbp", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "2" {
		t.Fatalf("Rate = %q, want 2", got)
	}
	if !strings.Contains(gotURL, "latest.json") || !strings.Contains(gotURL, "app_id=secret-id") {
		t.Errorf("url = %q, want latest.json with app_id", gotURL)
	}
}

func TestOXRRateWithDate(t *testing.T) {
	var gotURL string
	p := &OpenExchangeRatesProvider{AppID: "id", HTTP: covClient(func(r *http.Request) (*http.Response, error) {
		gotURL = r.URL.String()
		return covJSONResp(200, `{"rates":{"EUR":2,"GBP":4}}`), nil
	})}
	if _, err := p.Rate(context.Background(), "EUR", "GBP", "2025-01-02"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotURL, "historical/2025-01-02.json") {
		t.Errorf("url = %q, want the historical date endpoint", gotURL)
	}
}

func TestOXRRateMissingLeg(t *testing.T) {
	p := &OpenExchangeRatesProvider{AppID: "id", HTTP: covClient(func(*http.Request) (*http.Response, error) {
		return covJSONResp(200, `{"rates":{"EUR":2}}`), nil // no GBP leg
	})}
	_, err := p.Rate(context.Background(), "EUR", "GBP", "")
	if err == nil || !strings.Contains(err.Error(), "missing USD leg") {
		t.Fatalf("err = %v, want missing USD leg error", err)
	}
}

func TestOXRRateStatusError(t *testing.T) {
	p := &OpenExchangeRatesProvider{AppID: "id", HTTP: covClient(func(*http.Request) (*http.Response, error) {
		return covJSONResp(401, ``), nil
	})}
	_, err := p.Rate(context.Background(), "EUR", "GBP", "")
	if err == nil || !strings.Contains(err.Error(), "oxr status 401") {
		t.Fatalf("err = %v, want oxr status 401", err)
	}
}

func TestOXRRateMalformedJSON(t *testing.T) {
	p := &OpenExchangeRatesProvider{AppID: "id", HTTP: covClient(func(*http.Request) (*http.Response, error) {
		return covJSONResp(200, `{bad`), nil
	})}
	if _, err := p.Rate(context.Background(), "EUR", "GBP", ""); err == nil {
		t.Fatal("expected decode error for malformed JSON")
	}
}

func TestOXRRateTransportError(t *testing.T) {
	p := &OpenExchangeRatesProvider{AppID: "id", HTTP: covClient(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("boom")
	})}
	_, err := p.Rate(context.Background(), "EUR", "GBP", "")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v, want transport error", err)
	}
}
