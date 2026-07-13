package service

import (
	"context"
	"errors"
	"strings"

	"github.com/hafio/gosplit/internal/currency"
	"github.com/hafio/gosplit/internal/split"
	"github.com/hafio/gosplit/internal/store"
)

// GetRate returns the decimal rate for from->to on date (""=latest), using the
// DB cache first and falling back to the configured provider (then caching).
func (s *Service) GetRate(ctx context.Context, from, to, date string) (string, error) {
	from, to = strings.ToUpper(from), strings.ToUpper(to)
	if from == to {
		return "1", nil
	}
	key := date
	if key == "" {
		key = "latest"
	}
	if rate, err := s.Store.GetCachedRate(ctx, from, to, key); err == nil {
		return rate, nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return "", err
	}
	rate, err := s.Rates.Rate(ctx, from, to, date)
	if err != nil {
		return "", err
	}
	_ = s.Store.PutCachedRate(ctx, from, to, key, rate)
	return rate, nil
}

// GetRates looks up several targets from a single base (batch), caching each.
func (s *Service) GetRates(ctx context.Context, from string, to []string, date string) (map[string]string, error) {
	out := make(map[string]string, len(to))
	for _, t := range to {
		r, err := s.GetRate(ctx, from, t, date)
		if err != nil {
			return nil, err
		}
		out[t] = r
	}
	return out, nil
}

// CreateConversion converts a balance between two users at the current provider
// rate: it looks up from->to, computes the target amount with exact math, then
// delegates to CreateConversionExact.
func (s *Service) CreateConversion(ctx context.Context, actor, sender, receiver int64, amountFrom int64, fromCur, toCur string, date string, groupID *int64) (*store.Expense, error) {
	fromCur, toCur = strings.ToUpper(fromCur), strings.ToUpper(toCur)
	if fromCur == toCur {
		return nil, errors.New("conversion currencies must differ")
	}
	if amountFrom <= 0 {
		return nil, errors.New("conversion amount must be positive")
	}
	rate, err := s.GetRate(ctx, fromCur, toCur, date)
	if err != nil {
		return nil, err
	}
	amountTo, err := currency.Convert(amountFrom, fromCur, toCur, rate)
	if err != nil {
		return nil, err
	}
	return s.CreateConversionExact(ctx, actor, sender, receiver, amountFrom, amountTo, fromCur, toCur, date, groupID)
}

// CreateConversionExact converts a balance between two users using explicit,
// caller-supplied from/to amounts (the amounts the user saw and confirmed — no
// rate recomputation, no float drift). It creates the linked
// CURRENCY_CONVERSION pair (spec §5.1): the "from" leg in the source currency
// and the "to" leg in the target, so the balance shifts currencies with no net
// value change. sender is the creditor (the one owed) in both legs.
func (s *Service) CreateConversionExact(ctx context.Context, actor, sender, receiver int64, amountFrom, amountTo int64, fromCur, toCur string, date string, groupID *int64) (*store.Expense, error) {
	fromCur, toCur = strings.ToUpper(fromCur), strings.ToUpper(toCur)
	if fromCur == toCur {
		return nil, errors.New("conversion currencies must differ")
	}
	if amountFrom <= 0 || amountTo <= 0 {
		return nil, errors.New("conversion amounts must be positive")
	}
	if date == "" {
		date = todayISO()
	}
	fromExp := &store.Expense{
		ID: store.NewUUID(), Name: "Currency conversion", Category: "conversion",
		Amount: amountFrom, SplitType: string(split.CURRENCY_CONVERSION), ExpenseDate: date,
		Currency: fromCur, PaidBy: sender, AddedBy: actor, GroupID: nullInt(groupID),
	}
	fromParts := []store.ExpenseParticipant{
		{UserID: sender, Amount: amountFrom}, {UserID: receiver, Amount: -amountFrom},
	}
	toExp := &store.Expense{
		Name: "Currency conversion", Category: "conversion",
		Amount: amountTo, SplitType: string(split.CURRENCY_CONVERSION), ExpenseDate: date,
		Currency: toCur, PaidBy: receiver, AddedBy: actor, GroupID: nullInt(groupID),
	}
	toParts := []store.ExpenseParticipant{
		{UserID: sender, Amount: -amountTo}, {UserID: receiver, Amount: amountTo},
	}
	fromID, _, err := s.Store.CreateConversionPair(ctx, fromExp, fromParts, toExp, toParts)
	if err != nil {
		return nil, err
	}
	return s.Store.GetExpense(ctx, fromID)
}
