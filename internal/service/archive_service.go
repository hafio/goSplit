package service

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/hafio/gosplit/internal/money"
	"github.com/hafio/gosplit/internal/split"
	"github.com/hafio/gosplit/internal/store"
)

// Errors surfaced to the archive handlers.
var (
	ErrNothingToArchive = errors.New("no transactions before that date to archive")
	ErrNotGroupMember   = errors.New("only group members can archive its history")
)

// HistoricalName is the title of every synthetic collapse expense.
const HistoricalName = "Historical Transactions"

// ArchiveDirect collapses the non-group history between actor and friend dated
// before cutoff into one Historical Transactions expense per currency. Returns
// the number of collapsed expenses.
func (s *Service) ArchiveDirect(ctx context.Context, actor, friend int64, cutoff string) (int, error) {
	if actor == friend {
		return 0, errors.New("cannot archive history with yourself")
	}
	exps, err := s.Store.ListDirectCollapsible(ctx, actor, friend, cutoff)
	if err != nil {
		return 0, err
	}
	return s.collapse(ctx, exps, nil, actor)
}

// ArchiveGroup collapses a group's history dated before cutoff into one
// Historical Transactions expense per currency. actor must be a member.
//
// Each member's net position per currency (and therefore the group's simplified
// settlements) is preserved exactly. Because a single expense routes through one
// payer, pairwise debts between non-payer members are re-routed through the
// collapse — inherent to collapsing many rows into one transaction.
func (s *Service) ArchiveGroup(ctx context.Context, actor, groupID int64, cutoff string) (int, error) {
	ok, err := s.Store.IsGroupMember(ctx, groupID, actor)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ErrNotGroupMember
	}
	exps, err := s.Store.ListGroupCollapsible(ctx, groupID, cutoff)
	if err != nil {
		return 0, err
	}
	return s.collapse(ctx, exps, &groupID, actor)
}

// PreviewArchive counts the collapsible expenses per currency for a target
// (direct if groupID is nil, else the group), for the confirmation page.
func (s *Service) PreviewArchive(ctx context.Context, actor int64, friendID *int64, groupID *int64, cutoff string) (map[string]int, error) {
	var exps []*store.Expense
	var err error
	switch {
	case groupID != nil:
		exps, err = s.Store.ListGroupCollapsible(ctx, *groupID, cutoff)
	case friendID != nil:
		exps, err = s.Store.ListDirectCollapsible(ctx, actor, *friendID, cutoff)
	}
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, e := range exps {
		counts[e.Currency]++
	}
	return counts, nil
}

func (s *Service) collapse(ctx context.Context, exps []*store.Expense, groupID *int64, actor int64) (int, error) {
	batches, err := s.buildBatches(ctx, exps, groupID, actor)
	if err != nil {
		return 0, err
	}
	if len(batches) == 0 {
		return 0, ErrNothingToArchive
	}
	if err := s.Store.CollapseToHistorical(ctx, batches); err != nil {
		return 0, err
	}
	return len(exps), nil
}

// buildBatches groups the collapse set by currency and, for each, computes the
// per-user net, the synthetic Historical Transactions expense (with a CSV note),
// and the origin ids to archive.
func (s *Service) buildBatches(ctx context.Context, exps []*store.Expense, groupID *int64, actor int64) ([]store.HistoricalBatch, error) {
	if len(exps) == 0 {
		return nil, nil
	}
	byCur := map[string][]*store.Expense{}
	partsByExpense := map[string][]store.ExpenseParticipant{}
	userSet := map[int64]bool{}
	for _, e := range exps {
		byCur[e.Currency] = append(byCur[e.Currency], e)
		parts, err := s.Store.GetParticipants(ctx, e.ID)
		if err != nil {
			return nil, err
		}
		partsByExpense[e.ID] = parts
		userSet[e.PaidBy] = true
		for _, p := range parts {
			userSet[p.UserID] = true
		}
	}
	names := map[int64]string{}
	for uid := range userSet {
		names[uid] = "user"
		if u, err := s.Store.GetUser(ctx, uid); err == nil {
			if u.Name != "" {
				names[uid] = u.Name
			} else {
				names[uid] = u.Email
			}
		}
	}

	var batches []store.HistoricalBatch
	for _, cur := range sortedStrKeys(byCur) {
		list := byCur[cur]
		sort.SliceStable(list, func(i, j int) bool { return list[i].ExpenseDate < list[j].ExpenseDate })

		net := map[int64]int64{}
		involvedSet := map[int64]bool{}
		for _, e := range list {
			for _, p := range partsByExpense[e.ID] {
				net[p.UserID] += p.Amount
				involvedSet[p.UserID] = true
			}
		}
		involved := sortedInt64Keys(involvedSet)

		paidBy := actor
		var amount int64
		if len(involved) > 0 {
			paidBy = involved[0]
			for _, uid := range involved {
				if net[uid] > net[paidBy] {
					paidBy = uid
				}
				if net[uid] > 0 {
					amount += net[uid]
				}
			}
		}
		parts := make([]store.ExpenseParticipant, 0, len(involved))
		for _, uid := range involved {
			parts = append(parts, store.ExpenseParticipant{UserID: uid, Amount: net[uid]})
		}
		origin := make([]string, len(list))
		for i, e := range list {
			origin[i] = e.ID
		}
		e := &store.Expense{
			ID:          store.NewUUID(),
			Name:        HistoricalName,
			Category:    "archive",
			Amount:      amount,
			SplitType:   string(split.ARCHIVE),
			ExpenseDate: list[len(list)-1].ExpenseDate, // newest collapsed date
			Currency:    cur,
			PaidBy:      paidBy,
			AddedBy:     actor,
			UpdatedBy:   sql.NullInt64{Int64: actor, Valid: true},
			GroupID:     nullInt(groupID),
			Note:        buildHistoricalCSV(list, partsByExpense, names),
		}
		batches = append(batches, store.HistoricalBatch{Expense: e, Participants: parts, OriginIDs: origin})
	}
	return batches, nil
}

// buildHistoricalCSV renders the collapsed expenses as an RFC-4180 CSV stored in
// the Historical Transactions note. Shares column: "Name +12.34; Name2 -12.34".
func buildHistoricalCSV(exps []*store.Expense, partsByExpense map[string][]store.ExpenseParticipant, names map[int64]string) string {
	var b strings.Builder
	w := csv.NewWriter(&b)
	_ = w.Write([]string{"Date", "Description", "Paid by", "Amount", "Currency", "Category", "Shares"})
	for _, e := range exps {
		shares := make([]string, 0, len(partsByExpense[e.ID]))
		for _, p := range partsByExpense[e.ID] {
			sign := ""
			if p.Amount > 0 {
				sign = "+"
			}
			shares = append(shares, fmt.Sprintf("%s %s%s", names[p.UserID], sign, money.Format(p.Amount, e.Currency)))
		}
		_ = w.Write([]string{
			dateOnly(e.ExpenseDate), e.Name, names[e.PaidBy],
			money.Format(e.Amount, e.Currency), e.Currency, e.Category,
			strings.Join(shares, "; "),
		})
	}
	w.Flush()
	return b.String()
}

func dateOnly(iso string) string {
	if len(iso) >= 10 {
		return iso[:10]
	}
	return iso
}

func sortedStrKeys(m map[string][]*store.Expense) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedInt64Keys(m map[int64]bool) []int64 {
	out := make([]int64, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
