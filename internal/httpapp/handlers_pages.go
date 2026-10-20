package httpapp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/hafio/gosplit/internal/balance"
	"github.com/hafio/gosplit/internal/service"
	"github.com/hafio/gosplit/internal/store"
	"github.com/hafio/gosplit/internal/web"
)

// --- balances -------------------------------------------------------------

type balanceRow struct {
	FriendID int64
	Name     string
	Currency string
	Amount   int64
}

type currencyNet struct {
	Currency string
	Amount   int64
}

func (s *Server) handleBalances(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	u := s.currentUser(r)
	cum, err := s.Store.CumulatedBalances(ctx, u.ID)
	if err != nil { // cov:ignore store-fault: a read that fails only on a broken database
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	nc := s.newNameCache()
	rows := make([]balanceRow, 0, len(cum))
	summary := map[string]int64{}
	for _, c := range cum {
		rows = append(rows, balanceRow{c.FriendID, nc.user(ctx, c.FriendID), c.Currency, c.Amount})
		summary[c.Currency] += c.Amount
	}
	s.render(w, r, "balances", "title.balances", map[string]any{
		"Rows": rows, "Summary": sortedNets(summary),
	})
}

func sortedNets(m map[string]int64) []currencyNet {
	out := make([]currencyNet, 0, len(m))
	for c, a := range m {
		if a != 0 {
			out = append(out, currencyNet{c, a})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Currency < out[j].Currency })
	return out
}

// --- friends --------------------------------------------------------------

type friendRow struct {
	User     *store.User
	Balances []store.CumulatedBalance
	Hidden   bool
}

func (s *Server) handleFriends(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	u := s.currentUser(r)
	friends, err := s.Store.ListFriends(ctx, u.ID)
	if err != nil { // cov:ignore store-fault: a read that fails only on a broken database
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	hidden := map[int64]bool{}
	for _, id := range u.HiddenFriendIDs {
		hidden[id] = true
	}
	cum, err := s.Store.CumulatedBalances(ctx, u.ID)
	if err != nil { // cov:ignore store-fault: a read that fails only on a broken database
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	balsByFriend := map[int64][]store.CumulatedBalance{}
	for _, c := range cum {
		balsByFriend[c.FriendID] = append(balsByFriend[c.FriendID], c)
	}
	rows := make([]friendRow, 0, len(friends))
	for _, f := range friends {
		rows = append(rows, friendRow{User: f, Balances: balsByFriend[f.ID], Hidden: hidden[f.ID]})
	}
	s.render(w, r, "friends", "title.friends", map[string]any{"Friends": rows})
}

// resolveRelated returns the user a friend-scoped page's {id} names, when the
// caller shares a friendship, a group or an expense with them. Anyone else gets
// the same 404 as an unknown id: ids are sequential, and these pages show a
// name or an email. A failed lookup counts as unrelated.
func (s *Server) resolveRelated(ctx context.Context, w http.ResponseWriter, r *http.Request, me *store.User) (*store.User, bool) {
	fid, ok := atoi64(chi.URLParam(r, "id"))
	if !ok {
		http.NotFound(w, r)
		return nil, false
	}
	if related, err := s.Store.AreRelated(ctx, me.ID, fid); err != nil || !related {
		http.NotFound(w, r)
		return nil, false
	}
	u, err := s.Store.GetUser(ctx, fid)
	if err != nil { // cov:ignore impossible: a related user's row exists
		http.NotFound(w, r)
		return nil, false
	}
	return u, true
}

func (s *Server) handleFriendDetail(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	u := s.currentUser(r)
	friend, ok := s.resolveRelated(ctx, w, r, u)
	if !ok {
		return
	}
	fid := friend.ID
	bals, _ := s.Store.FriendBalance(ctx, u.ID, fid)
	filter, view := parseFilter(r)
	showAll := applyFeedLimit(r, &filter)
	expenses, err := s.Store.ListFriendExpenses(ctx, u.ID, fid, filter)
	if err != nil { // cov:ignore store-fault: a read that fails only on a broken database
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	href := showAllHref(r, showAll, &expenses)
	hidden := false
	for _, id := range u.HiddenFriendIDs { // cov:ignore glue: only labels the hide button
		if id == fid {
			hidden = true
			break
		}
	}
	nc := s.newNameCache()
	data := map[string]any{
		"Friend": friend, "Balances": bals, "Hidden": hidden,
		"Expenses": groupByMonth(s.buildRows(ctx, nc, u.ID, expenses)), "Filter": view,
		"ShowAllHref": href,
	}
	s.render(w, r, "friend", friend.Name, data)
}

func (s *Server) handleFriendAdd(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	u := s.currentUser(r)
	if err := s.Svc.AddFriendByEmail(ctx, u.ID, r.FormValue("email")); err != nil { // cov:ignore pass-through: the refusal rules are tested in internal/service
		s.redirectFlash(w, r, "/friends", "")
		return
	}
	http.Redirect(w, r, "/friends", http.StatusSeeOther)
}

func (s *Server) handleFriendHide(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	u := s.currentUser(r)
	fid, _ := atoi64(chi.URLParam(r, "id"))
	_ = s.Svc.ToggleHiddenFriend(ctx, u, fid)
	http.Redirect(w, r, "/friends", http.StatusSeeOther)
}

func (s *Server) handleFriendDelete(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	u := s.currentUser(r)
	fid, _ := atoi64(chi.URLParam(r, "id"))
	_ = s.Store.RemoveFriend(ctx, u.ID, fid)
	http.Redirect(w, r, "/friends", http.StatusSeeOther)
}

// --- groups ---------------------------------------------------------------

type groupRow struct {
	Group       *store.Group
	MemberCount int
	Balances    []currencyNet
}

func (s *Server) handleGroups(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	u := s.currentUser(r)
	groups, err := s.Store.ListGroupsForUser(ctx, u.ID, true)
	if err != nil { // cov:ignore store-fault: a read that fails only on a broken database
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	counts, err := s.Store.GroupMemberCounts(ctx, u.ID)
	if err != nil { // cov:ignore store-fault: a read that fails only on a broken database
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	nets, err := s.Store.UserGroupNets(ctx, u.ID)
	if err != nil { // cov:ignore store-fault: a read that fails only on a broken database
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Split into the normal list and a normally-hidden "Archived" section.
	// Per-group balances (nets) are still attached to archived rows so their
	// unsettled debt shows when the section is expanded.
	active := make([]groupRow, 0, len(groups))
	var archived []groupRow
	for _, g := range groups {
		row := groupRow{Group: g, MemberCount: counts[g.ID], Balances: sortedNets(nets[g.ID])}
		if g.IsArchived() {
			archived = append(archived, row)
		} else {
			active = append(active, row)
		}
	}
	s.render(w, r, "groups", "title.groups", map[string]any{"Groups": active, "Archived": archived})
}

type settlementRow struct {
	FromID   int64
	ToID     int64
	FromName string
	ToName   string
	Amount   int64
	Currency string
}

func (s *Server) handleGroupDetail(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	u := s.currentUser(r)
	g, member, ok := s.resolveGroupViewer(ctx, w, r, u)
	if !ok {
		return
	}
	gid := g.ID
	// readOnly: an admin viewing a group they are not in. They see its balances
	// and feed, but none of the member-only controls -- and never the join link,
	// which would let them in. The page also stays out of the offline cache.
	readOnly := !member
	if readOnly {
		markPrivate(w)
	}
	members, _ := s.Store.GroupMembers(ctx, gid)
	// Friends of the current user who aren't already in the group -- the "add a
	// friend by name" picker. Self is never a friend, so it's excluded for free.
	var addable []*store.User
	if !readOnly {
		friends, _ := s.Store.ListFriends(ctx, u.ID)
		inGroup := make(map[int64]bool, len(members))
		for _, m := range members {
			inGroup[m.ID] = true
		}
		addable = make([]*store.User, 0, len(friends))
		for _, f := range friends {
			if !inGroup[f.ID] {
				addable = append(addable, f)
			}
		}
	}
	filter, view := parseFilter(r)
	showAll := applyFeedLimit(r, &filter)
	expenses, _ := s.Store.ListGroupExpenses(ctx, gid, filter)
	href := showAllHref(r, showAll, &expenses)
	nc := s.newNameCache()

	simplified, pairs := s.groupSettlementsAndPairs(ctx, nc, gid, g.SimplifyDebts)

	// The current user's own per-currency net position within this group.
	gbals, _ := s.Store.UserGroupBalances(ctx, gid, u.ID)
	perCur := map[string]int64{}
	for _, b := range gbals {
		perCur[b.Currency] += b.Amount
	}

	joinURL := ""
	if !readOnly {
		joinURL = fmt.Sprintf("%s/g/%s", s.Cfg.BaseURL, g.PublicID)
	}
	data := map[string]any{
		"Group": g, "Members": members, "AddableFriends": addable,
		"JoinURL":    joinURL, "ReadOnly": readOnly,
		// Settled, but pairs still show leftovers (settled before netting existed,
		// or uneven after later edits): members get a one-click tidy-up.
		"NeedsTidy": !readOnly && balance.NeedsTidy(pairs),
		"Simplified": simplified, "Position": sortedNets(perCur),
		"Expenses": groupByMonth(s.buildRows(ctx, nc, u.ID, expenses)), "Filter": view,
		"ShowAllHref": href,
	}
	s.render(w, r, "group", g.Name, data)
}

// computeGroupSettlements turns the group's balances into the payment rows shown
// on the group page: with simplify set (the group's SimplifyDebts flag) the
// per-currency nets go through debt simplification (spec §9); otherwise each
// pair keeps its own raw debt, oriented debtor → creditor.
func (s *Server) computeGroupSettlements(ctx context.Context, nc *nameCache, gid int64, simplify bool) []settlementRow {
	rows, _ := s.groupSettlementsAndPairs(ctx, nc, gid, simplify)
	return rows
}

// groupSettlementsAndPairs is computeGroupSettlements plus the pair balances it
// was computed from, so a caller that also needs them (the group page's tidy-up
// check) reads balance_view once. The pairing and the nets come from package
// balance -- the same code the netting writer uses -- so the page and the
// stored netting entries cannot disagree about what "simplified" means. A read
// failure is logged and shows as no balances.
func (s *Server) groupSettlementsAndPairs(ctx context.Context, nc *nameCache, gid int64, simplify bool) ([]settlementRow, []balance.Pair) {
	stored, err := s.Store.GroupBalances(ctx, gid)
	if err != nil { // cov:ignore log-only: the page degrades to no balances
		slog.Warn("group balances unavailable", "group", gid, "err", err)
	}
	rows := make([]balance.Row, 0, len(stored))
	for _, b := range stored {
		rows = append(rows, balance.Row{UserID: b.UserID, FriendID: b.FriendID, Currency: b.Currency, Amount: b.Amount})
	}
	pairs := balance.PairsFromRows(rows)
	if !simplify {
		return rawSettlementRows(ctx, nc, pairs), pairs
	}
	nets := balance.Nets(pairs)
	currencies := make([]string, 0, len(nets))
	for c := range nets {
		currencies = append(currencies, c)
	}
	sort.Strings(currencies)
	var out []settlementRow
	for _, c := range currencies {
		for _, t := range balance.Simplify(nets[c]) {
			out = append(out, settlementRow{
				FromID: t.From, ToID: t.To,
				FromName: nc.user(ctx, t.From), ToName: nc.user(ctx, t.To),
				Amount: t.Amount, Currency: c,
			})
		}
	}
	return out, pairs
}

// rawSettlementRows renders each pair's own debt without simplification, oriented
// debtor → creditor, in a deterministic order.
func rawSettlementRows(ctx context.Context, nc *nameCache, pairs []balance.Pair) []settlementRow {
	out := make([]settlementRow, 0, len(pairs))
	for _, p := range pairs {
		// Owed > 0: B owes A.
		from, to, amt := p.B, p.A, p.Owed
		if p.Owed < 0 {
			from, to, amt = p.A, p.B, -p.Owed
		}
		out = append(out, settlementRow{
			FromID: from, ToID: to,
			FromName: nc.user(ctx, from), ToName: nc.user(ctx, to),
			Amount: amt, Currency: p.Currency,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Currency != out[j].Currency { // cov:ignore glue: sort tie-breaks; the pairs already arrive sorted
			return out[i].Currency < out[j].Currency
		}
		if out[i].FromID != out[j].FromID {
			return out[i].FromID < out[j].FromID
		}
		return out[i].ToID < out[j].ToID // cov:ignore glue: sort tie-breaks; the pairs already arrive sorted
	})
	return out
}

// renderLimited answers a LIMITED account that tried to create a group or run an
// import: 403 with a page that says why and who can change it, the same shape
// as the signup-disabled page.
func (s *Server) renderLimited(w http.ResponseWriter, r *http.Request) {
	vd := s.vdPage(w, r, "msg.limited_title", s.tr(r, "msg.limited_body"))
	s.Renderer.Render(w, http.StatusForbidden, "message", vd)
}

func (s *Server) handleGroupCreate(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	u := s.currentUser(r)
	name := r.FormValue("name")
	cur := r.FormValue("currency")
	if cur == "" { // cov:ignore glue: only a hand-built request skips this form field
		cur = u.DefaultCurrency
	}
	// Debt simplification on by default; the overflow menu turns it off to show
	// raw pairwise balances instead.
	g, err := s.Svc.CreateGroup(ctx, u, &store.Group{Name: name, DefaultCurrency: cur, SimplifyDebts: true})
	if errors.Is(err, service.ErrLimitedUser) {
		s.renderLimited(w, r)
		return
	}
	if err != nil { // cov:ignore store-fault: a write that fails only on a broken database
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/groups/%d", g.ID), http.StatusSeeOther)
}

// handleGroupArchive toggles a group's archived flag. Members only: archiving
// drops the group's debts out of every member's aggregate balances, and group
// ids are sequential, so an unchecked toggle let anyone hide any group's debt.
func (s *Server) handleGroupArchive(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	g, ok := s.resolveGroupCaller(ctx, w, r, s.currentUser(r))
	if !ok {
		return
	}
	_ = s.Store.SetGroupArchived(ctx, g.ID, !g.IsArchived())
	http.Redirect(w, r, fmt.Sprintf("/groups/%d", g.ID), http.StatusSeeOther)
}

// handleGroupSimplify toggles debt simplification. Members only, for the same
// reason as archiving: it changes what every member is told to pay.
func (s *Server) handleGroupSimplify(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	g, ok := s.resolveGroupCaller(ctx, w, r, s.currentUser(r))
	if !ok {
		return
	}
	_ = s.Store.SetGroupSimplify(ctx, g.ID, !g.SimplifyDebts)
	http.Redirect(w, r, fmt.Sprintf("/groups/%d", g.ID), http.StatusSeeOther)
}

// handleGroupTidy clears a settled group's leftover pair balances (Service.
// TidyGroup). Members only; a failure is shown, since tidying is the whole action.
func (s *Server) handleGroupTidy(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	g, ok := s.resolveGroupCaller(ctx, w, r, me)
	if !ok {
		return
	}
	if _, err := s.Svc.TidyGroup(ctx, me, g.ID); err != nil { // cov:ignore store-fault: a write that fails only on a broken database
		s.renderErr(w, r, "message", "msg.tidy_failed", err.Error(), http.StatusInternalServerError, err.Error())
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/groups/%d", g.ID), http.StatusSeeOther)
}

// handleGroupInvite adds a member. The services refuse non-members too, but
// their errors are not surfaced here, so the explicit check is what turns a
// refused invite into a 403 instead of a silent redirect.
func (s *Server) handleGroupInvite(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	u := s.currentUser(r)
	g, ok := s.resolveGroupCaller(ctx, w, r, u)
	if !ok {
		return
	}
	gid := g.ID
	// Prefer the friend picker (add an existing friend by name); fall back to the
	// email field for inviting someone who isn't a friend yet.
	if fid, ok := atoi64(r.FormValue("friend_id")); ok && fid > 0 {
		_ = s.Svc.AddFriendToGroup(ctx, u, gid, fid)
	} else {
		_ = s.Svc.InviteToGroup(ctx, u, gid, r.FormValue("email"))
	}
	http.Redirect(w, r, fmt.Sprintf("/groups/%d", gid), http.StatusSeeOther)
}

// handleGroupJoinPage shows the invitation behind a group's join link. Nothing
// joins anyone: there is deliberately no route that adds the caller, because
// the link travels in chats and email. A member adds them with Add member.
func (s *Server) handleGroupJoinPage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	g, err := s.Store.GetGroupByPublicID(ctx, chi.URLParam(r, "publicId"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	vd := s.vdPage(w, r, s.tr(r, "title.join")+" "+g.Name, fmt.Sprintf(s.tr(r, "msg.group_invite"), g.Name))
	s.Renderer.Render(w, http.StatusOK, "message", vd)
}

// --- activity -------------------------------------------------------------

func (s *Server) handleActivity(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	u := s.currentUser(r)
	filter, view := parseFilter(r)
	if gid := r.URL.Query().Get("group"); gid != "" {
		if id, ok := atoi64(gid); ok {
			filter.GroupID = &id
		}
	}
	showAll := applyFeedLimit(r, &filter)
	expenses, err := s.Store.ListActivity(ctx, u.ID, filter)
	if err != nil { // cov:ignore store-fault: a read that fails only on a broken database
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	href := showAllHref(r, showAll, &expenses)
	nc := s.newNameCache()
	data := map[string]any{
		"Items": groupByMonth(s.buildRows(ctx, nc, u.ID, expenses)), "Filter": view,
		"ShowAllHref": href,
	}
	s.render(w, r, "activity", "title.activity", data)
}

// --- profile & admin ------------------------------------------------------

// profileData is the context profile.html needs to render. Every path that
// renders that page -- including the validation-failure ones -- has to pass it,
// or the template cannot resolve .Data.Languages.
func (s *Server) profileData() map[string]any {
	return map[string]any{"Languages": s.Renderer.Languages()}
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "profile", "title.profile", s.profileData())
}

func (s *Server) handleProfileUpdate(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	u := s.currentUser(r)
	u.Name = r.FormValue("name")
	if c := r.FormValue("currency"); c != "" {
		u.Currency = c
	}
	if c := r.FormValue("default_currency"); c != "" {
		u.DefaultCurrency = c
	}
	if l := r.FormValue("language"); l != "" {
		u.PreferredLanguage = l
	}
	if t := r.FormValue("theme"); t != "" {
		if !web.ValidTheme(t) {
			s.renderErr(w, r, "profile", "title.profile", s.profileData(),
				http.StatusBadRequest, s.tr(r, "err.unknown_theme"))
			return
		}
		u.ThemeColor = t
	}
	// Assigned unconditionally, unlike the fields above: an unchecked box is
	// absent from the POST body, so an "only if present" guard would make the
	// opt-in impossible to turn back off.
	u.EmailExpenseNotify = r.FormValue("email_expense_notify") != ""
	if err := s.Svc.UpdateAvatar(ctx, u, r); err != nil { // cov:ignore pass-through: the avatar rules are tested in internal/service
		s.renderErr(w, r, "profile", "title.profile", s.profileData(), http.StatusBadRequest, err.Error())
		return
	}
	if err := s.Store.UpdateProfile(ctx, u); err != nil { // cov:ignore store-fault: a write that fails only on a broken database
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.redirectFlash(w, r, "/profile", "flash.profile_saved")
}

func (s *Server) handlePasswordChange(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	u := s.currentUser(r)
	if err := s.Svc.ChangePassword(ctx, u.ID, r.FormValue("current"), r.FormValue("password")); err != nil {
		s.renderErr(w, r, "profile", "title.profile", s.profileData(), http.StatusBadRequest, err.Error())
		return
	}
	s.Auth.ClearSession(w, r)
	s.redirectFlash(w, r, "/login", "flash.password_updated")
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	u := s.currentUser(r)
	data, err := s.Svc.ExportUserData(ctx, u.ID)
	if err != nil { // cov:ignore store-fault: a read that fails only on a broken database
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=gosplit-export.json")
	_, _ = w.Write(data)
}

func (s *Server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	s.renderAdmin(w, r, http.StatusOK, "", 0, "")
}

// renderAdmin lists users, optionally surfacing a generated magic link inside
// the card of the user identified by magicForID, or an error banner.
func (s *Server) renderAdmin(w http.ResponseWriter, r *http.Request, status int, magicLink string, magicForID int64, errMsg string) {
	ctx := r.Context()
	users, err := s.Store.ListUsers(ctx)
	if err != nil { // cov:ignore store-fault: a read that fails only on a broken database
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	admins := 0
	active := 0
	for _, u := range users {
		if u.IsAdmin() {
			admins++
		}
		if u.IsActive() {
			active++
		}
	}
	vd := s.vdPage(w, r, "title.admin", map[string]any{
		"Users":       users,
		"Languages":   s.Renderer.Languages(),
		"MagicLink":   magicLink,
		"MagicForID":  magicForID,
		"CountUsers":  len(users),
		"CountAdmin":  admins,
		"CountActive": active,
	})
	vd.Error = errMsg
	s.Renderer.Render(w, status, "admin", vd)
}

// handleAdminCreate creates a new user from the admin "Add user" form.
func (s *Server) handleAdminCreate(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	_, err := s.Svc.AdminCreateUser(ctx, r.FormValue("name"), r.FormValue("email"),
		r.FormValue("role"), r.FormValue("currency"), r.FormValue("language"), r.FormValue("password"))
	if err != nil {
		s.renderAdmin(w, r, http.StatusBadRequest, "", 0, s.tr(r, "err.admin_create")+": "+err.Error())
		return
	}
	s.redirectFlash(w, r, "/admin", "flash.user_created")
}

// handleAdminSetPassword sets a user's password (revoking their sessions).
func (s *Server) handleAdminSetPassword(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	uid, ok := atoi64(chi.URLParam(r, "id"))
	if !ok { // cov:ignore glue: a bad id maps to 404, the same mapping tested elsewhere
		http.NotFound(w, r)
		return
	}
	if err := s.Svc.AdminSetPassword(ctx, uid, r.FormValue("password")); err != nil { // cov:ignore pass-through: the password policy is tested in internal/service
		s.renderAdmin(w, r, http.StatusBadRequest, "", 0, s.tr(r, "err.admin_set_password")+": "+err.Error())
		return
	}
	s.redirectFlash(w, r, "/admin", "flash.password_set")
}

func (s *Server) handleAdminToggle(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	uid, _ := atoi64(chi.URLParam(r, "id"))
	target, err := s.Store.GetUser(ctx, uid)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = s.Store.SetDeactivated(ctx, uid, target.IsActive())
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// handleAdminUpdate edits a user's identity fields (name, email, role, currency,
// language) on behalf of an admin.
func (s *Server) handleAdminUpdate(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	uid, ok := atoi64(chi.URLParam(r, "id"))
	if !ok { // cov:ignore glue: a bad id maps to 404, the same mapping tested elsewhere
		http.NotFound(w, r)
		return
	}
	u, err := s.Store.GetUser(ctx, uid)
	if err != nil { // cov:ignore glue: a bad id maps to 404, the same mapping tested elsewhere
		http.NotFound(w, r)
		return
	}
	u.Name = strings.TrimSpace(r.FormValue("name"))
	if email := strings.TrimSpace(r.FormValue("email")); email != "" {
		u.Email = email
	}
	if role := r.FormValue("role"); store.ValidRole(role) {
		u.Role = role
	}
	if cur := strings.TrimSpace(r.FormValue("currency")); cur != "" {
		u.Currency = cur
	}
	if lang := r.FormValue("language"); lang != "" {
		u.PreferredLanguage = lang
	}
	if err := s.Store.AdminUpdateUser(ctx, u); err != nil { // cov:ignore pass-through: the UNIQUE email error comes from internal/store
		s.renderAdmin(w, r, http.StatusBadRequest, "", 0, s.tr(r, "err.admin_update")+": "+err.Error())
		return
	}
	s.redirectFlash(w, r, "/admin", "flash.user_updated")
}

// handleAdminMagic mints a magic-link URL for a user and shows it to the admin.
func (s *Server) handleAdminMagic(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	uid, ok := atoi64(chi.URLParam(r, "id"))
	if !ok { // cov:ignore glue: a bad id maps to 404, the same mapping tested elsewhere
		http.NotFound(w, r)
		return
	}
	if _, err := s.Store.GetUser(ctx, uid); err != nil { // cov:ignore glue: a bad id maps to 404, the same mapping tested elsewhere
		http.NotFound(w, r)
		return
	}
	link, err := s.Svc.AdminMagicLinkForUser(ctx, uid)
	if err != nil { // cov:ignore store-fault: a write that fails only on a broken database
		s.renderAdmin(w, r, http.StatusInternalServerError, "", 0, s.tr(r, "err.admin_magic")+": "+err.Error())
		return
	}
	s.renderAdmin(w, r, http.StatusOK, link, uid, "")
}

// redirectFlash redirects to path carrying a one-shot confirmation message.
// msg is either an i18n key or an already-formatted string; it is resolved here
// so the stored message is display-ready and translated exactly once, never
// re-run through the bundle on the way out.
func (s *Server) redirectFlash(w http.ResponseWriter, r *http.Request, path, msg string) {
	if msg != "" {
		s.setFlash(w, s.tr(r, msg))
	}
	http.Redirect(w, r, path, http.StatusSeeOther)
}
