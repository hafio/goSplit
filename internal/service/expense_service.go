package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/hafio/gosplit/internal/split"
	"github.com/hafio/gosplit/internal/store"
)

// ExpenseInput is the normalized form data for creating/editing an expense.
type ExpenseInput struct {
	Name        string
	Category    string
	Total       int64 // minor units
	Method      split.Method
	Currency    string
	ExpenseDate string // ISO date
	PaidBy      int64
	GroupID     *int64 // nil = direct/non-group expense
	FileKey     string
	Note        string       // free-text note (edit form)
	Lines       []split.Line // participants + method params
	ActorID     int64        // who is performing the action
	// Version is the expense version the edit form was rendered from. Ignored on
	// add; on edit it must still match the stored row or the save is rejected as
	// stale (see store.ErrStaleExpense).
	Version int64
}

// SettlementInput is the editable surface of a settlement. The pair, the
// direction and the currency are fixed when it is recorded -- changing one means
// deleting it and recording it again -- so only the amount, the metadata and the
// target group can change here. Currency is still carried so a resubmitted form
// can be checked against the stored one rather than silently ignored.
type SettlementInput struct {
	Name        string
	Amount      int64
	Currency    string
	ExpenseDate string
	Note        string
	GroupID     *int64
	ActorID     int64
	Version     int64
}

// Errors surfaced to expense handlers.
var (
	// ErrNotMember names the actor too, not just the participants: see assertMembers.
	ErrNotMember = errors.New("you and everyone involved must be members of the target group")
	// ErrNotRelated refuses money between people who share nothing: outside a
	// group, any account id would otherwise do (see assertRelated).
	ErrNotRelated      = errors.New("outside a group you can only share money with friends or people in one of your groups")
	ErrNotMovable      = errors.New("this expense cannot be moved")
	ErrMoveNoAck       = errors.New("you must acknowledge the recalculated split")
	ErrNotEditor       = errors.New("you can't edit or delete this expense")
	ErrExpenseNotFound = store.ErrNotFound
	ErrNotSettlement   = errors.New("this transaction is not a settlement")
	ErrIsSettlement    = errors.New("a settlement must be edited as a settlement")
	ErrBadSettlement   = errors.New("this settlement's rows are malformed and cannot be edited")
	// ErrImmutable is returned for any change to an automatic "Group debts
	// simplified" (NETTING) entry, by anyone: removing or reshaping one alone
	// would break the zero-sum set it belongs to.
	ErrImmutable = errors.New("this is an automatic entry and cannot be edited or deleted")
	// ErrSettlementAmount and ErrSettlementCurrency are sentinels rather than
	// bare errors so the handler can render them against the field that caused
	// them (see httpapp.fieldErrorsFor) instead of only as a banner.
	ErrSettlementAmount   = errors.New("settlement amount must be positive")
	ErrSettlementCurrency = errors.New("a settlement's currency is fixed; delete it and record it again in another currency")
)

// recordSplitInputs saves the raw per-participant inputs beside an expense so
// the edit form can restore exactly what was entered. Best-effort by design: the
// payload is reference data only, and when it is missing the form falls back to
// reconstructing values from the stored amounts, so a failure here must never
// fail the save the user already completed.
func (s *Service) recordSplitInputs(ctx context.Context, expenseID string, in ExpenseInput) {
	payload, err := split.EncodeInputs(in.Method, in.Lines)
	if err != nil || payload == "" { // cov:ignore impossible: Compute already rejected anything EncodeInputs would
		return
	}
	// Best-effort, but never silent: losing the payload downgrades the next edit
	// form to reconstructing values from the amounts, and nothing else would say
	// why.
	if err := s.Store.PutSplitInputs(ctx, expenseID, payload); err != nil {
		slog.Warn("split inputs not recorded; the edit form will fall back to derived values",
			"expense", expenseID, "err", err)
	}
}

