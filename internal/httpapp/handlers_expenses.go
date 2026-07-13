package httpapp

import (
	"context"
	"fmt"
	"net/http"
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
		return service.ExpenseInput{}, fmt.Errorf("invalid amount: %w", err)
	}
	method := split.Method(strings.ToUpper(r.FormValue("method")))
	payer, ok := atoi64(r.FormValue("paid_by"))
	if !ok {
		return service.ExpenseInput{}, fmt.Errorf("invalid payer")
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
			return service.ExpenseInput{}, err
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return service.ExpenseInput{}, fmt.Errorf("select at least one participant")
	}

	in := service.ExpenseInput{
		Name: strings.TrimSpace(r.FormValue("name")), Category: r.FormValue("category"),
		Total: total, Method: method, Currency: currency, ExpenseDate: date,
		PaidBy: payer, Note: r.FormValue("note"), Lines: lines, ActorID: actorID,
	}
	if in.Name == "" {
		return service.ExpenseInput{}, fmt.Errorf("description is required")
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

// lineForMethod interprets the per-participant value input per split method.
func lineForMethod(method split.Method, id int64, val, currency string) (split.Line, error) {
	l := split.Line{UserID: id}
	switch method {
	case split.EQUAL:
		// no value
	case split.PERCENTAGE:
		// percent entered as e.g. "70" or "33.33" -> basis points (x100).
		bp, err := money.Parse(orZero(val), "USD")
		if err != nil {
			return l, fmt.Errorf("invalid percentage %q", val)
		}
		l.BasisPoints = bp
	case split.EXACT:
		amt, err := money.Parse(orZero(val), currency)
		if err != nil {
			return l, fmt.Errorf("invalid exact amount %q", val)
		}
		l.Exact = amt
	case split.SHARE:
		n, err := strconv.ParseInt(orZero(val), 10, 64)
		if err != nil {
			return l, fmt.Errorf("invalid share units %q", val)
		}
		l.ShareUnits = n
	case split.ADJUSTMENT:
		amt, err := money.Parse(orZero(val), currency)
		if err != nil {
			return l, fmt.Errorf("invalid adjustment %q", val)
		}
		l.Adjustment = amt
	default:
		return l, fmt.Errorf("unknown split method %q", method)
	}
	return l, nil
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
		s.renderExpenseError(w, r, err.Error())
		return
	}
	// Membership guard when a group is targeted.
	if in.GroupID != nil {
		if ok, _ := s.Store.IsGroupMember(ctx, *in.GroupID, me.ID); !ok {
			s.renderExpenseError(w, r, "you are not a member of that group")
			return
		}
	}
	e, err := s.Svc.AddExpense(ctx, in)
	if err != nil {
		s.renderExpenseError(w, r, err.Error())
		return
	}
	http.Redirect(w, r, "/expenses/"+e.ID, http.StatusSeeOther)
}

