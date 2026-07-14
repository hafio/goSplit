package httpapp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/hafio/gosplit/internal/balance"
	"github.com/hafio/gosplit/internal/store"
	"github.com/hafio/gosplit/internal/web"
)

func urlQueryEscape(s string) string { return url.QueryEscape(s) }

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
	if err != nil {
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
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	hidden := map[int64]bool{}
	for _, id := range u.HiddenFriendIDs {
		hidden[id] = true
	}
	cum, err := s.Store.CumulatedBalances(ctx, u.ID)
	if err != nil {
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

func (s *Server) handleFriendDetail(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	u := s.currentUser(r)
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
	bals, _ := s.Store.FriendBalance(ctx, u.ID, fid)
	filter, view := parseFilter(r)
	showAll := applyFeedLimit(r, &filter)
	expenses, err := s.Store.ListFriendExpenses(ctx, u.ID, fid, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	href := showAllHref(r, showAll, &expenses)
	hidden := false
	for _, id := range u.HiddenFriendIDs {
		if id == fid {
			hidden = true
			break
		}
	}
	nc := s.newNameCache()
	s.render(w, r, "friend", friend.Name, map[string]any{
		"Friend": friend, "Balances": bals, "Hidden": hidden,
		"Expenses": groupByMonth(s.buildRows(ctx, nc, u.ID, expenses)), "Filter": view,
		"ShowAllHref": href,
	})
}

func (s *Server) handleFriendAdd(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	u := s.currentUser(r)
	if err := s.Svc.AddFriendByEmail(ctx, u.ID, r.FormValue("email")); err != nil {
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
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	counts, err := s.Store.GroupMemberCounts(ctx, u.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	nets, err := s.Store.UserGroupNets(ctx, u.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rows := make([]groupRow, 0, len(groups))
	for _, g := range groups {
		rows = append(rows, groupRow{Group: g, MemberCount: counts[g.ID], Balances: sortedNets(nets[g.ID])})
	}
	s.render(w, r, "groups", "title.groups", map[string]any{"Groups": rows})
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
	gid, ok := atoi64(chi.URLParam(r, "id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	g, err := s.Store.GetGroup(ctx, gid)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	member, _ := s.Store.IsGroupMember(ctx, gid, u.ID)
	if !member {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	members, _ := s.Store.GroupMembers(ctx, gid)
	filter, view := parseFilter(r)
	showAll := applyFeedLimit(r, &filter)
	expenses, _ := s.Store.ListGroupExpenses(ctx, gid, filter)
	href := showAllHref(r, showAll, &expenses)
	nc := s.newNameCache()

	simplified := s.computeGroupSettlements(ctx, nc, gid)

	// The current user's own per-currency net position within this group.
	gbals, _ := s.Store.UserGroupBalances(ctx, gid, u.ID)
	perCur := map[string]int64{}
	for _, b := range gbals {
		perCur[b.Currency] += b.Amount
	}

	s.render(w, r, "group", g.Name, map[string]any{
		"Group": g, "Members": members,
		"JoinURL":    fmt.Sprintf("%s/g/%s", s.Cfg.BaseURL, g.PublicID),
		"Simplified": simplified, "Position": sortedNets(perCur),
		"Expenses": groupByMonth(s.buildRows(ctx, nc, u.ID, expenses)), "Filter": view,
		"ShowAllHref": href,
	})
}

// computeGroupSettlements derives per-currency net positions from the group's
// balances and runs debt simplification (spec §9) for display. The balance_view
// already emits both directions, so each unordered pair is counted once.
func (s *Server) computeGroupSettlements(ctx context.Context, nc *nameCache, gid int64) []settlementRow {
	rows, _ := s.Store.GroupBalances(ctx, gid)
	netByCur := map[string]map[int64]int64{}
	seen := map[string]bool{}
	for _, b := range rows {
		key := fmt.Sprintf("%s|%d|%d", b.Currency, min64(b.UserID, b.FriendID), max64(b.UserID, b.FriendID))
		if seen[key] {
			continue
		}
		seen[key] = true
		if netByCur[b.Currency] == nil {
			netByCur[b.Currency] = map[int64]int64{}
		}
		// b.Amount is user_id's net vs friend_id for this pair.
		netByCur[b.Currency][b.UserID] += b.Amount
		netByCur[b.Currency][b.FriendID] -= b.Amount
	}
	var out []settlementRow
	currencies := make([]string, 0, len(netByCur))
	for c := range netByCur {
		currencies = append(currencies, c)
	}
	sort.Strings(currencies)
	for _, c := range currencies {
		for _, t := range balance.Simplify(netByCur[c]) {
			out = append(out, settlementRow{
				FromID: t.From, ToID: t.To,
				FromName: nc.user(ctx, t.From), ToName: nc.user(ctx, t.To),
				Amount: t.Amount, Currency: c,
			})
		}
	}
	return out
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func (s *Server) handleGroupCreate(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	u := s.currentUser(r)
	name := r.FormValue("name")
	cur := r.FormValue("currency")
	if cur == "" {
		cur = u.DefaultCurrency
	}
	g, err := s.Store.CreateGroup(ctx, &store.Group{Name: name, CreatedBy: u.ID, DefaultCurrency: cur})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/groups/%d", g.ID), http.StatusSeeOther)
}

func (s *Server) handleGroupArchive(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	gid, _ := atoi64(chi.URLParam(r, "id"))
	g, err := s.Store.GetGroup(ctx, gid)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = s.Store.SetGroupArchived(ctx, gid, !g.IsArchived())
	http.Redirect(w, r, fmt.Sprintf("/groups/%d", gid), http.StatusSeeOther)
}

func (s *Server) handleGroupSimplify(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	gid, _ := atoi64(chi.URLParam(r, "id"))
	g, err := s.Store.GetGroup(ctx, gid)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = s.Store.SetGroupSimplify(ctx, gid, !g.SimplifyDebts)
	http.Redirect(w, r, fmt.Sprintf("/groups/%d", gid), http.StatusSeeOther)
}

func (s *Server) handleGroupInvite(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	u := s.currentUser(r)
	gid, _ := atoi64(chi.URLParam(r, "id"))
	_ = s.Svc.InviteToGroup(ctx, u, gid, r.FormValue("email"))
	http.Redirect(w, r, fmt.Sprintf("/groups/%d", gid), http.StatusSeeOther)
}

func (s *Server) handleGroupJoinPage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	g, err := s.Store.GetGroupByPublicID(ctx, chi.URLParam(r, "publicId"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	vd := s.vd(r, s.tr(r, "title.join")+" "+g.Name, fmt.Sprintf(s.tr(r, "msg.group_invite"), g.Name))
	s.Renderer.Render(w, http.StatusOK, "message", vd)
}

func (s *Server) handleGroupJoin(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	u := s.currentUser(r)
	g, err := s.Store.GetGroupByPublicID(ctx, chi.URLParam(r, "publicId"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = s.Store.AddGroupMember(ctx, g.ID, u.ID)
	http.Redirect(w, r, fmt.Sprintf("/groups/%d", g.ID), http.StatusSeeOther)
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
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	href := showAllHref(r, showAll, &expenses)
	nc := s.newNameCache()
	s.render(w, r, "activity", "title.activity", map[string]any{
		"Items": groupByMonth(s.buildRows(ctx, nc, u.ID, expenses)), "Filter": view,
		"ShowAllHref": href,
	})
}

// --- profile & admin ------------------------------------------------------

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "profile", "title.profile", map[string]any{"Languages": s.Renderer.Languages()})
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
			s.renderErr(w, r, "profile", "title.profile",
				map[string]any{"Languages": s.Renderer.Languages()},
				http.StatusBadRequest, s.tr(r, "err.unknown_theme"))
			return
		}
		u.ThemeColor = t
	}
	if err := s.Svc.UpdateAvatar(ctx, u, r); err != nil {
		s.renderErr(w, r, "profile", "title.profile", nil, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.Store.UpdateProfile(ctx, u); err != nil {
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
		s.renderErr(w, r, "profile", "title.profile", nil, http.StatusBadRequest, err.Error())
		return
	}
	s.Auth.ClearSession(w, r)
	http.Redirect(w, r, "/login?flash=flash.password_updated", http.StatusSeeOther)
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	u := s.currentUser(r)
	data, err := s.Svc.ExportUserData(ctx, u.ID)
	if err != nil {
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
	if err != nil {
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
	vd := s.vd(r, "title.admin", map[string]any{
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
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := s.Svc.AdminSetPassword(ctx, uid, r.FormValue("password")); err != nil {
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
	if !ok {
		http.NotFound(w, r)
		return
	}
	u, err := s.Store.GetUser(ctx, uid)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	u.Name = strings.TrimSpace(r.FormValue("name"))
	if email := strings.TrimSpace(r.FormValue("email")); email != "" {
		u.Email = email
	}
	if role := r.FormValue("role"); role == "USER" || role == "ADMIN" {
		u.Role = role
	}
	if cur := strings.TrimSpace(r.FormValue("currency")); cur != "" {
		u.Currency = cur
	}
	if lang := r.FormValue("language"); lang != "" {
		u.PreferredLanguage = lang
	}
	if err := s.Store.AdminUpdateUser(ctx, u); err != nil {
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
	if !ok {
		http.NotFound(w, r)
		return
	}
	if _, err := s.Store.GetUser(ctx, uid); err != nil {
		http.NotFound(w, r)
		return
	}
	link, err := s.Svc.AdminMagicLinkForUser(ctx, uid)
	if err != nil {
		s.renderAdmin(w, r, http.StatusInternalServerError, "", 0, s.tr(r, "err.admin_magic")+": "+err.Error())
		return
	}
	s.renderAdmin(w, r, http.StatusOK, link, uid, "")
}

// redirectFlash redirects to path with a flash message query param.
func (s *Server) redirectFlash(w http.ResponseWriter, r *http.Request, path, msg string) {
	if msg != "" {
		path += "?flash=" + urlQueryEscape(msg)
	}
	http.Redirect(w, r, path, http.StatusSeeOther)
}
