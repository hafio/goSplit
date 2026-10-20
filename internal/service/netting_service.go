package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/hafio/gosplit/internal/balance"
	"github.com/hafio/gosplit/internal/split"
	"github.com/hafio/gosplit/internal/store"
)

// nettingName is the stored name of a netting entry. Like "Settlement" it is
// stored in English (search and the export read it); feeds and the detail page
// show the translated label instead.
const nettingName = "Group debts simplified"

// nettingTimeout bounds one netting pass. It runs after the settlement it follows
// has committed and is detached from the request's own context, so a client that
// disconnects in between cannot make it skip.
const nettingTimeout = 10 * time.Second

// netGroup rewires the group's pair balances to its simplified debts by writing
// NETTING entries -- two rows each, one per pair that has to move (see
// balance.Rewire). Nobody's net changes: each person's entries sum to zero, so
// the group page, the /groups list and every total read exactly as before, while
// Balances and friend pages stop showing debts that only cancel out. The read
// and the writes are one transaction (Store.ReplanGroup), which is what makes
// that hold even with other writes racing it. With onlySettled, nothing is
// written unless every net in the group is zero: that is the tidy-up, which must
// not reshape debts that are still owed. date is the entries' expense date
// (today when empty) and txID ties them to the settlement that caused them.
// Returns how many entries were written; rerunning on its own output writes none.
func (s *Service) netGroup(ctx context.Context, groupID int64, date string, actor int64, txID string, onlySettled bool) (int, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), nettingTimeout)
	defer cancel()
	if date == "" {
		date = time.Now().UTC().Format("2006-01-02")
	}
	gid := groupID
	return s.Store.ReplanGroup(ctx, groupID, func(bals []store.Balance) []store.PlannedExpense {
		rows := make([]balance.Row, 0, len(bals))
		for _, b := range bals {
			rows = append(rows, balance.Row{UserID: b.UserID, FriendID: b.FriendID, Currency: b.Currency, Amount: b.Amount})
		}
		pairs := balance.PairsFromRows(rows)
		if onlySettled && !balance.NeedsTidy(pairs) {
			return nil
		}
		shifts := balance.Rewire(pairs)
		out := make([]store.PlannedExpense, 0, len(shifts))
		for _, sh := range shifts {
			out = append(out, store.PlannedExpense{
				Expense: &store.Expense{
					ID: store.NewUUID(), Name: nettingName, Category: "netting", Amount: sh.Amount,
					SplitType: string(split.NETTING), ExpenseDate: date, Currency: sh.Currency,
					PaidBy: sh.Payer, AddedBy: actor, GroupID: nullInt(&gid), TransactionID: nullStr(txID),
				},
				Parts: []store.ExpenseParticipant{
					{UserID: sh.Payer, Amount: sh.Amount},
					{UserID: sh.Other, Amount: -sh.Amount},
				},
			})
		}
		return out
	})
}

// netGroupAfter is netGroup for the hooks that follow a write the caller has
// already been told succeeded: a failure is logged, never returned, so it cannot
// invite a retry that would record the settlement twice. The pairs then keep
// their old shape until the next settlement or a tidy-up; nets are unaffected
// either way.
func (s *Service) netGroupAfter(ctx context.Context, groupID int64, date string, actor int64, txID string) {
	if _, err := s.netGroup(ctx, groupID, date, actor, txID, false); err != nil {
		slog.Warn("netting skipped; the group's pair balances keep their old shape until the next settlement",
			"group", groupID, "err", err)
	}
}

// renetIfSimplified runs netGroupAfter when the group has debt simplification
// on: the hook after a single group settlement, a settlement edit or delete, or
// a recurring settlement. A group showing raw pairs keeps them.
func (s *Service) renetIfSimplified(ctx context.Context, groupID int64, date string, actor int64, txID string) {
	// Detached like netGroup itself: the settlement has committed, and a client
	// that disconnects now must not make the flag read -- and so the pass -- fail.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), nettingTimeout)
	defer cancel()
	g, err := s.Store.GetGroup(ctx, groupID)
	if err != nil { // cov:ignore log-only: netting is skipped and the settlement stands
		slog.Warn("netting skipped: could not read the group's simplify setting", "group", groupID, "err", err)
		return
	}
	if !g.SimplifyDebts {
		return
	}
	s.netGroupAfter(ctx, groupID, date, actor, txID)
}

// TidyGroup clears the leftover pair balances of a group whose debts are all
// settled -- groups settled before netting existed, or pairs left uneven by later
// edits -- so Balances and friend pages read settled too. Members only. Unlike
// the hooks it reports a failure, because tidying is the whole action; a group
// that still has debts, or nothing to tidy, is left as it is (0 entries).
func (s *Service) TidyGroup(ctx context.Context, actor *store.User, groupID int64) (int, error) {
	member, err := s.Store.IsGroupMember(ctx, groupID, actor.ID)
	if err != nil { // cov:ignore store-fault: needs a failing database
		return 0, err
	}
	if !member {
		return 0, ErrNotGroupMember
	}
	return s.netGroup(ctx, groupID, "", actor.ID, "", true)
}