// AddExpense computes the zero-sum participant rows via the split engine and
// persists the expense.
func (s *Service) AddExpense(ctx context.Context, in ExpenseInput) (*store.Expense, error) {
	if in.GroupID != nil {
		if err := s.assertMembers(ctx, *in.GroupID, in.Lines, in.PaidBy, in.ActorID); err != nil {
			return nil, err
		}
	} else if err := s.assertRelated(ctx, in.ActorID, lineIDs(in.PaidBy, in.Lines)...); err != nil {
		return nil, err
	}
	parts, err := s.buildParticipants(in)
	if err != nil { // cov:ignore pass-through: split rules are tested in internal/split
		return nil, err
	}
	e := s.newExpense(in)
	created, err := s.Store.CreateExpense(ctx, e, parts)
	if err != nil { // cov:ignore store-fault: needs a failing database
		return nil, err
	}
	s.recordSplitInputs(ctx, created.ID, in)
	// Ensure direct-expense counterparts become friends.
	if in.GroupID == nil {
		for _, p := range parts {
			if p.UserID != in.PaidBy {
				_ = s.Store.AddFriend(ctx, in.PaidBy, p.UserID)
			}
		}
	}
	s.notifyExpense(ctx, created, parts, in.ActorID, KindExpenseAdded)
	return created, nil
}

// CanEditExpense reports whether actorID may edit or delete e: a member of the
// transaction (IsTransactionMember) or an admin, who has the same powers over
// every transaction. Group membership alone is NOT enough: a group member who is
// not part of this expense can neither edit nor delete it.
func (s *Service) CanEditExpense(ctx context.Context, actorID int64, e *store.Expense) bool {
	ok, _ := s.editAccess(ctx, actorID, e)
	return ok
}

// editAccess is CanEditExpense plus whether the admin override alone granted it
// (the actor is not part of the transaction), decided before any write so the
// audit line reflects who the actor was to the transaction at that moment.
func (s *Service) editAccess(ctx context.Context, actorID int64, e *store.Expense) (ok, viaAdmin bool) {
	if Immutable(e) {
		return false, false // nobody edits a netting entry, admins included
	}
	if s.IsTransactionMember(ctx, actorID, e) {
		return true, false
	}
	if s.isAdmin(ctx, actorID) {
		return true, true
	}
	return false, false
}

// IsTransactionMember reports whether actorID is part of e: its payer, its
// creator, or one of its participants. A failed participants lookup counts as
// "not a member".
func (s *Service) IsTransactionMember(ctx context.Context, actorID int64, e *store.Expense) bool {
	if e.PaidBy == actorID || e.AddedBy == actorID {
		return true
	}
	parts, err := s.Store.GetParticipants(ctx, e.ID)
	if err != nil {
		return false
	}
	for _, p := range parts {
		if p.UserID == actorID {
			return true
		}
	}
	return false
}

// CanViewExpense reports whether viewerID may open e's detail page: a member of
// the transaction, a member of its group (whose feed lists it), or an admin.
// Anyone else is treated as if e did not exist. Lookup failures count as "no".
func (s *Service) CanViewExpense(ctx context.Context, viewerID int64, e *store.Expense) bool {
	if s.IsTransactionMember(ctx, viewerID, e) {
		return true
	}
	if e.GroupID.Valid {
		if member, err := s.Store.IsGroupMember(ctx, e.GroupID.Int64, viewerID); err == nil && member {
			return true
		}
	}
	return s.isAdmin(ctx, viewerID)
}

// isAdmin reports whether actorID is an active admin, read fresh from the store
// so a demotion or deactivation takes effect on the very next action. Any lookup
// failure counts as "not an admin": the override fails closed.
func (s *Service) isAdmin(ctx context.Context, actorID int64) bool {
	u, err := s.Store.GetUser(ctx, actorID)
	return err == nil && u.IsActive() && u.IsAdmin()
}

