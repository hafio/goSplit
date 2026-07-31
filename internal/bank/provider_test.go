package bank

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hafio/gosplit/internal/config"
)

// roundTripFunc lets a closure stand in for the Plaid HTTP endpoint, so the
// provider is exercised end-to-end without a network call.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResp(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func plaidStub(fn roundTripFunc) *PlaidProvider {
	return &PlaidProvider{
		cfg: &config.Config{
			PlaidClientID: "cid", PlaidSecret: "sec",
			PlaidEnvironment: "sandbox", PlaidCountryCodes: []string{"US"},
		},
		http: &http.Client{Transport: fn},
	}
}

func TestNewDisabledProvider(t *testing.T) {
	p := New(&config.Config{}) // no Plaid credentials
	if p.Name() != "disabled" || p.Enabled() {
		t.Fatalf("New without creds = %q enabled=%v, want disabled/false", p.Name(), p.Enabled())
	}
	ctx := context.Background()
	if _, err := p.CreateLinkToken(ctx, "u"); err == nil {
		t.Error("disabled CreateLinkToken should error")
	}
	if _, err := p.ExchangePublicToken(ctx, "pt"); err == nil {
		t.Error("disabled ExchangePublicToken should error")
	}
	if _, err := p.FetchTransactions(ctx, "at", "2025-01-01", "2025-01-31"); err == nil {
		t.Error("disabled FetchTransactions should error")
	}
}

func TestNewPlaidProvider(t *testing.T) {
	p := New(&config.Config{PlaidClientID: "cid", PlaidSecret: "sec"})
	if p.Name() != "plaid" || !p.Enabled() {
		t.Fatalf("New with creds = %q enabled=%v, want plaid/true", p.Name(), p.Enabled())
	}
}

func TestBaseURL(t *testing.T) {
	cases := map[string]string{
		"production":  "https://production.plaid.com",
		"development": "https://development.plaid.com",
		"sandbox":     "https://sandbox.plaid.com",
		"":            "https://sandbox.plaid.com",
		"unexpected":  "https://sandbox.plaid.com",
	}
	for env, want := range cases {
		p := &PlaidProvider{cfg: &config.Config{PlaidEnvironment: env}}
		if got := p.baseURL(); got != want {
			t.Errorf("baseURL(env=%q) = %q, want %q", env, got, want)
		}
	}
}

func TestCreateLinkToken(t *testing.T) {
	var gotPath, gotBody string
	p := plaidStub(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		return jsonResp(200, `{"link_token":"lt-123"}`), nil
	})
	tok, err := p.CreateLinkToken(context.Background(), "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if tok != "lt-123" {
		t.Fatalf("link token = %q, want lt-123", tok)
	}
	if gotPath != "/link/token/create" {
		t.Errorf("path = %q, want /link/token/create", gotPath)
	}
	if !strings.Contains(gotBody, `"client_id":"cid"`) || !strings.Contains(gotBody, `"secret":"sec"`) {
		t.Errorf("creds not merged into request body: %s", gotBody)
	}
}

func TestExchangePublicToken(t *testing.T) {
	p := plaidStub(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/item/public_token/exchange" {
			t.Errorf("path = %q", r.URL.Path)
		}
		return jsonResp(200, `{"access_token":"at-9"}`), nil
	})
	tok, err := p.ExchangePublicToken(context.Background(), "public-tok")
	if err != nil || tok != "at-9" {
		t.Fatalf("access token = %q err = %v, want at-9/nil", tok, err)
	}
}

func TestFetchTransactions(t *testing.T) {
	p := plaidStub(func(r *http.Request) (*http.Response, error) {
		return jsonResp(200, `{"transactions":[
			{"transaction_id":"t1","name":"Coffee","amount":12.34,"iso_currency_code":"USD","date":"2025-01-02"},
			{"transaction_id":"t2","name":"Book","amount":5,"iso_currency_code":"","date":"2025-01-03"}
		]}`), nil
	})
	txns, err := p.FetchTransactions(context.Background(), "at", "2025-01-01", "2025-01-31")
	if err != nil {
		t.Fatal(err)
	}
	if len(txns) != 2 {
		t.Fatalf("got %d transactions, want 2", len(txns))
	}
	if txns[0].ID != "t1" || txns[0].AmountMinor != 1234 || txns[0].Currency != "USD" {
		t.Errorf("txn[0] = %+v, want t1/1234/USD", txns[0])
	}
	if txns[1].AmountMinor != 500 || txns[1].Currency != "USD" {
		t.Errorf("txn[1] = %+v, want 500 and USD (empty currency defaults to USD)", txns[1])
	}
}

func TestPostErrorStatus(t *testing.T) {
	p := plaidStub(func(r *http.Request) (*http.Response, error) {
		return jsonResp(400, `{"error_message":"invalid client"}`), nil
	})
	_, err := p.CreateLinkToken(context.Background(), "u")
	if err == nil || !strings.Contains(err.Error(), "invalid client") || !strings.Contains(err.Error(), "400") {
		t.Fatalf("err = %v, want the Plaid message and status 400", err)
	}
}

func TestPostTransportError(t *testing.T) {
	p := plaidStub(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("boom")
	})
	_, err := p.ExchangePublicToken(context.Background(), "pt")
	if err == nil || !strings.Contains(err.Error(), "plaid:") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v, want wrapped transport error", err)
	}
}
