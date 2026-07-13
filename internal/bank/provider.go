// Package bank provides an optional, feature-flagged bank-transaction sync
// behind a pluggable Provider interface (so providers beyond Plaid can be added
// later). It is disabled cleanly when its credentials are unset.
package bank

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/hafio/gosplit/internal/config"
)

// Transaction is a provider-agnostic bank transaction (amounts in minor units).
type Transaction struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	AmountMinor int64  `json:"amountMinor"`
	Currency    string `json:"currency"`
	Date        string `json:"date"`
}

// Provider abstracts a bank-data source. New providers implement this interface.
type Provider interface {
	Name() string
	Enabled() bool
	CreateLinkToken(ctx context.Context, clientUserID string) (string, error)
	ExchangePublicToken(ctx context.Context, publicToken string) (accessToken string, err error)
	FetchTransactions(ctx context.Context, accessToken, startDate, endDate string) ([]Transaction, error)
}

// New selects a provider from config. Plaid is used when its client id + secret
// are present; otherwise a disabled provider is returned.
func New(cfg *config.Config) Provider {
	if cfg.PlaidClientID != "" && cfg.PlaidSecret != "" {
		return &PlaidProvider{cfg: cfg, http: &http.Client{Timeout: 20 * time.Second}}
	}
	slog.Info("bank: Plaid not configured, bank sync disabled")
	return Disabled{}
}

// Disabled is the no-op provider used when no bank credentials are set.
type Disabled struct{}

func (Disabled) Name() string   { return "disabled" }
func (Disabled) Enabled() bool  { return false }
func (Disabled) CreateLinkToken(context.Context, string) (string, error) {
	return "", fmt.Errorf("bank: sync is not configured on this server")
}
func (Disabled) ExchangePublicToken(context.Context, string) (string, error) {
	return "", fmt.Errorf("bank: sync is not configured on this server")
}
func (Disabled) FetchTransactions(context.Context, string, string, string) ([]Transaction, error) {
	return nil, fmt.Errorf("bank: sync is not configured on this server")
}

// PlaidProvider talks to the Plaid REST API.
type PlaidProvider struct {
	cfg  *config.Config
	http *http.Client
}

func (PlaidProvider) Name() string  { return "plaid" }
func (PlaidProvider) Enabled() bool { return true }

func (p *PlaidProvider) baseURL() string {
	switch p.cfg.PlaidEnvironment {
	case "production":
		return "https://production.plaid.com"
	case "development":
		return "https://development.plaid.com"
	default:
		return "https://sandbox.plaid.com"
	}
}

// creds returns the client_id/secret pair merged into a request body.
func (p *PlaidProvider) creds(extra map[string]any) map[string]any {
	body := map[string]any{"client_id": p.cfg.PlaidClientID, "secret": p.cfg.PlaidSecret}
	for k, v := range extra {
		body[k] = v
	}
	return body
}

func (p *PlaidProvider) post(ctx context.Context, path string, body map[string]any, out any) error {
	buf, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL()+path, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.http.Do(req)
	if err != nil {
		return fmt.Errorf("plaid: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		var e struct {
			ErrorMessage string `json:"error_message"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("plaid: %s (status %d)", e.ErrorMessage, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// CreateLinkToken creates a Plaid Link token for the client-side flow.
func (p *PlaidProvider) CreateLinkToken(ctx context.Context, clientUserID string) (string, error) {
	var out struct {
		LinkToken string `json:"link_token"`
	}
	err := p.post(ctx, "/link/token/create", p.creds(map[string]any{
		"user":          map[string]any{"client_user_id": clientUserID},
		"client_name":   "GoSplit",
		"products":      []string{"transactions"},
		"country_codes": p.cfg.PlaidCountryCodes,
		"language":      "en",
	}), &out)
	return out.LinkToken, err
}

// ExchangePublicToken swaps a public token for a long-lived access token.
func (p *PlaidProvider) ExchangePublicToken(ctx context.Context, publicToken string) (string, error) {
	var out struct {
		AccessToken string `json:"access_token"`
	}
	err := p.post(ctx, "/item/public_token/exchange", p.creds(map[string]any{
		"public_token": publicToken,
	}), &out)
	return out.AccessToken, err
}

// FetchTransactions returns transactions in [startDate, endDate] (YYYY-MM-DD).
func (p *PlaidProvider) FetchTransactions(ctx context.Context, accessToken, startDate, endDate string) ([]Transaction, error) {
	var out struct {
		Transactions []struct {
			ID       string  `json:"transaction_id"`
			Name     string  `json:"name"`
			Amount   float64 `json:"amount"`
			Currency string  `json:"iso_currency_code"`
			Date     string  `json:"date"`
		} `json:"transactions"`
	}
	err := p.post(ctx, "/transactions/get", p.creds(map[string]any{
		"access_token": accessToken,
		"start_date":   startDate,
		"end_date":     endDate,
		"options":      map[string]any{"count": 100, "offset": 0},
	}), &out)
	if err != nil {
		return nil, err
	}
	txns := make([]Transaction, 0, len(out.Transactions))
	for _, t := range out.Transactions {
		cur := t.Currency
		if cur == "" {
			cur = "USD"
		}
		txns = append(txns, Transaction{
			ID: t.ID, Name: t.Name, Currency: cur, Date: t.Date,
			AmountMinor: int64(math.Round(t.Amount * 100)),
		})
	}
	return txns, nil
}