// logAdminOverride records a write the admin override alone allowed, so a change
// made to someone else's books leaves a trace beyond updated_by/deleted_by.
func logAdminOverride(action string, e *store.Expense, actorID int64) {
	slog.Info("admin override", "action", action, "expense", e.ID, "actor", actorID)
}

// Transfer is one leg of a group settle-up: who pays whom, how much.
type Transfer struct {
	FromID   int64
	ToID     int64
	Amount   int64
	Currency string
}

// Settle records a settlement payment (sender pays receiver) as a SETTLEMENT
// expense driving balances toward zero.
func (s *Service) Settle(ctx context.Context, sender, receiver int64, amount int64, currency string, groupID *int64, date string, actor int64) (*store.Expense, error) {
	if groupID == nil {
		if err := s.assertRelated(ctx, actor, sender, receiver); err != nil {
			return nil, err
		}
	}
	created, parts, err := s.settle(ctx, sender, receiver, amount, currency, groupID, date, actor)
	if err != nil {
		return nil, err
	}
	s.notifyExpense(ctx, created, parts, actor, KindSettlementAdded)
	// Inside a group that shows simplified debts, paying one of them leaves the
	// pairs out of step with what the page shows; rewire them (netting_service).
	if groupID != nil {
		s.renetIfSimplified(ctx, *groupID, created.ExpenseDate, actor, created.ID)
	}
	return created, nil
}

// SettleAll records a group settle-up: every transfer becomes its own settlement
// and its own notification rows, but the batch produces a single push per
// recipient. It owns the loop that used to live in the handler, so a settle-up
// is one reported outcome rather than a half-applied set behind an error page.
func (s *Service) SettleAll(ctx context.Context, transfers []Transfer, groupID int64,
	date string, actor int64) ([]*store.Expense, error) {
	settled := make([]*store.Expense, 0, len(transfers))
	partsByID := make(map[string][]store.ExpenseParticipant, len(transfers))
	for _, t := range transfers {
		e, parts, err := s.settle(ctx, t.FromID, t.ToID, t.Amount, t.Currency, &groupID, date, actor)
		if err != nil {
			return settled, err
		}
		settled = append(settled, e)
		partsByID[e.ID] = parts
	}
	if len(settled) == 0 {
		return settled, nil
	}
	// A settle-up always pays the simplified transfers, whatever the group's
	// display setting, so the pairs it leaves behind are always rewired: after a
	// full settle-up every pair in the group reads zero.
	s.netGroupAfter(ctx, groupID, date, actor, settled[0].ID)
	name := ""
	if g, err := s.Store.GetGroup(ctx, groupID); err == nil {
		name = g.Name
	}
	s.notifySettleBatch(ctx, settled, partsByID, actor, groupID, name)
	return settled, nil
}

// settle writes one settlement and returns it with its participant rows. It
// records but never notifies, so Settle can announce a single transfer while
// SettleAll coalesces a batch -- neither needs a mode flag on the other.
func (s *Service) settle(ctx context.Context, sender, receiver int64, amount int64,
	currency string, groupID *int64, date string, actor int64) (*store.Expense, []store.ExpenseParticipant, error) {
	if amount <= 0 {
		return nil, nil, ErrSettlementAmount
	}
	e := &store.Expense{
		ID:          store.NewUUID(),
		Name:        "Settlement",
		Category:    "settlement",
		Amount:      amount,
		SplitType:   string(split.SETTLEMENT),
		ExpenseDate: date,
		Currency:    strings.ToUpper(currency),
		PaidBy:      sender,
		AddedBy:     actor,
		GroupID:     nullInt(groupID),
	}
	parts := []store.ExpenseParticipant{
		{UserID: sender, Amount: amount},
		{UserID: receiver, Amount: -amount},
	}
	created, err := s.Store.CreateExpense(ctx, e, parts)
	if err != nil { // cov:ignore store-fault: needs a failing database
		return nil, nil, err
	}
	return created, parts, nil
}

