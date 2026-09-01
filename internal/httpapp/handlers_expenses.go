package httpapp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hafio/gosplit/internal/money"
	"github.com/hafio/gosplit/internal/service"
	"github.com/hafio/gosplit/internal/split"
	"github.com/hafio/gosplit/internal/store"
)

// candidate is one selectable participant in the expense form.
type candidate struct {
	ID       int64
	Name     string
	Included bool
	Value    string
}

var allMethods = []string{"EQUAL", "PERCENTAGE", "EXACT", "SHARE", "ADJUSTMENT"}

// participantsField is the pseudo-field the split-engine sum errors attach to.
// Those failures (percentages must total 100%, exact amounts must sum to the
// total) are about the participant rows as a set, not any one input, so they
// render once under the list.
const participantsField = "participants"

// fieldError ties a validation failure to the form field that caused it, so the
// re-rendered form can show the message where the user needs to act rather than
// only as a banner at the top.
type fieldError struct {
	Field string
	Msg   string
}

func (e *fieldError) Error() string { return e.Msg }

// fieldErrorf builds a fieldError with a formatted message.
func fieldErrorf(field, format string, args ...any) *fieldError {
	return &fieldError{Field: field, Msg: fmt.Sprintf(format, args...)}
}

// fieldErrorsFor maps an error onto the fields it should render against. A
// tagged fieldError lands on its own field; the settlement guards land on the
// amount hero, which is where both the amount and its currency are entered; the
// split engine's set-level failures land on the participant list; anything else
// is left to the banner.
//
// The service layer raises these as sentinels rather than fieldErrors because
// fieldError lives here: services must not depend on the HTTP layer, the same
// direction service.ErrNotMember already follows.
func fieldErrorsFor(err error) map[string]string {
	out := map[string]string{}
	var fe *fieldError
	if errors.As(err, &fe) && fe.Field != "" {
		out[fe.Field] = fe.Msg
		return out
	}
	switch {
	case errors.Is(err, service.ErrSettlementAmount), errors.Is(err, service.ErrSettlementCurrency):
		out["amount"] = err.Error()
	case errors.Is(err, split.ErrPercentSum), errors.Is(err, split.ErrExactSum),
		errors.Is(err, split.ErrShareUnits), errors.Is(err, split.ErrNegativeBP),
		errors.Is(err, split.ErrNoParticipants), errors.Is(err, split.ErrDuplicateUser),
		errors.Is(err, service.ErrNotMember):
		out[participantsField] = err.Error()
	}
	return out
}

func displayName(u *store.User) string {
	if u.Name != "" {
		return u.Name
	}
	return u.Email
}

// candidatesForContext returns the participant candidates for a group (members)
// or a direct expense (me + friend, or me + all friends when unscoped).
func (s *Server) candidatesForContext(r *http.Request, me *store.User, groupID *int64, friendID *int64) []candidate {
	ctx := r.Context()
	var users []*store.User
	switch {
	case groupID != nil:
		users, _ = s.Store.GroupMembers(ctx, *groupID)
	case friendID != nil:
		f, err := s.Store.GetUser(ctx, *friendID)
		users = []*store.User{me}
		if err == nil {
			users = append(users, f)
		}
	default:
		users = []*store.User{me}
		friends, _ := s.Store.ListFriends(ctx, me.ID)
		users = append(users, friends...)
	}
	out := make([]candidate, 0, len(users))
	for _, u := range users {
		out = append(out, candidate{ID: u.ID, Name: displayName(u), Included: true})
	}
	return out
}

func (s *Server) handleExpenseNew(w http.ResponseWriter, r *http.Request) {
	me := s.currentUser(r)
	var groupID, friendID *int64
	groupIDStr := ""
	if g := r.URL.Query().Get("group"); g != "" {
		if id, ok := atoi64(g); ok {
			groupID = &id
			groupIDStr = g
		}
	}
	if f := r.URL.Query().Get("friend"); f != "" {
		if id, ok := atoi64(f); ok {
			friendID = &id
		}
	}
	cands := s.candidatesForContext(r, me, groupID, friendID)
	cancel := "/balances"
	if groupID != nil {
		cancel = fmt.Sprintf("/groups/%d", *groupID)
	} else if friendID != nil {
		cancel = fmt.Sprintf("/friends/%d", *friendID)
	}
	// Optional prefill (e.g. from a bank transaction).
	q := r.URL.Query()
	name, amount := q.Get("name"), q.Get("amount")
	currency := q.Get("currency")
	if currency == "" {
		currency = me.DefaultCurrency
	}
	date := q.Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	groups, _ := s.Store.ListGroupsForUser(r.Context(), me.ID, false)
	var selGroup int64
	if groupID != nil {
		selGroup = *groupID
	}
	s.render(w, r, "expense_form", "expense.add", map[string]any{
		"Heading": s.tr(r, "expense.add"), "Action": "/expenses",
		"Name": name, "Category": "general", "AmountStr": amount,
		"Currency": currency, "Date": date,
		"PaidBy": me.ID, "Method": "EQUAL", "Methods": allMethods,
		"Candidates": cands, "GroupIDStr": groupIDStr,
		"ShowTarget": false, "IsMove": false,
		"AllowRetarget": true, "Groups": groups, "SelectedGroupID": selGroup,
		"SubmitLabel": s.tr(r, "expense.add"), "CancelURL": cancel,
	})
}

