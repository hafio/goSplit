package service

import (
	"context"
	"errors"
	"strings"

	"github.com/hafio/gosplit/internal/split"
	"github.com/hafio/gosplit/internal/store"
)

// GetRate returns the decimal rate for from->to on date (""=today), using the
// DB cache first and falling back to the configured provider (then caching).
func (s *Service) GetRate(ctx context.Context, from, to, date string) (string, error) {
	from, to = strings.ToUpper(from), strings.ToUpper(to)
	if from == to {
		return "1", nil
	}
	// A lookup without a date means "today's rate", so it is cached under
	// today's date: tomorrow fetches a fresh one, and the scheduler's purge,
	// which drops dates before its cutoff, can remove it.
	key := date
	if key == "" {
		key = todayISO()
	}
	if rate, err := s.Store.GetCachedRate(ctx, from, to, key); err == nil {
		return rate, nil
	} else if !errors.Is(err, store.ErrNotFound) { // cov:ignore store-fault: needs a failing database
		return "", err
	}
	rate, err := s.Rates.Rate(ctx, from, to, date)
	if err != nil {
		return "", err
	}
	_ = s.Store.PutCachedRate(ctx, from, to, key, rate)
	return rate, nil
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
	// Both legs move money between sender and receiver, so they get the same
	// checks as any expense: group members inside a group, related outside.
	if groupID != nil {
		if err := s.assertGroupMembers(ctx, *groupID, actor, sender, receiver); err != nil {
			return nil, err
		}
	} else if err := s.assertRelated(ctx, actor, sender, receiver); err != nil {
		return nil, err
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
	if err != nil { // cov:ignore store-fault: needs a failing database
		return nil, err
	}
	return s.Store.GetExpense(ctx, fromID)
}