func (s *Server) renderExpenseError(w http.ResponseWriter, r *http.Request, msg string) {
	me := s.currentUser(r)
	var groupID *int64
	if g := r.FormValue("group_id"); g != "" {
		if id, ok := atoi64(g); ok {
			groupID = &id
		}
	}
	cands := s.candidatesForContext(r, me, groupID, nil)
	groups, _ := s.Store.ListGroupsForUser(r.Context(), me.ID, false)
	var selGroup int64
	if groupID != nil {
		selGroup = *groupID
	}
	s.renderErr(w, r, "expense_form", "expense.add", map[string]any{
		"Heading": s.tr(r, "expense.add"), "Action": "/expenses",
		"Name": r.FormValue("name"), "Category": r.FormValue("category"),
		"AmountStr": r.FormValue("amount"), "Currency": r.FormValue("currency"),
		"Date": r.FormValue("date"), "PaidBy": me.ID, "Method": r.FormValue("method"),
		"Methods": allMethods, "Candidates": cands, "GroupIDStr": r.FormValue("group_id"),
		"ShowTarget": false, "IsMove": false,
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
	v := s.buildExpenseDetail(ctx, nc, e, parts)
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
	IsArchive    bool
	Note         string
	Participants []participantView
}

// buildExpenseDetail assembles the expense detail view model, resolving names
// and computing move eligibility (§5.2: not conversions, not deleted).
func (s *Server) buildExpenseDetail(ctx context.Context, nc *nameCache, e *store.Expense, parts []store.ExpenseParticipant) expenseDetailView {
	v := expenseDetailView{
		ID: e.ID, Name: e.Name, Category: e.Category, SplitType: e.SplitType,
		Amount: e.Amount, Currency: e.Currency, Date: dateOnly(e.ExpenseDate),
		PaidByName: nc.user(ctx, e.PaidBy), Deleted: e.DeletedAt.Valid,
		Note: e.Note, IsArchive: e.SplitType == string(split.ARCHIVE),
	}
	if e.GroupID.Valid {
		v.GroupName = nc.group(ctx, e.GroupID.Int64)
	}
	v.Movable = !e.DeletedAt.Valid &&
		e.SplitType != string(split.CURRENCY_CONVERSION) &&
		!e.ConversionToID.Valid
	for _, p := range parts {
		v.Participants = append(v.Participants, participantView{UserID: p.UserID, Name: nc.user(ctx, p.UserID), Amount: p.Amount})
	}
	return v
}

func (s *Server) handleExpenseDelete(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	_ = s.Store.SoftDeleteExpense(ctx, chi.URLParam(r, "id"), me.ID)
	http.Redirect(w, r, "/activity", http.StatusSeeOther)
}

// prefillEdit sets each candidate's Included/Value from an expense's stored split
// so an unchanged edit round-trips, and returns the split method to preselect.
// Shares are recovered from the zero-sum rows (see split.finalize): the payer's
// share is total-amount, everyone else's is -amount. ADJUSTMENT (not uniquely
// invertible) is presented as EXACT.
func prefillEdit(cands []candidate, e *store.Expense, parts []store.ExpenseParticipant) string {
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
		case split.EXACT:
			c.Value = money.Format(sh, e.Currency)
		case split.SHARE:
			c.Value = strconv.FormatInt(sh, 10)
		}
	}
	// PERCENTAGE: derive basis points and fix integer-truncation drift so the
	// values re-validate (they must sum to exactly 10000).
	if split.Method(method) == split.PERCENTAGE && e.Amount != 0 {
		idx := make([]int, 0, len(cands))
		bps := make([]int64, 0, len(cands))
		var sum int64
		for i := range cands {
			if !cands[i].Included {
				continue
			}
			bp := share(cands[i].ID) * split.BasisPointsFull / e.Amount
			idx = append(idx, i)
			bps = append(bps, bp)
			sum += bp
		}
		if len(idx) > 0 && sum != split.BasisPointsFull {
			hi := 0
			for k := 1; k < len(bps); k++ {
				if bps[k] > bps[hi] {
					hi = k
				}
			}
			bps[hi] += split.BasisPointsFull - sum
		}
		for k, i := range idx {
			cands[i].Value = money.Format(bps[k], "USD")
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
		method = prefillEdit(cands, e, parts)
	}

	groups, _ := s.Store.ListGroupsForUser(ctx, me.ID, false)
	type tg struct {
		ID   int64
		Name string
	}
	tgs := make([]tg, 0, len(groups))
	for _, g := range groups {
		tgs = append(tgs, tg{g.ID, g.Name})
	}
	s.render(w, r, "expense_form", "expense.edit", map[string]any{
		"Heading": s.tr(r, "expense.edit"), "Action": "/expenses/" + e.ID + "/move",
		"Name": e.Name, "Category": e.Category, "AmountStr": money.Format(e.Amount, e.Currency),
		"Currency": e.Currency, "Date": dateOnly(e.ExpenseDate), "PaidBy": e.PaidBy,
		"Method": method, "Methods": allMethods, "Candidates": cands,
		"ShowTarget": true, "TargetGroups": tgs, "TargetGroupSel": effGroup, "TargetGroupID": effGroup,
		"IsMove": true, "GroupChanged": groupChanged,
		"ShowNote": true, "Note": e.Note,
		"SubmitLabel": s.tr(r, "expense.save_changes"), "CancelURL": "/expenses/" + e.ID,
	})
}

func (s *Server) handleExpenseMove(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	origID := chi.URLParam(r, "id")
	in, err := parseExpenseInput(r, me.ID)
	if err != nil {
		s.renderErr(w, r, "message", "msg.move_failed", err.Error(), http.StatusBadRequest, err.Error())
		return
	}
	acknowledged := r.FormValue("ack") == "1"
	e, err := s.Svc.MoveExpense(ctx, origID, in, acknowledged)
	if err != nil {
		s.renderErr(w, r, "message", "msg.move_failed", err.Error(), http.StatusBadRequest, err.Error())
		return
	}
	http.Redirect(w, r, "/expenses/"+e.ID, http.StatusSeeOther)
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
	cands := []candidate{{ID: me.ID, Name: displayName(me), Included: true}, {ID: fid, Name: displayName(friend), Included: true}}
	s.render(w, r, "expense_form", "expense.settle", map[string]any{
		"Heading": s.tr(r, "expense.settle_with") + " " + displayName(friend), "Action": fmt.Sprintf("/friends/%d/settle", fid),
		"Name": "Settlement", "Category": "settlement", "AmountStr": settleSuggestion(bals),
		"Currency": settleCurrency(bals, me.DefaultCurrency), "Date": time.Now().Format("2006-01-02"),
		"PaidBy": me.ID, "Method": "EQUAL", "Methods": []string{"EQUAL"},
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
	currency := strings.ToUpper(r.FormValue("currency"))
	amount, err := money.Parse(r.FormValue("amount"), currency)
	if err != nil || amount <= 0 {
		s.renderErr(w, r, "message", "msg.settle_failed", s.tr(r, "err.invalid_settlement_amount"), http.StatusBadRequest, "invalid amount")
		return
	}
	// The current user pays the friend (drives their balance toward zero).
	if _, err := s.Svc.Settle(ctx, me.ID, fid, amount, currency, nil, r.FormValue("date"), me.ID); err != nil {
		s.renderErr(w, r, "message", "msg.settle_failed", err.Error(), http.StatusBadRequest, err.Error())
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/friends/%d", fid), http.StatusSeeOther)
}

func settleSuggestion(bals []store.CumulatedBalance) string {
	for _, b := range bals {
		if b.Amount < 0 {
			return money.Format(-b.Amount, b.Currency)
		}
	}
	return ""
}

func settleCurrency(bals []store.CumulatedBalance, def string) string {
	if len(bals) > 0 {
		return bals[0].Currency
	}
	return def
}
