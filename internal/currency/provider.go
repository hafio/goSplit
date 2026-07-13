package currency

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Provider fetches exchange rates. Implementations are swappable via config.
type Provider interface {
	// Rate returns the decimal rate string for 1 from = ? to on the given date
	// ("" or "latest" for the newest rate).
	Rate(ctx context.Context, from, to, date string) (string, error)
	// Name identifies the provider (for logging/cache provenance).
	Name() string
}

// NewProvider selects a provider by name. Unknown/empty names fall back to
// Frankfurter (free, no API key). "none" disables live lookups.
func NewProvider(name, openExchangeAppID string) Provider {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "none", "disabled":
		return NoneProvider{}
	case "openexchangerates", "oxr":
		return &OpenExchangeRatesProvider{AppID: openExchangeAppID, HTTP: defaultClient()}
	default:
		return &FrankfurterProvider{HTTP: defaultClient()}
	}
}

func defaultClient() *http.Client { return &http.Client{Timeout: 10 * time.Second} }

// NoneProvider always errors — used when live rates are disabled.
type NoneProvider struct{}

func (NoneProvider) Name() string { return "none" }
func (NoneProvider) Rate(context.Context, string, string, string) (string, error) {
	return "", fmt.Errorf("currency: live rate lookups are disabled")
}

// FrankfurterProvider uses the free frankfurter.app API (ECB rates).
type FrankfurterProvider struct{ HTTP *http.Client }

func (FrankfurterProvider) Name() string { return "frankfurter" }

func (p *FrankfurterProvider) Rate(ctx context.Context, from, to, date string) (string, error) {
	from, to = strings.ToUpper(from), strings.ToUpper(to)
	if from == to {
		return "1", nil
	}
	when := "latest"
	if date != "" && date != "latest" {
		when = date
	}
	url := fmt.Sprintf("https://api.frankfurter.app/%s?from=%s&to=%s", when, from, to)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := p.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("currency: frankfurter status %d", resp.StatusCode)
	}
	var body struct {
		Rates map[string]float64 `json:"rates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	v, ok := body.Rates[to]
	if !ok {
		return "", fmt.Errorf("currency: no rate for %s->%s", from, to)
	}
	return strconv.FormatFloat(v, 'f', -1, 64), nil
}

// OpenExchangeRatesProvider uses openexchangerates.org (requires an app id).
// Base is USD on the free plan; cross rates are derived from USD legs.
type OpenExchangeRatesProvider struct {
	AppID string
	HTTP  *http.Client
}

func (OpenExchangeRatesProvider) Name() string { return "openexchangerates" }

func (p *OpenExchangeRatesProvider) Rate(ctx context.Context, from, to, date string) (string, error) {
	from, to = strings.ToUpper(from), strings.ToUpper(to)
	if from == to {
		return "1", nil
	}
	if p.AppID == "" {
		return "", fmt.Errorf("currency: OPEN_EXCHANGE_RATES_APP_ID not set")
	}
	endpoint := "https://openexchangerates.org/api/latest.json"
	if date != "" && date != "latest" {
		endpoint = fmt.Sprintf("https://openexchangerates.org/api/historical/%s.json", date)
	}
	url := fmt.Sprintf("%s?app_id=%s", endpoint, p.AppID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := p.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("currency: oxr status %d", resp.StatusCode)
	}
	var body struct {
		Rates map[string]float64 `json:"rates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	fromUSD, ok1 := body.Rates[from]
	toUSD, ok2 := body.Rates[to]
	if !ok1 || !ok2 || fromUSD == 0 {
		return "", fmt.Errorf("currency: missing USD leg for %s or %s", from, to)
	}
	return strconv.FormatFloat(toUSD/fromUSD, 'f', -1, 64), nil
}
