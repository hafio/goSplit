package httpapp

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/hafio/gosplit/internal/store"
)

// filterOpt is one choice in an admin filter select.
type filterOpt struct {
	ID    int64
	Label string
}

// handleAdminActivity is the admin feed: every user's transactions -- soft-deleted
// ones included, dimmed and tagged, read-only -- narrowed by ?user=, ?group= and
// the usual filter bar. Archived groups are always included: this is the record
// of everything, not a working view. A user or group id that is malformed or
// does not exist is ignored, so the feed falls back to "everyone" rather than to
// an empty list that looks like an answer.
func (s *Server) handleAdminActivity(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	filter, view := parseFilter(r)
	filter.IncludeArchived = true
	data := map[string]any{"UserSel": int64(0), "GroupSel": int64(0)}

	var userID int64
	if id, ok := atoi64(r.URL.Query().Get("user")); ok {
		if u, err := s.Store.GetUser(ctx, id); err == nil {
			userID = u.ID
			view["user"] = strconv.FormatInt(u.ID, 10)
			data["UserSel"], data["UserName"] = u.ID, displayName(u)
		}
	}
	if id, ok := atoi64(r.URL.Query().Get("group")); ok {
		if g, err := s.Store.GetGroup(ctx, id); err == nil {
			filter.GroupID = &g.ID
			view["group"] = strconv.FormatInt(g.ID, 10)
			data["GroupSel"], data["GroupName"] = g.ID, g.Name
		}
	}

	showAll := applyFeedLimit(r, &filter)
	expenses, err := s.Store.ListAllActivity(ctx, userID, filter)
	if err != nil { // cov:ignore store-fault: a read that fails only on a broken database
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	href := showAllHref(r, showAll, &expenses)

	users, _ := s.Store.ListUsers(ctx)
	userOpts := make([]filterOpt, 0, len(users))
	for _, u := range users {
		label := u.Email
		if u.Name != "" {
			label = fmt.Sprintf("%s (%s)", u.Name, u.Email)
		}
		userOpts = append(userOpts, filterOpt{u.ID, label})
	}
	groups, _ := s.Store.ListAllGroups(ctx, true)
	groupOpts := make([]filterOpt, 0, len(groups))
	for _, g := range groups {
		// Group names are not unique, so the id disambiguates.
		groupOpts = append(groupOpts, filterOpt{g.ID, fmt.Sprintf("%s (#%d)", g.Name, g.ID)})
	}

	nc := s.newNameCache()
	data["Items"] = groupByMonth(s.buildRows(ctx, nc, 0, expenses))
	data["Filter"] = view
	data["ShowAllHref"] = href
	data["UserOpts"] = userOpts
	data["GroupOpts"] = groupOpts
	s.render(w, r, "admin_activity", "title.admin_activity", data)
}

// groupListRow is one line of the admin group list.
type groupListRow struct {
	Group       *store.Group
	MemberCount int
	CreatorName string
	Created     string // creation date, YYYY-MM-DD
}

// handleAdminGroups lists every group on the instance, archived ones included,
// each linking to its read-only page and to its slice of the admin feed. The
// join token (PublicID) is never shown: holding it lets anyone join.
func (s *Server) handleAdminGroups(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	groups, err := s.Store.ListAllGroups(ctx, true)
	if err != nil { // cov:ignore store-fault: a read that fails only on a broken database
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	counts, _ := s.Store.AllGroupMemberCounts(ctx)
	nc := s.newNameCache()
	rows := make([]groupListRow, 0, len(groups))
	for _, g := range groups {
		rows = append(rows, groupListRow{Group: g, MemberCount: counts[g.ID],
			CreatorName: nc.user(ctx, g.CreatedBy), Created: dateOnly(g.CreatedAt)})
	}
	s.render(w, r, "admin_groups", "title.admin_groups", map[string]any{"Groups": rows})
}