// parseExpenseInput reads the shared expense form fields into a service input.
func parseExpenseInput(r *http.Request, actorID int64) (service.ExpenseInput, error) {
	if err := r.ParseForm(); err != nil {
		return service.ExpenseInput{}, err
	}
	currency := strings.ToUpper(strings.TrimSpace(r.FormValue("currency")))
	if currency == "" {
		currency = "USD"
	}
	total, err := money.Parse(r.FormValue("amount"), currency)
	if err != nil {
		return service.ExpenseInput{}, fieldErrorf("amount", "invalid amount: %v", err)
	}
	method := split.Method(strings.ToUpper(r.FormValue("method")))
	payer, ok := atoi64(r.FormValue("paid_by"))
	if !ok {
		return service.ExpenseInput{}, fieldErrorf("paid_by", "invalid payer")
	}
	date := r.FormValue("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	var lines []split.Line
	for key := range r.Form {
		if !strings.HasPrefix(key, "include_") {
			continue
		}
		if r.FormValue(key) != "1" {
			continue
		}
		idStr := strings.TrimPrefix(key, "include_")
		id, ok := atoi64(idStr)
		if !ok {
			continue
		}
		val := strings.TrimSpace(r.FormValue("value_" + idStr))
		line, err := lineForMethod(method, id, val, currency)
		if err != nil {
			// Point at this participant's own box when their value is the problem;
			// an unusable method is a form-level fault, not a per-person one.
			field := "value_" + idStr
			if !split.UserSelectable(method) {
				field = "method"
			}
			return service.ExpenseInput{}, &fieldError{Field: field, Msg: err.Error()}
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return service.ExpenseInput{}, fieldErrorf(participantsField, "select at least one participant")
	}
	// Canonical order, because the loop above walks r.Form -- a map, so Go
	// randomizes its iteration. The split engine hands leftover minor units out
	// by position (see split.distributeRemainder), so an unsorted set would let
	// the same form post land the odd cent on a different person each time, and
	// re-saving an unchanged edit could silently shift money.
	sort.Slice(lines, func(i, j int) bool { return lines[i].UserID < lines[j].UserID })

	in := service.ExpenseInput{
		Name: strings.TrimSpace(r.FormValue("name")), Category: r.FormValue("category"),
		Total: total, Method: method, Currency: currency, ExpenseDate: date,
		PaidBy: payer, Note: r.FormValue("note"), Lines: lines, ActorID: actorID,
		Version: formVersion(r),
	}
	if in.Name == "" {
		return service.ExpenseInput{}, fieldErrorf("name", "description is required")
	}
	// Group context: hidden group_id (create/edit) or target_group (move).
	if tg := r.FormValue("target_group"); tg != "" && tg != "none" {
		if id, ok := atoi64(tg); ok {
			in.GroupID = &id
		}
	} else if g := r.FormValue("group_id"); g != "" {
		if id, ok := atoi64(g); ok {
			in.GroupID = &id
		}
	}
	return in, nil
}

// valueForMethod parses one participant's form value into the single raw int64
// that method reads: percent -> basis points, exact/adjustment -> minor units in
// the expense currency, share -> integer weight. EQUAL takes no value.
func valueForMethod(method split.Method, val, currency string) (int64, error) {
	switch method {
	case split.EQUAL:
		return 0, nil
	case split.PERCENTAGE:
		// percent entered as e.g. "70" or "33.33" -> basis points (x100).
		bp, err := money.Parse(orZero(val), "USD")
		if err != nil {
			return 0, fmt.Errorf("invalid percentage %q", val)
		}
		return bp, nil
	case split.EXACT:
		amt, err := money.Parse(orZero(val), currency)
		if err != nil {
			return 0, fmt.Errorf("invalid exact amount %q", val)
		}
		return amt, nil
	case split.SHARE:
		n, err := strconv.ParseInt(orZero(val), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid share units %q", val)
		}
		return n, nil
	case split.ADJUSTMENT:
		amt, err := money.Parse(orZero(val), currency)
		if err != nil {
			return 0, fmt.Errorf("invalid adjustment %q", val)
		}
		return amt, nil
	default:
		return 0, fmt.Errorf("unknown split method %q", method)
	}
}

// formatValue renders a stored raw split input back into its form field text --
// the inverse of valueForMethod, so an untouched edit re-posts the same digits.
// It mirrors that function's currency handling exactly: percentages go through
// USD (two decimals) regardless of the expense currency, exact amounts and
// adjustments through the expense's own.
func formatValue(method split.Method, v int64, currency string) string {
	switch method {
	case split.PERCENTAGE:
		return money.Format(v, "USD")
	case split.EXACT, split.ADJUSTMENT:
		return money.Format(v, currency)
	case split.SHARE:
		return strconv.FormatInt(v, 10)
	default:
		// EQUAL (and anything not user-selectable) shows no per-person value.
		return ""
	}
}

// lineForMethod interprets the per-participant value input per split method.
func lineForMethod(method split.Method, id int64, val, currency string) (split.Line, error) {
	v, err := valueForMethod(method, val, currency)
	if err != nil {
		return split.Line{UserID: id}, err
	}
	return split.LineFromInput(method, id, v), nil
}

func orZero(s string) string {
	if strings.TrimSpace(s) == "" {
		return "0"
	}
	return s
}

func (s *Server) handleExpenseCreate(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	in, err := parseExpenseInput(r, me.ID)
	if err != nil {
		s.renderExpenseError(w, r, err.Error(), fieldErrorsFor(err))
		return
	}
	// Membership guard when a group is targeted.
	if in.GroupID != nil {
		if ok, _ := s.Store.IsGroupMember(ctx, *in.GroupID, me.ID); !ok {
			s.renderExpenseError(w, r, "you are not a member of that group", nil)
			return
		}
	}
	e, err := s.Svc.AddExpense(ctx, in)
	if err != nil {
		s.renderExpenseError(w, r, err.Error(), fieldErrorsFor(err))
		return
	}
	http.Redirect(w, r, "/expenses/"+e.ID, http.StatusSeeOther)
}

func (s *Server) renderExpenseError(w http.ResponseWriter, r *http.Request, msg string, fieldErrors map[string]string) {
	me := s.currentUser(r)
	var groupID *int64
	if g := r.FormValue("group_id"); g != "" {
		if id, ok := atoi64(g); ok {
			groupID = &id
		}
	}
	cands := s.candidatesForContext(r, me, groupID, nil)
	// Nothing was persisted -- this is purely the form coming back. Restore what
	// was submitted so a rejected add costs a correction, not a retype.
	prefillFromForm(cands, r)
	groups, _ := s.Store.ListGroupsForUser(r.Context(), me.ID, false)
	var selGroup int64
	if groupID != nil {
		selGroup = *groupID
	}
	paidBy := me.ID
	if id, ok := atoi64(r.FormValue("paid_by")); ok {
		paidBy = id
	}
	s.renderErr(w, r, "expense_form", "expense.add", map[string]any{
		"Heading": s.tr(r, "expense.add"), "Action": "/expenses",
		"Name": r.FormValue("name"), "Category": r.FormValue("category"),
		"AmountStr": r.FormValue("amount"), "Currency": r.FormValue("currency"),
		"Date": r.FormValue("date"), "PaidBy": paidBy, "Method": r.FormValue("method"),
		"Methods": allMethods, "Candidates": cands, "GroupIDStr": r.FormValue("group_id"),
		"ShowTarget": false, "IsMove": false, "FieldErrors": fieldErrors,
		"AllowRetarget": true, "Groups": groups, "SelectedGroupID": selGroup,
		"SubmitLabel": s.tr(r, "expense.add"), "CancelURL": "/balances",
	}, http.StatusBadRequest, msg)
}

func (s *Server) handleExpenseDetail(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	e, err := s.Store.GetExpense(ctx, chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	parts, _ := s.Store.GetParticipants(ctx, e.ID)
	nc := s.newNameCache()
	me := s.currentUser(r)
	v := s.buildExpenseDetail(ctx, nc, me.ID, e, parts)
	s.render(w, r, "expense_detail", e.Name, v)
}

type participantView struct {
	UserID int64
	Name   string
	Amount int64
}

type expenseDetailView struct {
	ID           string
	Name         string
	Category     string
	SplitType    string
	Amount       int64
	Currency     string
	Date         string
	PaidByName   string
	GroupName    string
	Deleted      bool
	Movable      bool
	CanEdit      bool
	IsArchive    bool
	Note         string
	Participants []participantView
	// Version is the optimistic-concurrency token the delete form posts back, so
	// a delete decided from a page that predates someone else's edit is refused
	// rather than silently discarding that edit.
	Version int64
}

// buildExpenseDetail assembles the expense detail view model, resolving names,
// computing move eligibility (§5.2: not conversions, not deleted) and whether the
// actor may edit/delete it (payer/creator/participant — see CanEditExpense).
func (s *Server) buildExpenseDetail(ctx context.Context, nc *nameCache, actorID int64, e *store.Expense, parts []store.ExpenseParticipant) expenseDetailView {
	v := expenseDetailView{
		ID: e.ID, Name: e.Name, Category: e.Category, SplitType: e.SplitType,
		Amount: e.Amount, Currency: e.Currency, Date: dateOnly(e.ExpenseDate),
		PaidByName: nc.user(ctx, e.PaidBy), Deleted: e.DeletedAt.Valid,
		Note: e.Note, IsArchive: e.SplitType == string(split.ARCHIVE), Version: e.Version,
	}
	if e.GroupID.Valid {
		v.GroupName = nc.group(ctx, e.GroupID.Int64)
	}
	v.Movable = !e.DeletedAt.Valid &&
		e.SplitType != string(split.CURRENCY_CONVERSION) &&
		!e.ConversionToID.Valid
	v.CanEdit = s.Svc.CanEditExpense(ctx, actorID, e)
	for _, p := range parts {
		v.Participants = append(v.Participants, participantView{UserID: p.UserID, Name: nc.user(ctx, p.UserID), Amount: p.Amount})
	}
	return v
}

func (s *Server) handleExpenseDelete(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	switch err := s.Svc.DeleteExpense(ctx, chi.URLParam(r, "id"), me.ID, formVersion(r)); {
	case errors.Is(err, service.ErrNotEditor):
		s.renderErr(w, r, "message", "msg.delete_failed", s.tr(r, "err.not_editor"),
			http.StatusForbidden, "actor not a member of the expense")
		return
	case errors.Is(err, service.ErrExpenseNotFound):
		http.NotFound(w, r)
		return
	case errors.Is(err, store.ErrStaleExpense):
		// The detail page this delete came from predates someone else's save (or
		// their delete). There is nothing to preserve the way the edit form
		// preserves typing, so it answers 409 and points back at the record --
		// reloading it shows what changed, and the button works from there.
		s.renderErr(w, r, "message", "msg.delete_failed", s.tr(r, "err.delete_conflict"),
			http.StatusConflict, "expense changed since the page was rendered")
		return
	}
	http.Redirect(w, r, "/activity", http.StatusSeeOther)
}

// prefillEdit sets each candidate's Included/Value from an expense's stored split
// so an unchanged edit round-trips, and returns the split method to preselect.
//
// An expense saved since split inputs became persistent carries what its author
// actually typed, so the method, the digits and the participant set all come
// back exactly as entered. Rows with no recorded inputs -- saved before that, or
// system splits that never ran through the split engine -- fall back to
// reconstructing shares from the amounts, which cannot express ADJUSTMENT and so
// still presents it as EXACT.
func prefillEdit(cands []candidate, e *store.Expense, parts []store.ExpenseParticipant, payload string) string {
	if values, ok := split.DecodeInputs(payload, split.Method(e.SplitType)); ok {
		return prefillStored(cands, e, values)
	}
	return prefillDerived(cands, e, parts)
}

// sumTarget returns the total a method's per-participant values must add up to,
// and whether that method has one at all. PERCENTAGE must reach 100% in basis
// points, EXACT the expense amount; EQUAL, SHARE and ADJUSTMENT are free-form,
// so nothing is adjusted for them.
func sumTarget(method split.Method, e *store.Expense) (int64, bool) {
	switch method {
	case split.PERCENTAGE:
		return split.BasisPointsFull, true
	case split.EXACT:
		return e.Amount, true
	default:
		return 0, false
	}
}

// rebalanceToTarget closes a shortfall (or overshoot) in a set of split values
// by moving the whole difference onto the largest entry, so the values the form
// renders still satisfy the split engine when they are posted back unchanged.
// Two things open such a gap: integer truncation when values are derived from
// the stored amounts, and a participant who has since left the group and is no
// longer selectable. A set that already sums to target is left alone.
func rebalanceToTarget(vals []int64, target int64) {
	if len(vals) == 0 {
		return
	}
	var sum int64
	for _, v := range vals {
		sum += v
	}
	if sum == target {
		return
	}
	hi := 0
	for i := 1; i < len(vals); i++ {
		if vals[i] > vals[hi] {
			hi = i
		}
	}
	vals[hi] += target - sum
}

// prefillStored restores the form from the raw inputs saved with the expense. A
// candidate is included exactly when they have a stored value, so a payer who
// paid without taking a share -- the creditor row split.finalize always emits --
// comes back unchecked rather than being guessed at from a zero share.
//
// The methods with a fixed sum (PERCENTAGE, EXACT) are re-balanced over the
// included candidates before rendering: someone who has since left the group is
// no longer selectable, and replaying the stored values verbatim would leave the
// form unable to reach 100% -- or the expense total -- so an untouched re-save
// would be rejected.
func prefillStored(cands []candidate, e *store.Expense, values map[int64]int64) string {
	method := split.Method(e.SplitType)
	idx := make([]int, 0, len(cands))
	for i := range cands {
		c := &cands[i]
		v, ok := values[c.ID]
		c.Included = ok
		c.Value = ""
		if !ok {
			continue
		}
		idx = append(idx, i)
		c.Value = formatValue(method, v, e.Currency)
	}
	if target, ok := sumTarget(method, e); ok && len(idx) > 0 {
		vals := make([]int64, len(idx))
		for k, i := range idx {
			vals[k] = values[cands[i].ID]
		}
		rebalanceToTarget(vals, target)
		for k, i := range idx {
			cands[i].Value = formatValue(method, vals[k], e.Currency)
		}
	}
	return e.SplitType
}

// prefillDerived reconstructs the form from the zero-sum rows when no inputs
// were recorded (see split.finalize): the payer's share is total-amount,
// everyone else's is -amount. ADJUSTMENT (not uniquely invertible) is presented
// as EXACT.
func prefillDerived(cands []candidate, e *store.Expense, parts []store.ExpenseParticipant) string {
	amtByUser := make(map[int64]int64, len(parts))
	present := make(map[int64]bool, len(parts))
	for _, p := range parts {
		amtByUser[p.UserID] = p.Amount
		present[p.UserID] = true
	}
	share := func(uid int64) int64 {
		if uid == e.PaidBy {
			return e.Amount - amtByUser[uid]
		}
		return -amtByUser[uid]
	}
	method := e.SplitType
	if split.Method(method) == split.ADJUSTMENT {
		method = string(split.EXACT)
	}
	for i := range cands {
		c := &cands[i]
		c.Value = ""
		c.Included = present[c.ID]
		if !c.Included {
			continue
		}
		sh := share(c.ID)
		switch split.Method(method) {
		case split.EQUAL:
			// The payer row always exists; if they paid without owing a share,
			// excluding them keeps the equal re-split over the real debtors.
			if c.ID == e.PaidBy && sh == 0 {
				c.Included = false
			}
		case split.EXACT, split.SHARE:
			c.Value = formatValue(split.Method(method), sh, e.Currency)
		}
	}
	// PERCENTAGE: derive basis points and fix integer-truncation drift so the
	// values re-validate (they must sum to exactly 10000).
	if split.Method(method) == split.PERCENTAGE && e.Amount != 0 {
		idx := make([]int, 0, len(cands))
		bps := make([]int64, 0, len(cands))
		for i := range cands {
			if !cands[i].Included {
				continue
			}
			idx = append(idx, i)
			bps = append(bps, share(cands[i].ID)*split.BasisPointsFull/e.Amount)
		}
		rebalanceToTarget(bps, split.BasisPointsFull)
		for k, i := range idx {
			cands[i].Value = formatValue(split.PERCENTAGE, bps[k], e.Currency)
		}
	}
	return method
}

func (s *Server) handleExpenseMovePage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	e, err := s.Store.GetExpense(ctx, chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Current group (0 = direct/no group).
	var curGroup int64
	if e.GroupID.Valid {
		curGroup = e.GroupID.Int64
	}
	// Effective target: the ?target= override (a group id or "none"), else the
	// current group. Parsed only via atoi64 — never interpolated.
	effGroup := curGroup
	if tp := r.URL.Query().Get("target"); tp != "" {
		if tp == "none" {
			effGroup = 0
		} else if id, ok := atoi64(tp); ok {
			effGroup = id
		}
	}
	groupChanged := effGroup != curGroup

	// Candidates match the effective target so a cross-group move can pick the
	// destination group's members.
	var cands []candidate
	if effGroup != 0 {
		cands = s.candidatesForContext(r, me, &effGroup, nil)
	} else {
		cands = s.candidatesForContext(r, me, nil, nil)
	}

	// Plain edit pre-fills the current split; relocating resets to an equal split
	// (everyone included, no values) and the user re-picks a method on-screen.
	method := string(split.EQUAL)
	if !groupChanged {
		parts, _ := s.Store.GetParticipants(ctx, e.ID)
		payload, _ := s.Store.GetSplitInputs(ctx, e.ID)
		method = prefillEdit(cands, e, parts, payload)
	}

	s.render(w, r, "expense_form", "expense.edit",
		s.moveFormData(ctx, r, me, e, cands, method, effGroup, groupChanged))
}

// moveFormData assembles the edit/move form's view model. It is shared by the
// GET render and the error re-render so a rejected save comes back as a working
// form -- same target selector, same warning banner, same ack state -- instead of
// a dead-end message page.
//
// A settlement renders without the method picker or the participant list: its
// two rows are hand-built rather than split, and the fields that make sense to
// change are the money, the date, the note and the target group.
func (s *Server) moveFormData(ctx context.Context, r *http.Request, me *store.User, e *store.Expense,
	cands []candidate, method string, effGroup int64, groupChanged bool) map[string]any {
	groups, _ := s.Store.ListGroupsForUser(ctx, me.ID, false)
	type tg struct {
		ID   int64
		Name string
	}
	tgs := make([]tg, 0, len(groups))
	for _, g := range groups {
		tgs = append(tgs, tg{g.ID, g.Name})
	}
	isSettlement := e.SplitType == string(split.SETTLEMENT)
	methods := allMethods
	if isSettlement {
		methods = nil
	}
	data := map[string]any{
		"Heading": s.tr(r, "expense.edit"), "Action": "/expenses/" + e.ID + "/move",
		"Name": e.Name, "Category": e.Category, "AmountStr": money.Format(e.Amount, e.Currency),
		"Currency": e.Currency, "Date": dateOnly(e.ExpenseDate), "PaidBy": e.PaidBy,
		"Method": method, "Methods": methods, "Candidates": cands,
		"ShowTarget": true, "TargetGroups": tgs, "TargetGroupSel": effGroup, "TargetGroupID": effGroup,
		"IsMove": true, "GroupChanged": groupChanged,
		"ShowNote": true, "Note": e.Note, "Version": e.Version,
		"IsSettlement": isSettlement,
		"SubmitLabel":  s.tr(r, "expense.save_changes"), "CancelURL": "/expenses/" + e.ID,
	}
	if isSettlement {
		nc := s.newNameCache()
		data["SettlementFrom"] = nc.user(ctx, e.PaidBy)
		if _, other, err := s.Svc.SettlementParties(ctx, e); err == nil {
			data["SettlementTo"] = nc.user(ctx, other)
		}
	}
	return data
}

func (s *Server) handleExpenseMove(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	origID := chi.URLParam(r, "id")
	orig, err := s.Store.GetExpense(ctx, origID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	// A settlement's rows are not a split, so it takes its own edit path -- see
	// Service.UpdateSettlement.
	if orig.SplitType == string(split.SETTLEMENT) {
		s.handleSettlementEdit(ctx, w, r, me, orig)
		return
	}
	in, err := parseExpenseInput(r, me.ID)
	if err != nil {
		s.renderMoveError(ctx, w, r, me, orig, err)
		return
	}
	acknowledged := r.FormValue("ack") == "1"
	e, err := s.Svc.MoveExpense(ctx, origID, in, acknowledged)
	if err != nil {
		s.renderMoveError(ctx, w, r, me, orig, err)
		return
	}
	http.Redirect(w, r, "/expenses/"+e.ID, http.StatusSeeOther)
}

// handleSettlementEdit applies an edit to a settlement: amount, currency, date,
// note and target group, with the pair and direction preserved.
func (s *Server) handleSettlementEdit(ctx context.Context, w http.ResponseWriter, r *http.Request,
	me *store.User, orig *store.Expense) {
	in, ferr := parseSettlementInput(r, me.ID, orig)
	if ferr != nil {
		s.renderMoveError(ctx, w, r, me, orig, ferr)
		return
	}
	e, err := s.Svc.UpdateSettlement(ctx, orig.ID, in, r.FormValue("ack") == "1")
	if err != nil {
		s.renderMoveError(ctx, w, r, me, orig, err)
		return
	}
	http.Redirect(w, r, "/expenses/"+e.ID, http.StatusSeeOther)
}

// parseSettlementInput reads the settlement-edit subset of the shared form.
//
// The currency comes from orig, not from the form: a settlement's currency is
// fixed (see service.UpdateSettlement) and the edit form renders it as text
// rather than a picker, so nothing posts one. It still decides how the amount is
// parsed -- "1000" is 1000 minor units in JPY and 100000 in USD -- so defaulting
// a missing field to a hardcoded USD would silently misread every settlement in
// a zero-decimal currency.
func parseSettlementInput(r *http.Request, actorID int64, orig *store.Expense) (service.SettlementInput, error) {
	if err := r.ParseForm(); err != nil {
		return service.SettlementInput{}, err
	}
	currency := strings.ToUpper(strings.TrimSpace(r.FormValue("currency")))
	if currency == "" {
		currency = orig.Currency
	}
	amount, err := money.Parse(r.FormValue("amount"), currency)
	if err != nil {
		return service.SettlementInput{}, fieldErrorf("amount", "invalid amount: %v", err)
	}
	in := service.SettlementInput{
		Name: strings.TrimSpace(r.FormValue("name")), Amount: amount, Currency: currency,
		ExpenseDate: r.FormValue("date"), Note: r.FormValue("note"),
		ActorID: actorID, Version: formVersion(r),
	}
	if tg := r.FormValue("target_group"); tg != "" && tg != "none" {
		if id, ok := atoi64(tg); ok {
			in.GroupID = &id
		}
	}
	return in, nil
}

// renderMoveError re-renders the edit form with what was submitted and the error
// attached to the field that caused it, so a rejected save never costs the user
// their typing. A stale-version conflict answers 409; everything else 400.
func (s *Server) renderMoveError(ctx context.Context, w http.ResponseWriter, r *http.Request,
	me *store.User, e *store.Expense, err error) {
	var curGroup int64
	if e.GroupID.Valid {
		curGroup = e.GroupID.Int64
	}
	effGroup := curGroup
	if tg := r.FormValue("target_group"); tg != "" {
		if tg == "none" {
			effGroup = 0
		} else if id, ok := atoi64(tg); ok {
			effGroup = id
		}
	}
	var cands []candidate
	if effGroup != 0 {
		cands = s.candidatesForContext(r, me, &effGroup, nil)
	} else {
		cands = s.candidatesForContext(r, me, nil, nil)
	}
	prefillFromForm(cands, r)

	method := r.FormValue("method")
	if method == "" {
		method = e.SplitType
	}
	data := s.moveFormData(ctx, r, me, e, cands, method, effGroup, effGroup != curGroup)
	applySubmittedFields(data, r)
	status := http.StatusBadRequest
	msg := err.Error()
	if errors.Is(err, store.ErrStaleExpense) {
		// The form still holds the user's edit; hand back the row's current
		// version so one resubmit is enough, rather than the one we read before
		// the winning write landed.
		status = http.StatusConflict
		msg = s.tr(r, "err.expense_conflict")
		if cur, cerr := s.Store.GetExpense(ctx, e.ID); cerr == nil {
			data["Version"] = cur.Version
		}
	}
	data["FieldErrors"] = fieldErrorsFor(err)
	s.renderErr(w, r, "expense_form", "expense.edit", data, status, msg)
}

// applySubmittedFields overlays the posted values onto a form view model so a
// re-render shows what the user typed rather than what is stored.
func applySubmittedFields(data map[string]any, r *http.Request) {
	if v := r.FormValue("name"); v != "" {
		data["Name"] = v
	}
	if v := r.FormValue("category"); v != "" {
		data["Category"] = v
	}
	if v := r.FormValue("amount"); v != "" {
		data["AmountStr"] = v
	}
	if v := strings.ToUpper(strings.TrimSpace(r.FormValue("currency"))); v != "" {
		data["Currency"] = v
	}
	if v := r.FormValue("date"); v != "" {
		data["Date"] = v
	}
	if id, ok := atoi64(r.FormValue("paid_by")); ok {
		data["PaidBy"] = id
	}
	data["Note"] = r.FormValue("note")
	data["Acked"] = r.FormValue("ack") == "1"
}

// prefillFromForm restores what the user actually submitted onto the candidate
// rows, so a rejected post comes back as posted instead of resetting every
// checkbox and clearing every value.
func prefillFromForm(cands []candidate, r *http.Request) {
	for i := range cands {
		id := strconv.FormatInt(cands[i].ID, 10)
		cands[i].Included = r.FormValue("include_"+id) == "1"
		cands[i].Value = r.FormValue("value_" + id)
	}
}

// formVersion reads the optimistic-concurrency token the edit or delete form
// carried. A missing or unparsable value yields 0, which matches no saved row --
// expenses are inserted at store.InitialVersion (1) and only ever counted up --
// so a stale or hand-built form fails closed rather than overwriting or deleting
// someone else's edit.
func formVersion(r *http.Request) int64 {
	v, _ := atoi64(r.FormValue("version"))
	return v
}

// --- settle up ------------------------------------------------------------

func (s *Server) handleSettlePage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	fid, ok := atoi64(chi.URLParam(r, "id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	friend, err := s.Store.GetUser(ctx, fid)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	bals, _ := s.Store.FriendBalance(ctx, me.ID, fid)
	amountStr, currency := friendSettlePrefill(bals, me.DefaultCurrency)
	paidBy, _ := s.settleDirection(ctx, me.ID, fid, nil, currency)
	cands := []candidate{{ID: me.ID, Name: displayName(me), Included: true}, {ID: fid, Name: displayName(friend), Included: true}}
	s.render(w, r, "expense_form", "expense.settle", map[string]any{
		"Heading": s.tr(r, "expense.settle_with") + " " + displayName(friend), "Action": fmt.Sprintf("/friends/%d/settle", fid),
		"Name": "Settlement", "Category": "settlement", "AmountStr": amountStr,
		"Currency": currency, "Date": time.Now().Format("2006-01-02"),
		"PaidBy": paidBy, "Method": "EQUAL", "Methods": []string{"EQUAL"},
		"Candidates": cands, "GroupIDStr": "", "ShowTarget": false, "IsMove": false,
		"SubmitLabel": s.tr(r, "expense.record_settlement"), "CancelURL": fmt.Sprintf("/friends/%d", fid),
	})
}

func (s *Server) handleSettle(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	fid, ok := atoi64(chi.URLParam(r, "id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	// A friend settlement carries no group: it clears the cross-group net shown on
	// the friend page, not any single group's balance.
	s.recordSettlement(ctx, w, r, me, fid, nil, fmt.Sprintf("/friends/%d", fid))
}

// handleGroupSettlePage renders the settle form for one debt *inside* a group.
// The amount is suggested from the group's own balances (not the cross-group
// friend net), so the payment clears exactly the row the user clicked.
func (s *Server) handleGroupSettlePage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	g, to, ok := s.resolveGroupSettle(ctx, w, r, me)
	if !ok {
		return
	}
	amountStr, currency := groupSettleSuggestion(
		s.computeGroupSettlements(ctx, s.newNameCache(), g.ID, g.SimplifyDebts),
		me.ID, to.ID, strings.ToUpper(r.URL.Query().Get("cur")), g.DefaultCurrency)
	paidBy, _ := s.settleDirection(ctx, me.ID, to.ID, &g.ID, currency)
	cands := []candidate{{ID: me.ID, Name: displayName(me), Included: true}, {ID: to.ID, Name: displayName(to), Included: true}}
	s.render(w, r, "expense_form", "expense.settle", map[string]any{
		"Heading": s.tr(r, "expense.settle_with") + " " + displayName(to),
		"Action":  fmt.Sprintf("/groups/%d/settle/%d", g.ID, to.ID),
		"Name":    "Settlement", "Category": "settlement", "AmountStr": amountStr,
		"Currency": currency, "Date": time.Now().Format("2006-01-02"),
		"PaidBy": paidBy, "Method": "EQUAL", "Methods": []string{"EQUAL"},
		"Candidates": cands, "GroupIDStr": strconv.FormatInt(g.ID, 10), "ShowTarget": false, "IsMove": false,
		"SubmitLabel": s.tr(r, "expense.record_settlement"), "CancelURL": fmt.Sprintf("/groups/%d", g.ID),
	})
}

func (s *Server) handleGroupSettle(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	g, to, ok := s.resolveGroupSettle(ctx, w, r, me)
	if !ok {
		return
	}
	gid := g.ID
	s.recordSettlement(ctx, w, r, me, to.ID, &gid, fmt.Sprintf("/groups/%d", gid))
}

// handleGroupSettleAllPage renders the whole-group settle confirmation: the
// minimum set of transfers (min-cash-flow) that nets every member's balance to
// zero, computed with simplify forced on regardless of the group's display
// toggle. Consistent with per-pair settle (one transaction nets a pair), settling
// the whole group takes the fewest payments — which may include transfers strictly
// between other members. The listed set is exactly what a Confirm will record.
func (s *Server) handleGroupSettleAllPage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	g, ok := s.resolveGroupCaller(ctx, w, r, me)
	if !ok {
		return
	}
	rows := s.computeGroupSettlements(ctx, s.newNameCache(), g.ID, true)
	s.render(w, r, "settle_group", "settle.all_title", map[string]any{
		"Group": g, "Transfers": rows, "Date": time.Now().Format("2006-01-02"),
	})
}

// handleGroupSettleAll records the minimal transfer set for the whole group. It
// recomputes from current balances (idempotent — a second submit sees an empty set
// and records nothing) and writes one SETTLEMENT per transfer with the acting
// member as added_by, even for transfers strictly between other members.
func (s *Server) handleGroupSettleAll(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	g, ok := s.resolveGroupCaller(ctx, w, r, me)
	if !ok {
		return
	}
	gid := g.ID
	date := r.FormValue("date")
	for _, t := range s.computeGroupSettlements(ctx, s.newNameCache(), gid, true) {
		if _, err := s.Svc.Settle(ctx, t.FromID, t.ToID, t.Amount, t.Currency, &gid, date, me.ID); err != nil {
			s.renderErr(w, r, "message", "msg.settle_failed", err.Error(), http.StatusBadRequest, err.Error())
			return
		}
	}
	http.Redirect(w, r, fmt.Sprintf("/groups/%d", gid), http.StatusSeeOther)
}

// resolveGroupCaller parses /groups/{id} and authorizes the caller: the group
// must exist (404) and the current user must be a member (403). It writes the
// response itself and reports ok=false when the request must not proceed. Shared
// by the per-pair and whole-group settle handlers.
func (s *Server) resolveGroupCaller(ctx context.Context, w http.ResponseWriter, r *http.Request, me *store.User) (*store.Group, bool) {
	gid, ok := atoi64(chi.URLParam(r, "id"))
	if !ok {
		http.NotFound(w, r)
		return nil, false
	}
	g, err := s.Store.GetGroup(ctx, gid)
	if err != nil {
		http.NotFound(w, r)
		return nil, false
	}
	if member, _ := s.Store.IsGroupMember(ctx, gid, me.ID); !member {
		http.Error(w, "forbidden", http.StatusForbidden)
		return nil, false
	}
	return g, true
}

// resolveGroupSettle parses and authorizes a /groups/{id}/settle/{to} request:
// both the caller and the recipient must be members of the group. AddExpense now
// enforces participant membership, so every group balance is between members and a
// non-member can never hold a settleable balance. It writes the response itself
// and reports ok=false when the request must not proceed.
func (s *Server) resolveGroupSettle(ctx context.Context, w http.ResponseWriter, r *http.Request, me *store.User) (*store.Group, *store.User, bool) {
	g, ok := s.resolveGroupCaller(ctx, w, r, me)
	if !ok {
		return nil, nil, false
	}
	toID, ok := atoi64(chi.URLParam(r, "to"))
	if !ok || toID == me.ID {
		http.NotFound(w, r)
		return nil, nil, false
	}
	if member, _ := s.Store.IsGroupMember(ctx, g.ID, toID); !member {
		s.renderErr(w, r, "message", "msg.settle_failed", s.tr(r, "err.settle_not_member"), http.StatusBadRequest, "recipient is not a group member")
		return nil, nil, false
	}
	to, err := s.Store.GetUser(ctx, toID)
	if err != nil {
		http.NotFound(w, r)
		return nil, nil, false
	}
	return g, to, true
}

// settleDirection returns payer,receiver for a settlement between me and other in
// this scope+currency. A settlement always runs debtor -> creditor, derived from
// the current balance server-side and never trusted from the request. For a group
// the direction follows the group's own suggested transfer for the pair, honouring
// the Simplify toggle so it matches the row the user saw/clicked; for a direct
// friend debt it follows the cross-group net. Defaults to me paying when no
// outstanding balance is found.
func (s *Server) settleDirection(ctx context.Context, me, other int64, groupID *int64, currency string) (from, to int64) {
	if groupID != nil {
		simplify := true
		if g, err := s.Store.GetGroup(ctx, *groupID); err == nil {
			simplify = g.SimplifyDebts
		}
		for _, row := range s.computeGroupSettlements(ctx, s.newNameCache(), *groupID, simplify) {
			if row.Currency != currency {
				continue
			}
			if row.FromID == me && row.ToID == other {
				return me, other
			}
			if row.FromID == other && row.ToID == me {
				return other, me
			}
		}
		return me, other
	}
	bals, _ := s.Store.FriendBalance(ctx, me, other)
	for _, b := range bals {
		if b.Currency != currency {
			continue
		}
		if b.Amount > 0 { // other owes me -> other pays
			return other, me
		}
		return me, other
	}
	return me, other
}

// recordSettlement validates the posted amount and records the payment. Direction
// (who pays whom) is derived from the current balance via settleDirection — a
// settlement always runs debtor -> creditor — so it is never taken from the
// request. groupID scopes the settlement to a group, or nil keeps it direct.
func (s *Server) recordSettlement(ctx context.Context, w http.ResponseWriter, r *http.Request, me *store.User, other int64, groupID *int64, redirect string) {
	currency := strings.ToUpper(r.FormValue("currency"))
	amount, err := money.Parse(r.FormValue("amount"), currency)
	if err != nil || amount <= 0 {
		s.renderErr(w, r, "message", "msg.settle_failed", s.tr(r, "err.invalid_settlement_amount"), http.StatusBadRequest, "invalid amount")
		return
	}
	from, to := s.settleDirection(ctx, me.ID, other, groupID, currency)
	if _, err := s.Svc.Settle(ctx, from, to, amount, currency, groupID, r.FormValue("date"), me.ID); err != nil {
		s.renderErr(w, r, "message", "msg.settle_failed", err.Error(), http.StatusBadRequest, err.Error())
		return
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

// groupSettleSuggestion prefills the group settle form from the group's own
// suggested transfers. It matches the pair a<->b in either orientation, so a
// creditor recording a debt owed to them prefills the same amount the debtor
// would. cur (from the clicked row) selects among multi-currency debts and is
// honoured only when it matches a real row — never echoed raw.
func groupSettleSuggestion(rows []settlementRow, a, b int64, cur, def string) (string, string) {
	var match *settlementRow
	for i := range rows {
		if !((rows[i].FromID == a && rows[i].ToID == b) || (rows[i].FromID == b && rows[i].ToID == a)) {
			continue
		}
		if cur != "" && rows[i].Currency == cur {
			match = &rows[i]
			break
		}
		if match == nil {
			match = &rows[i]
		}
	}
	if match == nil {
		return "", def
	}
	return money.Format(match.Amount, match.Currency), match.Currency
}

// friendSettlePrefill seeds the friend settle form with the amount and currency of
// the first non-zero balance in any currency, either direction — a debt you owe or
// one owed to you. Empty amount + the default currency when nothing is outstanding.
func friendSettlePrefill(bals []store.CumulatedBalance, def string) (string, string) {
	for _, b := range bals {
		if b.Amount == 0 {
			continue
		}
		amt := b.Amount
		if amt < 0 {
			amt = -amt
		}
		return money.Format(amt, b.Currency), b.Currency
	}
	return "", def
}
