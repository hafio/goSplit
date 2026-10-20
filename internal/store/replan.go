package store

import "context"

// PlannedExpense is one expense, with its participant rows, that a ReplanGroup
// plan wants written.
type PlannedExpense struct {
	Expense *Expense
	Parts   []ExpenseParticipant
}

// ReplanGroup runs plan over the group's current balance rows and writes the
// expenses it returns, all inside one transaction, so a partial set is never
// committed -- which matters for entries that only add up to nothing as a whole.
//
// The transaction starts by touching the group's own row. On Postgres that
// takes a row lock held to commit, so a concurrent ReplanGroup for the same
// group waits and then reads the committed result instead of planning from the
// same snapshot twice; on SQLite the single connection serializes everything.
// On Postgres an ordinary expense insert is not blocked by that lock and can
// commit mid-pass: nets still cannot change (every planned set is zero-sum per
// person), but the pairs may lag the newest write until the next pass.
// A group that does not exist is not an error: nothing is written and 0 is
// returned. plan must not touch the store: it runs while the transaction holds
// SQLite's only connection, so any other store call from it would wait for that
// connection until the context gives up.
func (s *Store) ReplanGroup(ctx context.Context, groupID int64, plan func([]Balance) []PlannedExpense) (int, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, s.rebind(`UPDATE groups SET simplify_debts = simplify_debts WHERE id = ?`), groupID)
	if err != nil { // cov:ignore store-fault: needs a failing database
		return 0, err
	}
	if n, err := res.RowsAffected(); err != nil { // cov:ignore store-fault: needs a failing database
		return 0, err
	} else if n == 0 {
		return 0, nil
	}

	rows, err := tx.QueryContext(ctx, s.rebind(
		`SELECT user_id, friend_id, group_id, currency, amount
		 FROM balance_view WHERE group_id = ? AND amount <> 0
		 ORDER BY currency, user_id, friend_id`), groupID)
	if err != nil { // cov:ignore store-fault: needs a failing database
		return 0, err
	}
	bals, err := scanBalances(rows)
	if err != nil { // cov:ignore store-fault: needs a failing database
		return 0, err
	}

	planned := plan(bals)
	for _, p := range planned {
		if err := s.insertExpenseTx(ctx, tx, p.Expense, p.Parts); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil { // cov:ignore env: a commit fails only on an I/O error or a full disk
		return 0, err
	}
	return len(planned), nil
}
