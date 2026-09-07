// Package service holds the business logic layer. Handlers stay thin and call
// into Service; Service composes the store, split engine, mailer, and config.
package service

import (
	"log/slog"
	"time"

	"github.com/hafio/gosplit/internal/bank"
	"github.com/hafio/gosplit/internal/config"
	"github.com/hafio/gosplit/internal/currency"
	"github.com/hafio/gosplit/internal/i18n"
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

	// I18n localizes notification push and email text in each recipient's own
	// preferred_language, rather than in whatever language the actor was using.
	// Nil if the embedded catalogs fail to load, in which case t() returns the
	// key -- visible but never blank, the same contract as web.ViewData.T.
	I18n *i18n.Bundle

	// dispatch runs a background email send. It defaults to a goroutine so slow
	// SMTP never blocks an HTTP request; tests can make it synchronous for
	// deterministic mail assertions (see SetSynchronousEmail).
	dispatch func(func())
}

// New constructs a Service, selecting the currency rate + bank providers from
// config.
func New(s *store.Store, m mail.Mailer, cfg *config.Config) *Service {
	// A broken embed is the only way this fails, and a service that cannot
	// localize a notification is still a service that can record one -- so warn
	// and degrade rather than refuse to start.
	bundle, err := i18n.Load()
	if err != nil {
		slog.Warn("service: load locales, notification text falls back to keys", "err", err)
	}
	return &Service{
		Store:    s,
		Mail:     m,
		Config:   cfg,
		Rates:    currency.NewProvider(cfg.CurrencyRateProvider, cfg.OpenExchangeRatesAppID),
		Push:     push.New(cfg),
		Bank:     bank.New(cfg),
		I18n:     bundle,
		dispatch: func(f func()) { go f() },
	}
}

// t translates key in lang, tolerating a nil bundle.
func (s *Service) t(lang, key string) string {
	if s.I18n == nil {
		return key
	}
	return s.I18n.T(lang, key)
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