// MoveExpense edits an expense in place (the shared move/edit action): the SAME
// record is updated with the recalculated split and note (no soft-delete, no
// duplicate). The actor must be allowed to edit it (CanEditExpense: payer,
// creator or participant, or an admin; group membership alone is not enough).
// The recalculated-split acknowledgment (acknowledged=true) is required ONLY
// when the expense is relocated to a different group — a plain in-place edit
// (same group, incl. direct→direct) needs no ack. Eligibility (§5.2) and target
// membership are validated: when a group is targeted the actor must belong to
// it too, except an admin, who skips only that actor check -- the payer and
// every participant are always checked. created_at/added_by are preserved; the
// note comes from the form (in.Note) and the receipt (file_key) is carried over
// since the form doesn't resubmit it. A settlement is refused outright: its two rows are hand-built,
// so re-splitting them here would rewrite split_type -- see UpdateSettlement.
func (s *Service) MoveExpense(ctx context.Context, origID string, in ExpenseInput, acknowledged bool) (*store.Expense, error) {
	orig, err := s.Store.GetExpense(ctx, origID)
	if err != nil { // cov:ignore store-fault: needs a failing database
		return nil, err
	}
	if Immutable(orig) {
		return nil, ErrImmutable
	}
	if orig.SplitType == string(split.SETTLEMENT) {
		return nil, ErrIsSettlement
	}
	allowed, viaAdmin := s.editAccess(ctx, in.ActorID, orig)
	if !allowed {
		return nil, ErrNotEditor
	}
	if !movable(orig) {
		return nil, ErrNotMovable
	}
	if !sameGroup(orig.GroupID, in.GroupID) && !acknowledged {
		return nil, ErrMoveNoAck
	}
	if in.GroupID != nil {
		// An admin is exempt from the actor-must-be-a-member half of the check:
		// passing the payer in the actor slot leaves exactly payer plus
		// participants, so an admin who is one of those is still checked as such.
		memberActor := in.ActorID
		if s.isAdmin(ctx, in.ActorID) {
			memberActor = in.PaidBy
		}
		if err := s.assertMembers(ctx, *in.GroupID, in.Lines, in.PaidBy, memberActor); err != nil {
			return nil, err
		}
	} else {
		// Same exemption outside a group: an admin who is not part of the
		// transaction is checked through its payer instead of themselves.
		owner := in.ActorID
		if viaAdmin {
			owner = in.PaidBy
		}
		if err := s.assertRelated(ctx, owner, lineIDs(in.PaidBy, in.Lines)...); err != nil {
			return nil, err
		}
	}
	parts, err := s.buildParticipants(in)
	if err != nil { // cov:ignore pass-through: split rules are tested in internal/split
		return nil, err
	}
	e := s.newExpense(in)
	e.ID = origID            // edit the original row in place
	e.FileKey = orig.FileKey // preserve the receipt (not resubmitted by the move form)
	e.Version = in.Version   // reject the save if someone else edited it meanwhile
	if err := s.Store.UpdateExpense(ctx, e, parts); err != nil {
		return nil, err
	}
	if viaAdmin {
		logAdminOverride("edit", orig, in.ActorID)
	}
	s.recordSplitInputs(ctx, origID, in)
	updated, err := s.Store.GetExpense(ctx, origID)
	if err != nil { // cov:ignore store-fault: needs a failing database
		return nil, err
	}
	s.notifyExpense(ctx, updated, parts, in.ActorID, KindExpenseUpdated)
	return updated, nil
}

