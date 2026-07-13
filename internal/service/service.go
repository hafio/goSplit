// Package service holds the business logic layer. Handlers stay thin and call
// into Service; Service composes the store, split engine, mailer, and config.
package service

import (
	"time"

	"github.com/hafio/gosplit/internal/bank"
	"github.com/hafio/gosplit/internal/config"
	"github.com/hafio/gosplit/internal/currency"
	"github.com/hafio/gosplit/internal/mail"
	"github.com/hafio/gosplit/internal/push"
	"github.com/hafio/gosplit/internal/store"
)

// Service is the application's business-logic facade.
type Service struct {
	Store  *store.Store
	Mail   mail.Mailer
	Config *config.Config
	Rates  currency.Provider
	Push   *push.Sender
	Bank   bank.Provider

	// dispatch runs a background email send. It defaults to a goroutine so slow
	// SMTP never blocks an HTTP request; tests can make it synchronous for
	// deterministic mail assertions (see SetSynchronousEmail).
	dispatch func(func())
}

// New constructs a Service, selecting the currency rate + bank providers from
// config.
func New(s *store.Store, m mail.Mailer, cfg *config.Config) *Service {
	return &Service{
		Store:    s,
		Mail:     m,
		Config:   cfg,
		Rates:    currency.NewProvider(cfg.CurrencyRateProvider, cfg.OpenExchangeRatesAppID),
		Push:     push.New(cfg),
		Bank:     bank.New(cfg),
		dispatch: func(f func()) { go f() },
	}
}

// sendMailAsync runs an email send off the request path (or inline under
// SetSynchronousEmail).
func (s *Service) sendMailAsync(f func()) {
	if s.dispatch == nil {
		go f()
		return
	}
	s.dispatch(f)
}

// SetSynchronousEmail makes background email sends run inline instead of in a
// goroutine. Intended for tests so mailer assertions are deterministic.
func (s *Service) SetSynchronousEmail() { s.dispatch = func(f func()) { f() } }

// todayISO returns today's date as an ISO date string (UTC).
func todayISO() string { return time.Now().UTC().Format("2006-01-02") }