// UpdateSettlement edits a SETTLEMENT in place, preserving its split type and
// its two zero-sum rows (payer +amount, counterparty -amount).
//
// Settlements get their own path deliberately: their rows are hand-built by
// Settle and never pass through the split engine, so routing an edit through the
// ordinary expense form would rewrite split_type and silently turn the record
// into a normal expense -- breaking the balance UI and every settlement query.
// The pair, the direction and the currency are fixed at creation; changing any
// of those means deleting the settlement and recording it again. Relocating to
// another group needs the same acknowledgment as any other move, because it
// changes which balance the settlement clears.
func (s *Service) UpdateSettlement(ctx context.Context, id string, in SettlementInput, acknowledged bool) (*store.Expense, error) {
	orig, err := s.Store.GetExpense(ctx, id)
	if err != nil { // cov:ignore store-fault: needs a failing database
		return nil, err
	}
	if Immutable(orig) {
		return nil, ErrImmutable
	}
	if orig.SplitType != string(split.SETTLEMENT) {
		return nil, ErrNotSettlement
	}
	allowed, viaAdmin := s.editAccess(ctx, in.ActorID, orig)
	if !allowed {
		return nil, ErrNotEditor
	}
	if !movable(orig) {
		return nil, ErrNotMovable
	}
	if in.Amount <= 0 {
		return nil, ErrSettlementAmount
	}
	// A settlement's currency picks which per-currency balance it clears, so
	// switching it would leave the original debt un-offset and invent a second
	// one in the new currency. It is fixed for the life of the record; an empty
	// value means the form carried none, which keeps the stored one.
	currency := strings.ToUpper(orDefault(in.Currency, orig.Currency))
	if currency != orig.Currency {
		return nil, ErrSettlementCurrency
	}
	if !sameGroup(orig.GroupID, in.GroupID) && !acknowledged { // cov:ignore redundant: tested end to end in httpapp settle_edit_test
		return nil, ErrMoveNoAck
	}
	payer, other, err := s.SettlementParties(ctx, orig)
	if err != nil {
		return nil, err
	}
	if in.GroupID != nil {
		// The actor is checked alongside the pair, not just the pair: settling a
		// whole group makes the acting member added_by on transfers between two
		// other people, and CanEditExpense then lets them edit it -- without this
		// they could file that settlement into a group they do not belong to. An
		// admin is exempt from the actor half only; the pair is always checked.
		ids := []int64{payer, other}
		if !s.isAdmin(ctx, in.ActorID) {
			ids = append(ids, in.ActorID)
		}
		if err := s.assertGroupMembers(ctx, *in.GroupID, ids...); err != nil {
			return nil, err
		}
	}
	e := &store.Expense{
		ID:          orig.ID,
		Name:        orDefault(in.Name, orig.Name),
		Category:    orig.Category,
		Amount:      in.Amount,
		SplitType:   orig.SplitType,
		ExpenseDate: orDefault(in.ExpenseDate, orig.ExpenseDate),
		Currency:    currency,
		PaidBy:      payer,
		AddedBy:     orig.AddedBy,
		UpdatedBy:   sql.NullInt64{Int64: in.ActorID, Valid: true},
		GroupID:     nullInt(in.GroupID),
		FileKey:     orig.FileKey,
		Note:        in.Note,
		Version:     in.Version,
	}
	parts := []store.ExpenseParticipant{
		{UserID: payer, Amount: in.Amount},
		{UserID: other, Amount: -in.Amount},
	}
	if err := s.Store.UpdateExpense(ctx, e, parts); err != nil {
		return nil, err
	}
	if viaAdmin {
		logAdminOverride("settlement edit", orig, in.ActorID)
	}
	// The edit changed what this settlement pays, possibly in two groups (a
	// move): rewire both, before anything below can return early.
	if orig.GroupID.Valid {
		s.renetIfSimplified(ctx, orig.GroupID.Int64, e.ExpenseDate, in.ActorID, orig.ID)
	}
	if in.GroupID != nil && !sameGroup(orig.GroupID, in.GroupID) {
		s.renetIfSimplified(ctx, *in.GroupID, e.ExpenseDate, in.ActorID, orig.ID)
	}
	updated, err := s.Store.GetExpense(ctx, id)
	if err != nil { // cov:ignore store-fault: needs a failing database
		return nil, err
	}
	s.notifyExpense(ctx, updated, parts, in.ActorID, KindSettlementUpdated)
	return updated, nil
}

// SettlementParties resolves a settlement's payer and the person they paid from
// its stored rows -- who owes whom is not recoverable from the expense row alone.
// A settlement is exactly two rows by construction (see Settle); anything else is
// a corrupt record, and editing it would have to guess at the direction, so it
// fails loudly rather than silently picking one.
func (s *Service) SettlementParties(ctx context.Context, e *store.Expense) (payer, other int64, err error) {
	parts, err := s.Store.GetParticipants(ctx, e.ID)
	if err != nil { // cov:ignore store-fault: needs a failing database
		return 0, 0, err
	}
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("%w (%d participant rows, want 2)", ErrBadSettlement, len(parts))
	}
	for _, p := range parts {
		if p.UserID == e.PaidBy {
			payer = p.UserID
		} else {
			other = p.UserID
		}
	}
	if payer == 0 || other == 0 {
		return 0, 0, fmt.Errorf("%w (payer %d is not among its rows)", ErrBadSettlement, e.PaidBy)
	}
	return payer, other, nil
}

// DeleteExpense soft-deletes an expense after confirming the actor may edit it
// (CanEditExpense: payer/creator/participant, or an admin). Deleting a settlement
// is the supported way to reverse it: the row leaves balance_view (deleted_at IS
// NOT NULL) and the balance is restored.
//
// version is the expense version the delete button was rendered from, and it is
// asserted like any other write: a delete decided from a stale page -- one that
// never showed the edit someone else has since saved -- is rejected with
// store.ErrStaleExpense instead of discarding a split the deleter never saw.
func (s *Service) DeleteExpense(ctx context.Context, id string, actorID, version int64) error {
	e, err := s.Store.GetExpense(ctx, id)
	if err != nil {
		return err // ErrExpenseNotFound
	}
	if Immutable(e) {
		return ErrImmutable
	}
	allowed, viaAdmin := s.editAccess(ctx, actorID, e)
	if !allowed {
		return ErrNotEditor
	}
	// Read the participants before the delete: they are who gets told, and a
	// failure here must not stop a delete the actor is entitled to make. A soft
	// delete leaves expense_participants intact either way.
	parts, pErr := s.Store.GetParticipants(ctx, id)
	if err := s.Store.SoftDeleteExpense(ctx, id, actorID, version); err != nil {
		return err
	}
	if viaAdmin {
		logAdminOverride("delete", e, actorID)
	}
	// Deleting a group settlement brings its debt back: rewire the group's pairs
	// to the simplified debts again, before the notification path can return.
	if e.SplitType == string(split.SETTLEMENT) && e.GroupID.Valid {
		s.renetIfSimplified(ctx, e.GroupID.Int64, "", actorID, e.ID)
	}
	if pErr != nil { // cov:ignore log-only: the delete stands; only the notification is skipped
		slog.Warn("notify: participants for deleted expense", "expense", id, "err", pErr)
		return nil
	}
	s.notifyExpense(ctx, e, parts, actorID, KindExpenseDeleted)
	return nil
}

// Movable reports whether e can be edited or moved at all (see movable). The
// detail page and the edit form's guard use it so they agree with the service.
func Movable(e *store.Expense) bool { return movable(e) }

// Immutable reports whether e is an automatic netting entry, which no one may
// edit, move or delete (ErrImmutable).
func Immutable(e *store.Expense) bool { return e.SplitType == string(split.NETTING) }

// movable enforces §5.2 eligibility: currency-conversion pairs and recurrence
// templates are not movable; everything else (incl. settlements) is.
func movable(e *store.Expense) bool {
	if e.SplitType == string(split.CURRENCY_CONVERSION) || e.ConversionToID.Valid {
		return false
	}
	// Historical Transactions are a synthetic collapse of many rows — not movable.
	if e.SplitType == string(split.ARCHIVE) {
		return false
	}
	// A netting entry belongs to a zero-sum set; it never moves on its own.
	if Immutable(e) {
		return false
	}
	// Expenses generated by a recurrence move like any other; a recurrence
	// *template* carries no recurrence_id and is guarded at the handler layer.
	return !e.DeletedAt.Valid
}

// assertMembers requires every user the save would touch -- the payer, each
// participant, and the actor performing it -- to belong to the target group.
// The actor is included because being a member of the transaction is not the
// same as being a member of the destination: creator rights on a transfer
// between two other people would otherwise let someone file it into a group
// they have no part in, where it would move that group's balances.
func (s *Service) assertMembers(ctx context.Context, groupID int64, lines []split.Line, payer, actor int64) error {
	ids := map[int64]bool{payer: true, actor: true}
	for _, l := range lines {
		ids[l.UserID] = true
	}
	uniq := make([]int64, 0, len(ids))
	for id := range ids {
		uniq = append(uniq, id)
	}
	return s.assertGroupMembers(ctx, groupID, uniq...)
}

// assertRelated fails closed with ErrNotRelated unless every id other than
// owner is related to owner (Store.AreRelated). It guards everything that puts
// money between people outside a group -- a direct expense, a friend
// settlement, a currency conversion -- where nothing else ties the ids in the
// request to the person making it. Without it any account id would do, and the
// stranger would be befriended, charged and notified.
func (s *Service) assertRelated(ctx context.Context, owner int64, ids ...int64) error {
	for _, id := range ids {
		if id == owner {
			continue
		}
		ok, err := s.Store.AreRelated(ctx, owner, id)
		if err != nil { // cov:ignore store-fault: needs a failing database
			return err
		}
		if !ok {
			return ErrNotRelated
		}
	}
	return nil
}

// lineIDs is the payer plus every participant of a split.
func lineIDs(paidBy int64, lines []split.Line) []int64 {
	ids := []int64{paidBy}
	for _, l := range lines {
		ids = append(ids, l.UserID)
	}
	return ids
}

// assertGroupMembers fails closed with ErrNotMember unless every id belongs to
// groupID; a lookup failure is reported as-is rather than treated as absence.
func (s *Service) assertGroupMembers(ctx context.Context, groupID int64, ids ...int64) error {
	for _, id := range ids {
		ok, err := s.Store.IsGroupMember(ctx, groupID, id)
		if err != nil { // cov:ignore store-fault: needs a failing database
			return err
		}
		if !ok {
			return ErrNotMember
		}
	}
	return nil
}

func (s *Service) buildParticipants(in ExpenseInput) ([]store.ExpenseParticipant, error) {
	ps, err := split.Compute(in.Method, in.Total, in.PaidBy, in.ExpenseDate, in.Lines)
	if err != nil { // cov:ignore pass-through: split rules are tested in internal/split
		return nil, err
	}
	out := make([]store.ExpenseParticipant, len(ps))
	for i, p := range ps {
		out[i] = store.ExpenseParticipant{UserID: p.UserID, Amount: p.Amount}
	}
	return out, nil
}

func (s *Service) newExpense(in ExpenseInput) *store.Expense {
	return &store.Expense{
		ID:          store.NewUUID(),
		Name:        in.Name,
		Category:    orDefault(in.Category, "general"),
		Amount:      in.Total,
		SplitType:   string(in.Method),
		ExpenseDate: in.ExpenseDate,
		Currency:    strings.ToUpper(in.Currency),
		PaidBy:      in.PaidBy,
		AddedBy:     in.ActorID,
		UpdatedBy:   sql.NullInt64{Int64: in.ActorID, Valid: true},
		GroupID:     nullInt(in.GroupID),
		FileKey:     nullStr(in.FileKey),
		Note:        in.Note,
	}
}

// sameGroup reports whether an expense's stored group (nullable) and a target
// group id (nil = no group) refer to the same group — used to decide whether a
// re-split acknowledgment is required. Both absent counts as the same group.
func sameGroup(orig sql.NullInt64, in *int64) bool {
	if !orig.Valid {
		return in == nil
	}
	return in != nil && *in == orig.Int64
}

func nullInt(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
